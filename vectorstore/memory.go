package vectorstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/embeddings"
	"github.com/Ranganaths/minion/metrics"
)

// MemoryVectorStore is an in-memory vector store implementation.
// MemoryVectorStore is safe for concurrent use by multiple goroutines.
type MemoryVectorStore struct {
	embedder       embeddings.Embedder
	documents      map[string]Document
	distanceMetric DistanceMetric
	mu             sync.RWMutex
	idCounter      int
	// Metrics
	docsTotal       metrics.Gauge
	searchesTotal   metrics.Counter
	searchDuration  metrics.Histogram
}

// MemoryVectorStoreConfig configures the in-memory vector store
type MemoryVectorStoreConfig struct {
	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// DistanceMetric is the distance metric to use (default: cosine)
	DistanceMetric DistanceMetric
}

// NewMemoryVectorStore creates a new in-memory vector store
func NewMemoryVectorStore(cfg MemoryVectorStoreConfig) (*MemoryVectorStore, error) {
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	metric := cfg.DistanceMetric
	if metric == "" {
		metric = DistanceCosine
	}

	m := metrics.GetMetrics()
	return &MemoryVectorStore{
		embedder:       cfg.Embedder,
		documents:      make(map[string]Document),
		distanceMetric: metric,
		docsTotal:      m.Gauge(metrics.MetricVectorStoreDocuments, nil),
		searchesTotal:  m.Counter(metrics.MetricVectorStoreSearches, nil),
		searchDuration: m.Histogram(metrics.MetricVectorStoreSearchDuration, nil),
	}, nil
}

// AddDocuments adds documents to the vector store.
// The input slice is not modified; documents are copied before storing.
func (vs *MemoryVectorStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Create a copy of documents to avoid modifying the input slice
	docsCopy := make([]Document, len(docs))
	for i, doc := range docs {
		docsCopy[i] = doc.Clone()
	}

	// Collect texts that need embedding
	var textsToEmbed []string
	var embedIndices []int

	for i, doc := range docsCopy {
		if len(doc.Embedding) == 0 {
			textsToEmbed = append(textsToEmbed, doc.PageContent)
			embedIndices = append(embedIndices, i)
		}
	}

	// Generate embeddings for texts that need them
	if len(textsToEmbed) > 0 {
		newEmbeddings, err := vs.embedder.EmbedDocuments(ctx, textsToEmbed)
		if err != nil {
			return nil, fmt.Errorf("failed to embed documents: %w", err)
		}

		for i, embedding := range newEmbeddings {
			docsCopy[embedIndices[i]].Embedding = embedding
		}
	}

	// Store documents
	vs.mu.Lock()
	defer vs.mu.Unlock()

	ids := make([]string, len(docsCopy))
	for i, doc := range docsCopy {
		id := doc.ID
		if id == "" {
			vs.idCounter++
			id = fmt.Sprintf("doc_%d", vs.idCounter)
		}
		doc.ID = id
		vs.documents[id] = doc
		ids[i] = id
	}

	// Update metrics
	vs.docsTotal.Set(float64(len(vs.documents)))

	return ids, nil
}

// SimilaritySearch finds the most similar documents to a query
func (vs *MemoryVectorStore) SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error) {
	results, err := vs.SimilaritySearchWithScore(ctx, query, k)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, result := range results {
		docs[i] = result.Document
	}
	return docs, nil
}

// SimilaritySearchWithScore returns documents with their similarity scores
func (vs *MemoryVectorStore) SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error) {
	start := time.Now()
	vs.searchesTotal.Inc()
	defer func() {
		vs.searchDuration.Observe(time.Since(start).Seconds())
	}()

	// Embed the query
	queryEmbedding, err := vs.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	return vs.similaritySearchByVector(queryEmbedding, k)
}

// SimilaritySearchByVector searches using a pre-computed embedding
func (vs *MemoryVectorStore) SimilaritySearchByVector(ctx context.Context, embedding []float32, k int) ([]Document, error) {
	results, err := vs.similaritySearchByVector(embedding, k)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, result := range results {
		docs[i] = result.Document
	}
	return docs, nil
}

// similaritySearchByVector performs the actual similarity search
func (vs *MemoryVectorStore) similaritySearchByVector(queryEmbedding []float32, k int) ([]SearchResult, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.documents) == 0 {
		return nil, nil
	}

	// Calculate similarity scores for all documents
	type scoredDoc struct {
		doc   Document
		score float32
	}
	var scored []scoredDoc

	for _, doc := range vs.documents {
		// Validate embedding dimensions match
		if len(doc.Embedding) != len(queryEmbedding) {
			continue // Skip documents with mismatched dimensions
		}
		score := vs.calculateSimilarity(queryEmbedding, doc.Embedding)
		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score (descending)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top k
	if k > len(scored) {
		k = len(scored)
	}

	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		results[i] = SearchResult{
			Document: scored[i].doc,
			Score:    scored[i].score,
		}
	}

	return results, nil
}

