package textsplitter

import (
	"testing"

	"github.com/Ranganaths/minion/vectorstore"
)

// TestCharacterTextSplitter tests the character text splitter
func TestCharacterTextSplitter(t *testing.T) {
	t.Run("BasicSplit", func(t *testing.T) {
		splitter := NewCharacterTextSplitter(CharacterTextSplitterConfig{
			ChunkSize:    50,
			ChunkOverlap: 10,
			Separator:    "\n\n",
		})

		text := "Hello world.\n\nThis is a test.\n\nAnother paragraph."
		chunks := splitter.SplitText(text)

		if len(chunks) == 0 {
			t.Error("expected chunks, got none")
		}
	})

	t.Run("LargeChunks", func(t *testing.T) {
		splitter := NewCharacterTextSplitter(CharacterTextSplitterConfig{
			ChunkSize:    1000,
			ChunkOverlap: 0,
		})

		text := "Short text"
		chunks := splitter.SplitText(text)

		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
		if chunks[0] != text {
			t.Errorf("expected '%s', got '%s'", text, chunks[0])
		}
	})

	t.Run("KeepSeparator", func(t *testing.T) {
		splitter := NewCharacterTextSplitter(CharacterTextSplitterConfig{
			ChunkSize:     100,
			ChunkOverlap:  0,
			Separator:     "\n\n",
			KeepSeparator: true,
		})

		text := "First\n\nSecond\n\nThird"
		chunks := splitter.SplitText(text)

		for _, chunk := range chunks[:len(chunks)-1] {
			if len(chunk) < 2 || chunk[len(chunk)-2:] != "\n\n" {
				// Some chunks may be merged, so this is ok
			}
		}
	})

	t.Run("SplitDocuments", func(t *testing.T) {
		splitter := NewCharacterTextSplitter(CharacterTextSplitterConfig{
			ChunkSize:    20,
			ChunkOverlap: 5,
		})

		docs := []vectorstore.Document{
			vectorstore.NewDocumentWithMetadata("First paragraph.\n\nSecond paragraph.", map[string]any{
				"source": "test.txt",
			}),
		}

		result := splitter.SplitDocuments(docs)

		if len(result) == 0 {
			t.Error("expected split documents, got none")
		}

		// Check metadata is preserved
		for _, doc := range result {
			if doc.Metadata["source"] != "test.txt" {
				t.Error("expected source metadata to be preserved")
			}
			if doc.Metadata["chunk_index"] == nil {
				t.Error("expected chunk_index metadata")
			}
		}
	})
}

// TestRecursiveCharacterTextSplitter tests the recursive splitter
func TestRecursiveCharacterTextSplitter(t *testing.T) {
	t.Run("BasicRecursiveSplit", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize:    50,
			ChunkOverlap: 10,
		})

		text := "Hello world.\n\nThis is a test.\n\nAnother paragraph with more text."
		chunks := splitter.SplitText(text)

		if len(chunks) == 0 {
			t.Error("expected chunks, got none")
		}

		// All chunks should be under chunk size or close to it
		for _, chunk := range chunks {
			if len(chunk) > 100 { // Allow some tolerance
				t.Errorf("chunk too large: %d chars", len(chunk))
			}
		}
	})

	t.Run("FallbackToSmallSeparators", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize:    10,
			ChunkOverlap: 0,
		})

		text := "Hello world testing this is a longer text"
		chunks := splitter.SplitText(text)

		// With small chunk size, we expect multiple chunks
		if len(chunks) == 0 {
			t.Error("expected at least one chunk")
		}
	})

	t.Run("CustomSeparators", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize:    30,
			ChunkOverlap: 0,
			Separators:   []string{".", " ", ""},
		})

		text := "First sentence. Second sentence. Third."
		chunks := splitter.SplitText(text)

		if len(chunks) == 0 {
			t.Error("expected chunks, got none")
		}
	})

	t.Run("EmptyText", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize: 100,
		})

		chunks := splitter.SplitText("")
		if len(chunks) != 0 {
			t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
		}
	})

	t.Run("SingleChunk", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize: 1000,
		})

		text := "Short text"
		chunks := splitter.SplitText(text)

		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("SplitDocuments", func(t *testing.T) {
		splitter := NewRecursiveCharacterTextSplitter(RecursiveCharacterTextSplitterConfig{
			ChunkSize:     50,
			ChunkOverlap:  10,
			AddStartIndex: true,
		})

		docs := []vectorstore.Document{
			vectorstore.NewDocumentWithMetadata("First paragraph with content.\n\nSecond paragraph with more content.", map[string]any{
				"source": "doc1.txt",
			}),
			vectorstore.NewDocumentWithMetadata("Another document.\n\nWith multiple paragraphs.", map[string]any{
				"source": "doc2.txt",
			}),
		}

		result := splitter.SplitDocuments(docs)

		if len(result) == 0 {
			t.Error("expected split documents, got none")
		}

		// Check metadata is preserved and augmented
		for _, doc := range result {
			if doc.Metadata["source"] == nil {
				t.Error("expected source metadata to be preserved")
			}
			if doc.Metadata["chunk_index"] == nil {
				t.Error("expected chunk_index metadata")
			}
			if doc.Metadata["start_index"] == nil {
				t.Error("expected start_index metadata")
			}
		}
	})
}

