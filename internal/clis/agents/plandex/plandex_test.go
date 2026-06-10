// Package plandex provides tests for Plandex agent integration
package plandex

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

// writeFakePlandex creates an executable shell script that echoes an unforgeable
// marker plus the args it was invoked with (`plandex tell <task>`), then returns
// its absolute path. Drives the REAL exec path deterministically.
func writeFakePlandex(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; " +
			"not portable to Windows. The real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-plandex")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake plandex: %v", err)
	}
	return bin
}

func TestNewPlandex(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)

	info := p.Info()
	assert.Equal(t, agents.TypePlandex, info.Type)
	assert.Equal(t, "Plandex", info.Name)
	assert.Equal(t, "Plandex", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestPlandexInitialize(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Mode: "manual",
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "manual", p.config.Mode)
}

func TestPlandexInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "auto", p.config.Mode) // Default value
}

func TestPlandexStartStop(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	err = p.Start(ctx)
	require.NoError(t, err)
	assert.True(t, p.IsStarted())

	err = p.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, p.IsStarted())
}

// TestPlandexExecute reconciled (§11.4.120): plan/execute now exec the real
// plandex CLI, so the success path is driven through an injected fake binary.
func TestPlandexExecute(t *testing.T) {
	bin := writeFakePlandex(t, "EXEC_OK")
	t.Setenv("PLANDEX_BIN", bin)

	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "plan command",
			command: "plan",
			params:  map[string]interface{}{"task": "Build an API"},
			wantErr: false,
		},
		{
			name:    "plan without task fails",
			command: "plan",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "execute command",
			command: "execute",
			params:  map[string]interface{}{"task": "Implement auth"},
			wantErr: false,
		},
		{
			name:    "execute without task fails",
			command: "execute",
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
			result, err := p.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPlandexCapabilities(t *testing.T) {
	t.Parallel()
	p := New()
	info := p.Info()

	expectedCaps := []string{"task_planning", "execution", "multi_step"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

// TestPlandexIsAvailable reconciled (§11.4.120): availability now reflects
// whether the real plandex binary is resolvable (honest, never hardcoded true).
func TestPlandexIsAvailable(t *testing.T) {
	bin := writeFakePlandex(t, "AVAIL_OK")
	t.Setenv("PLANDEX_BIN", bin)
	p := New()
	assert.True(t, p.IsAvailable())

	t.Setenv("PLANDEX_BIN", filepath.Join(t.TempDir(), "does-not-exist"))
	assert.False(t, p.IsAvailable())
}

// TestPlandexPlanResult reconciled (§11.4.120): the "plan" field MUST be the
// fake binary's REAL stdout (proves exec ran), never the old hardcoded
// ["Step 1: Analyze", ...] list.
func TestPlandexPlanResult(t *testing.T) {
	const marker = "PLANDEX_RAN_1f5b"
	bin := writeFakePlandex(t, marker)
	t.Setenv("PLANDEX_BIN", bin)

	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := p.Execute(ctx, "plan", map[string]interface{}{
		"task": "Create microservice",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create microservice", resultMap["task"])

	plan, _ := resultMap["plan"].(string)
	assert.Contains(t, plan, marker, "plan must be real plandex process output")
	assert.Contains(t, plan, "Create microservice", "task must be forwarded to the binary")
	assert.NotContains(t, plan, "Step 1: Analyze", "must not be the old hardcoded plan list")
}

// TestPlandexExecuteResult reconciled (§11.4.120): the "result" field MUST be
// the fake binary's REAL stdout, never the old "Executed: <task>" literal.
func TestPlandexExecuteResult(t *testing.T) {
	const marker = "PLANDEX_EXEC_8a2d"
	bin := writeFakePlandex(t, marker)
	t.Setenv("PLANDEX_BIN", bin)

	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Mode: "review",
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)

	result, err := p.Execute(ctx, "execute", map[string]interface{}{
		"task": "Deploy app",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Deploy app", resultMap["task"])
	assert.Equal(t, "review", resultMap["mode"])

	out, _ := resultMap["result"].(string)
	assert.Contains(t, out, marker, "result must be real plandex process output")
	assert.NotContains(t, out, "Executed: Deploy app", "must not be the old templated literal")
}

// TestD17_Plandex_AbsentBinaryIsHonestError proves that with NO plandex binary
// available, plan/execute return an honest error — never a fabricated template.
func TestD17_Plandex_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("PLANDEX_BIN", filepath.Join(t.TempDir(), "does-not-exist-plandex"))

	p := New()
	ctx := context.Background()
	if _, err := p.plan(ctx, map[string]interface{}{"task": "x"}); err == nil {
		t.Fatal("D17 BLUFF: plan returned success with NO plandex binary — " +
			"must return an honest error, never a fabricated plan list.")
	}
	if _, err := p.execute(ctx, map[string]interface{}{"task": "x"}); err == nil {
		t.Fatal("D17 BLUFF: execute returned success with NO plandex binary — " +
			"must return an honest error, never a fabricated result.")
	}
}

// TestD17_Plandex_PlanIsStubBluff is the §11.4.115 RED-polarity reproduction,
// runnable ONLY under PIN_STUB_BLUFF=1.
func TestD17_Plandex_PlanIsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1.")
	}
	const marker = "PLANDEX_RED_3c9e"
	bin := writeFakePlandex(t, marker)
	t.Setenv("PLANDEX_BIN", bin)

	p := New()
	ctx := context.Background()
	res, err := p.plan(ctx, map[string]interface{}{"task": "x"})
	require.NoError(t, err)
	m, _ := res.(map[string]interface{})
	if planList, ok := m["plan"].([]string); ok {
		_ = planList
		t.Fatal("D17 BLUFF PINNED: plandex.plan returned the hardcoded plan list " +
			"without exec-ing the real binary (BLUFF-001).")
	}
	if plan, _ := m["plan"].(string); strings.Contains(plan, "Step 1: Analyze") {
		t.Fatalf("D17 BLUFF PINNED: plandex.plan returned hardcoded plan steps %q.", plan)
	}
}
