package retriever

import (
	"context"
	"fmt"

	"github.com/Ranganaths/minion/vectorstore"
)

// VectorStoreRetriever retrieves documents from a vector store
type VectorStoreRetriever struct {
	vectorStore vectorstore.VectorStore
	config      RetrieverConfig

	// MMR configuration
	fetchK float32
	lambda float32
}

// VectorStoreRetrieverConfig configures the vector store retriever
type VectorStoreRetrieverConfig struct {
	// VectorStore is the vector store to search
	VectorStore vectorstore.VectorStore

	// K is the number of documents to retrieve
	K int

	// SearchType is the type of search to perform
	SearchType SearchType

	// ScoreThreshold filters results below this score
	ScoreThreshold float32

	// Filters are metadata filters to apply
	Filters []vectorstore.Filter

	// FetchK is the number of documents to fetch for MMR (multiplier of K)
	FetchK float32

	// Lambda is the diversity parameter for MMR (0=max diversity, 1=max relevance)
	Lambda float32
}

// NewVectorStoreRetriever creates a new vector store retriever
func NewVectorStoreRetriever(cfg VectorStoreRetrieverConfig) (*VectorStoreRetriever, error) {
	if cfg.VectorStore == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	config := DefaultRetrieverConfig()
	if cfg.K > 0 {
		config.K = cfg.K
	}
	if cfg.SearchType != "" {
		config.SearchType = cfg.SearchType
	}
	if cfg.ScoreThreshold > 0 {
		config.ScoreThreshold = cfg.ScoreThreshold
	}
	config.Filters = cfg.Filters

	fetchK := cfg.FetchK
	if fetchK <= 0 {
		fetchK = 2.0 // Default to 2x K
	}

	lambda := cfg.Lambda
	if lambda <= 0 {
		lambda = 0.5 // Default to balanced
	}

	return &VectorStoreRetriever{
		vectorStore: cfg.VectorStore,
		config:      config,
		fetchK:      fetchK,
		lambda:      lambda,
	}, nil
}

// GetRelevantDocuments retrieves documents relevant to a query
func (r *VectorStoreRetriever) GetRelevantDocuments(ctx context.Context, query string) ([]vectorstore.Document, error) {
	switch r.config.SearchType {
	case SearchTypeMMR:
		return r.mmrSearch(ctx, query)
	default:
		return r.similaritySearch(ctx, query)
	}
}

// similaritySearch performs a similarity search
func (r *VectorStoreRetriever) similaritySearch(ctx context.Context, query string) ([]vectorstore.Document, error) {
	// Check if vector store supports filtered search
	if len(r.config.Filters) > 0 {
		if memStore, ok := r.vectorStore.(*vectorstore.MemoryVectorStore); ok {
			results, err := memStore.SearchWithFilter(ctx, query, r.config.K, r.config.Filters)
			if err != nil {
				return nil, err
			}
			return r.filterByScore(results), nil
		}
	}

	// Use regular similarity search
	if r.config.ScoreThreshold > 0 {
		results, err := r.vectorStore.SimilaritySearchWithScore(ctx, query, r.config.K)
		if err != nil {
			return nil, err
		}
		return r.filterByScore(results), nil
	}

	return r.vectorStore.SimilaritySearch(ctx, query, r.config.K)
}

// mmrSearch performs a Max Marginal Relevance search
func (r *VectorStoreRetriever) mmrSearch(ctx context.Context, query string) ([]vectorstore.Document, error) {
	// Check if vector store supports MMR
	if mmrStore, ok := r.vectorStore.(vectorstore.VectorStoreRetriever); ok {
		fetchK := int(float32(r.config.K) * r.fetchK)
		return mmrStore.MaxMarginalRelevanceSearch(ctx, query, r.config.K, fetchK, r.lambda)
	}

	// Fallback to regular similarity search
	return r.similaritySearch(ctx, query)
}

// filterByScore filters results by score threshold
func (r *VectorStoreRetriever) filterByScore(results []vectorstore.SearchResult) []vectorstore.Document {
	var filtered []vectorstore.Document
	for _, result := range results {
		if result.Score >= r.config.ScoreThreshold {
			filtered = append(filtered, result.Document)
		}
	}
	return filtered
}

// K returns the number of documents to retrieve
func (r *VectorStoreRetriever) K() int {
	return r.config.K
}

// SetK updates the number of documents to retrieve
func (r *VectorStoreRetriever) SetK(k int) {
	r.config.K = k
}

// SearchType returns the current search type
func (r *VectorStoreRetriever) SearchType() SearchType {
	return r.config.SearchType
}

// SetSearchType updates the search type
func (r *VectorStoreRetriever) SetSearchType(searchType SearchType) {
	r.config.SearchType = searchType
}

// SetScoreThreshold updates the score threshold
func (r *VectorStoreRetriever) SetScoreThreshold(threshold float32) {
	r.config.ScoreThreshold = threshold
}

// SetFilters updates the metadata filters
func (r *VectorStoreRetriever) SetFilters(filters []vectorstore.Filter) {
	r.config.Filters = filters
}