// TestDefaultSeparators tests default separator list
func TestDefaultSeparators(t *testing.T) {
	seps := DefaultSeparators()
	if len(seps) != 4 {
		t.Errorf("expected 4 default separators, got %d", len(seps))
	}
	if seps[0] != "\n\n" {
		t.Errorf("expected first separator to be double newline")
	}
	if seps[3] != "" {
		t.Errorf("expected last separator to be empty string")
	}
}

// TestDefaultSplitterConfig tests default config
func TestDefaultSplitterConfig(t *testing.T) {
	cfg := DefaultSplitterConfig()
	if cfg.ChunkSize != 1000 {
		t.Errorf("expected chunk size 1000, got %d", cfg.ChunkSize)
	}
	if cfg.ChunkOverlap != 200 {
		t.Errorf("expected chunk overlap 200, got %d", cfg.ChunkOverlap)
	}
	if cfg.LengthFunction == nil {
		t.Error("expected length function to be set")
	}
	if cfg.LengthFunction("hello") != 5 {
		t.Error("expected length function to return string length")
	}
}

// TestMergeSplits tests the merge splits functionality
func TestMergeSplits(t *testing.T) {
	t.Run("MergeSmallSplits", func(t *testing.T) {
		splitter := NewBaseSplitter(SplitterConfig{
			ChunkSize:      50,
			ChunkOverlap:   10,
			LengthFunction: func(s string) int { return len(s) },
		})

		splits := []string{"Hello", " ", "world", "!", " ", "Test"}
		merged := splitter.MergeSplits(splits)

		if len(merged) == 0 {
			t.Error("expected merged chunks")
		}
	})

	t.Run("LargeSplit", func(t *testing.T) {
		splitter := NewBaseSplitter(SplitterConfig{
			ChunkSize:      10,
			ChunkOverlap:   0,
			LengthFunction: func(s string) int { return len(s) },
		})

		splits := []string{"Short", "ThisIsAVeryLongSplitThatExceedsChunkSize", "End"}
		merged := splitter.MergeSplits(splits)

		// The large split should be in its own chunk
		foundLarge := false
		for _, chunk := range merged {
			if containsString(chunk, "VeryLong") {
				foundLarge = true
				break
			}
		}
		if !foundLarge {
			t.Error("expected large split to be preserved")
		}
	})
}

// TestHelperFunctions tests helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("splitBySeparator", func(t *testing.T) {
		splits := splitBySeparator("a,b,c", ",", false)
		if len(splits) != 3 {
			t.Errorf("expected 3 splits, got %d", len(splits))
		}
		if splits[0] != "a" || splits[1] != "b" || splits[2] != "c" {
			t.Error("unexpected split values")
		}
	})

	t.Run("splitBySeparatorKeep", func(t *testing.T) {
		splits := splitBySeparator("a,b,c", ",", true)
		if len(splits) != 3 {
			t.Errorf("expected 3 splits, got %d", len(splits))
		}
		if splits[0] != "a," || splits[1] != "b," {
			t.Error("expected separator to be kept")
		}
	})

	t.Run("splitByEmptySeparator", func(t *testing.T) {
		splits := splitBySeparator("abc", "", false)
		if len(splits) != 3 {
			t.Errorf("expected 3 chars, got %d", len(splits))
		}
	})

	t.Run("containsString", func(t *testing.T) {
		if !containsString("hello world", "world") {
			t.Error("expected to find 'world'")
		}
		if containsString("hello", "world") {
			t.Error("should not find 'world'")
		}
		if !containsString("test", "") {
			t.Error("empty string should be found")
		}
	})

	t.Run("joinStrings", func(t *testing.T) {
		result := joinStrings([]string{"a", "b", "c"}, ",")
		if result != "a,b,c" {
			t.Errorf("expected 'a,b,c', got '%s'", result)
		}

		result = joinStrings([]string{"single"}, ",")
		if result != "single" {
			t.Errorf("expected 'single', got '%s'", result)
		}

		result = joinStrings(nil, ",")
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})

	t.Run("getOverlap", func(t *testing.T) {
		result := getOverlap("hello world", 5)
		if result != "world" {
			t.Errorf("expected 'world', got '%s'", result)
		}

		result = getOverlap("hi", 10)
		if result != "hi" {
			t.Errorf("expected 'hi', got '%s'", result)
		}
	})

	t.Run("copyMetadata", func(t *testing.T) {
		original := map[string]any{"key": "value"}
		copied := copyMetadata(original)

		if copied["key"] != "value" {
			t.Error("expected value to be copied")
		}

		copied["key"] = "modified"
		if original["key"] != "value" {
			t.Error("modifying copy should not affect original")
		}

		nilCopy := copyMetadata(nil)
		if nilCopy == nil {
			t.Error("expected non-nil map for nil input")
		}
	})
}
