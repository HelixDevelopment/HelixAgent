package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLegacyQueue implements LegacyTaskQueue for testing.
type mockLegacyQueue struct {
	mu            sync.RWMutex
	tasks         map[string]*LegacyTask
	enqueueErr    error
	dequeueErr    error
	completeErr   error
	failErr       error
	migrateErr    error
	enqueueCalled int
	taskCounter   int
}

func newMockLegacyQueue() *mockLegacyQueue {
	return &mockLegacyQueue{
		tasks: make(map[string]*LegacyTask),
	}
}

func (m *mockLegacyQueue) Enqueue(ctx context.Context, taskType string, payload []byte, priority int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enqueueErr != nil {
		return "", m.enqueueErr
	}

	m.taskCounter++
	m.enqueueCalled++
	id := generateMessageID()
	m.tasks[id] = &LegacyTask{
		ID:        id,
		Type:      taskType,
		Payload:   payload,
		Priority:  priority,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	return id, nil
}

func (m *mockLegacyQueue) Dequeue(ctx context.Context, workerID string) (*LegacyTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dequeueErr != nil {
		return nil, m.dequeueErr
	}

	for _, task := range m.tasks {
		if task.Status == "pending" {
			task.Status = "processing"
			task.WorkerID = workerID
			now := time.Now()
			task.StartedAt = &now
			return task, nil
		}
	}
	return nil, nil
}

func (m *mockLegacyQueue) Complete(ctx context.Context, taskID string, result []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.completeErr != nil {
		return m.completeErr
	}

	if task, ok := m.tasks[taskID]; ok {
		task.Status = "completed"
		task.Result = result
		now := time.Now()
		task.CompletedAt = &now
	}
	return nil
}

func (m *mockLegacyQueue) Fail(ctx context.Context, taskID string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failErr != nil {
		return m.failErr
	}

	if task, ok := m.tasks[taskID]; ok {
		task.Status = "failed"
		if err != nil {
			task.Error = err.Error()
		}
	}
	return nil
}

func (m *mockLegacyQueue) GetPendingTasks(ctx context.Context) ([]*LegacyTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []*LegacyTask
	for _, task := range m.tasks {
		if task.Status == "pending" && !task.Migrated {
			pending = append(pending, task)
		}
	}
	return pending, nil
}

func (m *mockLegacyQueue) GetTask(ctx context.Context, taskID string) (*LegacyTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if task, ok := m.tasks[taskID]; ok {
		return task, nil
	}
	return nil, nil
}

func (m *mockLegacyQueue) MarkMigrated(ctx context.Context, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.migrateErr != nil {
		return m.migrateErr
	}

	if task, ok := m.tasks[taskID]; ok {
		task.Migrated = true
	}
	return nil
}

// mockMessageBroker implements MessageBroker for testing.
type mockMessageBroker struct {
	mu              sync.RWMutex
	connected       bool
	messages        []*Message
	publishErr      error
	subscribeErr    error
	publishCalled   int
	subscribeCalled int
	brokerType      BrokerType
}

func newMockMessageBroker(brokerType BrokerType) *mockMessageBroker {
	return &mockMessageBroker{
		messages:   make([]*Message, 0),
		brokerType: brokerType,
	}
}

func (m *mockMessageBroker) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

func (m *mockMessageBroker) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

func (m *mockMessageBroker) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected {
		return errors.New("not connected")
	}
	return nil
}

func (m *mockMessageBroker) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

func (m *mockMessageBroker) Publish(ctx context.Context, topic string, message *Message, opts ...PublishOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return m.publishErr
	}

	m.publishCalled++
	m.messages = append(m.messages, message)
	return nil
}

func (m *mockMessageBroker) PublishBatch(ctx context.Context, topic string, messages []*Message, opts ...PublishOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return m.publishErr
	}

	for _, msg := range messages {
		m.publishCalled++
		m.messages = append(m.messages, msg)
	}
	return nil
}

func (m *mockMessageBroker) Subscribe(ctx context.Context, topic string, handler MessageHandler, opts ...SubscribeOption) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}

	m.subscribeCalled++
	return &mockSubscription{topic: topic, active: true}, nil
}

