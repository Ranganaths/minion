package coordination

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LockType represents the type of lock
type LockType string

const (
	LockTypeExclusive LockType = "exclusive"
	LockTypeShared    LockType = "shared"
)

// LockState represents the state of a lock
type LockState string

const (
	LockStateUnlocked LockState = "unlocked"
	LockStateLocked   LockState = "locked"
	LockStateWaiting  LockState = "waiting"
)

// DistributedLock represents a distributed lock
type DistributedLock struct {
	ID        string
	Key       string
	Owner     string
	Type      LockType
	State     LockState
	AcquiredAt time.Time
	ExpiresAt  time.Time
	Metadata  map[string]string
}

// LockConfig configures a lock acquisition
type LockConfig struct {
	Key           string
	Owner         string
	Type          LockType
	TTL           time.Duration
	RetryInterval time.Duration
	MaxRetries    int
	WaitTimeout   time.Duration
}

// LockStore interface for lock state persistence
type LockStore interface {
	// TryAcquire attempts to acquire a lock
	TryAcquire(ctx context.Context, config LockConfig) (*DistributedLock, error)

	// Release releases a lock
	Release(ctx context.Context, key, owner string) error

	// Renew renews a lock's TTL
	Renew(ctx context.Context, key, owner string, ttl time.Duration) error

	// Get gets a lock's current state
	Get(ctx context.Context, key string) (*DistributedLock, error)

	// List lists all locks matching a pattern
	List(ctx context.Context, pattern string) ([]*DistributedLock, error)
}

// LockManager manages distributed locks
type LockManager struct {
	store     LockStore
	locks     map[string]*DistributedLock
	callbacks map[string][]func(*DistributedLock)
	mu        sync.RWMutex
}

// NewLockManager creates a new lock manager
func NewLockManager(store LockStore) *LockManager {
	return &LockManager{
		store:     store,
		locks:     make(map[string]*DistributedLock),
		callbacks: make(map[string][]func(*DistributedLock)),
	}
}

// Acquire acquires a lock with the given configuration
func (m *LockManager) Acquire(ctx context.Context, config LockConfig) (*DistributedLock, error) {
	if config.Key == "" {
		return nil, errors.New("lock key is required")
	}
	if config.Owner == "" {
		config.Owner = uuid.New().String()
	}
	if config.TTL == 0 {
		config.TTL = 30 * time.Second
	}
	if config.Type == "" {
		config.Type = LockTypeExclusive
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 100 * time.Millisecond
	}

	// Try immediate acquisition
	lock, err := m.store.TryAcquire(ctx, config)
	if err == nil && lock != nil {
		m.mu.Lock()
		m.locks[config.Key] = lock
		m.mu.Unlock()
		m.notifyCallbacks(lock)
		return lock, nil
	}

	// If no wait timeout or immediate failure requested
	if config.WaitTimeout == 0 && config.MaxRetries == 0 {
		return nil, errors.New("failed to acquire lock: already held")
	}

	// Retry with backoff
	retries := 0
	deadline := time.Now().Add(config.WaitTimeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if config.WaitTimeout > 0 && time.Now().After(deadline) {
			return nil, errors.New("timeout waiting for lock")
		}

		if config.MaxRetries > 0 && retries >= config.MaxRetries {
			return nil, errors.New("max retries exceeded")
		}

		time.Sleep(config.RetryInterval)
		retries++

		lock, err = m.store.TryAcquire(ctx, config)
		if err == nil && lock != nil {
			m.mu.Lock()
			m.locks[config.Key] = lock
			m.mu.Unlock()
			m.notifyCallbacks(lock)
			return lock, nil
		}
	}
}

// Release releases a lock
func (m *LockManager) Release(ctx context.Context, key, owner string) error {
	err := m.store.Release(ctx, key, owner)
	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.locks, key)
	m.mu.Unlock()

	m.notifyCallbacks(&DistributedLock{
		Key:   key,
		State: LockStateUnlocked,
	})

	return nil
}

