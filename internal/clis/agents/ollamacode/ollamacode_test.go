// Package ollamacode provides tests for Ollama Code agent integration
package ollamacode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeOllama creates an executable shell script that echoes an unforgeable
// marker plus the args it was invoked with (`ollama run <model> <prompt>`), then
// returns its absolute path. Drives the REAL exec path deterministically.
func writeFakeOllama(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; " +
			"not portable to Windows. The real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-ollama")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ollama: %v", err)
	}
	return bin
}

func TestNewOllamaCode(t *testing.T) {
	t.Parallel()
	o := New()
	require.NotNil(t, o)

	info := o.Info()
	assert.Equal(t, agents.TypeOllamaCode, info.Type)
	assert.Equal(t, "Ollama Code", info.Name)
	assert.Equal(t, "Ollama", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestOllamaCodeInitialize(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Endpoint: "http://custom:11434",
		Model:    "llama2",
	}

	err := o.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "http://custom:11434", o.config.Endpoint)
	assert.Equal(t, "llama2", o.config.Model)
}

func TestOllamaCodeInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", o.config.Endpoint) // Default value
	assert.Equal(t, "codellama", o.config.Model)                 // Default value
}

func TestOllamaCodeStartStop(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	err = o.Start(ctx)
	require.NoError(t, err)
	assert.True(t, o.IsStarted())

	err = o.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, o.IsStarted())
}

// TestOllamaCodeExecute reconciled (§11.4.120): generate now exec's the real
// ollama CLI, so the success path is driven through an injected fake binary.
func TestOllamaCodeExecute(t *testing.T) {
	bin := writeFakeOllama(t, "EXEC_OK")
	t.Setenv("OLLAMA_BIN", bin)

	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "generate command",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create a function"},
			wantErr: false,
		},
		{
			name:    "generate without prompt fails",
			command: "generate",
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
			result, err := o.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOllamaCodeCapabilities(t *testing.T) {
	t.Parallel()
	o := New()
	info := o.Info()

	expectedCaps := []string{"local_llm", "privacy", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

// TestOllamaCodeIsAvailable reconciled (§11.4.120): availability now reflects
// whether the real ollama binary is resolvable (honest, never hardcoded /
// endpoint-string-only).
func TestOllamaCodeIsAvailable(t *testing.T) {
	bin := writeFakeOllama(t, "AVAIL_OK")
	t.Setenv("OLLAMA_BIN", bin)
	o := New()
	assert.True(t, o.IsAvailable())

	t.Setenv("OLLAMA_BIN", filepath.Join(t.TempDir(), "does-not-exist"))
	assert.False(t, o.IsAvailable())
}

// TestOllamaCodeGenerateResult reconciled (§11.4.120): the "code" field MUST be
// the fake binary's REAL stdout (proves exec ran), never a templated literal.
func TestOllamaCodeGenerateResult(t *testing.T) {
	const marker = "OLLAMA_RAN_9d3a"
	bin := writeFakeOllama(t, marker)
	t.Setenv("OLLAMA_BIN", bin)

	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Build a struct",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Build a struct", resultMap["prompt"])
	assert.Equal(t, "codellama", resultMap["model"])

	code, _ := resultMap["code"].(string)
	assert.Contains(t, code, marker, "code must be real ollama process output")
	assert.Contains(t, code, "Build a struct", "prompt must be forwarded to the binary")
	assert.Contains(t, code, "run codellama", "must invoke `ollama run <model>`")
	assert.NotContains(t, code, "// Ollama local", "must not be the old templated literal")
}

// TestD17_Ollama_AbsentBinaryIsHonestError proves that with NO ollama binary
// available, generate returns an honest error — never a fabricated template.
func TestD17_Ollama_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("OLLAMA_BIN", filepath.Join(t.TempDir(), "does-not-exist-ollama"))

	o := New()
	ctx := context.Background()
	if _, err := o.generate(ctx, map[string]interface{}{"prompt": "x"}); err == nil {
		t.Fatal("D17 BLUFF: generate returned success with NO ollama binary — " +
			"must return an honest error, never a fabricated template.")
	}
}

// TestD17_Ollama_GenerateIsStubBluff is the §11.4.115 RED-polarity
// reproduction, runnable ONLY under PIN_STUB_BLUFF=1.
func TestD17_Ollama_GenerateIsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1.")
	}
	const marker = "OLLAMA_RED_4e7c"
	bin := writeFakeOllama(t, marker)
	t.Setenv("OLLAMA_BIN", bin)

	o := New()
	ctx := context.Background()
	res, err := o.generate(ctx, map[string]interface{}{"prompt": "return 42"})
	require.NoError(t, err)
	m, _ := res.(map[string]interface{})
	code, _ := m["code"].(string)
	if strings.HasPrefix(code, "// Ollama local") {
		t.Fatalf("D17 BLUFF PINNED: ollamacode.generate returned the templated literal %q "+
			"without exec-ing the real binary (BLUFF-001).", code)
	}
}
