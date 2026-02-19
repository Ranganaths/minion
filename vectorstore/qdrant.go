// Package vectorstore provides a Qdrant implementation of VectorStore.
package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Ranganaths/minion/embeddings"
	"github.com/google/uuid"
)

// QdrantConfig configures the Qdrant connection
type QdrantConfig struct {
	// Host is the Qdrant server host (default: "localhost")
	Host string

	// Port is the Qdrant HTTP port (default: 6333)
	Port int

	// APIKey is the optional API key for authentication
	APIKey string

	// CollectionName is the name of the collection to use
	CollectionName string

	// Dimension is the vector dimension
	Dimension int

	// Distance is the distance metric (Cosine, Euclid, Dot)
	Distance QdrantDistance

	// Timeout for HTTP requests (default: 30s)
	Timeout time.Duration

	// OnDiskPayload stores payload on disk instead of RAM
	OnDiskPayload bool

	// HNSWConfig configures the HNSW index
	HNSWConfig *QdrantHNSWConfig
}

// QdrantDistance represents distance metrics supported by Qdrant
type QdrantDistance string

const (
	QdrantDistanceCosine QdrantDistance = "Cosine"
	QdrantDistanceEuclid QdrantDistance = "Euclid"
	QdrantDistanceDot    QdrantDistance = "Dot"
)

// QdrantHNSWConfig configures the HNSW index
type QdrantHNSWConfig struct {
	M              int `json:"m,omitempty"`               // Number of edges per node (default: 16)
	EfConstruct    int `json:"ef_construct,omitempty"`    // Number of neighbors to consider during index building (default: 100)
	FullScanThreshold int `json:"full_scan_threshold,omitempty"` // Minimal size for HNSW index
	MaxIndexingThreads int `json:"max_indexing_threads,omitempty"` // Max parallel threads for indexing
	OnDisk         bool `json:"on_disk,omitempty"`         // Store HNSW index on disk
}

// DefaultQdrantConfig returns a default Qdrant configuration
func DefaultQdrantConfig() QdrantConfig {
	return QdrantConfig{
		Host:           "localhost",
		Port:           6333,
		CollectionName: "minion_vectors",
		Dimension:      1536,
		Distance:       QdrantDistanceCosine,
		Timeout:        30 * time.Second,
	}
}

// QdrantStore implements VectorStoreRetriever using Qdrant
type QdrantStore struct {
	config   QdrantConfig
	embedder embeddings.Embedder
	client   *http.Client
	baseURL  string
	mu       sync.RWMutex
}

// NewQdrantStore creates a new Qdrant vector store
func NewQdrantStore(config QdrantConfig, embedder embeddings.Embedder) (*QdrantStore, error) {
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 6333
	}
	if config.CollectionName == "" {
		return nil, fmt.Errorf("collection name is required")
	}
	if config.Dimension == 0 {
		config.Dimension = 1536
	}
	if config.Distance == "" {
		config.Distance = QdrantDistanceCosine
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	store := &QdrantStore{
		config:   config,
		embedder: embedder,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: fmt.Sprintf("http://%s:%d", config.Host, config.Port),
	}

	return store, nil
}

// EnsureCollection creates the collection if it doesn't exist
func (s *QdrantStore) EnsureCollection(ctx context.Context) error {
	// Check if collection exists
	exists, err := s.collectionExists(ctx)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Create collection
	return s.createCollection(ctx)
}

func (s *QdrantStore) collectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.config.CollectionName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("failed to check collection: %s", string(body))
	}

	return true, nil
}

func (s *QdrantStore) createCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.config.CollectionName)

	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     s.config.Dimension,
			"distance": string(s.config.Distance),
		},
	}

	if s.config.OnDiskPayload {
		payload["on_disk_payload"] = true
	}

	if s.config.HNSWConfig != nil {
		payload["hnsw_config"] = s.config.HNSWConfig
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(respBody))
	}

	return nil
}

// AddDocuments adds documents to the vector store
func (s *QdrantStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.EnsureCollection(ctx); err != nil {
		return nil, err
	}

	ids := make([]string, len(docs))
	points := make([]qdrantPoint, len(docs))

	for i, doc := range docs {
		// Generate ID if not provided
		id := doc.ID
		if id == "" {
			id = uuid.New().String()
		}
		ids[i] = id

		// Generate embedding if not provided
		embedding := doc.Embedding
		if len(embedding) == 0 && s.embedder != nil {
			var err error
			embedding, err = s.embedder.EmbedQuery(ctx, doc.PageContent)
			if err != nil {
				return nil, fmt.Errorf("failed to embed document %d: %w", i, err)
			}
		}

		// Build payload
		payload := make(map[string]interface{})
		payload["page_content"] = doc.PageContent
		for k, v := range doc.Metadata {
			payload[k] = v
		}

		points[i] = qdrantPoint{
			ID:      id,
			Vector:  embedding,
			Payload: payload,
		}
	}

	// Upsert points
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", s.baseURL, s.config.CollectionName)

	requestBody := map[string]interface{}{
		"points": points,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to upsert points: %s", string(respBody))
	}

	return ids, nil
}

