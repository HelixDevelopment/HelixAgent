package background

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/models"
)

func TestTaskEventType_String(t *testing.T) {
	tests := []struct {
		eventType TaskEventType
		expected  string
	}{
		{TaskEventTypeCreated, "task.created"},
		{TaskEventTypeStarted, "task.started"},
		{TaskEventTypeProgress, "task.progress"},
		{TaskEventTypeCompleted, "task.completed"},
		{TaskEventTypeFailed, "task.failed"},
		{TaskEventTypeStuck, "task.stuck"},
		{TaskEventTypeCancelled, "task.cancelled"},
		{TaskEventTypeRetrying, "task.retrying"},
		{TaskEventTypeDeadLetter, "task.deadletter"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestTaskEventType_Topic(t *testing.T) {
	tests := []struct {
		eventType TaskEventType
		expected  string
	}{
		{TaskEventTypeCreated, TopicTaskCreated},
		{TaskEventTypeStarted, TopicTaskStarted},
		{TaskEventTypeProgress, TopicTaskProgress},
		{TaskEventTypeHeartbeat, TopicTaskProgress},
		{TaskEventTypeCompleted, TopicTaskCompleted},
		{TaskEventTypeFailed, TopicTaskFailed},
		{TaskEventTypeStuck, TopicTaskStuck},
		{TaskEventTypeCancelled, TopicTaskCancelled},
		{TaskEventTypeRetrying, TopicTaskRetrying},
		{TaskEventTypeDeadLetter, TopicTaskDeadLetter},
		{TaskEventType("unknown"), TopicTaskEvents},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.Topic())
		})
	}
}

func TestNewBackgroundTaskEvent(t *testing.T) {
	task := &models.BackgroundTask{
		ID:       "test-task-1",
		TaskType: "test_type",
		TaskName: "Test Task",
		Status:   models.TaskStatusRunning,
		Progress: 50.0,
	}

	workerID := "worker-1"
	task.WorkerID = &workerID
	progressMsg := "Processing..."
	task.ProgressMessage = &progressMsg
	correlationID := "corr-123"
	task.CorrelationID = &correlationID
	now := time.Now()
	task.StartedAt = &now
	task.RetryCount = 2

	event := NewBackgroundTaskEvent(TaskEventTypeProgress, task)

	assert.NotEmpty(t, event.EventID)
	assert.Equal(t, TaskEventTypeProgress, event.EventType)
	assert.Equal(t, "test-task-1", event.TaskID)
	assert.Equal(t, "test_type", event.TaskType)
	assert.Equal(t, "Test Task", event.TaskName)
	assert.Equal(t, models.TaskStatusRunning, event.Status)
	assert.Equal(t, 50.0, event.Progress)
	assert.Equal(t, "worker-1", event.WorkerID)
	assert.Equal(t, "Processing...", event.ProgressMessage)
	assert.Equal(t, "corr-123", event.CorrelationID)
	assert.Equal(t, 2, event.RetryCount)
	assert.NotZero(t, event.Timestamp)
	assert.NotNil(t, event.Metadata)
}

func TestNewBackgroundTaskEvent_WithError(t *testing.T) {
	task := &models.BackgroundTask{
		ID:       "test-task-1",
		TaskType: "test_type",
		TaskName: "Test Task",
		Status:   models.TaskStatusFailed,
	}
	lastError := "execution failed"
	task.LastError = &lastError

	event := NewBackgroundTaskEvent(TaskEventTypeFailed, task)

	assert.Equal(t, TaskEventTypeFailed, event.EventType)
	assert.Equal(t, "execution failed", event.Error)
}

func TestBackgroundTaskEvent_ToMessagingEvent(t *testing.T) {
	event := &BackgroundTaskEvent{
		EventID:       "event-1",
		EventType:     TaskEventTypeCompleted,
		TaskID:        "task-1",
		TaskType:      "test_type",
		TaskName:      "Test Task",
		Status:        models.TaskStatusCompleted,
		Progress:      100,
		Timestamp:     time.Now().UTC(),
		CorrelationID: "corr-123",
		TraceID:       "trace-456",
		Metadata:      map[string]interface{}{"key": "value"},
	}

	msgEvent := event.ToMessagingEvent()

	assert.Equal(t, "event-1", msgEvent.ID)
	assert.Equal(t, messaging.EventType("task.completed"), msgEvent.Type)
	assert.Equal(t, "helixagent.background", msgEvent.Source)
	assert.Equal(t, "task-1", msgEvent.Subject)
	assert.Equal(t, "corr-123", msgEvent.CorrelationID)
	assert.Equal(t, "trace-456", msgEvent.TraceID)
	assert.NotEmpty(t, msgEvent.Data)

	// Verify data can be unmarshaled
	var decoded BackgroundTaskEvent
	err := json.Unmarshal(msgEvent.Data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, event.TaskID, decoded.TaskID)
	assert.Equal(t, event.EventType, decoded.EventType)
}

func TestDefaultTaskEventPublisherConfig(t *testing.T) {
	config := DefaultTaskEventPublisherConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.AsyncPublish)
	assert.Equal(t, 1000, config.BufferSize)
}

// MockMessagingHub provides a mock for testing.
type MockMessagingHub struct {
	events      *safe.Slice[*messaging.Event]
	shouldError bool
}

func newMockMessagingHub() *MockMessagingHub {
	return &MockMessagingHub{events: safe.NewSlice[*messaging.Event]()}
}

