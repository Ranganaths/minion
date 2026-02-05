package coordination

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ConsulConfig configures the Consul connection
type ConsulConfig struct {
	Address    string
	Scheme     string // http or https
	Datacenter string
	Token      string
	TLSConfig  *ConsulTLSConfig
}

// ConsulTLSConfig holds TLS configuration for Consul
type ConsulTLSConfig struct {
	CAFile   string
	CertFile string
	KeyFile  string
}

// ConsulClient interface abstracts the Consul client for testing
type ConsulClient interface {
	// KV Operations
	Get(ctx context.Context, key string) ([]byte, error)
	GetWithPrefix(ctx context.Context, prefix string) (map[string][]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	DeleteWithPrefix(ctx context.Context, prefix string) error

	// Session Operations
	CreateSession(ctx context.Context, ttl time.Duration, behavior string) (string, error)
	DestroySession(ctx context.Context, sessionID string) error
	RenewSession(ctx context.Context, sessionID string) error

	// Lock Operations
	AcquireLock(ctx context.Context, key, sessionID string, value []byte) (bool, error)
	ReleaseLock(ctx context.Context, key, sessionID string) error

	// Watch Operations
	Watch(ctx context.Context, key string, index uint64) ([]byte, uint64, error)
	WatchPrefix(ctx context.Context, prefix string, index uint64) (map[string][]byte, uint64, error)

	// Service Registration
	RegisterService(ctx context.Context, service *ConsulServiceRegistration) error
	DeregisterService(ctx context.Context, serviceID string) error
	GetService(ctx context.Context, serviceName string) ([]*ConsulServiceInstance, error)

	// Health Checks
	UpdateHealthCheck(ctx context.Context, checkID, status, output string) error
}

// ConsulServiceRegistration represents a service registration
type ConsulServiceRegistration struct {
	ID      string
	Name    string
	Tags    []string
	Port    int
	Address string
	Check   *ConsulHealthCheck
	Meta    map[string]string
}

// ConsulHealthCheck represents a health check configuration
type ConsulHealthCheck struct {
	CheckID                        string
	TTL                            time.Duration
	DeregisterCriticalServiceAfter time.Duration
}

// ConsulServiceInstance represents a discovered service instance
type ConsulServiceInstance struct {
	ID      string
	Name    string
	Address string
	Port    int
	Tags    []string
	Meta    map[string]string
	Healthy bool
}

// ConsulElectionStore implements ElectionStore using Consul
type ConsulElectionStore struct {
	client   ConsulClient
	prefix   string
	mu       sync.RWMutex
	sessions map[string]string // key -> sessionID
}

// NewConsulElectionStore creates a new Consul-backed election store
func NewConsulElectionStore(client ConsulClient, prefix string) *ConsulElectionStore {
	if prefix == "" {
		prefix = "minion/elections"
	}
	return &ConsulElectionStore{
		client:   client,
		prefix:   prefix,
		sessions: make(map[string]string),
	}
}

// TryAcquire attempts to acquire leadership
func (s *ConsulElectionStore) TryAcquire(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	fullKey := s.prefix + "/" + key

	// Create a session with TTL
	sessionID, err := s.client.CreateSession(ctx, leaseDuration, "delete")
	if err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}

	// Create leader info
	now := time.Now()
	info := &LeaderInfo{
		NodeID:     nodeID,
		Term:       term,
		AcquiredAt: now,
		ExpiresAt:  now.Add(leaseDuration),
	}

	data, err := json.Marshal(info)
	if err != nil {
		s.client.DestroySession(ctx, sessionID)
		return false, fmt.Errorf("failed to marshal leader info: %w", err)
	}

	// Try to acquire lock
	acquired, err := s.client.AcquireLock(ctx, fullKey, sessionID, data)
	if err != nil {
		s.client.DestroySession(ctx, sessionID)
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		s.client.DestroySession(ctx, sessionID)
		return false, nil
	}

	// Store session for renewal
	s.mu.Lock()
	s.sessions[key] = sessionID
	s.mu.Unlock()

	// Start session renewal
	go s.renewSession(ctx, key, sessionID, leaseDuration)

	return true, nil
}

// Renew renews the leadership lease
func (s *ConsulElectionStore) Renew(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	// Get current leader
	leader, err := s.GetLeader(ctx, key)
	if err != nil {
		return false, err
	}

	if leader == nil || leader.NodeID != nodeID || leader.Term != term {
		return false, nil
	}

	// Renew session
	s.mu.RLock()
	sessionID := s.sessions[key]
	s.mu.RUnlock()

	if sessionID == "" {
		return false, errors.New("no session to renew")
	}

	err = s.client.RenewSession(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to renew session: %w", err)
	}

	// Update leader info with new expiry
	fullKey := s.prefix + "/" + key
	leader.ExpiresAt = time.Now().Add(leaseDuration)

	data, err := json.Marshal(leader)
	if err != nil {
		return false, err
	}

	return s.client.AcquireLock(ctx, fullKey, sessionID, data)
}

