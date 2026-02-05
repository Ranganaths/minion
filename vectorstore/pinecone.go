package vectorstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// PineconeConfig configures the Pinecone connection
type PineconeConfig struct {
	APIKey      string
	Environment string // e.g., "us-west1-gcp"
	IndexName   string
	Namespace   string
	ProjectID   string
	Dimension   int
	Metric      DistanceMetric // cosine, euclidean, dotproduct
}

// PineconeClient interface abstracts Pinecone operations for testing
type PineconeClient interface {
	// Vector operations
	Upsert(ctx context.Context, vectors []*PineconeVector) (*UpsertResponse, error)
	Query(ctx context.Context, query *PineconeQuery) (*QueryResponse, error)
	Fetch(ctx context.Context, ids []string, namespace string) (*FetchResponse, error)
	Delete(ctx context.Context, req *DeleteRequest) error
	Update(ctx context.Context, req *UpdateRequest) error

	// Index operations
	DescribeIndex(ctx context.Context) (*IndexDescription, error)
	DescribeIndexStats(ctx context.Context, filter map[string]interface{}) (*PineconeIndexStats, error)

	// Collection operations (for backups)
	CreateCollection(ctx context.Context, name, sourceIndex string) error
	ListCollections(ctx context.Context) ([]string, error)
	DeleteCollection(ctx context.Context, name string) error
}

// PineconeVector represents a vector to upsert
type PineconeVector struct {
	ID        string                 `json:"id"`
	Values    []float32              `json:"values"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	SparseValues *SparseValues       `json:"sparse_values,omitempty"`
}

// SparseValues represents sparse vector values for hybrid search
type SparseValues struct {
	Indices []uint32  `json:"indices"`
	Values  []float32 `json:"values"`
}

// PineconeQuery represents a query request
type PineconeQuery struct {
	Namespace       string                 `json:"namespace,omitempty"`
	TopK            int                    `json:"top_k"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeValues   bool                   `json:"include_values"`
	IncludeMetadata bool                   `json:"include_metadata"`
	Vector          []float32              `json:"vector,omitempty"`
	SparseVector    *SparseValues          `json:"sparse_vector,omitempty"`
	ID              string                 `json:"id,omitempty"` // Query by ID
}

// UpsertResponse represents an upsert response
type UpsertResponse struct {
	UpsertedCount int `json:"upserted_count"`
}

// QueryResponse represents a query response
type QueryResponse struct {
	Namespace string         `json:"namespace"`
	Matches   []QueryMatch   `json:"matches"`
}

// QueryMatch represents a query match
type QueryMatch struct {
	ID        string                 `json:"id"`
	Score     float32                `json:"score"`
	Values    []float32              `json:"values,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	SparseValues *SparseValues       `json:"sparse_values,omitempty"`
}

// FetchResponse represents a fetch response
type FetchResponse struct {
	Namespace string                     `json:"namespace"`
	Vectors   map[string]*PineconeVector `json:"vectors"`
}

// DeleteRequest represents a delete request
type DeleteRequest struct {
	IDs       []string               `json:"ids,omitempty"`
	DeleteAll bool                   `json:"delete_all,omitempty"`
	Namespace string                 `json:"namespace,omitempty"`
	Filter    map[string]interface{} `json:"filter,omitempty"`
}

// UpdateRequest represents an update request
type UpdateRequest struct {
	ID           string                 `json:"id"`
	Values       []float32              `json:"values,omitempty"`
	SetMetadata  map[string]interface{} `json:"set_metadata,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	SparseValues *SparseValues          `json:"sparse_values,omitempty"`
}

// IndexDescription describes an index
type IndexDescription struct {
	Name      string `json:"name"`
	Dimension int    `json:"dimension"`
	Metric    string `json:"metric"`
	Pods      int    `json:"pods"`
	Replicas  int    `json:"replicas"`
	PodType   string `json:"pod_type"`
	Status    string `json:"status"`
}

// PineconeIndexStats contains Pinecone index statistics
type PineconeIndexStats struct {
	Namespaces    map[string]NamespaceStats `json:"namespaces"`
	Dimension     int                       `json:"dimension"`
	IndexFullness float64                   `json:"index_fullness"`
	TotalCount    int64                     `json:"total_vector_count"`
}

// NamespaceStats contains namespace statistics
type NamespaceStats struct {
	VectorCount int64 `json:"vector_count"`
}

// PineconeStore implements VectorStore using Pinecone
type PineconeStore struct {
	client    PineconeClient
	config    *PineconeConfig
	dimension int
	mu        sync.RWMutex
}

