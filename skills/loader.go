package skills

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SkillLoader loads skills from various sources
type SkillLoader struct {
	registry   skillRegistrar
	parser     *MarkdownParser
	httpClient *http.Client
	pathMap    map[string]string // skill name -> source path
	mu         sync.RWMutex
}

// skillRegistrar is an interface for registering skills
type skillRegistrar interface {
	Register(skill Skill) error
	UpdateSkill(skill Skill) error
	HasSkill(name string) bool
}

// NewSkillLoader creates a new skill loader
func NewSkillLoader(registry skillRegistrar) *SkillLoader {
	return &SkillLoader{
		registry:   registry,
		parser:     NewMarkdownParser(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		pathMap:    make(map[string]string),
	}
}

// LoadFromFile loads a skill from a file path
func (l *SkillLoader) LoadFromFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	skill, err := l.parser.Parse(content, absPath)
	if err != nil {
		return fmt.Errorf("failed to parse skill from %s: %w", path, err)
	}

	// Track the path for this skill
	l.mu.Lock()
	l.pathMap[skill.Name()] = absPath
	l.mu.Unlock()

	return l.registry.Register(skill)
}

// LoadFromURL loads a skill from a URL
func (l *SkillLoader) LoadFromURL(url string) error {
	resp, err := l.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch URL: status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	skill, err := l.parser.Parse(content, url)
	if err != nil {
		return fmt.Errorf("failed to parse skill from URL: %w", err)
	}

	// Track the URL for this skill
	l.mu.Lock()
	l.pathMap[skill.Name()] = url
	l.mu.Unlock()

	return l.registry.Register(skill)
}

// LoadFromEmbed loads skills from an embedded filesystem
func (l *SkillLoader) LoadFromEmbed(embedFS embed.FS, pattern string) (int, error) {
	matches, err := fs.Glob(embedFS, pattern)
	if err != nil {
		return 0, fmt.Errorf("failed to glob pattern: %w", err)
	}

	loaded := 0
	var loadErrors []string

	for _, match := range matches {
		content, err := embedFS.ReadFile(match)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", match, err))
			continue
		}

		skill, err := l.parser.Parse(content, "embedded:"+match)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", match, err))
			continue
		}

		if err := l.registry.Register(skill); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", match, err))
			continue
		}

		// Track the embedded path
		l.mu.Lock()
		l.pathMap[skill.Name()] = "embedded:" + match
		l.mu.Unlock()

		loaded++
	}

	if len(loadErrors) > 0 && loaded == 0 {
		return 0, fmt.Errorf("failed to load any skills: %s", strings.Join(loadErrors, "; "))
	}

	return loaded, nil
}

// LoadDirectory loads all skills from a directory
func (l *SkillLoader) LoadDirectory(path string) (int, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("directory does not exist: %s", path)
		}
		return 0, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return 0, fmt.Errorf("path is not a directory: %s", path)
	}

	loaded := 0
	var loadErrors []string

	err = filepath.WalkDir(absPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if d.IsDir() {
			return nil
		}

		if !isSkillFile(p) {
			return nil
		}

		if err := l.LoadFromFile(p); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", p, err))
			return nil // Continue loading other files
		}

		loaded++
		return nil
	})

	if err != nil {
		return loaded, fmt.Errorf("error walking directory: %w", err)
	}

	return loaded, nil
}

// ReloadFile reloads a skill from a file (for hot-reload)
func (l *SkillLoader) ReloadFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	skill, err := l.parser.Parse(content, absPath)
	if err != nil {
		return fmt.Errorf("failed to parse skill: %w", err)
	}

	// Update the path map
	l.mu.Lock()
	l.pathMap[skill.Name()] = absPath
	l.mu.Unlock()

	// Use UpdateSkill to update existing or register new
	return l.registry.UpdateSkill(skill)
}

// GetPathForSkill returns the source path for a skill
func (l *SkillLoader) GetPathForSkill(name string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	path, ok := l.pathMap[name]
	return path, ok
}

// GetSkillForPath returns the skill name for a source path
func (l *SkillLoader) GetSkillForPath(path string) (string, bool) {
	absPath, _ := filepath.Abs(path)

	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, p := range l.pathMap {
		pathAbs, _ := filepath.Abs(p)
		if pathAbs == absPath {
			return name, true
		}
	}

	return "", false
}

// RemovePathMapping removes the path mapping for a skill
func (l *SkillLoader) RemovePathMapping(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.pathMap, name)
}

// isSkillFile checks if a file is a skill file
func isSkillFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// LoadFromBytes loads a skill from raw bytes
func (l *SkillLoader) LoadFromBytes(content []byte, source string) error {
	skill, err := l.parser.Parse(content, source)
	if err != nil {
		return fmt.Errorf("failed to parse skill: %w", err)
	}

	return l.registry.Register(skill)
}

// LoadFromString loads a skill from a string
func (l *SkillLoader) LoadFromString(content, source string) error {
	return l.LoadFromBytes([]byte(content), source)
}

// SetHTTPClient sets a custom HTTP client for URL loading
func (l *SkillLoader) SetHTTPClient(client *http.Client) {
	l.httpClient = client
}

// SkillLoaderOption is an option for configuring the skill loader
type SkillLoaderOption func(*SkillLoader)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) SkillLoaderOption {
	return func(l *SkillLoader) {
		l.httpClient = client
	}
}

// WithHTTPTimeout sets the HTTP client timeout
func WithHTTPTimeout(timeout time.Duration) SkillLoaderOption {
	return func(l *SkillLoader) {
		l.httpClient = &http.Client{Timeout: timeout}
	}
}

// NewSkillLoaderWithOptions creates a new skill loader with options
func NewSkillLoaderWithOptions(registry skillRegistrar, opts ...SkillLoaderOption) *SkillLoader {
	loader := NewSkillLoader(registry)
	for _, opt := range opts {
		opt(loader)
	}
	return loader
}

// LoadResult represents the result of loading skills from a directory
type LoadResult struct {
	Loaded    int
	Failed    int
	Errors    []LoadError
	TotalTime time.Duration
}

// LoadError represents an error loading a specific file
type LoadError struct {
	Path  string
	Error error
}

// LoadDirectoryWithResults loads all skills from a directory and returns detailed results
func (l *SkillLoader) LoadDirectoryWithResults(path string) (*LoadResult, error) {
	startTime := time.Now()
	result := &LoadResult{}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	err = filepath.WalkDir(absPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !isSkillFile(p) {
			return nil
		}

		if err := l.LoadFromFile(p); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, LoadError{Path: p, Error: err})
			return nil
		}

		result.Loaded++
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("error walking directory: %w", err)
	}

	result.TotalTime = time.Since(startTime)
	return result, nil
}
