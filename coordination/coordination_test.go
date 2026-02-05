package coordination

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Leader Election Tests

func TestNewLeaderElection(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 10 * time.Second,
	}

	election := NewLeaderElection(store, config)

	if election.ID == "" {
		t.Error("Election should have an ID")
	}

	if election.NodeID != "node-1" {
		t.Errorf("Expected node ID 'node-1', got '%s'", election.NodeID)
	}

	if election.State != ElectionStateFollower {
		t.Errorf("Expected initial state follower, got '%s'", election.State)
	}
}

func TestNewLeaderElectionDefaults(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey: "test-election",
	}

	election := NewLeaderElection(store, config)

	if election.NodeID == "" {
		t.Error("Election should have auto-generated node ID")
	}

	if election.LeaseDuration != 15*time.Second {
		t.Errorf("Expected default lease duration 15s, got %v", election.LeaseDuration)
	}
}

func TestLeaderElectionStart(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
	}

	election := NewLeaderElection(store, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := election.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start election: %v", err)
	}

	// Wait for election to happen
	time.Sleep(500 * time.Millisecond)

	// Should become leader since it's the only node
	if !election.IsLeader() {
		t.Error("Expected to become leader")
	}
}

func TestLeaderElectionStop(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
	}

	election := NewLeaderElection(store, config)
	ctx := context.Background()

	election.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	err := election.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop election: %v", err)
	}

	if election.IsLeader() {
		t.Error("Should not be leader after stop")
	}
}

func TestLeaderElectionCallbacks(t *testing.T) {
	store := NewInMemoryElectionStore()

	var leaderCalled, followerCalled atomic.Bool

	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
		OnLeader: func(ctx context.Context) {
			leaderCalled.Store(true)
		},
		OnFollower: func(ctx context.Context) {
			followerCalled.Store(true)
		},
	}

	election := NewLeaderElection(store, config)
	ctx, cancel := context.WithCancel(context.Background())

	election.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	if !leaderCalled.Load() {
		t.Error("OnLeader callback should have been called")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestLeaderElectionGetters(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
	}

	election := NewLeaderElection(store, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	election.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	state := election.GetState()
	if state != ElectionStateLeader {
		t.Errorf("Expected state leader, got %s", state)
	}

	leader := election.GetLeader()
	if leader != "node-1" {
		t.Errorf("Expected leader 'node-1', got '%s'", leader)
	}

	term := election.GetTerm()
	if term < 1 {
		t.Errorf("Expected term >= 1, got %d", term)
	}
}

func TestLeaderElectionResign(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
	}

	election := NewLeaderElection(store, config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	election.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	if !election.IsLeader() {
		t.Fatal("Should be leader before resign")
	}

	err := election.Resign(ctx)
	if err != nil {
		t.Fatalf("Failed to resign: %v", err)
	}

	if election.IsLeader() {
		t.Error("Should not be leader after resign")
	}
}

func TestLeaderElectionResignNotLeader(t *testing.T) {
	store := NewInMemoryElectionStore()
	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 1 * time.Second,
	}

	election := NewLeaderElection(store, config)

	err := election.Resign(context.Background())
	if err == nil {
		t.Error("Expected error when resigning as non-leader")
	}
}

