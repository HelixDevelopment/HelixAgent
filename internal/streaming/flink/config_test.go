package flink

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.JobManagerHost)
	assert.Equal(t, 6123, config.JobManagerPort)
	assert.Equal(t, 8082, config.WebUIPort)
	assert.Equal(t, "http://localhost:8082", config.RESTURL)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.True(t, config.CheckpointEnabled)
	assert.Equal(t, "exactly_once", config.CheckpointMode)
	assert.Equal(t, "rocksdb", config.StateBackend)
	assert.Equal(t, "fixed-delay", config.RestartStrategy)
	assert.Equal(t, 4, config.DefaultParallelism)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "empty jobmanager host",
			modify: func(c *Config) {
				c.JobManagerHost = ""
			},
			expectError: true,
			errorMsg:    "jobmanager_host is required",
		},
		{
			name: "invalid jobmanager port",
			modify: func(c *Config) {
				c.JobManagerPort = 0
			},
			expectError: true,
			errorMsg:    "jobmanager_port must be between 1 and 65535",
		},
		{
			name: "invalid web ui port",
			modify: func(c *Config) {
				c.WebUIPort = -1
			},
			expectError: true,
			errorMsg:    "web_ui_port must be between 1 and 65535",
		},
		{
			name: "empty rest url",
			modify: func(c *Config) {
				c.RESTURL = ""
			},
			expectError: true,
			errorMsg:    "rest_url is required",
		},
		{
			name: "invalid request timeout",
			modify: func(c *Config) {
				c.RequestTimeout = 0
			},
			expectError: true,
			errorMsg:    "request_timeout must be positive",
		},
		{
			name: "invalid checkpoint interval",
			modify: func(c *Config) {
				c.CheckpointEnabled = true
				c.CheckpointInterval = 0
			},
			expectError: true,
			errorMsg:    "checkpoint_interval must be positive",
		},
		{
			name: "invalid checkpoint mode",
			modify: func(c *Config) {
				c.CheckpointEnabled = true
				c.CheckpointMode = "invalid"
			},
			expectError: true,
			errorMsg:    "checkpoint_mode must be 'exactly_once' or 'at_least_once'",
		},
		{
			name: "empty checkpoint dir with checkpoints enabled",
			modify: func(c *Config) {
				c.CheckpointEnabled = true
				c.CheckpointDir = ""
			},
			expectError: true,
			errorMsg:    "checkpoint_dir is required when checkpoints are enabled",
		},
		{
			name: "invalid state backend",
			modify: func(c *Config) {
				c.StateBackend = "invalid"
			},
			expectError: true,
			errorMsg:    "state_backend must be 'hashmap' or 'rocksdb'",
		},
		{
			name: "invalid restart strategy",
			modify: func(c *Config) {
				c.RestartStrategy = "invalid"
			},
			expectError: true,
			errorMsg:    "restart_strategy must be 'none', 'fixed-delay', 'failure-rate', or 'exponential-delay'",
		},
		{
			name: "invalid parallelism",
			modify: func(c *Config) {
				c.DefaultParallelism = 0
			},
			expectError: true,
			errorMsg:    "default_parallelism must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)

			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigGetRESTURL(t *testing.T) {
	t.Run("returns configured URL", func(t *testing.T) {
		config := DefaultConfig()
		config.RESTURL = "http://custom:9999"

		assert.Equal(t, "http://custom:9999", config.GetRESTURL())
	})

	t.Run("constructs URL from host and port when empty", func(t *testing.T) {
		config := DefaultConfig()
		config.RESTURL = ""
		config.JobManagerHost = "flink-host"
		config.WebUIPort = 8081

		assert.Equal(t, "http://flink-host:8081", config.GetRESTURL())
	})
}

func TestDefaultJobConfig(t *testing.T) {
	config := DefaultJobConfig("test-job")

	assert.Equal(t, "test-job", config.Name)
	assert.Equal(t, 4, config.Parallelism)
	assert.Empty(t, config.ProgramArgs)
	assert.False(t, config.AllowNonRestoredState)
	assert.NotNil(t, config.Properties)
}

func TestJobConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *JobConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &JobConfig{
				Name:        "test-job",
				JarPath:     "/path/to/jar.jar",
				Parallelism: 4,
			},
			expectError: false,
		},
		{
			name: "empty name",
			config: &JobConfig{
				Name:        "",
				JarPath:     "/path/to/jar.jar",
				Parallelism: 4,
			},
			expectError: true,
			errorMsg:    "job name is required",
		},
		{
			name: "empty jar path",
			config: &JobConfig{
				Name:        "test-job",
				JarPath:     "",
				Parallelism: 4,
			},
			expectError: true,
			errorMsg:    "jar_path is required",
		},
		{
			name: "invalid parallelism",
			config: &JobConfig{
				Name:        "test-job",
				JarPath:     "/path/to/jar.jar",
				Parallelism: 0,
			},
			expectError: true,
			errorMsg:    "parallelism must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJobConfigChaining(t *testing.T) {
	config := DefaultJobConfig("test-job").
		WithParallelism(8).
		WithEntryClass("com.example.Main").
		WithProgramArgs("--input", "topic", "--output", "sink").
		WithSavepoint("/savepoints/sp1").
		WithProperty("key1", "value1").
		WithProperty("key2", "value2")

	assert.Equal(t, "test-job", config.Name)
	assert.Equal(t, 8, config.Parallelism)
	assert.Equal(t, "com.example.Main", config.EntryClass)
	assert.Equal(t, []string{"--input", "topic", "--output", "sink"}, config.ProgramArgs)
	assert.Equal(t, "/savepoints/sp1", config.SavepointPath)
	assert.Equal(t, "value1", config.Properties["key1"])
	assert.Equal(t, "value2", config.Properties["key2"])
}