// Renew renews a lock's TTL
func (m *LockManager) Renew(ctx context.Context, key, owner string, ttl time.Duration) error {
	return m.store.Renew(ctx, key, owner, ttl)
}

// Get gets a lock's current state
func (m *LockManager) Get(ctx context.Context, key string) (*DistributedLock, error) {
	return m.store.Get(ctx, key)
}

// IsLocked checks if a key is locked
func (m *LockManager) IsLocked(ctx context.Context, key string) (bool, error) {
	lock, err := m.store.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return lock != nil && lock.State == LockStateLocked && time.Now().Before(lock.ExpiresAt), nil
}

// OnLockChange registers a callback for lock state changes
func (m *LockManager) OnLockChange(key string, callback func(*DistributedLock)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks[key] = append(m.callbacks[key], callback)
}

// notifyCallbacks notifies all callbacks for a lock
func (m *LockManager) notifyCallbacks(lock *DistributedLock) {
	m.mu.RLock()
	callbacks := m.callbacks[lock.Key]
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(lock)
	}
}

// WithLock executes a function while holding a lock
func (m *LockManager) WithLock(ctx context.Context, config LockConfig, fn func() error) error {
	lock, err := m.Acquire(ctx, config)
	if err != nil {
		return err
	}
	defer m.Release(ctx, lock.Key, lock.Owner)

	return fn()
}

// InMemoryLockStore is an in-memory implementation of LockStore
type InMemoryLockStore struct {
	locks map[string]*DistributedLock
	mu    sync.RWMutex
}

// NewInMemoryLockStore creates a new in-memory lock store
func NewInMemoryLockStore() *InMemoryLockStore {
	return &InMemoryLockStore{
		locks: make(map[string]*DistributedLock),
	}
}

// TryAcquire attempts to acquire a lock
func (s *InMemoryLockStore) TryAcquire(ctx context.Context, config LockConfig) (*DistributedLock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.locks[config.Key]
	now := time.Now()

	// Check if lock exists and is still valid
	if existing != nil && existing.State == LockStateLocked && now.Before(existing.ExpiresAt) {
		// For shared locks, allow multiple readers
		if config.Type == LockTypeShared && existing.Type == LockTypeShared {
			// Allow shared lock
			lock := &DistributedLock{
				ID:         uuid.New().String(),
				Key:        config.Key,
				Owner:      config.Owner,
				Type:       config.Type,
				State:      LockStateLocked,
				AcquiredAt: now,
				ExpiresAt:  now.Add(config.TTL),
			}
			return lock, nil
		}
		return nil, errors.New("lock already held")
	}

	// Create new lock
	lock := &DistributedLock{
		ID:         uuid.New().String(),
		Key:        config.Key,
		Owner:      config.Owner,
		Type:       config.Type,
		State:      LockStateLocked,
		AcquiredAt: now,
		ExpiresAt:  now.Add(config.TTL),
	}
	s.locks[config.Key] = lock

	return lock, nil
}

// Release releases a lock
func (s *InMemoryLockStore) Release(ctx context.Context, key, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.locks[key]
	if existing == nil {
		return errors.New("lock not found")
	}
	if existing.Owner != owner {
		return errors.New("not lock owner")
	}

	delete(s.locks, key)
	return nil
}

// Renew renews a lock's TTL
func (s *InMemoryLockStore) Renew(ctx context.Context, key, owner string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.locks[key]
	if existing == nil {
		return errors.New("lock not found")
	}
	if existing.Owner != owner {
		return errors.New("not lock owner")
	}

	existing.ExpiresAt = time.Now().Add(ttl)
	return nil
}

// Get gets a lock's current state
func (s *InMemoryLockStore) Get(ctx context.Context, key string) (*DistributedLock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lock := s.locks[key]
	if lock == nil {
		return nil, nil
	}

	// Check if expired
	if time.Now().After(lock.ExpiresAt) {
		return nil, nil
	}

	return lock, nil
}