func (m *mockMessageBroker) BrokerType() BrokerType {
	return m.brokerType
}

func (m *mockMessageBroker) GetMetrics() *BrokerMetrics {
	return NewBrokerMetrics()
}

type mockSubscription struct {
	topic  string
	active bool
}

func (s *mockSubscription) Unsubscribe() error {
	s.active = false
	return nil
}

func (s *mockSubscription) IsActive() bool {
	return s.active
}

func (s *mockSubscription) Topic() string {
	return s.topic
}

func (s *mockSubscription) ID() string {
	return "mock-sub-" + s.topic
}

func TestMigrationMode_String(t *testing.T) {
	tests := []struct {
		mode     MigrationMode
		expected string
	}{
		{ModeLegacy, "legacy"},
		{ModeDualWrite, "dual_write"},
		{ModeMessaging, "messaging"},
		{ModeRollback, "rollback"},
		{MigrationMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestParseMigrationMode(t *testing.T) {
	tests := []struct {
		input    string
		expected MigrationMode
	}{
		{"legacy", ModeLegacy},
		{"dual_write", ModeDualWrite},
		{"messaging", ModeMessaging},
		{"rollback", ModeRollback},
		{"unknown", ModeLegacy},
		{"", ModeLegacy},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseMigrationMode(tt.input))
		})
	}
}

func TestNewMigrationManager(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		require.NotNil(t, m)
		assert.Equal(t, ModeLegacy, m.Mode())
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &MigrationConfig{
			Mode: ModeMessaging,
		}
		m := NewMigrationManager(cfg, nil)
		require.NotNil(t, m)
		assert.Equal(t, ModeMessaging, m.Mode())
	})
}

func TestMigrationManager_SetMode(t *testing.T) {
	t.Run("valid transition legacy to dual_write", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		err := m.SetMode(ModeDualWrite)
		require.NoError(t, err)
		assert.Equal(t, ModeDualWrite, m.Mode())
	})

	t.Run("valid transition dual_write to messaging", func(t *testing.T) {
		cfg := &MigrationConfig{Mode: ModeDualWrite}
		m := NewMigrationManager(cfg, nil)
		err := m.SetMode(ModeMessaging)
		require.NoError(t, err)
		assert.Equal(t, ModeMessaging, m.Mode())
	})

	t.Run("valid transition to rollback from any mode", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		err := m.SetMode(ModeRollback)
		require.NoError(t, err)
		assert.Equal(t, ModeRollback, m.Mode())
	})

	t.Run("invalid transition legacy to messaging", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		err := m.SetMode(ModeMessaging)
		require.Error(t, err)
		assert.Equal(t, ModeLegacy, m.Mode())
	})

	t.Run("same mode is noop", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		err := m.SetMode(ModeLegacy)
		require.NoError(t, err)
		assert.Equal(t, ModeLegacy, m.Mode())
	})
}

func TestMigrationManager_EnqueueTask_Legacy(t *testing.T) {
	m := NewMigrationManager(nil, nil)
	legacyQueue := newMockLegacyQueue()
	m.SetLegacyQueue(legacyQueue)

	ctx := context.Background()
	taskID, err := m.EnqueueTask(ctx, "test", []byte(`{"key":"value"}`), 5)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)
	assert.Equal(t, 1, legacyQueue.enqueueCalled)
}

func TestMigrationManager_EnqueueTask_Messaging(t *testing.T) {
	cfg := &MigrationConfig{Mode: ModeMessaging}
	m := NewMigrationManager(cfg, nil)

	rabbitBroker := newMockMessageBroker(BrokerTypeRabbitMQ)
	m.SetRabbitMQBroker(rabbitBroker)

	ctx := context.Background()
	taskID, err := m.EnqueueTask(ctx, "test", []byte(`{"key":"value"}`), 5)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)
	assert.Equal(t, 1, rabbitBroker.publishCalled)
}

