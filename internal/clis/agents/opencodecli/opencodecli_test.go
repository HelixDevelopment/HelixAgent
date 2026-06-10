// Package opencodecli provides tests for Opencode CLI agent integration
package opencodecli

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

// writeFakeOpencode creates an executable shell script that echoes an
// unforgeable marker plus the args it was invoked with (`opencode run <prompt>`),
// then returns its absolute path. Drives the REAL exec path deterministically.
func writeFakeOpencode(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; " +
			"not portable to Windows. The real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-opencode")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}
	return bin
}

func TestNewOpencodeCLI(t *testing.T) {
	t.Parallel()
	o := New()
	require.NotNil(t, o)

	info := o.Info()
	assert.Equal(t, agents.TypeOpencodeCLI, info.Type)
	assert.Equal(t, "Opencode CLI", info.Name)
	assert.Equal(t, "Opencode", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestOpencodeCLIInitialize(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model: "gpt-4",
	}

	err := o.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", o.config.Model)
}

func TestOpencodeCLIInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "default", o.config.Model) // Default value
}

func TestOpencodeCLIStartStop(t *testing.T) {
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

// TestOpencodeCLIExecute reconciled (§11.4.120): chat/generate now exec the real
// opencode CLI, so the success path is driven through an injected fake binary.
func TestOpencodeCLIExecute(t *testing.T) {
	bin := writeFakeOpencode(t, "EXEC_OK")
	t.Setenv("OPENCODE_BIN", bin)

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
			name:    "chat command",
			command: "chat",
			params:  map[string]interface{}{"message": "Hello"},
			wantErr: false,
		},
		{
			name:    "chat without message fails",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "generate command",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create code"},
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

func TestOpencodeCLICapabilities(t *testing.T) {
	t.Parallel()
	o := New()
	info := o.Info()

	expectedCaps := []string{"open_source", "code_generation", "chat"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

// TestOpencodeCLIIsAvailable reconciled (§11.4.120): availability now reflects
// whether the real opencode binary is resolvable (honest, never hardcoded true).
func TestOpencodeCLIIsAvailable(t *testing.T) {
	bin := writeFakeOpencode(t, "AVAIL_OK")
	t.Setenv("OPENCODE_BIN", bin)
	o := New()
	assert.True(t, o.IsAvailable())

	t.Setenv("OPENCODE_BIN", filepath.Join(t.TempDir(), "does-not-exist"))
	assert.False(t, o.IsAvailable())
}

// TestOpencodeCLIChatResult reconciled (§11.4.120): the "response" field MUST be
// the fake binary's REAL stdout (proves exec ran), never the old "Opencode: <msg>"
// echo template.
func TestOpencodeCLIChatResult(t *testing.T) {
	const marker = "OPENCODE_CHAT_5b2a"
	bin := writeFakeOpencode(t, marker)
	t.Setenv("OPENCODE_BIN", bin)

	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := o.Execute(ctx, "chat", map[string]interface{}{
		"message": "How are you?",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "How are you?", resultMap["message"])

	response, _ := resultMap["response"].(string)
	assert.Contains(t, response, marker, "response must be real opencode process output")
	assert.Contains(t, response, "run", "must invoke `opencode run`")
	assert.NotContains(t, response, "Opencode: How are you?", "must not be the old echo template")
}

// TestOpencodeCLIGenerateResult reconciled (§11.4.120): the "code" field MUST be
// the fake binary's REAL stdout, never the old "// Opencode\n// <prompt>" literal.
func TestOpencodeCLIGenerateResult(t *testing.T) {
	const marker = "OPENCODE_GEN_6c3b"
	bin := writeFakeOpencode(t, marker)
	t.Setenv("OPENCODE_BIN", bin)

	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a handler",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create a handler", resultMap["prompt"])

	code, _ := resultMap["code"].(string)
	assert.Contains(t, code, marker, "code must be real opencode process output")
	assert.Contains(t, code, "Create a handler", "prompt must be forwarded to the binary")
	assert.NotContains(t, code, "// Opencode", "must not be the old templated literal")
}

// TestD17_Opencode_AbsentBinaryIsHonestError proves that with NO opencode
// binary available, chat/generate return an honest error — never a fabricated
// template.
func TestD17_Opencode_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("OPENCODE_BIN", filepath.Join(t.TempDir(), "does-not-exist-opencode"))

	o := New()
	ctx := context.Background()
	if _, err := o.generate(ctx, map[string]interface{}{"prompt": "x"}); err == nil {
		t.Fatal("D17 BLUFF: generate returned success with NO opencode binary — " +
			"must return an honest error, never a fabricated template.")
	}
	if _, err := o.chat(ctx, map[string]interface{}{"message": "x"}); err == nil {
		t.Fatal("D17 BLUFF: chat returned success with NO opencode binary — " +
			"must return an honest error, never a fabricated echo.")
	}
}

// TestD17_Opencode_GenerateIsStubBluff is the §11.4.115 RED-polarity
// reproduction, runnable ONLY under PIN_STUB_BLUFF=1.
func TestD17_Opencode_GenerateIsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1.")
	}
	const marker = "OPENCODE_RED_7d4c"
	bin := writeFakeOpencode(t, marker)
	t.Setenv("OPENCODE_BIN", bin)

	o := New()
	ctx := context.Background()
	res, err := o.generate(ctx, map[string]interface{}{"prompt": "x"})
	require.NoError(t, err)
	m, _ := res.(map[string]interface{})
	code, _ := m["code"].(string)
	if strings.HasPrefix(code, "// Opencode") {
		t.Fatalf("D17 BLUFF PINNED: opencodecli.generate returned the templated literal %q "+
			"without exec-ing the real binary (BLUFF-001).", code)
	}
}
