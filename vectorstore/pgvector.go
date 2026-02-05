package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ranganaths/minion/embeddings"
	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// PgVectorStore implements VectorStore using PostgreSQL with pgvector extension
type PgVectorStore struct {
	db             *sql.DB
	embedder       embeddings.Embedder
	tableName      string
	dimension      int
	distanceMetric DistanceMetric
	indexType      IndexType
}

// PgVectorConfig configures the PgVector store
type PgVectorConfig struct {
	// ConnectionString is the PostgreSQL connection string
	ConnectionString string

	// DB is an existing database connection (optional, alternative to ConnectionString)
	DB *sql.DB

	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// TableName is the name of the table to use
	TableName string

	// Dimension is the embedding dimension
	Dimension int

	// DistanceMetric is the distance metric to use
	DistanceMetric DistanceMetric

	// CreateTable creates the table if it doesn't exist
	CreateTable bool

	// IndexType is the type of index to create
	IndexType IndexType

	// HNSW is the HNSW index configuration
	HNSW *HNSWConfig

	// IVFFlat is the IVFFlat index configuration
	IVFFlat *IVFFlatConfig
}

// NewPgVectorStore creates a new PgVector store
func NewPgVectorStore(ctx context.Context, config PgVectorConfig) (*PgVectorStore, error) {
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
		return nil, errors.New("either ConnectionString or DB must be provided")
	}

	if config.TableName == "" {
		config.TableName = "documents"
	}

	if config.Dimension == 0 {
		config.Dimension = 1536 // OpenAI default
	}

	if config.DistanceMetric == "" {
		config.DistanceMetric = DistanceCosine
	}

	if config.IndexType == "" {
		config.IndexType = IndexTypeHNSW
	}

	store := &PgVectorStore{
		db:             db,
		embedder:       config.Embedder,
		tableName:      config.TableName,
		dimension:      config.Dimension,
		distanceMetric: config.DistanceMetric,
		indexType:      config.IndexType,
	}

	// Ensure pgvector extension exists
	if err := store.ensureExtension(ctx); err != nil {
		return nil, err
	}

	// Create table if requested
	if config.CreateTable {
		if err := store.createTable(ctx); err != nil {
			return nil, err
		}

		// Create index
		indexConfig := IndexConfig{
			Name:           config.TableName + "_embedding_idx",
			Dimension:      config.Dimension,
			DistanceMetric: config.DistanceMetric,
			IndexType:      config.IndexType,
			HNSW:           config.HNSW,
			IVFFlat:        config.IVFFlat,
		}
		if err := store.CreateIndex(ctx, indexConfig); err != nil {
			// Index might already exist, log but don't fail
		}
	}

	return store, nil
}

// ensureExtension ensures the pgvector extension exists
func (s *PgVectorStore) ensureExtension(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}
	return nil
}

// createTable creates the documents table
func (s *PgVectorStore) createTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT NOT NULL,
			embedding vector(%d),
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, s.tableName, s.dimension)

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create GIN index on metadata for efficient filtering
	metaIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_metadata_idx ON %s USING GIN (metadata)
	`, s.tableName, s.tableName)
	_, _ = s.db.ExecContext(ctx, metaIndexQuery)

	// Create full-text search index
	ftsIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_content_fts_idx ON %s USING GIN (to_tsvector('english', content))
	`, s.tableName, s.tableName)
	_, _ = s.db.ExecContext(ctx, ftsIndexQuery)

	return nil
}

// CreateIndex creates a vector index
func (s *PgVectorStore) CreateIndex(ctx context.Context, config IndexConfig) error {
	if config.Name == "" {
		config.Name = s.tableName + "_embedding_idx"
	}

	var indexQuery string
	ops := s.getDistanceOps()

	switch config.IndexType {
	case IndexTypeHNSW:
		hnsw := config.HNSW
		if hnsw == nil {
			hnsw = DefaultHNSWConfig()
		}
		indexQuery = fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s
			USING hnsw (embedding %s)
			WITH (m = %d, ef_construction = %d)
		`, config.Name, s.tableName, ops, hnsw.M, hnsw.EfConstruction)

	case IndexTypeIVFFlat:
		ivf := config.IVFFlat
		if ivf == nil {
			ivf = DefaultIVFFlatConfig()
		}
		indexQuery = fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s
			USING ivfflat (embedding %s)
			WITH (lists = %d)
		`, config.Name, s.tableName, ops, ivf.NLists)

	default:
		return fmt.Errorf("unsupported index type: %s", config.IndexType)
	}

	_, err := s.db.ExecContext(ctx, indexQuery)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// DropIndex drops the vector index
