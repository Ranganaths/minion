package skills

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SkillWatcher watches directories for skill file changes and hot-reloads them
type SkillWatcher struct {
	watcher    *fsnotify.Watcher
	registry   skillUnregistrar
	loader     *SkillLoader
	watchPaths map[string]bool
	mu         sync.RWMutex
	done       chan struct{}
	logger     Logger
	debounce   time.Duration
	events     chan watchEvent
}

// skillUnregistrar extends skillRegistrar with Unregister capability
type skillUnregistrar interface {
	skillRegistrar
	Unregister(name string) error
	FindSkillByPath(path string) (Skill, bool)
}

// Logger is a simple logging interface
type Logger interface {
	Printf(format string, v ...interface{})
}

// defaultLogger is a simple default logger
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf("[SkillWatcher] "+format, v...)
}

// watchEvent represents a debounced file system event
type watchEvent struct {
	path string
	op   fsnotify.Op
}

// NewSkillWatcher creates a new skill watcher
func NewSkillWatcher(registry skillUnregistrar, loader *SkillLoader) (*SkillWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	sw := &SkillWatcher{
		watcher:    watcher,
		registry:   registry,
		loader:     loader,
		watchPaths: make(map[string]bool),
		done:       make(chan struct{}),
		logger:     &defaultLogger{},
		debounce:   100 * time.Millisecond,
		events:     make(chan watchEvent, 100),
	}

	// Start the event processing goroutine
	go sw.processEvents()

	return sw, nil
}

// Watch starts watching a directory for changes
func (w *SkillWatcher) Watch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if already watching
	w.mu.Lock()
	if w.watchPaths[absPath] {
		w.mu.Unlock()
		return nil // Already watching
	}
	w.watchPaths[absPath] = true
	w.mu.Unlock()

	// Verify path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Add the directory to the watcher
	if err := w.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Also watch subdirectories
	err = filepath.WalkDir(absPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return w.watcher.Add(p)
		}
		return nil
	})

	if err != nil {
		w.logger.Printf("warning: failed to watch some subdirectories: %v", err)
	}

	w.logger.Printf("watching directory: %s", absPath)

	// Start watching if not already started
	go w.watch()

	return nil
}

// watch is the main event loop for file system events
func (w *SkillWatcher) watch() {
	// Debounce timers for each file
	timers := make(map[string]*time.Timer)
	timersMu := sync.Mutex{}

	for {
		select {
		case <-w.done:
			// Clean up timers
			timersMu.Lock()
			for _, timer := range timers {
				timer.Stop()
			}
			timersMu.Unlock()
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process skill files
			if !isSkillFile(event.Name) {
				continue
			}

			// Debounce events for the same file
			timersMu.Lock()
			if timer, exists := timers[event.Name]; exists {
				timer.Stop()
			}

			timers[event.Name] = time.AfterFunc(w.debounce, func() {
				w.events <- watchEvent{path: event.Name, op: event.Op}
			})
			timersMu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Printf("watcher error: %v", err)
		}
	}
}

// processEvents processes debounced events
func (w *SkillWatcher) processEvents() {
	for {
		select {
		case <-w.done:
			return

		case event := <-w.events:
			w.handleEvent(event)
		}
	}
}

// handleEvent handles a single file system event
func (w *SkillWatcher) handleEvent(event watchEvent) {
	path := event.path

	switch {
	case event.op&fsnotify.Create == fsnotify.Create:
		w.handleCreate(path)

	case event.op&fsnotify.Write == fsnotify.Write:
		w.handleWrite(path)

	case event.op&fsnotify.Remove == fsnotify.Remove:
		w.handleRemove(path)

	case event.op&fsnotify.Rename == fsnotify.Rename:
		w.handleRemove(path) // Treat rename as remove (new file will trigger create)
	}
}

// handleCreate handles a new file being created
func (w *SkillWatcher) handleCreate(path string) {
	w.logger.Printf("skill file created: %s", path)

	// Wait a bit for the file to be fully written
	time.Sleep(50 * time.Millisecond)

	if err := w.loader.LoadFromFile(path); err != nil {
		w.logger.Printf("failed to load new skill from %s: %v", path, err)
		return
	}

	w.logger.Printf("loaded new skill from: %s", path)
}

