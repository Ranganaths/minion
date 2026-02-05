package memory

import (
	"context"
	"testing"
	"time"
)

// MockEmbedder for testing
type MockEmbedder struct {
	embedding []float32
}

func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
	}
}

func (e *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Generate slightly different embeddings based on text length
	result := make([]float32, len(e.embedding))
	copy(result, e.embedding)
	for i := range result {
		result[i] += float32(len(text)%10) * 0.01
	}
	return result, nil
}

func (e *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, _ := e.Embed(ctx, text)
		results[i] = embedding
	}
	return results, nil
}

// Test Semantic Memory
func TestSemanticMemoryRemember(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	ctx := context.Background()
	mem, err := sm.Remember(ctx, "agent-1", "The capital of France is Paris", MemoryTypeSemantic, 0.8, nil)

	if err != nil {
		t.Fatalf("Remember failed: %v", err)
	}

	if mem.ID == "" {
		t.Error("Expected memory ID to be set")
	}

	if mem.Content != "The capital of France is Paris" {
		t.Error("Content mismatch")
	}

	if len(mem.Embedding) == 0 {
		t.Error("Expected embedding to be generated")
	}
}

func TestSemanticMemoryRecall(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	ctx := context.Background()

	// Store some memories
	sm.Remember(ctx, "agent-1", "Paris is the capital of France", MemoryTypeSemantic, 0.8, nil)
	sm.Remember(ctx, "agent-1", "London is the capital of UK", MemoryTypeSemantic, 0.7, nil)
	sm.Remember(ctx, "agent-1", "Berlin is the capital of Germany", MemoryTypeSemantic, 0.6, nil)

	// Recall memories
	results, err := sm.Recall(ctx, "agent-1", "What is the capital of France?", 5)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result")
	}

	// Check that access count was updated
	if results[0].Memory.AccessCount == 0 {
		t.Error("Expected access count to be incremented")
	}
}

func TestSemanticMemoryForget(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	ctx := context.Background()

	mem, _ := sm.Remember(ctx, "agent-1", "Test memory", MemoryTypeSemantic, 0.5, nil)
	err := sm.Forget(ctx, mem.ID)
	if err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	_, err = store.Get(ctx, mem.ID)
	if err == nil {
		t.Error("Memory should have been deleted")
	}
}

func TestSemanticMemoryAssociate(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	ctx := context.Background()

	mem1, _ := sm.Remember(ctx, "agent-1", "Memory 1", MemoryTypeSemantic, 0.5, nil)
	mem2, _ := sm.Remember(ctx, "agent-1", "Memory 2", MemoryTypeSemantic, 0.5, nil)

	err := sm.Associate(ctx, mem1.ID, mem2.ID)
	if err != nil {
		t.Fatalf("Associate failed: %v", err)
	}

	// Check associations
	updated1, _ := store.Get(ctx, mem1.ID)
	updated2, _ := store.Get(ctx, mem2.ID)

	found1 := false
	for _, assoc := range updated1.Associations {
		if assoc == mem2.ID {
			found1 = true
			break
		}
	}
	if !found1 {
		t.Error("mem1 should have association to mem2")
	}

	found2 := false
	for _, assoc := range updated2.Associations {
		if assoc == mem1.ID {
			found2 = true
			break
		}
	}
	if !found2 {
		t.Error("mem2 should have association to mem1")
	}
}

func TestSemanticMemoryDecay(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:         store,
		Embedder:      embedder,
		DecayRate:     1.0, // Fast decay for testing
		MinImportance: 0.5,
	})

	ctx := context.Background()

	// Create memory with low importance
	mem, _ := sm.Remember(ctx, "agent-1", "Low importance memory", MemoryTypeSemantic, 0.3, nil)

	// Set last access to past
	mem.LastAccess = time.Now().Add(-24 * time.Hour)
	store.Update(ctx, mem)

	// Apply decay
	forgotten, err := sm.Decay(ctx, "agent-1")
	if err != nil {
		t.Fatalf("Decay failed: %v", err)
	}

	if forgotten == 0 {
		t.Error("Expected memory to be forgotten due to decay")
	}
}

func TestWorkingMemoryExpiration(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	ctx := context.Background()

	// Working memory should have expiry
	mem, _ := sm.Remember(ctx, "agent-1", "Working memory", MemoryTypeWorking, 0.5, nil)
	if mem.ExpiresAt == nil {
		t.Error("Working memory should have expiry time")
	}
}