// SimilaritySearch finds the most similar documents to a query
func (s *QdrantStore) SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error) {
	results, err := s.SimilaritySearchWithScore(ctx, query, k)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.Document
	}
	return docs, nil
}

// SimilaritySearchWithScore returns documents with their similarity scores
func (s *QdrantStore) SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error) {
	// Generate embedding for query
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder is required for similarity search")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	return s.SimilaritySearchByVectorWithScore(ctx, embedding, k)
}

// SimilaritySearchByVector searches using a pre-computed embedding
func (s *QdrantStore) SimilaritySearchByVector(ctx context.Context, embedding []float32, k int) ([]Document, error) {
	results, err := s.SimilaritySearchByVectorWithScore(ctx, embedding, k)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.Document
	}
	return docs, nil
}

// SimilaritySearchByVectorWithScore searches using a pre-computed embedding with scores
func (s *QdrantStore) SimilaritySearchByVectorWithScore(ctx context.Context, embedding []float32, k int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.config.CollectionName)

	requestBody := map[string]interface{}{
		"vector":       embedding,
		"limit":        k,
		"with_payload": true,
		"with_vector":  false,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(respBody))
	}

	var searchResp qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]SearchResult, len(searchResp.Result))
	for i, r := range searchResp.Result {
		doc := Document{
			ID:       fmt.Sprintf("%v", r.ID),
			Metadata: make(map[string]interface{}),
		}

		// Extract page_content and metadata from payload
		if content, ok := r.Payload["page_content"].(string); ok {
			doc.PageContent = content
		}
		for k, v := range r.Payload {
			if k != "page_content" {
				doc.Metadata[k] = v
			}
		}

		results[i] = SearchResult{
			Document: doc,
			Score:    r.Score,
		}
	}

	return results, nil
}

// SimilaritySearchWithFilters searches with metadata filters
func (s *QdrantStore) SimilaritySearchWithFilters(ctx context.Context, query string, k int, filters []Filter) ([]Document, error) {
	// Generate embedding for query
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder is required for similarity search")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.config.CollectionName)

	// Build Qdrant filter
	qdrantFilter := s.buildQdrantFilter(filters)

	requestBody := map[string]interface{}{
		"vector":       embedding,
		"limit":        k,
		"with_payload": true,
		"with_vector":  false,
	}

	if qdrantFilter != nil {
		requestBody["filter"] = qdrantFilter
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(respBody))
	}

	var searchResp qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	docs := make([]Document, len(searchResp.Result))
	for i, r := range searchResp.Result {
		doc := Document{
			ID:       fmt.Sprintf("%v", r.ID),
			Metadata: make(map[string]interface{}),
		}

		if content, ok := r.Payload["page_content"].(string); ok {
			doc.PageContent = content
		}
		for k, v := range r.Payload {
			if k != "page_content" {
				doc.Metadata[k] = v
			}
		}

		docs[i] = doc
	}

	return docs, nil
}

// MaxMarginalRelevanceSearch performs MMR search for diversity
func (s *QdrantStore) MaxMarginalRelevanceSearch(ctx context.Context, query string, k int, fetchK int, lambda float32) ([]Document, error) {
	// Generate embedding for query
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder is required for MMR search")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Fetch more candidates than needed
	if fetchK < k {
		fetchK = k * 4
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.config.CollectionName)

	requestBody := map[string]interface{}{
		"vector":       embedding,
		"limit":        fetchK,
		"with_payload": true,
		"with_vector":  true, // Need vectors for MMR
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(respBody))
	}

	var searchResp qdrantSearchResponseWithVectors
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Apply MMR selection
	selected := s.mmrSelect(embedding, searchResp.Result, k, lambda)

	docs := make([]Document, len(selected))
	for i, r := range selected {
		doc := Document{
			ID:       fmt.Sprintf("%v", r.ID),
			Metadata: make(map[string]interface{}),
		}

		if content, ok := r.Payload["page_content"].(string); ok {
			doc.PageContent = content
		}
		for key, v := range r.Payload {
			if key != "page_content" {
				doc.Metadata[key] = v
			}
		}

		docs[i] = doc
	}

	return docs, nil
}

// GetDocument retrieves a document by ID
func (s *QdrantStore) GetDocument(ctx context.Context, id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url := fmt.Sprintf("%s/collections/%s/points/%s", s.baseURL, s.config.CollectionName, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get document: %s", string(respBody))
	}

	var getResp qdrantGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	doc := &Document{
		ID:       id,
		Metadata: make(map[string]interface{}),
	}

	if getResp.Result != nil {
		if content, ok := getResp.Result.Payload["page_content"].(string); ok {
			doc.PageContent = content
		}
		for k, v := range getResp.Result.Payload {
			if k != "page_content" {
				doc.Metadata[k] = v
			}
		}
		doc.Embedding = getResp.Result.Vector
	}

	return doc, nil
}