// NewPineconeStore creates a new Pinecone vector store
func NewPineconeStore(client PineconeClient, config *PineconeConfig) *PineconeStore {
	return &PineconeStore{
		client:    client,
		config:    config,
		dimension: config.Dimension,
	}
}

// AddDocuments adds documents to the vector store
func (s *PineconeStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	vectors := make([]*PineconeVector, len(docs))
	ids := make([]string, len(docs))

	for i, doc := range docs {
		if len(doc.Embedding) != s.dimension {
			return nil, fmt.Errorf("document %d has wrong dimension: got %d, want %d", i, len(doc.Embedding), s.dimension)
		}

		id := doc.ID
		if id == "" {
			id = generateDocID()
		}
		ids[i] = id

		metadata := make(map[string]interface{})
		metadata["content"] = doc.PageContent
		for k, v := range doc.Metadata {
			metadata[k] = v
		}

		vectors[i] = &PineconeVector{
			ID:       id,
			Values:   doc.Embedding,
			Metadata: metadata,
		}
	}

	_, err := s.client.Upsert(ctx, vectors)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert vectors: %w", err)
	}

	return ids, nil
}

// AddDocumentsBatch adds documents in batches
func (s *PineconeStore) AddDocumentsBatch(ctx context.Context, docs []Document, batchSize int) ([]string, error) {
	if batchSize <= 0 {
		batchSize = 100 // Pinecone recommended batch size
	}

	var allIDs []string

	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		ids, err := s.AddDocuments(ctx, docs[i:end])
		if err != nil {
			return allIDs, err
		}

		allIDs = append(allIDs, ids...)
	}

	return allIDs, nil
}

// SimilaritySearch searches for similar documents
func (s *PineconeStore) SimilaritySearch(ctx context.Context, query []float32, k int, filter *Filter) ([]Document, error) {
	results, err := s.SimilaritySearchWithScore(ctx, query, k, filter)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.Document
	}

	return docs, nil
}

