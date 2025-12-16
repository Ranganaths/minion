package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App           AppConfig
	Database      DatabaseConfig
	LLM           LLMConfig
	Observability ObservabilityConfig
	Operations    OperationsConfig
	Session       SessionConfig
	Evaluation    EvaluationConfig
	Security      SecurityConfig
	Features      FeaturesConfig
	Registry      RegistryConfig
	API           APIConfig
	Health        HealthConfig
}

// AppConfig contains application-level configuration
type AppConfig struct {
	Name     string `mapstructure:"name"`
	Env      string `mapstructure:"env"`
	Port     int    `mapstructure:"port"`
	LogLevel string `mapstructure:"log_level"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	Name               string        `mapstructure:"name"`
	User               string        `mapstructure:"user"`
	Password           string        `mapstructure:"password"`
	SSLMode            string        `mapstructure:"sslmode"`
	MaxConnections     int           `mapstructure:"max_connections"`
	MaxIdleConnections int           `mapstructure:"max_idle_connections"`
	ConnMaxLifetime    time.Duration `mapstructure:"connection_max_lifetime"`
}

// LLMConfig contains LLM provider configuration
type LLMConfig struct {
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
	Gemini    GeminiConfig    `mapstructure:"gemini"`
	Default   DefaultLLMConfig `mapstructure:"default"`
}

type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
	OrgID  string `mapstructure:"org_id"`
}

type AnthropicConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type DefaultLLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	Model       string  `mapstructure:"model"`
	Temperature float32 `mapstructure:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens"`
}

// ObservabilityConfig contains observability configuration
type ObservabilityConfig struct {
	Tracing TracingConfig `mapstructure:"tracing"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Logging LoggingConfig `mapstructure:"logging"`
	Cost    CostConfig    `mapstructure:"cost"`
}

type TracingConfig struct {
	Enabled       bool    `mapstructure:"enabled"`
	ServiceName   string  `mapstructure:"service_name"`
	Exporter      string  `mapstructure:"exporter"`
	JaegerURL     string  `mapstructure:"jaeger_endpoint"`
	OTLPEndpoint  string  `mapstructure:"otlp_endpoint"`
	SamplingRatio float64 `mapstructure:"sampling_ratio"`
}

type MetricsConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Port               int    `mapstructure:"port"`
	PrometheusEnabled  bool   `mapstructure:"prometheus_enabled"`
	Path               string `mapstructure:"path"`
}

type LoggingConfig struct {
	Format      string `mapstructure:"format"`
	Output      string `mapstructure:"output"`
	FilePath    string `mapstructure:"file_path"`
	MaxSizeMB   int    `mapstructure:"max_size_mb"`
	MaxBackups  int    `mapstructure:"max_backups"`
	MaxAgeDays  int    `mapstructure:"max_age_days"`
}

type CostConfig struct {
	Enabled               bool    `mapstructure:"enabled"`
	PricingFile           string  `mapstructure:"pricing_file"`
	BudgetAlertThreshold  float64 `mapstructure:"budget_alert_threshold"`
	Currency              string  `mapstructure:"currency"`
}

// OperationsConfig contains operational controls configuration
type OperationsConfig struct {
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
	Retry          RetryConfig          `mapstructure:"retry"`
}

type CircuitBreakerConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold int           `mapstructure:"threshold"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

type RateLimitConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst            int     `mapstructure:"burst"`
}

type RetryConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	MaxAttempts     int           `mapstructure:"max_attempts"`
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval"`
	Multiplier      float64       `mapstructure:"multiplier"`
}

// SessionConfig contains session and memory configuration
type SessionConfig struct {
	Timeout                 time.Duration `mapstructure:"timeout"`
	MaxHistory              int           `mapstructure:"max_history"`
	MemoryExtractionEnabled bool          `mapstructure:"memory_extraction_enabled"`
	VectorDimensions        int           `mapstructure:"vector_dimensions"`
	SimilarityThreshold     float64       `mapstructure:"similarity_threshold"`
}

