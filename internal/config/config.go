package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"dev.helix.agent/internal/memory"
)

type Config struct {
	Server           ServerConfig
	Database         DatabaseConfig
	Redis            RedisConfig
	Cognee           CogneeConfig
	Memory           memory.MemoryConfig // Mem0-style memory system (PRIMARY)
	MemoryEnabled    bool                // Whether memory service is enabled (default: true)
	LLM              LLMConfig
	ModelsDev        ModelsDevConfig
	Monitoring       MonitoringConfig
	Security         SecurityConfig
	Plugins          PluginConfig
	Performance      PerformanceConfig
	MCP              MCPConfig
	ACP              ACPConfig
	Services         ServicesConfig
	BigData          BigDataConfig
	RemoteDeployment RemoteDeploymentConfig
}

// ServiceEndpoint represents a configurable service endpoint that can be local or remote.
type ServiceEndpoint struct {
	Host        string        `yaml:"host"`
	Port        string        `yaml:"port"`
	URL         string        `yaml:"url"`          // Full URL override (takes precedence over host:port)
	Enabled     bool          `yaml:"enabled"`      // Whether this service is used
	Required    bool          `yaml:"required"`     // Boot fails if unavailable
	Remote      bool          `yaml:"remote"`       // Skip compose start, only health check
	HealthPath  string        `yaml:"health_path"`  // HTTP health check path (e.g. "/health")
	HealthType  string        `yaml:"health_type"`  // "tcp", "http", "pgx", "redis"
	Timeout     time.Duration `yaml:"timeout"`      // Health check timeout
	RetryCount  int           `yaml:"retry_count"`  // Number of health check retries
	ComposeFile string        `yaml:"compose_file"` // Docker compose file path
	ServiceName string        `yaml:"service_name"` // Docker compose service name
	Profile     string        `yaml:"profile"`      // Docker compose profile
	// Discovery settings
	DiscoveryEnabled bool          `yaml:"discovery_enabled"` // Enable automatic service discovery
	DiscoveryMethod  string        `yaml:"discovery_method"`  // "tcp", "http", "dns", "mdns"
	DiscoveryTimeout time.Duration `yaml:"discovery_timeout"` // Timeout for discovery attempts
	// Runtime state (not configurable via YAML)
	Discovered bool `yaml:"-"` // Whether service was discovered in network
}

// ResolvedURL builds the full URL from host:port or returns the URL field if set.
func (e *ServiceEndpoint) ResolvedURL() string {
	if e.URL != "" {
		return e.URL
	}
	if e.Host == "" {
		return ""
	}
	port := e.Port
	if port == "" {
		return e.Host
	}
	return e.Host + ":" + port
}

