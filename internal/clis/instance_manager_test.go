// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstanceManager(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Expect recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)
	assert.NotNil(t, im)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInstanceManager_CreateInstance(t *testing.T) {
	// Not t.Parallel(): injects HELIX_AGENT_BIN_AIDER via t.Setenv (incompatible
	// with t.Parallel()).
	//
	// RECONCILED (§11.4.120) for D-11: CreateInstance gates on IsAgentTypeAvailable,
	// which is now a REAL exec.LookPath check instead of a hard-coded allowlist.
	// Creating a TypeAider instance therefore requires the aider binary to resolve;
	// inject a fake one (matching the new honest contract) so the type is genuinely
	// available on this host.
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	t.Setenv("HELIX_AGENT_BIN_AIDER", writeFakeAgentBin(t, "CREATE_INSTANCE_PROBE"))
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query expectation
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	// Test creating an instance
	ctx := context.Background()
	config := DefaultInstanceConfig(TypeAider)
	provider := "test-provider"

	mock.ExpectExec("INSERT INTO agent_instances").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE agent_instances").
		WillReturnResult(sqlmock.NewResult(1, 1))

	instance, err := im.CreateInstance(ctx, TypeAider, config, provider)
	require.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, TypeAider, instance.Type)
	assert.Equal(t, StatusIdle, instance.Status)
	assert.Equal(t, HealthHealthy, instance.Health)

	// Cleanup
	im.Close()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInstanceManager_AcquireInstance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	// Create pool for Aider
	pool := NewInstancePool(TypeAider, DefaultPoolConfig(), func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:         "test-instance",
			Type:       TypeAider,
			Name:       "test",
			Status:     StatusIdle,
			RequestCh:  make(chan *Request, 10),
			ResponseCh: make(chan *Response, 10),
			EventCh:    make(chan *Event, 10),
		}, nil
	})
	im.pools.Put(TypeAider, pool)

	ctx := context.Background()
	instance, err := im.AcquireInstance(ctx, TypeAider)
	require.NoError(t, err)
	assert.NotNil(t, instance)
	assert.Equal(t, TypeAider, instance.Type)

	im.Close()
}

func TestInstanceManager_ReleaseInstance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	mock.ExpectExec("UPDATE agent_instances").
		WillReturnResult(sqlmock.NewResult(1, 1))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	instance := &AgentInstance{
		ID:         "test-instance",
		Type:       TypeAider,
		Status:     StatusActive,
		SessionID:  "test-session",
		RequestCh:  make(chan *Request, 10),
		ResponseCh: make(chan *Response, 10),
		EventCh:    make(chan *Event, 10),
	}

	im.instances.Put(instance.ID, instance)

	ctx := context.Background()
	err = im.ReleaseInstance(ctx, instance)
	require.NoError(t, err)

	status := instance.Status
	assert.Contains(t, []InstanceStatus{StatusIdle, StatusTerminated, StatusTerminating}, status,
		"ReleaseInstance should set idle or trigger termination")
	assert.Equal(t, "", instance.SessionID)

	im.Close()
}

func TestInstanceManager_GetInstance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	instance := &AgentInstance{
		ID:     "test-instance",
		Type:   TypeAider,
		Status: StatusIdle,
	}

	im.instances.Put(instance.ID, instance)

	// Get existing instance
	retrieved, err := im.GetInstance("test-instance")
	require.NoError(t, err)
	assert.Equal(t, instance.ID, retrieved.ID)

	// Get non-existent instance
	_, err = im.GetInstance("non-existent")
	assert.Error(t, err)

	im.Close()
}

func TestInstanceManager_ListInstances(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	// Add test instances
	instances := []*AgentInstance{
		{ID: "1", Type: TypeAider, Status: StatusIdle},
		{ID: "2", Type: TypeClaudeCode, Status: StatusActive},
		{ID: "3", Type: TypeAider, Status: StatusIdle},
	}

	for _, inst := range instances {
		im.instances.Put(inst.ID, inst)
	}

	// List all
	all := im.ListInstances("", "")
	assert.Len(t, all, 3)

	// Filter by status
	idle := im.ListInstances(StatusIdle, "")
	assert.Len(t, idle, 2)

	// Filter by type
	aider := im.ListInstances("", TypeAider)
	assert.Len(t, aider, 2)

	// Filter by both
	aiderIdle := im.ListInstances(StatusIdle, TypeAider)
	assert.Len(t, aiderIdle, 2)

	im.Close()
}