func TestInMemoryElectionStore(t *testing.T) {
	store := NewInMemoryElectionStore()
	ctx := context.Background()

	// Try to acquire leadership
	acquired, err := store.TryAcquire(ctx, "test-key", "node-1", 1, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	if !acquired {
		t.Error("Should have acquired leadership")
	}

	// Get leader
	info, err := store.GetLeader(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to get leader: %v", err)
	}
	if info.NodeID != "node-1" {
		t.Errorf("Expected leader 'node-1', got '%s'", info.NodeID)
	}

	// Try to acquire with different node (should fail)
	acquired, err = store.TryAcquire(ctx, "test-key", "node-2", 1, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to try acquire: %v", err)
	}
	if acquired {
		t.Error("Should not have acquired leadership")
	}

	// Renew lease
	renewed, err := store.Renew(ctx, "test-key", "node-1", 1, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to renew: %v", err)
	}
	if !renewed {
		t.Error("Should have renewed lease")
	}

	// Release
	err = store.Release(ctx, "test-key", "node-1")
	if err != nil {
		t.Fatalf("Failed to release: %v", err)
	}

	// Now node-2 can acquire
	acquired, err = store.TryAcquire(ctx, "test-key", "node-2", 2, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	if !acquired {
		t.Error("Should have acquired leadership after release")
	}
}

func TestInMemoryElectionStoreWatch(t *testing.T) {
	store := NewInMemoryElectionStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := store.Watch(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to watch: %v", err)
	}

	// Acquire leadership
	store.TryAcquire(ctx, "test-key", "node-1", 1, 10*time.Second)

	// Should receive notification
	select {
	case info := <-ch:
		if info.NodeID != "node-1" {
			t.Errorf("Expected leader 'node-1', got '%s'", info.NodeID)
		}
	case <-time.After(time.Second):
		t.Error("Did not receive watch notification")
	}
}

func TestElectionManager(t *testing.T) {
	store := NewInMemoryElectionStore()
	manager := NewElectionManager(store)

	config := ElectionConfig{
		ElectionKey:   "test-election",
		NodeID:        "node-1",
		LeaseDuration: 10 * time.Second,
	}

	election, err := manager.CreateElection(config)
	if err != nil {
		t.Fatalf("Failed to create election: %v", err)
	}

	if election.ElectionKey != "test-election" {
		t.Errorf("Expected key 'test-election', got '%s'", election.ElectionKey)
	}

	// Get election
	retrieved, ok := manager.GetElection("test-election")
	if !ok {
		t.Error("Should find election")
	}
	if retrieved.ID != election.ID {
		t.Error("Retrieved election should match")
	}

	// Stop all
	err = manager.StopAll(context.Background())
	if err != nil {
		t.Fatalf("Failed to stop all: %v", err)
	}
}

// Distributed Lock Tests

func TestLockManagerAcquire(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	config := LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	}

	lock, err := manager.Acquire(ctx, config)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	if lock.Key != "test-lock" {
		t.Errorf("Expected key 'test-lock', got '%s'", lock.Key)
	}

	if lock.State != LockStateLocked {
		t.Errorf("Expected state locked, got %s", lock.State)
	}
}

func TestLockManagerAcquireNoKey(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)

	_, err := manager.Acquire(context.Background(), LockConfig{})
	if err == nil {
		t.Error("Expected error for missing key")
	}
}

func TestLockManagerAcquireConflict(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	// First acquisition
	manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	// Second acquisition should fail immediately
	_, err := manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-2",
		TTL:   10 * time.Second,
	})

	if err == nil {
		t.Error("Expected error for conflicting lock")
	}
}

func TestLockManagerAcquireWithRetry(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	// First acquisition
	lock1, _ := manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   200 * time.Millisecond,
	})

	// Release after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		manager.Release(ctx, lock1.Key, lock1.Owner)
	}()

	// Second acquisition with retry
	lock2, err := manager.Acquire(ctx, LockConfig{
		Key:           "test-lock",
		Owner:         "owner-2",
		TTL:           10 * time.Second,
		WaitTimeout:   time.Second,
		RetryInterval: 50 * time.Millisecond,
	})

	if err != nil {
		t.Fatalf("Failed to acquire lock with retry: %v", err)
	}

	if lock2.Owner != "owner-2" {
		t.Errorf("Expected owner 'owner-2', got '%s'", lock2.Owner)
	}
}

func TestLockManagerAcquireTimeout(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	// First acquisition (long TTL)
	manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	// Second acquisition should timeout
	_, err := manager.Acquire(ctx, LockConfig{
		Key:           "test-lock",
		Owner:         "owner-2",
		TTL:           10 * time.Second,
		WaitTimeout:   200 * time.Millisecond,
		RetryInterval: 50 * time.Millisecond,
	})

	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestLockManagerRelease(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	lock, _ := manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	err := manager.Release(ctx, lock.Key, lock.Owner)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Should be able to acquire again
	_, err = manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-2",
		TTL:   10 * time.Second,
	})

	if err != nil {
		t.Fatalf("Failed to acquire after release: %v", err)
	}
}

func TestLockManagerRenew(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	lock, _ := manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   1 * time.Second,
	})

	err := manager.Renew(ctx, lock.Key, lock.Owner, 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to renew lock: %v", err)
	}

	// Lock should still be held after original TTL
	time.Sleep(1100 * time.Millisecond)

	isLocked, _ := manager.IsLocked(ctx, "test-lock")
	if !isLocked {
		t.Error("Lock should still be held after renewal")
	}
}

func TestLockManagerIsLocked(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	isLocked, _ := manager.IsLocked(ctx, "test-lock")
	if isLocked {
		t.Error("Should not be locked initially")
	}

	manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	isLocked, _ = manager.IsLocked(ctx, "test-lock")
	if !isLocked {
		t.Error("Should be locked after acquire")
	}
}