// ServicesConfig holds configuration for all infrastructure services.
type ServicesConfig struct {
	PostgreSQL  ServiceEndpoint            `yaml:"postgresql"`
	Redis       ServiceEndpoint            `yaml:"redis"`
	Cognee      ServiceEndpoint            `yaml:"cognee"`
	ChromaDB    ServiceEndpoint            `yaml:"chromadb"`
	Prometheus  ServiceEndpoint            `yaml:"prometheus"`
	Grafana     ServiceEndpoint            `yaml:"grafana"`
	Neo4j       ServiceEndpoint            `yaml:"neo4j"`
	Kafka       ServiceEndpoint            `yaml:"kafka"`
	Zookeeper   ServiceEndpoint            `yaml:"zookeeper"`
	RabbitMQ    ServiceEndpoint            `yaml:"rabbitmq"`
	Qdrant      ServiceEndpoint            `yaml:"qdrant"`
	Weaviate    ServiceEndpoint            `yaml:"weaviate"`
	LangChain   ServiceEndpoint            `yaml:"langchain"`
	LlamaIndex  ServiceEndpoint            `yaml:"llamaindex"`
	ClickHouse  ServiceEndpoint            `yaml:"clickhouse"`
	MinIO       ServiceEndpoint            `yaml:"minio"`
	SparkMaster ServiceEndpoint            `yaml:"spark_master"`
	SparkWorker ServiceEndpoint            `yaml:"spark_worker"`
	MCPServers  map[string]ServiceEndpoint `yaml:"mcp_servers"`
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
	DefaultTimeout       time.Duration
	MaxRetries           int
	Providers            map[string]ProviderConfig
	Ensemble             EnsembleConfig
	Streaming            StreamingConfig
	DisableAutoDiscovery bool // Disable auto-discovery of providers from env vars (for testing)
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

// BigDataConfig holds configuration for big data integration components
type BigDataConfig struct {
	// Enable/disable individual components
	EnableInfiniteContext   bool
	EnableDistributedMemory bool
	EnableKnowledgeGraph    bool
	EnableAnalytics         bool
	EnableCrossLearning     bool

	// Kafka configuration
	KafkaBootstrapServers string
	KafkaConsumerGroup    string

	// ClickHouse configuration
	ClickHouseHost     string
	ClickHousePort     int
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string

	// Neo4j configuration
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string
	Neo4jDatabase string

	// Context engine configuration
	ContextCacheSize       int
	ContextCacheTTL        time.Duration
	ContextCompressionType string

	// Learning configuration
	LearningMinConfidence float64
	LearningMinFrequency  int
}

// RemoteDeploymentConfig holds configuration for deploying services to remote hosts.
type RemoteDeploymentConfig struct {
	Enabled          bool                            `yaml:"enabled"`
	SSHKey           string                          `yaml:"ssh_key"`
	DefaultSSHUser   string                          `yaml:"default_ssh_user"`
	DefaultRemoteDir string                          `yaml:"default_remote_dir"`
	Hosts            map[string]RemoteDeploymentHost `yaml:"hosts"`
}

// RemoteDeploymentHost defines SSH access and services for a remote host.
type RemoteDeploymentHost struct {
	SSHHost  string   `yaml:"ssh_host"` // user@hostname
	SSHKey   string   `yaml:"ssh_key"`  // optional, overrides global SSHKey
	Services []string `yaml:"services"` // service names to deploy on this host
}

func Load() *Config {
	cfg := &Config{
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
			SSLMode:        getEnvWithFallback("DB_SSLMODE", "DB_SSL_MODE", "disable"),
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
			AutoCognify: getBoolEnv("COGNEE_AUTO_COGNIFY", false),         // DISABLED - Mem0 is now primary memory provider
			Timeout:     getDurationEnv("COGNEE_TIMEOUT", 15*time.Second), // Increased to 15s for Cognee cold start + processing
			Enabled:     getBoolEnv("COGNEE_ENABLED", false),              // DISABLED - Replaced by Mem0 memory system
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
				Requests: getIntEnvWithFallback("RATE_LIMIT_REQUESTS", "RATE_LIMIT_RPM", 100),
				Window:   getDurationEnv("RATE_LIMIT_WINDOW", time.Minute),
				Strategy: getEnv("RATE_LIMIT_STRATEGY", "sliding_window"),
			},
			CORS: CORSConfig{
				Enabled:          getBoolEnv("CORS_ENABLED", true),
				AllowedOrigins:   getEnvSliceWithFallback("CORS_ALLOWED_ORIGINS", "ALLOWED_ORIGINS", []string{"*"}),
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
		BigData: BigDataConfig{
			EnableInfiniteContext:   getBoolEnv("BIGDATA_ENABLE_INFINITE_CONTEXT", true),
			EnableDistributedMemory: getBoolEnv("BIGDATA_ENABLE_DISTRIBUTED_MEMORY", false),
			EnableKnowledgeGraph:    getBoolEnv("BIGDATA_ENABLE_KNOWLEDGE_GRAPH", false),
			EnableAnalytics:         getBoolEnv("BIGDATA_ENABLE_ANALYTICS", false),
			EnableCrossLearning:     getBoolEnv("BIGDATA_ENABLE_CROSS_LEARNING", true),
			KafkaBootstrapServers:   getEnv("BIGDATA_KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"),
			KafkaConsumerGroup:      getEnv("BIGDATA_KAFKA_CONSUMER_GROUP", "helixagent-bigdata"),
			ClickHouseHost:          getEnv("BIGDATA_CLICKHOUSE_HOST", "localhost"),
			ClickHousePort:          getIntEnv("BIGDATA_CLICKHOUSE_PORT", 8123),
			ClickHouseDatabase:      getEnv("BIGDATA_CLICKHOUSE_DATABASE", "helixagent"),
			ClickHouseUser:          getEnv("BIGDATA_CLICKHOUSE_USER", "default"),
			ClickHousePassword:      getEnv("BIGDATA_CLICKHOUSE_PASSWORD", ""),
			Neo4jURI:                getEnv("BIGDATA_NEO4J_URI", "bolt://localhost:7687"),
			Neo4jUsername:           getEnv("BIGDATA_NEO4J_USERNAME", "neo4j"),
			Neo4jPassword:           getEnv("BIGDATA_NEO4J_PASSWORD", "password"),
			Neo4jDatabase:           getEnv("BIGDATA_NEO4J_DATABASE", "neo4j"),
			ContextCacheSize:        getIntEnv("BIGDATA_CONTEXT_CACHE_SIZE", 1000),
			ContextCacheTTL:         getDurationEnv("BIGDATA_CONTEXT_CACHE_TTL", 24*time.Hour),
			ContextCompressionType:  getEnv("BIGDATA_CONTEXT_COMPRESSION_TYPE", "gzip"),
			LearningMinConfidence:   getFloatEnv("BIGDATA_LEARNING_MIN_CONFIDENCE", 0.7),
			LearningMinFrequency:    getIntEnv("BIGDATA_LEARNING_MIN_FREQUENCY", 3),
		},
		RemoteDeployment: RemoteDeploymentConfig{
			Enabled:          false,
			SSHKey:           "",
			DefaultSSHUser:   "",
			DefaultRemoteDir: "/opt/helixagent",
			Hosts:            map[string]RemoteDeploymentHost{},
		},
		Services:      DefaultServicesConfig(),
		MemoryEnabled: getBoolEnv("MEMORY_ENABLED", true), // Mem0 memory enabled by default
	}

	// Apply environment variable overrides for services
	LoadServicesFromEnv(&cfg.Services)

	return cfg
}

// DefaultServicesConfig returns the default configuration for all infrastructure services.
// When CONTAINERS_REMOTE_ENABLED=true, all services are marked as Remote=true.
func DefaultServicesConfig() ServicesConfig {
	// Check if remote distribution is enabled
	remoteEnabled := isContainersRemoteEnabled()

	// Remote configuration is determined by CONTAINERS_REMOTE_ENABLED environment variable
	// or Containers/.env file. When true, all services are marked as Remote=true for
	// distribution to remote hosts.

	cfg := ServicesConfig{
		PostgreSQL: ServiceEndpoint{
			Host:        "localhost",
			Port:        "5432",
			Enabled:     true,
			Required:    true,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "pgx",
			Timeout:     10 * time.Second,
			RetryCount:  6,
			ComposeFile: "docker-compose.yml",
			ServiceName: "postgres",
			Profile:     "default",
		},
		Redis: ServiceEndpoint{
			Host:        "localhost",
			Port:        "6379",
			Enabled:     true,
			Required:    true,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "redis",
			Timeout:     5 * time.Second,
			RetryCount:  6,
			ComposeFile: "docker-compose.yml",
			ServiceName: "redis",
			Profile:     "default",
		},
		Cognee: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8000",
			Enabled:     false, // DISABLED - Replaced by Mem0 memory system
			Required:    false, // NOT REQUIRED - Mem0 is now primary memory provider
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/",
			HealthType:  "http",
			Timeout:     10 * time.Second,
			RetryCount:  6,
			ComposeFile: "docker-compose.yml",
			ServiceName: "cognee",
			Profile:     "default",
		},
		ChromaDB: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8001",
			Enabled:     true,
			Required:    true,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/api/v2/heartbeat",
			HealthType:  "http",
			Timeout:     10 * time.Second,
			RetryCount:  6,
			ComposeFile: "docker-compose.yml",
			ServiceName: "chromadb",
			Profile:     "default",
		},
		Prometheus: ServiceEndpoint{
			Host:        "localhost",
			Port:        "9090",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/-/healthy",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "prometheus",
		},
		Grafana: ServiceEndpoint{
			Host:        "localhost",
			Port:        "3000",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/api/health",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "grafana",
		},
		Neo4j: ServiceEndpoint{
			Host:        "localhost",
			Port:        "7474",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "http",
			HealthPath:  "/",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "neo4j",
		},
		Kafka: ServiceEndpoint{
			Host:        "localhost",
			Port:        "9092",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "tcp",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "kafka",
		},
		RabbitMQ: ServiceEndpoint{
			Host:        "localhost",
			Port:        "5672",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "tcp",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "rabbitmq",
		},
		Qdrant: ServiceEndpoint{
			Host:        "localhost",
			Port:        "6333",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/healthz",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "qdrant",
		},
		Weaviate: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8080",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/v1/.well-known/ready",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "weaviate",
		},
		LangChain: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8011",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/health",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "langchain",
		},
		LlamaIndex: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8012",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/health",
			HealthType:  "http",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.yml",
			ServiceName: "llamaindex",
		},
		Zookeeper: ServiceEndpoint{
			Host:        "localhost",
			Port:        "2181",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "tcp",
			Timeout:     5 * time.Second,
			RetryCount:  3,
			ComposeFile: "docker-compose.bigdata.yml",
			ServiceName: "zookeeper",
			Profile:     "bigdata",
		},
		ClickHouse: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8123",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/ping",
			HealthType:  "http",
			Timeout:     10 * time.Second,
			RetryCount:  5,
			ComposeFile: "docker-compose.bigdata.yml",
			ServiceName: "clickhouse",
			Profile:     "bigdata",
		},
		MinIO: ServiceEndpoint{
			Host:        "localhost",
			Port:        "9000",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/minio/health/live",
			HealthType:  "http",
			Timeout:     10 * time.Second,
			RetryCount:  5,
			ComposeFile: "docker-compose.bigdata.yml",
			ServiceName: "minio",
			Profile:     "bigdata",
		},
		SparkMaster: ServiceEndpoint{
			Host:        "localhost",
			Port:        "7077",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthType:  "tcp",
			Timeout:     10 * time.Second,
			RetryCount:  5,
			ComposeFile: "docker-compose.bigdata.yml",
			ServiceName: "spark-master",
			Profile:     "bigdata",
		},
		SparkWorker: ServiceEndpoint{
			Host:        "localhost",
			Port:        "8081",
			Enabled:     false,
			Required:    false,
			Remote:      remoteEnabled, // Set based on CONTAINERS_REMOTE_ENABLED
			HealthPath:  "/",
			HealthType:  "http",
			Timeout:     10 * time.Second,
			RetryCount:  5,
			ComposeFile: "docker-compose.bigdata.yml",
			ServiceName: "spark-worker",
			Profile:     "bigdata",
		},
		MCPServers: make(map[string]ServiceEndpoint),
	}
	return cfg
}