func TestMigrationManager_EnqueueTask_DualWrite(t *testing.T) {
	cfg := &MigrationConfig{
		Mode:             ModeDualWrite,
		DualWriteTimeout: 5 * time.Second,
	}
	m := NewMigrationManager(cfg, nil)

	legacyQueue := newMockLegacyQueue()
	rabbitBroker := newMockMessageBroker(BrokerTypeRabbitMQ)
	m.SetLegacyQueue(legacyQueue)
	m.SetRabbitMQBroker(rabbitBroker)

	ctx := context.Background()
	taskID, err := m.EnqueueTask(ctx, "llm_request", []byte(`{"prompt":"hello"}`), 8)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)
	assert.Equal(t, 1, legacyQueue.enqueueCalled)
	assert.Equal(t, 1, rabbitBroker.publishCalled)

	metrics := m.GetMetrics()
	assert.Equal(t, int64(1), metrics.DualWriteCount.Load())
}

func TestMigrationManager_EnqueueTask_DualWrite_RabbitMQFailure(t *testing.T) {
	cfg := &MigrationConfig{
		Mode:             ModeDualWrite,
		LogDiscrepancies: true,
		DualWriteTimeout: 5 * time.Second,
	}
	m := NewMigrationManager(cfg, nil)

	legacyQueue := newMockLegacyQueue()
	rabbitBroker := newMockMessageBroker(BrokerTypeRabbitMQ)
	rabbitBroker.publishErr = errors.New("connection failed")
	m.SetLegacyQueue(legacyQueue)
	m.SetRabbitMQBroker(rabbitBroker)

	ctx := context.Background()
	// Should succeed even if RabbitMQ fails
	taskID, err := m.EnqueueTask(ctx, "test", []byte(`{}`), 5)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)
	assert.Equal(t, 1, legacyQueue.enqueueCalled)

	metrics := m.GetMetrics()
	assert.Equal(t, int64(1), metrics.DiscrepanciesFound.Load())
	assert.Equal(t, int64(1), metrics.ErrorCount.Load())
}

func TestMigrationManager_EnqueueTask_NoQueue(t *testing.T) {
	t.Run("legacy mode without queue", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		ctx := context.Background()
		_, err := m.EnqueueTask(ctx, "test", []byte(`{}`), 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "legacy queue not configured")
	})

	t.Run("messaging mode without broker", func(t *testing.T) {
		cfg := &MigrationConfig{Mode: ModeMessaging}
		m := NewMigrationManager(cfg, nil)
		ctx := context.Background()
		_, err := m.EnqueueTask(ctx, "test", []byte(`{}`), 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RabbitMQ broker not configured")
	})
}

func TestMigrationManager_TaskTypeToQueue(t *testing.T) {
	m := NewMigrationManager(nil, nil)

	tests := []struct {
		taskType string
		expected string
	}{
		{"llm_request", "helixagent.tasks.llm"},
		{"llm.request", "helixagent.tasks.llm"},
		{"debate", "helixagent.tasks.debate"},
		{"debate.round", "helixagent.tasks.debate"},
		{"verification", "helixagent.tasks.verification"},
		{"verify", "helixagent.tasks.verification"},
		{"notification", "helixagent.tasks.notifications"},
		{"notify", "helixagent.tasks.notifications"},
		{"other", "helixagent.tasks.background"},
		{"", "helixagent.tasks.background"},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			assert.Equal(t, tt.expected, m.taskTypeToQueue(tt.taskType))
		})
	}
}

func TestMigrationManager_MigratePendingTasks(t *testing.T) {
	cfg := &MigrationConfig{Mode: ModeDualWrite}
	m := NewMigrationManager(cfg, nil)

	legacyQueue := newMockLegacyQueue()
	rabbitBroker := newMockMessageBroker(BrokerTypeRabbitMQ)
	m.SetLegacyQueue(legacyQueue)
	m.SetRabbitMQBroker(rabbitBroker)

	ctx := context.Background()

	// Add some pending tasks
	legacyQueue.Enqueue(ctx, "llm_request", []byte(`{"prompt":"task1"}`), 5)
	legacyQueue.Enqueue(ctx, "debate", []byte(`{"topic":"task2"}`), 5)
	legacyQueue.Enqueue(ctx, "notification", []byte(`{"msg":"task3"}`), 3)

	// Migrate
	err := m.MigratePendingTasks(ctx)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, 3, rabbitBroker.publishCalled)
	assert.Equal(t, int64(3), m.GetMetrics().TasksMigrated.Load())

	// Verify tasks marked as migrated
	for _, task := range legacyQueue.tasks {
		assert.True(t, task.Migrated, "task %s should be marked as migrated", task.ID)
	}
}

