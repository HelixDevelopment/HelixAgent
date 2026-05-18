// Package aider provides tests for Aider agent integration
package aider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

func TestNewAider(t *testing.T) {
	t.Parallel()
	a := New()

	if a == nil {
		t.Fatal("New() = nil")
	}

	info := a.Info()
	if info.Type != agents.TypeAider {
		t.Errorf("Info().Type = %q, want %q", info.Type, agents.TypeAider)
	}

	if info.Name != "Aider" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "Aider")
	}
}

func TestAiderInitialize(t *testing.T) {
	t.Parallel()
	a := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
		EditorModel:   "gpt-4",
		ArchitectMode: false,
	}

	err := a.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if a.config.EditorModel != "gpt-4" {
		t.Errorf("config.EditorModel = %q, want %q", a.config.EditorModel, "gpt-4")
	}
}

func TestAiderStartStop(t *testing.T) {
	t.Parallel()
	a := New()
	ctx := context.Background()

	err := a.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err = a.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !a.IsStarted() {
		t.Error("IsStarted() = false after Start()")
	}

	err = a.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if a.IsStarted() {
		t.Error("IsStarted() = true after Stop()")
	}
}

func TestAiderExecute(t *testing.T) {
	t.Parallel()
	a := New()
	ctx := context.Background()

	err := a.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Round 109 §11.9 fix: previously asserted wantErr=true for chat/commit
	// solely because they shell out to `aider` which "should be" absent.
	// When `aider` IS in PATH (operator's dev machine), Execute returns nil
	// → test failed despite production behaving correctly. The chat/commit
	// happy paths require real LLM credentials and are exercised in the
	// integration suite; here we only assert the dispatch shape (known vs
	// unknown command) which is what this unit test can actually certify.
	aiderAbsent := false
	if _, lookErr := exec.LookPath("aider"); lookErr != nil {
		aiderAbsent = true
	}

	tests := []struct {
		name             string
		command          string
		params           map[string]interface{}
		wantErr          bool
		requiresNoBinary bool // true → only assert when aider is NOT in PATH
	}{
		{
			name:    "chat command",
			command: "chat",
			params: map[string]interface{}{
				"message": "Hello",
			},
			wantErr:          true,
			requiresNoBinary: true,
		},
		{
			name:    "architect command",
			command: "architect",
			params: map[string]interface{}{
				"prompt": "Design a system",
			},
			// Aider has no "architect" subcommand in Execute switch →
			// hits default → returns "unknown command" error
			// regardless of binary presence.
			wantErr: true,
		},
		{
			name:             "commit command",
			command:          "commit",
			params:           map[string]interface{}{},
			wantErr:          true,
			requiresNoBinary: true,
		},
		{
			name:    "lint command",
			command: "lint",
			params:  map[string]interface{}{},
			// Not in Execute switch → "unknown command" → deterministic err
			wantErr: true,
		},
		{
			name:    "test command",
			command: "test",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.requiresNoBinary && !aiderAbsent {
				// SKIP-OK: round-109 §11.9 — happy path requires real
				// LLM creds; covered in integration suite. Unit asserting
				// "process exec failed" is a bluff when binary is present.
				t.Skipf("aider binary present in PATH; skipping wantErr=true assertion (SKIP-OK: round-109 §11.9)")
			}
			result, err := a.Execute(ctx, tt.command, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Execute() result = nil, want non-nil")
			}
		})
	}
}

func TestAiderCapabilities(t *testing.T) {
	t.Parallel()
	a := New()
	info := a.Info()

	// Just check that we have some capabilities
	if len(info.Capabilities) == 0 {
		t.Error("No capabilities found")
	}

	// Check for some expected capabilities
	expectedCapabilities := []string{
		"repo_map",
		"multi_file_editing",
		"git_integration",
	}

	for _, cap := range expectedCapabilities {
		found := false
		for _, has := range info.Capabilities {
			if has == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

func TestAiderHealth(t *testing.T) {
	t.Parallel()
	a := New()
	ctx := context.Background()

	// Before start, health should fail
	if err := a.Health(ctx); err == nil {
		t.Error("Health() before Start = nil, want error")
	}

	_ = a.Initialize(ctx, nil)
	_ = a.Start(ctx)

	// After start, health should pass
	if err := a.Health(ctx); err != nil {
		t.Errorf("Health() after Start error = %v", err)
	}
}

func TestAiderRepoMap(t *testing.T) {
	t.Parallel()
	a := New()
	ctx := context.Background()

	tempDir := t.TempDir()

	// Create a test Go file
	testFile := filepath.Join(tempDir, "test.go")
	content := `package main

func Hello() string {
	return "Hello"
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	if err := a.Initialize(ctx, config); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Test repo map command (may fail if aider not in PATH)
	result, err := a.Execute(ctx, "repo_map", map[string]interface{}{
		"path": tempDir,
	})

	// Just verify the command runs without panic
	// Result depends on whether aider is installed
	_ = result
	_ = err
}

func TestAiderConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				EditorModel:   "gpt-4",
				ArchitectMode: true,
			},
			wantErr: false,
		},
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "empty model uses default",
			config: &Config{
				EditorModel: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := New()
			ctx := context.Background()
			err := a.Initialize(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkAiderExecute(b *testing.B) {
	a := New()
	ctx := context.Background()
	_ = a.Initialize(ctx, nil)
	_ = a.Start(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = a.Execute(ctx, "chat", map[string]interface{}{
			"message": "test",
		})
	}
}
