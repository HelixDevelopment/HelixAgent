// Package sandbox provides secure command execution with container-based isolation
// Inspired by Codex's sandboxed execution system
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Runtime defines the sandbox runtime type
type Runtime string

const (
	RuntimeDocker   Runtime = "docker"
	RuntimePodman   Runtime = "podman"
	RuntimeSeatbelt Runtime = "seatbelt" // macOS only
	RuntimeNone     Runtime = "none"
)

// Config configures the sandbox
type Config struct {
	Runtime        Runtime
	EnableNetwork  bool
	WorkingDir     string
	MountReadOnly  []string
	MountReadWrite []string
	EnvVars        map[string]string
	MemoryLimit    string // e.g., "512m"
	CPULimit       string // e.g., "1.0"
	Timeout        time.Duration
	UserID         string // e.g., "1000:1000"
}

// DefaultConfig returns default sandbox configuration
func DefaultConfig() Config {
	return Config{
		Runtime:        RuntimeDocker,
		EnableNetwork:  false,
		WorkingDir:     "/workspace",
		MountReadOnly:  []string{},
		MountReadWrite: []string{},
		EnvVars:        make(map[string]string),
		MemoryLimit:    "512m",
		CPULimit:       "1.0",
		Timeout:        60 * time.Second,
	}
}

// Sandbox provides secure command execution
type Sandbox struct {
	config Config
	image  string
}

// NewSandbox creates a new sandbox instance
func NewSandbox(config Config, image string) (*Sandbox, error) {
	if config.Runtime == "" {
		config.Runtime = RuntimeDocker
	}

	// Detect available runtime if not specified
	if config.Runtime == RuntimeDocker || config.Runtime == RuntimePodman {
		if !isRuntimeAvailable(config.Runtime) {
			// Try fallback
			if config.Runtime == RuntimeDocker && isRuntimeAvailable(RuntimePodman) {
				config.Runtime = RuntimePodman
			} else if config.Runtime == RuntimePodman && isRuntimeAvailable(RuntimeDocker) {
				config.Runtime = RuntimeDocker
			} else {
				return nil, fmt.Errorf("no container runtime available (tried docker, podman)")
			}
		}
	}

	return &Sandbox{
		config: config,
		image:  image,
	}, nil
}

// isRuntimeAvailable checks if a runtime is available
func isRuntimeAvailable(runtime Runtime) bool {
	cmd := exec.Command(string(runtime), "version")
	err := cmd.Run()
	return err == nil
}

// Result represents the result of a sandboxed command execution
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Execute runs a command in the sandbox
func (s *Sandbox) Execute(ctx context.Context, command []string) (*Result, error) {
	// Apply timeout
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	switch s.config.Runtime {
	case RuntimeDocker, RuntimePodman:
		return s.executeContainer(ctx, command)
	case RuntimeSeatbelt:
		return s.executeSeatbelt(ctx, command)
	case RuntimeNone:
		return s.executeDirect(ctx, command)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", s.config.Runtime)
	}
}

// executeContainer runs command in container (Docker/Podman)
func (s *Sandbox) executeContainer(ctx context.Context, command []string) (*Result, error) {
	startTime := time.Now()
	args := []string{"run", "--rm", "-i"}

	// Network
	if !s.config.EnableNetwork {
		args = append(args, "--network", "none")
	}

	// Resource limits
	if s.config.MemoryLimit != "" {
		args = append(args, "--memory", s.config.MemoryLimit)
	}
	if s.config.CPULimit != "" {
		args = append(args, "--cpus", s.config.CPULimit)
	}

	// User
	if s.config.UserID != "" {
		args = append(args, "--user", s.config.UserID)
	}

	// Working directory
	if s.config.WorkingDir != "" {
		args = append(args, "--workdir", s.config.WorkingDir)
	}

	// Environment variables
	for key, value := range s.config.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Mounts (read-only)
	for _, mount := range s.config.MountReadOnly {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", mount, mount))
	}

	// Mounts (read-write)
	for _, mount := range s.config.MountReadWrite {
		args = append(args, "-v", fmt.Sprintf("%s:%s", mount, mount))
	}

	// Image and command
	args = append(args, s.image)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, string(s.config.Runtime), args...)

	// Capture output
	stdout, err := cmd.Output()
	duration := time.Since(startTime)

	result := &Result{
		Stdout:   string(stdout),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			// Check if context was cancelled due to timeout
			if ctx.Err() != nil {
				result.ExitCode = -1
				result.Stderr = "execution timeout"
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr = "execution timeout"
		} else if ctx.Err() != nil {
			result.ExitCode = -1
			result.Stderr = "execution timeout"
		} else {
			return nil, fmt.Errorf("failed to execute: %w", err)
		}
	}

	return result, nil
}

