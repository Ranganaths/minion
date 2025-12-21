package documentloader

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Ranganaths/minion/vectorstore"
)

// CSVLoader loads documents from CSV files
type CSVLoader struct {
	BaseLoader
	path          string
	sourceColumn  string
	contentColumn string
	columns       []string
	delimiter     rune
}

// CSVLoaderConfig configures the CSV loader
type CSVLoaderConfig struct {
	// Path is the path to the CSV file (required)
	Path string

	// SourceColumn is the column to use for document source metadata
	SourceColumn string

	// ContentColumn is the column to use for document content
	// If not set, all columns are concatenated
	ContentColumn string

	// Columns are specific columns to include (if empty, all columns are used)
	Columns []string

	// Delimiter is the CSV field delimiter (default: comma)
	Delimiter rune
}

// NewCSVLoader creates a new CSV loader
func NewCSVLoader(cfg CSVLoaderConfig) *CSVLoader {
	delimiter := cfg.Delimiter
	if delimiter == 0 {
		delimiter = ','
	}

	return &CSVLoader{
		BaseLoader:    NewBaseLoader(DefaultLoaderConfig()),
		path:          cfg.Path,
		sourceColumn:  cfg.SourceColumn,
		contentColumn: cfg.ContentColumn,
		columns:       cfg.Columns,
		delimiter:     delimiter,
	}
}

// Load loads documents from the CSV file
func (l *CSVLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	file, err := os.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	return l.loadFromReader(ctx, file)
}

// LoadAndSplit loads and splits documents
func (l *CSVLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// loadFromReader loads CSV from a reader
func (l *CSVLoader) loadFromReader(ctx context.Context, r io.Reader) ([]vectorstore.Document, error) {
	reader := csv.NewReader(r)
	reader.Comma = l.delimiter

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Create column index map
	columnIndex := make(map[string]int)
	for i, col := range header {
		columnIndex[col] = i
	}

	// Validate columns if specified
	if len(l.columns) > 0 {
		for _, col := range l.columns {
			if _, ok := columnIndex[col]; !ok {
				return nil, fmt.Errorf("column not found: %s", col)
			}
		}
	}

	// Validate content column
	contentColIdx := -1
	if l.contentColumn != "" {
		if idx, ok := columnIndex[l.contentColumn]; ok {
			contentColIdx = idx
		} else {
			return nil, fmt.Errorf("content column not found: %s", l.contentColumn)
		}
	}

	// Read rows
	var docs []vectorstore.Document
	rowNum := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading row %d: %w", rowNum, err)
		}
		rowNum++

		// Build content
		var content string
		if contentColIdx >= 0 && contentColIdx < len(row) {
			content = row[contentColIdx]
		} else {
			// Concatenate specified or all columns
			var parts []string
			columnsToUse := l.columns
			if len(columnsToUse) == 0 {
				columnsToUse = header
			}

			for _, col := range columnsToUse {
				if idx, ok := columnIndex[col]; ok && idx < len(row) {
					parts = append(parts, fmt.Sprintf("%s: %s", col, row[idx]))
				}
			}
			content = strings.Join(parts, "\n")
		}

		// Build metadata
		metadata := DocumentMetadata{
			Source: l.path,
		}.ToMap()

		metadata["row"] = rowNum

		// Add all columns as metadata
		for i, col := range header {
			if i < len(row) {
				metadata[col] = row[i]
			}
		}

		// Add source column to metadata if specified
		if l.sourceColumn != "" {
			if idx, ok := columnIndex[l.sourceColumn]; ok && idx < len(row) {
				metadata["source_value"] = row[idx]
			}
		}

		doc := vectorstore.NewDocumentWithMetadata(content, metadata)
		docs = append(docs, doc)
	}

	return docs, nil
}

// CSVReaderLoader loads CSV from an io.Reader
type CSVReaderLoader struct {
	*CSVLoader
	reader io.Reader
}

// NewCSVReaderLoader creates a CSV loader from a reader
func NewCSVReaderLoader(reader io.Reader, cfg CSVLoaderConfig) *CSVReaderLoader {
	loader := NewCSVLoader(cfg)
	return &CSVReaderLoader{
		CSVLoader: loader,
		reader:    reader,
	}
}

// Load loads documents from the reader
func (l *CSVReaderLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	return l.loadFromReader(ctx, l.reader)
}
