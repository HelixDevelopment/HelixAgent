package notifications

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for DefaultPollingConfig
func TestDefaultPollingConfig(t *testing.T) {
	t.Parallel()
	config := DefaultPollingConfig()

	assert.Equal(t, 100, config.MaxEventsPerTask)
	assert.Equal(t, 1000, config.MaxGlobalEvents)
	assert.Equal(t, 15*time.Minute, config.EventTTL)
	assert.Equal(t, 1*time.Minute, config.CleanupInterval)
}

// Tests for NewPollingStore
func TestNewPollingStore(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	t.Run("with default config", func(t *testing.T) {
		t.Parallel()
		store := NewPollingStore(nil, logger)
		require.NotNil(t, store)

		assert.NotNil(t, store.taskEvents)
		assert.NotNil(t, store.globalEvents)
		assert.Equal(t, logger, store.logger)

		_ = store.Stop()
	})

	t.Run("with custom config", func(t *testing.T) {
		t.Parallel()
		config := &PollingConfig{
			MaxEventsPerTask: 50,
			MaxGlobalEvents:  500,
			EventTTL:         5 * time.Minute,
			CleanupInterval:  30 * time.Second,
		}

		store := NewPollingStore(config, logger)
		require.NotNil(t, store)

		assert.Equal(t, 50, store.config.MaxEventsPerTask)
		assert.Equal(t, 500, store.config.MaxGlobalEvents)

		_ = store.Stop()
	})
}

// Tests for PollingStore Start/Stop
func TestPollingStore_StartStop(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)

	err := store.Start()
	assert.NoError(t, err)

	err = store.Stop()
	assert.NoError(t, err)
}

// Tests for StoreEvent
func TestPollingStore_StoreEvent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	t.Run("store single event", func(t *testing.T) {
		t.Parallel()
		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"progress": 50},
		}

		store.StoreEvent(notification)

		count := store.GetEventCount("task-1")
		assert.Equal(t, 1, count)
	})

	t.Run("store multiple events", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 5; i++ {
			notification := &TaskNotification{
				TaskID:    "task-2",
				EventType: "progress",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"progress": i * 20},
			}
			store.StoreEvent(notification)
		}

		count := store.GetEventCount("task-2")
		assert.Equal(t, 5, count)
	})

	t.Run("global event count increases", func(t *testing.T) {
		t.Parallel()
		initialCount := store.GetGlobalEventCount()

		notification := &TaskNotification{
			TaskID:    "task-3",
			EventType: "status",
			Timestamp: time.Now(),
		}
		store.StoreEvent(notification)

		newCount := store.GetGlobalEventCount()
		assert.Equal(t, initialCount+1, newCount)
	})
}

// Tests for event limit enforcement
func TestPollingStore_EventLimits(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	config := &PollingConfig{
		MaxEventsPerTask: 5,
		MaxGlobalEvents:  10,
		EventTTL:         15 * time.Minute,
		CleanupInterval:  1 * time.Minute,
	}

	store := NewPollingStore(config, logger)
	defer func() { _ = store.Stop() }()

	t.Run("task events trimmed at limit", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 10; i++ {
			notification := &TaskNotification{
				TaskID:    "task-1",
				EventType: "progress",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"num": i},
			}
			store.StoreEvent(notification)
		}

		count := store.GetEventCount("task-1")
		assert.Equal(t, 5, count) // Should be limited to MaxEventsPerTask
	})

	t.Run("global events trimmed at limit", func(t *testing.T) {
		t.Parallel()
		// Clear store
		store2 := NewPollingStore(config, logger)
		defer func() { _ = store2.Stop() }()

		for i := 0; i < 15; i++ {
			notification := &TaskNotification{
				TaskID:    "task-" + string(rune('a'+i)),
				EventType: "progress",
				Timestamp: time.Now(),
			}
			store2.StoreEvent(notification)
		}

		count := store2.GetGlobalEventCount()
		assert.Equal(t, 10, count) // Should be limited to MaxGlobalEvents
	})
}

// Tests for GetTaskEvents
func TestPollingStore_GetTaskEvents(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Store some events with different timestamps
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Data:      map[string]interface{}{"num": i},
		}
		store.StoreEvent(notification)
	}

	t.Run("get all events", func(t *testing.T) {
		t.Parallel()
		events := store.GetTaskEvents("task-1", nil, 0)
		assert.Len(t, events, 5)
	})

	t.Run("get events with limit", func(t *testing.T) {
		t.Parallel()
		events := store.GetTaskEvents("task-1", nil, 3)
		assert.Len(t, events, 3)
	})

	t.Run("get events since time", func(t *testing.T) {
		t.Parallel()
		since := baseTime.Add(2 * time.Second)
		events := store.GetTaskEvents("task-1", &since, 0)
		assert.Len(t, events, 2) // Only events after 2 seconds
	})

	t.Run("get events with since and limit", func(t *testing.T) {
		t.Parallel()
		since := baseTime.Add(1 * time.Second)
		events := store.GetTaskEvents("task-1", &since, 2)
		assert.Len(t, events, 2)
	})

	t.Run("get events for nonexistent task", func(t *testing.T) {
		t.Parallel()
		events := store.GetTaskEvents("nonexistent", nil, 0)
		assert.Nil(t, events)
	})
}

