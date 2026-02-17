package agui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// StateManager manages shared state between agent and UI
type StateManager struct {
	mu       sync.RWMutex
	state    map[string]any
	emitter  *EventEmitter
	onChange func(delta []JSONPatchOperation)
}

// NewStateManager creates a new state manager
func NewStateManager(emitter *EventEmitter) *StateManager {
	return &StateManager{
		state:   make(map[string]any),
		emitter: emitter,
	}
}

// NewStateManagerWithInitial creates a state manager with initial state
func NewStateManagerWithInitial(emitter *EventEmitter, initial map[string]any) *StateManager {
	sm := &StateManager{
		state:   make(map[string]any),
		emitter: emitter,
	}
	if initial != nil {
		for k, v := range initial {
			sm.state[k] = deepCopy(v)
		}
	}
	return sm
}

// OnChange sets a callback for state changes
func (s *StateManager) OnChange(fn func(delta []JSONPatchOperation)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// Get retrieves a value from state
func (s *StateManager) Get(path string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return getPath(s.state, path)
}

// GetAll returns a copy of the entire state
func (s *StateManager) GetAll() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return deepCopy(s.state).(map[string]any)
}

// Set sets a value in state and emits a delta
func (s *StateManager) Set(path string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if path exists
	_, exists := getPath(s.state, path)

	// Set the value
	if err := setPath(s.state, path, value); err != nil {
		return err
	}

	// Create delta
	op := "replace"
	if !exists {
		op = "add"
	}

	delta := []JSONPatchOperation{{
		Op:    op,
		Path:  toJSONPointer(path),
		Value: value,
	}}

	s.emitDelta(delta)
	return nil
}

// Remove removes a value from state
func (s *StateManager) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if path exists
	if _, exists := getPath(s.state, path); !exists {
		return fmt.Errorf("path %s does not exist", path)
	}

	// Remove the value
	if err := removePath(s.state, path); err != nil {
		return err
	}

	delta := []JSONPatchOperation{{
		Op:   "remove",
		Path: toJSONPointer(path),
	}}

	s.emitDelta(delta)
	return nil
}

// Patch applies a JSON Patch to the state
func (s *StateManager) Patch(operations []JSONPatchOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Apply each operation
	for _, op := range operations {
		path := fromJSONPointer(op.Path)
		switch op.Op {
		case "add", "replace":
			if err := setPath(s.state, path, op.Value); err != nil {
				return fmt.Errorf("failed to apply %s at %s: %w", op.Op, op.Path, err)
			}
		case "remove":
			if err := removePath(s.state, path); err != nil {
				return fmt.Errorf("failed to apply remove at %s: %w", op.Path, err)
			}
		case "move":
			from := fromJSONPointer(op.From)
			value, exists := getPath(s.state, from)
			if !exists {
				return fmt.Errorf("move source %s does not exist", op.From)
			}
			if err := removePath(s.state, from); err != nil {
				return err
			}
			if err := setPath(s.state, path, value); err != nil {
				return err
			}
		case "copy":
			from := fromJSONPointer(op.From)
			value, exists := getPath(s.state, from)
			if !exists {
				return fmt.Errorf("copy source %s does not exist", op.From)
			}
			if err := setPath(s.state, path, deepCopy(value)); err != nil {
				return err
			}
		case "test":
			value, exists := getPath(s.state, path)
			if !exists {
				return fmt.Errorf("test failed: path %s does not exist", op.Path)
			}
			if !reflect.DeepEqual(value, op.Value) {
				return fmt.Errorf("test failed: value at %s does not match", op.Path)
			}
		default:
			return fmt.Errorf("unknown operation: %s", op.Op)
		}
	}

	s.emitDelta(operations)
	return nil
}

// EmitSnapshot emits the current state as a snapshot
func (s *StateManager) EmitSnapshot() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.emitter != nil {
		s.emitter.EmitStateSnapshot(deepCopy(s.state).(map[string]any))
	}
}

// emitDelta emits a state delta
func (s *StateManager) emitDelta(delta []JSONPatchOperation) {
	if s.emitter != nil {
		s.emitter.EmitStateDelta(delta)
	}
	if s.onChange != nil {
		s.onChange(delta)
	}
}

// Diff computes the JSON Patch between old and new state
func Diff(oldState, newState map[string]any) []JSONPatchOperation {
	var ops []JSONPatchOperation
	diffObjects("", oldState, newState, &ops)
	return ops
}

// diffObjects recursively diffs two objects
func diffObjects(path string, old, new map[string]any, ops *[]JSONPatchOperation) {
	// Check for removed and changed keys
	for key, oldValue := range old {
		newPath := path + "/" + escapeJSONPointer(key)
		if newValue, exists := new[key]; exists {
			diffValues(newPath, oldValue, newValue, ops)
		} else {
			*ops = append(*ops, JSONPatchOperation{
				Op:   "remove",
				Path: newPath,
			})
		}
	}

	// Check for added keys
	for key, newValue := range new {
		if _, exists := old[key]; !exists {
			newPath := path + "/" + escapeJSONPointer(key)
			*ops = append(*ops, JSONPatchOperation{
				Op:    "add",
				Path:  newPath,
				Value: newValue,
			})
		}
	}
}

