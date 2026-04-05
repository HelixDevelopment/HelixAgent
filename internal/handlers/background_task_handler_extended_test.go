package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewBackgroundTaskHandler_Extended tests handler creation with all nil dependencies
func TestNewBackgroundTaskHandler_Extended(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewBackgroundTaskHandler(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger,
	)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.logger)
	assert.NotNil(t, handler.cliRenderer)
}

// TestBackgroundTaskHandler_CreateTask_InvalidJSON tests create with invalid JSON
func TestBackgroundTaskHandler_CreateTask_InvalidJSON(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/tasks", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateTask(c)

	// With nil services the handler returns 503 before parsing the body
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestBackgroundTaskHandler_CreateTask_MissingRequiredFields tests create with missing fields
func TestBackgroundTaskHandler_CreateTask_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	reqBody := map[string]interface{}{
		"task_type": "test",
		// Missing task_name
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/tasks", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateTask(c)

	// With nil services the handler returns 503 before parsing the body
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestBackgroundTaskHandler_CreateTask_EmptyBody tests create with empty body
func TestBackgroundTaskHandler_CreateTask_EmptyBody(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/tasks", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateTask(c)

	// With nil services the handler returns 503 before parsing the body
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestBackgroundTaskHandler_GetTask_NilRepository tests get with nil repository
func TestBackgroundTaskHandler_GetTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id", nil)

	// This will panic with nil repository, so we recover
	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.GetTask(c)
}

// TestBackgroundTaskHandler_GetTaskStatus_NilRepository tests status with nil repository
func TestBackgroundTaskHandler_GetTaskStatus_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/status", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.GetTaskStatus(c)
}

// TestBackgroundTaskHandler_GetTaskLogs_NilRepository tests logs with nil repository
func TestBackgroundTaskHandler_GetTaskLogs_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/logs", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.GetTaskLogs(c)
}

// TestBackgroundTaskHandler_GetTaskResources_NilRepository tests resources with nil repository
func TestBackgroundTaskHandler_GetTaskResources_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/resources", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.GetTaskResources(c)
}

// TestBackgroundTaskHandler_PauseTask_NilRepository tests pause with nil repository
func TestBackgroundTaskHandler_PauseTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("POST", "/v1/tasks/test-task-id/pause", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.PauseTask(c)
}

// TestBackgroundTaskHandler_ResumeTask_NilRepository tests resume with nil repository
func TestBackgroundTaskHandler_ResumeTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("POST", "/v1/tasks/test-task-id/resume", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.ResumeTask(c)
}

// TestBackgroundTaskHandler_CancelTask_NilRepository tests cancel with nil repository
func TestBackgroundTaskHandler_CancelTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("POST", "/v1/tasks/test-task-id/cancel", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.CancelTask(c)
}

// TestBackgroundTaskHandler_DeleteTask_NilRepository tests delete with nil repository
func TestBackgroundTaskHandler_DeleteTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("DELETE", "/v1/tasks/test-task-id", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.DeleteTask(c)
}

// TestBackgroundTaskHandler_ListTasks_NilRepository tests list with nil repository
func TestBackgroundTaskHandler_ListTasks_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/tasks", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.ListTasks(c)
}

// TestBackgroundTaskHandler_GetQueueStats_NilQueue tests queue stats with nil queue
func TestBackgroundTaskHandler_GetQueueStats_NilQueue(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/tasks/queue/stats", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil queue")
		}
	}()

	handler.GetQueueStats(c)
}

// TestBackgroundTaskHandler_PollEvents_NilPollingStore tests poll events with nil store
func TestBackgroundTaskHandler_PollEvents_NilPollingStore(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/poll", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil polling store")
		}
	}()

	handler.PollEvents(c)
}

// TestBackgroundTaskHandler_RegisterWebhook_NilDispatcher tests webhook registration
func TestBackgroundTaskHandler_RegisterWebhook_NilDispatcher(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	reqBody := map[string]interface{}{
		"task_id": "test-task-id",
		"url":     "https://example.com/webhook",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/webhooks", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil webhook dispatcher")
		}
	}()

	handler.RegisterWebhook(c)
}

// TestBackgroundTaskHandler_ListWebhooks_NilDispatcher tests list webhooks
func TestBackgroundTaskHandler_ListWebhooks_NilDispatcher(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/webhooks", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil webhook dispatcher")
		}
	}()

	handler.ListWebhooks(c)
}

// TestBackgroundTaskHandler_DeleteWebhook_NilDispatcher tests delete webhook
func TestBackgroundTaskHandler_DeleteWebhook_NilDispatcher(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: "test-task-id"},
		{Key: "webhook_id", Value: "webhook-123"},
	}
	c.Request = httptest.NewRequest("DELETE", "/v1/tasks/test-task-id/webhooks/webhook-123", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil webhook dispatcher")
		}
	}()

	handler.DeleteWebhook(c)
}

// TestBackgroundTaskHandler_AnalyzeTask_NilRepository tests analyze task
func TestBackgroundTaskHandler_AnalyzeTask_NilRepository(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}
	c.Request = httptest.NewRequest("GET", "/v1/tasks/test-task-id/analyze", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil repository")
		}
	}()

	handler.AnalyzeTask(c)
}

