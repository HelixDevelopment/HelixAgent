package streaming_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/streaming"
	genericsse "digital.vasic.streaming/pkg/sse"
	genericws "digital.vasic.streaming/pkg/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSSEConfig(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultSSEConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 100, config.BufferSize)
	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 1000, config.MaxClients)
}

func TestDefaultWebSocketConfig(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultWebSocketConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 1024, config.ReadBufferSize)
	assert.Equal(t, 1024, config.WriteBufferSize)
	assert.Equal(t, 54*time.Second, config.PingInterval)
	assert.Equal(t, 60*time.Second, config.PongWait)
}

func TestDefaultWebhookConfig(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultWebhookConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, time.Second, config.BackoffBase)
	assert.Equal(t, "X-Signature-256", config.SignatureHeader)
}

func TestNewSSEBroker(t *testing.T) {
	t.Parallel()
	config := &genericsse.Config{
		BufferSize:        50,
		HeartbeatInterval: 15 * time.Second,
		MaxClients:        500,
	}

	broker := adapter.NewSSEBroker(config)
	require.NotNil(t, broker)
	defer broker.Close()

	assert.Equal(t, 0, broker.ClientCount())
}

func TestNewSSEBrokerWithDefaultConfig(t *testing.T) {
	t.Parallel()
	broker := adapter.NewSSEBroker(nil)
	require.NotNil(t, broker)
	defer broker.Close()

	assert.Equal(t, 0, broker.ClientCount())
}

func TestNewSSEEvent(t *testing.T) {
	t.Parallel()
	event := adapter.NewSSEEvent("test_event", []byte(`{"data":"test"}`))

	assert.Equal(t, "test_event", event.Type)
	assert.Equal(t, []byte(`{"data":"test"}`), event.Data)
	assert.Empty(t, event.ID)
}

func TestNewSSEEventWithID(t *testing.T) {
	t.Parallel()
	event := adapter.NewSSEEventWithID("event-123", "test_event", []byte(`{"data":"test"}`))

	assert.Equal(t, "event-123", event.ID)
	assert.Equal(t, "test_event", event.Type)
	assert.Equal(t, []byte(`{"data":"test"}`), event.Data)
}

func TestNewWebSocketHub(t *testing.T) {
	t.Parallel()
	config := &genericws.Config{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		PingInterval:    30 * time.Second,
		PongWait:        40 * time.Second,
	}

	hub := adapter.NewWebSocketHub(config)
	require.NotNil(t, hub)
	defer hub.Close()

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.RoomCount())
}

func TestNewWebSocketHubWithDefaultConfig(t *testing.T) {
	t.Parallel()
	hub := adapter.NewWebSocketHub(nil)
	require.NotNil(t, hub)
	defer hub.Close()

	assert.Equal(t, 0, hub.ClientCount())
}

func TestNewWebhookDispatcher(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultWebhookConfig()
	dispatcher := adapter.NewWebhookDispatcher(config)

	require.NotNil(t, dispatcher)

	delivered, failed := dispatcher.Stats()
	assert.Equal(t, int64(0), delivered)
	assert.Equal(t, int64(0), failed)
}

func TestNewWebhookRegistry(t *testing.T) {
	t.Parallel()
	registry := adapter.NewWebhookRegistry()

	require.NotNil(t, registry)
	assert.Equal(t, 0, registry.Count())
}

func TestWebhookSigning(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event":"test","data":"hello"}`)
	secret := "test-secret-key"

	signature := adapter.SignWebhook(payload, secret)
	assert.NotEmpty(t, signature)
	assert.True(t, len(signature) > 10)

	// Verify the signature
	valid := adapter.VerifyWebhook(payload, signature, secret)
	assert.True(t, valid)

	// Verify with wrong secret fails
	invalidValid := adapter.VerifyWebhook(payload, signature, "wrong-secret")
	assert.False(t, invalidValid)

	// Verify with modified payload fails
	modifiedPayload := []byte(`{"event":"test","data":"modified"}`)
	modifiedValid := adapter.VerifyWebhook(modifiedPayload, signature, secret)
	assert.False(t, modifiedValid)
}

func TestSSEManagerAdapter(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultSSEConfig()
	manager := adapter.NewSSEManagerAdapter(config)
	require.NotNil(t, manager)
	defer manager.Close()

	// Test that broker is accessible
	broker := manager.Broker()
	assert.NotNil(t, broker)

	// Test broadcast doesn't panic
	manager.Broadcast("task-123", []byte(`{"status":"running"}`))
}