// SimilaritySearchWithScore searches with scores
func (s *PineconeStore) SimilaritySearchWithScore(ctx context.Context, query []float32, k int, filter *Filter) ([]SearchResult, error) {
	if len(query) != s.dimension {
		return nil, fmt.Errorf("query has wrong dimension: got %d, want %d", len(query), s.dimension)
	}

	pineconeQuery := &PineconeQuery{
		Namespace:       s.config.Namespace,
		TopK:            k,
		Vector:          query,
		IncludeValues:   true,
		IncludeMetadata: true,
	}

	// Convert filter
	if filter != nil {
		pineconeQuery.Filter = s.convertFilter(filter)
	}

	resp, err := s.client.Query(ctx, pineconeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	results := make([]SearchResult, len(resp.Matches))
	for i, match := range resp.Matches {
		doc := Document{
			ID:        match.ID,
			Embedding: match.Values,
			Metadata:  make(map[string]any),
		}

		if match.Metadata != nil {
			if content, ok := match.Metadata["content"].(string); ok {
				doc.PageContent = content
			}
			for k, v := range match.Metadata {
				if k != "content" {
					doc.Metadata[k] = v
				}
			}
		}

		results[i] = SearchResult{
			Document: doc,
			Score:    match.Score,
		}
	}

	return results, nil
}

// HybridSearch performs hybrid search with sparse vectors
func (s *PineconeStore) HybridSearch(ctx context.Context, denseVector []float32, sparseVector *SparseValues, k int, alpha float64, filter *Filter) ([]SearchResult, error) {
	if len(denseVector) != s.dimension {
		return nil, fmt.Errorf("dense vector has wrong dimension: got %d, want %d", len(denseVector), s.dimension)
	}

	pineconeQuery := &PineconeQuery{
		Namespace:       s.config.Namespace,
		TopK:            k,
		Vector:          denseVector,
		SparseVector:    sparseVector,
		IncludeValues:   true,
		IncludeMetadata: true,
	}

	if filter != nil {
		pineconeQuery.Filter = s.convertFilter(filter)
	}

	resp, err := s.client.Query(ctx, pineconeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to hybrid query: %w", err)
	}

	results := make([]SearchResult, len(resp.Matches))
	for i, match := range resp.Matches {
		doc := Document{
			ID:        match.ID,
			Embedding: match.Values,
			Metadata:  make(map[string]any),
		}

		if match.Metadata != nil {
			if content, ok := match.Metadata["content"].(string); ok {
				doc.PageContent = content
			}
			for k, v := range match.Metadata {
				if k != "content" {
					doc.Metadata[k] = v
				}
			}
		}

		results[i] = SearchResult{
			Document: doc,
			Score:    match.Score,
		}
	}

	return results, nil
}

// GetDocument retrieves a document by ID
func (s *PineconeStore) GetDocument(ctx context.Context, id string) (*Document, error) {
	resp, err := s.client.Fetch(ctx, []string{id}, s.config.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch document: %w", err)
	}

	vec, ok := resp.Vectors[id]
	if !ok {
		return nil, nil
	}

	doc := &Document{
		ID:        vec.ID,
		Embedding: vec.Values,
		Metadata:  make(map[string]any),
	}

	if vec.Metadata != nil {
		if content, ok := vec.Metadata["content"].(string); ok {
			doc.PageContent = content
		}
		for k, v := range vec.Metadata {
			if k != "content" {
				doc.Metadata[k] = v
			}
		}
	}

	return doc, nil
}

// DeleteDocuments removes documents by IDs
func (s *PineconeStore) DeleteDocuments(ctx context.Context, ids []string) error {
	req := &DeleteRequest{
		IDs:       ids,
		Namespace: s.config.Namespace,
	}

	return s.client.Delete(ctx, req)
}

// DeleteByFilter deletes documents matching a filter
func (s *PineconeStore) DeleteByFilter(ctx context.Context, filter *Filter) error {
	if filter == nil {
		return errors.New("filter is required")
	}

	req := &DeleteRequest{
		Namespace: s.config.Namespace,
		Filter:    s.convertFilter(filter),
	}

	return s.client.Delete(ctx, req)
}

// UpdateDocument updates a document
func (s *PineconeStore) UpdateDocument(ctx context.Context, id string, doc *Document) error {
	metadata := make(map[string]interface{})
	metadata["content"] = doc.PageContent
	for k, v := range doc.Metadata {
		metadata[k] = v
	}

	req := &UpdateRequest{
		ID:          id,
		Values:      doc.Embedding,
		SetMetadata: metadata,
		Namespace:   s.config.Namespace,
	}

	return s.client.Update(ctx, req)
}

// CreateIndex is a no-op for Pinecone (indexes are created via console/API)
func (s *PineconeStore) CreateIndex(ctx context.Context, config *IndexConfig) error {
	// Pinecone indexes are created via the console or management API
	// This is a no-op as we assume the index already exists
	return nil
}

// DeleteIndex is a no-op for Pinecone
func (s *PineconeStore) DeleteIndex(ctx context.Context) error {
	// Pinecone indexes are deleted via the console or management API
	return nil
}

// Count returns the number of vectors in the index
func (s *PineconeStore) Count(ctx context.Context) (int64, error) {
	stats, err := s.client.DescribeIndexStats(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get index stats: %w", err)
	}

	if s.config.Namespace != "" {
		if ns, ok := stats.Namespaces[s.config.Namespace]; ok {
			return ns.VectorCount, nil
		}
		return 0, nil
	}

	return stats.TotalCount, nil
}

// GetStats returns index statistics
func (s *PineconeStore) GetStats(ctx context.Context) (*PineconeIndexStats, error) {
	return s.client.DescribeIndexStats(ctx, nil)
}

// convertFilter converts our filter format to Pinecone format
func (s *PineconeStore) convertFilter(filter *Filter) map[string]interface{} {
	if filter == nil {
		return nil
	}

	result := make(map[string]interface{})
	result[filter.Field] = s.convertOperator(filter.Operator, filter.Value)

	return result
}

func (s *PineconeStore) convertOperator(op FilterOperator, value any) map[string]interface{} {
	switch op {
	case FilterEquals:
		return map[string]interface{}{"$eq": value}
	case FilterNotEquals:
		return map[string]interface{}{"$ne": value}
	case FilterGreaterThan:
		return map[string]interface{}{"$gt": value}
	case FilterLessThan:
		return map[string]interface{}{"$lt": value}
	case FilterIn:
		return map[string]interface{}{"$in": value}
	case FilterContains:
		return map[string]interface{}{"$contains": value}
	default:
		return map[string]interface{}{"$eq": value}
	}
}

// PineconeBatchWriter handles batch writes with automatic flushing
type PineconeBatchWriter struct {
	store     *PineconeStore
	buffer    []Document
	batchSize int
	mu        sync.Mutex
}

// NewPineconeBatchWriter creates a new batch writer
func NewPineconeBatchWriter(store *PineconeStore, batchSize int) *PineconeBatchWriter {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &PineconeBatchWriter{
		store:     store,
		buffer:    make([]Document, 0, batchSize),
		batchSize: batchSize,
	}
}

// Add adds a document to the batch
func (w *PineconeBatchWriter) Add(ctx context.Context, doc Document) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, doc)

	if len(w.buffer) >= w.batchSize {
		return w.flushLocked(ctx)
	}

	return nil
}

