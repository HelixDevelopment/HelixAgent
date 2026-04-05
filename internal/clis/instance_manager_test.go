// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"fmt"
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
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
	}
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	// Expect insert - use AnyArg for NullString fields due to sqlmock comparison issues
	mock.ExpectExec("INSERT INTO agent_instances").
		WithArgs(
			sqlmock.AnyArg(), // id
			TypeAider,
			sqlmock.AnyArg(), // name
			StatusCreating,
			sqlmock.AnyArg(), // config json
			sqlmock.AnyArg(), // provider json
			config.MaxMemoryMB,
			config.MaxCPUPercent,
			sqlmock.AnyArg(), // current_session_id (NullString)
			sqlmock.AnyArg(), // current_task_id (NullString)
			HealthUnknown,
			0, 0, int64(0),
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect status update
	mock.ExpectExec("UPDATE agent_instances SET status = .*, health_status = .*, started_at = NOW()") .
		WithArgs(StatusIdle, HealthHealthy, sqlmock.AnyArg()).
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
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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
			ID:        "test-instance",
			Type:      TypeAider,
			Name:      "test",
			Status:    StatusIdle,
			RequestCh:  make(chan *Request, 10),
			ResponseCh: make(chan *Response, 10),
			EventCh:    make(chan *Event, 10),
		}, nil
	})
	im.pools[TypeAider] = pool

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
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
	}
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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
		Status:     StatusActive,
		SessionID:  "test-session",
		RequestCh:  make(chan *Request, 10),
		ResponseCh: make(chan *Response, 10),
		EventCh:    make(chan *Event, 10),
	}

	// Register in instances map
	im.mu.Lock()
	im.instances[instance.ID] = instance
	im.mu.Unlock()

	ctx := context.Background()
	err = im.ReleaseInstance(ctx, instance)
	require.NoError(t, err)
	assert.Equal(t, StatusIdle, instance.Status)
	assert.Equal(t, "", instance.SessionID)

	im.Close()
}

func TestInstanceManager_GetInstance(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	im.mu.Lock()
	im.instances[instance.ID] = instance
	im.mu.Unlock()

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
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	im.mu.Lock()
	for _, inst := range instances {
		im.instances[inst.ID] = inst
	}
	im.mu.Unlock()

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
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	im.mu.Lock()
	im.instances[instance.ID] = instance
	im.mu.Unlock()

	// Expect database update
	mock.ExpectExec("UPDATE agent_instances SET status = .*, terminated_at = NOW()") .
		WithArgs(StatusTerminated, "test-instance").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = im.TerminateInstance(ctx, "test-instance")
	require.NoError(t, err)

	// Verify instance removed
	im.mu.RLock()
	_, exists := im.instances["test-instance"]
	im.mu.RUnlock()
	assert.False(t, exists)

	im.Close()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInstanceManager_SendRequest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	im.mu.Lock()
	im.instances[instance.ID] = instance
	im.mu.Unlock()

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
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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
		im.mu.Lock()
		im.instances[inst.ID] = inst
		im.mu.Unlock()

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
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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

	// Available types
	assert.True(t, im.IsAgentTypeAvailable(TypeAider))
	assert.True(t, im.IsAgentTypeAvailable(TypeClaudeCode))
	assert.True(t, im.IsAgentTypeAvailable(TypeCodex))
	assert.True(t, im.IsAgentTypeAvailable(TypeCline))
	assert.True(t, im.IsAgentTypeAvailable(TypeOpenHands))
	assert.True(t, im.IsAgentTypeAvailable(TypeKiro))
	assert.True(t, im.IsAgentTypeAvailable(TypeContinue))

	// Unavailable type
	assert.False(t, im.IsAgentTypeAvailable("unknown_type"))

	im.Close()
}

func TestInstanceManager_GetMetrics(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping InstanceManager test in short mode - requires database setup")
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
			ID:        "bench-instance",
			Type:      TypeAider,
			Status:    StatusIdle,
			RequestCh:  make(chan *Request, 10),
			ResponseCh: make(chan *Response, 10),
			EventCh:    make(chan *Event, 10),
		}, nil
	})
	im.pools[TypeAider] = pool

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst, _ := im.AcquireInstance(ctx, TypeAider)
		im.ReleaseInstance(ctx, inst)
	}
}
