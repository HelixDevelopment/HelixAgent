package flink

import (
	"fmt"
	"time"
)

// Config holds Apache Flink connection and job configuration
type Config struct {
	// JobManager settings
	JobManagerHost string `json:"jobmanager_host" yaml:"jobmanager_host"`
	JobManagerPort int    `json:"jobmanager_port" yaml:"jobmanager_port"`
	WebUIPort      int    `json:"web_ui_port" yaml:"web_ui_port"`

	// REST API settings
	RESTURL        string        `json:"rest_url" yaml:"rest_url"`
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`

	// Checkpoint settings
	CheckpointEnabled  bool          `json:"checkpoint_enabled" yaml:"checkpoint_enabled"`
	CheckpointInterval time.Duration `json:"checkpoint_interval" yaml:"checkpoint_interval"`
	CheckpointMode     string        `json:"checkpoint_mode" yaml:"checkpoint_mode"` // exactly_once, at_least_once
	CheckpointTimeout  time.Duration `json:"checkpoint_timeout" yaml:"checkpoint_timeout"`
	CheckpointMinPause time.Duration `json:"checkpoint_min_pause" yaml:"checkpoint_min_pause"`
	CheckpointDir      string        `json:"checkpoint_dir" yaml:"checkpoint_dir"`

	// Savepoint settings
	SavepointDir string `json:"savepoint_dir" yaml:"savepoint_dir"`

	// State backend
	StateBackend             string `json:"state_backend" yaml:"state_backend"` // hashmap, rocksdb
	IncrementalCheckpoints   bool   `json:"incremental_checkpoints" yaml:"incremental_checkpoints"`

	// Restart strategy
	RestartStrategy       string        `json:"restart_strategy" yaml:"restart_strategy"` // none, fixed-delay, failure-rate, exponential-delay
	RestartAttempts       int           `json:"restart_attempts" yaml:"restart_attempts"`
	RestartDelay          time.Duration `json:"restart_delay" yaml:"restart_delay"`

	// Kafka integration
	KafkaBootstrapServers string        `json:"kafka_bootstrap_servers" yaml:"kafka_bootstrap_servers"`
	KafkaGroupID          string        `json:"kafka_group_id" yaml:"kafka_group_id"`
	KafkaConsumerTimeout  time.Duration `json:"kafka_consumer_timeout" yaml:"kafka_consumer_timeout"`

	// Parallelism
	DefaultParallelism int `json:"default_parallelism" yaml:"default_parallelism"`

	// Metrics
	MetricsEnabled bool   `json:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsPort    int    `json:"metrics_port" yaml:"metrics_port"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		JobManagerHost:         "localhost",
		JobManagerPort:         6123,
		WebUIPort:              8082,
		RESTURL:                "http://localhost:8082",
		RequestTimeout:         30 * time.Second,
		CheckpointEnabled:      true,
		CheckpointInterval:     60 * time.Second,
		CheckpointMode:         "exactly_once",
		CheckpointTimeout:      600 * time.Second,
		CheckpointMinPause:     500 * time.Millisecond,
		CheckpointDir:          "s3://helixagent-checkpoints/flink",
		SavepointDir:           "s3://helixagent-checkpoints/savepoints",
		StateBackend:           "rocksdb",
		IncrementalCheckpoints: true,
		RestartStrategy:        "fixed-delay",
		RestartAttempts:        3,
		RestartDelay:           10 * time.Second,
		KafkaBootstrapServers:  "localhost:9092",
		KafkaGroupID:           "flink-helixagent-group",
		KafkaConsumerTimeout:   30 * time.Second,
		DefaultParallelism:     4,
		MetricsEnabled:         true,
		MetricsPort:            9249,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.JobManagerHost == "" {
		return fmt.Errorf("jobmanager_host is required")
	}
	if c.JobManagerPort <= 0 || c.JobManagerPort > 65535 {
		return fmt.Errorf("jobmanager_port must be between 1 and 65535")
	}
	if c.WebUIPort <= 0 || c.WebUIPort > 65535 {
		return fmt.Errorf("web_ui_port must be between 1 and 65535")
	}
	if c.RESTURL == "" {
		return fmt.Errorf("rest_url is required")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive")
	}
	if c.CheckpointEnabled {
		if c.CheckpointInterval <= 0 {
			return fmt.Errorf("checkpoint_interval must be positive when checkpoints are enabled")
		}
		if c.CheckpointMode != "exactly_once" && c.CheckpointMode != "at_least_once" {
			return fmt.Errorf("checkpoint_mode must be 'exactly_once' or 'at_least_once'")
		}
		if c.CheckpointDir == "" {
			return fmt.Errorf("checkpoint_dir is required when checkpoints are enabled")
		}
	}
	if c.StateBackend != "" && c.StateBackend != "hashmap" && c.StateBackend != "rocksdb" {
		return fmt.Errorf("state_backend must be 'hashmap' or 'rocksdb'")
	}
	validStrategies := map[string]bool{"none": true, "fixed-delay": true, "failure-rate": true, "exponential-delay": true}
	if c.RestartStrategy != "" && !validStrategies[c.RestartStrategy] {
		return fmt.Errorf("restart_strategy must be 'none', 'fixed-delay', 'failure-rate', or 'exponential-delay'")
	}
	if c.DefaultParallelism < 1 {
		return fmt.Errorf("default_parallelism must be at least 1")
	}
	return nil
}

// GetRESTURL returns the full REST API URL
func (c *Config) GetRESTURL() string {
	if c.RESTURL != "" {
		return c.RESTURL
	}
	return fmt.Sprintf("http://%s:%d", c.JobManagerHost, c.WebUIPort)
}

// JobConfig holds configuration for a specific Flink job
type JobConfig struct {
	Name           string            `json:"name" yaml:"name"`
	JarPath        string            `json:"jar_path" yaml:"jar_path"`
	EntryClass     string            `json:"entry_class" yaml:"entry_class"`
	Parallelism    int               `json:"parallelism" yaml:"parallelism"`
	ProgramArgs    []string          `json:"program_args" yaml:"program_args"`
	AllowNonRestoredState bool       `json:"allow_non_restored_state" yaml:"allow_non_restored_state"`
	SavepointPath  string            `json:"savepoint_path" yaml:"savepoint_path"`
	Properties     map[string]string `json:"properties" yaml:"properties"`
}

// DefaultJobConfig returns a JobConfig with defaults
func DefaultJobConfig(name string) *JobConfig {
	return &JobConfig{
		Name:        name,
		Parallelism: 4,
		ProgramArgs: []string{},
		AllowNonRestoredState: false,
		Properties:  make(map[string]string),
	}
}

// Validate validates the job configuration
func (jc *JobConfig) Validate() error {
	if jc.Name == "" {
		return fmt.Errorf("job name is required")
	}
	if jc.JarPath == "" {
		return fmt.Errorf("jar_path is required")
	}
	if jc.Parallelism < 1 {
		return fmt.Errorf("parallelism must be at least 1")
	}
	return nil
}

// WithParallelism sets the parallelism and returns the config for chaining
func (jc *JobConfig) WithParallelism(p int) *JobConfig {
	jc.Parallelism = p
	return jc
}

// WithEntryClass sets the entry class and returns the config for chaining
func (jc *JobConfig) WithEntryClass(class string) *JobConfig {
	jc.EntryClass = class
	return jc
}

// WithProgramArgs sets the program arguments and returns the config for chaining
func (jc *JobConfig) WithProgramArgs(args ...string) *JobConfig {
	jc.ProgramArgs = args
	return jc
}

// WithSavepoint sets the savepoint path and returns the config for chaining
func (jc *JobConfig) WithSavepoint(path string) *JobConfig {
	jc.SavepointPath = path
	return jc
}

// WithProperty sets a property and returns the config for chaining
func (jc *JobConfig) WithProperty(key, value string) *JobConfig {
	if jc.Properties == nil {
		jc.Properties = make(map[string]string)
	}
	jc.Properties[key] = value
	return jc
}
