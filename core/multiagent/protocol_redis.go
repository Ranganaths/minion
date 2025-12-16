package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisProtocolConfig configures the Redis protocol backend
type RedisProtocolConfig struct {
	// Redis connection
	Addr     string // Redis server address (e.g., "localhost:6379")
	Password string // Redis password (empty if no auth)
	DB       int    // Redis database number

	// Connection pooling
	PoolSize     int           // Maximum number of socket connections
	MinIdleConns int           // Minimum number of idle connections
	MaxRetries   int           // Maximum number of retries
	DialTimeout  time.Duration // Timeout for establishing new connections
	ReadTimeout  time.Duration // Timeout for socket reads
	WriteTimeout time.Duration // Timeout for socket writes

	// Message settings
	MessageTTL      time.Duration // How long messages are retained
	MaxMessageSize  int64         // Maximum message size in bytes
	StreamMaxLen    int64         // Maximum stream length (0 = unlimited)
	ConsumerGroup   string        // Consumer group name
	ConsumerName    string        // Consumer name (unique per instance)
	BlockTimeout    time.Duration // Block timeout for stream reads

	// Performance
	PipelineSize    int           // Number of operations to pipeline
	FlushInterval   time.Duration // How often to flush pipeline
}

// DefaultRedisConfig returns default Redis protocol configuration
func DefaultRedisConfig() *RedisProtocolConfig {
	return &RedisProtocolConfig{
		Addr:            "localhost:6379",
		Password:        "",
		DB:              0,
		PoolSize:        10,
		MinIdleConns:    5,
		MaxRetries:      3,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		MessageTTL:      24 * time.Hour,
		MaxMessageSize:  1024 * 1024, // 1MB
		StreamMaxLen:    10000,
		ConsumerGroup:   "multiagent",
		ConsumerName:    uuid.New().String(),
		BlockTimeout:    5 * time.Second,
		PipelineSize:    100,
		FlushInterval:   100 * time.Millisecond,
	}
}

// RedisProtocol implements Protocol interface using Redis Streams and Pub/Sub
type RedisProtocol struct {
	config       *RedisProtocolConfig
	client       *redis.Client
	pubsub       *redis.PubSub
	subscriptions map[string][]MessageType // agentID -> subscribed message types
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	metrics      *ProtocolMetrics
}

// NewRedisProtocol creates a new Redis-based protocol implementation
func NewRedisProtocol(config *RedisProtocolConfig) (*RedisProtocol, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	rp := &RedisProtocol{
		config:        config,
		client:        client,
		subscriptions: make(map[string][]MessageType),
		ctx:           ctx,
		cancel:        cancel,
		metrics: &ProtocolMetrics{
			LastUpdated: time.Now(),
		},
	}

	return rp, nil
}

// Send sends a message to an agent using Redis Streams
func (rp *RedisProtocol) Send(ctx context.Context, msg *Message) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	start := time.Now()

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		rp.metrics.TotalMessagesFailed++
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Check message size
	if int64(len(data)) > rp.config.MaxMessageSize {
		rp.metrics.TotalMessagesFailed++
		return fmt.Errorf("message size %d exceeds maximum %d", len(data), rp.config.MaxMessageSize)
	}

	// Determine stream key (agent ID)
	streamKey := rp.getStreamKey(msg.To)

	// Add to Redis Stream
	args := &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: rp.config.StreamMaxLen,
		Approx: true, // Use ~ for approximate trimming (faster)
		Values: map[string]interface{}{
			"message": string(data),
			"type":    string(msg.Type),
			"from":    msg.From,
		},
	}

	if _, err := rp.client.XAdd(ctx, args).Result(); err != nil {
		rp.metrics.TotalMessagesFailed++
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	// Set TTL on stream (for cleanup)
	if rp.config.MessageTTL > 0 {
		rp.client.Expire(ctx, streamKey, rp.config.MessageTTL)
	}

	// Update metrics
	rp.metrics.TotalMessagesSent++
	rp.updateLatency(time.Since(start))

	return nil
}

