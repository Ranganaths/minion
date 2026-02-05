package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Neo4jConfig configures the Neo4j connection
type Neo4jConfig struct {
	URI      string
	Username string
	Password string
	Database string
	MaxConns int
}

// Neo4jDriver interface abstracts the Neo4j driver for testing
type Neo4jDriver interface {
	// ExecuteQuery executes a Cypher query and returns results
	ExecuteQuery(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)

	// ExecuteWrite executes a write transaction
	ExecuteWrite(ctx context.Context, query string, params map[string]interface{}) error

	// Close closes the driver connection
	Close() error
}

// Neo4jKnowledgeGraph implements KnowledgeGraph using Neo4j
type Neo4jKnowledgeGraph struct {
	driver   Neo4jDriver
	database string
	embedder Embedder
}

// NewNeo4jKnowledgeGraph creates a new Neo4j-backed knowledge graph
func NewNeo4jKnowledgeGraph(driver Neo4jDriver, database string, embedder Embedder) *Neo4jKnowledgeGraph {
	if database == "" {
		database = "neo4j"
	}
	return &Neo4jKnowledgeGraph{
		driver:   driver,
		database: database,
		embedder: embedder,
	}
}

// Initialize creates necessary indexes and constraints
func (kg *Neo4jKnowledgeGraph) Initialize(ctx context.Context) error {
	// Create constraint for unique entity IDs
	constraints := []string{
		"CREATE CONSTRAINT entity_id IF NOT EXISTS FOR (e:Entity) REQUIRE e.id IS UNIQUE",
		"CREATE INDEX entity_type IF NOT EXISTS FOR (e:Entity) ON (e.type)",
		"CREATE INDEX entity_name IF NOT EXISTS FOR (e:Entity) ON (e.name)",
		"CREATE INDEX relation_type IF NOT EXISTS FOR ()-[r:RELATES]-() ON (r.type)",
	}

	for _, constraint := range constraints {
		if err := kg.driver.ExecuteWrite(ctx, constraint, nil); err != nil {
			// Ignore errors for already existing constraints
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("failed to create constraint: %w", err)
			}
		}
	}

	return nil
}