// EvaluationConfig contains evaluation system configuration
type EvaluationConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	LLMModel       string `mapstructure:"llm_model"`
	GoldenSetPath  string `mapstructure:"golden_set_path"`
	ParallelWorkers int   `mapstructure:"parallel_workers"`
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	HITLEnabled              bool `mapstructure:"hitl_enabled"`
	PIIDetectionEnabled      bool `mapstructure:"pii_detection_enabled"`
	InputValidationEnabled   bool `mapstructure:"input_validation_enabled"`
	MaxInputLength           int  `mapstructure:"max_input_length"`
}

// FeaturesConfig contains feature flags
type FeaturesConfig struct {
	MCPEnabled        bool `mapstructure:"mcp_enabled"`
	A2AEnabled        bool `mapstructure:"a2a_enabled"`
	StreamingEnabled  bool `mapstructure:"streaming_enabled"`
	MultiAgentEnabled bool `mapstructure:"multi_agent_enabled"`
}

// RegistryConfig contains registry configuration
type RegistryConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Type               string `mapstructure:"type"`
	GovernanceEnabled  bool   `mapstructure:"governance_enabled"`
}

// APIConfig contains API configuration
type APIConfig struct {
	Timeout        time.Duration `mapstructure:"timeout"`
	MaxRequestSize string        `mapstructure:"max_request_size"`
	CORSEnabled    bool          `mapstructure:"cors_enabled"`
	CORSOrigins    string        `mapstructure:"cors_origins"`
}

