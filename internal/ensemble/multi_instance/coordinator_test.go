// Package multi_instance provides multi-instance ensemble coordination for HelixAgent.
package multi_instance

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/clis"
	"dev.helix.agent/internal/ensemble/synchronization"
)

func TestNewCoordinator(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)

	coord := NewCoordinator(db, nil, im, syncMgr)
	require.NotNil(t, coord)
	assert.NotNil(t, coord.sessions)
	assert.NotNil(t, coord.loadBalancer)
	assert.NotNil(t, coord.healthMonitor)
	assert.NotNil(t, coord.workerPool)
	assert.NotNil(t, coord.eventBus)
}

func TestCoordinator_CreateSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	for i := 0; i < 2; i++ {
		mock.ExpectExec("INSERT INTO agent_instances").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE agent_instances SET status").
			WillReturnResult(sqlmock.NewResult(1, 1))
	}

	mock.ExpectExec("INSERT INTO ensemble_sessions").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	participants := ParticipantConfig{
		Primary: InstanceConfig{
			Type: clis.TypeAider,
		},
		Critiques: []InstanceConfig{
			{Type: clis.TypeClaudeCode},
		},
	}

	session, err := coord.CreateSession(ctx, StrategyVoting, DefaultEnsembleConfig(), participants)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, StrategyVoting, session.Strategy)
	assert.Equal(t, SessionStatusCreating, session.Status)
	assert.NotEmpty(t, session.ID)

	coord.Close()
}

func TestCoordinator_ExecuteSession_Voting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	session := &EnsembleSession{
		ID:       "test-session",
		Strategy: StrategyVoting,
		Config:   DefaultEnsembleConfig(),
		Status:   SessionStatusCreating,
	}

	coord.sessions.Put(session.ID, session)

	mock.ExpectExec("UPDATE ensemble_sessions SET status = .* started_at").
		WithArgs(SessionStatusActive, "test-session").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE ensemble_sessions SET").
		WithArgs(
			SessionStatusFailed,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"test-session",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("UPDATE ensemble_sessions SET error_message").
		WithArgs(sqlmock.AnyArg(), "test-session").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	task := Task{
		Type:    "test",
		Content: "test content",
		Timeout: 5 * time.Second,
	}

	_, err = coord.ExecuteSession(ctx, session.ID, task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient participants")

	coord.Close()
}

func TestCoordinator_GetSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	session := &EnsembleSession{
		ID:     "test-session",
		Status: SessionStatusActive,
	}

	coord.sessions.Put(session.ID, session)

	// Get existing session
	retrieved, err := coord.GetSession("test-session")
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)

	// Get non-existent session
	_, err = coord.GetSession("non-existent")
	assert.Error(t, err)

	coord.Close()
}

func TestCoordinator_ListSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	// Add test sessions
	sessions := []*EnsembleSession{
		{ID: "1", Status: SessionStatusActive},
		{ID: "2", Status: SessionStatusCompleted},
		{ID: "3", Status: SessionStatusActive},
	}

	for _, s := range sessions {
		coord.sessions.Put(s.ID, s)
	}

	// List all
	all := coord.ListSessions("")
	assert.Len(t, all, 3)

	// Filter by status
	active := coord.ListSessions(SessionStatusActive)
	assert.Len(t, active, 2)

	completed := coord.ListSessions(SessionStatusCompleted)
	assert.Len(t, completed, 1)

	coord.Close()
}

func TestCoordinator_CancelSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	session := &EnsembleSession{
		ID:     "test-session",
		Status: SessionStatusActive,
	}

	coord.sessions.Put(session.ID, session)

	mock.ExpectExec("UPDATE ensemble_sessions SET status = .* WHERE id = .").
		WithArgs(SessionStatusCancelled, "test-session").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	err = coord.CancelSession(ctx, "test-session")
	require.NoError(t, err)
	assert.Equal(t, SessionStatusCancelled, session.Status)

	coord.Close()
}

func TestCoordinator_collectParticipants(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	session := &EnsembleSession{
		Primary: &clis.AgentInstance{ID: "primary"},
		Critiques: []*clis.AgentInstance{
			{ID: "critique1"},
			{ID: "critique2"},
		},
		Verifiers: []*clis.AgentInstance{
			{ID: "verifier1"},
		},
	}

	participants := coord.collectParticipants(session)
	assert.Len(t, participants, 4)

	coord.Close()
}