// LoadServicesFromEnv applies environment variable overrides to the services config.
// MANDATORY: When CONTAINERS_REMOTE_ENABLED=true in Containers/.env, ALL services
// (except HelixAgent itself) are automatically marked as Remote=true for distribution
// to remote hosts via the Containers module with resource-aware scheduling.
func LoadServicesFromEnv(cfg *ServicesConfig) {
	// Remote flag is already set in DefaultServicesConfig() based on CONTAINERS_REMOTE_ENABLED
	// Here we just apply explicit environment variable overrides

	// DEBUG: Log before and after
	loadServiceEndpointFromEnv("SVC_POSTGRESQL", &cfg.PostgreSQL)
	loadServiceEndpointFromEnv("SVC_REDIS", &cfg.Redis)
	loadServiceEndpointFromEnv("SVC_COGNEE", &cfg.Cognee)
	loadServiceEndpointFromEnv("SVC_CHROMADB", &cfg.ChromaDB)
	loadServiceEndpointFromEnv("SVC_PROMETHEUS", &cfg.Prometheus)
	loadServiceEndpointFromEnv("SVC_GRAFANA", &cfg.Grafana)
	loadServiceEndpointFromEnv("SVC_NEO4J", &cfg.Neo4j)
	loadServiceEndpointFromEnv("SVC_KAFKA", &cfg.Kafka)
	loadServiceEndpointFromEnv("SVC_ZOOKEEPER", &cfg.Zookeeper)
	loadServiceEndpointFromEnv("SVC_RABBITMQ", &cfg.RabbitMQ)
	loadServiceEndpointFromEnv("SVC_QDRANT", &cfg.Qdrant)
	loadServiceEndpointFromEnv("SVC_WEAVIATE", &cfg.Weaviate)
	loadServiceEndpointFromEnv("SVC_LANGCHAIN", &cfg.LangChain)
	loadServiceEndpointFromEnv("SVC_LLAMAINDEX", &cfg.LlamaIndex)
	loadServiceEndpointFromEnv("SVC_CLICKHOUSE", &cfg.ClickHouse)
	loadServiceEndpointFromEnv("SVC_MINIO", &cfg.MinIO)
	loadServiceEndpointFromEnv("SVC_SPARK_MASTER", &cfg.SparkMaster)
	loadServiceEndpointFromEnv("SVC_SPARK_WORKER", &cfg.SparkWorker)

	// Apply environment overrides to MCP servers
	for name, ep := range cfg.MCPServers {
		loadServiceEndpointFromEnv("SVC_MCP_"+strings.ToUpper(name), &ep)
		cfg.MCPServers[name] = ep
	}
}