func (s *PgVectorStore) DropIndex(ctx context.Context) error {
	indexName := s.tableName + "_embedding_idx"
	query := fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName)
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// IndexExists checks if the vector index exists
func (s *PgVectorStore) IndexExists(ctx context.Context) (bool, error) {
	indexName := s.tableName + "_embedding_idx"
	query := `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`

	var exists bool
	err := s.db.QueryRowContext(ctx, query, indexName).Scan(&exists)
	return exists, err
}

// GetIndexStats returns index statistics
func (s *PgVectorStore) GetIndexStats(ctx context.Context) (*IndexStats, error) {
	count, err := s.Count(ctx)
	if err != nil {
		return nil, err
	}

	// Get index size
	indexName := s.tableName + "_embedding_idx"
	var indexSize int64
	sizeQuery := `SELECT pg_relation_size($1::regclass)`
	_ = s.db.QueryRowContext(ctx, sizeQuery, indexName).Scan(&indexSize)

	return &IndexStats{
		TotalDocuments: count,
		IndexSize:      indexSize,
		Dimension:      s.dimension,
		IndexType:      s.indexType,
		LastModified:   time.Now(),
	}, nil
}

// getDistanceOps returns the pgvector operator class for the distance metric
func (s *PgVectorStore) getDistanceOps() string {
	switch s.distanceMetric {
	case DistanceCosine:
		return "vector_cosine_ops"
	case DistanceEuclidean:
		return "vector_l2_ops"
	case DistanceDotProduct:
		return "vector_ip_ops"
	default:
		return "vector_cosine_ops"
	}
}

// getDistanceOperator returns the pgvector distance operator
func (s *PgVectorStore) getDistanceOperator() string {
	switch s.distanceMetric {
	case DistanceCosine:
		return "<=>"
	case DistanceEuclidean:
		return "<->"
	case DistanceDotProduct:
		return "<#>"
	default:
		return "<=>"
	}
}

// AddDocuments adds documents to the vector store
func (s *PgVectorStore) AddDocuments(ctx context.Context, docs []Document) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	ids := make([]string, len(docs))

	for i, doc := range docs {
		// Generate embedding if not provided
		if len(doc.Embedding) == 0 && s.embedder != nil {
			embedding, err := s.embedder.EmbedQuery(ctx, doc.PageContent)
			if err != nil {
				return nil, fmt.Errorf("failed to generate embedding for document %d: %w", i, err)
			}
			doc.Embedding = embedding
		}

		// Generate ID if not provided
		if doc.ID == "" {
			doc.ID = uuid.New().String()
		}

		// Marshal metadata
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// Convert embedding to pgvector format
		embeddingStr := s.embeddingToString(doc.Embedding)

		// Insert document
		query := fmt.Sprintf(`
			INSERT INTO %s (id, content, embedding, metadata)
			VALUES ($1, $2, $3::vector, $4)
			ON CONFLICT (id) DO UPDATE SET
				content = EXCLUDED.content,
				embedding = EXCLUDED.embedding,
				metadata = EXCLUDED.metadata,
				updated_at = NOW()
		`, s.tableName)

		_, err = s.db.ExecContext(ctx, query, doc.ID, doc.PageContent, embeddingStr, metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to insert document: %w", err)
		}

		ids[i] = doc.ID
	}

	return ids, nil
}