func (m *MockMessagingHub) PublishEvent(ctx context.Context, topic string, event *messaging.Event) error {
	if m.shouldError {
		return messaging.NewBrokerError(messaging.ErrCodePublishFailed, "mock error", nil)
	}
	if m.events == nil {
		m.events = safe.NewSlice[*messaging.Event]()
	}
	m.events.Append(event)
	return nil
}

func (m *MockMessagingHub) GetPublishedEvents() []*messaging.Event {
	if m.events == nil {
		return nil
	}
	return m.events.Snapshot()
}

func TestTaskEventPublisher_Publish_Disabled(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, nil, config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
	}
	event := NewBackgroundTaskEvent(TaskEventTypeCreated, task)

	err := publisher.Publish(context.Background(), event)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_SetEnabled(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      true,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, nil, config)

	assert.True(t, publisher.IsEnabled())

	publisher.SetEnabled(false)
	assert.False(t, publisher.IsEnabled())

	publisher.SetEnabled(true)
	assert.True(t, publisher.IsEnabled())
}

func TestTaskEventPublisher_PublishNilEvent(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      true,
		AsyncPublish: false,
	}

	// Create with nil hub - will skip publish
	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	err := publisher.Publish(context.Background(), nil)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskCreated(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false, // Disabled for this test
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusPending,
	}

	err := publisher.PublishTaskCreated(context.Background(), task)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskStarted(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusRunning,
	}

	err := publisher.PublishTaskStarted(context.Background(), task)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskProgress(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Progress: 50,
	}

	err := publisher.PublishTaskProgress(context.Background(), task)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskCompleted(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusCompleted,
		Progress: 100,
	}

	err := publisher.PublishTaskCompleted(context.Background(), task, map[string]string{"result": "success"})
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskFailed(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusFailed,
	}

	err := publisher.PublishTaskFailed(context.Background(), task, assert.AnError)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskStuck(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusStuck,
	}

	err := publisher.PublishTaskStuck(context.Background(), task, "no heartbeat")
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskCancelled(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusCancelled,
	}

	err := publisher.PublishTaskCancelled(context.Background(), task)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskRetrying(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:         "test-1",
		TaskType:   "test_type",
		TaskName:   "Test",
		RetryCount: 1,
	}

	err := publisher.PublishTaskRetrying(context.Background(), task, 5*time.Second)
	assert.NoError(t, err)
}

func TestTaskEventPublisher_PublishTaskDeadLetter(t *testing.T) {
	config := &TaskEventPublisherConfig{
		Enabled:      false,
		AsyncPublish: false,
	}

	publisher := NewTaskEventPublisher(nil, logrus.New(), config)

	task := &models.BackgroundTask{
		ID:       "test-1",
		TaskType: "test_type",
		TaskName: "Test",
		Status:   models.TaskStatusDeadLetter,
	}

	err := publisher.PublishTaskDeadLetter(context.Background(), task, "max retries exceeded")
	assert.NoError(t, err)
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	time.Sleep(time.Millisecond)
	id2 := generateEventID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, "helixagent.events.tasks", TopicTaskEvents)
	assert.Equal(t, "helixagent.events.tasks.created", TopicTaskCreated)
	assert.Equal(t, "helixagent.events.tasks.started", TopicTaskStarted)
	assert.Equal(t, "helixagent.events.tasks.progress", TopicTaskProgress)
	assert.Equal(t, "helixagent.events.tasks.completed", TopicTaskCompleted)
	assert.Equal(t, "helixagent.events.tasks.failed", TopicTaskFailed)
	assert.Equal(t, "helixagent.events.tasks.stuck", TopicTaskStuck)
	assert.Equal(t, "helixagent.events.tasks.cancelled", TopicTaskCancelled)
	assert.Equal(t, "helixagent.events.tasks.retrying", TopicTaskRetrying)
	assert.Equal(t, "helixagent.events.tasks.deadletter", TopicTaskDeadLetter)
}

// --- Benchmarks ---

func BenchmarkTaskEventType_String(b *testing.B) {
	eventTypes := []TaskEventType{
		TaskEventTypeCreated,
		TaskEventTypeStarted,
		TaskEventTypeProgress,
		TaskEventTypeCompleted,
		TaskEventTypeFailed,
		TaskEventTypeStuck,
		TaskEventTypeCancelled,
		TaskEventTypeRetrying,
		TaskEventTypeDeadLetter,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		et := eventTypes[i%len(eventTypes)]
		_ = et.String()
	}
}

func BenchmarkNewBackgroundTaskEvent(b *testing.B) {
	task := &models.BackgroundTask{
		ID:       "bench-task-1",
		TaskType: "benchmark_type",
		TaskName: "Bench Task",
		Status:   models.TaskStatusRunning,
		Progress: 50.0,
	}
	workerID := "worker-bench"
	task.WorkerID = &workerID
	progressMsg := "Processing..."
	task.ProgressMessage = &progressMsg

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBackgroundTaskEvent(TaskEventTypeProgress, task)
	}
}

func BenchmarkGenerateEventID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateEventID()
	}
}

func BenchmarkBackgroundTaskEvent_ToMessagingEvent(b *testing.B) {
	event := &BackgroundTaskEvent{
		EventID:       "event-bench",
		EventType:     TaskEventTypeCompleted,
		TaskID:        "task-bench",
		TaskType:      "bench_type",
		TaskName:      "Bench Task",
		Status:        models.TaskStatusCompleted,
		Progress:      100,
		Timestamp:     time.Now().UTC(),
		CorrelationID: "corr-bench",
		Metadata:      map[string]interface{}{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.ToMessagingEvent()
	}
}
