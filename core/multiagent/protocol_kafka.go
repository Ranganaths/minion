package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// KafkaProtocolConfig configures the Kafka protocol backend
type KafkaProtocolConfig struct {
	// Kafka connection
	Brokers      []string      // Kafka broker addresses (e.g., ["localhost:9092"])
	GroupID      string        // Consumer group ID
	ClientID     string        // Client identifier
	RequireAcks  int           // 0=NoResponse, 1=LeaderOnly, -1=AllReplicas
	Compression  kafka.Compression // None, Gzip, Snappy, Lz4, Zstd

	// Topic settings
	TopicPrefix       string        // Prefix for all topics
	NumPartitions     int           // Number of partitions per topic
	ReplicationFactor int           // Replication factor for topics

	// Consumer settings
	MinBytes          int           // Minimum bytes to fetch
	MaxBytes          int           // Maximum bytes to fetch
	MaxWait           time.Duration // Maximum time to wait for MinBytes
	ReadBatchTimeout  time.Duration // Timeout for batch reads
	CommitInterval    time.Duration // How often to commit offsets

	// Producer settings
	WriteTimeout      time.Duration // Timeout for writes
	BatchSize         int           // Number of messages to batch
	BatchTimeout      time.Duration // Maximum time to wait for batch
	Async             bool          // Asynchronous writes
	MaxAttempts       int           // Maximum retry attempts

	// Performance
	MessageTTL        time.Duration // How long messages are retained
	MaxMessageSize    int           // Maximum message size in bytes
}

// DefaultKafkaConfig returns default Kafka protocol configuration
func DefaultKafkaConfig() *KafkaProtocolConfig {
	return &KafkaProtocolConfig{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "multiagent-group",
		ClientID:          uuid.New().String(),
		RequireAcks:       1, // Wait for leader acknowledgment
		Compression:       kafka.Snappy,
		TopicPrefix:       "multiagent",
		NumPartitions:     3,
		ReplicationFactor: 1,
		MinBytes:          1,
		MaxBytes:          10e6, // 10MB
		MaxWait:           500 * time.Millisecond,
		ReadBatchTimeout:  10 * time.Second,
		CommitInterval:    1 * time.Second,
		WriteTimeout:      10 * time.Second,
		BatchSize:         100,
		BatchTimeout:      100 * time.Millisecond,
		Async:             false,
		MaxAttempts:       3,
		MessageTTL:        7 * 24 * time.Hour, // 7 days
		MaxMessageSize:    1024 * 1024,        // 1MB
	}
}

// KafkaProtocol implements Protocol interface using Apache Kafka
type KafkaProtocol struct {
	config        *KafkaProtocolConfig
	writers       map[string]*kafka.Writer // topic -> writer
	readers       map[string]*kafka.Reader // topic -> reader
	subscriptions map[string][]MessageType // agentID -> subscribed message types
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	metrics       *ProtocolMetrics
}

// NewKafkaProtocol creates a new Kafka-based protocol implementation
func NewKafkaProtocol(config *KafkaProtocolConfig) (*KafkaProtocol, error) {
	if config == nil {
		config = DefaultKafkaConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	kp := &KafkaProtocol{
		config:        config,
		writers:       make(map[string]*kafka.Writer),
		readers:       make(map[string]*kafka.Reader),
		subscriptions: make(map[string][]MessageType),
		ctx:           ctx,
		cancel:        cancel,
		metrics: &ProtocolMetrics{
			LastUpdated: time.Now(),
		},
	}

	// Test connection by creating a client
	conn, err := kafka.DialContext(ctx, "tcp", config.Brokers[0])
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	return kp, nil
}

// Send sends a message to an agent using Kafka
func (kp *KafkaProtocol) Send(ctx context.Context, msg *Message) error {
	start := time.Now()

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Check message size
	if len(data) > kp.config.MaxMessageSize {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return fmt.Errorf("message size %d exceeds maximum %d", len(data), kp.config.MaxMessageSize)
	}

	// Get or create writer for topic
	topic := kp.getTopic(msg.To)
	writer, err := kp.getWriter(topic)
	if err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return err
	}

	// Create Kafka message
	kafkaMsg := kafka.Message{
		Key:   []byte(msg.ID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "type", Value: []byte(msg.Type)},
			{Key: "from", Value: []byte(msg.From)},
			{Key: "to", Value: []byte(msg.To)},
		},
		Time: msg.CreatedAt,
	}

	// Write message
	if err := writer.WriteMessages(ctx, kafkaMsg); err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return fmt.Errorf("failed to write message to Kafka: %w", err)
	}

	// Update metrics
	kp.mu.Lock()
	kp.metrics.TotalMessagesSent++
	kp.updateLatency(time.Since(start))
	kp.mu.Unlock()

	return nil
}