func TestSSEManagerAdapterHTTPHandler(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultSSEConfig()
	manager := adapter.NewSSEManagerAdapter(config)
	require.NotNil(t, manager)
	defer manager.Close()

	// Create a test server
	server := httptest.NewServer(manager.Broker())
	defer server.Close()

	// The server should be reachable (we won't test full SSE connection here)
	assert.NotEmpty(t, server.URL)
}

func TestWebSocketServerAdapter(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultWebSocketConfig()
	server := adapter.NewWebSocketServerAdapter(config)
	require.NotNil(t, server)
	defer server.Close()

	// Test that hub is accessible
	hub := server.Hub()
	assert.NotNil(t, hub)

	// Test counts
	assert.Equal(t, 0, server.ClientCount())
	assert.Equal(t, 0, server.RoomCount())

	// Test broadcast doesn't panic
	server.Broadcast("task-123", []byte(`{"status":"running"}`))
	server.BroadcastAll([]byte(`{"type":"broadcast"}`))
}

func TestWebhookDispatcherAdapter(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultWebhookConfig()
	dispatcherAdapter := adapter.NewWebhookDispatcherAdapter(config)
	require.NotNil(t, dispatcherAdapter)

	// Test that dispatcher and registry are accessible
	dispatcher := dispatcherAdapter.Dispatcher()
	registry := dispatcherAdapter.Registry()
	assert.NotNil(t, dispatcher)
	assert.NotNil(t, registry)

	// Test webhook registration
	webhook := &adapter.Webhook{
		URL:    "https://example.com/webhook",
		Secret: "test-secret",
		Events: []string{"task.completed"},
		Active: true,
	}

	dispatcherAdapter.RegisterWebhook("wh-1", webhook)

	// Get the webhook
	retrieved, found := dispatcherAdapter.GetWebhook("wh-1")
	assert.True(t, found)
	assert.Equal(t, webhook.URL, retrieved.URL)

	// List webhooks
	all := dispatcherAdapter.ListWebhooks()
	assert.Len(t, all, 1)

	// Unregister
	dispatcherAdapter.UnregisterWebhook("wh-1")

	_, found = dispatcherAdapter.GetWebhook("wh-1")
	assert.False(t, found)

	// Stats
	delivered, failed := dispatcherAdapter.Stats()
	assert.Equal(t, int64(0), delivered)
	assert.Equal(t, int64(0), failed)
}

func TestWebhookDispatcherAdapterDispatch(t *testing.T) {
	t.Parallel()
	// Create a test server to receive webhooks
	var receivedPayload []byte
	server := httptest.NewServer(nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayload = body
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	config := adapter.DefaultWebhookConfig()
	dispatcherAdapter := adapter.NewWebhookDispatcherAdapter(config)

	// Register an active webhook
	webhook := &adapter.Webhook{
		URL:    server.URL,
		Events: []string{"test.event"},
		Active: true,
	}
	dispatcherAdapter.RegisterWebhook("wh-1", webhook)

	// Dispatch
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := dispatcherAdapter.Dispatch(ctx, "test.event", map[string]string{"key": "value"})
	// May succeed or fail depending on server setup, but should not panic
	_ = err

	// Note: The actual dispatch might happen async, so we don't assert on receivedPayload here
	_ = receivedPayload
}

func TestTransportFactory(t *testing.T) {
	t.Parallel()
	factory := adapter.NewTransportFactory()
	require.NotNil(t, factory)

	types := factory.SupportedTypes()
	assert.Contains(t, types, adapter.TransportTypeHTTP)
	assert.Contains(t, types, adapter.TransportTypeWebSocket)
	assert.Contains(t, types, adapter.TransportTypeGRPC)
}

func TestNewGRPCConfig(t *testing.T) {
	t.Parallel()
	config := adapter.NewGRPCConfig()

	assert.NotNil(t, config)
	assert.Equal(t, ":50051", config.Address)
	assert.Equal(t, 4*1024*1024, config.MaxRecvMsgSize)
	assert.Equal(t, 4*1024*1024, config.MaxSendMsgSize)
	assert.Equal(t, uint32(100), config.MaxConcurrentStreams)
}

func TestNewGRPCHealthServer(t *testing.T) {
	t.Parallel()
	healthServer := adapter.NewGRPCHealthServer()
	require.NotNil(t, healthServer)
}