func TestMigrationManager_Rollback(t *testing.T) {
	cfg := &MigrationConfig{Mode: ModeMessaging}
	m := NewMigrationManager(cfg, nil)

	err := m.Rollback()
	require.NoError(t, err)
	assert.Equal(t, ModeRollback, m.Mode())
	assert.Equal(t, int64(1), m.GetMetrics().RollbackCount.Load())
}

func TestMigrationManager_AutoRollback(t *testing.T) {
	cfg := &MigrationConfig{
		Mode:           ModeMessaging,
		AutoRollback:   true,
		ErrorThreshold: 3,
	}
	m := NewMigrationManager(cfg, nil)

	// Record multiple errors to trigger rollback
	for i := 0; i < 5; i++ {
		m.recordError()
	}

	// Give time for async rollback
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, ModeRollback, m.Mode())
}

func TestMigrationManager_GetStatus(t *testing.T) {
	cfg := &MigrationConfig{Mode: ModeDualWrite}
	m := NewMigrationManager(cfg, nil)

	// Generate some activity
	m.metrics.TasksMigrated.Store(10)
	m.metrics.DualWriteCount.Store(100)
	m.metrics.DiscrepanciesFound.Store(2)
	m.metrics.ErrorCount.Store(5)

	status := m.GetStatus()
	assert.Equal(t, ModeDualWrite, status.Mode)
	assert.Equal(t, int64(10), status.TasksMigrated)
	assert.Equal(t, int64(100), status.DualWriteCount)
	assert.Equal(t, int64(2), status.DiscrepanciesFound)
	assert.Equal(t, int64(5), status.ErrorCount)
}

func TestMigrationManager_ShouldUseMessaging(t *testing.T) {
	t.Run("legacy mode returns false", func(t *testing.T) {
		m := NewMigrationManager(nil, nil)
		assert.False(t, m.ShouldUseMessaging())
	})

	t.Run("messaging mode returns true", func(t *testing.T) {
		cfg := &MigrationConfig{Mode: ModeMessaging}
		m := NewMigrationManager(cfg, nil)
		assert.True(t, m.ShouldUseMessaging())
	})

	t.Run("dual_write with 0% split returns false", func(t *testing.T) {
		cfg := &MigrationConfig{
			Mode:                 ModeDualWrite,
			ConsumerTrafficSplit: 0,
		}
		m := NewMigrationManager(cfg, nil)
		assert.False(t, m.ShouldUseMessaging())
	})

	t.Run("dual_write with 100% split returns true", func(t *testing.T) {
		cfg := &MigrationConfig{
			Mode:                 ModeDualWrite,
			ConsumerTrafficSplit: 100,
		}
		m := NewMigrationManager(cfg, nil)
		assert.True(t, m.ShouldUseMessaging())
	})
}

func TestDefaultMigrationConfig(t *testing.T) {
	cfg := DefaultMigrationConfig()
	assert.Equal(t, ModeLegacy, cfg.Mode)
	assert.True(t, cfg.VerifyConsistency)
	assert.True(t, cfg.LogDiscrepancies)
	assert.Equal(t, 0, cfg.ConsumerTrafficSplit)
	assert.False(t, cfg.AutoRollback)
	assert.Equal(t, 10, cfg.ErrorThreshold)
	assert.Equal(t, 5000, cfg.LatencyThreshold)
	assert.Equal(t, 10*time.Second, cfg.DualWriteTimeout)
}

func TestMigrationStatus_MarshalJSON(t *testing.T) {
	status := &MigrationStatus{
		Mode:               ModeDualWrite,
		TasksMigrated:      100,
		DualWriteCount:     1000,
		DiscrepanciesFound: 5,
		ErrorCount:         10,
		RollbackCount:      0,
		ErrorsPerMinute:    2,
		LastErrorTime:      time.Now(),
	}

	data, err := status.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), `"mode":"dual_write"`)
	assert.Contains(t, string(data), `"tasks_migrated":100`)
}