// Receive receives messages for an agent from Kafka
func (kp *KafkaProtocol) Receive(ctx context.Context, agentID string) ([]*Message, error) {
	kp.mu.RLock()
	subscriptions := kp.subscriptions[agentID]
	kp.mu.RUnlock()

	topic := kp.getTopic(agentID)
	reader, err := kp.getReader(topic)
	if err != nil {
		return nil, err
	}

	// Set read deadline
	ctx, cancel := context.WithTimeout(ctx, kp.config.ReadBatchTimeout)
	defer cancel()

	var messages []*Message

	// Read up to 10 messages
	for i := 0; i < 10; i++ {
		kafkaMsg, err := reader.ReadMessage(ctx)
		if err != nil {
			if err == context.DeadlineExceeded {
				// Timeout - return what we have
				break
			}
			// Other error
			return nil, fmt.Errorf("failed to read message: %w", err)
		}

		// Deserialize message
		var msg Message
		if err := json.Unmarshal(kafkaMsg.Value, &msg); err != nil {
			// Skip malformed message
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
				continue
			}
		}

		messages = append(messages, &msg)
	}

	// Update metrics
	kp.mu.Lock()
	kp.metrics.TotalMessagesReceived += int64(len(messages))
	kp.metrics.LastUpdated = time.Now()
	kp.mu.Unlock()

	return messages, nil
}

// Broadcast sends a message to all agents in a group
func (kp *KafkaProtocol) Broadcast(ctx context.Context, msg *Message, groupID string) error {
	start := time.Now()

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.CreatedAt = time.Now()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Use broadcast topic
	topic := kp.getBroadcastTopic(groupID)
	writer, err := kp.getWriter(topic)
	if err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return err
	}

	// Create Kafka message
	kafkaMsg := kafka.Message{
		Key:   []byte(msg.ID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "type", Value: []byte(msg.Type)},
			{Key: "from", Value: []byte(msg.From)},
			{Key: "group", Value: []byte(groupID)},
		},
		Time: msg.CreatedAt,
	}

	// Write message
	if err := writer.WriteMessages(ctx, kafkaMsg); err != nil {
		kp.mu.Lock()
		kp.metrics.TotalMessagesFailed++
		kp.mu.Unlock()
		return fmt.Errorf("failed to write broadcast message: %w", err)
	}

	// Update metrics
	kp.mu.Lock()
	kp.metrics.TotalMessagesSent++
	kp.updateLatency(time.Since(start))
	kp.mu.Unlock()

	return nil
}

// Subscribe subscribes an agent to messages of specific types
func (kp *KafkaProtocol) Subscribe(ctx context.Context, agentID string, messageTypes []MessageType) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	// Store subscription
	kp.subscriptions[agentID] = messageTypes

	// Ensure topic exists
	topic := kp.getTopic(agentID)
	return kp.ensureTopic(ctx, topic)
}

// Unsubscribe removes subscription for an agent
func (kp *KafkaProtocol) Unsubscribe(ctx context.Context, agentID string) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	delete(kp.subscriptions, agentID)

	// Close reader if exists
	topic := kp.getTopic(agentID)
	if reader, ok := kp.readers[topic]; ok {
		reader.Close()
		delete(kp.readers, topic)
	}

	return nil
}

