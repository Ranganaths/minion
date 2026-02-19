// Package memory provides tiered memory management for agents.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Ranganaths/minion/debug/snapshot"
)

// TierType represents the storage tier
type TierType string

const (
	TierHot  TierType = "hot"  // Fast, frequently accessed (Redis/Memory)
	TierWarm TierType = "warm" // Moderate speed, queryable (PostgreSQL/SQLite)
	TierCold TierType = "cold" // Slow, archived (S3/File)
)

// TieredMemoryOption is a functional option for configuring TieredMemoryManager
type TieredMemoryOption func(*TieredMemoryManager)

// TieredMemoryManager manages memory across hot, warm, and cold storage tiers.
// It automatically promotes/demotes data based on access patterns and age.
type TieredMemoryManager struct {
	mu sync.RWMutex

	// Storage backends
	hotStore  HotStore
	warmStore WarmStore
	coldStore ColdStore

	// Configuration
	config TieredMemoryConfig

	// Background workers
	stopCh   chan struct{}
	doneCh   chan struct{}
	running  bool

	// Metrics
	metrics TieredMemoryMetrics
}

// TieredMemoryConfig configures the tiered memory manager
type TieredMemoryConfig struct {
	// HotTTL is how long items stay in hot storage (default: 1 hour)
	HotTTL time.Duration

	// WarmTTL is how long items stay in warm storage before archiving (default: 7 days)
	WarmTTL time.Duration

	// HotMaxItems is the maximum number of items in hot storage (default: 10000)
	HotMaxItems int

	// PromoteOnAccess promotes items back to hot tier when accessed (default: true)
	PromoteOnAccess bool

	// CompactionInterval is how often to run compaction (default: 1 hour)
	CompactionInterval time.Duration

	// ArchiveInterval is how often to archive old data (default: 24 hours)
	ArchiveInterval time.Duration
}

// DefaultTieredMemoryConfig returns default configuration
func DefaultTieredMemoryConfig() TieredMemoryConfig {
	return TieredMemoryConfig{
		HotTTL:             1 * time.Hour,
		WarmTTL:            7 * 24 * time.Hour,
		HotMaxItems:        10000,
		PromoteOnAccess:    true,
		CompactionInterval: 1 * time.Hour,
		ArchiveInterval:    24 * time.Hour,
	}
}

// TieredMemoryMetrics tracks memory tier metrics
type TieredMemoryMetrics struct {
	HotHits       int64
	HotMisses     int64
	WarmHits      int64
	WarmMisses    int64
	ColdHits      int64
	ColdMisses    int64
	Promotions    int64
	Demotions     int64
	Archivals     int64
	mu            sync.Mutex
}

// HotStore interface for hot tier storage (Redis-like)
type HotStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Keys(ctx context.Context, pattern string) ([]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Close() error
}

// WarmStore interface for warm tier storage (database-like)
type WarmStore interface {
	Get(ctx context.Context, key string) (*TieredMemoryItem, error)
	Set(ctx context.Context, item *TieredMemoryItem) error
	Delete(ctx context.Context, key string) error
	Query(ctx context.Context, query *TieredMemoryQuery) ([]*TieredMemoryItem, error)
	GetOlderThan(ctx context.Context, age time.Duration, limit int) ([]*TieredMemoryItem, error)
	DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error)
	Close() error
}

// ColdStore interface for cold tier storage (archive-like)
type ColdStore interface {
	Archive(ctx context.Context, items []*TieredMemoryItem) error
	Retrieve(ctx context.Context, key string) (*TieredMemoryItem, error)
	List(ctx context.Context, prefix string, limit int) ([]string, error)
	Delete(ctx context.Context, key string) error
	Close() error
}

