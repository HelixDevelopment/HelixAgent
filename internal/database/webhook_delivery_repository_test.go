package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helper Functions for WebhookDelivery Repository
// =============================================================================

func setupWebhookDeliveryTestDB(t *testing.T) (*pgxpool.Pool, *WebhookDeliveryRepository) {
	ctx := context.Background()
	connString := getTestDBConnString()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	repo := NewWebhookDeliveryRepository(pool)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupWebhookDeliveryTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM webhook_deliveries WHERE webhook_url LIKE '%test%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup webhook_deliveries: %v", err)
	}
}

func createTestWebhookDelivery() *WebhookDelivery {
	return &WebhookDelivery{
		WebhookURL: "https://test-webhook.example.com/callback",
		EventType:  "task.completed",
		Payload:    json.RawMessage(`{"task_id": "test-123", "status": "completed"}`),
		Status:     WebhookStatusPending,
		Attempts:   0,
	}
}

// =============================================================================
// Unit Tests (no database required)
// =============================================================================

func TestNewWebhookDeliveryRepository(t *testing.T) {
	repo := NewWebhookDeliveryRepository(nil)
	assert.NotNil(t, repo)
}

func TestWebhookDeliveryStatus_Constants(t *testing.T) {
	assert.Equal(t, WebhookDeliveryStatus("pending"), WebhookStatusPending)
	assert.Equal(t, WebhookDeliveryStatus("delivered"), WebhookStatusDelivered)
	assert.Equal(t, WebhookDeliveryStatus("failed"), WebhookStatusFailed)
	assert.Equal(t, WebhookDeliveryStatus("retrying"), WebhookStatusRetrying)
}

func TestWebhookDelivery_Fields(t *testing.T) {
	now := time.Now()
	taskID := "task-123"
	errorMsg := "Connection timeout"
	responseCode := 500

	delivery := &WebhookDelivery{
		ID:            "delivery-id",
		TaskID:        &taskID,
		WebhookURL:    "https://example.com/webhook",
		EventType:     "task.failed",
		Payload:       json.RawMessage(`{"error": "test"}`),
		Status:        WebhookStatusFailed,
		Attempts:      3,
		LastAttemptAt: &now,
		LastError:     &errorMsg,
		ResponseCode:  &responseCode,
		CreatedAt:     now,
	}

	assert.Equal(t, "delivery-id", delivery.ID)
	assert.Equal(t, "task-123", *delivery.TaskID)
	assert.Equal(t, "https://example.com/webhook", delivery.WebhookURL)
	assert.Equal(t, "task.failed", delivery.EventType)
	assert.Equal(t, WebhookStatusFailed, delivery.Status)
	assert.Equal(t, 3, delivery.Attempts)
	assert.Equal(t, "Connection timeout", *delivery.LastError)
	assert.Equal(t, 500, *delivery.ResponseCode)
}

func TestWebhookDelivery_JSONMarshal(t *testing.T) {
	delivery := &WebhookDelivery{
		ID:         "test-id",
		WebhookURL: "https://example.com/hook",
		EventType:  "test.event",
		Payload:    json.RawMessage(`{"data": "test"}`),
		Status:     WebhookStatusPending,
	}

	data, err := json.Marshal(delivery)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-id")
	assert.Contains(t, string(data), "test.event")
	assert.Contains(t, string(data), "pending")
}

