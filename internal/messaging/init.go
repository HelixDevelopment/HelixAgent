// Package messaging provides initialization and configuration for the messaging system.
package messaging

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// MessagingConfig holds configuration for the messaging system initialization.
type MessagingConfig struct {
	// Enabled enables the messaging system.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// RabbitMQ configuration
	RabbitMQ struct {
		Enabled  bool   `json:"enabled" yaml:"enabled"`
		Host     string `json:"host" yaml:"host"`
		Port     int    `json:"port" yaml:"port"`
		Username string `json:"username" yaml:"username"`
		Password string `json:"password" yaml:"password"`
		VHost    string `json:"vhost" yaml:"vhost"`
	} `json:"rabbitmq" yaml:"rabbitmq"`

	// Kafka configuration
	Kafka struct {
		Enabled  bool     `json:"enabled" yaml:"enabled"`
		Brokers  []string `json:"brokers" yaml:"brokers"`
		ClientID string   `json:"client_id" yaml:"client_id"`
		GroupID  string   `json:"group_id" yaml:"group_id"`
	} `json:"kafka" yaml:"kafka"`

	// FallbackToInMemory enables in-memory fallback when brokers unavailable.
	FallbackToInMemory bool `json:"fallback_to_inmemory" yaml:"fallback_to_inmemory"`

	// ConnectionTimeout for broker connections.
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
}

// DefaultMessagingConfig returns the default messaging configuration.
func DefaultMessagingConfig() *MessagingConfig {
	cfg := &MessagingConfig{
		Enabled:            true,
		FallbackToInMemory: true,
		ConnectionTimeout:  30 * time.Second,
	}

	// RabbitMQ defaults
	cfg.RabbitMQ.Enabled = true
	cfg.RabbitMQ.Host = getEnvOrDefault("RABBITMQ_HOST", "localhost")
	cfg.RabbitMQ.Port = 5672
	cfg.RabbitMQ.Username = getEnvOrDefault("RABBITMQ_USER", "helixagent")
	cfg.RabbitMQ.Password = getEnvOrDefault("RABBITMQ_PASSWORD", "helixagent123")
	cfg.RabbitMQ.VHost = getEnvOrDefault("RABBITMQ_VHOST", "/")

	// Kafka defaults
	cfg.Kafka.Enabled = true
	cfg.Kafka.Brokers = []string{getEnvOrDefault("KAFKA_BROKERS", "localhost:9092")}
	cfg.Kafka.ClientID = getEnvOrDefault("KAFKA_CLIENT_ID", "helixagent")
	cfg.Kafka.GroupID = getEnvOrDefault("KAFKA_GROUP_ID", "helixagent-group")

	return cfg
}

// LoadMessagingConfigFromEnv loads messaging configuration from environment variables.
func LoadMessagingConfigFromEnv() *MessagingConfig {
	cfg := DefaultMessagingConfig()

	// Override from environment
	if os.Getenv("MESSAGING_ENABLED") == "false" {
		cfg.Enabled = false
	}
	if os.Getenv("RABBITMQ_ENABLED") == "false" {
		cfg.RabbitMQ.Enabled = false
	}
	if os.Getenv("KAFKA_ENABLED") == "false" {
		cfg.Kafka.Enabled = false
	}
	if os.Getenv("MESSAGING_FALLBACK_INMEMORY") == "false" {
		cfg.FallbackToInMemory = false
	}

	return cfg
}

// FallbackBrokerFactory creates a fallback broker instance.
// This allows callers to provide their own fallback broker implementation.
type FallbackBrokerFactory func() MessageBroker

// MessagingSystem holds all messaging components.
type MessagingSystem struct {
	Hub                   *MessagingHub
	Config                *MessagingConfig
	Logger                *logrus.Logger
	FallbackBrokerFactory FallbackBrokerFactory
	started               bool
}

