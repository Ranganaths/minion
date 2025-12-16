package multiagent

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/redis/go-redis/v9"
)

// DedupBackend defines interface for deduplication storage
type DedupBackend interface {
	// IsDuplicate checks if message ID was already processed
	IsDuplicate(ctx context.Context, messageID string) (bool, error)

	// MarkProcessed marks message as processed with TTL
	MarkProcessed(ctx context.Context, messageID string, ttl time.Duration) error

	// Cleanup removes expired entries
	Cleanup(ctx context.Context, olderThan time.Time) error

	// Stats returns deduplication statistics
	Stats(ctx context.Context) (*DedupStats, error)

	// Close closes the backend
	Close() error
}

// DedupStats contains deduplication statistics
type DedupStats struct {
	TotalChecks     int64
	Duplicates      int64
	UniqueMessages  int64
	FalsePositives  int64
	StorageSize     int64
}

// DeduplicationService provides message deduplication
type DeduplicationService struct {
	backend      DedupBackend
	bloomFilter  *bloom.BloomFilter
	windowSize   time.Duration
	mu           sync.RWMutex
	stats        *DedupStats
}

// DeduplicationConfig configures deduplication service
type DeduplicationConfig struct {
	Backend            string        // "inmemory", "redis", "postgres"
	WindowSize         time.Duration // How long to remember message IDs
	BloomFilterSize    uint          // Bloom filter size
	BloomFilterHashes  uint          // Number of hash functions
	CleanupInterval    time.Duration // How often to run cleanup
}

// DefaultDeduplicationConfig returns default configuration
func DefaultDeduplicationConfig() *DeduplicationConfig {
	return &DeduplicationConfig{
		Backend:           "inmemory",
		WindowSize:        1 * time.Hour,
		BloomFilterSize:   100000,
		BloomFilterHashes: 5,
		CleanupInterval:   10 * time.Minute,
	}
}

// NewDeduplicationService creates a new deduplication service
func NewDeduplicationService(config *DeduplicationConfig, backend DedupBackend) *DeduplicationService {
	if config == nil {
		config = DefaultDeduplicationConfig()
	}

	return &DeduplicationService{
		backend:     backend,
		bloomFilter: bloom.NewWithEstimates(config.BloomFilterSize, 0.01), // 1% false positive rate
		windowSize:  config.WindowSize,
		stats:       &DedupStats{},
	}
}

// CheckAndMark checks if message is duplicate and marks it as processed
func (ds *DeduplicationService) CheckAndMark(ctx context.Context, messageID string) (bool, error) {
	ds.mu.Lock()
	ds.stats.TotalChecks++
	ds.mu.Unlock()

	// Fast check with Bloom filter first
	ds.mu.RLock()
	inBloom := ds.bloomFilter.TestString(messageID)
	ds.mu.RUnlock()

	if inBloom {
		// Might be duplicate - check backend
		isDup, err := ds.backend.IsDuplicate(ctx, messageID)
		if err != nil {
			return false, fmt.Errorf("failed to check duplicate: %w", err)
		}

		if isDup {
			ds.mu.Lock()
			ds.stats.Duplicates++
			ds.mu.Unlock()
			return true, nil
		}

		// False positive from bloom filter
		ds.mu.Lock()
		ds.stats.FalsePositives++
		ds.mu.Unlock()
	}

	// Not a duplicate - mark as processed
	if err := ds.backend.MarkProcessed(ctx, messageID, ds.windowSize); err != nil {
		return false, fmt.Errorf("failed to mark processed: %w", err)
	}

	// Add to bloom filter
	ds.mu.Lock()
	ds.bloomFilter.AddString(messageID)
	ds.stats.UniqueMessages++
	ds.mu.Unlock()

	return false, nil
}

// GetStats returns deduplication statistics
func (ds *DeduplicationService) GetStats() *DedupStats {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return &DedupStats{
		TotalChecks:    ds.stats.TotalChecks,
		Duplicates:     ds.stats.Duplicates,
		UniqueMessages: ds.stats.UniqueMessages,
		FalsePositives: ds.stats.FalsePositives,
		StorageSize:    ds.stats.StorageSize,
	}
}

// Close closes the deduplication service
func (ds *DeduplicationService) Close() error {
	return ds.backend.Close()
}

// ===== In-Memory Backend =====

type InMemoryDedupBackend struct {
	mu       sync.RWMutex
	messages map[string]time.Time // messageID -> expiry time
}

func NewInMemoryDedupBackend() *InMemoryDedupBackend {
	return &InMemoryDedupBackend{
		messages: make(map[string]time.Time),
	}
}

func (im *InMemoryDedupBackend) IsDuplicate(ctx context.Context, messageID string) (bool, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	expiry, exists := im.messages[messageID]
	if !exists {
		return false, nil
	}

	// Check if expired
	if time.Now().After(expiry) {
		return false, nil
	}

	return true, nil
}

func (im *InMemoryDedupBackend) MarkProcessed(ctx context.Context, messageID string, ttl time.Duration) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.messages[messageID] = time.Now().Add(ttl)
	return nil
}

