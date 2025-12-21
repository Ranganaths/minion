package documentloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Ranganaths/minion/vectorstore"
)

// JSONLoader loads documents from JSON files
type JSONLoader struct {
	BaseLoader
	path        string
	jqFilter    string
	contentKeys []string
	textContent bool
}

// JSONLoaderConfig configures the JSON loader
type JSONLoaderConfig struct {
	// Path is the path to the JSON file (required)
	Path string

	// JQFilter is an optional jq-style filter (simplified subset)
	JQFilter string

	// ContentKeys are the keys to extract as content
	// If empty, the entire JSON object is converted to string
	ContentKeys []string

	// TextContent treats value as plain text (for JSON with embedded text)
	TextContent bool
}

// NewJSONLoader creates a new JSON loader
func NewJSONLoader(cfg JSONLoaderConfig) *JSONLoader {
	return &JSONLoader{
		BaseLoader:  NewBaseLoader(DefaultLoaderConfig()),
		path:        cfg.Path,
		jqFilter:    cfg.JQFilter,
		contentKeys: cfg.ContentKeys,
		textContent: cfg.TextContent,
	}
}

// Load loads documents from the JSON file
func (l *JSONLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	return l.loadFromBytes(ctx, data)
}

// LoadAndSplit loads and splits documents
func (l *JSONLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// loadFromBytes parses JSON from bytes
func (l *JSONLoader) loadFromBytes(ctx context.Context, data []byte) ([]vectorstore.Document, error) {
	// Try to parse as array first
	var arrayData []interface{}
	if err := json.Unmarshal(data, &arrayData); err == nil {
		return l.loadFromArray(ctx, arrayData)
	}

	// Try to parse as object
	var objectData map[string]interface{}
	if err := json.Unmarshal(data, &objectData); err == nil {
		return l.loadFromObject(ctx, objectData)
	}

	return nil, fmt.Errorf("JSON must be an array or object")
}

// loadFromArray loads documents from a JSON array
func (l *JSONLoader) loadFromArray(ctx context.Context, data []interface{}) ([]vectorstore.Document, error) {
	var docs []vectorstore.Document

	for i, item := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		doc, err := l.itemToDocument(item, i)
		if err != nil {
			continue // Skip invalid items
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// loadFromObject loads a document from a JSON object
func (l *JSONLoader) loadFromObject(ctx context.Context, data map[string]interface{}) ([]vectorstore.Document, error) {
	// Check if there's a data key containing an array
	if arrayData, ok := data["data"].([]interface{}); ok {
		return l.loadFromArray(ctx, arrayData)
	}
	if arrayData, ok := data["items"].([]interface{}); ok {
		return l.loadFromArray(ctx, arrayData)
	}
	if arrayData, ok := data["results"].([]interface{}); ok {
		return l.loadFromArray(ctx, arrayData)
	}

	// Treat the object as a single document
	doc, err := l.itemToDocument(data, 0)
	if err != nil {
		return nil, err
	}

	return []vectorstore.Document{doc}, nil
}

// itemToDocument converts a JSON item to a document
func (l *JSONLoader) itemToDocument(item interface{}, index int) (vectorstore.Document, error) {
	// Build content
	var content string

	switch v := item.(type) {
	case map[string]interface{}:
		if len(l.contentKeys) > 0 {
			var parts []string
			for _, key := range l.contentKeys {
				if val, ok := v[key]; ok {
					parts = append(parts, fmt.Sprintf("%v", val))
				}
			}
			content = strings.Join(parts, "\n")
		} else {
			// Convert entire object to formatted JSON
			jsonBytes, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return vectorstore.Document{}, err
			}
			content = string(jsonBytes)
		}

		// Build metadata from object keys
		metadata := DocumentMetadata{
			Source: l.path,
		}.ToMap()
		metadata["index"] = index

		// Add all keys as metadata (for small values)
		for key, val := range v {
			if str, ok := val.(string); ok && len(str) < 1000 {
				metadata[key] = val
			} else if _, isMap := val.(map[string]interface{}); !isMap {
				if _, isArr := val.([]interface{}); !isArr {
					metadata[key] = val
				}
			}
		}

		return vectorstore.NewDocumentWithMetadata(content, metadata), nil

	case string:
		if l.textContent {
			content = v
		} else {
			content = v
		}

	default:
		content = fmt.Sprintf("%v", item)
	}

	metadata := DocumentMetadata{
		Source: l.path,
	}.ToMap()
	metadata["index"] = index

	return vectorstore.NewDocumentWithMetadata(content, metadata), nil
}

// JSONLLoader loads documents from JSON Lines (newline-delimited JSON) files
type JSONLLoader struct {
	*JSONLoader
}

// NewJSONLLoader creates a new JSONL loader
func NewJSONLLoader(cfg JSONLoaderConfig) *JSONLLoader {
	return &JSONLLoader{
		JSONLoader: NewJSONLoader(cfg),
	}
}

// Load loads documents from the JSONL file
func (l *JSONLLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSONL file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var docs []vectorstore.Document

	for i, line := range lines {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var item map[string]interface{}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue // Skip invalid lines
		}

		doc, err := l.itemToDocument(item, i)
		if err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, nil
}