// GetLeader returns the current leader
func (s *ConsulElectionStore) GetLeader(ctx context.Context, key string) (*LeaderInfo, error) {
	fullKey := s.prefix + "/" + key

	data, err := s.client.Get(ctx, fullKey)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var info LeaderInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal leader info: %w", err)
	}

	return &info, nil
}

// Release releases leadership
func (s *ConsulElectionStore) Release(ctx context.Context, key, nodeID string) error {
	// Verify we are the leader
	leader, err := s.GetLeader(ctx, key)
	if err != nil {
		return err
	}

	if leader == nil || leader.NodeID != nodeID {
		return errors.New("not the current leader")
	}

	fullKey := s.prefix + "/" + key

	// Release lock and destroy session
	s.mu.Lock()
	sessionID := s.sessions[key]
	delete(s.sessions, key)
	s.mu.Unlock()

	if sessionID != "" {
		s.client.ReleaseLock(ctx, fullKey, sessionID)
		s.client.DestroySession(ctx, sessionID)
	}

	return nil
}

// Watch watches for leadership changes
func (s *ConsulElectionStore) Watch(ctx context.Context, key string) (<-chan *LeaderInfo, error) {
	fullKey := s.prefix + "/" + key
	infoCh := make(chan *LeaderInfo, 10)

	go func() {
		defer close(infoCh)
		var index uint64 = 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			data, newIndex, err := s.client.Watch(ctx, fullKey, index)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			index = newIndex

			if len(data) == 0 {
				select {
				case infoCh <- nil:
				default:
				}
				continue
			}

			var info LeaderInfo
			if err := json.Unmarshal(data, &info); err == nil {
				select {
				case infoCh <- &info:
				default:
				}
			}
		}
	}()

	return infoCh, nil
}

func (s *ConsulElectionStore) renewSession(ctx context.Context, key, sessionID string, ttl time.Duration) {
	ticker := time.NewTicker(ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			currentSession := s.sessions[key]
			s.mu.RUnlock()

			if currentSession != sessionID {
				return
			}

			if err := s.client.RenewSession(ctx, sessionID); err != nil {
				return
			}
		}
	}
}

// ConsulLockStore implements LockStore using Consul
type ConsulLockStore struct {
	client   ConsulClient
	prefix   string
	mu       sync.RWMutex
	sessions map[string]string
}

// NewConsulLockStore creates a new Consul-backed lock store
func NewConsulLockStore(client ConsulClient, prefix string) *ConsulLockStore {
	if prefix == "" {
		prefix = "minion/locks"
	}
	return &ConsulLockStore{
		client:   client,
		prefix:   prefix,
		sessions: make(map[string]string),
	}
}

// TryAcquire attempts to acquire a lock
func (s *ConsulLockStore) TryAcquire(ctx context.Context, config LockConfig) (*DistributedLock, error) {
	fullKey := s.prefix + "/" + config.Key

	// Check if lock exists for shared lock logic
	existing, _ := s.Get(ctx, config.Key)
	if existing != nil && existing.State == LockStateLocked && time.Now().Before(existing.ExpiresAt) {
		if config.Type == LockTypeShared && existing.Type == LockTypeShared {
			// Allow shared lock
		} else {
			return nil, errors.New("lock already held")
		}
	}

	// Create session
	sessionID, err := s.client.CreateSession(ctx, config.TTL, "delete")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create lock
	now := time.Now()
	lock := &DistributedLock{
		ID:         config.Key + "-" + config.Owner,
		Key:        config.Key,
		Owner:      config.Owner,
		Type:       config.Type,
		State:      LockStateLocked,
		AcquiredAt: now,
		ExpiresAt:  now.Add(config.TTL),
		Metadata:   make(map[string]string),
	}

	data, err := json.Marshal(lock)
	if err != nil {
		s.client.DestroySession(ctx, sessionID)
		return nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Try to acquire
	acquired, err := s.client.AcquireLock(ctx, fullKey, sessionID, data)
	if err != nil {
		s.client.DestroySession(ctx, sessionID)
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		s.client.DestroySession(ctx, sessionID)
		return nil, errors.New("lock already held")
	}

	s.mu.Lock()
	s.sessions[config.Key] = sessionID
	s.mu.Unlock()

	return lock, nil
}

// Release releases a lock
func (s *ConsulLockStore) Release(ctx context.Context, key, owner string) error {
	lock, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	if lock == nil {
		return errors.New("lock not found")
	}

	if lock.Owner != owner {
		return errors.New("not lock owner")
	}

	fullKey := s.prefix + "/" + key

	s.mu.Lock()
	sessionID := s.sessions[key]
	delete(s.sessions, key)
	s.mu.Unlock()

	if sessionID != "" {
		s.client.ReleaseLock(ctx, fullKey, sessionID)
		s.client.DestroySession(ctx, sessionID)
	}

	return nil
}

// Renew renews a lock's TTL
func (s *ConsulLockStore) Renew(ctx context.Context, key, owner string, ttl time.Duration) error {
	lock, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	if lock == nil {
		return errors.New("lock not found")
	}

	if lock.Owner != owner {
		return errors.New("not lock owner")
	}

	s.mu.RLock()
	sessionID := s.sessions[key]
	s.mu.RUnlock()

	if sessionID == "" {
		return errors.New("no session to renew")
	}

	return s.client.RenewSession(ctx, sessionID)
}

// Get gets a lock's current state
func (s *ConsulLockStore) Get(ctx context.Context, key string) (*DistributedLock, error) {
	fullKey := s.prefix + "/" + key

	data, err := s.client.Get(ctx, fullKey)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var lock DistributedLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lock: %w", err)
	}

	return &lock, nil
}

