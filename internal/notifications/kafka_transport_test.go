package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

func TestDefaultKafkaTransportConfig(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, TopicTaskNotifications, config.Topic)
	assert.True(t, config.Async)
	assert.Equal(t, 1000, config.BufferSize)
	assert.True(t, config.EnableAuditLog)
}

func TestNewKafkaTransport(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	assert.NotNil(t, transport)
	assert.NotNil(t, transport.eventCh)
	assert.NotNil(t, transport.stopCh)
}

func TestNewKafkaTransport_NilConfig(t *testing.T) {
	t.Parallel()
	transport := NewKafkaTransport(nil, nil, nil)

	assert.NotNil(t, transport)
	assert.NotNil(t, transport.config)
	assert.True(t, transport.config.Enabled)
}

func TestKafkaTransport_IsEnabled_NoHub(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	// Should be false when hub is nil
	assert.False(t, transport.IsEnabled())
}

func TestKafkaTransport_IsEnabled_Disabled(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	config.Enabled = false
	transport := NewKafkaTransport(nil, nil, config)

	assert.False(t, transport.IsEnabled())
}

func TestKafkaTransport_Send_NoHub(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "task.started",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"key": "value"},
	}

	// Should not error when hub is nil (just skips publish)
	err := transport.Send(context.Background(), notification)
	assert.NoError(t, err)
}

func TestKafkaTransport_Send_WithTask(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	task := &models.BackgroundTask{
		ID:       "task-1",
		TaskType: "test_type",
		TaskName: "Test Task",
		Status:   models.TaskStatusRunning,
		Progress: 50.0,
	}

	notification := &TaskNotification{
		TaskID:    task.ID,
		EventType: "task.progress",
		Timestamp: time.Now(),
		Task:      task,
	}

	err := transport.Send(context.Background(), notification)
	assert.NoError(t, err)
}

func TestKafkaTransport_PublishAuditEvent_NoHub(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	err := transport.PublishAuditEvent(context.Background(), "create", "task", map[string]interface{}{
		"task_id": "task-1",
	})
	assert.NoError(t, err)
}

func TestKafkaTransport_PublishAuditEvent_Disabled(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	config.EnableAuditLog = false
	transport := NewKafkaTransport(nil, nil, config)

	err := transport.PublishAuditEvent(context.Background(), "create", "task", nil)
	assert.NoError(t, err)
}

func TestKafkaTransport_StartStop(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	transport.Start()
	assert.True(t, transport.started)

	// Starting again should be a no-op
	transport.Start()
	assert.True(t, transport.started)

	transport.Stop()
}

func TestKafkaNotificationEvent_Fields(t *testing.T) {
	t.Parallel()
	event := &KafkaNotificationEvent{
		ID:            "event-1",
		TaskID:        "task-1",
		EventType:     "task.completed",
		Timestamp:     time.Now().UTC(),
		Data:          map[string]interface{}{"result": "success"},
		Source:        "helixagent.notifications",
		CorrelationID: "corr-123",
	}

	assert.Equal(t, "event-1", event.ID)
	assert.Equal(t, "task-1", event.TaskID)
	assert.Equal(t, "task.completed", event.EventType)
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, "success", event.Data["result"])
	assert.Equal(t, "helixagent.notifications", event.Source)
	assert.Equal(t, "corr-123", event.CorrelationID)
}

func TestGenerateNotificationEventID(t *testing.T) {
	t.Parallel()
	id1 := generateNotificationEventID()
	time.Sleep(time.Millisecond)
	id2 := generateNotificationEventID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestTopicNotificationConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "helixagent.notifications", TopicNotifications)
	assert.Equal(t, "helixagent.notifications.tasks", TopicTaskNotifications)
	assert.Equal(t, "helixagent.events.audit", TopicAuditLog)
}

func TestKafkaTransportConfig_Fields(t *testing.T) {
	t.Parallel()
	config := &KafkaTransportConfig{
		Enabled:        true,
		Topic:          "custom-topic",
		Async:          true,
		BufferSize:     500,
		EnableAuditLog: true,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "custom-topic", config.Topic)
	assert.True(t, config.Async)
	assert.Equal(t, 500, config.BufferSize)
	assert.True(t, config.EnableAuditLog)
}

func TestKafkaTransport_SyncMode(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	config.Async = false
	transport := NewKafkaTransport(nil, nil, config)

	assert.Nil(t, transport.eventCh)

	// Start should be a no-op in sync mode
	transport.Start()
	assert.False(t, transport.started)

	// Send should still work (just skips publish since no hub)
	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "test",
		Timestamp: time.Now(),
	}
	err := transport.Send(context.Background(), notification)
	assert.NoError(t, err)
}

func TestNewNotificationHubKafkaIntegration(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	integration := NewNotificationHubKafkaIntegration(nil, nil, nil, config)

	assert.NotNil(t, integration)
	assert.NotNil(t, integration.Transport())
}

func TestNotificationHubKafkaIntegration_StartStop(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	integration := NewNotificationHubKafkaIntegration(nil, nil, nil, config)

	integration.Start()
	assert.True(t, integration.transport.started)

	integration.Stop()
}

func TestNotificationHubKafkaIntegration_PublishNotification(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	integration := NewNotificationHubKafkaIntegration(nil, nil, nil, config)

	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "task.started",
		Timestamp: time.Now(),
	}

	err := integration.PublishNotification(context.Background(), notification)
	assert.NoError(t, err)
}

func TestNotificationHubKafkaIntegration_PublishAudit(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	integration := NewNotificationHubKafkaIntegration(nil, nil, nil, config)

	err := integration.PublishAudit(context.Background(), "update", "task", map[string]interface{}{
		"task_id": "task-1",
		"field":   "status",
	})
	assert.NoError(t, err)
}

func TestKafkaTransport_Stop_DoubleCloseNoPanic(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	transport.Start()
	assert.True(t, transport.started)

	// First stop should succeed normally
	assert.NotPanics(t, func() {
		transport.Stop()
	})

	// Second stop must not panic (double-close protection via sync.Once)
	assert.NotPanics(t, func() {
		transport.Stop()
	})
}

func TestKafkaTransport_Stop_ConcurrentStopNoPanic(t *testing.T) {
	t.Parallel()
	config := DefaultKafkaTransportConfig()
	transport := NewKafkaTransport(nil, nil, config)

	transport.Start()
	assert.True(t, transport.started)

	// 10 goroutines all calling Stop concurrently — must not panic
	const goroutines = 10
	done := make(chan struct{}, goroutines)

	assert.NotPanics(t, func() {
		for i := 0; i < goroutines; i++ {
			go func() {
				defer func() { done <- struct{}{} }()
				transport.Stop()
			}()
		}
		for i := 0; i < goroutines; i++ {
			<-done
		}
	})
}

func TestRandomNotificationString(t *testing.T) {
	t.Parallel()
	str1 := randomNotificationString(8)
	time.Sleep(time.Millisecond)
	str2 := randomNotificationString(8)

	require.Len(t, str1, 8)
	require.Len(t, str2, 8)
	assert.NotEqual(t, str1, str2)
}
