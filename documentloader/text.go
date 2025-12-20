package documentloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Ranganaths/minion/vectorstore"
)

// DefaultMaxFileSize is the default maximum file size (100MB)
const DefaultMaxFileSize = 100 * 1024 * 1024

// TextLoader loads documents from text files
type TextLoader struct {
	BaseLoader
	path        string
	maxFileSize int64
}

// TextLoaderConfig configures the text loader
type TextLoaderConfig struct {
	// Path is the path to the text file
	Path string

	// Encoding is the text encoding
	Encoding string

	// MaxFileSize is the maximum file size in bytes (default: 100MB)
	// Set to 0 for no limit (not recommended)
	MaxFileSize int64
}

// NewTextLoader creates a new text loader
func NewTextLoader(cfg TextLoaderConfig) *TextLoader {
	loaderCfg := DefaultLoaderConfig()
	if cfg.Encoding != "" {
		loaderCfg.Encoding = cfg.Encoding
	}

	maxFileSize := cfg.MaxFileSize
	if maxFileSize == 0 {
		maxFileSize = DefaultMaxFileSize
	}

	return &TextLoader{
		BaseLoader:  NewBaseLoader(loaderCfg),
		path:        cfg.Path,
		maxFileSize: maxFileSize,
	}
}

// Load loads the text file as a document.
// Returns an error if the file exceeds the maximum file size.
func (l *TextLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	// Check file size before reading
	if l.maxFileSize > 0 {
		info, err := os.Stat(l.path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}
		if info.Size() > l.maxFileSize {
			return nil, fmt.Errorf("file size %d bytes exceeds limit of %d bytes", info.Size(), l.maxFileSize)
		}
	}

	content, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	doc := vectorstore.NewDocumentWithMetadata(string(content), map[string]any{
		"source":   l.path,
		"filename": filepath.Base(l.path),
	})

	return []vectorstore.Document{doc}, nil
}

// LoadAndSplit loads and splits the text file
func (l *TextLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// DirectoryLoader loads documents from a directory
type DirectoryLoader struct {
	BaseLoader
	path       string
	glob       string
	recursive  bool
	loaderFunc func(path string) Loader
}

// DirectoryLoaderConfig configures the directory loader
type DirectoryLoaderConfig struct {
	// Path is the directory path
	Path string

	// Glob is a glob pattern for matching files (e.g., "*.txt")
	Glob string

	// Recursive searches subdirectories
	Recursive bool

	// LoaderFunc creates a loader for each file
	LoaderFunc func(path string) Loader
}

// NewDirectoryLoader creates a new directory loader
func NewDirectoryLoader(cfg DirectoryLoaderConfig) *DirectoryLoader {
	loaderFunc := cfg.LoaderFunc
	if loaderFunc == nil {
		// Default to text loader
		loaderFunc = func(path string) Loader {
			return NewTextLoader(TextLoaderConfig{Path: path})
		}
	}

	glob := cfg.Glob
	if glob == "" {
		glob = "*"
	}

	return &DirectoryLoader{
		BaseLoader: NewBaseLoader(DefaultLoaderConfig()),
		path:       cfg.Path,
		glob:       glob,
		recursive:  cfg.Recursive,
		loaderFunc: loaderFunc,
	}
}

// Load loads all matching documents from the directory
func (l *DirectoryLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	var allDocs []vectorstore.Document

	err := l.walkDir(ctx, l.path, func(path string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		loader := l.loaderFunc(path)
		docs, err := loader.Load(ctx)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", path, err)
		}
		allDocs = append(allDocs, docs...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return allDocs, nil
}

// LoadAndSplit loads and splits all documents from the directory
func (l *DirectoryLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// walkDir walks the directory and calls fn for each matching file
func (l *DirectoryLoader) walkDir(ctx context.Context, dir string, fn func(path string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			if l.recursive {
				if err := l.walkDir(ctx, path, fn); err != nil {
					return err
				}
			}
			continue
		}

		// Check if file matches glob
		matched, err := filepath.Match(l.glob, entry.Name())
		if err != nil {
			return fmt.Errorf("invalid glob pattern: %w", err)
		}
		if !matched {
			continue
		}

		if err := fn(path); err != nil {
			return err
		}
	}

	return nil
}

// ReaderLoader loads documents from an io.Reader
type ReaderLoader struct {
	BaseLoader
	reader   io.Reader
	metadata map[string]any
}

// ReaderLoaderConfig configures the reader loader
type ReaderLoaderConfig struct {
	// Reader is the source reader
	Reader io.Reader

	// Metadata is optional metadata to attach
	Metadata map[string]any
}

// NewReaderLoader creates a new reader loader
func NewReaderLoader(cfg ReaderLoaderConfig) *ReaderLoader {
	metadata := cfg.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}

	return &ReaderLoader{
		BaseLoader: NewBaseLoader(DefaultLoaderConfig()),
		reader:     cfg.Reader,
		metadata:   metadata,
	}
}

// Load loads the content from the reader
func (l *ReaderLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	content, err := io.ReadAll(l.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	doc := vectorstore.NewDocumentWithMetadata(string(content), l.metadata)
	return []vectorstore.Document{doc}, nil
}

// LoadAndSplit loads and splits the content
func (l *ReaderLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// StringLoader loads a document from a string
type StringLoader struct {
	BaseLoader
	content  string
	metadata map[string]any
}

// NewStringLoader creates a loader from a string
func NewStringLoader(content string, metadata map[string]any) *StringLoader {
	if metadata == nil {
		metadata = make(map[string]any)
	}

	return &StringLoader{
		BaseLoader: NewBaseLoader(DefaultLoaderConfig()),
		content:    content,
		metadata:   metadata,
	}
}

// Load returns the string as a document
func (l *StringLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	doc := vectorstore.NewDocumentWithMetadata(l.content, l.metadata)
	return []vectorstore.Document{doc}, nil
}

// LoadAndSplit loads and splits the string
func (l *StringLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}
