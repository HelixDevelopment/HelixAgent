package messaging

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultMessagingConfig(t *testing.T) {
	cfg := DefaultMessagingConfig()

	assert.True(t, cfg.Enabled)
	assert.True(t, cfg.FallbackToInMemory)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)

	// RabbitMQ defaults
	assert.True(t, cfg.RabbitMQ.Enabled)
	assert.Equal(t, "localhost", cfg.RabbitMQ.Host)
	assert.Equal(t, 5672, cfg.RabbitMQ.Port)
	assert.Equal(t, "helixagent", cfg.RabbitMQ.Username)
	assert.Equal(t, "helixagent123", cfg.RabbitMQ.Password)
	assert.Equal(t, "/", cfg.RabbitMQ.VHost)

	// Kafka defaults
	assert.True(t, cfg.Kafka.Enabled)
	assert.Equal(t, []string{"localhost:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "helixagent", cfg.Kafka.ClientID)
	assert.Equal(t, "helixagent-group", cfg.Kafka.GroupID)
}

func TestDefaultMessagingConfig_EnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(*testing.T, *MessagingConfig)
	}{
		{
			name:     "custom RabbitMQ host",
			envKey:   "RABBITMQ_HOST",
			envValue: "rabbit.example.com",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "rabbit.example.com", cfg.RabbitMQ.Host)
			},
		},
		{
			name:     "custom RabbitMQ password",
			envKey:   "RABBITMQ_PASSWORD",
			envValue: "secret123",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "secret123", cfg.RabbitMQ.Password)
			},
		},
		{
			name:     "custom RabbitMQ user",
			envKey:   "RABBITMQ_USER",
			envValue: "admin",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "admin", cfg.RabbitMQ.Username)
			},
		},
		{
			name:     "custom RabbitMQ vhost",
			envKey:   "RABBITMQ_VHOST",
			envValue: "/production",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "/production", cfg.RabbitMQ.VHost)
			},
		},
		{
			name:     "custom Kafka brokers",
			envKey:   "KAFKA_BROKERS",
			envValue: "kafka1:9092",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, []string{"kafka1:9092"}, cfg.Kafka.Brokers)
			},
		},
		{
			name:     "custom Kafka client ID",
			envKey:   "KAFKA_CLIENT_ID",
			envValue: "my-client",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "my-client", cfg.Kafka.ClientID)
			},
		},
		{
			name:     "custom Kafka group ID",
			envKey:   "KAFKA_GROUP_ID",
			envValue: "my-group",
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.Equal(t, "my-group", cfg.Kafka.GroupID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Getenv(tt.envKey)
			os.Setenv(tt.envKey, tt.envValue)
			defer os.Setenv(tt.envKey, old)

			cfg := DefaultMessagingConfig()
			tt.check(t, cfg)
		})
	}
}

func TestLoadMessagingConfigFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		envs  map[string]string
		check func(*testing.T, *MessagingConfig)
	}{
		{
			name: "all defaults when no env set",
			envs: map[string]string{},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.True(t, cfg.Enabled)
				assert.True(t, cfg.RabbitMQ.Enabled)
				assert.True(t, cfg.Kafka.Enabled)
				assert.True(t, cfg.FallbackToInMemory)
			},
		},
		{
			name: "messaging disabled",
			envs: map[string]string{"MESSAGING_ENABLED": "false"},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.False(t, cfg.Enabled)
			},
		},
		{
			name: "rabbitmq disabled",
			envs: map[string]string{"RABBITMQ_ENABLED": "false"},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.False(t, cfg.RabbitMQ.Enabled)
			},
		},
		{
			name: "kafka disabled",
			envs: map[string]string{"KAFKA_ENABLED": "false"},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.False(t, cfg.Kafka.Enabled)
			},
		},
		{
			name: "fallback disabled",
			envs: map[string]string{"MESSAGING_FALLBACK_INMEMORY": "false"},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.False(t, cfg.FallbackToInMemory)
			},
		},
		{
			name: "messaging enabled stays true for non-false values",
			envs: map[string]string{"MESSAGING_ENABLED": "true"},
			check: func(t *testing.T, cfg *MessagingConfig) {
				assert.True(t, cfg.Enabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env vars
			restoreFuncs := make([]func(), 0, len(tt.envs))
			for k, v := range tt.envs {
				old := os.Getenv(k)
				os.Setenv(k, v)
				key := k
				oldVal := old
				restoreFuncs = append(restoreFuncs, func() {
					if oldVal == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, oldVal)
					}
				})
			}
			defer func() {
				for _, fn := range restoreFuncs {
					fn()
				}
			}()

			cfg := LoadMessagingConfigFromEnv()
			tt.check(t, cfg)
		})
	}
}

func TestNewMessagingSystem(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *MessagingConfig
		logger *logrus.Logger
	}{
		{
			name:   "with nil config and nil logger",
			cfg:    nil,
			logger: nil,
		},
		{
			name:   "with custom config",
			cfg:    &MessagingConfig{Enabled: true},
			logger: logrus.New(),
		},
		{
			name:   "with nil logger only",
			cfg:    DefaultMessagingConfig(),
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := NewMessagingSystem(tt.cfg, tt.logger)
			require.NotNil(t, system)
			assert.NotNil(t, system.Config)
			assert.NotNil(t, system.Logger)
			assert.False(t, system.IsInitialized())
			assert.Nil(t, system.GetHub())
		})
	}
}

