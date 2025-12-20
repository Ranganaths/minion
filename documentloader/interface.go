// Package documentloader provides interfaces and implementations for loading documents from various sources.
// Document loaders convert files and data sources into documents for processing in RAG pipelines.
package documentloader

import (
	"context"

	"github.com/Ranganaths/minion/vectorstore"
)

// Loader is the core interface for document loading
type Loader interface {
	// Load loads documents from the source
	Load(ctx context.Context) ([]vectorstore.Document, error)

	// LoadAndSplit loads documents and splits them using the provided splitter
	LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error)
}

// TextSplitter is the interface for splitting text into chunks
type TextSplitter interface {
	// SplitText splits text into chunks
	SplitText(text string) []string

	// SplitDocuments splits documents into smaller documents
	SplitDocuments(docs []vectorstore.Document) []vectorstore.Document
}

// LoaderConfig holds common configuration for loaders
type LoaderConfig struct {
	// Encoding is the text encoding (default: utf-8)
	Encoding string

	// AutodetectEncoding tries to detect encoding automatically
	AutodetectEncoding bool
}

// DefaultLoaderConfig returns default loader configuration
func DefaultLoaderConfig() LoaderConfig {
	return LoaderConfig{
		Encoding:           "utf-8",
		AutodetectEncoding: false,
	}
}

// BaseLoader provides common functionality for loaders
type BaseLoader struct {
	config LoaderConfig
}

// NewBaseLoader creates a new base loader
func NewBaseLoader(config LoaderConfig) BaseLoader {
	return BaseLoader{config: config}
}

// Config returns the loader configuration
func (l *BaseLoader) Config() LoaderConfig {
	return l.config
}

// LoadAndSplit implements the LoadAndSplit method using the Load method
func (l *BaseLoader) LoadAndSplit(ctx context.Context, loader Loader, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// DocumentMetadata contains common metadata fields
type DocumentMetadata struct {
	Source    string
	Title     string
	Author    string
	CreatedAt string
	MimeType  string
	Page      int
	TotalPages int
}

// ToMap converts metadata to a map
func (m DocumentMetadata) ToMap() map[string]any {
	result := make(map[string]any)
	if m.Source != "" {
		result["source"] = m.Source
	}
	if m.Title != "" {
		result["title"] = m.Title
	}
	if m.Author != "" {
		result["author"] = m.Author
	}
	if m.CreatedAt != "" {
		result["created_at"] = m.CreatedAt
	}
	if m.MimeType != "" {
		result["mime_type"] = m.MimeType
	}
	if m.Page > 0 {
		result["page"] = m.Page
	}
	if m.TotalPages > 0 {
		result["total_pages"] = m.TotalPages
	}
	return result
}