// Close closes all Kafka connections
func (kp *KafkaProtocol) Close() error {
	kp.cancel()

	// Close all writers
	for _, writer := range kp.writers {
		if err := writer.Close(); err != nil {
			// Log error but continue
		}
	}

	// Close all readers
	for _, reader := range kp.readers {
		if err := reader.Close(); err != nil {
			// Log error but continue
		}
	}

	return nil
}

// GetMetrics returns protocol metrics
func (kp *KafkaProtocol) GetMetrics() *ProtocolMetrics {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	// Return a copy
	return &ProtocolMetrics{
		TotalMessagesSent:     kp.metrics.TotalMessagesSent,
		TotalMessagesReceived: kp.metrics.TotalMessagesReceived,
		TotalMessagesFailed:   kp.metrics.TotalMessagesFailed,
		AverageLatency:        kp.metrics.AverageLatency,
		LastUpdated:           kp.metrics.LastUpdated,
	}
}

// getWriter returns or creates a Kafka writer for a topic
func (kp *KafkaProtocol) getWriter(topic string) (*kafka.Writer, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if writer, ok := kp.writers[topic]; ok {
		return writer, nil
	}

	// Create new writer
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kp.config.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{}, // Use least bytes balancer
		RequiredAcks: kafka.RequiredAcks(kp.config.RequireAcks),
		Compression:  kp.config.Compression,
		WriteTimeout: kp.config.WriteTimeout,
		BatchSize:    kp.config.BatchSize,
		BatchTimeout: kp.config.BatchTimeout,
		Async:        kp.config.Async,
		MaxAttempts:  kp.config.MaxAttempts,
	}

	kp.writers[topic] = writer
	return writer, nil
}

// getReader returns or creates a Kafka reader for a topic
func (kp *KafkaProtocol) getReader(topic string) (*kafka.Reader, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if reader, ok := kp.readers[topic]; ok {
		return reader, nil
	}

	// Create new reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        kp.config.Brokers,
		Topic:          topic,
		GroupID:        kp.config.GroupID,
		MinBytes:       kp.config.MinBytes,
		MaxBytes:       kp.config.MaxBytes,
		MaxWait:        kp.config.MaxWait,
		CommitInterval: kp.config.CommitInterval,
		StartOffset:    kafka.LastOffset, // Start from latest
	})

	kp.readers[topic] = reader
	return reader, nil
}

// ensureTopic creates a topic if it doesn't exist
func (kp *KafkaProtocol) ensureTopic(ctx context.Context, topic string) error {
	conn, err := kafka.DialContext(ctx, "tcp", kp.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer conn.Close()

	// Create topic
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     kp.config.NumPartitions,
			ReplicationFactor: kp.config.ReplicationFactor,
		},
	}

	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		// Ignore error if topic already exists
		// Kafka returns error even if topic exists
		return nil
	}

	return nil
}

// getTopic returns the Kafka topic for an agent
func (kp *KafkaProtocol) getTopic(agentID string) string {
	return fmt.Sprintf("%s-%s", kp.config.TopicPrefix, agentID)
}

// getBroadcastTopic returns the Kafka topic for group broadcasts
func (kp *KafkaProtocol) getBroadcastTopic(groupID string) string {
	return fmt.Sprintf("%s-broadcast-%s", kp.config.TopicPrefix, groupID)
}

// updateLatency updates average latency metric
func (kp *KafkaProtocol) updateLatency(latency time.Duration) {
	// Simple moving average
	if kp.metrics.AverageLatency == 0 {
		kp.metrics.AverageLatency = latency
	} else {
		// Exponential moving average (alpha = 0.2)
		kp.metrics.AverageLatency = time.Duration(
			0.8*float64(kp.metrics.AverageLatency) + 0.2*float64(latency),
		)
	}
}

// Health checks Kafka connection health
func (kp *KafkaProtocol) Health(ctx context.Context) error {
	conn, err := kafka.DialContext(ctx, "tcp", kp.config.Brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}
