package coordination

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ElectionState represents the state of a leader election
type ElectionState string

const (
	ElectionStateFollower  ElectionState = "follower"
	ElectionStateCandidate ElectionState = "candidate"
	ElectionStateLeader    ElectionState = "leader"
)

// LeaderElection provides leader election capabilities
type LeaderElection struct {
	ID            string
	NodeID        string
	ElectionKey   string
	State         ElectionState
	CurrentLeader string
	Term          int64
	LeaseDuration time.Duration
	LeaseExpiry   time.Time
	store         ElectionStore
	onLeader      func(ctx context.Context)
	onFollower    func(ctx context.Context)
	onResign      func(ctx context.Context)
	stopCh        chan struct{}
	mu            sync.RWMutex
}

// ElectionStore interface for election state persistence
type ElectionStore interface {
	// TryAcquire attempts to acquire leadership
	TryAcquire(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error)

	// Renew renews the leadership lease
	Renew(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error)

	// GetLeader returns the current leader
	GetLeader(ctx context.Context, key string) (*LeaderInfo, error)

	// Release releases leadership
	Release(ctx context.Context, key, nodeID string) error

	// Watch watches for leadership changes
	Watch(ctx context.Context, key string) (<-chan *LeaderInfo, error)
}

// LeaderInfo contains information about the current leader
type LeaderInfo struct {
	NodeID      string    `json:"node_id"`
	Term        int64     `json:"term"`
	AcquiredAt  time.Time `json:"acquired_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ElectionConfig configures leader election
type ElectionConfig struct {
	ElectionKey   string
	NodeID        string
	LeaseDuration time.Duration
	RetryInterval time.Duration
	OnLeader      func(ctx context.Context)
	OnFollower    func(ctx context.Context)
	OnResign      func(ctx context.Context)
}

// NewLeaderElection creates a new leader election
func NewLeaderElection(store ElectionStore, config ElectionConfig) *LeaderElection {
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 15 * time.Second
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 5 * time.Second
	}
	if config.NodeID == "" {
		config.NodeID = uuid.New().String()
	}

	return &LeaderElection{
		ID:            uuid.New().String(),
		NodeID:        config.NodeID,
		ElectionKey:   config.ElectionKey,
		State:         ElectionStateFollower,
		LeaseDuration: config.LeaseDuration,
		store:         store,
		onLeader:      config.OnLeader,
		onFollower:    config.OnFollower,
		onResign:      config.OnResign,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the leader election process
func (e *LeaderElection) Start(ctx context.Context) error {
	// Start election loop
	go e.electionLoop(ctx)

	return nil
}

// Stop stops the leader election process
func (e *LeaderElection) Stop(ctx context.Context) error {
	close(e.stopCh)

	// Resign if leader
	if e.IsLeader() {
		return e.Resign(ctx)
	}
	return nil
}

// IsLeader returns true if this node is the leader
func (e *LeaderElection) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.State == ElectionStateLeader
}

// GetState returns the current election state
func (e *LeaderElection) GetState() ElectionState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.State
}

// GetLeader returns the current leader ID
func (e *LeaderElection) GetLeader() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.CurrentLeader
}

// GetTerm returns the current term
func (e *LeaderElection) GetTerm() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Term
}

// Resign resigns from leadership
func (e *LeaderElection) Resign(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State != ElectionStateLeader {
		return errors.New("not the leader")
	}

	err := e.store.Release(ctx, e.ElectionKey, e.NodeID)
	if err != nil {
		return err
	}

	e.State = ElectionStateFollower
	e.CurrentLeader = ""

	if e.onResign != nil {
		go e.onResign(ctx)
	}

	return nil
}

// electionLoop runs the main election loop
func (e *LeaderElection) electionLoop(ctx context.Context) {
	// Try election immediately on start
	e.tryElection(ctx)

	ticker := time.NewTicker(e.LeaseDuration / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tryElection(ctx)
		}
	}
}

// tryElection attempts to become or remain the leader
func (e *LeaderElection) tryElection(ctx context.Context) {
	e.mu.Lock()
	currentState := e.State
	term := e.Term
	e.mu.Unlock()

	switch currentState {
	case ElectionStateFollower, ElectionStateCandidate:
		// Check if we can become leader
		info, err := e.store.GetLeader(ctx, e.ElectionKey)
		if err != nil || info == nil || time.Now().After(info.ExpiresAt) {
			// No leader or lease expired, try to become leader immediately
			e.mu.Lock()
			e.State = ElectionStateCandidate
			newTerm := e.Term + 1
			e.Term = newTerm
			e.mu.Unlock()

			// Try to acquire leadership
			acquired, err := e.store.TryAcquire(ctx, e.ElectionKey, e.NodeID, newTerm, e.LeaseDuration)
			if err == nil && acquired {
				e.mu.Lock()
				e.State = ElectionStateLeader
				e.CurrentLeader = e.NodeID
				e.LeaseExpiry = time.Now().Add(e.LeaseDuration)
				e.mu.Unlock()

				if e.onLeader != nil {
					go e.onLeader(ctx)
				}
			} else {
				// Failed to acquire, go back to follower
				e.mu.Lock()
				e.State = ElectionStateFollower
				e.mu.Unlock()

				if e.onFollower != nil {
					go e.onFollower(ctx)
				}
			}
		} else {
			// There is a leader, stay as follower
			e.mu.Lock()
			e.State = ElectionStateFollower
			e.CurrentLeader = info.NodeID
			e.mu.Unlock()
		}

	case ElectionStateLeader:
		// Renew lease
		renewed, err := e.store.Renew(ctx, e.ElectionKey, e.NodeID, term, e.LeaseDuration)
		if err != nil || !renewed {
			// Lost leadership
			e.mu.Lock()
			e.State = ElectionStateFollower
			e.CurrentLeader = ""
			e.mu.Unlock()

			if e.onFollower != nil {
				go e.onFollower(ctx)
			}
		} else {
			e.mu.Lock()
			e.LeaseExpiry = time.Now().Add(e.LeaseDuration)
			e.mu.Unlock()
		}
	}
}

// InMemoryElectionStore is an in-memory implementation of ElectionStore
type InMemoryElectionStore struct {
	leaders  map[string]*LeaderInfo
	watchers map[string][]chan *LeaderInfo
	mu       sync.RWMutex
}

// NewInMemoryElectionStore creates a new in-memory election store
func NewInMemoryElectionStore() *InMemoryElectionStore {
	return &InMemoryElectionStore{
		leaders:  make(map[string]*LeaderInfo),
		watchers: make(map[string][]chan *LeaderInfo),
	}
}

// TryAcquire attempts to acquire leadership
func (s *InMemoryElectionStore) TryAcquire(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.leaders[key]
	now := time.Now()

	// Check if lease is expired or no leader
	if current == nil || now.After(current.ExpiresAt) || current.Term < term {
		info := &LeaderInfo{
			NodeID:     nodeID,
			Term:       term,
			AcquiredAt: now,
			ExpiresAt:  now.Add(leaseDuration),
		}
		s.leaders[key] = info
		s.notifyWatchers(key, info)
		return true, nil
	}

	return false, nil
}

// Renew renews the leadership lease
func (s *InMemoryElectionStore) Renew(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.leaders[key]
	if current == nil || current.NodeID != nodeID || current.Term != term {
		return false, nil
	}

	current.ExpiresAt = time.Now().Add(leaseDuration)
	s.notifyWatchers(key, current)
	return true, nil
}

// GetLeader returns the current leader
func (s *InMemoryElectionStore) GetLeader(ctx context.Context, key string) (*LeaderInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.leaders[key], nil
}

// Release releases leadership
func (s *InMemoryElectionStore) Release(ctx context.Context, key, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.leaders[key]
	if current == nil || current.NodeID != nodeID {
		return errors.New("not the current leader")
	}

	delete(s.leaders, key)
	s.notifyWatchers(key, nil)
	return nil
}

// Watch watches for leadership changes
func (s *InMemoryElectionStore) Watch(ctx context.Context, key string) (<-chan *LeaderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan *LeaderInfo, 10)
	s.watchers[key] = append(s.watchers[key], ch)

	// Send current state
	if leader := s.leaders[key]; leader != nil {
		ch <- leader
	}

	// Clean up when context is cancelled
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		watchers := s.watchers[key]
		for i, w := range watchers {
			if w == ch {
				s.watchers[key] = append(watchers[:i], watchers[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

// notifyWatchers notifies all watchers of a key
func (s *InMemoryElectionStore) notifyWatchers(key string, info *LeaderInfo) {
	for _, ch := range s.watchers[key] {
		select {
		case ch <- info:
		default:
			// Channel full, skip
		}
	}
}

// ElectionManager manages multiple leader elections
type ElectionManager struct {
	store     ElectionStore
	elections map[string]*LeaderElection
	mu        sync.RWMutex
}

// NewElectionManager creates a new election manager
func NewElectionManager(store ElectionStore) *ElectionManager {
	return &ElectionManager{
		store:     store,
		elections: make(map[string]*LeaderElection),
	}
}

// CreateElection creates a new leader election
func (m *ElectionManager) CreateElection(config ElectionConfig) (*LeaderElection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	election := NewLeaderElection(m.store, config)
	m.elections[election.ElectionKey] = election
	return election, nil
}

// GetElection retrieves an election by key
func (m *ElectionManager) GetElection(key string) (*LeaderElection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	election, ok := m.elections[key]
	return election, ok
}

// StopAll stops all elections
func (m *ElectionManager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, election := range m.elections {
		if err := election.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}