func TestMessagingSystem_Initialize_Disabled(t *testing.T) {
	cfg := DefaultMessagingConfig()
	cfg.Enabled = false

	system := NewMessagingSystem(cfg, logrus.New())
	err := system.Initialize(context.Background())

	assert.NoError(t, err)
	assert.False(t, system.IsInitialized())
}

func TestMessagingSystem_Initialize_FallbackNoFactory(t *testing.T) {
	cfg := DefaultMessagingConfig()
	cfg.FallbackToInMemory = true
	cfg.RabbitMQ.Enabled = true
	cfg.Kafka.Enabled = true

	system := NewMessagingSystem(cfg, logrus.New())
	// No fallback factory set - should warn but not fail
	err := system.Initialize(context.Background())
	// The hub init may fail but we get past the factory check
	// because factory is nil (just warns, doesn't error)
	assert.NoError(t, err)
}

func TestMessagingSystem_Close_NotStarted(t *testing.T) {
	cfg := DefaultMessagingConfig()
	cfg.Enabled = false

	system := NewMessagingSystem(cfg, logrus.New())
	err := system.Close(context.Background())
	assert.NoError(t, err)
}

func TestMessagingSystem_Close_NilHub(t *testing.T) {
	system := NewMessagingSystem(nil, nil)
	err := system.Close(context.Background())
	assert.NoError(t, err)
}

func TestMessagingSystem_IsInitialized_Default(t *testing.T) {
	system := NewMessagingSystem(nil, nil)
	assert.False(t, system.IsInitialized())
}

func TestMessagingSystem_GetHub_Default(t *testing.T) {
	system := NewMessagingSystem(nil, nil)
	assert.Nil(t, system.GetHub())
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal string
		envValue   string
		setEnv     bool
		expected   string
	}{
		{
			name:       "returns default when env not set",
			key:        "TEST_GETENVDEFAULT_UNSET_12345",
			defaultVal: "default-value",
			setEnv:     false,
			expected:   "default-value",
		},
		{
			name:       "returns env value when set",
			key:        "TEST_GETENVDEFAULT_SET_12345",
			defaultVal: "default-value",
			envValue:   "custom-value",
			setEnv:     true,
			expected:   "custom-value",
		},
		{
			name:       "returns default for empty env value",
			key:        "TEST_GETENVDEFAULT_EMPTY_12345",
			defaultVal: "fallback",
			envValue:   "",
			setEnv:     true,
			expected:   "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultVal)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGlobalMessagingSystem(t *testing.T) {
	// Clear any existing global state
	oldGlobal := GetGlobalMessagingSystem()
	defer SetGlobalMessagingSystem(oldGlobal)

	// Initially may be nil
	SetGlobalMessagingSystem(nil)
	assert.Nil(t, GetGlobalMessagingSystem())

	// Set a system
	system := NewMessagingSystem(nil, nil)
	SetGlobalMessagingSystem(system)
	assert.Equal(t, system, GetGlobalMessagingSystem())

	// Replace with another
	system2 := NewMessagingSystem(nil, nil)
	SetGlobalMessagingSystem(system2)
	assert.Equal(t, system2, GetGlobalMessagingSystem())
}

func TestMessagingSystem_FallbackBrokerFactory(t *testing.T) {
	system := NewMessagingSystem(nil, nil)
	assert.Nil(t, system.FallbackBrokerFactory)

	factory := func() MessageBroker { return nil }
	system.FallbackBrokerFactory = factory
	assert.NotNil(t, system.FallbackBrokerFactory)
}

func TestMessagingConfig_StructFields(t *testing.T) {
	cfg := &MessagingConfig{
		Enabled:            true,
		FallbackToInMemory: false,
		ConnectionTimeout:  10 * time.Second,
	}
	cfg.RabbitMQ.Enabled = true
	cfg.RabbitMQ.Host = "rabbit-host"
	cfg.RabbitMQ.Port = 5673
	cfg.RabbitMQ.Username = "user"
	cfg.RabbitMQ.Password = "pass"
	cfg.RabbitMQ.VHost = "/test"
	cfg.Kafka.Enabled = true
	cfg.Kafka.Brokers = []string{"broker1", "broker2"}
	cfg.Kafka.ClientID = "client1"
	cfg.Kafka.GroupID = "group1"

	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.FallbackToInMemory)
	assert.Equal(t, 10*time.Second, cfg.ConnectionTimeout)
	assert.True(t, cfg.RabbitMQ.Enabled)
	assert.Equal(t, "rabbit-host", cfg.RabbitMQ.Host)
	assert.Equal(t, 5673, cfg.RabbitMQ.Port)
	assert.Equal(t, "user", cfg.RabbitMQ.Username)
	assert.Equal(t, "pass", cfg.RabbitMQ.Password)
	assert.Equal(t, "/test", cfg.RabbitMQ.VHost)
	assert.True(t, cfg.Kafka.Enabled)
	assert.Equal(t, []string{"broker1", "broker2"}, cfg.Kafka.Brokers)
	assert.Equal(t, "client1", cfg.Kafka.ClientID)
	assert.Equal(t, "group1", cfg.Kafka.GroupID)
}
