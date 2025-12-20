package multiagent

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ProtocolType defines the type of protocol backend
type ProtocolType string

const (
	ProtocolTypeInMemory ProtocolType = "inmemory"
	ProtocolTypeRedis    ProtocolType = "redis"
	ProtocolTypeKafka    ProtocolType = "kafka"
	ProtocolTypeHybrid   ProtocolType = "hybrid"
)

// ProtocolFactoryConfig contains configuration for protocol creation
type ProtocolFactoryConfig struct {
	Type ProtocolType

	// In-memory config
	InMemoryConfig *InMemoryProtocolConfig

	// Redis config
	RedisConfig *RedisProtocolConfig

	// Kafka config
	KafkaConfig *KafkaProtocolConfig
}

// ProtocolFactory creates protocol instances based on configuration
type ProtocolFactory struct {
	config *ProtocolFactoryConfig
}

// NewProtocolFactory creates a new protocol factory
func NewProtocolFactory(config *ProtocolFactoryConfig) *ProtocolFactory {
	if config == nil {
		config = DefaultProtocolFactoryConfig()
	}

	return &ProtocolFactory{
		config: config,
	}
}

// NewProtocolFactoryFromEnv creates a protocol factory from environment variables
func NewProtocolFactoryFromEnv() *ProtocolFactory {
	config := &ProtocolFactoryConfig{
		Type: ProtocolType(getEnvOrDefault("PROTOCOL_TYPE", string(ProtocolTypeInMemory))),
	}

	// Configure based on type
	switch config.Type {
	case ProtocolTypeRedis:
		config.RedisConfig = RedisConfigFromEnv()
	case ProtocolTypeKafka:
		config.KafkaConfig = KafkaConfigFromEnv()
	case ProtocolTypeInMemory:
		config.InMemoryConfig = DefaultInMemoryProtocolConfig()
	}

	return NewProtocolFactory(config)
}

// DefaultProtocolFactoryConfig returns default protocol factory configuration
func DefaultProtocolFactoryConfig() *ProtocolFactoryConfig {
	return &ProtocolFactoryConfig{
		Type:           ProtocolTypeInMemory,
		InMemoryConfig: DefaultInMemoryProtocolConfig(),
	}
}

// CreateProtocol creates a protocol instance based on configuration
func (pf *ProtocolFactory) CreateProtocol() (Protocol, error) {
	switch pf.config.Type {
	case ProtocolTypeInMemory:
		return pf.createInMemoryProtocol()

	case ProtocolTypeRedis:
		return pf.createRedisProtocol()

	case ProtocolTypeKafka:
		return pf.createKafkaProtocol()

	case ProtocolTypeHybrid:
		return nil, fmt.Errorf("hybrid protocol not yet implemented")

	default:
		return nil, fmt.Errorf("unknown protocol type: %s", pf.config.Type)
	}
}

// createInMemoryProtocol creates an in-memory protocol
func (pf *ProtocolFactory) createInMemoryProtocol() (Protocol, error) {
	config := pf.config.InMemoryConfig
	if config == nil {
		config = DefaultInMemoryProtocolConfig()
	}

	return NewInMemoryProtocol(config), nil
}

// createRedisProtocol creates a Redis protocol
func (pf *ProtocolFactory) createRedisProtocol() (Protocol, error) {
	config := pf.config.RedisConfig
	if config == nil {
		config = DefaultRedisConfig()
	}

	protocol, err := NewRedisProtocol(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis protocol: %w", err)
	}

	return protocol, nil
}

// createKafkaProtocol creates a Kafka protocol
func (pf *ProtocolFactory) createKafkaProtocol() (Protocol, error) {
	config := pf.config.KafkaConfig
	if config == nil {
		config = DefaultKafkaConfig()
	}

	protocol, err := NewKafkaProtocol(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka protocol: %w", err)
	}

	return protocol, nil
}

// Validate validates the protocol factory configuration
func (pf *ProtocolFactory) Validate() error {
	switch pf.config.Type {
	case ProtocolTypeInMemory:
		// Always valid
		return nil

	case ProtocolTypeRedis:
		if pf.config.RedisConfig == nil {
			return fmt.Errorf("Redis configuration required")
		}
		if pf.config.RedisConfig.Addr == "" {
			return fmt.Errorf("Redis address required")
		}
		return nil

	case ProtocolTypeKafka:
		if pf.config.KafkaConfig == nil {
			return fmt.Errorf("Kafka configuration required")
		}
		if len(pf.config.KafkaConfig.Brokers) == 0 {
			return fmt.Errorf("Kafka brokers required")
		}
		return nil

	case ProtocolTypeHybrid:
		return fmt.Errorf("hybrid protocol not yet implemented")

	default:
		return fmt.Errorf("unknown protocol type: %s", pf.config.Type)
	}
}

// RedisConfigFromEnv creates Redis configuration from environment variables
func RedisConfigFromEnv() *RedisProtocolConfig {
	config := DefaultRedisConfig()

	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		config.Addr = addr
	}

	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Password = password
	}

	if db := os.Getenv("REDIS_DB"); db != "" {
		if dbNum, err := strconv.Atoi(db); err == nil {
			config.DB = dbNum
		}
	}

	if poolSize := os.Getenv("REDIS_POOL_SIZE"); poolSize != "" {
		if size, err := strconv.Atoi(poolSize); err == nil {
			config.PoolSize = size
		}
	}

	if ttl := os.Getenv("REDIS_MESSAGE_TTL"); ttl != "" {
		if duration, err := time.ParseDuration(ttl); err == nil {
			config.MessageTTL = duration
		}
	}

	if group := os.Getenv("REDIS_CONSUMER_GROUP"); group != "" {
		config.ConsumerGroup = group
	}

	return config
}

// KafkaConfigFromEnv creates Kafka configuration from environment variables
func KafkaConfigFromEnv() *KafkaProtocolConfig {
	config := DefaultKafkaConfig()

	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		config.Brokers = strings.Split(brokers, ",")
	}

	if groupID := os.Getenv("KAFKA_GROUP_ID"); groupID != "" {
		config.GroupID = groupID
	}

	if clientID := os.Getenv("KAFKA_CLIENT_ID"); clientID != "" {
		config.ClientID = clientID
	}

	if topicPrefix := os.Getenv("KAFKA_TOPIC_PREFIX"); topicPrefix != "" {
		config.TopicPrefix = topicPrefix
	}

	if partitions := os.Getenv("KAFKA_NUM_PARTITIONS"); partitions != "" {
		if num, err := strconv.Atoi(partitions); err == nil {
			config.NumPartitions = num
		}
	}

	if replication := os.Getenv("KAFKA_REPLICATION_FACTOR"); replication != "" {
		if factor, err := strconv.Atoi(replication); err == nil {
			config.ReplicationFactor = factor
		}
	}

	if ttl := os.Getenv("KAFKA_MESSAGE_TTL"); ttl != "" {
		if duration, err := time.ParseDuration(ttl); err == nil {
			config.MessageTTL = duration
		}
	}

	return config
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// InMemoryProtocolConfig configures in-memory protocol (from protocol_impl.go)
type InMemoryProtocolConfig struct {
	MaxQueueSize int
}

// DefaultInMemoryProtocolConfig returns default in-memory protocol configuration
func DefaultInMemoryProtocolConfig() *InMemoryProtocolConfig {
	return &InMemoryProtocolConfig{
		MaxQueueSize: 1000,
	}
}
