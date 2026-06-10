// Package qwencode provides tests for Qwen Code agent integration
package qwencode

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// osWriteFakeQwen writes a deterministic fake qwen shell script (no *testing.T;
// usable from benchmarks). Returns an error on non-POSIX hosts or write failure.
func osWriteFakeQwen(path string) error {
	if runtime.GOOS == "windows" {
		return os.ErrInvalid
	}
	return os.WriteFile(path, []byte("#!/bin/sh\nprintf '{\"response\":\"BENCH_OK\"}'\n"), 0o755)
}

func TestNewQwenCode(t *testing.T) {
	t.Parallel()
	q := New()

	if q == nil {
		t.Fatal("New() = nil")
	}

	info := q.Info()
	if info.Type != agents.TypeQwenCode {
		t.Errorf("Info().Type = %q, want %q", info.Type, agents.TypeQwenCode)
	}

	if info.Name != "Qwen Code" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "Qwen Code")
	}

	if info.Vendor != "Alibaba" {
		t.Errorf("Info().Vendor = %q, want %q", info.Vendor, "Alibaba")
	}
}

func TestQwenCodeInitialize(t *testing.T) {
	t.Parallel()
	q := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
		APIKey: "test-api-key",
		Model:  "qwen-coder-plus-latest",
	}

	err := q.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if q.config.APIKey != "test-api-key" {
		t.Errorf("config.APIKey = %q, want %q", q.config.APIKey, "test-api-key")
	}

	if q.config.Model != "qwen-coder-plus-latest" {
		t.Errorf("config.Model = %q, want %q", q.config.Model, "qwen-coder-plus-latest")
	}
}

func TestQwenCodeStartStop(t *testing.T) {
	t.Parallel()
	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err = q.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !q.IsStarted() {
		t.Error("IsStarted() = false after Start()")
	}

	err = q.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if q.IsStarted() {
		t.Error("IsStarted() = true after Stop()")
	}
}

func TestQwenCodeExecute(t *testing.T) {
	// Reconciled (§11.4.120): generate/complete/chat now exec the real qwen CLI,
	// so the happy-path commands are driven through an injected fake qwen binary
	// (deterministic, no credentials). Not t.Parallel() — uses t.Setenv.
	bin := writeFakeQwen(t, "EXEC_OK")
	t.Setenv("QWEN_BIN", bin)

	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "complete command",
			command: "complete",
			params: map[string]interface{}{
				"prefix": "func main()",
			},
			wantErr: false,
		},
		{
			name:    "generate command",
			command: "generate",
			params: map[string]interface{}{
				"prompt": "Create a hello world function",
			},
			wantErr: false,
		},
		{
			name:    "generate without prompt fails",
			command: "generate",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "chat command",
			command: "chat",
			params: map[string]interface{}{
				"message": "Hello Qwen",
			},
			wantErr: false,
		},
		{
			name:    "chat without message fails",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
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
			result, err := q.Execute(ctx, tt.command, tt.params)
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

func TestQwenCodeGenerate(t *testing.T) {
	// Reconciled (§11.4.120): drives the real-exec path via an injected fake qwen.
	bin := writeFakeQwen(t, "GEN_OK")
	t.Setenv("QWEN_BIN", bin)

	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := q.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a REST API",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["prompt"] != "Create a REST API" {
		t.Errorf("prompt = %v, want %v", resultMap["prompt"], "Create a REST API")
	}

	if resultMap["model"] != "qwen-coder-plus" {
		t.Errorf("model = %v, want %v", resultMap["model"], "qwen-coder-plus")
	}

	// The "code" field MUST be the fake binary's real stdout, not a template.
	code, _ := resultMap["code"].(string)
	if !strings.Contains(code, "GEN_OK") {
		t.Errorf("code = %q, expected real qwen process output containing the fake marker", code)
	}
}

func TestQwenCodeChat(t *testing.T) {
	bin := writeFakeQwen(t, "CHAT_OK")
	t.Setenv("QWEN_BIN", bin)

	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := q.Execute(ctx, "chat", map[string]interface{}{
		"message": "Explain Go interfaces",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["message"] != "Explain Go interfaces" {
		t.Errorf("message = %v, want %v", resultMap["message"], "Explain Go interfaces")
	}

	response, _ := resultMap["response"].(string)
	if !strings.Contains(response, "CHAT_OK") {
		t.Errorf("response = %q, expected real qwen process output containing the fake marker", response)
	}
}

func TestQwenCodeComplete(t *testing.T) {
	bin := writeFakeQwen(t, "COMPLETE_OK")
	t.Setenv("QWEN_BIN", bin)

	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := q.Execute(ctx, "complete", map[string]interface{}{
		"prefix": "func fibonacci(n int) int {",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if resultMap["prefix"] != "func fibonacci(n int) int {" {
		t.Errorf("prefix = %v, want %v", resultMap["prefix"], "func fibonacci(n int) int {")
	}

	completion, _ := resultMap["completion"].(string)
	if !strings.Contains(completion, "COMPLETE_OK") {
		t.Errorf("completion = %q, expected real qwen process output containing the fake marker", completion)
	}
}

func TestQwenCodeStatus(t *testing.T) {
	t.Parallel()
	q := New()
	ctx := context.Background()

	err := q.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := q.Execute(ctx, "status", nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if _, ok := resultMap["available"]; !ok {
		t.Error("status missing 'available' field")
	}

	if _, ok := resultMap["model"]; !ok {
		t.Error("status missing 'model' field")
	}
}

func TestQwenCodeCapabilities(t *testing.T) {
	t.Parallel()
	q := New()
	info := q.Info()

	expectedCapabilities := []string{
		"code_generation",
		"code_completion",
		"chat",
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

func TestQwenCodeHealth(t *testing.T) {
	t.Parallel()
	q := New()
	ctx := context.Background()

	// Before start, health should fail
	if err := q.Health(ctx); err == nil {
		t.Error("Health() before Start = nil, want error")
	}

	_ = q.Initialize(ctx, nil)
	_ = q.Start(ctx)

	// After start, health should pass
	if err := q.Health(ctx); err != nil {
		t.Errorf("Health() after Start error = %v", err)
	}
}

func TestQwenCodeIsAvailable(t *testing.T) {
	t.Parallel()
	q := New()
	ctx := context.Background()

	// Without API key, should not be available
	if q.IsAvailable() {
		t.Error("IsAvailable() = true without API key")
	}

	// With API key, should be available
	config := &Config{
		APIKey: "test-api-key",
	}
	_ = q.Initialize(ctx, config)

	if !q.IsAvailable() {
		t.Error("IsAvailable() = false with API key")
	}
}

func TestQwenCodeConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				APIKey: "test-key",
				Model:  "qwen-coder-plus",
			},
			wantErr: false,
		},
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "empty API key is valid but not available",
			config: &Config{
				APIKey: "",
				Model:  "qwen-coder-plus",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := New()
			ctx := context.Background()
			err := q.Initialize(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkQwenCodeExecute(b *testing.B) {
	// Drive the real-exec path through a deterministic fake qwen binary.
	dir := b.TempDir()
	bin := dir + "/fake-qwen"
	if err := osWriteFakeQwen(bin); err != nil {
		b.Skipf("SKIP-OK: ATM-SP2-D6 — cannot stage fake qwen binary on this host: %v", err)
	}
	b.Setenv("QWEN_BIN", bin)

	q := New()
	ctx := context.Background()
	_ = q.Initialize(ctx, nil)
	_ = q.Start(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = q.Execute(ctx, "chat", map[string]interface{}{
			"message": "test",
		})
	}
}
