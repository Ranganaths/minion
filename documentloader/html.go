package documentloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/Ranganaths/minion/vectorstore"
)

// HTMLLoader loads documents from HTML files or URLs
type HTMLLoader struct {
	BaseLoader
	path        string
	url         string
	extractText bool
	excludeTags []string
}

// HTMLLoaderConfig configures the HTML loader
type HTMLLoaderConfig struct {
	// Path is the path to the HTML file (mutually exclusive with URL)
	Path string

	// URL is the URL to fetch HTML from (mutually exclusive with Path)
	URL string

	// ExtractText extracts only text content, removing HTML tags
	ExtractText bool

	// ExcludeTags are HTML tags to exclude from extraction
	ExcludeTags []string
}

// NewHTMLLoader creates a new HTML loader
func NewHTMLLoader(cfg HTMLLoaderConfig) *HTMLLoader {
	excludeTags := cfg.ExcludeTags
	if len(excludeTags) == 0 {
		excludeTags = []string{"script", "style", "head", "nav", "footer", "aside"}
	}

	return &HTMLLoader{
		BaseLoader:  NewBaseLoader(DefaultLoaderConfig()),
		path:        cfg.Path,
		url:         cfg.URL,
		extractText: cfg.ExtractText,
		excludeTags: excludeTags,
	}
}

// Load loads documents from the HTML source
func (l *HTMLLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	var content string
	var source string
	var err error

	if l.path != "" {
		content, err = l.loadFromFile(l.path)
		source = l.path
	} else if l.url != "" {
		content, err = l.loadFromURL(ctx, l.url)
		source = l.url
	} else {
		return nil, fmt.Errorf("either path or URL must be specified")
	}

	if err != nil {
		return nil, err
	}

	// Extract text if requested
	if l.extractText {
		content = l.extractTextFromHTML(content)
	}

	// Extract title
	title := l.extractTitle(content)

	metadata := DocumentMetadata{
		Source:   source,
		Title:    title,
		MimeType: "text/html",
	}.ToMap()

	doc := vectorstore.NewDocumentWithMetadata(content, metadata)
	return []vectorstore.Document{doc}, nil
}

// LoadAndSplit loads and splits documents
func (l *HTMLLoader) LoadAndSplit(ctx context.Context, splitter TextSplitter) ([]vectorstore.Document, error) {
	docs, err := l.Load(ctx)
	if err != nil {
		return nil, err
	}
	return splitter.SplitDocuments(docs), nil
}

// loadFromFile loads HTML from a file
func (l *HTMLLoader) loadFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read HTML file: %w", err)
	}
	return string(data), nil
}

// loadFromURL loads HTML from a URL
func (l *HTMLLoader) loadFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Minion/1.0)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(data), nil
}

// extractTextFromHTML extracts text content from HTML
func (l *HTMLLoader) extractTextFromHTML(html string) string {
	// Remove excluded tags and their content
	for _, tag := range l.excludeTags {
		pattern := regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*>[\s\S]*?</%s>`, tag, tag))
		html = pattern.ReplaceAllString(html, "")
	}

	// Remove comments
	commentPattern := regexp.MustCompile(`<!--[\s\S]*?-->`)
	html = commentPattern.ReplaceAllString(html, "")

	// Replace block-level tags with newlines
	blockTags := []string{"p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "td", "th"}
	for _, tag := range blockTags {
		// Opening and self-closing tags
		pattern := regexp.MustCompile(fmt.Sprintf(`(?i)</?%s[^>]*>`, tag))
		html = pattern.ReplaceAllString(html, "\n")
	}

	// Remove remaining tags
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	html = tagPattern.ReplaceAllString(html, "")

	// Decode HTML entities
	html = l.decodeHTMLEntities(html)

	// Clean up whitespace
	html = l.cleanWhitespace(html)

	return strings.TrimSpace(html)
}

// extractTitle extracts the title from HTML
func (l *HTMLLoader) extractTitle(html string) string {
	titlePattern := regexp.MustCompile(`(?i)<title[^>]*>([\s\S]*?)</title>`)
	matches := titlePattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// decodeHTMLEntities decodes common HTML entities
func (l *HTMLLoader) decodeHTMLEntities(s string) string {
	entities := map[string]string{
		"&nbsp;":  " ",
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&apos;":  "'",
		"&#39;":   "'",
		"&copy;":  "©",
		"&reg;":   "®",
		"&mdash;": "—",
		"&ndash;": "–",
		"&hellip;": "...",
	}

	for entity, replacement := range entities {
		s = strings.ReplaceAll(s, entity, replacement)
	}

	// Handle numeric entities
	numPattern := regexp.MustCompile(`&#(\d+);`)
	s = numPattern.ReplaceAllStringFunc(s, func(match string) string {
		var code int
		fmt.Sscanf(match, "&#%d;", &code)
		if code > 0 && code < 128 {
			return string(rune(code))
		}
		return match
	})

	return s
}

// cleanWhitespace normalizes whitespace in text
func (l *HTMLLoader) cleanWhitespace(s string) string {
	// Replace multiple spaces/tabs with single space
	spacePattern := regexp.MustCompile(`[ \t]+`)
	s = spacePattern.ReplaceAllString(s, " ")

	// Replace multiple newlines with double newline
	newlinePattern := regexp.MustCompile(`\n\s*\n`)
	s = newlinePattern.ReplaceAllString(s, "\n\n")

	return s
}

// WebPageLoader loads documents from web pages
type WebPageLoader struct {
	*HTMLLoader
	urls []string
}

// NewWebPageLoader creates a loader for multiple web pages
func NewWebPageLoader(urls []string) *WebPageLoader {
	return &WebPageLoader{
		HTMLLoader: NewHTMLLoader(HTMLLoaderConfig{
			ExtractText: true,
		}),
		urls: urls,
	}
}

// Load loads documents from all URLs
func (l *WebPageLoader) Load(ctx context.Context) ([]vectorstore.Document, error) {
	var allDocs []vectorstore.Document

	for _, url := range l.urls {
		l.HTMLLoader.url = url
		docs, err := l.HTMLLoader.Load(ctx)
		if err != nil {
			// Log error but continue with other URLs
			continue
		}
		allDocs = append(allDocs, docs...)
	}

	if len(allDocs) == 0 {
		return nil, fmt.Errorf("no documents loaded from any URL")
	}

	return allDocs, nil
}
