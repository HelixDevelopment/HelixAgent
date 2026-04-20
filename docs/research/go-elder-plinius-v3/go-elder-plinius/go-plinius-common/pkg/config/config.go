// Package config provides centralized configuration management for all
// Plinius Go service clients. It supports environment variables, YAML
// configuration files, and programmatic configuration.
//
// Use this package to uniformly configure connection parameters, timeouts,
// retry policies, and authentication across all Plinius service clients.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the common configuration for any Plinius gRPC service client.
type Config struct {
	ServiceName       string            `yaml:"service_name"`
	Address           string            `yaml:"address"`
	Timeout           time.Duration     `yaml:"timeout"`
	ConnectionTimeout time.Duration     `yaml:"connection_timeout"`
	MaxRetries        int               `yaml:"max_retries"`
	RetryBackoff      time.Duration     `yaml:"retry_backoff"`
	MaxRetryBackoff   time.Duration     `yaml:"max_retry_backoff"`
	EnableTLS         bool              `yaml:"enable_tls"`
	TLSCertPath       string            `yaml:"tls_cert_path"`
	TLSKeyPath        string            `yaml:"tls_key_path"`
	TLSCAPath         string            `yaml:"tls_ca_path"`
	TLSServerName     string            `yaml:"tls_server_name"`
	InsecureSkipVerify bool             `yaml:"insecure_skip_verify"`
	KeepaliveTime     time.Duration     `yaml:"keepalive_time"`
	KeepaliveTimeout  time.Duration     `yaml:"keepalive_timeout"`
	MaxRecvMsgSize    int               `yaml:"max_recv_msg_size"`
	MaxSendMsgSize    int               `yaml:"max_send_msg_size"`
	AuthToken         string            `yaml:"auth_token"`
	Compression       string            `yaml:"compression"`
	Metadata          map[string]string `yaml:"metadata"`
}

// Option is a functional option for configuring a Config.
type Option func(*Config)

// New creates a new Config with the given service name and options.
func New(serviceName string, opts ...Option) *Config {
	cfg := &Config{
		ServiceName:        serviceName,
		Address:            "localhost:50051",
		Timeout:            30 * time.Second,
		ConnectionTimeout:  10 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       1 * time.Second,
		MaxRetryBackoff:    30 * time.Second,
		EnableTLS:          false,
		KeepaliveTime:      10 * time.Second,
		KeepaliveTimeout:   5 * time.Second,
		MaxRecvMsgSize:     64 * 1024 * 1024,
		MaxSendMsgSize:     16 * 1024 * 1024,
		Compression:        "",
		Metadata:           make(map[string]string),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithAddress sets the gRPC server address.
func WithAddress(addr string) Option {
	return func(c *Config) { c.Address = addr }
}

// WithTimeout sets the RPC timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) { c.Timeout = d }
}

// WithConnectionTimeout sets the connection establishment timeout.
func WithConnectionTimeout(d time.Duration) Option {
	return func(c *Config) { c.ConnectionTimeout = d }
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *Config) { c.MaxRetries = n }
}

// WithRetryBackoff sets the base retry backoff duration.
func WithRetryBackoff(d time.Duration) Option {
	return func(c *Config) { c.RetryBackoff = d }
}

// WithMaxRetryBackoff sets the maximum retry backoff duration.
func WithMaxRetryBackoff(d time.Duration) Option {
	return func(c *Config) { c.MaxRetryBackoff = d }
}

// WithTLS enables TLS with the given certificate paths.
func WithTLS(certPath, keyPath, caPath string) Option {
	return func(c *Config) {
		c.EnableTLS = true
		c.TLSCertPath = certPath
		c.TLSKeyPath = keyPath
		c.TLSCAPath = caPath
	}
}

// WithInsecureSkipVerify disables TLS verification (dev only).
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *Config) { c.InsecureSkipVerify = skip }
}

// WithAuthToken sets the bearer token for authentication.
func WithAuthToken(token string) Option {
	return func(c *Config) { c.AuthToken = token }
}