func TestInstanceManager_TerminateInstance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	instance := &AgentInstance{
		ID:         "test-instance",
		Type:       TypeAider,
		Status:     StatusIdle,
		RequestCh:  make(chan *Request, 10),
		ResponseCh: make(chan *Response, 10),
		EventCh:    make(chan *Event, 10),
	}

	im.instances.Put(instance.ID, instance)

	// Expect database update
	mock.ExpectExec("UPDATE agent_instances SET status = .*, terminated_at = NOW()").
		WithArgs(StatusTerminated, "test-instance").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = im.TerminateInstance(ctx, "test-instance")
	require.NoError(t, err)

	// Verify instance removed
	_, exists := im.instances.Get("test-instance")
	assert.False(t, exists)

	im.Close()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInstanceManager_SendRequest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	instance := &AgentInstance{
		ID:         "test-instance",
		Type:       TypeAider,
		Status:     StatusIdle,
		Health:     HealthHealthy,
		RequestCh:  make(chan *Request, 10),
		ResponseCh: make(chan *Response, 10),
		EventCh:    make(chan *Event, 10),
	}

	im.instances.Put(instance.ID, instance)

	// Start response handler
	go func() {
		for req := range instance.RequestCh {
			instance.ResponseCh <- &Response{
				RequestID: req.ID,
				Success:   true,
				Result:    "test-result",
				Duration:  100 * time.Millisecond,
			}
		}
	}()

	ctx := context.Background()
	req := &Request{
		ID:      "test-request",
		Type:    RequestTypeExecute,
		Payload: "test-payload",
		Timeout: 5 * time.Second,
	}

	resp, err := im.SendRequest(ctx, "test-instance", req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "test-result", resp.Result)

	im.Close()
}

func TestInstanceManager_BroadcastRequest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	// Create test instances
	for i := 0; i < 3; i++ {
		inst := &AgentInstance{
			ID:         fmt.Sprintf("instance-%d", i),
			Type:       TypeAider,
			Status:     StatusIdle,
			Health:     HealthHealthy,
			RequestCh:  make(chan *Request, 10),
			ResponseCh: make(chan *Response, 10),
			EventCh:    make(chan *Event, 10),
		}
		im.instances.Put(inst.ID, inst)

		// Start response handler
		go func(id string, reqCh chan *Request, respCh chan *Response) {
			for req := range reqCh {
				respCh <- &Response{
					RequestID: req.ID,
					Success:   true,
					Result:    fmt.Sprintf("result-from-%s", id),
					Duration:  100 * time.Millisecond,
				}
			}
		}(inst.ID, inst.RequestCh, inst.ResponseCh)
	}

	ctx := context.Background()
	req := &Request{
		ID:      "broadcast-request",
		Type:    RequestTypeQuery,
		Timeout: 5 * time.Second,
	}

	results := im.BroadcastRequest(ctx, TypeAider, req)
	assert.Len(t, results, 3)

	for id, resp := range results {
		assert.True(t, resp.Success)
		assert.Equal(t, fmt.Sprintf("result-from-%s", id), resp.Result)
	}

	im.Close()
}

func TestInstanceManager_IsAgentTypeAvailable(t *testing.T) {
	// Not t.Parallel(): this test injects HELIX_AGENT_BIN_<TYPE> via t.Setenv,
	// which is incompatible with t.Parallel().
	//
	// RECONCILED (§11.4.120) for D-11: IsAgentTypeAvailable USED to be a hard-coded
	// enum allowlist that returned true for TypeKiro/TypeContinue/TypeHelixAgent
	// purely by membership — even though those agents have no resolvable CLI binary
	// (they are IDE-only / project-owned-name-unverified). It now performs a REAL
	// per-type exec.LookPath check (resolveAgentBinary, honoring the
	// HELIX_AGENT_BIN_<TYPE> override) via the shared agentDefaultCommand table.
	// These assertions now exercise the NEW mechanism: a type is available IFF its
	// table command resolves to a real binary on this host.
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	// A real-CLI agent type becomes available the moment its binary resolves —
	// inject a fake binary on PATH via the HELIX_AGENT_BIN_<TYPE> override (the
	// same mechanism resolveAgentBinary honors). This proves the check is a REAL
	// exec.LookPath, not a hard-coded allowlist.
	fakeBin := writeFakeAgentBin(t, "AVAILABILITY_PROBE")
	for _, tc := range []struct {
		envKey string
		typ    CLIAgentType
	}{
		{"HELIX_AGENT_BIN_AIDER", TypeAider},
		{"HELIX_AGENT_BIN_CLAUDE_CODE", TypeClaudeCode},
		{"HELIX_AGENT_BIN_CODEX", TypeCodex},
		{"HELIX_AGENT_BIN_CLINE", TypeCline},
		{"HELIX_AGENT_BIN_OPENHANDS", TypeOpenHands},
		{"HELIX_AGENT_BIN_QWENCODER", TypeQwenCoder},
		{"HELIX_AGENT_BIN_GITHUB_COPILOT", TypeGitHubCopilot},
		{"HELIX_AGENT_BIN_GEMINI_ASSIST", TypeGeminiAssist},
	} {
		t.Setenv(tc.envKey, fakeBin)
		assert.Truef(t, im.IsAgentTypeAvailable(tc.typ),
			"%s must be available with a real binary injected", tc.typ)
	}

	// IDE-only / hosted-web / project-name-unverified agents have an empty command
	// in the table → never available (the old allowlist falsely reported these as
	// available even though they cannot run). This is the §11.4.120 reconciliation:
	// the assertion now matches the corrected contract, not the removed allowlist.
	assert.False(t, im.IsAgentTypeAvailable(TypeKiro))
	assert.False(t, im.IsAgentTypeAvailable(TypeContinue))
	assert.False(t, im.IsAgentTypeAvailable(TypeHelixAgent))
	assert.False(t, im.IsAgentTypeAvailable(TypeCursor))

	// The negative branch of the real exec.LookPath check (a real-CLI agent whose
	// binary override points at a non-existent path → false) is covered by
	// TestD11_IsAgentTypeAvailable_RealLookPath case (c).

	// Unknown type (not in the table) → false.
	assert.False(t, im.IsAgentTypeAvailable("unknown_type"))

	im.Close()
}