// UpdateDocument updates a document
func (s *QdrantStore) UpdateDocument(ctx context.Context, doc Document) error {
	// For Qdrant, update is the same as upsert
	_, err := s.AddDocuments(ctx, []Document{doc})
	return err
}

// Delete removes documents by their IDs
func (s *QdrantStore) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", s.baseURL, s.config.CollectionName)

	requestBody := map[string]interface{}{
		"points": ids,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete points: %s", string(respBody))
	}

	return nil
}

// Count returns the number of documents in the store
func (s *QdrantStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.config.CollectionName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to get collection info: %s", string(respBody))
	}

	var infoResp qdrantCollectionInfo
	if err := json.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return infoResp.Result.PointsCount, nil
}

// Close closes the vector store connection
func (s *QdrantStore) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// DeleteCollection deletes the entire collection
func (s *QdrantStore) DeleteCollection(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.config.CollectionName)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete collection: %s", string(respBody))
	}

	return nil
}

// CreateSnapshot creates a snapshot of the collection
func (s *QdrantStore) CreateSnapshot(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/collections/%s/snapshots", s.baseURL, s.config.CollectionName)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create snapshot: %s", string(respBody))
	}

	var snapshotResp struct {
		Result struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&snapshotResp); err != nil {
		return "", err
	}

	return snapshotResp.Result.Name, nil
}

// Helper methods

func (s *QdrantStore) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if s.config.APIKey != "" {
		req.Header.Set("api-key", s.config.APIKey)
	}
}

func (s *QdrantStore) buildQdrantFilter(filters []Filter) map[string]interface{} {
	if len(filters) == 0 {
		return nil
	}

	must := make([]map[string]interface{}, 0, len(filters))

	for _, f := range filters {
		var condition map[string]interface{}

		switch f.Operator {
		case FilterEquals:
			condition = map[string]interface{}{
				"key":   f.Field,
				"match": map[string]interface{}{"value": f.Value},
			}
		case FilterNotEquals:
			// Qdrant uses must_not for negation
			continue
		case FilterGreaterThan:
			condition = map[string]interface{}{
				"key":   f.Field,
				"range": map[string]interface{}{"gt": f.Value},
			}
		case FilterLessThan:
			condition = map[string]interface{}{
				"key":   f.Field,
				"range": map[string]interface{}{"lt": f.Value},
			}
		case FilterIn:
			if vals, ok := f.Value.([]interface{}); ok {
				condition = map[string]interface{}{
					"key":   f.Field,
					"match": map[string]interface{}{"any": vals},
				}
			}
		case FilterContains:
			if str, ok := f.Value.(string); ok {
				condition = map[string]interface{}{
					"key":   f.Field,
					"match": map[string]interface{}{"text": str},
				}
			}
		}

		if condition != nil {
			must = append(must, condition)
		}
	}

	if len(must) == 0 {
		return nil
	}

	return map[string]interface{}{
		"must": must,
	}
}

func (s *QdrantStore) mmrSelect(queryEmbedding []float32, candidates []qdrantSearchResultWithVector, k int, lambda float32) []qdrantSearchResultWithVector {
	if len(candidates) <= k {
		return candidates
	}

	selected := make([]qdrantSearchResultWithVector, 0, k)
	remaining := make([]qdrantSearchResultWithVector, len(candidates))
	copy(remaining, candidates)

	// Select first document (most similar to query)
	selected = append(selected, remaining[0])
	remaining = remaining[1:]

	// Iteratively select remaining documents
	for len(selected) < k && len(remaining) > 0 {
		bestIdx := 0
		bestScore := float32(-1.0)

		for i, candidate := range remaining {
			// Compute MMR score
			querySim := qdrantCosineSimilarity(queryEmbedding, candidate.Vector)

			// Find max similarity to already selected documents
			maxDocSim := float32(0.0)
			for _, sel := range selected {
				sim := qdrantCosineSimilarity(candidate.Vector, sel.Vector)
				if sim > maxDocSim {
					maxDocSim = sim
				}
			}

			// MMR = lambda * sim(doc, query) - (1 - lambda) * max(sim(doc, selected))
			mmrScore := lambda*querySim - (1-lambda)*maxDocSim

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

func qdrantCosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (qdrantSqrt(normA) * qdrantSqrt(normB))
}

func qdrantSqrt(x float32) float32 {
	// Newton's method for square root
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// Internal types for Qdrant API

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantSearchResponseWithVectors struct {
	Result []qdrantSearchResultWithVector `json:"result"`
}

type qdrantSearchResultWithVector struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Vector  []float32              `json:"vector,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantGetResponse struct {
	Result *qdrantPointResult `json:"result"`
}

type qdrantPointResult struct {
	ID      interface{}            `json:"id"`
	Vector  []float32              `json:"vector,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type qdrantCollectionInfo struct {
	Result struct {
		PointsCount int64 `json:"points_count"`
		Status      string `json:"status"`
	} `json:"result"`
}

// Ensure QdrantStore implements VectorStoreRetriever
var _ VectorStoreRetriever = (*QdrantStore)(nil)