func TestLockManagerGet(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	lock, err := manager.Get(ctx, "test-lock")
	if err != nil {
		t.Fatalf("Failed to get lock: %v", err)
	}

	if lock.Owner != "owner-1" {
		t.Errorf("Expected owner 'owner-1', got '%s'", lock.Owner)
	}
}

func TestLockManagerOnLockChange(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	var notified atomic.Bool

	manager.OnLockChange("test-lock", func(lock *DistributedLock) {
		notified.Store(true)
	})

	manager.Acquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	})

	time.Sleep(100 * time.Millisecond)

	if !notified.Load() {
		t.Error("Callback should have been notified")
	}
}

func TestLockManagerWithLock(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	var executed bool

	err := manager.WithLock(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   10 * time.Second,
	}, func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("WithLock failed: %v", err)
	}

	if !executed {
		t.Error("Function should have been executed")
	}

	// Lock should be released
	isLocked, _ := manager.IsLocked(ctx, "test-lock")
	if isLocked {
		t.Error("Lock should be released after WithLock")
	}
}

func TestInMemoryLockStoreSharedLocks(t *testing.T) {
	store := NewInMemoryLockStore()
	ctx := context.Background()

	// First shared lock
	lock1, err := store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		Type:  LockTypeShared,
		TTL:   10 * time.Second,
	})

	if err != nil {
		t.Fatalf("Failed to acquire first shared lock: %v", err)
	}

	// Second shared lock should succeed
	lock2, err := store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-2",
		Type:  LockTypeShared,
		TTL:   10 * time.Second,
	})

	if err != nil {
		t.Fatalf("Failed to acquire second shared lock: %v", err)
	}

	if lock1.Owner == lock2.Owner {
		t.Error("Locks should have different owners")
	}
}

func TestInMemoryLockStoreExclusiveBlocksShared(t *testing.T) {
	store := NewInMemoryLockStore()
	ctx := context.Background()

	// Exclusive lock
	store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		Type:  LockTypeExclusive,
		TTL:   10 * time.Second,
	})

	// Shared lock should fail
	_, err := store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-2",
		Type:  LockTypeShared,
		TTL:   10 * time.Second,
	})

	if err == nil {
		t.Error("Shared lock should fail when exclusive lock held")
	}
}

func TestInMemoryLockStoreList(t *testing.T) {
	store := NewInMemoryLockStore()
	ctx := context.Background()

	store.TryAcquire(ctx, LockConfig{Key: "lock-1", Owner: "o1", TTL: 10 * time.Second})
	store.TryAcquire(ctx, LockConfig{Key: "lock-2", Owner: "o2", TTL: 10 * time.Second})
	store.TryAcquire(ctx, LockConfig{Key: "other-1", Owner: "o3", TTL: 10 * time.Second})

	// List all
	locks, _ := store.List(ctx, "*")
	if len(locks) != 3 {
		t.Errorf("Expected 3 locks, got %d", len(locks))
	}

	// List with prefix
	locks, _ = store.List(ctx, "lock-*")
	if len(locks) != 2 {
		t.Errorf("Expected 2 locks with prefix, got %d", len(locks))
	}
}

func TestInMemoryLockStoreExpiration(t *testing.T) {
	store := NewInMemoryLockStore()
	ctx := context.Background()

	store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-1",
		TTL:   100 * time.Millisecond,
	})

	time.Sleep(150 * time.Millisecond)

	// Lock should be expired
	lock, _ := store.Get(ctx, "test-lock")
	if lock != nil {
		t.Error("Lock should be expired")
	}

	// Should be able to acquire
	newLock, err := store.TryAcquire(ctx, LockConfig{
		Key:   "test-lock",
		Owner: "owner-2",
		TTL:   10 * time.Second,
	})

	if err != nil {
		t.Fatalf("Failed to acquire after expiration: %v", err)
	}

	if newLock.Owner != "owner-2" {
		t.Errorf("Expected owner 'owner-2', got '%s'", newLock.Owner)
	}
}

// Semaphore Tests