// Test Knowledge Graph
func TestKnowledgeGraphAddEntity(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()
	entity := &Entity{
		Name:        "Paris",
		Type:        EntityTypeLocation,
		Description: "Capital of France",
	}

	err := kg.AddEntity(ctx, entity)
	if err != nil {
		t.Fatalf("AddEntity failed: %v", err)
	}

	if entity.ID == "" {
		t.Error("Entity ID should be set")
	}

	if len(entity.Embedding) == 0 {
		t.Error("Entity embedding should be generated")
	}
}

func TestKnowledgeGraphAddRelation(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities
	paris := &Entity{Name: "Paris", Type: EntityTypeLocation}
	france := &Entity{Name: "France", Type: EntityTypeLocation}
	kg.AddEntity(ctx, paris)
	kg.AddEntity(ctx, france)

	// Add relation
	relation := &Relation{
		FromID: paris.ID,
		ToID:   france.ID,
		Type:   RelationTypePartOf,
	}

	err := kg.AddRelation(ctx, relation)
	if err != nil {
		t.Fatalf("AddRelation failed: %v", err)
	}

	if relation.ID == "" {
		t.Error("Relation ID should be set")
	}
}

func TestKnowledgeGraphGetRelatedEntities(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Create entities
	paris := &Entity{Name: "Paris", Type: EntityTypeLocation}
	france := &Entity{Name: "France", Type: EntityTypeLocation}
	europe := &Entity{Name: "Europe", Type: EntityTypeLocation}
	kg.AddEntity(ctx, paris)
	kg.AddEntity(ctx, france)
	kg.AddEntity(ctx, europe)

	// Create relations
	kg.AddRelation(ctx, &Relation{FromID: paris.ID, ToID: france.ID, Type: RelationTypePartOf})
	kg.AddRelation(ctx, &Relation{FromID: france.ID, ToID: europe.ID, Type: RelationTypePartOf})

	// Get related entities
	related, err := kg.GetRelatedEntities(ctx, paris.ID, "", "outgoing")
	if err != nil {
		t.Fatalf("GetRelatedEntities failed: %v", err)
	}

	if len(related) != 1 {
		t.Errorf("Expected 1 related entity, got %d", len(related))
	}

	if related[0].Name != "France" {
		t.Errorf("Expected France, got %s", related[0].Name)
	}
}

func TestKnowledgeGraphQuery(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Create a small graph
	a := &Entity{Name: "A", Type: EntityTypeConcept}
	b := &Entity{Name: "B", Type: EntityTypeConcept}
	c := &Entity{Name: "C", Type: EntityTypeConcept}
	kg.AddEntity(ctx, a)
	kg.AddEntity(ctx, b)
	kg.AddEntity(ctx, c)

	kg.AddRelation(ctx, &Relation{FromID: a.ID, ToID: b.ID, Type: RelationTypeRelatedTo})
	kg.AddRelation(ctx, &Relation{FromID: b.ID, ToID: c.ID, Type: RelationTypeRelatedTo})

	// Query from A with depth 2
	result, err := kg.Query(ctx, &GraphQuery{
		StartEntityID: a.ID,
		MaxDepth:      3,
	})

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Entities) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(result.Entities))
	}
}

func TestKnowledgeGraphFindSimilar(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities
	kg.AddEntity(ctx, &Entity{Name: "Python programming", Type: EntityTypeConcept})
	kg.AddEntity(ctx, &Entity{Name: "Java programming", Type: EntityTypeConcept})
	kg.AddEntity(ctx, &Entity{Name: "Cooking recipes", Type: EntityTypeConcept})

	// Find similar
	similar, err := kg.FindSimilarEntities(ctx, "programming languages", 10, 0.0)
	if err != nil {
		t.Fatalf("FindSimilarEntities failed: %v", err)
	}

	if len(similar) == 0 {
		t.Error("Expected at least one similar entity")
	}
}

func TestKnowledgeGraphStats(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities and relations
	e1 := &Entity{Name: "E1", Type: EntityTypeConcept}
	e2 := &Entity{Name: "E2", Type: EntityTypeConcept}
	e3 := &Entity{Name: "E3", Type: EntityTypePerson}
	kg.AddEntity(ctx, e1)
	kg.AddEntity(ctx, e2)
	kg.AddEntity(ctx, e3)

	kg.AddRelation(ctx, &Relation{FromID: e1.ID, ToID: e2.ID, Type: RelationTypeRelatedTo})

	stats := kg.GetStats(ctx)

	if stats.TotalEntities != 3 {
		t.Errorf("Expected 3 entities, got %d", stats.TotalEntities)
	}

	if stats.TotalRelations != 1 {
		t.Errorf("Expected 1 relation, got %d", stats.TotalRelations)
	}

	if stats.EntitiesByType[EntityTypeConcept] != 2 {
		t.Errorf("Expected 2 concept entities, got %d", stats.EntitiesByType[EntityTypeConcept])
	}
}

