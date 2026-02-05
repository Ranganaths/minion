package coordination

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// EtcdConfig configures the etcd connection
type EtcdConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
	Username    string
	Password    string
	TLSCert     string
	TLSKey      string
	TLSCA       string
}

// EtcdClient interface abstracts the etcd client for testing
type EtcdClient interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) ([]byte, error)

	// GetWithPrefix retrieves all values with a key prefix
	GetWithPrefix(ctx context.Context, prefix string) (map[string][]byte, error)

	// Put stores a key-value pair
	Put(ctx context.Context, key string, value []byte) error

	// PutWithLease stores a key-value pair with a lease
	PutWithLease(ctx context.Context, key string, value []byte, ttl time.Duration) (int64, error)

	// Delete removes a key
	Delete(ctx context.Context, key string) error

	// DeleteWithPrefix removes all keys with prefix
	DeleteWithPrefix(ctx context.Context, prefix string) error

	// Watch watches for changes on a key or prefix
	Watch(ctx context.Context, key string, prefix bool) (<-chan *WatchEvent, error)

	// KeepAliveLease keeps a lease alive
	KeepAliveLease(ctx context.Context, leaseID int64) (<-chan struct{}, error)

	// RevokeLease revokes a lease
	RevokeLease(ctx context.Context, leaseID int64) error

	// Campaign starts a leader election campaign
	Campaign(ctx context.Context, prefix, value string) (<-chan struct{}, error)

	// Resign resigns from leadership
	Resign(ctx context.Context, prefix string) error

	// Close closes the client connection
	Close() error
}

// WatchEvent represents a watch event
type WatchEvent struct {
	Type  WatchEventType
	Key   string
	Value []byte
}

// WatchEventType represents the type of watch event
type WatchEventType string

const (
	WatchEventPut    WatchEventType = "PUT"
	WatchEventDelete WatchEventType = "DELETE"
)

// EtcdElectionStore implements ElectionStore using etcd
type EtcdElectionStore struct {
	client EtcdClient
	prefix string
	mu     sync.RWMutex
	leases map[string]int64
}

// NewEtcdElectionStore creates a new etcd-backed election store
func NewEtcdElectionStore(client EtcdClient, prefix string) *EtcdElectionStore {
	if prefix == "" {
		prefix = "/minion/elections"
	}
	return &EtcdElectionStore{
		client: client,
		prefix: prefix,
		leases: make(map[string]int64),
	}
}

// TryAcquire attempts to acquire leadership
func (s *EtcdElectionStore) TryAcquire(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	fullKey := s.prefix + "/" + key

	// Check if there's an existing leader
	existing, err := s.client.Get(ctx, fullKey)
	if err == nil && len(existing) > 0 {
		var info LeaderInfo
		if err := json.Unmarshal(existing, &info); err == nil {
			// If the leader is still valid and not us, fail
			if info.NodeID != nodeID && time.Now().Before(info.ExpiresAt) {
				return false, nil
			}
		}
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
		return false, fmt.Errorf("failed to marshal leader info: %w", err)
	}

	// Put with lease
	leaseID, err := s.client.PutWithLease(ctx, fullKey, data, leaseDuration)
	if err != nil {
		return false, fmt.Errorf("failed to acquire leadership: %w", err)
	}

	// Store lease ID for renewal
	s.mu.Lock()
	s.leases[key] = leaseID
	s.mu.Unlock()

	// Start keep-alive
	go s.keepAlive(ctx, key, leaseID)

	return true, nil
}

// Renew renews the leadership lease
func (s *EtcdElectionStore) Renew(ctx context.Context, key, nodeID string, term int64, leaseDuration time.Duration) (bool, error) {
	fullKey := s.prefix + "/" + key

	// Get current leader
	existing, err := s.client.Get(ctx, fullKey)
	if err != nil || len(existing) == 0 {
		return false, errors.New("no leader to renew")
	}

	var info LeaderInfo
	if err := json.Unmarshal(existing, &info); err != nil {
		return false, fmt.Errorf("failed to unmarshal leader info: %w", err)
	}

	// Verify we are the leader
	if info.NodeID != nodeID || info.Term != term {
		return false, nil
	}

	// Update expiry
	info.ExpiresAt = time.Now().Add(leaseDuration)

	data, err := json.Marshal(info)
	if err != nil {
		return false, err
	}

	// Re-put with new lease
	s.mu.RLock()
	oldLeaseID := s.leases[key]
	s.mu.RUnlock()

	if oldLeaseID != 0 {
		// Revoke old lease
		s.client.RevokeLease(ctx, oldLeaseID)
	}

	leaseID, err := s.client.PutWithLease(ctx, fullKey, data, leaseDuration)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	s.leases[key] = leaseID
	s.mu.Unlock()

	return true, nil
}

// GetLeader returns the current leader
func (s *EtcdElectionStore) GetLeader(ctx context.Context, key string) (*LeaderInfo, error) {
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

	// Check if expired
	if time.Now().After(info.ExpiresAt) {
		return nil, nil
	}

	return &info, nil
}

