// Package embeddings provides interfaces and implementations for text embeddings.
// Embeddings convert text into dense vector representations for semantic search and similarity.
package embeddings

import (
	"context"
)

// Embedder is the core interface for generating text embeddings
type Embedder interface {
	// EmbedQuery embeds a single query text
	EmbedQuery(ctx context.Context, text string) ([]float32, error)

	// EmbedDocuments embeds multiple document texts
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension size
	Dimension() int
}

// BatchEmbedder extends Embedder with batch processing capabilities
type BatchEmbedder interface {
	Embedder

	// EmbedBatch embeds texts in configurable batches
	EmbedBatch(ctx context.Context, texts []string, batchSize int) ([][]float32, error)
}

// EmbeddingResult contains an embedding with metadata
type EmbeddingResult struct {
	// Embedding is the vector representation
	Embedding []float32

	// Text is the original text (optional)
	Text string

	// Index is the position in the original batch
	Index int

	// TokenCount is the number of tokens in the text (if available)
	TokenCount int
}

// EmbedderConfig holds common configuration for embedders
type EmbedderConfig struct {
	// Model is the embedding model name
	Model string

	// BatchSize is the default batch size for document embedding
	BatchSize int

	// MaxRetries is the number of retries on failure
	MaxRetries int

	// Timeout is the timeout for embedding requests (in seconds)
	Timeout int
}

// DefaultEmbedderConfig returns default configuration
func DefaultEmbedderConfig() EmbedderConfig {
	return EmbedderConfig{
		BatchSize:  100,
		MaxRetries: 3,
		Timeout:    60,
	}
}

// CosineSimilarity calculates cosine similarity between two embeddings
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
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

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// EuclideanDistance calculates Euclidean distance between two embeddings
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return sqrt(sum)
}

// DotProduct calculates dot product between two embeddings
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}

	return sum
}

// sqrt calculates square root using Newton's method
func sqrt(x float32) float32 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// NormalizeEmbedding normalizes an embedding to unit length
func NormalizeEmbedding(embedding []float32) []float32 {
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	norm = sqrt(norm)

	if norm == 0 {
		return embedding
	}

	result := make([]float32, len(embedding))
	for i, v := range embedding {
		result[i] = v / norm
	}
	return result
}

// AverageEmbeddings calculates the average of multiple embeddings
func AverageEmbeddings(embeddings [][]float32) []float32 {
	if len(embeddings) == 0 {
		return nil
	}

	dim := len(embeddings[0])
	result := make([]float32, dim)

	for _, emb := range embeddings {
		for i, v := range emb {
			result[i] += v
		}
	}

	n := float32(len(embeddings))
	for i := range result {
		result[i] /= n
	}

	return result
}

// FindMostSimilar finds the most similar embedding to a query from a set of candidates
func FindMostSimilar(query []float32, candidates [][]float32) (int, float32) {
	if len(candidates) == 0 {
		return -1, 0
	}

	bestIdx := 0
	bestScore := CosineSimilarity(query, candidates[0])

	for i := 1; i < len(candidates); i++ {
		score := CosineSimilarity(query, candidates[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx, bestScore
}

// TopKSimilar finds the top K most similar embeddings
type SimilarityResult struct {
	Index      int
	Similarity float32
}

func TopKSimilar(query []float32, candidates [][]float32, k int) []SimilarityResult {
	if len(candidates) == 0 || k <= 0 {
		return nil
	}

	// Calculate all similarities
	results := make([]SimilarityResult, len(candidates))
	for i, candidate := range candidates {
		results[i] = SimilarityResult{
			Index:      i,
			Similarity: CosineSimilarity(query, candidate),
		}
	}

	// Sort by similarity (descending) using simple bubble sort for small k
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if k > len(results) {
		k = len(results)
	}

	return results[:k]
}