func TestKnowledgeGraphTriples(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Create entities
	paris := &Entity{Name: "Paris", Type: EntityTypeLocation}
	france := &Entity{Name: "France", Type: EntityTypeLocation}
	kg.AddEntity(ctx, paris)
	kg.AddEntity(ctx, france)

	// Create relation
	kg.AddRelation(ctx, &Relation{
		FromID: paris.ID,
		ToID:   france.ID,
		Type:   RelationTypePartOf,
	})

	// Get triples
	triples, err := kg.GetTriples(ctx, 10)
	if err != nil {
		t.Fatalf("GetTriples failed: %v", err)
	}

	if len(triples) != 1 {
		t.Errorf("Expected 1 triple, got %d", len(triples))
	}

	if triples[0].Subject.Name != "Paris" {
		t.Error("Expected Paris as subject")
	}

	if triples[0].Predicate != RelationTypePartOf {
		t.Error("Expected part_of predicate")
	}

	if triples[0].Object.Name != "France" {
		t.Error("Expected France as object")
	}
}

// Test Context Manager
func TestContextManagerCreateWindow(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	window := cm.CreateWindow("agent-1", 4000)

	if window.ID == "" {
		t.Error("Window ID should be set")
	}

	if window.MaxTokens != 4000 {
		t.Errorf("Expected max tokens 4000, got %d", window.MaxTokens)
	}
}

func TestContextManagerAddItem(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	ctx := context.Background()
	window := cm.CreateWindow("agent-1", 1000)

	item := &ContextItem{
		Type:     ContextItemTypeMessage,
		Content:  "Hello, how can I help you today?",
		Priority: 1.0,
	}

	err := cm.AddItem(ctx, window.ID, item)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	if item.ID == "" {
		t.Error("Item ID should be set")
	}

	if item.Tokens == 0 {
		t.Error("Item tokens should be estimated")
	}

	if window.CurrentTokens != item.Tokens {
		t.Error("Window token count should be updated")
	}
}

func TestContextManagerEviction(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
		Summarizer:     NewMockSummarizer(),
	})

	ctx := context.Background()
	window := cm.CreateWindow("agent-1", 100) // Small window

	// Add items that will exceed capacity
	for i := 0; i < 10; i++ {
		item := &ContextItem{
			Type:     ContextItemTypeMessage,
			Content:  "This is a test message with some content",
			Priority: float64(i) / 10.0,
		}
		cm.AddItem(ctx, window.ID, item)
	}

	// Window should have evicted some items
	if window.CurrentTokens > window.MaxTokens {
		t.Error("Current tokens should not exceed max tokens")
	}
}

