package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Ranganaths/minion/embeddings"
)

// HybridStore combines vector similarity search with full-text search (BM25)
type HybridStore struct {
	db             *sql.DB
	embedder       embeddings.Embedder
	tableName      string
	dimension      int
	distanceMetric DistanceMetric
	alpha          float32 // Weight for vector similarity (1-alpha for text search)
}

// HybridConfig configures the hybrid store
type HybridConfig struct {
	// ConnectionString is the PostgreSQL connection string
	ConnectionString string

	// DB is an existing database connection
	DB *sql.DB

	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// TableName is the name of the table
	TableName string

	// Dimension is the embedding dimension
	Dimension int

	// DistanceMetric for vector similarity
	DistanceMetric DistanceMetric

	// Alpha is the weight for vector similarity (0-1)
	// Score = alpha * vector_score + (1 - alpha) * text_score
	Alpha float32

	// CreateTable creates the table if it doesn't exist
	CreateTable bool
}

// NewHybridStore creates a new hybrid store
func NewHybridStore(ctx context.Context, config HybridConfig) (*HybridStore, error) {
	var db *sql.DB
	var err error

	if config.DB != nil {
		db = config.DB
	} else if config.ConnectionString != "" {
		db, err = sql.Open("postgres", config.ConnectionString)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
		if err := db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to ping database: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either ConnectionString or DB must be provided")
	}

	if config.TableName == "" {
		config.TableName = "documents"
	}

	if config.Dimension == 0 {
		config.Dimension = 1536
	}

	if config.DistanceMetric == "" {
		config.DistanceMetric = DistanceCosine
	}

	if config.Alpha == 0 {
		config.Alpha = 0.5 // Equal weight by default
	}

	store := &HybridStore{
		db:             db,
		embedder:       config.Embedder,
		tableName:      config.TableName,
		dimension:      config.Dimension,
		distanceMetric: config.DistanceMetric,
		alpha:          config.Alpha,
	}

	// Ensure extensions
	if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return nil, fmt.Errorf("failed to create vector extension: %w", err)
	}

	if config.CreateTable {
		if err := store.createTable(ctx); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// createTable creates the hybrid table with both vector and full-text search support
func (s *HybridStore) createTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			embedding vector(%d),
			metadata JSONB DEFAULT '{}',
			content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, s.tableName, s.dimension)

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create vector index
	vectorIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %s
		USING hnsw (embedding vector_cosine_ops)
		WITH (m = 16, ef_construction = 64)
	`, s.tableName, s.tableName)
	s.db.ExecContext(ctx, vectorIndexQuery)

	// Create full-text search index
	ftsIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_content_tsv_idx ON %s USING GIN (content_tsv)
	`, s.tableName, s.tableName)
	s.db.ExecContext(ctx, ftsIndexQuery)

	// Create metadata index
	metaIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_metadata_idx ON %s USING GIN (metadata)
	`, s.tableName, s.tableName)
	s.db.ExecContext(ctx, metaIndexQuery)

	return nil
}

// HybridSearch performs combined vector and full-text search
func (s *HybridStore) HybridSearch(ctx context.Context, query string, k int) ([]SearchResult, error) {
	return s.HybridSearchWithAlpha(ctx, query, k, s.alpha)
}

// HybridSearchWithAlpha performs hybrid search with custom alpha
func (s *HybridStore) HybridSearchWithAlpha(ctx context.Context, query string, k int, alpha float32) ([]SearchResult, error) {
	// Generate embedding for query
	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	return s.HybridSearchWithEmbedding(ctx, query, embedding, k, alpha)
}

// HybridSearchWithEmbedding performs hybrid search with pre-computed embedding
func (s *HybridStore) HybridSearchWithEmbedding(ctx context.Context, textQuery string, embedding []float32, k int, alpha float32) ([]SearchResult, error) {
	embeddingStr := embeddingToString(embedding)

	// Fetch more candidates than k to combine results
	fetchK := k * 3
	if fetchK < 20 {
		fetchK = 20
	}

	// Combined query using CTE
	sqlQuery := fmt.Sprintf(`
		WITH vector_results AS (
			SELECT id, content, metadata, embedding,
				   1 - (embedding <=> $1::vector) as vector_score
			FROM %s
			ORDER BY embedding <=> $1::vector
			LIMIT $2
		),
		text_results AS (
			SELECT id, content, metadata, embedding,
				   ts_rank_cd(content_tsv, plainto_tsquery('english', $3)) as text_score
			FROM %s
			WHERE content_tsv @@ plainto_tsquery('english', $3)
			LIMIT $2
		),
		combined AS (
			SELECT COALESCE(v.id, t.id) as id,
				   COALESCE(v.content, t.content) as content,
				   COALESCE(v.metadata, t.metadata) as metadata,
				   COALESCE(v.embedding, t.embedding) as embedding,
				   COALESCE(v.vector_score, 0) as vector_score,
				   COALESCE(t.text_score, 0) as text_score
			FROM vector_results v
			FULL OUTER JOIN text_results t ON v.id = t.id
		)
		SELECT id, content, metadata, embedding,
			   ($4 * vector_score + (1 - $4) * text_score) as combined_score
		FROM combined
		ORDER BY combined_score DESC
		LIMIT $5
	`, s.tableName, s.tableName)

	rows, err := s.db.QueryContext(ctx, sqlQuery, embeddingStr, fetchK, textQuery, alpha, k)
	if err != nil {
		return nil, fmt.Errorf("failed to execute hybrid search: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, k)
	for rows.Next() {
		var doc Document
		var metadataJSON []byte
		var embeddingStr string
		var score float32

		if err := rows.Scan(&doc.ID, &doc.PageContent, &metadataJSON, &embeddingStr, &score); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]any)
		}

		doc.Embedding = stringToEmbedding(embeddingStr)

		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	return results, rows.Err()
}

// SetAlpha sets the hybrid search alpha parameter
func (s *HybridStore) SetAlpha(alpha float32) {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	s.alpha = alpha
}

// GetAlpha returns the current alpha value
func (s *HybridStore) GetAlpha() float32 {
	return s.alpha
}

// VectorOnlySearch performs pure vector similarity search
func (s *HybridStore) VectorOnlySearch(ctx context.Context, query string, k int) ([]SearchResult, error) {
	return s.HybridSearchWithAlpha(ctx, query, k, 1.0)
}

// TextOnlySearch performs pure full-text search
func (s *HybridStore) TextOnlySearch(ctx context.Context, query string, k int) ([]SearchResult, error) {
	return s.HybridSearchWithAlpha(ctx, query, k, 0.0)
}

// AddDocuments adds documents to the hybrid store
func (s *HybridStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	pgStore := &PgVectorStore{
		db:             s.db,
		embedder:       s.embedder,
		tableName:      s.tableName,
		dimension:      s.dimension,
		distanceMetric: s.distanceMetric,
	}
	return pgStore.AddDocuments(ctx, docs)
}

// Delete removes documents by IDs
func (s *HybridStore) Delete(ctx context.Context, ids []string) error {
	pgStore := &PgVectorStore{
		db:        s.db,
		tableName: s.tableName,
	}
	return pgStore.Delete(ctx, ids)
}

// SimilaritySearch implements VectorStore interface
func (s *HybridStore) SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error) {
	results, err := s.HybridSearch(ctx, query, k)
	if err != nil {
		return nil, err
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.Document
	}
	return docs, nil
}

// SimilaritySearchWithScore implements VectorStore interface
func (s *HybridStore) SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error) {
	return s.HybridSearch(ctx, query, k)
}

// Close closes the database connection
func (s *HybridStore) Close() error {
	return s.db.Close()
}

// ReciprocalRankFusion combines results from multiple search methods
type ReciprocalRankFusion struct {
	K int // Constant for RRF formula (typically 60)
}

// NewReciprocalRankFusion creates a new RRF combiner
func NewReciprocalRankFusion(k int) *ReciprocalRankFusion {
	if k <= 0 {
		k = 60
	}
	return &ReciprocalRankFusion{K: k}
}

// Combine merges results from multiple searches using RRF
func (rrf *ReciprocalRankFusion) Combine(resultSets ...[]SearchResult) []SearchResult {
	scores := make(map[string]float32)
	docs := make(map[string]Document)

	for _, results := range resultSets {
		for rank, result := range results {
			id := result.Document.ID
			// RRF score = 1 / (K + rank)
			scores[id] += 1.0 / float32(rrf.K+rank+1)
			if _, exists := docs[id]; !exists {
				docs[id] = result.Document
			}
		}
	}

	// Convert to slice and sort
	combined := make([]SearchResult, 0, len(scores))
	for id, score := range scores {
		combined = append(combined, SearchResult{
			Document: docs[id],
			Score:    score,
		})
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	return combined
}

// Helper functions

func embeddingToString(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func stringToEmbedding(str string) []float32 {
	if str == "" {
		return nil
	}
	str = strings.TrimPrefix(str, "[")
	str = strings.TrimSuffix(str, "]")

	parts := strings.Split(str, ",")
	embedding := make([]float32, len(parts))
	for i, p := range parts {
		var v float64
		fmt.Sscanf(strings.TrimSpace(p), "%f", &v)
		embedding[i] = float32(v)
	}
	return embedding
}

// Ensure HybridStore implements VectorStore
var _ VectorStore = (*HybridStore)(nil)