// List lists all locks matching a pattern
func (s *ConsulLockStore) List(ctx context.Context, pattern string) ([]*DistributedLock, error) {
	prefix := s.prefix + "/"
	if pattern != "*" && pattern != "" {
		if pattern[len(pattern)-1] == '*' {
			prefix = s.prefix + "/" + pattern[:len(pattern)-1]
		} else {
			prefix = s.prefix + "/" + pattern
		}
	}

	data, err := s.client.GetWithPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var locks []*DistributedLock
	now := time.Now()

	for _, value := range data {
		var lock DistributedLock
		if err := json.Unmarshal(value, &lock); err == nil {
			if now.Before(lock.ExpiresAt) {
				locks = append(locks, &lock)
			}
		}
	}

	return locks, nil
}

// ConsulCoordinator provides high-level coordination using Consul
type ConsulCoordinator struct {
	client        ConsulClient
	electionStore *ConsulElectionStore
	lockStore     *ConsulLockStore
	prefix        string
}

// NewConsulCoordinator creates a new Consul coordinator
func NewConsulCoordinator(client ConsulClient, prefix string) *ConsulCoordinator {
	if prefix == "" {
		prefix = "minion"
	}
	return &ConsulCoordinator{
		client:        client,
		electionStore: NewConsulElectionStore(client, prefix+"/elections"),
		lockStore:     NewConsulLockStore(client, prefix+"/locks"),
		prefix:        prefix,
	}
}

// ElectionStore returns the election store
func (c *ConsulCoordinator) ElectionStore() ElectionStore {
	return c.electionStore
}

// LockStore returns the lock store
func (c *ConsulCoordinator) LockStore() LockStore {
	return c.lockStore
}

// Put stores a key-value pair
func (c *ConsulCoordinator) Put(ctx context.Context, key string, value []byte) error {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Put(ctx, fullKey, value)
}

// Get retrieves a value by key
func (c *ConsulCoordinator) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Get(ctx, fullKey)
}

// Delete removes a key
func (c *ConsulCoordinator) Delete(ctx context.Context, key string) error {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Delete(ctx, fullKey)
}

// ConsulServiceRegistry provides service discovery using Consul
type ConsulServiceRegistry struct {
	client ConsulClient
	mu     sync.RWMutex
	checks map[string]context.CancelFunc
}

// NewConsulServiceRegistry creates a new Consul service registry
func NewConsulServiceRegistry(client ConsulClient) *ConsulServiceRegistry {
	return &ConsulServiceRegistry{
		client: client,
		checks: make(map[string]context.CancelFunc),
	}
}

// Register registers a service with Consul
func (r *ConsulServiceRegistry) Register(ctx context.Context, instance *ServiceInstance, ttl time.Duration) error {
	registration := &ConsulServiceRegistration{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Tags:    instance.Tags,
		Meta:    instance.Metadata,
		Check: &ConsulHealthCheck{
			CheckID:                        "check-" + instance.ID,
			TTL:                            ttl,
			DeregisterCriticalServiceAfter: ttl * 10,
		},
	}

	if err := r.client.RegisterService(ctx, registration); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Start TTL updater
	checkCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.checks[instance.ID] = cancel
	r.mu.Unlock()

	go r.updateTTL(checkCtx, registration.Check.CheckID, ttl)

	return nil
}

// Deregister removes a service from Consul
func (r *ConsulServiceRegistry) Deregister(ctx context.Context, serviceName, instanceID string) error {
	// Stop TTL updater
	r.mu.Lock()
	if cancel, ok := r.checks[instanceID]; ok {
		cancel()
		delete(r.checks, instanceID)
	}
	r.mu.Unlock()

	return r.client.DeregisterService(ctx, instanceID)
}

// Discover finds all instances of a service
func (r *ConsulServiceRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	instances, err := r.client.GetService(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}

	var result []*ServiceInstance
	for _, inst := range instances {
		result = append(result, &ServiceInstance{
			ID:       inst.ID,
			Name:     inst.Name,
			Address:  inst.Address,
			Port:     inst.Port,
			Tags:     inst.Tags,
			Metadata: inst.Meta,
			Health:   HealthStatusHealthy,
		})
	}

	return result, nil
}

func (r *ConsulServiceRegistry) updateTTL(ctx context.Context, checkID string, ttl time.Duration) {
	ticker := time.NewTicker(ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.client.UpdateHealthCheck(ctx, checkID, "passing", "healthy"); err != nil {
				// Log error but continue
			}
		}
	}
}