// isContainersRemoteEnabled checks if CONTAINERS_REMOTE_ENABLED=true in Containers/.env
func isContainersRemoteEnabled() bool {
	// First check environment variable directly
	if v := os.Getenv("CONTAINERS_REMOTE_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}

	// Try to read from Containers/.env file
	envPaths := []string{
		"Containers/.env",
		"/run/media/milosvasic/DATA4TB/Projects/HelixCode/Containers/.env",
	}

	for _, path := range envPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "CONTAINERS_REMOTE_ENABLED=") {
				value := strings.Trim(strings.TrimPrefix(line, "CONTAINERS_REMOTE_ENABLED="), `"'`)
				if b, err := strconv.ParseBool(value); err == nil {
					return b
				}
			}
		}
	}

	return false
}

func loadServiceEndpointFromEnv(prefix string, ep *ServiceEndpoint) {
	if v := os.Getenv(prefix + "_HOST"); v != "" {
		ep.Host = v
	}
	if v := os.Getenv(prefix + "_PORT"); v != "" {
		ep.Port = v
	}
	if v := os.Getenv(prefix + "_URL"); v != "" {
		ep.URL = v
	}
	if v := os.Getenv(prefix + "_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			ep.Enabled = b
		}
	}
	if v := os.Getenv(prefix + "_REQUIRED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			ep.Required = b
		}
	}
	// Allow explicit override of Remote flag via environment variable
	// (by default, Remote is set in DefaultServicesConfig based on CONTAINERS_REMOTE_ENABLED)
	// CRITICAL FIX: Only override Remote if CONTAINERS_REMOTE_ENABLED is not set to true
	// This ensures global remote distribution setting takes precedence over individual service overrides
	if v := os.Getenv(prefix + "_REMOTE"); v != "" && !isContainersRemoteEnabled() {
		if b, err := strconv.ParseBool(v); err == nil {
			ep.Remote = b
		}
	}
	if v := os.Getenv(prefix + "_HEALTH_PATH"); v != "" {
		ep.HealthPath = v
	}
	if v := os.Getenv(prefix + "_HEALTH_TYPE"); v != "" {
		ep.HealthType = v
	}
	if v := os.Getenv(prefix + "_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ep.Timeout = d
		}
	}
	if v := os.Getenv(prefix + "_RETRY_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ep.RetryCount = n
		}
	}
	if v := os.Getenv(prefix + "_DISCOVERY_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			ep.DiscoveryEnabled = b
		}
	}
	if v := os.Getenv(prefix + "_DISCOVERY_METHOD"); v != "" {
		ep.DiscoveryMethod = v
	}
	if v := os.Getenv(prefix + "_DISCOVERY_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ep.DiscoveryTimeout = d
		}
	}
}