// diffValues diffs two values
func diffValues(path string, old, new any, ops *[]JSONPatchOperation) {
	oldMap, oldIsMap := old.(map[string]any)
	newMap, newIsMap := new.(map[string]any)

	if oldIsMap && newIsMap {
		diffObjects(path, oldMap, newMap, ops)
		return
	}

	oldSlice, oldIsSlice := old.([]any)
	newSlice, newIsSlice := new.([]any)

	if oldIsSlice && newIsSlice {
		diffArrays(path, oldSlice, newSlice, ops)
		return
	}

	// For non-composite types, just replace if different
	if !reflect.DeepEqual(old, new) {
		*ops = append(*ops, JSONPatchOperation{
			Op:    "replace",
			Path:  path,
			Value: new,
		})
	}
}

// diffArrays diffs two arrays
func diffArrays(path string, old, new []any, ops *[]JSONPatchOperation) {
	// Simple approach: replace the entire array if different
	// A more sophisticated approach would compute minimal edit distance
	if !reflect.DeepEqual(old, new) {
		*ops = append(*ops, JSONPatchOperation{
			Op:    "replace",
			Path:  path,
			Value: new,
		})
	}
}

// Helper functions for path manipulation

// toJSONPointer converts a dot-notation path to JSON Pointer
func toJSONPointer(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	var escaped []string
	for _, p := range parts {
		escaped = append(escaped, escapeJSONPointer(p))
	}
	return "/" + strings.Join(escaped, "/")
}

// fromJSONPointer converts a JSON Pointer to dot-notation path
func fromJSONPointer(pointer string) string {
	if pointer == "" || pointer == "/" {
		return ""
	}
	pointer = strings.TrimPrefix(pointer, "/")
	parts := strings.Split(pointer, "/")
	var unescaped []string
	for _, p := range parts {
		unescaped = append(unescaped, unescapeJSONPointer(p))
	}
	return strings.Join(unescaped, ".")
}

// escapeJSONPointer escapes a path segment for JSON Pointer
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// unescapeJSONPointer unescapes a JSON Pointer segment
func unescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// getPath gets a value at a dot-notation path
func getPath(obj map[string]any, path string) (any, bool) {
	if path == "" {
		return obj, true
	}

	parts := strings.Split(path, ".")
	var current any = obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, exists := v[part]
			if !exists {
				return nil, false
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]
		default:
			return nil, false
		}
	}

	return current, true
}

// setPath sets a value at a dot-notation path
func setPath(obj map[string]any, path string, value any) error {
	if path == "" {
		return fmt.Errorf("cannot set root")
	}

	parts := strings.Split(path, ".")
	var current any = obj

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		switch v := current.(type) {
		case map[string]any:
			next, exists := v[part]
			if !exists {
				// Create intermediate map
				v[part] = make(map[string]any)
				current = v[part]
			} else {
				current = next
			}
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return fmt.Errorf("invalid array index: %s", part)
			}
			current = v[idx]
		default:
			return fmt.Errorf("cannot traverse %T", current)
		}
	}

	// Set the final value
	lastPart := parts[len(parts)-1]
	switch v := current.(type) {
	case map[string]any:
		v[lastPart] = value
	case []any:
		idx, err := strconv.Atoi(lastPart)
		if err != nil || idx < 0 || idx >= len(v) {
			return fmt.Errorf("invalid array index: %s", lastPart)
		}
		v[idx] = value
	default:
		return fmt.Errorf("cannot set on %T", current)
	}

	return nil
}

// removePath removes a value at a dot-notation path
func removePath(obj map[string]any, path string) error {
	if path == "" {
		return fmt.Errorf("cannot remove root")
	}

	parts := strings.Split(path, ".")
	var current any = obj

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		switch v := current.(type) {
		case map[string]any:
			next, exists := v[part]
			if !exists {
				return fmt.Errorf("path does not exist")
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return fmt.Errorf("invalid array index: %s", part)
			}
			current = v[idx]
		default:
			return fmt.Errorf("cannot traverse %T", current)
		}
	}

	// Remove the final value
	lastPart := parts[len(parts)-1]
	switch v := current.(type) {
	case map[string]any:
		delete(v, lastPart)
	default:
		return fmt.Errorf("cannot remove from %T", current)
	}

	return nil
}

// deepCopy creates a deep copy of a value
func deepCopy(v any) any {
	if v == nil {
		return nil
	}

	// Use JSON round-trip for simplicity
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return v
	}

	return result
}