func TestSemaphoreAcquireRelease(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	sem := NewSemaphore(manager, "test-sem", 3)
	ctx := context.Background()

	// Acquire 3 slots
	holders := make([]string, 3)
	for i := 0; i < 3; i++ {
		holder, err := sem.Acquire(ctx, 10*time.Second)
		if err != nil {
			t.Fatalf("Failed to acquire slot %d: %v", i, err)
		}
		holders[i] = holder
	}

	if sem.Available() != 0 {
		t.Errorf("Expected 0 available, got %d", sem.Available())
	}

	// Fourth should fail
	_, err := sem.Acquire(ctx, 10*time.Second)
	if err == nil {
		t.Error("Should fail when at capacity")
	}

	// Release one
	sem.Release(ctx, holders[0])

	if sem.Available() != 1 {
		t.Errorf("Expected 1 available, got %d", sem.Available())
	}

	// Now can acquire again
	_, err = sem.Acquire(ctx, 10*time.Second)
	if err != nil {
		t.Fatalf("Should be able to acquire after release: %v", err)
	}
}

func TestSemaphoreReleaseInvalid(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	sem := NewSemaphore(manager, "test-sem", 3)

	err := sem.Release(context.Background(), "invalid-holder")
	if err == nil {
		t.Error("Should fail to release invalid holder")
	}
}

// Barrier Tests

func TestBarrierWait(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	barrier := NewBarrier(manager, "test-barrier", 3)
	ctx := context.Background()

	var wg sync.WaitGroup
	arrived := make(chan string, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			participantID := string(rune('A' + id))
			barrier.Wait(ctx, participantID)
			arrived <- participantID
		}(i)
	}

	wg.Wait()

	// All should have arrived
	if len(arrived) != 3 {
		t.Errorf("Expected 3 arrivals, got %d", len(arrived))
	}
}

func TestBarrierWaitTimeout(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	barrier := NewBarrier(manager, "test-barrier", 3)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Only one participant
	err := barrier.Wait(ctx, "A")
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestBarrierReset(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	barrier := NewBarrier(manager, "test-barrier", 3)

	// Add one waiter
	go barrier.Wait(context.Background(), "A")
	time.Sleep(50 * time.Millisecond)

	if barrier.Arrived() != 1 {
		t.Errorf("Expected 1 arrived, got %d", barrier.Arrived())
	}

	barrier.Reset()

	if barrier.Arrived() != 0 {
		t.Errorf("Expected 0 arrived after reset, got %d", barrier.Arrived())
	}
}

func TestBarrierCount(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	barrier := NewBarrier(manager, "test-barrier", 5)

	if barrier.Count() != 5 {
		t.Errorf("Expected count 5, got %d", barrier.Count())
	}
}

func TestBarrierDoubleWait(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	barrier := NewBarrier(manager, "test-barrier", 3)
	ctx := context.Background()

	// First wait
	go barrier.Wait(ctx, "A")
	time.Sleep(50 * time.Millisecond)

	// Second wait with same ID should fail
	err := barrier.Wait(ctx, "A")
	if err == nil {
		t.Error("Expected error for duplicate participant")
	}
}

// Concurrent Access Tests

func TestConcurrentLockAcquisition(t *testing.T) {
	store := NewInMemoryLockStore()
	manager := NewLockManager(store)
	ctx := context.Background()

	var wg sync.WaitGroup
	var acquired atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lock, err := manager.Acquire(ctx, LockConfig{
				Key:   "concurrent-lock",
				Owner: string(rune('A' + id)),
				TTL:   time.Second,
			})
			if err == nil && lock != nil {
				acquired.Add(1)
				time.Sleep(50 * time.Millisecond)
				manager.Release(ctx, lock.Key, lock.Owner)
			}
		}(i)
	}

	wg.Wait()

	// Only one should have acquired initially
	if acquired.Load() < 1 {
		t.Error("At least one should have acquired")
	}
}

func TestConcurrentElection(t *testing.T) {
	store := NewInMemoryElectionStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var leaderCount atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			config := ElectionConfig{
				ElectionKey:   "concurrent-election",
				NodeID:        string(rune('A' + id)),
				LeaseDuration: time.Second,
				OnLeader: func(ctx context.Context) {
					leaderCount.Add(1)
				},
			}

			election := NewLeaderElection(store, config)
			election.Start(ctx)

			time.Sleep(500 * time.Millisecond)
			election.Stop(ctx)
		}(i)
	}

	wg.Wait()

	// At least one should have become leader
	if leaderCount.Load() < 1 {
		t.Error("At least one should have become leader")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		key      string
		pattern  string
		expected bool
	}{
		{"lock-1", "*", true},
		{"lock-1", "lock-*", true},
		{"lock-1", "other-*", false},
		{"lock-1", "lock-1", true},
		{"lock-1", "lock-2", false},
		{"", "*", true},
	}

	for _, tt := range tests {
		result := matchPattern(tt.key, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.key, tt.pattern, result, tt.expected)
		}
	}
}