// Tests for GetGlobalEvents
func TestPollingStore_GetGlobalEvents(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Store some events with different timestamps
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		notification := &TaskNotification{
			TaskID:    "task-" + string(rune('a'+i)),
			EventType: "progress",
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
		}
		store.StoreEvent(notification)
	}

	t.Run("get all global events", func(t *testing.T) {
		t.Parallel()
		events := store.GetGlobalEvents(nil, 0)
		assert.Len(t, events, 5)
	})

	t.Run("get global events with limit", func(t *testing.T) {
		t.Parallel()
		events := store.GetGlobalEvents(nil, 3)
		assert.Len(t, events, 3)
	})

	t.Run("get global events since time", func(t *testing.T) {
		t.Parallel()
		since := baseTime.Add(2 * time.Second)
		events := store.GetGlobalEvents(&since, 0)
		assert.Len(t, events, 2)
	})

	t.Run("get global events with since and limit", func(t *testing.T) {
		t.Parallel()
		since := baseTime.Add(1 * time.Second)
		events := store.GetGlobalEvents(&since, 2)
		assert.Len(t, events, 2)
	})
}

// Tests for GetLatestTaskEvent.
//
// Each subtest constructs its OWN store — the previous shared-store
// pattern caused "no events" to see a non-nil event after "returns
// latest event" had stored 3 on the same store (race-debt BUGFIX #25).
func TestPollingStore_GetLatestTaskEvent(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	newStore := func(t *testing.T) *PollingStore {
		t.Helper()
		s := NewPollingStore(nil, logger)
		t.Cleanup(func() { _ = s.Stop() })
		return s
	}

	t.Run("no events", func(t *testing.T) {
		t.Parallel()
		store := newStore(t)
		event := store.GetLatestTaskEvent("task-1")
		assert.Nil(t, event)
	})

	t.Run("returns latest event", func(t *testing.T) {
		t.Parallel()
		store := newStore(t)
		for i := 0; i < 3; i++ {
			notification := &TaskNotification{
				TaskID:    "task-1",
				EventType: "progress",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"num": i},
			}
			store.StoreEvent(notification)
		}

		event := store.GetLatestTaskEvent("task-1")
		require.NotNil(t, event)

		num, ok := event.Data["num"].(int)
		require.True(t, ok)
		assert.Equal(t, 2, num) // Last event stored
	})
}

// Tests for GetEventCount
func TestPollingStore_GetEventCount(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	assert.Equal(t, 0, store.GetEventCount("task-1"))

	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "test",
		Timestamp: time.Now(),
	}
	store.StoreEvent(notification)

	assert.Equal(t, 1, store.GetEventCount("task-1"))
}

// Tests for GetGlobalEventCount
func TestPollingStore_GetGlobalEventCount(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	assert.Equal(t, 0, store.GetGlobalEventCount())

	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "test",
		Timestamp: time.Now(),
	}
	store.StoreEvent(notification)

	assert.Equal(t, 1, store.GetGlobalEventCount())
}

// Tests for ClearTaskEvents
func TestPollingStore_ClearTaskEvents(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Store some events
	for i := 0; i < 5; i++ {
		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "test",
			Timestamp: time.Now(),
		}
		store.StoreEvent(notification)
	}

	assert.Equal(t, 5, store.GetEventCount("task-1"))

	store.ClearTaskEvents("task-1")

	assert.Equal(t, 0, store.GetEventCount("task-1"))
}

// Tests for cleanup
func TestPollingStore_Cleanup(t *testing.T) {
	t.Parallel()
	logger := testLogger()

	config := &PollingConfig{
		MaxEventsPerTask: 100,
		MaxGlobalEvents:  1000,
		EventTTL:         100 * time.Millisecond, // Very short TTL for testing
		CleanupInterval:  50 * time.Millisecond,  // Short cleanup interval
	}

	store := NewPollingStore(config, logger)
	defer func() { _ = store.Stop() }()

	// Store an event
	notification := &TaskNotification{
		TaskID:    "task-1",
		EventType: "test",
		Timestamp: time.Now(),
	}
	store.StoreEvent(notification)

	assert.Equal(t, 1, store.GetEventCount("task-1"))

	// Wait for event to expire and cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Event should be cleaned up
	assert.Equal(t, 0, store.GetEventCount("task-1"))
}

