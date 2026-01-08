package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Cognee      CogneeConfig
	LLM         LLMConfig
	ModelsDev   ModelsDevConfig
	Monitoring  MonitoringConfig
	Security    SecurityConfig
	Plugins     PluginConfig
	Performance PerformanceConfig
	MCP         MCPConfig
	ACP         ACPConfig
}

// ACPConfig contains ACP (Agent Client Protocol) configuration
type ACPConfig struct {
	Enabled        bool
	DefaultTimeout time.Duration
	MaxRetries     int
	Servers        []ACPServerConfig
}

// ACPServerConfig represents a pre-configured ACP server
type ACPServerConfig struct {
	ID      string
	Name    string
	URL     string
	Enabled bool
}

type ServerConfig struct {
	Port           string
	APIKey         string
	JWTSecret      string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	TokenExpiry    time.Duration
	Host           string
	Mode           string // "debug" or "release"
	EnableCORS     bool
	CORSOrigins    []string
	RequestLogging bool
	DebugEnabled   bool
}

type DatabaseConfig struct {
	Host           string
	Port           string
	User           string
	Password       string
	Name           string
	SSLMode        string
	MaxConnections int
	ConnTimeout    time.Duration
	PoolSize       int
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
	Timeout  time.Duration
}

type CogneeConfig struct {
	BaseURL     string
	APIKey      string
	AutoCognify bool
	Timeout     time.Duration
	Enabled     bool
}

type LLMConfig struct {
	DefaultTimeout time.Duration
	MaxRetries     int
	Providers      map[string]ProviderConfig
	Ensemble       EnsembleConfig
	Streaming      StreamingConfig
}

type ProviderConfig struct {
	Enabled       bool
	APIKey        string
	BaseURL       string
	Model         string
	Timeout       time.Duration
	MaxTokens     int
	Temperature   float64
	Weight        float64
	RateLimitRPS  int
	RetryAttempts int
}

type EnsembleConfig struct {
	Strategy            string
	MinProviders        int
	MaxProviders        int
	ConfidenceThreshold float64
	FallbackToBest      bool
	Timeout             time.Duration
	PreferredProviders  []string
}

type StreamingConfig struct {
	Enabled         bool
	BufferSize      int
	KeepAlive       time.Duration
	EnableHeartbeat bool
}

type ModelsDevConfig struct {
	Enabled          bool
	APIKey           string
	BaseURL          string
	RefreshInterval  time.Duration
	CacheTTL         time.Duration
	DefaultBatchSize int
	MaxRetries       int
	AutoRefresh      bool
}

type MonitoringConfig struct {
	Enabled        bool
	MetricsPath    string
	LogLevel       string
	TracingEnabled bool
	JaegerEndpoint string
	Prometheus     PrometheusConfig
}

type PrometheusConfig struct {
	Enabled   bool
	Path      string
	Port      string
	Namespace string
}

type SecurityConfig struct {
	SessionTimeout   time.Duration
	MaxLoginAttempts int
	LockoutDuration  time.Duration
	RateLimiting     RateLimitConfig
	CORS             CORSConfig
}

type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Window   time.Duration
	Strategy string // "sliding_window", "token_bucket"
}

type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	MaxAge           time.Duration
	AllowCredentials bool
}

type PluginConfig struct {
	WatchPaths     []string
	AutoReload     bool
	EnabledPlugins []string
	PluginDirs     []string
	HotReload      bool
}