// Receive receives messages for an agent from Redis Streams
func (rp *RedisProtocol) Receive(ctx context.Context, agentID string) ([]*Message, error) {
	rp.mu.RLock()
	subscriptions := rp.subscriptions[agentID]
	rp.mu.RUnlock()

	streamKey := rp.getStreamKey(agentID)
	groupName := rp.config.ConsumerGroup
	consumerName := rp.config.ConsumerName

	// Ensure consumer group exists
	if err := rp.ensureConsumerGroup(ctx, streamKey, groupName); err != nil {
		return nil, err
	}

	// Read from stream using consumer group
	streams, err := rp.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamKey, ">"}, // ">" means read new messages
		Count:    10,                        // Read up to 10 messages
		Block:    rp.config.BlockTimeout,
		NoAck:    false, // We'll ACK manually
	}).Result()

	if err != nil {
		if err == redis.Nil {
			// No messages available
			return []*Message{}, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	var messages []*Message

	for _, stream := range streams {
		for _, xmsg := range stream.Messages {
			// Extract message data
			msgData, ok := xmsg.Values["message"].(string)
			if !ok {
				continue
			}

			// Deserialize message
			var msg Message
			if err := json.Unmarshal([]byte(msgData), &msg); err != nil {
				continue
			}

			// Filter by subscription
			if len(subscriptions) > 0 {
				subscribed := false
				for _, msgType := range subscriptions {
					if msg.Type == msgType {
						subscribed = true
						break
					}
				}
				if !subscribed {
					// ACK but don't return
					rp.client.XAck(ctx, streamKey, groupName, xmsg.ID)
					continue
				}
			}

			messages = append(messages, &msg)

			// ACK the message
			if err := rp.client.XAck(ctx, streamKey, groupName, xmsg.ID).Err(); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	// Update metrics
	rp.mu.Lock()
	rp.metrics.TotalMessagesReceived += int64(len(messages))
	rp.metrics.LastUpdated = time.Now()
	rp.mu.Unlock()

	return messages, nil
}

// Broadcast sends a message to all agents in a group using Redis Pub/Sub
func (rp *RedisProtocol) Broadcast(ctx context.Context, msg *Message, groupID string) error {
	start := time.Now()

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		rp.mu.Lock()
		rp.metrics.TotalMessagesFailed++
		rp.mu.Unlock()
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to group channel
	channel := rp.getBroadcastChannel(groupID)
	if err := rp.client.Publish(ctx, channel, string(data)).Err(); err != nil {
		rp.mu.Lock()
		rp.metrics.TotalMessagesFailed++
		rp.mu.Unlock()
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Update metrics
	rp.mu.Lock()
	rp.metrics.TotalMessagesSent++
	rp.updateLatency(time.Since(start))
	rp.mu.Unlock()

	return nil
}

// Subscribe subscribes an agent to messages of specific types
func (rp *RedisProtocol) Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Store subscription
	rp.subscriptions[agentID] = messageTypes

	// Ensure stream exists
	streamKey := rp.getStreamKey(agentID)
	groupName := rp.config.ConsumerGroup

	return rp.ensureConsumerGroup(ctx, streamKey, groupName)
}

// Unsubscribe removes subscription for an agent
func (rp *RedisProtocol) Unsubscribe(ctx context.Context, agentID string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	delete(rp.subscriptions, agentID)
	return nil
}

// Close closes the Redis connection
func (rp *RedisProtocol) Close() error {
	rp.cancel()

	if rp.pubsub != nil {
		if err := rp.pubsub.Close(); err != nil {
			return err
		}
	}

	return rp.client.Close()
}

// GetMetrics returns protocol metrics
func (rp *RedisProtocol) GetMetrics() *ProtocolMetrics {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	// Return a copy
	return &ProtocolMetrics{
		TotalMessagesSent:     rp.metrics.TotalMessagesSent,
		TotalMessagesReceived: rp.metrics.TotalMessagesReceived,
		TotalMessagesFailed:   rp.metrics.TotalMessagesFailed,
		AverageLatency:        rp.metrics.AverageLatency,
		LastUpdated:           rp.metrics.LastUpdated,
	}
}

// ensureConsumerGroup creates consumer group if it doesn't exist
func (rp *RedisProtocol) ensureConsumerGroup(ctx context.Context, stream, group string) error {
	// Try to create consumer group
	err := rp.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()

	if err != nil {
		// Check if error is "BUSYGROUP" (group already exists)
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	return nil
}

// getStreamKey returns the Redis stream key for an agent
func (rp *RedisProtocol) getStreamKey(agentID string) string {
	return fmt.Sprintf("multiagent:stream:%s", agentID)
}

// getBroadcastChannel returns the Redis channel for group broadcasts
func (rp *RedisProtocol) getBroadcastChannel(groupID string) string {
	return fmt.Sprintf("multiagent:broadcast:%s", groupID)
}

// updateLatency updates average latency metric
func (rp *RedisProtocol) updateLatency(latency time.Duration) {
	// Simple moving average
	if rp.metrics.AverageLatency == 0 {
		rp.metrics.AverageLatency = latency
	} else {
		// Exponential moving average (alpha = 0.2)
		rp.metrics.AverageLatency = time.Duration(
			0.8*float64(rp.metrics.AverageLatency) + 0.2*float64(latency),
		)
	}
}

// CleanupOldMessages removes old messages from streams
func (rp *RedisProtocol) CleanupOldMessages(ctx context.Context, olderThan time.Duration) error {
	// Get all stream keys
	pattern := "multiagent:stream:*"
	iter := rp.client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		streamKey := iter.Val()

		// Get stream info
		info, err := rp.client.XInfoStream(ctx, streamKey).Result()
		if err != nil {
			continue
		}

		// Calculate cutoff time
		cutoffTime := time.Now().Add(-olderThan).UnixMilli()

		// Trim stream
		if info.Length > 0 {
			// Find minimum ID to keep
			minID := fmt.Sprintf("%d-0", cutoffTime)
			_, err := rp.client.XTrimMinID(ctx, streamKey, minID).Result()
			if err != nil {
				continue
			}
		}
	}

	return iter.Err()
}

// Health checks Redis connection health
func (rp *RedisProtocol) Health(ctx context.Context) error {
	return rp.client.Ping(ctx).Err()
}