// MaxMarginalRelevanceSearch performs MMR search for diversity.
// Lambda should be between 0 and 1, where 1 means pure relevance and 0 means pure diversity.
func (vs *MemoryVectorStore) MaxMarginalRelevanceSearch(ctx context.Context, query string, k int, fetchK int, lambda float32) ([]Document, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k must be positive, got %d", k)
	}
	if lambda < 0 || lambda > 1 {
		return nil, fmt.Errorf("lambda must be between 0 and 1, got %f", lambda)
	}

	// Embed the query
	queryEmbedding, err := vs.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if fetchK < k {
		fetchK = k * 2
	}

	// Get initial candidates
	candidates, err := vs.similaritySearchByVector(queryEmbedding, fetchK)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Apply MMR algorithm
	selected := make([]Document, 0, k)
	selectedEmbeddings := make([][]float32, 0, k)
	remaining := make([]SearchResult, len(candidates))
	copy(remaining, candidates)

	for len(selected) < k && len(remaining) > 0 {
		var bestIdx int
		var bestScore float32 = -1000

		for i, candidate := range remaining {
			// Calculate relevance to query
			relevance := embeddings.CosineSimilarity(queryEmbedding, candidate.Document.Embedding)

			// Calculate max similarity to already selected documents
			var maxSimilarity float32 = 0
			for _, selEmb := range selectedEmbeddings {
				sim := embeddings.CosineSimilarity(candidate.Document.Embedding, selEmb)
				if sim > maxSimilarity {
					maxSimilarity = sim
				}
			}

			// MMR score: lambda * relevance - (1 - lambda) * max_similarity
			mmrScore := lambda*relevance - (1-lambda)*maxSimilarity

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		// Add best candidate to selected
		selected = append(selected, remaining[bestIdx].Document)
		selectedEmbeddings = append(selectedEmbeddings, remaining[bestIdx].Document.Embedding)

		// Remove from remaining
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected, nil
}

// Delete removes documents by their IDs
func (vs *MemoryVectorStore) Delete(ctx context.Context, ids []string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	for _, id := range ids {
		delete(vs.documents, id)
	}

	// Update metrics
	vs.docsTotal.Set(float64(len(vs.documents)))

	return nil
}

// calculateSimilarity calculates similarity based on the configured metric
func (vs *MemoryVectorStore) calculateSimilarity(a, b []float32) float32 {
	switch vs.distanceMetric {
	case DistanceEuclidean:
		// Convert distance to similarity (inverse)
		dist := embeddings.EuclideanDistance(a, b)
		return 1 / (1 + dist)
	case DistanceDotProduct:
		return embeddings.DotProduct(a, b)
	default: // Cosine
		return embeddings.CosineSimilarity(a, b)
	}
}

// Count returns the number of documents in the store
func (vs *MemoryVectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.documents)
}

// GetDocument retrieves a document by ID
func (vs *MemoryVectorStore) GetDocument(id string) (Document, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	doc, ok := vs.documents[id]
	return doc, ok
}

// GetAllDocuments returns all documents in the store
func (vs *MemoryVectorStore) GetAllDocuments() []Document {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	docs := make([]Document, 0, len(vs.documents))
	for _, doc := range vs.documents {
		docs = append(docs, doc)
	}
	return docs
}

// Clear removes all documents from the store
func (vs *MemoryVectorStore) Clear() {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.documents = make(map[string]Document)
	vs.idCounter = 0

	// Update metrics
	vs.docsTotal.Set(0)
}

// Close releases resources used by the vector store.
// After calling Close, the vector store should not be used.
func (vs *MemoryVectorStore) Close() error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.documents = nil
	return nil
}

// SearchWithFilter performs similarity search with metadata filters
func (vs *MemoryVectorStore) SearchWithFilter(ctx context.Context, query string, k int, filters []Filter) ([]SearchResult, error) {
	// Embed the query
	queryEmbedding, err := vs.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Filter and score documents
	type scoredDoc struct {
		doc   Document
		score float32
	}
	var scored []scoredDoc

	for _, doc := range vs.documents {
		// Apply filters
		if !vs.matchesFilters(doc, filters) {
			continue
		}

		score := vs.calculateSimilarity(queryEmbedding, doc.Embedding)
		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score (descending)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top k
	if k > len(scored) {
		k = len(scored)
	}

	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		results[i] = SearchResult{
			Document: scored[i].doc,
			Score:    scored[i].score,
		}
	}

	return results, nil
}

// matchesFilters checks if a document matches all filters
func (vs *MemoryVectorStore) matchesFilters(doc Document, filters []Filter) bool {
	for _, filter := range filters {
		val, ok := doc.Metadata[filter.Field]
		if !ok {
			return false
		}

		if !vs.matchesFilter(val, filter) {
			return false
		}
	}
	return true
}

// matchesFilter checks if a value matches a single filter
func (vs *MemoryVectorStore) matchesFilter(val any, filter Filter) bool {
	switch filter.Operator {
	case FilterEquals:
		return val == filter.Value
	case FilterNotEquals:
		return val != filter.Value
	case FilterContains:
		str, ok := val.(string)
		if !ok {
			return false
		}
		substr, ok := filter.Value.(string)
		if !ok {
			return false
		}
		return containsString(str, substr)
	case FilterIn:
		list, ok := filter.Value.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if val == item {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
