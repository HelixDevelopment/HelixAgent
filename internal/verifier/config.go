package verifier

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the verifier configuration
type Config struct {
	Enabled      bool               `yaml:"enabled"`
	Database     DatabaseConfig     `yaml:"database"`
	Verification VerificationConfig `yaml:"verification"`
	Scoring      ScoringConfig      `yaml:"scoring"`
	Health       HealthConfig       `yaml:"health"`
	API          APIConfig          `yaml:"api"`
	Events       EventsConfig       `yaml:"events"`
	Monitoring   MonitoringConfig   `yaml:"monitoring"`
	Brotli       BrotliConfig       `yaml:"brotli"`
	Challenges   ChallengesConfig   `yaml:"challenges"`
	Scheduling   SchedulingConfig   `yaml:"scheduling"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path              string `yaml:"path"`
	EncryptionEnabled bool   `yaml:"encryption_enabled"`
	EncryptionKey     string `yaml:"encryption_key"`
}

// VerificationConfig represents verification settings
type VerificationConfig struct {
	MandatoryCodeCheck    bool          `yaml:"mandatory_code_check"`
	CodeVisibilityPrompt  string        `yaml:"code_visibility_prompt"`
	VerificationTimeout   time.Duration `yaml:"verification_timeout"`
	RetryCount            int           `yaml:"retry_count"`
	RetryDelay            time.Duration `yaml:"retry_delay"`
	Tests                 []string      `yaml:"tests"`
}

// ScoringConfig represents scoring configuration
type ScoringConfig struct {
	Weights           ScoringWeightsConfig `yaml:"weights"`
	ModelsDevEnabled  bool                 `yaml:"models_dev_enabled"`
	ModelsDevEndpoint string               `yaml:"models_dev_endpoint"`
	CacheTTL          time.Duration        `yaml:"cache_ttl"`
}

// ScoringWeightsConfig represents scoring weights configuration
type ScoringWeightsConfig struct {
	ResponseSpeed     float64 `yaml:"response_speed"`
	ModelEfficiency   float64 `yaml:"model_efficiency"`
	CostEffectiveness float64 `yaml:"cost_effectiveness"`
	Capability        float64 `yaml:"capability"`
	Recency           float64 `yaml:"recency"`
}

// HealthConfig represents health checking configuration
type HealthConfig struct {
	CheckInterval     time.Duration       `yaml:"check_interval"`
	Timeout           time.Duration       `yaml:"timeout"`
	FailureThreshold  int                 `yaml:"failure_threshold"`
	RecoveryThreshold int                 `yaml:"recovery_threshold"`
	CircuitBreaker    CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled         bool          `yaml:"enabled"`
	HalfOpenTimeout time.Duration `yaml:"half_open_timeout"`
}

// APIConfig represents API configuration
type APIConfig struct {
	Enabled   bool            `yaml:"enabled"`
	Port      string          `yaml:"port"`
	BasePath  string          `yaml:"base_path"`
	JWTSecret string          `yaml:"jwt_secret"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

// EventsConfig represents event notification configuration
type EventsConfig struct {
	Slack     SlackConfig     `yaml:"slack"`
	Email     EmailConfig     `yaml:"email"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	WebSocket WebSocketConfig `yaml:"websocket"`
}

// SlackConfig represents Slack notification configuration
type SlackConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
}

// TelegramConfig represents Telegram notification configuration
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// WebSocketConfig represents WebSocket configuration
type WebSocketConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Grafana    GrafanaConfig    `yaml:"grafana"`
}

// PrometheusConfig represents Prometheus configuration
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// GrafanaConfig represents Grafana configuration
type GrafanaConfig struct {
	Enabled       bool   `yaml:"enabled"`
	DashboardPath string `yaml:"dashboard_path"`
}

// BrotliConfig represents Brotli compression configuration
type BrotliConfig struct {
	Enabled          bool `yaml:"enabled"`
	HTTP3Support     bool `yaml:"http3_support"`
	CompressionLevel int  `yaml:"compression_level"`
}

// ChallengesConfig represents challenge system configuration
type ChallengesConfig struct {
	Enabled           bool `yaml:"enabled"`
	ProviderDiscovery bool `yaml:"provider_discovery"`
	ModelVerification bool `yaml:"model_verification"`
	ConfigGeneration  bool `yaml:"config_generation"`
}

// SchedulingConfig represents scheduling configuration
type SchedulingConfig struct {
	ReVerification      ScheduleConfig `yaml:"re_verification"`
	ScoreRecalculation  ScheduleConfig `yaml:"score_recalculation"`
}

// ScheduleConfig represents a schedule configuration
type ScheduleConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

// LoadConfig loads verifier configuration from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables
	content := os.ExpandEnv(string(data))

	var config struct {
		Verifier Config `yaml:"verifier"`
	}

	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, err
	}

	// Apply defaults
	applyDefaults(&config.Verifier)

	return &config.Verifier, nil
}

// LoadConfigFromBytes loads configuration from bytes
func LoadConfigFromBytes(data []byte) (*Config, error) {
	content := os.ExpandEnv(string(data))

	var config struct {
		Verifier Config `yaml:"verifier"`
	}

	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, err
	}

	applyDefaults(&config.Verifier)

	return &config.Verifier, nil
}

// applyDefaults applies default values to configuration
func applyDefaults(cfg *Config) {
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/llm-verifier.db"
	}

	if cfg.Verification.VerificationTimeout == 0 {
		cfg.Verification.VerificationTimeout = 60 * time.Second
	}

	if cfg.Verification.RetryCount == 0 {
		cfg.Verification.RetryCount = 3
	}

	if cfg.Verification.RetryDelay == 0 {
		cfg.Verification.RetryDelay = 5 * time.Second
	}

	if cfg.Verification.CodeVisibilityPrompt == "" {
		cfg.Verification.CodeVisibilityPrompt = "Do you see my code?"
	}

	if len(cfg.Verification.Tests) == 0 {
		cfg.Verification.Tests = []string{
			"existence", "responsiveness", "latency", "streaming",
			"function_calling", "coding_capability", "error_detection",
		}
	}

	// Scoring defaults
	if cfg.Scoring.Weights.ResponseSpeed == 0 {
		cfg.Scoring.Weights.ResponseSpeed = 0.25
	}
	if cfg.Scoring.Weights.ModelEfficiency == 0 {
		cfg.Scoring.Weights.ModelEfficiency = 0.20
	}
	if cfg.Scoring.Weights.CostEffectiveness == 0 {
		cfg.Scoring.Weights.CostEffectiveness = 0.25
	}
	if cfg.Scoring.Weights.Capability == 0 {
		cfg.Scoring.Weights.Capability = 0.20
	}
	if cfg.Scoring.Weights.Recency == 0 {
		cfg.Scoring.Weights.Recency = 0.10
	}

	if cfg.Scoring.ModelsDevEndpoint == "" {
		cfg.Scoring.ModelsDevEndpoint = "https://api.models.dev"
	}

	if cfg.Scoring.CacheTTL == 0 {
		cfg.Scoring.CacheTTL = 24 * time.Hour
	}

	// Health defaults
	if cfg.Health.CheckInterval == 0 {
		cfg.Health.CheckInterval = 30 * time.Second
	}

	if cfg.Health.Timeout == 0 {
		cfg.Health.Timeout = 10 * time.Second
	}

	if cfg.Health.FailureThreshold == 0 {
		cfg.Health.FailureThreshold = 5
	}

	if cfg.Health.RecoveryThreshold == 0 {
		cfg.Health.RecoveryThreshold = 3
	}

	if cfg.Health.CircuitBreaker.HalfOpenTimeout == 0 {
		cfg.Health.CircuitBreaker.HalfOpenTimeout = 60 * time.Second
	}

	// API defaults
	if cfg.API.Port == "" {
		cfg.API.Port = "8081"
	}

	if cfg.API.BasePath == "" {
		cfg.API.BasePath = "/api/v1/verifier"
	}

	if cfg.API.RateLimit.RequestsPerMinute == 0 {
		cfg.API.RateLimit.RequestsPerMinute = 100
	}

	// Monitoring defaults
	if cfg.Monitoring.Prometheus.Path == "" {
		cfg.Monitoring.Prometheus.Path = "/metrics/verifier"
	}

	// Brotli defaults
	if cfg.Brotli.CompressionLevel == 0 {
		cfg.Brotli.CompressionLevel = 6
	}

	// Events defaults
	if cfg.Events.WebSocket.Path == "" {
		cfg.Events.WebSocket.Path = "/ws/verifier/events"
	}

	// Scheduling defaults
	if cfg.Scheduling.ReVerification.Interval == 0 {
		cfg.Scheduling.ReVerification.Interval = 24 * time.Hour
	}

	if cfg.Scheduling.ScoreRecalculation.Interval == 0 {
		cfg.Scheduling.ScoreRecalculation.Interval = 12 * time.Hour
	}
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	cfg := &Config{
		Enabled: true,
	}
	applyDefaults(cfg)
	return cfg
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate scoring weights sum to 1.0
	sum := c.Scoring.Weights.ResponseSpeed +
		c.Scoring.Weights.ModelEfficiency +
		c.Scoring.Weights.CostEffectiveness +
		c.Scoring.Weights.Capability +
		c.Scoring.Weights.Recency

	if sum < 0.99 || sum > 1.01 {
		return &ConfigValidationError{
			Field:   "scoring.weights",
			Message: "weights must sum to 1.0",
		}
	}

	return nil
}

// ConfigValidationError represents a configuration validation error
type ConfigValidationError struct {
	Field   string
	Message string
}

func (e *ConfigValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ExpandEnvVars expands environment variables in a string
func ExpandEnvVars(s string) string {
	return os.ExpandEnv(s)
}

// MaskSensitiveData masks sensitive data in configuration for logging
func (c *Config) MaskSensitiveData() *Config {
	masked := *c
	if masked.Database.EncryptionKey != "" {
		masked.Database.EncryptionKey = "***"
	}
	if masked.API.JWTSecret != "" {
		masked.API.JWTSecret = "***"
	}
	if masked.Events.Slack.WebhookURL != "" {
		masked.Events.Slack.WebhookURL = maskURL(masked.Events.Slack.WebhookURL)
	}
	if masked.Events.Telegram.BotToken != "" {
		masked.Events.Telegram.BotToken = "***"
	}
	return &masked
}

// maskURL masks the sensitive parts of a URL
func maskURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 3 {
		for i := 3; i < len(parts); i++ {
			parts[i] = "***"
		}
	}
	return strings.Join(parts, "/")
}