// Tests for GetStats
func TestPollingStore_GetStats(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Initial stats
	stats := store.GetStats()
	assert.Equal(t, 0, stats["tasks_with_events"])
	assert.Equal(t, 0, stats["task_events_total"])
	assert.Equal(t, 0, stats["global_events"])

	// Add events
	for i := 0; i < 3; i++ {
		notification := &TaskNotification{
			TaskID:    "task-" + string(rune('a'+i)),
			EventType: "test",
			Timestamp: time.Now(),
		}
		store.StoreEvent(notification)
	}

	// Add more events for task-a
	for i := 0; i < 2; i++ {
		notification := &TaskNotification{
			TaskID:    "task-a",
			EventType: "test",
			Timestamp: time.Now(),
		}
		store.StoreEvent(notification)
	}

	stats = store.GetStats()
	assert.Equal(t, 3, stats["tasks_with_events"])
	assert.Equal(t, 5, stats["task_events_total"])
	assert.Equal(t, 5, stats["global_events"])
}

// Tests for Poll
func TestPollingStore_Poll(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Store some events
	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
		}
		store.StoreEvent(notification)
	}

	t.Run("poll task events", func(t *testing.T) {
		t.Parallel()
		req := &PollRequest{
			TaskID: "task-1",
			Limit:  5,
		}

		resp := store.Poll(req)

		assert.Len(t, resp.Events, 5)
		assert.Equal(t, 5, resp.Count)
		assert.True(t, resp.HasMore)
		assert.False(t, resp.Timestamp.IsZero())
	})

	t.Run("poll global events", func(t *testing.T) {
		t.Parallel()
		req := &PollRequest{
			Limit: 5,
		}

		resp := store.Poll(req)

		assert.Len(t, resp.Events, 5)
		assert.True(t, resp.HasMore)
	})

	t.Run("poll with since", func(t *testing.T) {
		t.Parallel()
		since := baseTime.Add(5 * time.Second)
		req := &PollRequest{
			TaskID: "task-1",
			Since:  &since,
			Limit:  100,
		}

		resp := store.Poll(req)

		assert.Len(t, resp.Events, 4) // Events 6, 7, 8, 9
		assert.False(t, resp.HasMore)
	})

	t.Run("poll with default limit", func(t *testing.T) {
		t.Parallel()
		req := &PollRequest{
			TaskID: "task-1",
		}

		resp := store.Poll(req)

		assert.Equal(t, 10, resp.Count) // All events returned with default limit of 100
	})
}

// Tests for PollRequest
func TestPollRequest(t *testing.T) {
	t.Parallel()
	since := time.Now()
	req := &PollRequest{
		TaskID: "task-1",
		Since:  &since,
		Limit:  50,
	}

	assert.Equal(t, "task-1", req.TaskID)
	assert.NotNil(t, req.Since)
	assert.Equal(t, 50, req.Limit)
}

// Tests for PollResponse
func TestPollResponse(t *testing.T) {
	t.Parallel()
	events := []*TaskNotification{
		{TaskID: "task-1", EventType: "test"},
		{TaskID: "task-1", EventType: "test"},
	}

	resp := &PollResponse{
		Events:    events,
		Count:     2,
		Timestamp: time.Now(),
		HasMore:   true,
	}

	assert.Len(t, resp.Events, 2)
	assert.Equal(t, 2, resp.Count)
	assert.True(t, resp.HasMore)
	assert.False(t, resp.Timestamp.IsZero())
}

// Tests for PollingSubscriber
func TestPollingSubscriber(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	t.Run("create new subscriber", func(t *testing.T) {
		t.Parallel()
		subscriber := NewPollingSubscriber("sub-1", "task-1", store)

		assert.Equal(t, "sub-1", subscriber.ID())
		assert.Equal(t, NotificationTypePolling, subscriber.Type())
		assert.True(t, subscriber.IsActive())
	})

	t.Run("notify subscriber stores event", func(t *testing.T) {
		t.Parallel()
		subscriber := NewPollingSubscriber("sub-1", "task-1", store)

		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: time.Now(),
		}

		err := subscriber.Notify(context.Background(), notification)
		assert.NoError(t, err)

		// Event should be in the store
		assert.GreaterOrEqual(t, store.GetEventCount("task-1"), 1)
	})

	t.Run("close subscriber", func(t *testing.T) {
		t.Parallel()
		subscriber := NewPollingSubscriber("sub-2", "task-2", store)

		err := subscriber.Close()
		assert.NoError(t, err)
		assert.False(t, subscriber.IsActive())
	})
}