func TestCoordinator_calculateAgreement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	// All agree
	results := map[string]*AgentResult{
		"1": {Success: true, Result: "answer-A"},
		"2": {Success: true, Result: "answer-A"},
		"3": {Success: true, Result: "answer-A"},
	}
	agreement := coord.calculateAgreement(results)
	assert.Equal(t, 1.0, agreement)

	// Partial agreement
	results = map[string]*AgentResult{
		"1": {Success: true, Result: "answer-A"},
		"2": {Success: true, Result: "answer-A"},
		"3": {Success: true, Result: "answer-B"},
	}
	agreement = coord.calculateAgreement(results)
	assert.Equal(t, 2.0/3.0, agreement)

	// No agreement
	results = map[string]*AgentResult{
		"1": {Success: true, Result: "answer-A"},
		"2": {Success: true, Result: "answer-B"},
		"3": {Success: true, Result: "answer-C"},
	}
	agreement = coord.calculateAgreement(results)
	assert.Equal(t, 1.0/3.0, agreement)

	// Empty results
	results = map[string]*AgentResult{}
	agreement = coord.calculateAgreement(results)
	assert.Equal(t, 0.0, agreement)

	coord.Close()
}

func TestCoordinator_resultKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping coordinator test in short mode - requires database setup")  // SKIP-OK: #short-mode
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT (.+) FROM agent_instances").
		WillReturnRows(sqlmock.NewRows([]string{"id", "agent_type", "instance_name", "status", "config", "provider_config", "max_memory_mb", "max_cpu_percent", "current_session_id", "current_task_id", "health_status", "requests_processed", "errors_count", "total_execution_time_ms", "created_at", "updated_at"}))

	im := createMockInstanceManagerWithDB(t, db)
	syncMgr := createMockSyncManager(t, db)
	coord := NewCoordinator(db, nil, im, syncMgr)

	// Same results should have same key
	key1 := coord.resultKey(map[string]string{"answer": "A"})
	key2 := coord.resultKey(map[string]string{"answer": "A"})
	assert.Equal(t, key1, key2)

	// Different results should have different keys
	key3 := coord.resultKey(map[string]string{"answer": "B"})
	assert.NotEqual(t, key1, key3)

	coord.Close()
}

func TestEnsembleConfig_Defaults(t *testing.T) {
	config := DefaultEnsembleConfig()
	assert.Equal(t, 2, config.MinParticipants)
	assert.Equal(t, 5, config.MaxParticipants)
	assert.Equal(t, 0.6, config.ConsensusThreshold)
	assert.Equal(t, 3, config.MaxRounds)
	assert.Equal(t, 5*time.Minute, config.TimeoutPerRound)
	assert.Equal(t, 15*time.Minute, config.TotalTimeout)
	assert.True(t, config.EnableStreaming)
	assert.True(t, config.EnableFallbacks)
	assert.False(t, config.RequireConsensus)
}

func TestConsensusResult_Validation(t *testing.T) {
	// Test consensus reached
	result := &ConsensusResult{
		Reached:    true,
		Winner:     "answer-A",
		Confidence: 0.8,
		AllResults: map[string]*AgentResult{
			"1": {Success: true, Result: "answer-A"},
			"2": {Success: true, Result: "answer-A"},
		},
		Rounds:    1,
		Agreement: map[string]int{"answer-A": 2},
	}
	assert.True(t, result.Reached)
	assert.Equal(t, 0.8, result.Confidence)

	// Test consensus not reached
	result2 := &ConsensusResult{
		Reached:    false,
		Confidence: 0.4,
	}
	assert.False(t, result2.Reached)
}

// Helper functions

func createMockInstanceManagerWithDB(t *testing.T, db *sql.DB) *clis.InstanceManager {
	im, err := clis.NewInstanceManager(db, nil)
	require.NoError(t, err)
	return im
}

func createMockInstanceManager(t *testing.T) (*clis.InstanceManager, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return createMockInstanceManagerWithDB(t, db), mock
}

func createMockSyncManager(t *testing.T, db *sql.DB) *synchronization.SyncManager {
	return synchronization.NewSyncManager(db, nil, "test-node")
}

// Benchmarks

func BenchmarkCoordinator_collectParticipants(b *testing.B) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	im := createMockInstanceManagerWithDB(nil, db)
	syncMgr := createMockSyncManager(nil, db)
	coord := NewCoordinator(db, nil, im, syncMgr)
	defer coord.Close()

	session := &EnsembleSession{
		Primary: &clis.AgentInstance{ID: "primary"},
		Critiques: []*clis.AgentInstance{
			{ID: "critique1"},
			{ID: "critique2"},
			{ID: "critique3"},
		},
		Verifiers: []*clis.AgentInstance{
			{ID: "verifier1"},
			{ID: "verifier2"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coord.collectParticipants(session)
	}
}

func BenchmarkCoordinator_calculateAgreement(b *testing.B) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	im := createMockInstanceManagerWithDB(nil, db)
	syncMgr := createMockSyncManager(nil, db)
	coord := NewCoordinator(db, nil, im, syncMgr)
	defer coord.Close()

	results := map[string]*AgentResult{
		"1": {Success: true, Result: "answer-A"},
		"2": {Success: true, Result: "answer-A"},
		"3": {Success: true, Result: "answer-B"},
		"4": {Success: true, Result: "answer-A"},
		"5": {Success: true, Result: "answer-C"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coord.calculateAgreement(results)
	}
}