// List lists all locks matching a pattern (simple prefix match for in-memory)
func (s *InMemoryLockStore) List(ctx context.Context, pattern string) ([]*DistributedLock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*DistributedLock
	now := time.Now()

	for key, lock := range s.locks {
		if pattern == "" || matchPattern(key, pattern) {
			if now.Before(lock.ExpiresAt) {
				result = append(result, lock)
			}
		}
	}

	return result, nil
}

// matchPattern performs a simple prefix match
func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return key == pattern
}

// Semaphore provides a distributed semaphore implementation
type Semaphore struct {
	key       string
	capacity  int
	manager   *LockManager
	holders   map[string]struct{}
	mu        sync.Mutex
}

// NewSemaphore creates a new distributed semaphore
func NewSemaphore(manager *LockManager, key string, capacity int) *Semaphore {
	return &Semaphore{
		key:      key,
		capacity: capacity,
		manager:  manager,
		holders:  make(map[string]struct{}),
	}
}

// Acquire acquires a semaphore slot
func (s *Semaphore) Acquire(ctx context.Context, ttl time.Duration) (string, error) {
	s.mu.Lock()
	if len(s.holders) >= s.capacity {
		s.mu.Unlock()
		return "", errors.New("semaphore at capacity")
	}

	holderID := uuid.New().String()
	s.holders[holderID] = struct{}{}
	s.mu.Unlock()

	// Also acquire underlying lock for coordination
	config := LockConfig{
		Key:   s.key + ":" + holderID,
		Owner: holderID,
		Type:  LockTypeShared,
		TTL:   ttl,
	}

	_, err := s.manager.Acquire(ctx, config)
	if err != nil {
		s.mu.Lock()
		delete(s.holders, holderID)
		s.mu.Unlock()
		return "", err
	}

	return holderID, nil
}

// Release releases a semaphore slot
func (s *Semaphore) Release(ctx context.Context, holderID string) error {
	s.mu.Lock()
	if _, ok := s.holders[holderID]; !ok {
		s.mu.Unlock()
		return errors.New("holder not found")
	}
	delete(s.holders, holderID)
	s.mu.Unlock()

	return s.manager.Release(ctx, s.key+":"+holderID, holderID)
}

// Available returns the number of available slots
func (s *Semaphore) Available() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capacity - len(s.holders)
}

// Barrier provides a distributed barrier implementation
type Barrier struct {
	key       string
	count     int
	manager   *LockManager
	waiters   map[string]chan struct{}
	arrived   int
	mu        sync.Mutex
}

// NewBarrier creates a new distributed barrier
func NewBarrier(manager *LockManager, key string, count int) *Barrier {
	return &Barrier{
		key:     key,
		count:   count,
		manager: manager,
		waiters: make(map[string]chan struct{}),
	}
}

// Wait waits at the barrier until all participants arrive
func (b *Barrier) Wait(ctx context.Context, participantID string) error {
	b.mu.Lock()

	// Check if already waiting
	if _, ok := b.waiters[participantID]; ok {
		b.mu.Unlock()
		return errors.New("participant already waiting")
	}

	// Create wait channel
	waitCh := make(chan struct{})
	b.waiters[participantID] = waitCh
	b.arrived++

	// Check if all arrived
	if b.arrived >= b.count {
		// Release all waiters
		for _, ch := range b.waiters {
			close(ch)
		}
		b.waiters = make(map[string]chan struct{})
		b.arrived = 0
		b.mu.Unlock()
		return nil
	}

	b.mu.Unlock()

	// Wait for release or context cancellation
	select {
	case <-waitCh:
		return nil
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.waiters, participantID)
		b.arrived--
		b.mu.Unlock()
		return ctx.Err()
	}
}

// Reset resets the barrier
func (b *Barrier) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Cancel all waiters
	for _, ch := range b.waiters {
		close(ch)
	}
	b.waiters = make(map[string]chan struct{})
	b.arrived = 0
}

// Count returns the barrier count
func (b *Barrier) Count() int {
	return b.count
}

// Arrived returns the number of arrived participants
func (b *Barrier) Arrived() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.arrived
}
