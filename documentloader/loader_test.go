package documentloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ranganaths/minion/vectorstore"
)

// MockSplitter is a simple mock splitter for testing
type MockSplitter struct {
	chunkSize int
}

func (s *MockSplitter) SplitText(text string) []string {
	if len(text) <= s.chunkSize {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(text); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
	}
	return chunks
}

func (s *MockSplitter) SplitDocuments(docs []vectorstore.Document) []vectorstore.Document {
	var result []vectorstore.Document
	for _, doc := range docs {
		chunks := s.SplitText(doc.PageContent)
		for i, chunk := range chunks {
			newDoc := vectorstore.NewDocumentWithMetadata(chunk, copyMetadata(doc.Metadata))
			newDoc.Metadata["chunk_index"] = i
			result = append(result, newDoc)
		}
	}
	return result
}

func copyMetadata(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// TestTextLoader tests the text file loader
func TestTextLoader(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!\nThis is a test file."
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("Load", func(t *testing.T) {
		loader := NewTextLoader(TextLoaderConfig{Path: filePath})
		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("expected 1 document, got %d", len(docs))
		}

		if docs[0].PageContent != content {
			t.Errorf("expected content '%s', got '%s'", content, docs[0].PageContent)
		}

		if docs[0].Metadata["source"] != filePath {
			t.Errorf("expected source '%s', got '%v'", filePath, docs[0].Metadata["source"])
		}

		if docs[0].Metadata["filename"] != "test.txt" {
			t.Errorf("expected filename 'test.txt', got '%v'", docs[0].Metadata["filename"])
		}
	})

	t.Run("LoadAndSplit", func(t *testing.T) {
		loader := NewTextLoader(TextLoaderConfig{Path: filePath})
		splitter := &MockSplitter{chunkSize: 10}

		docs, err := loader.LoadAndSplit(context.Background(), splitter)
		if err != nil {
			t.Fatalf("failed to load and split: %v", err)
		}

		if len(docs) < 2 {
			t.Errorf("expected multiple documents after split, got %d", len(docs))
		}
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		loader := NewTextLoader(TextLoaderConfig{Path: "/nonexistent/file.txt"})
		_, err := loader.Load(context.Background())
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

// TestDirectoryLoader tests the directory loader
func TestDirectoryLoader(t *testing.T) {
	// Create temp directory with files
	tmpDir := t.TempDir()

	files := map[string]string{
		"file1.txt": "Content of file 1",
		"file2.txt": "Content of file 2",
		"file3.md":  "Markdown content",
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	t.Run("LoadAll", func(t *testing.T) {
		loader := NewDirectoryLoader(DirectoryLoaderConfig{
			Path: tmpDir,
			Glob: "*",
		})

		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) != 3 {
			t.Errorf("expected 3 documents, got %d", len(docs))
		}
	})

	t.Run("LoadWithGlob", func(t *testing.T) {
		loader := NewDirectoryLoader(DirectoryLoaderConfig{
			Path: tmpDir,
			Glob: "*.txt",
		})

		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) != 2 {
			t.Errorf("expected 2 .txt documents, got %d", len(docs))
		}
	})

	t.Run("RecursiveLoad", func(t *testing.T) {
		// Create subdirectory
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.Mkdir(subDir, 0755)
		if err != nil {
			t.Fatalf("failed to create subdir: %v", err)
		}

		subFile := filepath.Join(subDir, "subfile.txt")
		err = os.WriteFile(subFile, []byte("Subdirectory content"), 0644)
		if err != nil {
			t.Fatalf("failed to create subfile: %v", err)
		}

		loader := NewDirectoryLoader(DirectoryLoaderConfig{
			Path:      tmpDir,
			Glob:      "*.txt",
			Recursive: true,
		})

		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) < 3 {
			t.Errorf("expected at least 3 documents with recursive, got %d", len(docs))
		}
	})

	t.Run("NonexistentDirectory", func(t *testing.T) {
		loader := NewDirectoryLoader(DirectoryLoaderConfig{
			Path: "/nonexistent/directory",
		})

		_, err := loader.Load(context.Background())
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		loader := NewDirectoryLoader(DirectoryLoaderConfig{
			Path: tmpDir,
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := loader.Load(ctx)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

// TestReaderLoader tests the reader loader
func TestReaderLoader(t *testing.T) {
	t.Run("Load", func(t *testing.T) {
		content := "Content from reader"
		reader := strings.NewReader(content)

		loader := NewReaderLoader(ReaderLoaderConfig{
			Reader: reader,
			Metadata: map[string]any{
				"source": "stream",
			},
		})

		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("expected 1 document, got %d", len(docs))
		}

		if docs[0].PageContent != content {
			t.Errorf("expected '%s', got '%s'", content, docs[0].PageContent)
		}

		if docs[0].Metadata["source"] != "stream" {
			t.Errorf("expected source 'stream', got '%v'", docs[0].Metadata["source"])
		}
	})

	t.Run("LoadAndSplit", func(t *testing.T) {
		content := "This is a long content that should be split"
		reader := strings.NewReader(content)

		loader := NewReaderLoader(ReaderLoaderConfig{Reader: reader})
		splitter := &MockSplitter{chunkSize: 10}

		docs, err := loader.LoadAndSplit(context.Background(), splitter)
		if err != nil {
			t.Fatalf("failed to load and split: %v", err)
		}

		if len(docs) < 2 {
			t.Errorf("expected multiple documents, got %d", len(docs))
		}
	})
}

// TestStringLoader tests the string loader
func TestStringLoader(t *testing.T) {
	t.Run("Load", func(t *testing.T) {
		content := "String content"
		loader := NewStringLoader(content, map[string]any{"key": "value"})

		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("expected 1 document, got %d", len(docs))
		}

		if docs[0].PageContent != content {
			t.Errorf("expected '%s', got '%s'", content, docs[0].PageContent)
		}

		if docs[0].Metadata["key"] != "value" {
			t.Errorf("expected metadata 'value', got '%v'", docs[0].Metadata["key"])
		}
	})

	t.Run("NilMetadata", func(t *testing.T) {
		loader := NewStringLoader("content", nil)
		docs, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if docs[0].Metadata == nil {
			t.Error("expected non-nil metadata")
		}
	})
}

// TestDocumentMetadata tests the metadata helper
func TestDocumentMetadata(t *testing.T) {
	t.Run("ToMap", func(t *testing.T) {
		meta := DocumentMetadata{
			Source:     "/path/to/file.txt",
			Title:      "Test Document",
			Author:     "Test Author",
			CreatedAt:  "2024-01-01",
			MimeType:   "text/plain",
			Page:       1,
			TotalPages: 10,
		}

		m := meta.ToMap()

		if m["source"] != "/path/to/file.txt" {
			t.Errorf("expected source, got '%v'", m["source"])
		}
		if m["title"] != "Test Document" {
			t.Errorf("expected title, got '%v'", m["title"])
		}
		if m["page"] != 1 {
			t.Errorf("expected page 1, got '%v'", m["page"])
		}
	})

	t.Run("EmptyMetadata", func(t *testing.T) {
		meta := DocumentMetadata{}
		m := meta.ToMap()

		if len(m) != 0 {
			t.Errorf("expected empty map for empty metadata, got %d fields", len(m))
		}
	})
}

// TestDefaultLoaderConfig tests default configuration
func TestDefaultLoaderConfig(t *testing.T) {
	cfg := DefaultLoaderConfig()

	if cfg.Encoding != "utf-8" {
		t.Errorf("expected encoding 'utf-8', got '%s'", cfg.Encoding)
	}

	if cfg.AutodetectEncoding {
		t.Error("expected AutodetectEncoding to be false")
	}
}