// AddEntity adds an entity to the knowledge graph
func (kg *Neo4jKnowledgeGraph) AddEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		return errors.New("entity ID is required")
	}

	// Generate embedding if embedder is available and embedding is empty
	if kg.embedder != nil && len(entity.Embedding) == 0 && entity.Description != "" {
		embedding, err := kg.embedder.Embed(ctx, entity.Description)
		if err == nil {
			entity.Embedding = embedding
		}
	}

	propsJSON, err := json.Marshal(entity.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	embeddingJSON, err := json.Marshal(entity.Embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	query := `
		MERGE (e:Entity {id: $id})
		SET e.name = $name,
			e.type = $type,
			e.description = $description,
			e.properties = $properties,
			e.embedding = $embedding,
			e.createdAt = COALESCE(e.createdAt, datetime()),
			e.updatedAt = datetime()
		RETURN e
	`

	params := map[string]interface{}{
		"id":          entity.ID,
		"name":        entity.Name,
		"type":        string(entity.Type),
		"description": entity.Description,
		"properties":  string(propsJSON),
		"embedding":   string(embeddingJSON),
	}

	return kg.driver.ExecuteWrite(ctx, query, params)
}

// GetEntity retrieves an entity by ID
func (kg *Neo4jKnowledgeGraph) GetEntity(ctx context.Context, id string) (*Entity, error) {
	query := `
		MATCH (e:Entity {id: $id})
		RETURN e.id AS id, e.name AS name, e.type AS type, e.description AS description,
			   e.properties AS properties, e.embedding AS embedding,
			   e.createdAt AS createdAt, e.updatedAt AS updatedAt
	`

	results, err := kg.driver.ExecuteQuery(ctx, query, map[string]interface{}{"id": id})
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return kg.recordToEntity(results[0])
}

// UpdateEntity updates an existing entity
func (kg *Neo4jKnowledgeGraph) UpdateEntity(ctx context.Context, entity *Entity) error {
	return kg.AddEntity(ctx, entity) // MERGE handles update
}

// DeleteEntity removes an entity and its relationships
func (kg *Neo4jKnowledgeGraph) DeleteEntity(ctx context.Context, id string) error {
	query := `
		MATCH (e:Entity {id: $id})
		DETACH DELETE e
	`
	return kg.driver.ExecuteWrite(ctx, query, map[string]interface{}{"id": id})
}

// AddRelation adds a relationship between entities
func (kg *Neo4jKnowledgeGraph) AddRelation(ctx context.Context, relation *Relation) error {
	if relation.ID == "" {
		return errors.New("relation ID is required")
	}

	propsJSON, err := json.Marshal(relation.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	query := `
		MATCH (from:Entity {id: $fromId})
		MATCH (to:Entity {id: $toId})
		MERGE (from)-[r:RELATES {id: $id}]->(to)
		SET r.type = $type,
			r.properties = $properties,
			r.weight = $weight,
			r.createdAt = COALESCE(r.createdAt, datetime()),
			r.updatedAt = datetime()
		RETURN r
	`

	params := map[string]interface{}{
		"id":         relation.ID,
		"fromId":     relation.FromID,
		"toId":       relation.ToID,
		"type":       string(relation.Type),
		"properties": string(propsJSON),
		"weight":     relation.Weight,
	}

	return kg.driver.ExecuteWrite(ctx, query, params)
}

// GetRelation retrieves a relation by ID
func (kg *Neo4jKnowledgeGraph) GetRelation(ctx context.Context, id string) (*Relation, error) {
	query := `
		MATCH (from:Entity)-[r:RELATES {id: $id}]->(to:Entity)
		RETURN r.id AS id, from.id AS fromId, to.id AS toId, r.type AS type,
			   r.properties AS properties, r.weight AS weight,
			   r.createdAt AS createdAt, r.updatedAt AS updatedAt
	`

	results, err := kg.driver.ExecuteQuery(ctx, query, map[string]interface{}{"id": id})
	if err != nil {
		return nil, fmt.Errorf("failed to get relation: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return kg.recordToRelation(results[0])
}

// DeleteRelation removes a relationship
func (kg *Neo4jKnowledgeGraph) DeleteRelation(ctx context.Context, id string) error {
	query := `
		MATCH ()-[r:RELATES {id: $id}]->()
		DELETE r
	`
	return kg.driver.ExecuteWrite(ctx, query, map[string]interface{}{"id": id})
}

// Query executes a graph query
func (kg *Neo4jKnowledgeGraph) Query(ctx context.Context, query *GraphQuery) (*GraphQueryResult, error) {
	var cypher string
	params := make(map[string]interface{})

	switch {
	case query.StartEntityID != "":
		// Traverse from a specific entity
		cypher = kg.buildTraversalQuery(query)
		params["startId"] = query.StartEntityID
		params["maxDepth"] = query.MaxDepth
		if query.MaxDepth == 0 {
			params["maxDepth"] = 3
		}

	case len(query.EntityTypes) > 0:
		// Query by entity types
		cypher = `
			MATCH (e:Entity)
			WHERE e.type IN $types
			RETURN e.id AS id, e.name AS name, e.type AS type, e.description AS description,
				   e.properties AS properties, e.embedding AS embedding
			LIMIT $limit
		`
		params["types"] = entityTypesToStrings(query.EntityTypes)
		params["limit"] = 100

	case len(query.RelationTypes) > 0:
		// Query by relation types
		cypher = `
			MATCH (from:Entity)-[r:RELATES]->(to:Entity)
			WHERE r.type IN $types
			RETURN from.id AS fromId, to.id AS toId, r.id AS relId, r.type AS relType,
				   from.name AS fromName, to.name AS toName
			LIMIT $limit
		`
		params["types"] = relationTypesToStrings(query.RelationTypes)
		params["limit"] = 100

	default:
		return nil, errors.New("invalid query: must specify startEntityID, entityTypes, or relationTypes")
	}

	results, err := kg.driver.ExecuteQuery(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return kg.processQueryResults(ctx, results, query)
}

// FindSimilarEntities finds entities similar to a query using embeddings
func (kg *Neo4jKnowledgeGraph) FindSimilarEntities(ctx context.Context, query string, limit int, threshold float64) ([]*Entity, error) {
	if kg.embedder == nil {
		return nil, errors.New("embedder not configured")
	}

	queryEmbedding, err := kg.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Fetch all entities with embeddings and compute similarity in Go
	// (Neo4j native vector search requires specific plugins)
	cypher := `
		MATCH (e:Entity)
		WHERE e.embedding IS NOT NULL AND e.embedding <> '[]'
		RETURN e.id AS id, e.name AS name, e.type AS type, e.description AS description,
			   e.properties AS properties, e.embedding AS embedding
	`

	results, err := kg.driver.ExecuteQuery(ctx, cypher, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch entities: %w", err)
	}

	type scoredEntity struct {
		entity *Entity
		score  float64
	}

	var scored []scoredEntity
	for _, record := range results {
		entity, err := kg.recordToEntity(record)
		if err != nil || len(entity.Embedding) == 0 {
			continue
		}

		score := cosineSimilarity(queryEmbedding, entity.Embedding)
		if score >= threshold {
			scored = append(scored, scoredEntity{entity: entity, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top N
	var entities []*Entity
	for i := 0; i < len(scored) && i < limit; i++ {
		entities = append(entities, scored[i].entity)
	}

	return entities, nil
}

// GetTriples returns all triples (subject, predicate, object) in the graph
func (kg *Neo4jKnowledgeGraph) GetTriples(ctx context.Context, limit int) ([]*Triple, error) {
	if limit == 0 {
		limit = 1000
	}

	query := `
		MATCH (from:Entity)-[r:RELATES]->(to:Entity)
		RETURN from.id AS subjectId, from.name AS subjectName,
			   r.type AS predicate,
			   to.id AS objectId, to.name AS objectName
		LIMIT $limit
	`

	results, err := kg.driver.ExecuteQuery(ctx, query, map[string]interface{}{"limit": limit})
	if err != nil {
		return nil, fmt.Errorf("failed to get triples: %w", err)
	}

	var triples []*Triple
	for _, record := range results {
		triple := &Triple{
			Subject:   &Entity{Name: getStringValue(record, "subjectName"), ID: getStringValue(record, "subjectId")},
			Predicate: RelationType(getStringValue(record, "predicate")),
			Object:    &Entity{Name: getStringValue(record, "objectName"), ID: getStringValue(record, "objectId")},
		}
		triples = append(triples, triple)
	}

	return triples, nil
}

// Close closes the Neo4j connection
func (kg *Neo4jKnowledgeGraph) Close() error {
	return kg.driver.Close()
}

// Helper methods

func (kg *Neo4jKnowledgeGraph) buildTraversalQuery(query *GraphQuery) string {
	var relTypeFilter string
	if len(query.RelationTypes) > 0 {
		types := relationTypesToStrings(query.RelationTypes)
		relTypeFilter = fmt.Sprintf("AND type(r) IN %v", types)
	}

	return fmt.Sprintf(`
		MATCH path = (start:Entity {id: $startId})-[r:RELATES*1..%d]-(connected:Entity)
		WHERE true %s
		UNWIND nodes(path) AS n
		WITH DISTINCT n
		RETURN n.id AS id, n.name AS name, n.type AS type, n.description AS description,
			   n.properties AS properties, n.embedding AS embedding
		LIMIT 100
	`, query.MaxDepth, relTypeFilter)
}

func (kg *Neo4jKnowledgeGraph) processQueryResults(ctx context.Context, results []map[string]interface{}, query *GraphQuery) (*GraphQueryResult, error) {
	result := &GraphQueryResult{
		Entities:  make([]*Entity, 0),
		Relations: make([]*Relation, 0),
	}

	seenEntities := make(map[string]bool)

	for _, record := range results {
		// Check if this is an entity result
		if id := getStringValue(record, "id"); id != "" && !seenEntities[id] {
			entity, err := kg.recordToEntity(record)
			if err == nil {
				result.Entities = append(result.Entities, entity)
				seenEntities[id] = true
			}
		}

		// Check if this is a relation result
		if relId := getStringValue(record, "relId"); relId != "" {
			relation, err := kg.recordToRelation(record)
			if err == nil {
				result.Relations = append(result.Relations, relation)
			}
		}
	}

	return result, nil
}

func (kg *Neo4jKnowledgeGraph) recordToEntity(record map[string]interface{}) (*Entity, error) {
	entity := &Entity{
		ID:          getStringValue(record, "id"),
		Name:        getStringValue(record, "name"),
		Type:        EntityType(getStringValue(record, "type")),
		Description: getStringValue(record, "description"),
	}

	// Parse properties
	if propsStr := getStringValue(record, "properties"); propsStr != "" {
		var props map[string]interface{}
		if err := json.Unmarshal([]byte(propsStr), &props); err == nil {
			entity.Properties = props
		}
	}

	// Parse embedding
	if embStr := getStringValue(record, "embedding"); embStr != "" {
		var embedding []float32
		if err := json.Unmarshal([]byte(embStr), &embedding); err == nil {
			entity.Embedding = embedding
		}
	}

	return entity, nil
}

func (kg *Neo4jKnowledgeGraph) recordToRelation(record map[string]interface{}) (*Relation, error) {
	relation := &Relation{
		ID:     getStringValue(record, "id"),
		FromID: getStringValue(record, "fromId"),
		ToID:   getStringValue(record, "toId"),
		Type:   RelationType(getStringValue(record, "type")),
		Weight: getFloatValue(record, "weight"),
	}

	// Parse properties
	if propsStr := getStringValue(record, "properties"); propsStr != "" {
		var props map[string]interface{}
		if err := json.Unmarshal([]byte(propsStr), &props); err == nil {
			relation.Properties = props
		}
	}

	return relation, nil
}

func getStringValue(record map[string]interface{}, key string) string {
	if val, ok := record[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloatValue(record map[string]interface{}, key string) float64 {
	if val, ok := record[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0
}

func entityTypesToStrings(types []EntityType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
}

func relationTypesToStrings(types []RelationType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
}

// Neo4jMemoryStore implements MemoryStore using Neo4j
type Neo4jMemoryStore struct {
	driver   Neo4jDriver
	database string
}

// NewNeo4jMemoryStore creates a new Neo4j-backed memory store
func NewNeo4jMemoryStore(driver Neo4jDriver, database string) *Neo4jMemoryStore {
	return &Neo4jMemoryStore{
		driver:   driver,
		database: database,
	}
}

// Initialize creates necessary indexes
func (s *Neo4jMemoryStore) Initialize(ctx context.Context) error {
	constraints := []string{
		"CREATE CONSTRAINT memory_id IF NOT EXISTS FOR (m:Memory) REQUIRE m.id IS UNIQUE",
		"CREATE INDEX memory_agent IF NOT EXISTS FOR (m:Memory) ON (m.agentId)",
		"CREATE INDEX memory_type IF NOT EXISTS FOR (m:Memory) ON (m.type)",
	}

	for _, constraint := range constraints {
		if err := s.driver.ExecuteWrite(ctx, constraint, nil); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}

	return nil
}

// Store stores a memory entry
func (s *Neo4jMemoryStore) Store(ctx context.Context, memory *MemoryEntry) error {
	embeddingJSON, _ := json.Marshal(memory.Embedding)
	metadataJSON, _ := json.Marshal(memory.Metadata)
	associationsJSON, _ := json.Marshal(memory.Associations)

	query := `
		MERGE (m:Memory {id: $id})
		SET m.agentId = $agentId,
			m.type = $type,
			m.content = $content,
			m.embedding = $embedding,
			m.importance = $importance,
			m.accessCount = $accessCount,
			m.lastAccess = datetime($lastAccess),
			m.createdAt = datetime($createdAt),
			m.metadata = $metadata,
			m.associations = $associations
		RETURN m
	`

	params := map[string]interface{}{
		"id":           memory.ID,
		"agentId":      memory.AgentID,
		"type":         string(memory.Type),
		"content":      memory.Content,
		"embedding":    string(embeddingJSON),
		"importance":   memory.Importance,
		"accessCount":  memory.AccessCount,
		"lastAccess":   memory.LastAccess.Format(time.RFC3339),
		"createdAt":    memory.CreatedAt.Format(time.RFC3339),
		"metadata":     string(metadataJSON),
		"associations": string(associationsJSON),
	}

	return s.driver.ExecuteWrite(ctx, query, params)
}

// Get retrieves a memory entry by ID
func (s *Neo4jMemoryStore) Get(ctx context.Context, id string) (*MemoryEntry, error) {
	query := `
		MATCH (m:Memory {id: $id})
		RETURN m
	`

	results, err := s.driver.ExecuteQuery(ctx, query, map[string]interface{}{"id": id})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	return s.recordToMemory(results[0])
}

// Search searches for memories by agent ID
func (s *Neo4jMemoryStore) Search(ctx context.Context, agentID string, query MemoryQuery) ([]*MemoryEntry, error) {
	cypher := `
		MATCH (m:Memory {agentId: $agentId})
		WHERE ($types IS NULL OR m.type IN $types)
		  AND ($minImportance IS NULL OR m.importance >= $minImportance)
		RETURN m
		ORDER BY m.importance DESC, m.lastAccess DESC
		LIMIT $limit
	`

	var memTypes interface{}
	if len(query.Types) > 0 {
		typeStrs := make([]string, len(query.Types))
		for i, t := range query.Types {
			typeStrs[i] = string(t)
		}
		memTypes = typeStrs
	}

	limit := query.Limit
	if limit == 0 {
		limit = 100
	}

	params := map[string]interface{}{
		"agentId":       agentID,
		"types":         memTypes,
		"minImportance": query.MinImportance,
		"limit":         limit,
	}

	results, err := s.driver.ExecuteQuery(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var memories []*MemoryEntry
	for _, record := range results {
		if mem, err := s.recordToMemory(record); err == nil {
			memories = append(memories, mem)
		}
	}

	return memories, nil
}

// Delete removes a memory entry
func (s *Neo4jMemoryStore) Delete(ctx context.Context, id string) error {
	query := `
		MATCH (m:Memory {id: $id})
		DELETE m
	`
	return s.driver.ExecuteWrite(ctx, query, map[string]interface{}{"id": id})
}

// DeleteByAgent removes all memories for an agent
func (s *Neo4jMemoryStore) DeleteByAgent(ctx context.Context, agentID string) error {
	query := `
		MATCH (m:Memory {agentId: $agentId})
		DELETE m
	`
	return s.driver.ExecuteWrite(ctx, query, map[string]interface{}{"agentId": agentID})
}

// ListByAgent lists all memories for an agent
func (s *Neo4jMemoryStore) ListByAgent(ctx context.Context, agentID string) ([]*MemoryEntry, error) {
	return s.Search(ctx, agentID, MemoryQuery{Limit: 1000})
}

// Update updates access statistics for a memory
func (s *Neo4jMemoryStore) Update(ctx context.Context, memory *MemoryEntry) error {
	return s.Store(ctx, memory)
}

func (s *Neo4jMemoryStore) recordToMemory(record map[string]interface{}) (*MemoryEntry, error) {
	// Extract from nested node if present
	node := record
	if m, ok := record["m"].(map[string]interface{}); ok {
		node = m
	}

	memory := &MemoryEntry{
		ID:          getStringValue(node, "id"),
		AgentID:     getStringValue(node, "agentId"),
		Type:        MemoryType(getStringValue(node, "type")),
		Content:     getStringValue(node, "content"),
		Importance:  getFloatValue(node, "importance"),
		AccessCount: int(getFloatValue(node, "accessCount")),
	}

	// Parse embedding
	if embStr := getStringValue(node, "embedding"); embStr != "" {
		var embedding []float32
		json.Unmarshal([]byte(embStr), &embedding)
		memory.Embedding = embedding
	}

	// Parse metadata
	if metaStr := getStringValue(node, "metadata"); metaStr != "" {
		var metadata map[string]interface{}
		json.Unmarshal([]byte(metaStr), &metadata)
		memory.Metadata = metadata
	}

	// Parse associations
	if assocStr := getStringValue(node, "associations"); assocStr != "" {
		var associations []string
		json.Unmarshal([]byte(assocStr), &associations)
		memory.Associations = associations
	}

	return memory, nil
}

// Close closes the connection
func (s *Neo4jMemoryStore) Close() error {
	return s.driver.Close()
}