// Release releases leadership
func (s *EtcdElectionStore) Release(ctx context.Context, key, nodeID string) error {
	fullKey := s.prefix + "/" + key

	// Verify we are the leader
	leader, err := s.GetLeader(ctx, key)
	if err != nil {
		return err
	}

	if leader == nil || leader.NodeID != nodeID {
		return errors.New("not the current leader")
	}

	// Revoke lease
	s.mu.Lock()
	leaseID := s.leases[key]
	delete(s.leases, key)
	s.mu.Unlock()

	if leaseID != 0 {
		s.client.RevokeLease(ctx, leaseID)
	}

	return s.client.Delete(ctx, fullKey)
}

// Watch watches for leadership changes
func (s *EtcdElectionStore) Watch(ctx context.Context, key string) (<-chan *LeaderInfo, error) {
	fullKey := s.prefix + "/" + key

	watchCh, err := s.client.Watch(ctx, fullKey, false)
	if err != nil {
		return nil, err
	}

	infoCh := make(chan *LeaderInfo, 10)

	go func() {
		defer close(infoCh)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watchCh:
				if !ok {
					return
				}

				if event.Type == WatchEventDelete {
					select {
					case infoCh <- nil:
					default:
					}
					continue
				}

				var info LeaderInfo
				if err := json.Unmarshal(event.Value, &info); err == nil {
					select {
					case infoCh <- &info:
					default:
					}
				}
			}
		}
	}()

	return infoCh, nil
}

func (s *EtcdElectionStore) keepAlive(ctx context.Context, key string, leaseID int64) {
	doneCh, err := s.client.KeepAliveLease(ctx, leaseID)
	if err != nil {
		return
	}

	<-doneCh
}

// EtcdLockStore implements LockStore using etcd
type EtcdLockStore struct {
	client EtcdClient
	prefix string
	mu     sync.RWMutex
	leases map[string]int64
}

// NewEtcdLockStore creates a new etcd-backed lock store
func NewEtcdLockStore(client EtcdClient, prefix string) *EtcdLockStore {
	if prefix == "" {
		prefix = "/minion/locks"
	}
	return &EtcdLockStore{
		client: client,
		prefix: prefix,
		leases: make(map[string]int64),
	}
}

// TryAcquire attempts to acquire a lock
func (s *EtcdLockStore) TryAcquire(ctx context.Context, config LockConfig) (*DistributedLock, error) {
	fullKey := s.prefix + "/" + config.Key

	// Check if lock exists and is valid
	existing, err := s.client.Get(ctx, fullKey)
	if err == nil && len(existing) > 0 {
		var lock DistributedLock
		if err := json.Unmarshal(existing, &lock); err == nil {
			if lock.State == LockStateLocked && time.Now().Before(lock.ExpiresAt) {
				// Lock is held - allow shared locks if both are shared
				if config.Type == LockTypeShared && lock.Type == LockTypeShared {
					// Allow acquiring shared lock
				} else {
					return nil, errors.New("lock already held")
				}
			}
		}
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
		return nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Put with lease
	leaseID, err := s.client.PutWithLease(ctx, fullKey, data, config.TTL)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	s.mu.Lock()
	s.leases[config.Key] = leaseID
	s.mu.Unlock()

	return lock, nil
}

// Release releases a lock
func (s *EtcdLockStore) Release(ctx context.Context, key, owner string) error {
	fullKey := s.prefix + "/" + key

	// Verify ownership
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

	// Revoke lease
	s.mu.Lock()
	leaseID := s.leases[key]
	delete(s.leases, key)
	s.mu.Unlock()

	if leaseID != 0 {
		s.client.RevokeLease(ctx, leaseID)
	}

	return s.client.Delete(ctx, fullKey)
}

// Renew renews a lock's TTL
func (s *EtcdLockStore) Renew(ctx context.Context, key, owner string, ttl time.Duration) error {
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

	// Update expiry and re-put
	lock.ExpiresAt = time.Now().Add(ttl)

	data, err := json.Marshal(lock)
	if err != nil {
		return err
	}

	fullKey := s.prefix + "/" + key

	// Revoke old lease
	s.mu.Lock()
	oldLeaseID := s.leases[key]
	s.mu.Unlock()

	if oldLeaseID != 0 {
		s.client.RevokeLease(ctx, oldLeaseID)
	}

	leaseID, err := s.client.PutWithLease(ctx, fullKey, data, ttl)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.leases[key] = leaseID
	s.mu.Unlock()

	return nil
}

// Get gets a lock's current state
func (s *EtcdLockStore) Get(ctx context.Context, key string) (*DistributedLock, error) {
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

	// Check if expired
	if time.Now().After(lock.ExpiresAt) {
		return nil, nil
	}

	return &lock, nil
}

// List lists all locks matching a pattern
func (s *EtcdLockStore) List(ctx context.Context, pattern string) ([]*DistributedLock, error) {
	prefix := s.prefix + "/"
	if pattern != "*" && pattern != "" {
		// Convert glob pattern to prefix
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

// EtcdCoordinator provides high-level coordination using etcd
type EtcdCoordinator struct {
	client        EtcdClient
	electionStore *EtcdElectionStore
	lockStore     *EtcdLockStore
	prefix        string
}

// NewEtcdCoordinator creates a new etcd coordinator
func NewEtcdCoordinator(client EtcdClient, prefix string) *EtcdCoordinator {
	if prefix == "" {
		prefix = "/minion"
	}
	return &EtcdCoordinator{
		client:        client,
		electionStore: NewEtcdElectionStore(client, prefix+"/elections"),
		lockStore:     NewEtcdLockStore(client, prefix+"/locks"),
		prefix:        prefix,
	}
}

// ElectionStore returns the election store
func (c *EtcdCoordinator) ElectionStore() ElectionStore {
	return c.electionStore
}

// LockStore returns the lock store
func (c *EtcdCoordinator) LockStore() LockStore {
	return c.lockStore
}

// Put stores a key-value pair
func (c *EtcdCoordinator) Put(ctx context.Context, key string, value []byte) error {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Put(ctx, fullKey, value)
}

// Get retrieves a value by key
func (c *EtcdCoordinator) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Get(ctx, fullKey)
}

// Delete removes a key
func (c *EtcdCoordinator) Delete(ctx context.Context, key string) error {
	fullKey := c.prefix + "/kv/" + key
	return c.client.Delete(ctx, fullKey)
}

// Watch watches for changes on a key prefix
func (c *EtcdCoordinator) Watch(ctx context.Context, prefix string) (<-chan *WatchEvent, error) {
	fullPrefix := c.prefix + "/kv/" + prefix
	return c.client.Watch(ctx, fullPrefix, true)
}

// Close closes the coordinator
func (c *EtcdCoordinator) Close() error {
	return c.client.Close()
}

// ServiceRegistry provides service discovery using etcd
type ServiceRegistry struct {
	client EtcdClient
	prefix string
	mu     sync.RWMutex
	leases map[string]int64
}

// ServiceInstance represents a registered service instance
type ServiceInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Health   HealthStatus      `json:"health"`
}

// HealthStatus represents service health
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(client EtcdClient, prefix string) *ServiceRegistry {
	if prefix == "" {
		prefix = "/minion/services"
	}
	return &ServiceRegistry{
		client: client,
		prefix: prefix,
		leases: make(map[string]int64),
	}
}

// Register registers a service instance
func (r *ServiceRegistry) Register(ctx context.Context, instance *ServiceInstance, ttl time.Duration) error {
	key := r.prefix + "/" + instance.Name + "/" + instance.ID

	data, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal instance: %w", err)
	}

	leaseID, err := r.client.PutWithLease(ctx, key, data, ttl)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	r.mu.Lock()
	r.leases[instance.ID] = leaseID
	r.mu.Unlock()

	// Start keep-alive
	go r.keepAlive(ctx, instance.ID, leaseID)

	return nil
}

