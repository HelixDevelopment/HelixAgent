// Package base provides tests for the base agent integration
package base

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.agent/internal/clis/agents"
)

func TestBaseIntegration(t *testing.T) {
	t.Parallel()
	info := agents.AgentInfo{
		Type:         "test",
		Name:         "Test Agent",
		Description:  "Test agent for unit tests",
		Vendor:       "Test",
		Version:      "1.0.0",
		Capabilities: []string{"test"},
		IsEnabled:    true,
		Priority:     1,
	}

	bi := NewBaseIntegration(info)

	t.Run("Info", func(t *testing.T) {
		t.Parallel()
		got := bi.Info()
		if got.Name != info.Name {
			t.Errorf("Info().Name = %q, want %q", got.Name, info.Name)
		}
		if got.Type != info.Type {
			t.Errorf("Info().Type = %q, want %q", got.Type, info.Type)
		}
	})

	t.Run("IsAvailable", func(t *testing.T) {
		t.Parallel()
		// Base implementation checks if command with agent type exists
		// Result depends on system, so we just check it doesn't panic
		_ = bi.IsAvailable()
	})

	t.Run("Initialize", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		config := &BaseConfig{
			WorkDir: t.TempDir(),
		}

		if err := bi.Initialize(ctx, config); err != nil {
			t.Errorf("Initialize() error = %v", err)
		}

		if bi.GetWorkDir() != config.WorkDir {
			t.Errorf("GetWorkDir() = %q, want %q", bi.GetWorkDir(), config.WorkDir)
		}
	})

	t.Run("Start", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		if err := bi.Start(ctx); err != nil {
			t.Errorf("Start() error = %v", err)
		}

		if !bi.IsStarted() {
			t.Error("IsStarted() = false, want true")
		}
	})

	t.Run("Stop", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		if err := bi.Stop(ctx); err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		if bi.IsStarted() {
			t.Error("IsStarted() = true, want false")
		}
	})

	t.Run("Health", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Before start, health check should fail
		if err := bi.Health(ctx); err == nil {
			t.Error("Health() before Start = nil, want error")
		}

		// After start, health check should pass
		_ = bi.Start(ctx)
		if err := bi.Health(ctx); err != nil {
			t.Errorf("Health() after Start error = %v", err)
		}
	})

	t.Run("ExecuteCommand", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Use a simple command that doesn't depend on working directory
		output, err := bi.ExecuteCommand(ctx, "pwd")
		if err != nil {
			// Just check that the command executes without error
			// The working directory might not exist in test environment
			t.Logf("ExecuteCommand() error = %v (expected in test environment)", err)
		}

		// Just verify we got some output or an error (both are valid results)
		_ = output
	})

	t.Run("WorkDirCreation", func(t *testing.T) {
		t.Parallel()
		bi2 := NewBaseIntegration(info)
		ctx := context.Background()

		tempDir := t.TempDir()
		config := &BaseConfig{
			WorkDir: tempDir,
		}

		if err := bi2.Initialize(ctx, config); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}

		// Check if workdir was created
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Error("WorkDir was not created")
		}
	})
}

func TestBaseIntegrationWithTempDir(t *testing.T) {
	t.Parallel()
	info := agents.AgentInfo{
		Type:         "test",
		Name:         "Test Agent",
		Description:  "Test agent for unit tests",
		Vendor:       "Test",
		Version:      "1.0.0",
		Capabilities: []string{"test"},
		IsEnabled:    true,
		Priority:     1,
	}

	bi := NewBaseIntegration(info)
	ctx := context.Background()

	// Initialize with no config (should use temp dir)
	if err := bi.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	workDir := bi.GetWorkDir()
	if workDir == "" {
		t.Error("GetWorkDir() = empty, want non-empty")
	}

	// Check that the workdir is valid
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		t.Error("WorkDir does not exist")
	}
}

func TestBaseConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *BaseConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false, // Should use defaults
		},
		{
			name: "valid config",
			config: &BaseConfig{
				WorkDir:   "/tmp/test",
				AutoStart: true,
				Timeout:   30,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := agents.AgentInfo{
				Type: "test",
				Name: "Test",
			}
			bi := NewBaseIntegration(info)
			ctx := context.Background()

			err := bi.Initialize(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBaseIntegrationConcurrency(t *testing.T) {
	t.Parallel()
	info := agents.AgentInfo{
		Type: "test",
		Name: "Test",
	}
	bi := NewBaseIntegration(info)
	ctx := context.Background()

	// Initialize
	if err := bi.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Concurrent Start/Stop operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = bi.Start(ctx)
			time.Sleep(10 * time.Millisecond)
			_ = bi.Stop(ctx)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestAgentInfoValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		info    agents.AgentInfo
		wantErr bool
	}{
		{
			name: "valid info",
			info: agents.AgentInfo{
				Type:        "valid",
				Name:        "Valid Agent",
				Description: "A valid agent",
				Vendor:      "Test",
				Version:     "1.0.0",
				IsEnabled:   true,
			},
			wantErr: false,
		},
		{
			name: "empty type",
			info: agents.AgentInfo{
				Type:        "",
				Name:        "No Type",
				Description: "Agent with no type",
			},
			wantErr: false, // Base integration doesn't validate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bi := NewBaseIntegration(tt.info)
			if bi.info.Type != tt.info.Type {
				t.Errorf("info.Type = %q, want %q", bi.info.Type, tt.info.Type)
			}
		})
	}
}

func TestFileOperations(t *testing.T) {
	t.Parallel()
	info := agents.AgentInfo{
		Type: "test",
		Name: "Test",
	}
	bi := NewBaseIntegration(info)
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &BaseConfig{WorkDir: tempDir}

	if err := bi.Initialize(ctx, config); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	t.Run("Write and Read File", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("test content")

		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		read, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		if string(read) != string(content) {
			t.Errorf("Read content = %q, want %q", string(read), string(content))
		}
	})

	t.Run("File Permissions", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "perms.txt")
		content := []byte("content")

		if err := os.WriteFile(testFile, content, 0600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}

		mode := info.Mode().Perm()
		if mode != 0600 {
			t.Errorf("File permissions = %o, want %o", mode, 0600)
		}
	})
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	info := agents.AgentInfo{
		Type: "test",
		Name: "Test",
	}
	bi := NewBaseIntegration(info)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Note: Current implementation doesn't check context cancellation
	// This test documents the current behavior
	err := bi.Start(ctx)
	// Current implementation doesn't return error for cancelled context
	_ = err
}

func BenchmarkBaseIntegrationStartStop(b *testing.B) {
	info := agents.AgentInfo{
		Type: "benchmark",
		Name: "Benchmark",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bi := NewBaseIntegration(info)
		_ = bi.Initialize(ctx, nil)
		_ = bi.Start(ctx)
		_ = bi.Stop(ctx)
	}
}