func TestWebhookDelivery_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "test-id",
		"webhook_url": "https://example.com/hook",
		"event_type": "task.started",
		"payload": {"key": "value"},
		"status": "pending",
		"attempts": 0
	}`

	var delivery WebhookDelivery
	err := json.Unmarshal([]byte(jsonData), &delivery)
	require.NoError(t, err)
	assert.Equal(t, "test-id", delivery.ID)
	assert.Equal(t, "task.started", delivery.EventType)
	assert.Equal(t, WebhookStatusPending, delivery.Status)
}

func TestWebhookDeliveryFilter_Empty(t *testing.T) {
	filter := WebhookDeliveryFilter{}
	assert.Empty(t, filter.TaskID)
	assert.Empty(t, filter.Status)
	assert.Empty(t, filter.EventType)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestWebhookDeliveryFilter_WithValues(t *testing.T) {
	filter := WebhookDeliveryFilter{
		TaskID:    "task-123",
		Status:    WebhookStatusPending,
		EventType: "task.completed",
		Limit:     50,
		Offset:    10,
	}
	assert.Equal(t, "task-123", filter.TaskID)
	assert.Equal(t, WebhookStatusPending, filter.Status)
	assert.Equal(t, "task.completed", filter.EventType)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 10, filter.Offset)
}

func TestWebhookDelivery_NilOptionalFields(t *testing.T) {
	delivery := &WebhookDelivery{
		ID:         "test-id",
		WebhookURL: "https://example.com/hook",
		EventType:  "test",
		Payload:    json.RawMessage(`{}`),
		Status:     WebhookStatusPending,
	}

	assert.Nil(t, delivery.TaskID)
	assert.Nil(t, delivery.LastAttemptAt)
	assert.Nil(t, delivery.LastError)
	assert.Nil(t, delivery.ResponseCode)
	assert.Nil(t, delivery.DeliveredAt)
}

// =============================================================================
// Integration Tests (require database)
// =============================================================================

func TestWebhookDeliveryRepository_Create(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()

	err := repo.Create(ctx, delivery)
	require.NoError(t, err)
	assert.NotEmpty(t, delivery.ID)
	assert.False(t, delivery.CreatedAt.IsZero())
}

func TestWebhookDeliveryRepository_GetByID(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, delivery.ID, retrieved.ID)
	assert.Equal(t, delivery.WebhookURL, retrieved.WebhookURL)
	assert.Equal(t, delivery.EventType, retrieved.EventType)
}

func TestWebhookDeliveryRepository_Update(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	delivery.Status = WebhookStatusDelivered
	delivery.Attempts = 1
	now := time.Now()
	delivery.DeliveredAt = &now

	err = repo.Update(ctx, delivery)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, WebhookStatusDelivered, retrieved.Status)
	assert.Equal(t, 1, retrieved.Attempts)
}

func TestWebhookDeliveryRepository_Delete(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	err = repo.Delete(ctx, delivery.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, delivery.ID)
	assert.Error(t, err)
}

func TestWebhookDeliveryRepository_List(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		delivery := createTestWebhookDelivery()
		err := repo.Create(ctx, delivery)
		require.NoError(t, err)
	}

	filter := WebhookDeliveryFilter{Limit: 3}
	deliveries, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(deliveries), 3)
}

func TestWebhookDeliveryRepository_MarkDelivered(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	err = repo.MarkDelivered(ctx, delivery.ID, 200)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, WebhookStatusDelivered, retrieved.Status)
	assert.NotNil(t, retrieved.ResponseCode)
	assert.Equal(t, 200, *retrieved.ResponseCode)
}

func TestWebhookDeliveryRepository_MarkFailed(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	responseCode := 500
	err = repo.MarkFailed(ctx, delivery.ID, "Server error", &responseCode)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, WebhookStatusFailed, retrieved.Status)
	assert.NotNil(t, retrieved.LastError)
	assert.Contains(t, *retrieved.LastError, "Server error")
}

func TestWebhookDeliveryRepository_MarkRetrying(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	responseCode := 503
	err = repo.MarkRetrying(ctx, delivery.ID, "Service unavailable", &responseCode)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, WebhookStatusRetrying, retrieved.Status)
	assert.Equal(t, 1, retrieved.Attempts)
}

func TestWebhookDeliveryRepository_GetPendingDeliveries(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	pending, err := repo.GetPendingDeliveries(ctx, 5, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 1)
}

func TestWebhookDeliveryRepository_Count(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()

	initialCount, err := repo.Count(ctx)
	require.NoError(t, err)

	delivery := createTestWebhookDelivery()
	err = repo.Create(ctx, delivery)
	require.NoError(t, err)

	newCount, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCount+1, newCount)
}

func TestWebhookDeliveryRepository_CountByStatus(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()

	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	counts, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.NotNil(t, counts)
}

func TestWebhookDeliveryRepository_GetDeliveryStats(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()

	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	since := time.Now().Add(-1 * time.Hour)
	stats, err := repo.GetDeliveryStats(ctx, since)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total")
	assert.Contains(t, stats, "success_rate")
}

func TestWebhookDeliveryRepository_DeleteOldDeliveries(t *testing.T) {
	pool, repo := setupWebhookDeliveryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupWebhookDeliveryTestDB(t, pool)

	ctx := context.Background()

	// Create and mark as delivered
	delivery := createTestWebhookDelivery()
	err := repo.Create(ctx, delivery)
	require.NoError(t, err)

	err = repo.MarkDelivered(ctx, delivery.ID, 200)
	require.NoError(t, err)

	// Try to delete - should not delete because delivered_at is recent
	cutoff := time.Now().Add(-1 * time.Hour)
	_, err = repo.DeleteOldDeliveries(ctx, cutoff)
	require.NoError(t, err)
}