// TestBackgroundTaskHandler_RegisterRoutes tests route registration
func TestBackgroundTaskHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewBackgroundTaskHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, logger)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	routes := router.Routes()

	// Check that main routes are registered
	expectedRoutes := []string{
		"/v1/tasks",
		"/v1/tasks/:id",
		"/v1/tasks/:id/status",
		"/v1/tasks/:id/logs",
	}

	routePaths := make(map[string]bool)
	for _, route := range routes {
		routePaths[route.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routePaths[expected], "Route %s should be registered", expected)
	}
}

// TestCreateTaskRequest_Struct tests request struct fields
func TestCreateTaskRequest_Struct(t *testing.T) {
	t.Parallel()
	now := time.Now()
	later := now.Add(time.Hour)

	req := CreateTaskRequest{
		TaskType:         "test_type",
		TaskName:         "Test Task",
		CorrelationID:    "corr-123",
		ParentTaskID:     "parent-456",
		Priority:         "high",
		ScheduledAt:      &now,
		Deadline:         &later,
		RequiredCPUCores: 2,
		RequiredMemoryMB: 512,
		Payload: map[string]interface{}{
			"key": "value",
		},
		Config: &TaskConfigRequest{
			TimeoutSeconds:    300,
			MaxRetries:        5,
			RetryDelaySeconds: 10,
			Endless:           false,
			AllowPause:        true,
			AllowCancel:       true,
		},
		NotificationConfig: &NotificationConfigRequest{
			EnableSSE:       true,
			EnableWebSocket: true,
			EnablePolling:   true,
		},
	}

	assert.Equal(t, "test_type", req.TaskType)
	assert.Equal(t, "Test Task", req.TaskName)
	assert.Equal(t, "corr-123", req.CorrelationID)
	assert.Equal(t, "parent-456", req.ParentTaskID)
	assert.Equal(t, "high", req.Priority)
	assert.NotNil(t, req.ScheduledAt)
	assert.NotNil(t, req.Deadline)
	assert.Equal(t, 2, req.RequiredCPUCores)
	assert.Equal(t, 512, req.RequiredMemoryMB)
	assert.NotNil(t, req.Config)
	assert.NotNil(t, req.NotificationConfig)
}

// TestTaskConfigRequest_Struct tests config struct fields
func TestTaskConfigRequest_Struct(t *testing.T) {
	t.Parallel()
	config := TaskConfigRequest{
		TimeoutSeconds:        600,
		MaxRetries:            3,
		RetryDelaySeconds:     30,
		Endless:               true,
		AllowPause:            true,
		AllowCancel:           true,
		StuckThresholdSecs:    120,
		HeartbeatIntervalSecs: 10,
		Tags:                  []string{"tag1", "tag2"},
	}

	assert.Equal(t, 600, config.TimeoutSeconds)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 30, config.RetryDelaySeconds)
	assert.True(t, config.Endless)
	assert.True(t, config.AllowPause)
	assert.True(t, config.AllowCancel)
	assert.Equal(t, 120, config.StuckThresholdSecs)
	assert.Equal(t, 10, config.HeartbeatIntervalSecs)
	assert.Len(t, config.Tags, 2)
}

// TestNotificationConfigRequest_Struct tests notification config fields
func TestNotificationConfigRequest_Struct(t *testing.T) {
	t.Parallel()
	config := NotificationConfigRequest{
		EnableSSE:       true,
		EnableWebSocket: true,
		EnablePolling:   false,
		Webhooks: []WebhookConfigRequest{
			{
				URL:    "https://example.com/webhook",
				Secret: "secret123",
				Events: []string{"completed", "failed"},
				Headers: map[string]string{
					"X-Custom": "value",
				},
			},
		},
	}

	assert.True(t, config.EnableSSE)
	assert.True(t, config.EnableWebSocket)
	assert.False(t, config.EnablePolling)
	assert.Len(t, config.Webhooks, 1)
	assert.Equal(t, "https://example.com/webhook", config.Webhooks[0].URL)
}

// TestWebhookConfigRequest_Struct tests webhook config fields
func TestWebhookConfigRequest_Struct(t *testing.T) {
	t.Parallel()
	config := WebhookConfigRequest{
		URL:    "https://example.com/hook",
		Secret: "webhook_secret",
		Events: []string{"started", "completed"},
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "header",
		},
	}

	assert.Equal(t, "https://example.com/hook", config.URL)
	assert.Equal(t, "webhook_secret", config.Secret)
	assert.Len(t, config.Events, 2)
	assert.Len(t, config.Headers, 2)
}

// TestNilIfEmpty_Extended tests the nilIfEmpty helper function with various inputs
func TestNilIfEmpty_Extended(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input    string
		expected *string
	}{
		{"", nil},
		{"value", strPtr("value")},
		{"  ", strPtr("  ")}, // Spaces are not empty
	}

	for _, tc := range testCases {
		result := nilIfEmpty(tc.input)
		if tc.expected == nil {
			assert.Nil(t, result)
		} else {
			assert.NotNil(t, result)
			assert.Equal(t, *tc.expected, *result)
		}
	}
}

func strPtr(s string) *string {
	return &s
}