func TestContextManagerEnrich(t *testing.T) {
	store := NewInMemoryMemoryStore()
	embedder := NewMockEmbedder()

	sm := NewSemanticMemory(SemanticMemoryConfig{
		Store:    store,
		Embedder: embedder,
	})

	kg := NewKnowledgeGraph(embedder)

	cm := NewContextManager(ContextManagerConfig{
		SemanticMemory: sm,
		KnowledgeGraph: kg,
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	ctx := context.Background()

	// Store some memories
	sm.Remember(ctx, "agent-1", "Paris is the capital of France", MemoryTypeSemantic, 0.8, nil)

	// Add knowledge graph entity
	kg.AddEntity(ctx, &Entity{
		Name:        "Paris",
		Type:        EntityTypeLocation,
		Description: "City of lights",
	})

	// Create window and enrich
	window := cm.CreateWindow("agent-1", 4000)
	err := cm.EnrichContext(ctx, window.ID, "Tell me about Paris")

	if err != nil {
		t.Fatalf("EnrichContext failed: %v", err)
	}

	// Should have added relevant items
	if len(window.Items) == 0 {
		t.Error("Expected context to be enriched with relevant items")
	}
}

func TestContextManagerGetContent(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	ctx := context.Background()
	window := cm.CreateWindow("agent-1", 4000)

	cm.AddItem(ctx, window.ID, &ContextItem{
		Type:     ContextItemTypeMessage,
		Content:  "Message 1",
		Priority: 0.5,
	})
	cm.AddItem(ctx, window.ID, &ContextItem{
		Type:     ContextItemTypeMessage,
		Content:  "Message 2",
		Priority: 1.0,
	})

	content := cm.GetContextContent(window.ID)

	if content == "" {
		t.Error("Expected content to be returned")
	}

	// Higher priority item should come first
	if len(content) < 10 {
		t.Error("Expected content to include both messages")
	}
}

func TestContextManagerClearWindow(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	ctx := context.Background()
	window := cm.CreateWindow("agent-1", 4000)

	cm.AddItem(ctx, window.ID, &ContextItem{
		Type:    ContextItemTypeMessage,
		Content: "Test message",
	})

	cm.ClearWindow(window.ID)

	if len(window.Items) != 0 {
		t.Error("Window items should be cleared")
	}

	if window.CurrentTokens != 0 {
		t.Error("Current tokens should be zero")
	}
}

func TestContextManagerDeleteWindow(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	window := cm.CreateWindow("agent-1", 4000)
	windowID := window.ID

	cm.DeleteWindow(windowID)

	_, exists := cm.GetWindow(windowID)
	if exists {
		t.Error("Window should have been deleted")
	}
}

// Test Memory Store
func TestInMemoryStoreSearch(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ctx := context.Background()

	// Store memories
	mem1 := &MemoryEntry{
		ID:         "1",
		AgentID:    "agent-1",
		Type:       MemoryTypeSemantic,
		Content:    "Test 1",
		Embedding:  []float32{0.1, 0.2, 0.3},
		Importance: 0.8,
		LastAccess: time.Now(),
	}
	mem2 := &MemoryEntry{
		ID:         "2",
		AgentID:    "agent-1",
		Type:       MemoryTypeEpisodic,
		Content:    "Test 2",
		Embedding:  []float32{0.2, 0.3, 0.4},
		Importance: 0.5,
		LastAccess: time.Now(),
	}
	mem3 := &MemoryEntry{
		ID:         "3",
		AgentID:    "agent-2",
		Type:       MemoryTypeSemantic,
		Content:    "Test 3",
		Embedding:  []float32{0.3, 0.4, 0.5},
		Importance: 0.7,
		LastAccess: time.Now(),
	}

	store.Store(ctx, mem1)
	store.Store(ctx, mem2)
	store.Store(ctx, mem3)

	// Search by agent
	results, _ := store.Search(ctx, &MemoryQuery{
		AgentID: "agent-1",
	})
	if len(results) != 2 {
		t.Errorf("Expected 2 results for agent-1, got %d", len(results))
	}

	// Search by type
	results, _ = store.Search(ctx, &MemoryQuery{
		AgentID: "agent-1",
		Types:   []MemoryType{MemoryTypeSemantic},
	})
	if len(results) != 1 {
		t.Errorf("Expected 1 semantic memory, got %d", len(results))
	}

	// Search with embedding
	results, _ = store.Search(ctx, &MemoryQuery{
		AgentID:   "agent-1",
		Embedding: []float32{0.1, 0.2, 0.3},
		Limit:     2,
	})
	if len(results) > 2 {
		t.Error("Should respect limit")
	}
}

func TestDeleteExpiredMemories(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ctx := context.Background()

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	// Store expired and non-expired memories
	store.Store(ctx, &MemoryEntry{
		ID:        "expired",
		AgentID:   "agent-1",
		ExpiresAt: &past,
	})
	store.Store(ctx, &MemoryEntry{
		ID:        "not-expired",
		AgentID:   "agent-1",
		ExpiresAt: &future,
	})
	store.Store(ctx, &MemoryEntry{
		ID:        "no-expiry",
		AgentID:   "agent-1",
	})

	deleted, _ := store.DeleteExpired(ctx)

	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	// Check remaining
	memories, _ := store.GetByAgent(ctx, "agent-1", 100)
	if len(memories) != 2 {
		t.Errorf("Expected 2 memories remaining, got %d", len(memories))
	}
}

// Test Token Estimator
func TestSimpleTokenEstimator(t *testing.T) {
	estimator := NewSimpleTokenEstimator()

	// 100 characters should be roughly 25 tokens
	tokens := estimator.EstimateTokens("Hello, this is a test message that is approximately one hundred characters long for testing purposes.")
	if tokens < 20 || tokens > 30 {
		t.Errorf("Token estimate seems off: %d", tokens)
	}
}

// Test Cosine Similarity
func TestCosineSimilarity(t *testing.T) {
	// Identical vectors
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim < 0.99 {
		t.Errorf("Expected similarity ~1, got %f", sim)
	}

	// Orthogonal vectors
	c := []float32{1, 0, 0}
	d := []float32{0, 1, 0}
	sim = cosineSimilarity(c, d)
	if sim > 0.01 {
		t.Errorf("Expected similarity ~0, got %f", sim)
	}

	// Opposite vectors
	e := []float32{1, 0, 0}
	f := []float32{-1, 0, 0}
	sim = cosineSimilarity(e, f)
	if sim > -0.99 {
		t.Errorf("Expected similarity ~-1, got %f", sim)
	}
}

func TestKnowledgeGraphDeleteEntity(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities
	e1 := &Entity{Name: "E1", Type: EntityTypeConcept}
	e2 := &Entity{Name: "E2", Type: EntityTypeConcept}
	kg.AddEntity(ctx, e1)
	kg.AddEntity(ctx, e2)

	// Add relation
	kg.AddRelation(ctx, &Relation{FromID: e1.ID, ToID: e2.ID, Type: RelationTypeRelatedTo})

	// Delete entity
	err := kg.DeleteEntity(ctx, e1.ID)
	if err != nil {
		t.Fatalf("DeleteEntity failed: %v", err)
	}

	// Verify entity is deleted
	_, err = kg.GetEntity(ctx, e1.ID)
	if err == nil {
		t.Error("Entity should be deleted")
	}

	// Verify stats
	stats := kg.GetStats(ctx)
	if stats.TotalEntities != 1 {
		t.Errorf("Expected 1 entity after deletion, got %d", stats.TotalEntities)
	}
	if stats.TotalRelations != 0 {
		t.Errorf("Expected 0 relations after deletion, got %d", stats.TotalRelations)
	}
}

func TestKnowledgeGraphGetRelationsBetween(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities
	e1 := &Entity{Name: "E1", Type: EntityTypeConcept}
	e2 := &Entity{Name: "E2", Type: EntityTypeConcept}
	kg.AddEntity(ctx, e1)
	kg.AddEntity(ctx, e2)

	// Add multiple relations
	kg.AddRelation(ctx, &Relation{FromID: e1.ID, ToID: e2.ID, Type: RelationTypeRelatedTo})
	kg.AddRelation(ctx, &Relation{FromID: e1.ID, ToID: e2.ID, Type: RelationTypeDependsOn})

	// Get relations between
	relations, err := kg.GetRelationsBetween(ctx, e1.ID, e2.ID)
	if err != nil {
		t.Fatalf("GetRelationsBetween failed: %v", err)
	}

	if len(relations) != 2 {
		t.Errorf("Expected 2 relations, got %d", len(relations))
	}
}

func TestKnowledgeGraphGetEntitiesByType(t *testing.T) {
	embedder := NewMockEmbedder()
	kg := NewKnowledgeGraph(embedder)

	ctx := context.Background()

	// Add entities of different types
	kg.AddEntity(ctx, &Entity{Name: "Person1", Type: EntityTypePerson})
	kg.AddEntity(ctx, &Entity{Name: "Person2", Type: EntityTypePerson})
	kg.AddEntity(ctx, &Entity{Name: "Concept1", Type: EntityTypeConcept})

	// Get by type
	people, err := kg.GetEntitiesByType(ctx, EntityTypePerson)
	if err != nil {
		t.Fatalf("GetEntitiesByType failed: %v", err)
	}

	if len(people) != 2 {
		t.Errorf("Expected 2 people, got %d", len(people))
	}
}

func TestRemoveItem(t *testing.T) {
	cm := NewContextManager(ContextManagerConfig{
		TokenEstimator: NewSimpleTokenEstimator(),
	})

	ctx := context.Background()
	window := cm.CreateWindow("agent-1", 4000)

	item := &ContextItem{
		Type:    ContextItemTypeMessage,
		Content: "Test message",
	}
	cm.AddItem(ctx, window.ID, item)

	initialTokens := window.CurrentTokens

	cm.RemoveItem(window.ID, item.ID)

	if len(window.Items) != 0 {
		t.Error("Item should be removed")
	}

	if window.CurrentTokens != 0 {
		t.Errorf("Tokens should be decremented, but got %d (was %d)", window.CurrentTokens, initialTokens)
	}
}