func (im *InMemoryDedupBackend) Cleanup(ctx context.Context, olderThan time.Time) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	now := time.Now()
	for id, expiry := range im.messages {
		if now.After(expiry) {
			delete(im.messages, id)
		}
	}

	return nil
}

func (im *InMemoryDedupBackend) Stats(ctx context.Context) (*DedupStats, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	return &DedupStats{
		UniqueMessages: int64(len(im.messages)),
	}, nil
}

func (im *InMemoryDedupBackend) Close() error {
	return nil
}

// ===== Redis Backend =====

type RedisDedupBackend struct {
	client *redis.Client
}

func NewRedisDedupBackend(client *redis.Client) *RedisDedupBackend {
	return &RedisDedupBackend{
		client: client,
	}
}

func (rd *RedisDedupBackend) IsDuplicate(ctx context.Context, messageID string) (bool, error) {
	exists, err := rd.client.Exists(ctx, rd.key(messageID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (rd *RedisDedupBackend) MarkProcessed(ctx context.Context, messageID string, ttl time.Duration) error {
	return rd.client.Set(ctx, rd.key(messageID), "1", ttl).Err()
}

func (rd *RedisDedupBackend) Cleanup(ctx context.Context, olderThan time.Time) error {
	// Redis handles TTL automatically
	return nil
}

func (rd *RedisDedupBackend) Stats(ctx context.Context) (*DedupStats, error) {
	// Get count of dedup keys
	iter := rd.client.Scan(ctx, 0, "dedup:*", 0).Iterator()
	count := int64(0)

	for iter.Next(ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return &DedupStats{
		UniqueMessages: count,
	}, nil
}

func (rd *RedisDedupBackend) Close() error {
	return rd.client.Close()
}

func (rd *RedisDedupBackend) key(messageID string) string {
	return fmt.Sprintf("dedup:%s", messageID)
}

// ===== PostgreSQL Backend =====

type PostgresDedupBackend struct {
	db    *sql.DB
	table string
}

func NewPostgresDedupBackend(db *sql.DB, table string) *PostgresDedupBackend {
	if table == "" {
		table = "message_dedup"
	}

	return &PostgresDedupBackend{
		db:    db,
		table: table,
	}
}

func (pg *PostgresDedupBackend) IsDuplicate(ctx context.Context, messageID string) (bool, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE message_id = $1 AND expires_at > NOW()
	`, pg.table)

	var count int
	err := pg.db.QueryRowContext(ctx, query, messageID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (pg *PostgresDedupBackend) MarkProcessed(ctx context.Context, messageID string, ttl time.Duration) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (message_id, processed_at, expires_at)
		VALUES ($1, NOW(), $2)
		ON CONFLICT (message_id) DO NOTHING
	`, pg.table)

	expiresAt := time.Now().Add(ttl)
	_, err := pg.db.ExecContext(ctx, query, messageID, expiresAt)
	return err
}

func (pg *PostgresDedupBackend) Cleanup(ctx context.Context, olderThan time.Time) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE expires_at < $1
	`, pg.table)

	_, err := pg.db.ExecContext(ctx, query, olderThan)
	return err
}

func (pg *PostgresDedupBackend) Stats(ctx context.Context) (*DedupStats, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			pg_total_relation_size('%s') as size
		FROM %s
		WHERE expires_at > NOW()
	`, pg.table, pg.table)

	stats := &DedupStats{}
	err := pg.db.QueryRowContext(ctx, query).Scan(&stats.UniqueMessages, &stats.StorageSize)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (pg *PostgresDedupBackend) Close() error {
	return pg.db.Close()
}

// ===== Deduplication Factory =====

type DeduplicationFactory struct {
	config *DeduplicationConfig
}

func NewDeduplicationFactory(config *DeduplicationConfig) *DeduplicationFactory {
	if config == nil {
		config = DefaultDeduplicationConfig()
	}
	return &DeduplicationFactory{config: config}
}

func (df *DeduplicationFactory) CreateService(
	redisClient *redis.Client,
	postgresDB *sql.DB,
) (*DeduplicationService, error) {
	var backend DedupBackend

	switch df.config.Backend {
	case "inmemory":
		backend = NewInMemoryDedupBackend()

	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("Redis client required for redis backend")
		}
		backend = NewRedisDedupBackend(redisClient)

	case "postgres":
		if postgresDB == nil {
			return nil, fmt.Errorf("PostgreSQL DB required for postgres backend")
		}
		backend = NewPostgresDedupBackend(postgresDB, "message_dedup")

	default:
		return nil, fmt.Errorf("unknown deduplication backend: %s", df.config.Backend)
	}

	service := NewDeduplicationService(df.config, backend)

	// Start cleanup goroutine
	go service.cleanupLoop(df.config.CleanupInterval)

	return service, nil
}

// cleanupLoop periodically cleans up expired entries
func (ds *DeduplicationService) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		cutoff := time.Now().Add(-ds.windowSize)

		if err := ds.backend.Cleanup(ctx, cutoff); err != nil {
			// Log error but continue
			continue
		}
	}
}