// NewMessagingSystem creates a new messaging system with the given configuration.
func NewMessagingSystem(cfg *MessagingConfig, logger *logrus.Logger) *MessagingSystem {
	if cfg == nil {
		cfg = DefaultMessagingConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &MessagingSystem{
		Config: cfg,
		Logger: logger,
	}
}

// Initialize initializes the messaging system.
// It attempts to connect to configured brokers and falls back to in-memory if enabled.
func (m *MessagingSystem) Initialize(ctx context.Context) error {
	if !m.Config.Enabled {
		m.Logger.Info("Messaging system is disabled")
		return nil
	}

	m.Logger.Info("Initializing messaging system...")

	// Create hub configuration
	hubConfig := DefaultHubConfig()
	hubConfig.TaskQueueEnabled = m.Config.RabbitMQ.Enabled
	hubConfig.EventStreamEnabled = m.Config.Kafka.Enabled
	hubConfig.FallbackEnabled = m.Config.FallbackToInMemory
	hubConfig.UseFallbackOnError = m.Config.FallbackToInMemory

	// Create the hub
	m.Hub = NewMessagingHub(hubConfig)

	// Initialize brokers
	var taskQueueConnected, eventStreamConnected bool
	var taskQueueErr, eventStreamErr error

	// Try to connect to RabbitMQ for task queue
	if m.Config.RabbitMQ.Enabled {
		m.Logger.Info("Connecting to RabbitMQ for task queue...")
		// Note: In production, you would create the actual RabbitMQ broker here
		// For now, we'll use the in-memory fallback
		taskQueueConnected = false
		taskQueueErr = fmt.Errorf("RabbitMQ broker not yet implemented in init.go - using fallback")
		m.Logger.WithError(taskQueueErr).Warn("RabbitMQ connection not available")
	}

	// Try to connect to Kafka for event stream
	if m.Config.Kafka.Enabled {
		m.Logger.Info("Connecting to Kafka for event stream...")
		// Note: In production, you would create the actual Kafka broker here
		// For now, we'll use the in-memory fallback
		eventStreamConnected = false
		eventStreamErr = fmt.Errorf("Kafka broker not yet implemented in init.go - using fallback")
		m.Logger.WithError(eventStreamErr).Warn("Kafka connection not available")
	}

	// Set up fallback broker if needed
	if m.Config.FallbackToInMemory && (!taskQueueConnected || !eventStreamConnected) {
		if m.FallbackBrokerFactory == nil {
			m.Logger.Warn("Fallback enabled but no FallbackBrokerFactory provided - skipping fallback setup")
		} else {
			m.Logger.Info("Setting up fallback broker...")
			fallback := m.FallbackBrokerFactory()
			if fallback == nil {
				return fmt.Errorf("FallbackBrokerFactory returned nil")
			}
			if err := fallback.Connect(ctx); err != nil {
				return fmt.Errorf("failed to connect fallback broker: %w", err)
			}
			m.Hub.SetFallbackBroker(fallback)
			m.Logger.Info("Fallback broker connected")
		}
	}

	// Initialize the hub
	if err := m.Hub.Initialize(ctx); err != nil {
		if !m.Config.FallbackToInMemory {
			return fmt.Errorf("failed to initialize messaging hub: %w", err)
		}
		m.Logger.WithError(err).Warn("Messaging hub initialization had errors, using fallback")
	}

	// Set as global hub for easy access
	SetGlobalHub(m.Hub)

	m.started = true
	m.Logger.WithFields(logrus.Fields{
		"task_queue_connected":   taskQueueConnected,
		"event_stream_connected": eventStreamConnected,
		"using_fallback":         m.Config.FallbackToInMemory && (!taskQueueConnected || !eventStreamConnected),
	}).Info("Messaging system initialized")

	return nil
}

// Close shuts down the messaging system.
func (m *MessagingSystem) Close(ctx context.Context) error {
	if !m.started || m.Hub == nil {
		return nil
	}

	m.Logger.Info("Shutting down messaging system...")

	if err := m.Hub.Close(ctx); err != nil {
		m.Logger.WithError(err).Error("Error closing messaging hub")
		return err
	}

	m.started = false
	m.Logger.Info("Messaging system shut down")
	return nil
}

// IsInitialized returns true if the messaging system is initialized.
func (m *MessagingSystem) IsInitialized() bool {
	return m.started && m.Hub != nil
}

// GetHub returns the messaging hub.
func (m *MessagingSystem) GetHub() *MessagingHub {
	return m.Hub
}

// Helper to get environment variable with default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Global messaging system instance.
var globalMessagingSystem *MessagingSystem

// SetGlobalMessagingSystem sets the global messaging system instance.
func SetGlobalMessagingSystem(system *MessagingSystem) {
	globalMessagingSystem = system
}

// GetGlobalMessagingSystem returns the global messaging system instance.
func GetGlobalMessagingSystem() *MessagingSystem {
	return globalMessagingSystem
}

// InitializeGlobalMessagingSystem creates and initializes a global messaging system.
// The fallbackFactory parameter is optional; if nil, no fallback broker will be created.
func InitializeGlobalMessagingSystem(ctx context.Context, logger *logrus.Logger, fallbackFactory FallbackBrokerFactory) (*MessagingSystem, error) {
	cfg := LoadMessagingConfigFromEnv()
	system := NewMessagingSystem(cfg, logger)
	system.FallbackBrokerFactory = fallbackFactory

	if err := system.Initialize(ctx); err != nil {
		return nil, err
	}

	SetGlobalMessagingSystem(system)
	return system, nil
}