// executeSeatbelt runs command with macOS Seatbelt (placeholder)
func (s *Sandbox) executeSeatbelt(ctx context.Context, command []string) (*Result, error) {
	// Seatbelt is macOS-specific sandbox
	// This would use /usr/bin/sandbox-exec
	return nil, fmt.Errorf("seatbolt execution not yet implemented")
}

// executeDirect runs command directly (no sandbox)
func (s *Sandbox) executeDirect(ctx context.Context, command []string) (*Result, error) {
	startTime := time.Now()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = s.config.WorkingDir

	// Set environment
	for key, value := range s.config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	stdout, err := cmd.Output()
	duration := time.Since(startTime)

	result := &Result{
		Stdout:   string(stdout),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			// Check if context was cancelled due to timeout
			if ctx.Err() != nil {
				result.ExitCode = -1
				result.Stderr = "execution timeout"
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr = "execution timeout"
		} else if ctx.Err() != nil {
			result.ExitCode = -1
			result.Stderr = "execution timeout"
		} else {
			return nil, fmt.Errorf("failed to execute: %w", err)
		}
	}

	return result, nil
}

// ExecuteWithStreams runs command with streaming I/O
func (s *Sandbox) ExecuteWithStreams(ctx context.Context, command []string, stdin io.Reader, stdout, stderr io.Writer) (*Result, error) {
	startTime := time.Now()

	// Apply timeout
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	args := []string{"run", "--rm", "-i"}

	// Network
	if !s.config.EnableNetwork {
		args = append(args, "--network", "none")
	}

	// Resource limits
	if s.config.MemoryLimit != "" {
		args = append(args, "--memory", s.config.MemoryLimit)
	}
	if s.config.CPULimit != "" {
		args = append(args, "--cpus", s.config.CPULimit)
	}

	// Working directory
	if s.config.WorkingDir != "" {
		args = append(args, "--workdir", s.config.WorkingDir)
	}

	// Environment variables
	for key, value := range s.config.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Mounts
	for _, mount := range s.config.MountReadWrite {
		args = append(args, "-v", fmt.Sprintf("%s:%s", mount, mount))
	}

	// Image and command
	args = append(args, s.image)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, string(s.config.Runtime), args...)

	// Set up streaming
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	err := cmd.Run()
	duration := time.Since(startTime)

	result := &Result{
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
		} else {
			return nil, fmt.Errorf("failed to execute: %w", err)
		}
	}

	return result, nil
}

// AvailableRuntimes returns list of available sandbox runtimes
func AvailableRuntimes() []Runtime {
	var available []Runtime

	if isRuntimeAvailable(RuntimeDocker) {
		available = append(available, RuntimeDocker)
	}
	if isRuntimeAvailable(RuntimePodman) {
		available = append(available, RuntimePodman)
	}
	// Check for macOS seatbelt
	if isSeatbeltAvailable() {
		available = append(available, RuntimeSeatbelt)
	}

	// Always available (fallback)
	available = append(available, RuntimeNone)

	return available
}

// isSeatbeltAvailable checks if macOS Seatbelt is available
func isSeatbeltAvailable() bool {
	cmd := exec.Command("which", "sandbox-exec")
	err := cmd.Run()
	return err == nil
}
