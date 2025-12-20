package textsplitter

import (
	"log"

	"github.com/Ranganaths/minion/vectorstore"
)

// CharacterTextSplitter splits text by character count
type CharacterTextSplitter struct {
	BaseSplitter
	separator string
}

// CharacterTextSplitterConfig configures the character text splitter
type CharacterTextSplitterConfig struct {
	// ChunkSize is the maximum size of each chunk
	ChunkSize int

	// ChunkOverlap is the overlap between chunks
	ChunkOverlap int

	// Separator is the string to split on (default: "\n\n")
	Separator string

	// KeepSeparator keeps the separator in chunks
	KeepSeparator bool

	// AddStartIndex adds start index to metadata
	AddStartIndex bool
}

// NewCharacterTextSplitter creates a new character text splitter.
// If configuration is invalid, defaults will be used and a warning logged.
func NewCharacterTextSplitter(cfg CharacterTextSplitterConfig) *CharacterTextSplitter {
	config := DefaultSplitterConfig()
	if cfg.ChunkSize > 0 {
		config.ChunkSize = cfg.ChunkSize
	}
	if cfg.ChunkOverlap >= 0 {
		config.ChunkOverlap = cfg.ChunkOverlap
	}
	config.KeepSeparator = cfg.KeepSeparator
	config.AddStartIndex = cfg.AddStartIndex

	// Validate configuration
	if err := ValidateSplitterConfig(config); err != nil {
		log.Printf("textsplitter: invalid configuration, using defaults: %v", err)
		config = DefaultSplitterConfig()
	}

	separator := cfg.Separator
	if separator == "" {
		separator = "\n\n"
	}

	return &CharacterTextSplitter{
		BaseSplitter: NewBaseSplitter(config),
		separator:    separator,
	}
}

// SplitText splits text into chunks
func (s *CharacterTextSplitter) SplitText(text string) []string {
	// First split by separator
	splits := splitBySeparator(text, s.separator, s.config.KeepSeparator)

	// Then merge into appropriately sized chunks
	return s.MergeSplits(splits)
}

// SplitDocuments splits documents into smaller documents
func (s *CharacterTextSplitter) SplitDocuments(docs []vectorstore.Document) []vectorstore.Document {
	return s.BaseSplitter.SplitDocuments(docs, s.SplitText)
}

// RecursiveCharacterTextSplitter splits text recursively using multiple separators
type RecursiveCharacterTextSplitter struct {
	BaseSplitter
	separators []string
}

// RecursiveCharacterTextSplitterConfig configures the recursive splitter
type RecursiveCharacterTextSplitterConfig struct {
	// ChunkSize is the maximum size of each chunk
	ChunkSize int

	// ChunkOverlap is the overlap between chunks
	ChunkOverlap int

	// Separators is the list of separators to try (in order)
	Separators []string

	// KeepSeparator keeps the separator in chunks
	KeepSeparator bool

	// AddStartIndex adds start index to metadata
	AddStartIndex bool
}

// DefaultSeparators returns the default separators for recursive splitting
func DefaultSeparators() []string {
	return []string{"\n\n", "\n", " ", ""}
}

// NewRecursiveCharacterTextSplitter creates a new recursive text splitter.
// If configuration is invalid, defaults will be used and a warning logged.
func NewRecursiveCharacterTextSplitter(cfg RecursiveCharacterTextSplitterConfig) *RecursiveCharacterTextSplitter {
	config := DefaultSplitterConfig()
	if cfg.ChunkSize > 0 {
		config.ChunkSize = cfg.ChunkSize
	}
	if cfg.ChunkOverlap >= 0 {
		config.ChunkOverlap = cfg.ChunkOverlap
	}
	config.KeepSeparator = cfg.KeepSeparator
	config.AddStartIndex = cfg.AddStartIndex

	// Validate configuration
	if err := ValidateSplitterConfig(config); err != nil {
		log.Printf("textsplitter: invalid configuration, using defaults: %v", err)
		config = DefaultSplitterConfig()
	}

	separators := cfg.Separators
	if len(separators) == 0 {
		separators = DefaultSeparators()
	}

	return &RecursiveCharacterTextSplitter{
		BaseSplitter: NewBaseSplitter(config),
		separators:   separators,
	}
}

// SplitText splits text into chunks
func (s *RecursiveCharacterTextSplitter) SplitText(text string) []string {
	return s.splitTextRecursive(text, s.separators)
}

// splitTextRecursive recursively splits text using separators
func (s *RecursiveCharacterTextSplitter) splitTextRecursive(text string, separators []string) []string {
	var result []string

	// Find the best separator to use
	separator := separators[len(separators)-1]
	newSeparators := separators

	for i, sep := range separators {
		if sep == "" {
			separator = sep
			newSeparators = separators[i+1:]
			break
		}
		if containsString(text, sep) {
			separator = sep
			newSeparators = separators[i+1:]
			break
		}
	}

	// Split by the chosen separator
	splits := splitBySeparator(text, separator, s.config.KeepSeparator)

	var goodSplits []string
	for _, split := range splits {
		if s.config.LengthFunction(split) < s.config.ChunkSize {
			goodSplits = append(goodSplits, split)
		} else if len(newSeparators) > 0 {
			// Recursively split with remaining separators
			if len(goodSplits) > 0 {
				mergedText := joinStrings(goodSplits, separator)
				result = append(result, s.MergeSplits([]string{mergedText})...)
				goodSplits = nil
			}
			subSplits := s.splitTextRecursive(split, newSeparators)
			result = append(result, subSplits...)
		} else {
			// Can't split further, just add it
			if len(goodSplits) > 0 {
				mergedText := joinStrings(goodSplits, separator)
				result = append(result, s.MergeSplits([]string{mergedText})...)
				goodSplits = nil
			}
			result = append(result, split)
		}
	}

	if len(goodSplits) > 0 {
		mergedText := joinStrings(goodSplits, separator)
		result = append(result, s.MergeSplits([]string{mergedText})...)
	}

	return result
}

// SplitDocuments splits documents into smaller documents
func (s *RecursiveCharacterTextSplitter) SplitDocuments(docs []vectorstore.Document) []vectorstore.Document {
	return s.BaseSplitter.SplitDocuments(docs, s.SplitText)
}

// splitBySeparator splits text by a separator
func splitBySeparator(text, separator string, keepSeparator bool) []string {
	if separator == "" {
		// Split into individual characters
		result := make([]string, len(text))
		for i, c := range text {
			result[i] = string(c)
		}
		return result
	}

	var result []string
	start := 0
	sepLen := len(separator)

	for i := 0; i <= len(text)-sepLen; i++ {
		if text[i:i+sepLen] == separator {
			chunk := text[start:i]
			if keepSeparator {
				chunk += separator
			}
			if chunk != "" {
				result = append(result, chunk)
			}
			start = i + sepLen
		}
	}

	// Add remaining text
	if start < len(text) {
		result = append(result, text[start:])
	}

	return result
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	// Calculate total length
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}
	totalLen += len(sep) * (len(strs) - 1)

	// Build result
	result := make([]byte, totalLen)
	pos := 0
	for i, s := range strs {
		copy(result[pos:], s)
		pos += len(s)
		if i < len(strs)-1 {
			copy(result[pos:], sep)
			pos += len(sep)
		}
	}

	return string(result)
}