// handleWrite handles a file being modified
func (w *SkillWatcher) handleWrite(path string) {
	w.logger.Printf("skill file modified: %s", path)

	// Wait a bit for the file to be fully written
	time.Sleep(50 * time.Millisecond)

	if err := w.loader.ReloadFile(path); err != nil {
		w.logger.Printf("failed to reload skill from %s: %v", path, err)
		return
	}

	w.logger.Printf("reloaded skill from: %s", path)
}

// handleRemove handles a file being deleted
func (w *SkillWatcher) handleRemove(path string) {
	w.logger.Printf("skill file removed: %s", path)

	// Find the skill that was loaded from this path
	absPath, _ := filepath.Abs(path)

	skill, found := w.registry.FindSkillByPath(absPath)
	if !found {
		w.logger.Printf("no skill found for path: %s", path)
		return
	}

	if err := w.registry.Unregister(skill.Name()); err != nil {
		w.logger.Printf("failed to unregister skill %s: %v", skill.Name(), err)
		return
	}

	w.loader.RemovePathMapping(skill.Name())
	w.logger.Printf("unregistered skill: %s", skill.Name())
}

// Stop stops the watcher
func (w *SkillWatcher) Stop() error {
	close(w.done)
	return w.watcher.Close()
}

// SetLogger sets a custom logger
func (w *SkillWatcher) SetLogger(logger Logger) {
	w.logger = logger
}

// SetDebounce sets the debounce duration for file events
func (w *SkillWatcher) SetDebounce(d time.Duration) {
	w.debounce = d
}

// WatchedPaths returns the list of watched paths
func (w *SkillWatcher) WatchedPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, 0, len(w.watchPaths))
	for path := range w.watchPaths {
		paths = append(paths, path)
	}
	return paths
}

// Unwatch stops watching a directory
func (w *SkillWatcher) Unwatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	w.mu.Lock()
	delete(w.watchPaths, absPath)
	w.mu.Unlock()

	if err := w.watcher.Remove(absPath); err != nil {
		if !strings.Contains(err.Error(), "can't remove non-existent") {
			return fmt.Errorf("failed to remove watch: %w", err)
		}
	}

	w.logger.Printf("stopped watching: %s", absPath)
	return nil
}

// IsWatching checks if a path is being watched
func (w *SkillWatcher) IsWatching(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.watchPaths[absPath]
}

// SkillWatcherOption is an option for configuring the skill watcher
type SkillWatcherOption func(*SkillWatcher)

// WithWatcherLogger sets a custom logger
func WithWatcherLogger(logger Logger) SkillWatcherOption {
	return func(w *SkillWatcher) {
		w.logger = logger
	}
}

// WithWatcherDebounce sets the debounce duration
func WithWatcherDebounce(d time.Duration) SkillWatcherOption {
	return func(w *SkillWatcher) {
		w.debounce = d
	}
}

// NewSkillWatcherWithOptions creates a new skill watcher with options
func NewSkillWatcherWithOptions(registry skillUnregistrar, loader *SkillLoader, opts ...SkillWatcherOption) (*SkillWatcher, error) {
	watcher, err := NewSkillWatcher(registry, loader)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(watcher)
	}

	return watcher, nil
}

// WatchCallback is called when a skill is loaded, reloaded, or unloaded
type WatchCallback func(event WatchEventType, skillName string, path string)

// WatchEventType represents the type of watch event
type WatchEventType string

const (
	WatchEventLoaded   WatchEventType = "loaded"
	WatchEventReloaded WatchEventType = "reloaded"
	WatchEventUnloaded WatchEventType = "unloaded"
)

// SkillWatcherWithCallback extends SkillWatcher with callback support
type SkillWatcherWithCallback struct {
	*SkillWatcher
	callback WatchCallback
}

// NewSkillWatcherWithCallback creates a watcher with a callback
func NewSkillWatcherWithCallback(registry skillUnregistrar, loader *SkillLoader, callback WatchCallback) (*SkillWatcherWithCallback, error) {
	watcher, err := NewSkillWatcher(registry, loader)
	if err != nil {
		return nil, err
	}

	return &SkillWatcherWithCallback{
		SkillWatcher: watcher,
		callback:     callback,
	}, nil
}