// SimilaritySearch finds similar documents using text query
func (s *PgVectorStore) SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error) {
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

// SimilaritySearchWithScore returns documents with similarity scores
func (s *PgVectorStore) SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error) {
	if s.embedder == nil {
		return nil, errors.New("embedder is required for text query search")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	return s.SimilaritySearchByVectorWithScore(ctx, embedding, k)
}

// SimilaritySearchByVector searches using a pre-computed embedding
func (s *PgVectorStore) SimilaritySearchByVector(ctx context.Context, embedding []float32, k int) ([]Document, error) {
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

// SimilaritySearchByVectorWithScore searches with a pre-computed embedding and returns scores
func (s *PgVectorStore) SimilaritySearchByVectorWithScore(ctx context.Context, embedding []float32, k int) ([]SearchResult, error) {
	embeddingStr := s.embeddingToString(embedding)
	distanceOp := s.getDistanceOperator()

	query := fmt.Sprintf(`
		SELECT id, content, metadata, embedding,
			   1 - (embedding %s $1::vector) as score
		FROM %s
		ORDER BY embedding %s $1::vector
		LIMIT $2
	`, distanceOp, s.tableName, distanceOp)

	rows, err := s.db.QueryContext(ctx, query, embeddingStr, k)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}
	defer rows.Close()

	return s.scanSearchResults(rows)
}

// SimilaritySearchWithFilters searches with metadata filters
func (s *PgVectorStore) SimilaritySearchWithFilters(ctx context.Context, query string, k int, filters []Filter) ([]Document, error) {
	if s.embedder == nil {
		return nil, errors.New("embedder is required for text query search")
	}

	embedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	embeddingStr := s.embeddingToString(embedding)
	distanceOp := s.getDistanceOperator()

	// Build filter conditions
	whereClause, args := s.buildFilterClause(filters, 2) // Start at $2 since $1 is embedding

	sqlQuery := fmt.Sprintf(`
		SELECT id, content, metadata, embedding
		FROM %s
		WHERE %s
		ORDER BY embedding %s $1::vector
		LIMIT $%d
	`, s.tableName, whereClause, distanceOp, len(args)+2)

	allArgs := append([]interface{}{embeddingStr}, args...)
	allArgs = append(allArgs, k)

	rows, err := s.db.QueryContext(ctx, sqlQuery, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute filtered search: %w", err)
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}

// MaxMarginalRelevanceSearch performs MMR search for diversity
func (s *PgVectorStore) MaxMarginalRelevanceSearch(ctx context.Context, query string, k int, fetchK int, lambda float32) ([]Document, error) {
	if s.embedder == nil {
		return nil, errors.New("embedder is required for text query search")
	}

	queryEmbedding, err := s.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Fetch more candidates
	results, err := s.SimilaritySearchByVectorWithScore(ctx, queryEmbedding, fetchK)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Apply MMR
	selected := make([]Document, 0, k)
	selectedEmbeddings := make([][]float32, 0, k)

	for len(selected) < k && len(results) > 0 {
		bestIdx := -1
		bestScore := float32(-1)

		for i, result := range results {
			// Relevance to query
			relevance := result.Score

			// Max similarity to already selected
			maxSim := float32(0)
			for _, selEmb := range selectedEmbeddings {
				sim := cosineSimilarity(result.Document.Embedding, selEmb)
				if sim > maxSim {
					maxSim = sim
				}
			}

			// MMR score
			mmrScore := lambda*float32(relevance) - (1-lambda)*maxSim

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			break
		}

		selected = append(selected, results[bestIdx].Document)
		selectedEmbeddings = append(selectedEmbeddings, results[bestIdx].Document.Embedding)

		// Remove selected from candidates
		results = append(results[:bestIdx], results[bestIdx+1:]...)
	}

	return selected, nil
}

// GetDocument retrieves a document by ID
func (s *PgVectorStore) GetDocument(ctx context.Context, id string) (*Document, error) {
	query := fmt.Sprintf(`
		SELECT id, content, metadata, embedding, created_at, updated_at
		FROM %s WHERE id = $1
	`, s.tableName)

	var doc Document
	var metadataJSON []byte
	var embeddingStr string
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&doc.ID, &doc.PageContent, &metadataJSON, &embeddingStr, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
		doc.Metadata = make(map[string]any)
	}

	doc.Embedding = s.stringToEmbedding(embeddingStr)

	return &doc, nil
}

// UpdateDocument updates a document
func (s *PgVectorStore) UpdateDocument(ctx context.Context, doc Document) error {
	// Regenerate embedding if content changed and we have an embedder
	if s.embedder != nil && len(doc.Embedding) == 0 {
		embedding, err := s.embedder.EmbedQuery(ctx, doc.PageContent)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		doc.Embedding = embedding
	}

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	embeddingStr := s.embeddingToString(doc.Embedding)

	query := fmt.Sprintf(`
		UPDATE %s SET
			content = $2,
			embedding = $3::vector,
			metadata = $4,
			updated_at = NOW()
		WHERE id = $1
	`, s.tableName)

	result, err := s.db.ExecContext(ctx, query, doc.ID, doc.PageContent, embeddingStr, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("document not found")
	}

	return nil
}

// Delete removes documents by their IDs
func (s *PgVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id IN (%s)`,
		s.tableName, strings.Join(placeholders, ", "))

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// Count returns the number of documents
func (s *PgVectorStore) Count(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)

	var count int64
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// Close closes the database connection
func (s *PgVectorStore) Close() error {
	return s.db.Close()
}

// Helper functions

func (s *PgVectorStore) embeddingToString(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}

	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (s *PgVectorStore) stringToEmbedding(str string) []float32 {
	if str == "" {
		return nil
	}

	// Remove brackets
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

func (s *PgVectorStore) buildFilterClause(filters []Filter, startParam int) (string, []interface{}) {
	if len(filters) == 0 {
		return "TRUE", nil
	}

	conditions := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))
	paramNum := startParam

	for _, f := range filters {
		var condition string
		switch f.Operator {
		case FilterEquals:
			condition = fmt.Sprintf("metadata->>'%s' = $%d", f.Field, paramNum)
			args = append(args, f.Value)
		case FilterNotEquals:
			condition = fmt.Sprintf("metadata->>'%s' != $%d", f.Field, paramNum)
			args = append(args, f.Value)
		case FilterContains:
			condition = fmt.Sprintf("metadata->>'%s' LIKE '%%' || $%d || '%%'", f.Field, paramNum)
			args = append(args, f.Value)
		case FilterIn:
			// Handle IN operator
			values, ok := f.Value.([]interface{})
			if ok && len(values) > 0 {
				placeholders := make([]string, len(values))
				for i, v := range values {
					placeholders[i] = fmt.Sprintf("$%d", paramNum+i)
					args = append(args, v)
				}
				condition = fmt.Sprintf("metadata->>'%s' IN (%s)", f.Field, strings.Join(placeholders, ","))
				paramNum += len(values) - 1
			}
		default:
			continue
		}
		conditions = append(conditions, condition)
		paramNum++
	}

	if len(conditions) == 0 {
		return "TRUE", nil
	}

	return strings.Join(conditions, " AND "), args
}

func (s *PgVectorStore) scanSearchResults(rows *sql.Rows) ([]SearchResult, error) {
	results := make([]SearchResult, 0)

	for rows.Next() {
		var doc Document
		var metadataJSON []byte
		var embeddingStr string
		var score float32

		err := rows.Scan(&doc.ID, &doc.PageContent, &metadataJSON, &embeddingStr, &score)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]any)
		}

		doc.Embedding = s.stringToEmbedding(embeddingStr)

		results = append(results, SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	return results, rows.Err()
}

func (s *PgVectorStore) scanDocuments(rows *sql.Rows) ([]Document, error) {
	docs := make([]Document, 0)

	for rows.Next() {
		var doc Document
		var metadataJSON []byte
		var embeddingStr string

		err := rows.Scan(&doc.ID, &doc.PageContent, &metadataJSON, &embeddingStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]any)
		}

		doc.Embedding = s.stringToEmbedding(embeddingStr)
		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	// Fast inverse square root approximation
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// Ensure PgVectorStore implements the interfaces
var _ VectorStore = (*PgVectorStore)(nil)
var _ VectorStoreRetriever = (*PgVectorStore)(nil)
var _ IndexManager = (*PgVectorStore)(nil)
