package memory

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EntityType represents the type of entity in the knowledge graph
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganization EntityType = "organization"
	EntityTypeConcept      EntityType = "concept"
	EntityTypeEvent        EntityType = "event"
	EntityTypeLocation     EntityType = "location"
	EntityTypeTool         EntityType = "tool"
	EntityTypeTask         EntityType = "task"
	EntityTypeDocument     EntityType = "document"
	EntityTypeCustom       EntityType = "custom"
)

// RelationType represents the type of relationship
type RelationType string

const (
	RelationTypeRelatedTo    RelationType = "related_to"
	RelationTypePartOf       RelationType = "part_of"
	RelationTypeCausedBy     RelationType = "caused_by"
	RelationTypeUsedBy       RelationType = "used_by"
	RelationTypeBelongsTo    RelationType = "belongs_to"
	RelationTypeDependsOn    RelationType = "depends_on"
	RelationTypeCreatedBy    RelationType = "created_by"
	RelationTypeMentions     RelationType = "mentions"
	RelationTypeSimilarTo    RelationType = "similar_to"
	RelationTypeInstanceOf   RelationType = "instance_of"
	RelationTypeHasProperty  RelationType = "has_property"
	RelationTypeCustom       RelationType = "custom"
)

// Entity represents a node in the knowledge graph
type Entity struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        EntityType             `json:"type"`
	Description string                 `json:"description,omitempty"`
	Embedding   []float32              `json:"embedding,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Source      string                 `json:"source,omitempty"` // Where this entity was extracted from
}

// Relation represents an edge in the knowledge graph
type Relation struct {
	ID         string                 `json:"id"`
	FromID     string                 `json:"from_id"`
	ToID       string                 `json:"to_id"`
	Type       RelationType           `json:"type"`
	Weight     float64                `json:"weight"`     // Strength of relationship (0-1)
	Confidence float64                `json:"confidence"` // Confidence in this relation (0-1)
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Source     string                 `json:"source,omitempty"`
}

// Triple represents a subject-predicate-object triple
type Triple struct {
	Subject   *Entity      `json:"subject"`
	Predicate RelationType `json:"predicate"`
	Object    *Entity      `json:"object"`
	Relation  *Relation    `json:"relation,omitempty"`
}

// KnowledgeGraph manages a knowledge graph
type KnowledgeGraph struct {
	entities  map[string]*Entity
	relations map[string]*Relation
	// Indexes for fast lookup
	outgoing  map[string][]string // entityID -> []relationID (outgoing)
	incoming  map[string][]string // entityID -> []relationID (incoming)
	byType    map[EntityType][]string
	byName    map[string][]string // name -> []entityID
	embedder  Embedder
	mu        sync.RWMutex
}

// NewKnowledgeGraph creates a new knowledge graph
func NewKnowledgeGraph(embedder Embedder) *KnowledgeGraph {
	return &KnowledgeGraph{
		entities:  make(map[string]*Entity),
		relations: make(map[string]*Relation),
		outgoing:  make(map[string][]string),
		incoming:  make(map[string][]string),
		byType:    make(map[EntityType][]string),
		byName:    make(map[string][]string),
		embedder:  embedder,
	}
}

// AddEntity adds an entity to the graph
func (kg *KnowledgeGraph) AddEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now()
	}
	entity.UpdatedAt = time.Now()

	// Generate embedding if not provided
	if len(entity.Embedding) == 0 && kg.embedder != nil {
		text := entity.Name
		if entity.Description != "" {
			text += " " + entity.Description
		}
		embedding, err := kg.embedder.Embed(ctx, text)
		if err == nil {
			entity.Embedding = embedding
		}
	}

	kg.mu.Lock()
	defer kg.mu.Unlock()

	kg.entities[entity.ID] = entity
	kg.byType[entity.Type] = append(kg.byType[entity.Type], entity.ID)
	kg.byName[entity.Name] = append(kg.byName[entity.Name], entity.ID)

	return nil
}

// GetEntity retrieves an entity by ID
func (kg *KnowledgeGraph) GetEntity(ctx context.Context, id string) (*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	entity, ok := kg.entities[id]
	if !ok {
		return nil, errors.New("entity not found")
	}
	return entity, nil
}

// UpdateEntity updates an entity
func (kg *KnowledgeGraph) UpdateEntity(ctx context.Context, entity *Entity) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if _, ok := kg.entities[entity.ID]; !ok {
		return errors.New("entity not found")
	}

	entity.UpdatedAt = time.Now()
	kg.entities[entity.ID] = entity
	return nil
}

// DeleteEntity deletes an entity and its relations
func (kg *KnowledgeGraph) DeleteEntity(ctx context.Context, id string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	entity, ok := kg.entities[id]
	if !ok {
		return errors.New("entity not found")
	}

	// Delete related relations
	for _, relID := range kg.outgoing[id] {
		delete(kg.relations, relID)
	}
	for _, relID := range kg.incoming[id] {
		delete(kg.relations, relID)
	}
	delete(kg.outgoing, id)
	delete(kg.incoming, id)

	// Remove from indexes
	kg.byType[entity.Type] = removeString(kg.byType[entity.Type], id)
	kg.byName[entity.Name] = removeString(kg.byName[entity.Name], id)

	delete(kg.entities, id)
	return nil
}

// AddRelation adds a relation between entities
func (kg *KnowledgeGraph) AddRelation(ctx context.Context, relation *Relation) error {
	if relation.ID == "" {
		relation.ID = uuid.New().String()
	}
	if relation.CreatedAt.IsZero() {
		relation.CreatedAt = time.Now()
	}
	relation.UpdatedAt = time.Now()
	if relation.Weight == 0 {
		relation.Weight = 1.0
	}
	if relation.Confidence == 0 {
		relation.Confidence = 1.0
	}

	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Verify entities exist
	if _, ok := kg.entities[relation.FromID]; !ok {
		return errors.New("from entity not found")
	}
	if _, ok := kg.entities[relation.ToID]; !ok {
		return errors.New("to entity not found")
	}

	kg.relations[relation.ID] = relation
	kg.outgoing[relation.FromID] = append(kg.outgoing[relation.FromID], relation.ID)
	kg.incoming[relation.ToID] = append(kg.incoming[relation.ToID], relation.ID)

	return nil
}

// GetRelation retrieves a relation by ID
func (kg *KnowledgeGraph) GetRelation(ctx context.Context, id string) (*Relation, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	relation, ok := kg.relations[id]
	if !ok {
		return nil, errors.New("relation not found")
	}
	return relation, nil
}

// DeleteRelation deletes a relation
func (kg *KnowledgeGraph) DeleteRelation(ctx context.Context, id string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	relation, ok := kg.relations[id]
	if !ok {
		return errors.New("relation not found")
	}

	kg.outgoing[relation.FromID] = removeString(kg.outgoing[relation.FromID], id)
	kg.incoming[relation.ToID] = removeString(kg.incoming[relation.ToID], id)
	delete(kg.relations, id)

	return nil
}

// GetEntitiesByType gets all entities of a type
func (kg *KnowledgeGraph) GetEntitiesByType(ctx context.Context, entityType EntityType) ([]*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var results []*Entity
	for _, id := range kg.byType[entityType] {
		if entity, ok := kg.entities[id]; ok {
			results = append(results, entity)
		}
	}
	return results, nil
}

// GetEntitiesByName gets entities by name (supports partial match)
func (kg *KnowledgeGraph) GetEntitiesByName(ctx context.Context, name string) ([]*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var results []*Entity
	for _, id := range kg.byName[name] {
		if entity, ok := kg.entities[id]; ok {
			results = append(results, entity)
		}
	}
	return results, nil
}

// GetRelatedEntities gets entities related to a given entity
func (kg *KnowledgeGraph) GetRelatedEntities(ctx context.Context, entityID string, relationType RelationType, direction string) ([]*Entity, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var relIDs []string
	if direction == "outgoing" || direction == "both" {
		relIDs = append(relIDs, kg.outgoing[entityID]...)
	}
	if direction == "incoming" || direction == "both" {
		relIDs = append(relIDs, kg.incoming[entityID]...)
	}

	var results []*Entity
	seen := make(map[string]bool)

	for _, relID := range relIDs {
		rel := kg.relations[relID]
		if relationType != "" && rel.Type != relationType {
			continue
		}

		var targetID string
		if rel.FromID == entityID {
			targetID = rel.ToID
		} else {
			targetID = rel.FromID
		}

		if !seen[targetID] {
			if entity, ok := kg.entities[targetID]; ok {
				results = append(results, entity)
				seen[targetID] = true
			}
		}
	}

	return results, nil
}

// GetRelationsBetween gets relations between two entities
func (kg *KnowledgeGraph) GetRelationsBetween(ctx context.Context, fromID, toID string) ([]*Relation, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var results []*Relation
	for _, relID := range kg.outgoing[fromID] {
		rel := kg.relations[relID]
		if rel.ToID == toID {
			results = append(results, rel)
		}
	}

	return results, nil
}

// GetTriples returns all triples (subject-predicate-object)
func (kg *KnowledgeGraph) GetTriples(ctx context.Context, limit int) ([]*Triple, error) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	var triples []*Triple
	for _, rel := range kg.relations {
		subject := kg.entities[rel.FromID]
		object := kg.entities[rel.ToID]
		if subject != nil && object != nil {
			triples = append(triples, &Triple{
				Subject:   subject,
				Predicate: rel.Type,
				Object:    object,
				Relation:  rel,
			})
		}
		if limit > 0 && len(triples) >= limit {
			break
		}
	}

	return triples, nil
}

// Query performs a graph query
type GraphQuery struct {
	StartEntityID  string
	EntityTypes    []EntityType
	RelationTypes  []RelationType
	MaxDepth       int
	MinConfidence  float64
	IncludeWeights bool
}

// GraphQueryResult represents the result of a graph query
type GraphQueryResult struct {
	Entities  []*Entity   `json:"entities"`
	Relations []*Relation `json:"relations"`
	Paths     [][]string  `json:"paths"` // Paths of entity IDs
}

// Query executes a graph query (BFS traversal)
func (kg *KnowledgeGraph) Query(ctx context.Context, query *GraphQuery) (*GraphQueryResult, error) {
	if query.MaxDepth == 0 {
		query.MaxDepth = 3
	}

	kg.mu.RLock()
	defer kg.mu.RUnlock()

	result := &GraphQueryResult{
		Entities:  make([]*Entity, 0),
		Relations: make([]*Relation, 0),
		Paths:     make([][]string, 0),
	}

	if query.StartEntityID == "" {
		return result, nil
	}

	// BFS traversal
	visited := make(map[string]bool)
	visitedRels := make(map[string]bool)
	queue := [][]string{{query.StartEntityID}} // Queue of paths

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		currentID := path[len(path)-1]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		entity, ok := kg.entities[currentID]
		if !ok {
			continue
		}

		// Check entity type filter
		if len(query.EntityTypes) > 0 {
			found := false
			for _, t := range query.EntityTypes {
				if entity.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		result.Entities = append(result.Entities, entity)
		if len(path) > 1 {
			result.Paths = append(result.Paths, path)
		}

		// Continue traversal if not at max depth
		if len(path) < query.MaxDepth {
			// Traverse outgoing relations
			for _, relID := range kg.outgoing[currentID] {
				rel := kg.relations[relID]
				if !kg.relationMatchesQuery(rel, query) {
					continue
				}
				if !visitedRels[relID] {
					result.Relations = append(result.Relations, rel)
					visitedRels[relID] = true
				}
				if !visited[rel.ToID] {
					newPath := make([]string, len(path)+1)
					copy(newPath, path)
					newPath[len(path)] = rel.ToID
					queue = append(queue, newPath)
				}
			}

			// Traverse incoming relations
			for _, relID := range kg.incoming[currentID] {
				rel := kg.relations[relID]
				if !kg.relationMatchesQuery(rel, query) {
					continue
				}
				if !visitedRels[relID] {
					result.Relations = append(result.Relations, rel)
					visitedRels[relID] = true
				}
				if !visited[rel.FromID] {
					newPath := make([]string, len(path)+1)
					copy(newPath, path)
					newPath[len(path)] = rel.FromID
					queue = append(queue, newPath)
				}
			}
		}
	}

	return result, nil
}

// relationMatchesQuery checks if a relation matches query filters
func (kg *KnowledgeGraph) relationMatchesQuery(rel *Relation, query *GraphQuery) bool {
	if rel.Confidence < query.MinConfidence {
		return false
	}
	if len(query.RelationTypes) > 0 {
		found := false
		for _, t := range query.RelationTypes {
			if rel.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// FindSimilarEntities finds entities similar to a query
func (kg *KnowledgeGraph) FindSimilarEntities(ctx context.Context, query string, limit int, threshold float64) ([]*Entity, error) {
	if kg.embedder == nil {
		return nil, errors.New("embedder not configured")
	}

	embedding, err := kg.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	kg.mu.RLock()
	defer kg.mu.RUnlock()

	type scored struct {
		entity *Entity
		score  float64
	}
	var scored_entities []scored

	for _, entity := range kg.entities {
		if len(entity.Embedding) == 0 {
			continue
		}
		similarity := cosineSimilarity(embedding, entity.Embedding)
		if similarity >= threshold {
			scored_entities = append(scored_entities, scored{entity, similarity})
		}
	}

	sort.Slice(scored_entities, func(i, j int) bool {
		return scored_entities[i].score > scored_entities[j].score
	})

	var results []*Entity
	for i, s := range scored_entities {
		if i >= limit {
			break
		}
		results = append(results, s.entity)
	}

	return results, nil
}

// Stats returns statistics about the knowledge graph
type GraphStats struct {
	TotalEntities     int                   `json:"total_entities"`
	TotalRelations    int                   `json:"total_relations"`
	EntitiesByType    map[EntityType]int    `json:"entities_by_type"`
	RelationsByType   map[RelationType]int  `json:"relations_by_type"`
	AverageConnections float64              `json:"average_connections"`
}

// GetStats returns graph statistics
func (kg *KnowledgeGraph) GetStats(ctx context.Context) *GraphStats {
	kg.mu.RLock()
	defer kg.mu.RUnlock()

	stats := &GraphStats{
		TotalEntities:   len(kg.entities),
		TotalRelations:  len(kg.relations),
		EntitiesByType:  make(map[EntityType]int),
		RelationsByType: make(map[RelationType]int),
	}

	for _, entity := range kg.entities {
		stats.EntitiesByType[entity.Type]++
	}

	for _, rel := range kg.relations {
		stats.RelationsByType[rel.Type]++
	}

	if stats.TotalEntities > 0 {
		stats.AverageConnections = float64(stats.TotalRelations*2) / float64(stats.TotalEntities)
	}

	return stats
}

// ExtractEntities extracts entities from text (placeholder for NER)
func (kg *KnowledgeGraph) ExtractEntities(ctx context.Context, text string) ([]*Entity, error) {
	// This is a placeholder - in a real implementation, you'd use NER
	// For now, return empty list
	return []*Entity{}, nil
}

// ExtractRelations extracts relations from text (placeholder for relation extraction)
func (kg *KnowledgeGraph) ExtractRelations(ctx context.Context, text string, entities []*Entity) ([]*Relation, error) {
	// This is a placeholder - in a real implementation, you'd use relation extraction
	// For now, return empty list
	return []*Relation{}, nil
}

// Helper functions

func removeString(slice []string, item string) []string {
	for i, s := range slice {
		if s == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