type PerformanceConfig struct {
	MaxConcurrentRequests int
	RequestTimeout        time.Duration
	IdleTimeout           time.Duration
	ReadBufferSize        int
	WriteBufferSize       int
	EnableCompression     bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           getEnv("PORT", "7061"),
			APIKey:         getEnv("HELIXAGENT_API_KEY", ""), // SECURITY: Must be set via environment variable
			JWTSecret:      getEnv("JWT_SECRET", ""),         // SECURITY: Must be set via environment variable
			ReadTimeout:    getDurationEnv("READ_TIMEOUT", 30*time.Second),
			WriteTimeout:   getDurationEnv("WRITE_TIMEOUT", 30*time.Second),
			TokenExpiry:    getDurationEnv("TOKEN_EXPIRY", 24*time.Hour),
			Host:           getEnv("SERVER_HOST", "0.0.0.0"),
			Mode:           getEnv("GIN_MODE", "release"),
			EnableCORS:     getBoolEnv("CORS_ENABLED", true),
			CORSOrigins:    getEnvSlice("CORS_ORIGINS", []string{"*"}),
			RequestLogging: getBoolEnv("REQUEST_LOGGING", true),
			DebugEnabled:   getBoolEnv("DEBUG_ENABLED", false),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           getEnv("DB_PORT", "5432"),
			User:           getEnv("DB_USER", "helixagent"),
			Password:       getEnv("DB_PASSWORD", "secret"),
			Name:           getEnv("DB_NAME", "helixagent_db"),
			SSLMode:        getEnv("DB_SSLMODE", "disable"),
			MaxConnections: getIntEnv("DB_MAX_CONNECTIONS", 20),
			ConnTimeout:    getDurationEnv("DB_CONN_TIMEOUT", 10*time.Second),
			PoolSize:       getIntEnv("DB_POOL_SIZE", 10),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
			PoolSize: getIntEnv("REDIS_POOL_SIZE", 10),
			Timeout:  getDurationEnv("REDIS_TIMEOUT", 5*time.Second),
		},
		Cognee: CogneeConfig{
			BaseURL:     getEnv("COGNEE_BASE_URL", "http://localhost:8000"),
			APIKey:      getEnv("COGNEE_API_KEY", ""),
			AutoCognify: getBoolEnv("COGNEE_AUTO_COGNIFY", true),
			Timeout:     getDurationEnv("COGNEE_TIMEOUT", 30*time.Second),
			Enabled:     getBoolEnv("COGNEE_ENABLED", true),
		},
		LLM: LLMConfig{
			DefaultTimeout: getDurationEnv("LLM_TIMEOUT", 60*time.Second),
			MaxRetries:     getIntEnv("LLM_MAX_RETRIES", 3),
			Providers:      make(map[string]ProviderConfig),
			Ensemble: EnsembleConfig{
				Strategy:            getEnv("ENSEMBLE_STRATEGY", "confidence_weighted"),
				MinProviders:        getIntEnv("ENSEMBLE_MIN_PROVIDERS", 2),
				MaxProviders:        getIntEnv("ENSEMBLE_MAX_PROVIDERS", 5),
				ConfidenceThreshold: getFloatEnv("ENSEMBLE_CONFIDENCE_THRESHOLD", 0.8),
				FallbackToBest:      getBoolEnv("ENSEMBLE_FALLBACK_BEST", true),
				Timeout:             getDurationEnv("ENSEMBLE_TIMEOUT", 30*time.Second),
				PreferredProviders:  getEnvSlice("ENSEMBLE_PREFERRED_PROVIDERS", []string{}),
			},
			Streaming: StreamingConfig{
				Enabled:         getBoolEnv("STREAMING_ENABLED", true),
				BufferSize:      getIntEnv("STREAMING_BUFFER_SIZE", 1024),
				KeepAlive:       getDurationEnv("STREAMING_KEEP_ALIVE", 30*time.Second),
				EnableHeartbeat: getBoolEnv("STREAMING_HEARTBEAT", true),
			},
		},
		ModelsDev: ModelsDevConfig{
			Enabled:          getBoolEnv("MODELSDEV_ENABLED", false),
			APIKey:           getEnv("MODELSDEV_API_KEY", ""),
			BaseURL:          getEnv("MODELSDEV_BASE_URL", "https://api.models.dev/v1"),
			RefreshInterval:  getDurationEnv("MODELSDEV_REFRESH_INTERVAL", 24*time.Hour),
			CacheTTL:         getDurationEnv("MODELSDEV_CACHE_TTL", 1*time.Hour),
			DefaultBatchSize: getIntEnv("MODELSDEV_BATCH_SIZE", 100),
			MaxRetries:       getIntEnv("MODELSDEV_MAX_RETRIES", 3),
			AutoRefresh:      getBoolEnv("MODELSDEV_AUTO_REFRESH", true),
		},
		Monitoring: MonitoringConfig{
			Enabled:        getBoolEnv("METRICS_ENABLED", true),
			MetricsPath:    getEnv("METRICS_PATH", "/metrics"),
			LogLevel:       getEnv("LOG_LEVEL", "info"),
			TracingEnabled: getBoolEnv("TRACING_ENABLED", false),
			JaegerEndpoint: getEnv("JAEGER_ENDPOINT", ""),
			Prometheus: PrometheusConfig{
				Enabled:   getBoolEnv("PROMETHEUS_ENABLED", true),
				Path:      getEnv("PROMETHEUS_PATH", "/metrics"),
				Port:      getEnv("PROMETHEUS_PORT", "9090"),
				Namespace: getEnv("PROMETHEUS_NAMESPACE", "helixagent"),
			},
		},
		Security: SecurityConfig{
			SessionTimeout:   getDurationEnv("SESSION_TIMEOUT", 24*time.Hour),
			MaxLoginAttempts: getIntEnv("MAX_LOGIN_ATTEMPTS", 5),
			LockoutDuration:  getDurationEnv("LOCKOUT_DURATION", 15*time.Minute),
			RateLimiting: RateLimitConfig{
				Enabled:  getBoolEnv("RATE_LIMITING_ENABLED", true),
				Requests: getIntEnv("RATE_LIMIT_REQUESTS", 100),
				Window:   getDurationEnv("RATE_LIMIT_WINDOW", time.Minute),
				Strategy: getEnv("RATE_LIMIT_STRATEGY", "sliding_window"),
			},
			CORS: CORSConfig{
				Enabled:          getBoolEnv("CORS_ENABLED", true),
				AllowedOrigins:   getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
				AllowedMethods:   getEnvSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
				AllowedHeaders:   getEnvSlice("CORS_ALLOWED_HEADERS", []string{"*"}),
				MaxAge:           getDurationEnv("CORS_MAX_AGE", time.Hour),
				AllowCredentials: getBoolEnv("CORS_ALLOW_CREDENTIALS", false),
			},
		},
		Plugins: PluginConfig{
			WatchPaths:     getEnvSlice("PLUGIN_WATCH_PATHS", []string{"./plugins"}),
			AutoReload:     getBoolEnv("PLUGIN_AUTO_RELOAD", false),
			EnabledPlugins: getEnvSlice("PLUGIN_ENABLED_PLUGINS", []string{}),
			PluginDirs:     getEnvSlice("PLUGIN_DIRS", []string{"./plugins"}),
			HotReload:      getBoolEnv("PLUGIN_HOT_RELOAD", false),
		},
		Performance: PerformanceConfig{
			MaxConcurrentRequests: getIntEnv("MAX_CONCURRENT_REQUESTS", 10),
			RequestTimeout:        getDurationEnv("REQUEST_TIMEOUT", 60*time.Second),
			IdleTimeout:           getDurationEnv("IDLE_TIMEOUT", 120*time.Second),
			ReadBufferSize:        getIntEnv("READ_BUFFER_SIZE", 4096),
			WriteBufferSize:       getIntEnv("WRITE_BUFFER_SIZE", 4096),
			EnableCompression:     getBoolEnv("ENABLE_COMPRESSION", true),
		},
		MCP: MCPConfig{
			Enabled:              getBoolEnv("MCP_ENABLED", true),
			ExposeAllTools:       getBoolEnv("MCP_EXPOSE_ALL_TOOLS", true),
			UnifiedToolNamespace: getBoolEnv("MCP_UNIFIED_NAMESPACE", true),
		},
		ACP: ACPConfig{
			Enabled:        getBoolEnv("ACP_ENABLED", true),
			DefaultTimeout: getDurationEnv("ACP_DEFAULT_TIMEOUT", 30*time.Second),
			MaxRetries:     getIntEnv("ACP_MAX_RETRIES", 3),
			Servers:        []ACPServerConfig{},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
