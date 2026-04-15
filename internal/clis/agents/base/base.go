// Package base provides the base integration for all CLI agents.
package base

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"

	"dev.helix.agent/internal/clis/agents"
)

// BaseIntegration provides common functionality for all CLI agent integrations
type BaseIntegration struct {
	info    agents.AgentInfo
	config  interface{}
	workDir string
	envVars map[string]string
	mu      sync.RWMutex
	started bool
}

// NewBaseIntegration creates a new base integration
func NewBaseIntegration(info agents.AgentInfo) *BaseIntegration {
	return &BaseIntegration{
		info:    info,
		envVars: make(map[string]string),
	}
}

// Info returns agent information
func (b *BaseIntegration) Info() agents.AgentInfo {
	return b.info
}

// Initialize initializes the base integration
func (b *BaseIntegration) Initialize(ctx context.Context, config interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.config = config

	b.workDir = extractWorkDir(config)

	// Set default work directory if still empty
	if b.workDir == "" {
		home, _ := os.UserHomeDir()
		b.workDir = filepath.Join(home, ".helixagent", "agents", string(b.info.Type))
		if _, err := os.Stat(b.workDir); err == nil {
			if lockFile, lfErr := os.CreateTemp(b.workDir, ".lock-*"); lfErr == nil {
				lockFile.Close()
				os.Remove(lockFile.Name())
			}
		}
		if isTestBinary() {
			b.workDir = filepath.Join(os.TempDir(), "helixagent-test", string(b.info.Type), uniqueSuffix())
		}
	}

	// Create work directory
	if err := os.MkdirAll(b.workDir, 0755); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}

	return nil
}

// Start starts the base integration
func (b *BaseIntegration) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return nil
	}

	b.started = true
	return nil
}

// Stop stops the base integration
func (b *BaseIntegration) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.started {
		return nil
	}

	b.started = false
	return nil
}

// IsStarted returns whether the integration is started
func (b *BaseIntegration) IsStarted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.started
}

// Health checks the base integration health
func (b *BaseIntegration) Health(ctx context.Context) error {
	if !b.started {
		return fmt.Errorf("not started")
	}
	return nil
}

// IsAvailable checks if the agent binary is available
func (b *BaseIntegration) IsAvailable() bool {
	// Check if the agent command exists
	_, err := exec.LookPath(string(b.info.Type))
	return err == nil
}

// ExecuteCommand executes a shell command
func (b *BaseIntegration) ExecuteCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = b.workDir

	// Set environment variables
	for k, v := range b.envVars {
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", k, v))
	}

	return cmd.CombinedOutput()
}

// SetEnvVar sets an environment variable
func (b *BaseIntegration) SetEnvVar(key, value string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.envVars[key] = value
}

// GetWorkDir returns the working directory
func (b *BaseIntegration) GetWorkDir() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.workDir
}

// SetWorkDir sets the working directory
func (b *BaseIntegration) SetWorkDir(dir string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.workDir = dir
}

// GetConfig returns the configuration
func (b *BaseIntegration) GetConfig() interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

// BaseConfig provides common configuration options
type BaseConfig struct {
	WorkDir   string
	APIKey    string
	Model     string
	AutoStart bool
	LogLevel  string
	Timeout   int
}

func extractWorkDir(config interface{}) string {
	if config == nil {
		return ""
	}

	if cfg, ok := config.(*BaseConfig); ok && cfg != nil && cfg.WorkDir != "" {
		return cfg.WorkDir
	}

	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
		if v.Kind() == reflect.Struct {
			if f := v.FieldByName("BaseConfig"); f.IsValid() {
				if bc, ok := f.Interface().(BaseConfig); ok && bc.WorkDir != "" {
					return bc.WorkDir
				}
			}
		}
	}

	return ""
}

func uniqueSuffix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func isTestBinary() bool {
	exe, _ := os.Executable()
	return len(exe) > 5 && exe[len(exe)-5:] == ".test"
}