func TestInstanceManager_GetMetrics(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup") // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup recovery query
	mock.ExpectQuery("SELECT id, agent_type").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "agent_type", "instance_name", "status", "config", "provider_config",
			"max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id",
			"health_status", "requests_processed", "errors_count", "total_execution_time_ms",
			"created_at", "updated_at",
		}))

	im, err := NewInstanceManager(db, nil)
	require.NoError(t, err)

	metrics := im.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "created_total")
	assert.Contains(t, metrics, "destroyed_total")
	assert.Contains(t, metrics, "active_count")
	assert.Contains(t, metrics, "pool_count")

	im.Close()
}

func TestAgentInstance_IsActive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		status   InstanceStatus
		expected bool
	}{
		{"active", StatusActive, true},
		{"idle", StatusIdle, true},
		{"background", StatusBackground, true},
		{"creating", StatusCreating, false},
		{"terminated", StatusTerminated, false},
		{"failed", StatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inst := &AgentInstance{Status: tt.status}
			assert.Equal(t, tt.expected, inst.IsActive())
		})
	}
}

func TestAgentInstance_IsHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		health   HealthStatus
		expected bool
	}{
		{"healthy", HealthHealthy, true},
		{"degraded", HealthDegraded, false},
		{"unhealthy", HealthUnhealthy, false},
		{"unknown", HealthUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inst := &AgentInstance{Health: tt.health}
			assert.Equal(t, tt.expected, inst.IsHealthy())
		})
	}
}

func TestAgentInstance_CanAcceptWork(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		status   InstanceStatus
		health   HealthStatus
		expected bool
	}{
		{"idle-healthy", StatusIdle, HealthHealthy, true},
		{"active-healthy", StatusActive, HealthHealthy, true},
		{"idle-degraded", StatusIdle, HealthDegraded, false},
		{"terminated-healthy", StatusTerminated, HealthHealthy, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inst := &AgentInstance{Status: tt.status, Health: tt.health}
			assert.Equal(t, tt.expected, inst.CanAcceptWork())
		})
	}
}

func BenchmarkInstanceManager_CreateInstance(b *testing.B) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	mock.ExpectQuery("SELECT id, agent_type").WillReturnRows(sqlmock.NewRows([]string{}))

	im, _ := NewInstanceManager(db, nil)
	defer im.Close()

	// D-11: CreateInstance gates on the real IsAgentTypeAvailable exec.LookPath
	// check; inject a fake aider binary so the type resolves on this host.
	if runtime.GOOS != "windows" {
		bin := filepath.Join(b.TempDir(), "fake-aider")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			b.Fatalf("write fake aider binary: %v", err)
		}
		b.Setenv("HELIX_AGENT_BIN_AIDER", bin)
	}

	ctx := context.Background()
	config := DefaultInstanceConfig(TypeAider)
	provider := "test-provider"

	mock.ExpectExec("INSERT INTO agent_instances").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE agent_instances SET status").WillReturnResult(sqlmock.NewResult(1, 1))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		im.CreateInstance(ctx, TypeAider, config, provider)
	}
}

func BenchmarkInstanceManager_AcquireRelease(b *testing.B) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	mock.ExpectQuery("SELECT id, agent_type").WillReturnRows(sqlmock.NewRows([]string{}))

	im, _ := NewInstanceManager(db, nil)
	defer im.Close()

	// Create pool
	pool := NewInstancePool(TypeAider, DefaultPoolConfig(), func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:         "bench-instance",
			Type:       TypeAider,
			Status:     StatusIdle,
			RequestCh:  make(chan *Request, 10),
			ResponseCh: make(chan *Response, 10),
			EventCh:    make(chan *Event, 10),
		}, nil
	})
	im.pools.Put(TypeAider, pool)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst, _ := im.AcquireInstance(ctx, TypeAider)
		im.ReleaseInstance(ctx, inst)
	}
}
