// Package textsplitter provides text splitting utilities for chunking documents.
// Text splitters break large documents into smaller chunks for embedding and retrieval.
package textsplitter

import (
	"fmt"

	"github.com/Ranganaths/minion/vectorstore"
)

// TextSplitter is the core interface for splitting text
type TextSplitter interface {
	// SplitText splits text into chunks
	SplitText(text string) []string

	// SplitDocuments splits documents into smaller documents
	SplitDocuments(docs []vectorstore.Document) []vectorstore.Document
}

// SplitterConfig holds common configuration for splitters
type SplitterConfig struct {
	// ChunkSize is the target size of each chunk
	ChunkSize int

	// ChunkOverlap is the overlap between chunks
	ChunkOverlap int

	// LengthFunction calculates the length of text (default: len)
	LengthFunction func(string) int

	// KeepSeparator keeps the separator in chunks
	KeepSeparator bool

	// AddStartIndex adds start index to metadata
	AddStartIndex bool
}

// DefaultSplitterConfig returns default splitter configuration
func DefaultSplitterConfig() SplitterConfig {
	return SplitterConfig{
		ChunkSize:      1000,
		ChunkOverlap:   200,
		LengthFunction: func(s string) int { return len(s) },
		KeepSeparator:  false,
		AddStartIndex:  false,
	}
}

// ValidateSplitterConfig validates splitter configuration.
// Returns an error if the configuration is invalid.
func ValidateSplitterConfig(config SplitterConfig) error {
	if config.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive, got %d", config.ChunkSize)
	}
	if config.ChunkOverlap < 0 {
		return fmt.Errorf("chunk overlap must be non-negative, got %d", config.ChunkOverlap)
	}
	if config.ChunkOverlap >= config.ChunkSize {
		return fmt.Errorf("chunk overlap (%d) must be less than chunk size (%d)", config.ChunkOverlap, config.ChunkSize)
	}
	return nil
}

// BaseSplitter provides common functionality for splitters
type BaseSplitter struct {
	config SplitterConfig
}

// NewBaseSplitter creates a new base splitter
func NewBaseSplitter(config SplitterConfig) BaseSplitter {
	if config.LengthFunction == nil {
		config.LengthFunction = func(s string) int { return len(s) }
	}
	return BaseSplitter{config: config}
}

// Config returns the splitter configuration
func (s *BaseSplitter) Config() SplitterConfig {
	return s.config
}

// SplitDocuments splits documents using the provided split function
func (s *BaseSplitter) SplitDocuments(docs []vectorstore.Document, splitFunc func(string) []string) []vectorstore.Document {
	var result []vectorstore.Document

	for _, doc := range docs {
		chunks := splitFunc(doc.PageContent)
		for i, chunk := range chunks {
			newDoc := vectorstore.NewDocumentWithMetadata(chunk, copyMetadata(doc.Metadata))
			newDoc.Metadata["chunk_index"] = i
			newDoc.Metadata["total_chunks"] = len(chunks)

			if s.config.AddStartIndex {
				// Calculate approximate start index
				startIdx := 0
				for j := 0; j < i; j++ {
					startIdx += s.config.LengthFunction(chunks[j])
					if j > 0 {
						startIdx -= s.config.ChunkOverlap
					}
				}
				newDoc.Metadata["start_index"] = startIdx
			}

			result = append(result, newDoc)
		}
	}

	return result
}

// MergeSplits merges small splits into larger chunks
func (s *BaseSplitter) MergeSplits(splits []string) []string {
	if len(splits) == 0 {
		return nil
	}

	var result []string
	var currentChunk string

	for _, split := range splits {
		splitLen := s.config.LengthFunction(split)

		if splitLen > s.config.ChunkSize {
			// Split is too large, add current chunk and this one separately
			if currentChunk != "" {
				result = append(result, currentChunk)
				currentChunk = ""
			}
			result = append(result, split)
			continue
		}

		if currentChunk == "" {
			currentChunk = split
			continue
		}

		// Check if adding this split would exceed chunk size
		if s.config.LengthFunction(currentChunk)+splitLen > s.config.ChunkSize {
			result = append(result, currentChunk)

			// Start new chunk with overlap
			if s.config.ChunkOverlap > 0 {
				overlap := getOverlap(currentChunk, s.config.ChunkOverlap)
				currentChunk = overlap + split
			} else {
				currentChunk = split
			}
		} else {
			currentChunk += split
		}
	}

	if currentChunk != "" {
		result = append(result, currentChunk)
	}

	return result
}

// copyMetadata creates a shallow copy of metadata
func copyMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(metadata))
	for k, v := range metadata {
		result[k] = v
	}
	return result
}

// getOverlap returns the last n characters of a string
func getOverlap(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