// AllEndpoints returns all service endpoints as a name->endpoint map.
func (s *ServicesConfig) AllEndpoints() map[string]ServiceEndpoint {
	endpoints := map[string]ServiceEndpoint{
		"postgresql":   s.PostgreSQL,
		"redis":        s.Redis,
		"cognee":       s.Cognee,
		"chromadb":     s.ChromaDB,
		"prometheus":   s.Prometheus,
		"grafana":      s.Grafana,
		"neo4j":        s.Neo4j,
		"kafka":        s.Kafka,
		"zookeeper":    s.Zookeeper,
		"rabbitmq":     s.RabbitMQ,
		"qdrant":       s.Qdrant,
		"weaviate":     s.Weaviate,
		"langchain":    s.LangChain,
		"llamaindex":   s.LlamaIndex,
		"clickhouse":   s.ClickHouse,
		"minio":        s.MinIO,
		"spark_master": s.SparkMaster,
		"spark_worker": s.SparkWorker,
	}
	for name, ep := range s.MCPServers {
		endpoints["mcp_"+name] = ep
	}
	return endpoints
}

// RequiredEndpoints returns only the enabled and required service endpoints.
func (s *ServicesConfig) RequiredEndpoints() map[string]ServiceEndpoint {
	all := s.AllEndpoints()
	required := make(map[string]ServiceEndpoint)
	for name, ep := range all {
		if ep.Enabled && ep.Required {
			required[name] = ep
		}
	}
	return required
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

// getEnvWithFallback reads the primary key, falling back to an alternate key.
func getEnvWithFallback(primary, fallback, defaultValue string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	if value := os.Getenv(fallback); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnvWithFallback reads an int from the primary key, falling back to an alternate key.
func getIntEnvWithFallback(primary, fallback string, defaultValue int) int {
	if value := os.Getenv(primary); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	if value := os.Getenv(fallback); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvSliceWithFallback reads a comma-separated slice from the primary key,
// falling back to an alternate key.
func getEnvSliceWithFallback(primary, fallback string, defaultValue []string) []string {
	if value := os.Getenv(primary); value != "" {
		return strings.Split(value, ",")
	}
	if value := os.Getenv(fallback); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
