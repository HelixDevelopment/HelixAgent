// Package nanocoder provides tests for Nanocoder agent integration
package nanocoder

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

// writeFakeNanocoder creates an executable shell script that echoes an
// unforgeable marker plus the args it was invoked with, then returns its
// absolute path. Skips on non-POSIX hosts. Drives the REAL exec path
// deterministically (no real nanocoder binary, no credentials).
func writeFakeNanocoder(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; " +
			"not portable to Windows. The real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-nanocoder")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake nanocoder: %v", err)
	}
	return bin
}

func TestNewNanocoder(t *testing.T) {
	t.Parallel()
	n := New()
	require.NotNil(t, n)

	info := n.Info()
	assert.Equal(t, agents.TypeNanocoder, info.Type)
	assert.Equal(t, "Nanocoder", info.Name)
	assert.Equal(t, "Nanocoder", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestNanocoderInitialize(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model: "custom-nano",
	}

	err := n.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "custom-nano", n.config.Model)
}

func TestNanocoderInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()

	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "nano", n.config.Model) // Default value
}

func TestNanocoderStartStop(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()

	err := n.Initialize(ctx, nil)
	require.NoError(t, err)

	err = n.Start(ctx)
	require.NoError(t, err)
	assert.True(t, n.IsStarted())

	err = n.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, n.IsStarted())
}

// TestNanocoderExecute reconciled (§11.4.120): generate now exec's the real
// nanocoder CLI, so the success path is driven through an injected fake binary.
// Not t.Parallel() — uses t.Setenv.
func TestNanocoderExecute(t *testing.T) {
	bin := writeFakeNanocoder(t, "EXEC_OK")
	t.Setenv("NANOCODER_BIN", bin)

	n := New()
	ctx := context.Background()

	err := n.Initialize(ctx, nil)
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
			result, err := n.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestNanocoderCapabilities(t *testing.T) {
	t.Parallel()
	n := New()
	info := n.Info()

	expectedCaps := []string{"minimal", "fast", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

// TestNanocoderIsAvailable reconciled (§11.4.120): availability now reflects
// whether the real nanocoder binary is resolvable (honest, never hardcoded).
func TestNanocoderIsAvailable(t *testing.T) {
	bin := writeFakeNanocoder(t, "AVAIL_OK")
	t.Setenv("NANOCODER_BIN", bin)
	n := New()
	assert.True(t, n.IsAvailable())

	t.Setenv("NANOCODER_BIN", filepath.Join(t.TempDir(), "does-not-exist"))
	assert.False(t, n.IsAvailable())
}

// TestNanocoderGenerateResult reconciled (§11.4.120): the "code" field MUST be
// the fake binary's REAL stdout (proves exec ran), never a templated literal.
func TestNanocoderGenerateResult(t *testing.T) {
	const marker = "NANO_RAN_7c1e"
	bin := writeFakeNanocoder(t, marker)
	t.Setenv("NANOCODER_BIN", bin)

	n := New()
	ctx := context.Background()

	err := n.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := n.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a struct",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create a struct", resultMap["prompt"])

	code, _ := resultMap["code"].(string)
	assert.Contains(t, code, marker, "code must be real nanocoder process output")
	assert.Contains(t, code, "Create a struct", "prompt must be forwarded to the binary")
	assert.NotContains(t, code, "// Nanocoder", "must not be the old templated literal")
}

// TestD17_Nanocoder_AbsentBinaryIsHonestError proves that with NO nanocoder
// binary available, generate returns an honest error — never a fabricated
// template (BLUFF-001 anti-bluff guarantee).
func TestD17_Nanocoder_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("NANOCODER_BIN", filepath.Join(t.TempDir(), "does-not-exist-nanocoder"))

	n := New()
	ctx := context.Background()
	if _, err := n.generate(ctx, map[string]interface{}{"prompt": "x"}); err == nil {
		t.Fatal("D17 BLUFF: generate returned success with NO nanocoder binary — " +
			"must return an honest error, never a fabricated template.")
	}
}

// TestD17_Nanocoder_GenerateIsStubBluff is the §11.4.115 RED-polarity
// reproduction, runnable ONLY under PIN_STUB_BLUFF=1. The historical bluff
// returned "// Nanocoder\n<prompt>" with no exec; on the fixed artifact the
// result is the fake binary's real output (the literal is ABSENT). Standing
// GREEN guard is TestNanocoderGenerateResult.
func TestD17_Nanocoder_GenerateIsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1.")
	}
	const marker = "NANO_RED_2b8f"
	bin := writeFakeNanocoder(t, marker)
	t.Setenv("NANOCODER_BIN", bin)

	n := New()
	ctx := context.Background()
	res, err := n.generate(ctx, map[string]interface{}{"prompt": "return 42"})
	require.NoError(t, err)
	m, _ := res.(map[string]interface{})
	code, _ := m["code"].(string)
	if strings.HasPrefix(code, "// Nanocoder") {
		t.Fatalf("D17 BLUFF PINNED: nanocoder.generate returned the templated literal %q "+
			"without exec-ing the real binary (BLUFF-001).", code)
	}
}