// Flush flushes all pending documents
func (w *PineconeBatchWriter) Flush(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.flushLocked(ctx)
}

func (w *PineconeBatchWriter) flushLocked(ctx context.Context) error {
	if len(w.buffer) == 0 {
		return nil
	}

	_, err := w.store.AddDocuments(ctx, w.buffer)
	if err != nil {
		return err
	}

	w.buffer = w.buffer[:0]
	return nil
}

// PineconeCollectionManager manages Pinecone collections (backups)
type PineconeCollectionManager struct {
	client PineconeClient
}

// NewPineconeCollectionManager creates a new collection manager
func NewPineconeCollectionManager(client PineconeClient) *PineconeCollectionManager {
	return &PineconeCollectionManager{client: client}
}

// CreateBackup creates a backup collection from an index
func (m *PineconeCollectionManager) CreateBackup(ctx context.Context, name, sourceIndex string) error {
	return m.client.CreateCollection(ctx, name, sourceIndex)
}

// ListBackups lists all backup collections
func (m *PineconeCollectionManager) ListBackups(ctx context.Context) ([]string, error) {
	return m.client.ListCollections(ctx)
}

// DeleteBackup deletes a backup collection
func (m *PineconeCollectionManager) DeleteBackup(ctx context.Context, name string) error {
	return m.client.DeleteCollection(ctx, name)
}

// PineconeMetrics collects metrics about Pinecone operations
type PineconeMetrics struct {
	QueriesTotal    int64
	UpsertTotal     int64
	DeleteTotal     int64
	QueryLatencySum time.Duration
	UpsertLatencySum time.Duration
	ErrorsTotal     int64
	mu              sync.Mutex
}

// RecordQuery records a query operation
func (m *PineconeMetrics) RecordQuery(latency time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.QueriesTotal++
	m.QueryLatencySum += latency
	if err != nil {
		m.ErrorsTotal++
	}
}

// RecordUpsert records an upsert operation
func (m *PineconeMetrics) RecordUpsert(latency time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpsertTotal++
	m.UpsertLatencySum += latency
	if err != nil {
		m.ErrorsTotal++
	}
}

// GetStats returns current metrics
func (m *PineconeMetrics) GetStats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	var avgQueryLatency, avgUpsertLatency time.Duration
	if m.QueriesTotal > 0 {
		avgQueryLatency = m.QueryLatencySum / time.Duration(m.QueriesTotal)
	}
	if m.UpsertTotal > 0 {
		avgUpsertLatency = m.UpsertLatencySum / time.Duration(m.UpsertTotal)
	}

	return map[string]interface{}{
		"queries_total":       m.QueriesTotal,
		"upserts_total":       m.UpsertTotal,
		"deletes_total":       m.DeleteTotal,
		"errors_total":        m.ErrorsTotal,
		"avg_query_latency":   avgQueryLatency,
		"avg_upsert_latency":  avgUpsertLatency,
	}
}

// MetricsPineconeStore wraps PineconeStore with metrics collection
type MetricsPineconeStore struct {
	*PineconeStore
	metrics *PineconeMetrics
}

// NewMetricsPineconeStore creates a metrics-enabled Pinecone store
func NewMetricsPineconeStore(client PineconeClient, config *PineconeConfig) *MetricsPineconeStore {
	return &MetricsPineconeStore{
		PineconeStore: NewPineconeStore(client, config),
		metrics:       &PineconeMetrics{},
	}
}

// SimilaritySearchWithScore wraps the base method with metrics
func (s *MetricsPineconeStore) SimilaritySearchWithScore(ctx context.Context, query []float32, k int, filter *Filter) ([]SearchResult, error) {
	start := time.Now()
	results, err := s.PineconeStore.SimilaritySearchWithScore(ctx, query, k, filter)
	s.metrics.RecordQuery(time.Since(start), err)
	return results, err
}

// AddDocuments wraps the base method with metrics
func (s *MetricsPineconeStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	start := time.Now()
	ids, err := s.PineconeStore.AddDocuments(ctx, docs)
	s.metrics.RecordUpsert(time.Since(start), err)
	return ids, err
}

// GetMetrics returns collected metrics
func (s *MetricsPineconeStore) GetMetrics() map[string]interface{} {
	return s.metrics.GetStats()
}

func generateDocID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}