// WithCompression sets the compression algorithm ("gzip", "snappy", or "").
func WithCompression(algo string) Option {
	return func(c *Config) { c.Compression = algo }
}

// WithKeepalive sets keepalive ping parameters.
func WithKeepalive(time_, timeout time.Duration) Option {
	return func(c *Config) {
		c.KeepaliveTime = time_
		c.KeepaliveTimeout = timeout
	}
}

// WithMaxMessageSize sets both send and receive message size limits.
func WithMaxMessageSize(bytes int) Option {
	return func(c *Config) {
		c.MaxRecvMsgSize = bytes
		c.MaxSendMsgSize = bytes
	}
}

// WithMetadata adds custom gRPC metadata.
func WithMetadata(key, value string) Option {
	return func(c *Config) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]string)
		}
		c.Metadata[key] = value
	}
}

// FromEnv creates a Config from environment variables.
// Variables are prefixed with {SERVICE_NAME}_ (uppercase).
func FromEnv(serviceName string) *Config {
	prefix := strings.ToUpper(serviceName) + "_"
	cfg := New(serviceName)

	if v := os.Getenv(prefix + "ADDRESS"); v != "" {
		cfg.Address = v
	}
	if v := os.Getenv(prefix + "TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
		}
	}
	if v := os.Getenv(prefix + "CONNECTION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ConnectionTimeout = d
		}
	}
	if v := os.Getenv(prefix + "MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv(prefix + "RETRY_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RetryBackoff = d
		}
	}
	if v := os.Getenv(prefix + "MAX_RETRY_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxRetryBackoff = d
		}
	}
	if v := os.Getenv(prefix + "ENABLE_TLS"); v != "" {
		cfg.EnableTLS = strings.EqualFold(v, "true")
	}
	if v := os.Getenv(prefix + "TLS_CERT_PATH"); v != "" {
		cfg.TLSCertPath = v
	}
	if v := os.Getenv(prefix + "TLS_KEY_PATH"); v != "" {
		cfg.TLSKeyPath = v
	}
	if v := os.Getenv(prefix + "TLS_CA_PATH"); v != "" {
		cfg.TLSCAPath = v
	}
	if v := os.Getenv(prefix + "TLS_SERVER_NAME"); v != "" {
		cfg.TLSServerName = v
	}
	if v := os.Getenv(prefix + "INSECURE_SKIP_VERIFY"); v != "" {
		cfg.InsecureSkipVerify = strings.EqualFold(v, "true")
	}
	if v := os.Getenv(prefix + "AUTH_TOKEN"); v != "" {
		cfg.AuthToken = v
	}
	if v := os.Getenv(prefix + "COMPRESSION"); v != "" {
		cfg.Compression = v
	}
	if v := os.Getenv(prefix + "KEEPALIVE_TIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.KeepaliveTime = d
		}
	}
	if v := os.Getenv(prefix + "KEEPALIVE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.KeepaliveTimeout = d
		}
	}
	if v := os.Getenv(prefix + "MAX_RECV_MSG_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRecvMsgSize = n
		}
	}
	if v := os.Getenv(prefix + "MAX_SEND_MSG_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSendMsgSize = n
		}
	}

	return cfg
}

// FromFile creates a Config from a YAML file.
func FromFile(path string, serviceName string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if svcData, ok := root[serviceName]; ok {
		svcBytes, err := yaml.Marshal(svcData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal service config: %w", err)
		}
		var cfg Config
		if err := yaml.Unmarshal(svcBytes, &cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal service config: %w", err)
		}
		cfg.ServiceName = serviceName
		return &cfg, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = serviceName
	}
	return &cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection_timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if c.RetryBackoff <= 0 {
		return fmt.Errorf("retry_backoff must be positive")
	}
	if c.EnableTLS && c.TLSCertPath == "" {
		return fmt.Errorf("tls_cert_path is required when TLS is enabled")
	}
	return nil
}

// EnvPrefix returns the environment variable prefix for this service.
func (c *Config) EnvPrefix() string {
	return strings.ToUpper(c.ServiceName) + "_"
}
