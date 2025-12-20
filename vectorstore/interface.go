// Package vectorstore provides interfaces and implementations for vector storage and similarity search.
// Vector stores are used to store embeddings and perform efficient similarity search for RAG applications.
package vectorstore

import (
	"context"

	"github.com/Ranganaths/minion/embeddings"
)

// VectorStore is the core interface for vector storage and retrieval
type VectorStore interface {
	// AddDocuments adds documents to the vector store
	AddDocuments(ctx context.Context, docs []Document) ([]string, error)

	// SimilaritySearch finds the most similar documents to a query
	SimilaritySearch(ctx context.Context, query string, k int) ([]Document, error)

	// SimilaritySearchWithScore returns documents with their similarity scores
	SimilaritySearchWithScore(ctx context.Context, query string, k int) ([]SearchResult, error)

	// Delete removes documents by their IDs
	Delete(ctx context.Context, ids []string) error
}

// VectorStoreRetriever extends VectorStore with retriever capabilities
type VectorStoreRetriever interface {
	VectorStore

	// SimilaritySearchByVector searches using a pre-computed embedding
	SimilaritySearchByVector(ctx context.Context, embedding []float32, k int) ([]Document, error)

	// MaxMarginalRelevanceSearch performs MMR search for diversity
	MaxMarginalRelevanceSearch(ctx context.Context, query string, k int, fetchK int, lambda float32) ([]Document, error)
}

// Document represents a document with content and metadata
type Document struct {
	// ID is the unique identifier for the document
	ID string

	// PageContent is the main text content
	PageContent string

	// Metadata contains additional information about the document
	Metadata map[string]any

	// Embedding is the vector representation (optional, computed if not provided)
	Embedding []float32
}

// SearchResult contains a document and its similarity score
type SearchResult struct {
	// Document is the matched document
	Document Document

	// Score is the similarity score (higher is more similar)
	Score float32
}

// VectorStoreConfig holds common configuration for vector stores
type VectorStoreConfig struct {
	// Embedder is the embedding provider
	Embedder embeddings.Embedder

	// CollectionName is the name of the collection/index
	CollectionName string

	// DistanceMetric is the distance metric to use
	DistanceMetric DistanceMetric
}

// DistanceMetric represents the distance/similarity metric
type DistanceMetric string

const (
	// DistanceCosine uses cosine similarity
	DistanceCosine DistanceMetric = "cosine"

	// DistanceEuclidean uses Euclidean distance
	DistanceEuclidean DistanceMetric = "euclidean"

	// DistanceDotProduct uses dot product similarity
	DistanceDotProduct DistanceMetric = "dot_product"
)

// Filter represents a metadata filter for searches
type Filter struct {
	// Field is the metadata field to filter on
	Field string

	// Operator is the comparison operator
	Operator FilterOperator

	// Value is the value to compare against
	Value any
}

// FilterOperator represents a filter comparison operator
type FilterOperator string

const (
	FilterEquals      FilterOperator = "eq"
	FilterNotEquals   FilterOperator = "ne"
	FilterGreaterThan FilterOperator = "gt"
	FilterLessThan    FilterOperator = "lt"
	FilterIn          FilterOperator = "in"
	FilterContains    FilterOperator = "contains"
)

// SearchOptions configures a similarity search
type SearchOptions struct {
	// K is the number of results to return
	K int

	// ScoreThreshold filters results below this score
	ScoreThreshold float32

	// Filters are metadata filters to apply
	Filters []Filter

	// IncludeMetadata includes metadata in results
	IncludeMetadata bool

	// IncludeEmbeddings includes embeddings in results
	IncludeEmbeddings bool
}

// DefaultSearchOptions returns default search options
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		K:               4,
		ScoreThreshold:  0.0,
		IncludeMetadata: true,
	}
}

// NewDocument creates a new document with the given content
func NewDocument(content string) Document {
	return Document{
		PageContent: content,
		Metadata:    make(map[string]any),
	}
}

// NewDocumentWithMetadata creates a new document with content and metadata
func NewDocumentWithMetadata(content string, metadata map[string]any) Document {
	return Document{
		PageContent: content,
		Metadata:    metadata,
	}
}

// WithID sets the document ID
func (d Document) WithID(id string) Document {
	d.ID = id
	return d
}

// WithMetadata adds metadata to the document
func (d Document) WithMetadata(key string, value any) Document {
	if d.Metadata == nil {
		d.Metadata = make(map[string]any)
	}
	d.Metadata[key] = value
	return d
}

// GetMetadata retrieves a metadata value
func (d Document) GetMetadata(key string) (any, bool) {
	if d.Metadata == nil {
		return nil, false
	}
	val, ok := d.Metadata[key]
	return val, ok
}

// GetMetadataString retrieves a metadata value as a string
func (d Document) GetMetadataString(key string) string {
	val, ok := d.GetMetadata(key)
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// Clone creates a deep copy of the document
func (d Document) Clone() Document {
	clone := Document{
		ID:          d.ID,
		PageContent: d.PageContent,
	}

	// Deep copy metadata
	if d.Metadata != nil {
		clone.Metadata = make(map[string]any, len(d.Metadata))
		for k, v := range d.Metadata {
			clone.Metadata[k] = v
		}
	}

	// Deep copy embedding
	if d.Embedding != nil {
		clone.Embedding = make([]float32, len(d.Embedding))
		copy(clone.Embedding, d.Embedding)
	}

	return clone
}