// Deregister removes a service instance
func (r *ServiceRegistry) Deregister(ctx context.Context, serviceName, instanceID string) error {
	key := r.prefix + "/" + serviceName + "/" + instanceID

	// Revoke lease
	r.mu.Lock()
	leaseID := r.leases[instanceID]
	delete(r.leases, instanceID)
	r.mu.Unlock()

	if leaseID != 0 {
		r.client.RevokeLease(ctx, leaseID)
	}

	return r.client.Delete(ctx, key)
}

// Discover finds all instances of a service
func (r *ServiceRegistry) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	prefix := r.prefix + "/" + serviceName + "/"

	data, err := r.client.GetWithPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	var instances []*ServiceInstance
	for _, value := range data {
		var instance ServiceInstance
		if err := json.Unmarshal(value, &instance); err == nil {
			instances = append(instances, &instance)
		}
	}

	return instances, nil
}

// DiscoverAll finds all registered services
func (r *ServiceRegistry) DiscoverAll(ctx context.Context) (map[string][]*ServiceInstance, error) {
	data, err := r.client.GetWithPrefix(ctx, r.prefix+"/")
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}

	services := make(map[string][]*ServiceInstance)
	for _, value := range data {
		var instance ServiceInstance
		if err := json.Unmarshal(value, &instance); err == nil {
			services[instance.Name] = append(services[instance.Name], &instance)
		}
	}

	return services, nil
}

// Watch watches for service changes
func (r *ServiceRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	prefix := r.prefix + "/" + serviceName + "/"

	watchCh, err := r.client.Watch(ctx, prefix, true)
	if err != nil {
		return nil, err
	}

	instancesCh := make(chan []*ServiceInstance, 10)

	go func() {
		defer close(instancesCh)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-watchCh:
				if !ok {
					return
				}

				// On any change, fetch current state
				instances, err := r.Discover(ctx, serviceName)
				if err == nil {
					select {
					case instancesCh <- instances:
					default:
					}
				}
			}
		}
	}()

	return instancesCh, nil
}

func (r *ServiceRegistry) keepAlive(ctx context.Context, instanceID string, leaseID int64) {
	doneCh, err := r.client.KeepAliveLease(ctx, leaseID)
	if err != nil {
		return
	}

	<-doneCh
}
