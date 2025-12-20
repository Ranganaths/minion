// Package retriever provides interfaces and implementations for document retrieval.
// Retrievers are used to fetch relevant documents from various sources for RAG applications.
package retriever

import (
	"context"

	"github.com/Ranganaths/minion/vectorstore"
)

// Retriever is the core interface for retrieving documents
type Retriever interface {
	// GetRelevantDocuments retrieves documents relevant to a query
	GetRelevantDocuments(ctx context.Context, query string) ([]vectorstore.Document, error)
}

// SearchType represents the type of search to perform
type SearchType string

const (
	// SearchTypeSimilarity uses pure similarity search
	SearchTypeSimilarity SearchType = "similarity"

	// SearchTypeMMR uses Max Marginal Relevance for diversity
	SearchTypeMMR SearchType = "mmr"
)

// RetrieverConfig holds common configuration for retrievers
type RetrieverConfig struct {
	// K is the number of documents to retrieve
	K int

	// SearchType is the type of search to perform
	SearchType SearchType

	// ScoreThreshold filters results below this score
	ScoreThreshold float32

	// Filters are metadata filters to apply
	Filters []vectorstore.Filter
}

// DefaultRetrieverConfig returns default retriever configuration
func DefaultRetrieverConfig() RetrieverConfig {
	return RetrieverConfig{
		K:          4,
		SearchType: SearchTypeSimilarity,
	}
}