// Tests for concurrent operations
func TestPollingStore_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	var wg sync.WaitGroup
	numGoroutines := 10
	numEvents := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				notification := &TaskNotification{
					TaskID:    "task-1",
					EventType: "test",
					Timestamp: time.Now(),
					Data:      map[string]interface{}{"goroutine": goroutineID, "event": j},
				}
				store.StoreEvent(notification)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				store.GetTaskEvents("task-1", nil, 10)
				store.GetGlobalEvents(nil, 10)
				store.GetEventCount("task-1")
				store.GetStats()
			}
		}()
	}

	wg.Wait()

	// Verify no panic and data is consistent
	count := store.GetEventCount("task-1")
	assert.Greater(t, count, 0)
}

// Tests for PollingConfig
func TestPollingConfig(t *testing.T) {
	t.Parallel()
	config := &PollingConfig{
		MaxEventsPerTask: 200,
		MaxGlobalEvents:  2000,
		EventTTL:         30 * time.Minute,
		CleanupInterval:  5 * time.Minute,
	}

	assert.Equal(t, 200, config.MaxEventsPerTask)
	assert.Equal(t, 2000, config.MaxGlobalEvents)
	assert.Equal(t, 30*time.Minute, config.EventTTL)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
}

// Tests for empty store edge cases
func TestPollingStore_EmptyStoreEdgeCases(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	t.Run("get events from empty store", func(t *testing.T) {
		t.Parallel()
		events := store.GetTaskEvents("nonexistent", nil, 10)
		assert.Nil(t, events)
	})

	t.Run("get global events from empty store", func(t *testing.T) {
		t.Parallel()
		events := store.GetGlobalEvents(nil, 10)
		assert.Nil(t, events)
	})

	t.Run("poll empty store", func(t *testing.T) {
		t.Parallel()
		req := &PollRequest{TaskID: "nonexistent"}
		resp := store.Poll(req)

		assert.Nil(t, resp.Events)
		assert.Equal(t, 0, resp.Count)
		assert.False(t, resp.HasMore)
	})

	t.Run("clear nonexistent task", func(t *testing.T) {
		t.Parallel()
		// Should not panic
		store.ClearTaskEvents("nonexistent")
	})

	t.Run("get latest from empty task", func(t *testing.T) {
		t.Parallel()
		event := store.GetLatestTaskEvent("nonexistent")
		assert.Nil(t, event)
	})
}

// Tests for event order preservation
func TestPollingStore_EventOrder(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	store := NewPollingStore(nil, logger)
	defer func() { _ = store.Stop() }()

	// Store events in order
	for i := 0; i < 5; i++ {
		notification := &TaskNotification{
			TaskID:    "task-1",
			EventType: "progress",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"order": i},
		}
		store.StoreEvent(notification)
		time.Sleep(time.Millisecond) // Ensure unique timestamps
	}

	events := store.GetTaskEvents("task-1", nil, 0)

	// Verify order is preserved
	for i, event := range events {
		order, ok := event.Data["order"].(int)
		require.True(t, ok)
		assert.Equal(t, i, order)
	}
}

// --- Benchmarks ---

func BenchmarkPollingStore_StoreEvent(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := &PollingConfig{
		MaxEventsPerTask: 1000,
		MaxGlobalEvents:  10000,
		EventTTL:         15 * time.Minute,
		CleanupInterval:  10 * time.Minute,
	}
	store := NewPollingStore(config, logger)
	defer func() { _ = store.Stop() }()

	notification := &TaskNotification{
		TaskID:    "bench-task-1",
		EventType: "task.progress",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"progress": 50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.StoreEvent(notification)
	}
}

func BenchmarkPollingStore_GetTaskEvents(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := &PollingConfig{
		MaxEventsPerTask: 1000,
		MaxGlobalEvents:  10000,
		EventTTL:         15 * time.Minute,
		CleanupInterval:  10 * time.Minute,
	}
	store := NewPollingStore(config, logger)
	defer func() { _ = store.Stop() }()

	// Pre-populate with 100 events
	for i := 0; i < 100; i++ {
		store.StoreEvent(&TaskNotification{
			TaskID:    "bench-task-1",
			EventType: "task.progress",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"progress": i},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.GetTaskEvents("bench-task-1", nil, 10)
	}
}

func BenchmarkPollingStore_GetLatestTaskEvent(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := &PollingConfig{
		MaxEventsPerTask: 1000,
		MaxGlobalEvents:  10000,
		EventTTL:         15 * time.Minute,
		CleanupInterval:  10 * time.Minute,
	}
	store := NewPollingStore(config, logger)
	defer func() { _ = store.Stop() }()

	// Pre-populate
	for i := 0; i < 50; i++ {
		store.StoreEvent(&TaskNotification{
			TaskID:    "bench-task-1",
			EventType: "task.progress",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"progress": i},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.GetLatestTaskEvent("bench-task-1")
	}
}