// HealthConfig contains health check configuration
type HealthConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Interval        time.Duration `mapstructure:"interval"`
	CheckDB         bool          `mapstructure:"check_db"`
	CheckLLM        bool          `mapstructure:"check_llm"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	// Set up viper
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// Set defaults
	setDefaults(v)

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Override with environment variables
	v.SetEnvPrefix("") // No prefix, use full env var names
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map environment variables to config structure
	bindEnvVars(v)

	// Unmarshal into config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("app.name", "minion")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.log_level", "info")

	// Database
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "minion")
	v.SetDefault("database.user", "minion")
	v.SetDefault("database.password", "minion")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.max_idle_connections", 5)
	v.SetDefault("database.connection_max_lifetime", "300s")

	// LLM Defaults
	v.SetDefault("llm.default.provider", "openai")
	v.SetDefault("llm.default.model", "gpt-4-turbo-preview")
	v.SetDefault("llm.default.temperature", 0.7)
	v.SetDefault("llm.default.max_tokens", 2000)

	// Observability
	v.SetDefault("observability.tracing.enabled", true)
	v.SetDefault("observability.tracing.service_name", "minion-agent")
	v.SetDefault("observability.tracing.exporter", "jaeger")
	v.SetDefault("observability.tracing.jaeger_endpoint", "http://localhost:14268/api/traces")
	v.SetDefault("observability.tracing.sampling_ratio", 1.0)

	v.SetDefault("observability.metrics.enabled", true)
	v.SetDefault("observability.metrics.port", 9090)
	v.SetDefault("observability.metrics.prometheus_enabled", true)
	v.SetDefault("observability.metrics.path", "/metrics")

	v.SetDefault("observability.logging.format", "json")
	v.SetDefault("observability.logging.output", "stdout")
	v.SetDefault("observability.logging.max_size_mb", 100)
	v.SetDefault("observability.logging.max_backups", 3)
	v.SetDefault("observability.logging.max_age_days", 28)

	v.SetDefault("observability.cost.enabled", true)
	v.SetDefault("observability.cost.pricing_file", "config/model_pricing.json")
	v.SetDefault("observability.cost.budget_alert_threshold", 100.0)
	v.SetDefault("observability.cost.currency", "USD")

	// Operations
	v.SetDefault("operations.circuit_breaker.enabled", true)
	v.SetDefault("operations.circuit_breaker.threshold", 5)
	v.SetDefault("operations.circuit_breaker.timeout", "60s")

	v.SetDefault("operations.rate_limit.enabled", true)
	v.SetDefault("operations.rate_limit.requests_per_second", 10.0)
	v.SetDefault("operations.rate_limit.burst", 20)

	v.SetDefault("operations.retry.enabled", true)
	v.SetDefault("operations.retry.max_attempts", 3)
	v.SetDefault("operations.retry.initial_interval", "1s")
	v.SetDefault("operations.retry.max_interval", "30s")
	v.SetDefault("operations.retry.multiplier", 2.0)

	// Session & Memory
	v.SetDefault("session.timeout", "30m")
	v.SetDefault("session.max_history", 100)
	v.SetDefault("session.memory_extraction_enabled", true)
	v.SetDefault("session.vector_dimensions", 1536)
	v.SetDefault("session.similarity_threshold", 0.7)

	// Evaluation
	v.SetDefault("evaluation.enabled", true)
	v.SetDefault("evaluation.llm_model", "gpt-4-turbo-preview")
	v.SetDefault("evaluation.golden_set_path", "testdata/golden_set.json")
	v.SetDefault("evaluation.parallel_workers", 4)

	// Security
	v.SetDefault("security.hitl_enabled", true)
	v.SetDefault("security.pii_detection_enabled", true)
	v.SetDefault("security.input_validation_enabled", true)
	v.SetDefault("security.max_input_length", 10000)

	// Features
	v.SetDefault("features.mcp_enabled", false)
	v.SetDefault("features.a2a_enabled", false)
	v.SetDefault("features.streaming_enabled", false)
	v.SetDefault("features.multi_agent_enabled", false)

	// Registry
	v.SetDefault("registry.enabled", false)
	v.SetDefault("registry.type", "local")
	v.SetDefault("registry.governance_enabled", false)

	// API
	v.SetDefault("api.timeout", "120s")
	v.SetDefault("api.max_request_size", "10MB")
	v.SetDefault("api.cors_enabled", true)
	v.SetDefault("api.cors_origins", "*")

	// Health
	v.SetDefault("health.enabled", true)
	v.SetDefault("health.interval", "30s")
	v.SetDefault("health.check_db", true)
	v.SetDefault("health.check_llm", true)
}

func bindEnvVars(v *viper.Viper) {
	// App
	_ = v.BindEnv("app.name", "APP_NAME")
	_ = v.BindEnv("app.env", "APP_ENV")
	_ = v.BindEnv("app.port", "APP_PORT")
	_ = v.BindEnv("app.log_level", "APP_LOG_LEVEL")

	// Database
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.name", "DB_NAME")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.sslmode", "DB_SSLMODE")
	_ = v.BindEnv("database.max_connections", "DB_MAX_CONNECTIONS")
	_ = v.BindEnv("database.max_idle_connections", "DB_MAX_IDLE_CONNECTIONS")
	_ = v.BindEnv("database.connection_max_lifetime", "DB_CONNECTION_MAX_LIFETIME")

	// LLM
	_ = v.BindEnv("llm.openai.api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("llm.openai.org_id", "OPENAI_ORG_ID")
	_ = v.BindEnv("llm.anthropic.api_key", "ANTHROPIC_API_KEY")
	_ = v.BindEnv("llm.gemini.api_key", "GEMINI_API_KEY")
	_ = v.BindEnv("llm.default.provider", "DEFAULT_LLM_PROVIDER")
	_ = v.BindEnv("llm.default.model", "DEFAULT_LLM_MODEL")
	_ = v.BindEnv("llm.default.temperature", "DEFAULT_LLM_TEMPERATURE")
	_ = v.BindEnv("llm.default.max_tokens", "DEFAULT_LLM_MAX_TOKENS")

	// Observability - Tracing
	_ = v.BindEnv("observability.tracing.enabled", "OTEL_ENABLED")
	_ = v.BindEnv("observability.tracing.service_name", "OTEL_SERVICE_NAME")
	_ = v.BindEnv("observability.tracing.exporter", "OTEL_EXPORTER")
	_ = v.BindEnv("observability.tracing.jaeger_endpoint", "JAEGER_ENDPOINT")
	_ = v.BindEnv("observability.tracing.otlp_endpoint", "OTLP_ENDPOINT")
	_ = v.BindEnv("observability.tracing.sampling_ratio", "OTEL_SAMPLING_RATIO")

	// Observability - Metrics
	_ = v.BindEnv("observability.metrics.enabled", "METRICS_ENABLED")
	_ = v.BindEnv("observability.metrics.port", "METRICS_PORT")
	_ = v.BindEnv("observability.metrics.prometheus_enabled", "PROMETHEUS_ENABLED")
	_ = v.BindEnv("observability.metrics.path", "METRICS_PATH")

	// Observability - Logging
	_ = v.BindEnv("observability.logging.format", "LOG_FORMAT")
	_ = v.BindEnv("observability.logging.output", "LOG_OUTPUT")
	_ = v.BindEnv("observability.logging.file_path", "LOG_FILE_PATH")
	_ = v.BindEnv("observability.logging.max_size_mb", "LOG_MAX_SIZE_MB")
	_ = v.BindEnv("observability.logging.max_backups", "LOG_MAX_BACKUPS")
	_ = v.BindEnv("observability.logging.max_age_days", "LOG_MAX_AGE_DAYS")

	// Observability - Cost
	_ = v.BindEnv("observability.cost.enabled", "COST_TRACKING_ENABLED")
	_ = v.BindEnv("observability.cost.pricing_file", "COST_MODEL_PRICING_FILE")
	_ = v.BindEnv("observability.cost.budget_alert_threshold", "COST_BUDGET_ALERT_THRESHOLD")
	_ = v.BindEnv("observability.cost.currency", "COST_CURRENCY")

	// Operations
	_ = v.BindEnv("operations.circuit_breaker.enabled", "CIRCUIT_BREAKER_ENABLED")
	_ = v.BindEnv("operations.circuit_breaker.threshold", "CIRCUIT_BREAKER_THRESHOLD")
	_ = v.BindEnv("operations.circuit_breaker.timeout", "CIRCUIT_BREAKER_TIMEOUT")

	_ = v.BindEnv("operations.rate_limit.enabled", "RATE_LIMIT_ENABLED")
	_ = v.BindEnv("operations.rate_limit.requests_per_second", "RATE_LIMIT_REQUESTS_PER_SECOND")
	_ = v.BindEnv("operations.rate_limit.burst", "RATE_LIMIT_BURST")

	_ = v.BindEnv("operations.retry.enabled", "RETRY_ENABLED")
	_ = v.BindEnv("operations.retry.max_attempts", "RETRY_MAX_ATTEMPTS")
	_ = v.BindEnv("operations.retry.initial_interval", "RETRY_INITIAL_INTERVAL")
	_ = v.BindEnv("operations.retry.max_interval", "RETRY_MAX_INTERVAL")
	_ = v.BindEnv("operations.retry.multiplier", "RETRY_MULTIPLIER")

	// Session & Memory
	_ = v.BindEnv("session.timeout", "SESSION_TIMEOUT")
	_ = v.BindEnv("session.max_history", "SESSION_MAX_HISTORY")
	_ = v.BindEnv("session.memory_extraction_enabled", "MEMORY_EXTRACTION_ENABLED")
	_ = v.BindEnv("session.vector_dimensions", "MEMORY_VECTOR_DIMENSIONS")
	_ = v.BindEnv("session.similarity_threshold", "MEMORY_SIMILARITY_THRESHOLD")

	// Evaluation
	_ = v.BindEnv("evaluation.enabled", "EVALUATION_ENABLED")
	_ = v.BindEnv("evaluation.llm_model", "EVALUATION_LLM_MODEL")
	_ = v.BindEnv("evaluation.golden_set_path", "EVALUATION_GOLDEN_SET_PATH")
	_ = v.BindEnv("evaluation.parallel_workers", "EVALUATION_PARALLEL_WORKERS")

	// Security
	_ = v.BindEnv("security.hitl_enabled", "SECURITY_HITL_ENABLED")
	_ = v.BindEnv("security.pii_detection_enabled", "SECURITY_PII_DETECTION_ENABLED")
	_ = v.BindEnv("security.input_validation_enabled", "SECURITY_INPUT_VALIDATION_ENABLED")
	_ = v.BindEnv("security.max_input_length", "SECURITY_MAX_INPUT_LENGTH")

	// Features
	_ = v.BindEnv("features.mcp_enabled", "FEATURE_MCP_ENABLED")
	_ = v.BindEnv("features.a2a_enabled", "FEATURE_A2A_ENABLED")
	_ = v.BindEnv("features.streaming_enabled", "FEATURE_STREAMING_ENABLED")
	_ = v.BindEnv("features.multi_agent_enabled", "FEATURE_MULTI_AGENT_ENABLED")

	// Registry
	_ = v.BindEnv("registry.enabled", "REGISTRY_ENABLED")
	_ = v.BindEnv("registry.type", "REGISTRY_TYPE")
	_ = v.BindEnv("registry.governance_enabled", "REGISTRY_GOVERNANCE_ENABLED")

	// API
	_ = v.BindEnv("api.timeout", "API_TIMEOUT")
	_ = v.BindEnv("api.max_request_size", "API_MAX_REQUEST_SIZE")
	_ = v.BindEnv("api.cors_enabled", "API_CORS_ENABLED")
	_ = v.BindEnv("api.cors_origins", "API_CORS_ORIGINS")

	// Health
	_ = v.BindEnv("health.enabled", "HEALTH_CHECK_ENABLED")
	_ = v.BindEnv("health.interval", "HEALTH_CHECK_INTERVAL")
	_ = v.BindEnv("health.check_db", "READINESS_CHECK_DB")
	_ = v.BindEnv("health.check_llm", "READINESS_CHECK_LLM")
}

func validate(cfg *Config) error {
	// Validate app
	if cfg.App.Port < 1 || cfg.App.Port > 65535 {
		return fmt.Errorf("invalid app.port: must be between 1 and 65535")
	}

	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[cfg.App.Env] {
		return fmt.Errorf("invalid app.env: must be development, staging, or production")
	}

	// Validate database
	if cfg.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if cfg.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}

	// Validate LLM
	validProviders := map[string]bool{"openai": true, "anthropic": true, "gemini": true}
	if !validProviders[cfg.LLM.Default.Provider] {
		return fmt.Errorf("invalid llm.default.provider: must be openai, anthropic, or gemini")
	}

	// Provider-specific validation
	if cfg.LLM.Default.Provider == "openai" && cfg.LLM.OpenAI.APIKey == "" {
		return fmt.Errorf("llm.openai.api_key is required when provider is openai")
	}

	// Validate observability
	if cfg.Observability.Tracing.SamplingRatio < 0 || cfg.Observability.Tracing.SamplingRatio > 1.0 {
		return fmt.Errorf("invalid observability.tracing.sampling_ratio: must be between 0.0 and 1.0")
	}

	// Validate operations
	if cfg.Operations.CircuitBreaker.Threshold < 1 {
		return fmt.Errorf("invalid operations.circuit_breaker.threshold: must be >= 1")
	}
	if cfg.Operations.RateLimit.RequestsPerSecond <= 0 {
		return fmt.Errorf("invalid operations.rate_limit.requests_per_second: must be > 0")
	}
	if cfg.Operations.Retry.MaxAttempts < 1 {
		return fmt.Errorf("invalid operations.retry.max_attempts: must be >= 1")
	}

	return nil
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// IsProduction returns true if running in production environment
func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// IsDevelopment returns true if running in development environment
func (c *AppConfig) IsDevelopment() bool {
	return c.Env == "development"
}

// IsStaging returns true if running in staging environment
func (c *AppConfig) IsStaging() bool {
	return c.Env == "staging"
}