// TieredMemoryItem represents an item in the tiered memory system
type TieredMemoryItem struct {
	Key         string                 `json:"key"`
	Value       []byte                 `json:"value"`
	Tier        TierType               `json:"tier"`
	AgentID     string                 `json:"agent_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	ItemType    string                 `json:"item_type"` // "message", "snapshot", "memory", etc.
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessedAt  time.Time              `json:"accessed_at"`
	AccessCount int64                  `json:"access_count"`
}

// TieredMemoryQuery defines query parameters for warm/cold storage
type TieredMemoryQuery struct {
	AgentID     string
	SessionID   string
	ItemType    string
	FromTime    *time.Time
	ToTime      *time.Time
	Limit       int
	Offset      int
	OrderBy     string // "created_at", "accessed_at", "access_count"
	OrderDesc   bool
}

// NewTieredMemoryManager creates a new tiered memory manager.
// All stores are optional - pass nil for any tier you don't need.
//
// Example with all tiers:
//
//	manager := NewTieredMemoryManager(hotStore, warmStore, coldStore, config)
//
// Example with only hot and warm:
//
//	manager := NewTieredMemoryManager(hotStore, warmStore, nil, config)
//
// Example with only in-memory hot tier:
//
//	manager := NewTieredMemoryManager(NewMemoryHotStore(), nil, nil, DefaultTieredMemoryConfig())
func NewTieredMemoryManager(hot HotStore, warm WarmStore, cold ColdStore, config TieredMemoryConfig) *TieredMemoryManager {
	if config.HotTTL == 0 {
		config.HotTTL = 1 * time.Hour
	}
	if config.WarmTTL == 0 {
		config.WarmTTL = 7 * 24 * time.Hour
	}
	if config.HotMaxItems == 0 {
		config.HotMaxItems = 10000
	}
	if config.CompactionInterval == 0 {
		config.CompactionInterval = 1 * time.Hour
	}
	if config.ArchiveInterval == 0 {
		config.ArchiveInterval = 24 * time.Hour
	}

	return &TieredMemoryManager{
		hotStore:  hot,
		warmStore: warm,
		coldStore: cold,
		config:    config,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// NewTieredMemoryManagerWithOptions creates a tiered memory manager using functional options.
// This provides a more flexible API that doesn't require all parameters.
//
// Example:
//
//	manager := NewTieredMemoryManagerWithOptions(
//	    WithHotStore(redisHotStore),
//	    WithWarmStore(postgresWarmStore),
//	    WithHotTTL(2 * time.Hour),
//	)
func NewTieredMemoryManagerWithOptions(opts ...TieredMemoryOption) *TieredMemoryManager {
	m := &TieredMemoryManager{
		config: DefaultTieredMemoryConfig(),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithHotStore sets the hot tier storage
func WithHotStore(store HotStore) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.hotStore = store
	}
}

// WithWarmStore sets the warm tier storage
func WithWarmStore(store WarmStore) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.warmStore = store
	}
}

// WithColdStore sets the cold tier storage
func WithColdStore(store ColdStore) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.coldStore = store
	}
}

// WithHotTTL sets how long items stay in hot storage
func WithHotTTL(ttl time.Duration) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.HotTTL = ttl
	}
}

// WithWarmTTL sets how long items stay in warm storage before archiving
func WithWarmTTL(ttl time.Duration) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.WarmTTL = ttl
	}
}

// WithHotMaxItems sets the maximum number of items in hot storage
func WithHotMaxItems(max int) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.HotMaxItems = max
	}
}

// WithPromoteOnAccess enables/disables automatic promotion to hot tier on access
func WithPromoteOnAccess(enable bool) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.PromoteOnAccess = enable
	}
}

// WithCompactionInterval sets how often to run compaction
func WithCompactionInterval(interval time.Duration) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.CompactionInterval = interval
	}
}

// WithArchiveInterval sets how often to archive old data
func WithArchiveInterval(interval time.Duration) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.config.ArchiveInterval = interval
	}
}

// WithInMemoryHotStore is a convenience option that creates an in-memory hot store
func WithInMemoryHotStore() TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.hotStore = NewMemoryHotStore()
	}
}

// WithInMemoryWarmStore is a convenience option that creates an in-memory warm store
func WithInMemoryWarmStore() TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.warmStore = NewMemoryWarmStore()
	}
}

// WithSnapshotStoreAsWarm wraps an existing SnapshotStore as the warm tier
func WithSnapshotStoreAsWarm(store snapshot.SnapshotStore) TieredMemoryOption {
	return func(m *TieredMemoryManager) {
		m.warmStore = NewSnapshotStoreWarmAdapter(store)
	}
}

// Start begins background workers for tier management
func (m *TieredMemoryManager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.backgroundWorker()
}

// Stop stops background workers
func (m *TieredMemoryManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	close(m.stopCh)
	<-m.doneCh
}

// Get retrieves an item, checking all tiers
func (m *TieredMemoryManager) Get(ctx context.Context, key string) (*TieredMemoryItem, error) {
	// Try hot tier first
	if m.hotStore != nil {
		data, err := m.hotStore.Get(ctx, key)
		if err == nil && len(data) > 0 {
			var item TieredMemoryItem
			if err := json.Unmarshal(data, &item); err == nil {
				m.recordHit(TierHot)
				// Refresh TTL on access
				m.hotStore.Expire(ctx, key, m.config.HotTTL)
				return &item, nil
			}
		}
		m.recordMiss(TierHot)
	}

	// Try warm tier
	if m.warmStore != nil {
		item, err := m.warmStore.Get(ctx, key)
		if err == nil && item != nil {
			m.recordHit(TierWarm)
			// Promote to hot tier if configured
			if m.config.PromoteOnAccess && m.hotStore != nil {
				m.promoteToHot(ctx, item)
			}
			return item, nil
		}
		m.recordMiss(TierWarm)
	}

	// Try cold tier
	if m.coldStore != nil {
		item, err := m.coldStore.Retrieve(ctx, key)
		if err == nil && item != nil {
			m.recordHit(TierCold)
			// Promote to warm tier
			if m.warmStore != nil {
				m.warmStore.Set(ctx, item)
			}
			// Optionally promote to hot tier
			if m.config.PromoteOnAccess && m.hotStore != nil {
				m.promoteToHot(ctx, item)
			}
			return item, nil
		}
		m.recordMiss(TierCold)
	}

	return nil, fmt.Errorf("item not found: %s", key)
}

// Set stores an item in the hot tier
func (m *TieredMemoryManager) Set(ctx context.Context, item *TieredMemoryItem) error {
	if item.Key == "" {
		return fmt.Errorf("key is required")
	}

	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	item.AccessedAt = now
	item.Tier = TierHot

	// Store in hot tier
	if m.hotStore != nil {
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}
		if err := m.hotStore.Set(ctx, item.Key, data, m.config.HotTTL); err != nil {
			return fmt.Errorf("failed to store in hot tier: %w", err)
		}
	}

	// Also store in warm tier for durability
	if m.warmStore != nil {
		if err := m.warmStore.Set(ctx, item); err != nil {
			// Log but don't fail - hot tier succeeded
			_ = err
		}
	}

	return nil
}

// Delete removes an item from all tiers
func (m *TieredMemoryManager) Delete(ctx context.Context, key string) error {
	var lastErr error

	if m.hotStore != nil {
		if err := m.hotStore.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	if m.warmStore != nil {
		if err := m.warmStore.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	if m.coldStore != nil {
		if err := m.coldStore.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Query searches across warm and cold tiers
func (m *TieredMemoryManager) Query(ctx context.Context, query *TieredMemoryQuery) ([]*TieredMemoryItem, error) {
	var results []*TieredMemoryItem

	// Query warm tier
	if m.warmStore != nil {
		items, err := m.warmStore.Query(ctx, query)
		if err == nil {
			results = append(results, items...)
		}
	}

	return results, nil
}

// promoteToHot promotes an item to the hot tier
func (m *TieredMemoryManager) promoteToHot(ctx context.Context, item *TieredMemoryItem) {
	item.Tier = TierHot
	item.AccessedAt = time.Now()
	item.AccessCount++

	data, err := json.Marshal(item)
	if err != nil {
		return
	}

	m.hotStore.Set(ctx, item.Key, data, m.config.HotTTL)
	m.metrics.mu.Lock()
	m.metrics.Promotions++
	m.metrics.mu.Unlock()
}

// demoteToWarm demotes an item from hot to warm tier
func (m *TieredMemoryManager) demoteToWarm(ctx context.Context, key string) error {
	if m.hotStore == nil || m.warmStore == nil {
		return nil
	}

	// Get from hot
	data, err := m.hotStore.Get(ctx, key)
	if err != nil {
		return err
	}

	var item TieredMemoryItem
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}

	item.Tier = TierWarm
	item.UpdatedAt = time.Now()

	// Store in warm
	if err := m.warmStore.Set(ctx, &item); err != nil {
		return err
	}

	// Delete from hot
	m.hotStore.Delete(ctx, key)

	m.metrics.mu.Lock()
	m.metrics.Demotions++
	m.metrics.mu.Unlock()

	return nil
}

// archiveToCol archives items from warm to cold tier
func (m *TieredMemoryManager) archiveToCold(ctx context.Context) error {
	if m.warmStore == nil || m.coldStore == nil {
		return nil
	}

	// Get items older than warm TTL
	items, err := m.warmStore.GetOlderThan(ctx, m.config.WarmTTL, 100)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		return nil
	}

	// Archive to cold storage
	if err := m.coldStore.Archive(ctx, items); err != nil {
		return err
	}

	// Delete from warm storage
	for _, item := range items {
		m.warmStore.Delete(ctx, item.Key)
	}

	m.metrics.mu.Lock()
	m.metrics.Archivals += int64(len(items))
	m.metrics.mu.Unlock()

	return nil
}

// backgroundWorker runs periodic maintenance tasks
func (m *TieredMemoryManager) backgroundWorker() {
	defer close(m.doneCh)

	compactTicker := time.NewTicker(m.config.CompactionInterval)
	archiveTicker := time.NewTicker(m.config.ArchiveInterval)
	defer compactTicker.Stop()
	defer archiveTicker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-compactTicker.C:
			m.runCompaction(context.Background())
		case <-archiveTicker.C:
			m.archiveToCold(context.Background())
		}
	}
}

// runCompaction demotes cold items from hot to warm tier
func (m *TieredMemoryManager) runCompaction(ctx context.Context) {
	if m.hotStore == nil {
		return
	}

	// Get all keys from hot storage
	keys, err := m.hotStore.Keys(ctx, "*")
	if err != nil {
		return
	}

	// Check each item's age and access pattern
	for _, key := range keys {
		data, err := m.hotStore.Get(ctx, key)
		if err != nil {
			continue
		}

		var item TieredMemoryItem
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}

		// Demote if not accessed recently
		if time.Since(item.AccessedAt) > m.config.HotTTL {
			m.demoteToWarm(ctx, key)
		}
	}
}

// GetMetrics returns current metrics
func (m *TieredMemoryManager) GetMetrics() TieredMemoryMetrics {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	return m.metrics
}

// recordHit records a cache hit for a tier
func (m *TieredMemoryManager) recordHit(tier TierType) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	switch tier {
	case TierHot:
		m.metrics.HotHits++
	case TierWarm:
		m.metrics.WarmHits++
	case TierCold:
		m.metrics.ColdHits++
	}
}

// recordMiss records a cache miss for a tier
func (m *TieredMemoryManager) recordMiss(tier TierType) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	switch tier {
	case TierHot:
		m.metrics.HotMisses++
	case TierWarm:
		m.metrics.WarmMisses++
	case TierCold:
		m.metrics.ColdMisses++
	}
}

// Close closes all storage backends
func (m *TieredMemoryManager) Close() error {
	m.Stop()

	var lastErr error
	if m.hotStore != nil {
		if err := m.hotStore.Close(); err != nil {
			lastErr = err
		}
	}
	if m.warmStore != nil {
		if err := m.warmStore.Close(); err != nil {
			lastErr = err
		}
	}
	if m.coldStore != nil {
		if err := m.coldStore.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// MemoryHotStore wraps an in-memory map as a HotStore
type MemoryHotStore struct {
	mu    sync.RWMutex
	data  map[string]hotStoreEntry
	ttl   time.Duration
}

type hotStoreEntry struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryHotStore creates an in-memory hot store
func NewMemoryHotStore() *MemoryHotStore {
	return &MemoryHotStore{
		data: make(map[string]hotStoreEntry),
	}
}

func (s *MemoryHotStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, fmt.Errorf("expired")
	}
	return entry.value, nil
}

func (s *MemoryHotStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = hotStoreEntry{value: value, expiresAt: expiresAt}
	return nil
}

func (s *MemoryHotStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *MemoryHotStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *MemoryHotStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.data[key]; ok {
		entry.expiresAt = time.Now().Add(ttl)
		s.data[key] = entry
	}
	return nil
}

func (s *MemoryHotStore) Close() error {
	return nil
}

// MemoryWarmStore wraps an in-memory map as a WarmStore (for testing)
type MemoryWarmStore struct {
	mu   sync.RWMutex
	data map[string]*TieredMemoryItem
}

// NewMemoryWarmStore creates an in-memory warm store
func NewMemoryWarmStore() *MemoryWarmStore {
	return &MemoryWarmStore{
		data: make(map[string]*TieredMemoryItem),
	}
}

func (s *MemoryWarmStore) Get(ctx context.Context, key string) (*TieredMemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return item, nil
}

func (s *MemoryWarmStore) Set(ctx context.Context, item *TieredMemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[item.Key] = item
	return nil
}

func (s *MemoryWarmStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *MemoryWarmStore) Query(ctx context.Context, query *TieredMemoryQuery) ([]*TieredMemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*TieredMemoryItem
	for _, item := range s.data {
		if query.AgentID != "" && item.AgentID != query.AgentID {
			continue
		}
		if query.SessionID != "" && item.SessionID != query.SessionID {
			continue
		}
		if query.ItemType != "" && item.ItemType != query.ItemType {
			continue
		}
		results = append(results, item)
	}
	return results, nil
}

func (s *MemoryWarmStore) GetOlderThan(ctx context.Context, age time.Duration, limit int) ([]*TieredMemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-age)
	var results []*TieredMemoryItem
	for _, item := range s.data {
		if item.UpdatedAt.Before(cutoff) {
			results = append(results, item)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (s *MemoryWarmStore) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-age)
	var deleted int64
	for key, item := range s.data {
		if item.UpdatedAt.Before(cutoff) {
			delete(s.data, key)
			deleted++
		}
	}
	return deleted, nil
}

func (s *MemoryWarmStore) Close() error {
	return nil
}

// SnapshotStoreWarmAdapter adapts a snapshot.SnapshotStore to the WarmStore interface
type SnapshotStoreWarmAdapter struct {
	store snapshot.SnapshotStore
}

// NewSnapshotStoreWarmAdapter creates a warm store adapter from a snapshot store
func NewSnapshotStoreWarmAdapter(store snapshot.SnapshotStore) *SnapshotStoreWarmAdapter {
	return &SnapshotStoreWarmAdapter{store: store}
}

func (a *SnapshotStoreWarmAdapter) Get(ctx context.Context, key string) (*TieredMemoryItem, error) {
	snap, err := a.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return snapshotToTieredItem(snap), nil
}

func (a *SnapshotStoreWarmAdapter) Set(ctx context.Context, item *TieredMemoryItem) error {
	snap := tieredItemToSnapshot(item)
	return a.store.Save(ctx, snap)
}

func (a *SnapshotStoreWarmAdapter) Delete(ctx context.Context, key string) error {
	_, err := a.store.PurgeExecution(ctx, key)
	return err
}

func (a *SnapshotStoreWarmAdapter) Query(ctx context.Context, query *TieredMemoryQuery) ([]*TieredMemoryItem, error) {
	snapQuery := &snapshot.SnapshotQuery{
		Filter: snapshot.SnapshotFilter{
			AgentID:   query.AgentID,
			SessionID: query.SessionID,
			FromTime:  query.FromTime,
			ToTime:    query.ToTime,
		},
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	result, err := a.store.Query(ctx, snapQuery)
	if err != nil {
		return nil, err
	}

	items := make([]*TieredMemoryItem, len(result.Snapshots))
	for i, snap := range result.Snapshots {
		items[i] = snapshotToTieredItem(snap)
	}
	return items, nil
}

func (a *SnapshotStoreWarmAdapter) GetOlderThan(ctx context.Context, age time.Duration, limit int) ([]*TieredMemoryItem, error) {
	cutoff := time.Now().Add(-age)
	snapQuery := &snapshot.SnapshotQuery{
		Filter: snapshot.SnapshotFilter{
			ToTime: &cutoff,
		},
		Limit:   limit,
		OrderBy: "time_asc",
	}

	result, err := a.store.Query(ctx, snapQuery)
	if err != nil {
		return nil, err
	}

	items := make([]*TieredMemoryItem, len(result.Snapshots))
	for i, snap := range result.Snapshots {
		items[i] = snapshotToTieredItem(snap)
	}
	return items, nil
}

func (a *SnapshotStoreWarmAdapter) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	return a.store.PurgeOlderThan(ctx, age)
}

func (a *SnapshotStoreWarmAdapter) Close() error {
	return a.store.Close()
}

func snapshotToTieredItem(snap *snapshot.ExecutionSnapshot) *TieredMemoryItem {
	data, _ := json.Marshal(snap)
	return &TieredMemoryItem{
		Key:        snap.ID,
		Value:      data,
		Tier:       TierWarm,
		AgentID:    snap.AgentID,
		SessionID:  snap.SessionID,
		ItemType:   "snapshot",
		CreatedAt:  snap.Timestamp,
		UpdatedAt:  snap.Timestamp,
		AccessedAt: snap.Timestamp,
	}
}

func tieredItemToSnapshot(item *TieredMemoryItem) *snapshot.ExecutionSnapshot {
	var snap snapshot.ExecutionSnapshot
	json.Unmarshal(item.Value, &snap)
	if snap.ID == "" {
		snap.ID = item.Key
	}
	if snap.AgentID == "" {
		snap.AgentID = item.AgentID
	}
	if snap.SessionID == "" {
		snap.SessionID = item.SessionID
	}
	return &snap
}

// Ensure interfaces are implemented
var _ HotStore = (*MemoryHotStore)(nil)
var _ WarmStore = (*MemoryWarmStore)(nil)
var _ WarmStore = (*SnapshotStoreWarmAdapter)(nil)
