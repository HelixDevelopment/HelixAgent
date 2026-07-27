// Package integration provides integration tests for HelixAgent.
package integration

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"dev.helix.agent/internal/clis"
	"dev.helix.agent/internal/ensemble/background"
	"dev.helix.agent/internal/ensemble/multi_instance"
	"dev.helix.agent/internal/ensemble/synchronization"
	_ "github.com/lib/pq"
)

// EnsembleIntegrationTestSuite tests ensemble functionality end-to-end
type EnsembleIntegrationTestSuite struct {
	suite.Suite
	db          *sql.DB
	instanceMgr *clis.InstanceManager
	syncMgr     *synchronization.SyncManager
	coordinator *multi_instance.Coordinator
	ctx         context.Context
	cancel      context.CancelFunc
}

func (s *EnsembleIntegrationTestSuite) SetupSuite() {
	// Setup database connection
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://helixagent:helixagent123@localhost:5432/helixagent_test?sslmode=disable"
	}

	var err error
	s.db, err = sql.Open("postgres", dbURL)
	if err != nil {
		s.T().Skipf("test database not reachable (%s): %v", dbURL, err)
		return
	}
	if err := s.db.Ping(); err != nil {
		s.T().Skipf("test database ping failed (%s): %v", dbURL, err)
		return
	}

	// Require the schema the coordinator/instance manager depends on.
	// The integration package does not own the schema — it expects
	// helixagent_test to already have agent_instances + ensemble_sessions.
	// When that schema is absent (fresh db, CI, local dev without
	// migrations), skip cleanly instead of flooding the suite with
	// INSERT failures.
	var exists bool
	err = s.db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			WHERE table_name = 'agent_instances')`,
	).Scan(&exists)
	if err != nil || !exists {
		s.T().Skip(
			"skipping ensemble integration: helixagent_test is missing the " +
				"agent_instances / ensemble_sessions schema (apply sql/schema/*.sql " +
				"to enable this suite)",
		)
		return
	}

	// Clean database
	s.cleanDatabase()

	// Setup context
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)

	// Initialize components
	s.instanceMgr, err = clis.NewInstanceManager(s.db, nil)
	s.Require().NoError(err)

	s.syncMgr = synchronization.NewSyncManager(s.db, nil, "test-node")

	s.coordinator = multi_instance.NewCoordinator(s.db, nil, s.instanceMgr, s.syncMgr)
	s.Require().NotNil(s.coordinator)
}

func (s *EnsembleIntegrationTestSuite) TearDownSuite() {
	if s.coordinator != nil {
		s.coordinator.Close()
	}
	if s.instanceMgr != nil {
		s.instanceMgr.Close()
	}
	if s.syncMgr != nil {
		s.syncMgr.Close()
	}
	if s.db != nil {
		s.cleanDatabase()
		s.db.Close()
	}
	s.cancel()
}

func (s *EnsembleIntegrationTestSuite) cleanDatabase() {
	tables := []string{
		"ensemble_sessions",
		"agent_instances",
		"distributed_locks",
		"crdt_state",
		"background_tasks",
		"semantic_cache",
	}

	for _, table := range tables {
		s.db.Exec("DELETE FROM " + table)
	}
}

func (s *EnsembleIntegrationTestSuite) TestCreateAndExecuteVotingSession() {
	// Create ensemble session with voting strategy
	participants := multi_instance.ParticipantConfig{
		Primary: multi_instance.InstanceConfig{
			Type:   clis.TypeHelixAgent,
			Config: clis.DefaultInstanceConfig(clis.TypeHelixAgent),
		},
		Critiques: []multi_instance.InstanceConfig{
			{Type: clis.TypeHelixAgent, Config: clis.DefaultInstanceConfig(clis.TypeHelixAgent)},
			{Type: clis.TypeHelixAgent, Config: clis.DefaultInstanceConfig(clis.TypeHelixAgent)},
		},
	}

	session, err := s.coordinator.CreateSession(
		s.ctx,
		multi_instance.StrategyVoting,
		multi_instance.DefaultEnsembleConfig(),
		participants,
	)
	s.Require().NoError(err)
	s.NotNil(session)
	s.Equal(multi_instance.StrategyVoting, session.Strategy)

	// Execute a task
	task := multi_instance.Task{
		Type:    "test",
		Content: "Generate a test response",
		Timeout: 30 * time.Second,
	}

	// Note: In a real test, we'd need actual running instances
	// For integration test without real instances, we expect an error
	_, err = s.coordinator.ExecuteSession(s.ctx, session.ID, task)
	// Expected to fail without real instances, but shouldn't panic
	s.Error(err)
}

func (s *EnsembleIntegrationTestSuite) TestAllCoordinationStrategies() {
	strategies := []multi_instance.EnsembleStrategy{
		multi_instance.StrategyVoting,
		multi_instance.StrategyDebate,
		multi_instance.StrategyConsensus,
		multi_instance.StrategyPipeline,
		multi_instance.StrategyParallel,
		multi_instance.StrategySequential,
		multi_instance.StrategyExpertPanel,
	}

	for _, strategy := range strategies {
		s.Run(string(strategy), func() {
			participants := multi_instance.ParticipantConfig{
				Primary: multi_instance.InstanceConfig{
					Type: clis.TypeHelixAgent,
				},
			}

			session, err := s.coordinator.CreateSession(
				s.ctx,
				strategy,
				multi_instance.DefaultEnsembleConfig(),
				participants,
			)
			s.Require().NoError(err)
			s.NotNil(session)
			s.Equal(strategy, session.Strategy)

			// Cleanup
			s.coordinator.CancelSession(s.ctx, session.ID)
		})
	}
}

func (s *EnsembleIntegrationTestSuite) TestSessionLifecycle() {
	// Create session
	participants := multi_instance.ParticipantConfig{
		Primary: multi_instance.InstanceConfig{Type: clis.TypeHelixAgent},
	}

	session, err := s.coordinator.CreateSession(
		s.ctx,
		multi_instance.StrategyVoting,
		multi_instance.DefaultEnsembleConfig(),
		participants,
	)
	s.Require().NoError(err)

	// Get session
	retrieved, err := s.coordinator.GetSession(session.ID)
	s.Require().NoError(err)
	s.Equal(session.ID, retrieved.ID)

	// List sessions
	sessions := s.coordinator.ListSessions("")
	s.True(len(sessions) > 0)

	// Cancel session
	err = s.coordinator.CancelSession(s.ctx, session.ID)
	s.NoError(err)

	// Verify cancelled
	cancelled, err := s.coordinator.GetSession(session.ID)
	s.NoError(err)
	s.Equal(multi_instance.SessionStatusCancelled, cancelled.Status)
}

func (s *EnsembleIntegrationTestSuite) TestInstanceManagerWithCoordinator() {
	// Create an instance
	config := clis.DefaultInstanceConfig(clis.TypeHelixAgent)
	providerName := "test-provider"

	instance, err := s.instanceMgr.CreateInstance(s.ctx, clis.TypeHelixAgent, config, providerName)
	s.Require().NoError(err)
	s.NotNil(instance)

	// Verify instance is tracked
	retrieved, err := s.instanceMgr.GetInstance(instance.ID)
	s.Require().NoError(err)
	s.Equal(instance.ID, retrieved.ID)

	// Acquire instance
	acquired, err := s.instanceMgr.AcquireInstance(s.ctx, clis.TypeHelixAgent)
	s.Require().NoError(err)
	s.NotNil(acquired)

	// Release instance
	err = s.instanceMgr.ReleaseInstance(s.ctx, acquired)
	s.NoError(err)

	// Terminate instance
	err = s.instanceMgr.TerminateInstance(s.ctx, instance.ID)
	s.NoError(err)
}

func (s *EnsembleIntegrationTestSuite) TestSyncManagerDistributedLock() {
	// Acquire lock
	lock, err := s.syncMgr.AcquireLock(s.ctx, "test-lock", 5*time.Second)
	s.Require().NoError(err)
	s.NotNil(lock)

	// Try to acquire same lock from another "node" should fail
	s.syncMgr.Close()
	s.syncMgr = synchronization.NewSyncManager(s.db, nil, "test-node-2")

	_, err = s.syncMgr.AcquireLock(s.ctx, "test-lock", 5*time.Second)
	s.Error(err) // Should fail - lock is held

	// Release lock from first node
	s.syncMgr.Close()
	s.syncMgr = synchronization.NewSyncManager(s.db, nil, "test-node")
	err = s.syncMgr.ReleaseLock(lock)
	s.NoError(err)

	// Now acquire should succeed
	s.syncMgr.Close()
	s.syncMgr = synchronization.NewSyncManager(s.db, nil, "test-node-2")
	lock2, err := s.syncMgr.AcquireLock(s.ctx, "test-lock", 5*time.Second)
	s.NoError(err)
	s.NotNil(lock2)

	// Cleanup
	s.syncMgr.ReleaseLock(lock2)
}

func (s *EnsembleIntegrationTestSuite) TestBackgroundWorkerPool() {
	pool := background.NewWorkerPoolWithDB(s.db, nil, 5)
	err := pool.Start(s.ctx)
	s.Require().NoError(err)

	// Submit multiple tasks
	var taskIDs []string
	for i := 0; i < 10; i++ {
		task := &clis.Task{
			Type:     clis.TaskTypeCodeAnalysis,
			Name:     "test-task",
			Payload:  map[string]string{"file": "test.go"},
			Priority: 3,
		}
		err := pool.Submit(s.ctx, task)
		s.NoError(err)
		taskIDs = append(taskIDs, task.ID)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Get stats
	stats := pool.GetStats()
	s.Equal(5, stats["size"])
	s.True(stats["tasks_submitted"].(uint64) >= 10)

	pool.Stop()
}

func (s *EnsembleIntegrationTestSuite) TestEventBusCommunication() {
	eb := clis.NewEventBus()
	defer eb.Close()

	// Subscribe to events
	sub := eb.Subscribe(clis.EventTypeStatus, 10)

	// Publish event
	event := &clis.Event{
		ID:        uuid.New(),
		Type:      clis.EventTypeStatus,
		Source:    "test",
		Payload:   map[string]interface{}{"status": "active"},
		Timestamp: time.Now(),
	}

	eb.PublishSync(event)

	// Receive event
	select {
	case received := <-sub.Ch:
		s.Equal(event.ID, received.ID)
	case <-time.After(time.Second):
		s.Fail("Timeout waiting for event")
	}

	eb.Unsubscribe(sub)
}

func TestEnsembleIntegration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration tests") // SKIP-OK: #integration-mode-only
	}
	suite.Run(t, new(EnsembleIntegrationTestSuite))
}

// Concurrent Ensemble Test
func TestConcurrentEnsembleSessions(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration tests") // SKIP-OK: #integration-mode-only
	}

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://helixagent:helixagent123@localhost:5432/helixagent_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("test database not reachable (%s): %v (SKIP-OK: #infra-unavailable)", dbURL, err)
		return
	}
	defer db.Close()

	// Mirror EnsembleIntegrationTestSuite.SetupSuite: the concurrency
	// test requires the agent_instances schema and skips cleanly when
	// it is missing instead of flooding the log with PQ 42P01 errors.
	var schemaExists bool
	if err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			WHERE table_name = 'agent_instances')`,
	).Scan(&schemaExists); err != nil || !schemaExists {
		t.Skip( // SKIP-OK: #legacy-untriaged
			"skipping concurrent ensemble test: helixagent_test is missing " +
				"agent_instances schema (apply sql/schema/*.sql to enable)",
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	instanceMgr, err := clis.NewInstanceManager(db, nil)
	require.NoError(t, err)
	defer instanceMgr.Close()

	syncMgr := synchronization.NewSyncManager(db, nil, "test-concurrent")
	defer syncMgr.Close()

	coordinator := multi_instance.NewCoordinator(db, nil, instanceMgr, syncMgr)
	defer coordinator.Close()

	// Create multiple sessions concurrently
	const numSessions = 10
	var wg sync.WaitGroup
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			participants := multi_instance.ParticipantConfig{
				Primary: multi_instance.InstanceConfig{
					Type: clis.TypeHelixAgent,
				},
			}

			_, err := coordinator.CreateSession(
				ctx,
				multi_instance.StrategyVoting,
				multi_instance.DefaultEnsembleConfig(),
				participants,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		if err != nil {
			t.Logf("Error: %v", err)
			errCount++
		}
	}

	assert.Equal(t, 0, errCount, "Should have no errors creating concurrent sessions")

	// Verify all sessions exist
	sessions := coordinator.ListSessions("")
	assert.True(t, len(sessions) >= numSessions, "Should have created all sessions")
}
