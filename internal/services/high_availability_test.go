package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFederationTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// MockFederatedProtocol implements FederatedProtocol for testing
type MockFederatedProtocol struct {
	name         string
	capabilities map[string]interface{}
	handler      func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error)
}

func (m *MockFederatedProtocol) Name() string {
	return m.name
}

func (m *MockFederatedProtocol) HandleFederatedRequest(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
	if m.handler != nil {
		return m.handler(ctx, request)
	}
	return &FederatedResponse{
		ID:        request.ID,
		Success:   true,
		Data:      request.Data,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockFederatedProtocol) PublishEvent(ctx context.Context, event *ProtocolEvent) error {
	return nil
}

func (m *MockFederatedProtocol) GetCapabilities() map[string]interface{} {
	return m.capabilities
}

func TestNewProtocolFederation(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	require.NotNil(t, federation)
	assert.NotNil(t, federation.protocols)
	assert.NotNil(t, federation.translators)
	assert.NotNil(t, federation.eventBus)
	assert.NotNil(t, federation.subscriptions)
}

func TestProtocolFederation_RegisterProtocol(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("register new protocol", func(t *testing.T) {
		protocol := &MockFederatedProtocol{
			name:         "test-protocol",
			capabilities: map[string]interface{}{"test": true},
		}

		err := federation.RegisterProtocol(protocol)
		require.NoError(t, err)
	})

	t.Run("register duplicate protocol", func(t *testing.T) {
		protocol := &MockFederatedProtocol{
			name: "duplicate",
		}

		err := federation.RegisterProtocol(protocol)
		require.NoError(t, err)

		err = federation.RegisterProtocol(protocol)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestProtocolFederation_UnregisterProtocol(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("unregister existing protocol", func(t *testing.T) {
		protocol := &MockFederatedProtocol{name: "to-unregister"}
		federation.RegisterProtocol(protocol)

		err := federation.UnregisterProtocol("to-unregister")
		require.NoError(t, err)

		protocols := federation.GetRegisteredProtocols()
		assert.NotContains(t, protocols, "to-unregister")
	})

	t.Run("unregister non-existent protocol", func(t *testing.T) {
		err := federation.UnregisterProtocol("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestProtocolFederation_SendFederatedRequest(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	protocol := &MockFederatedProtocol{
		name: "target-protocol",
		handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
			return &FederatedResponse{
				ID:      request.ID,
				Success: true,
				Data:    map[string]interface{}{"result": "success"},
			}, nil
		},
	}
	federation.RegisterProtocol(protocol)

	t.Run("send request to registered protocol", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Source: "source-protocol",
			Target: "target-protocol",
			Action: "test-action",
			Data:   map[string]interface{}{"key": "value"},
		}

		response, err := federation.SendFederatedRequest(context.Background(), request)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.NotNil(t, response.Data)
	})

	t.Run("send request to non-existent protocol", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Source: "source",
			Target: "non-existent",
			Action: "test",
		}

		response, err := federation.SendFederatedRequest(context.Background(), request)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestProtocolFederation_GetRegisteredProtocols(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("empty federation", func(t *testing.T) {
		protocols := federation.GetRegisteredProtocols()
		assert.Empty(t, protocols)
	})

	t.Run("with registered protocols", func(t *testing.T) {
		federation.RegisterProtocol(&MockFederatedProtocol{name: "protocol1"})
		federation.RegisterProtocol(&MockFederatedProtocol{name: "protocol2"})
		federation.RegisterProtocol(&MockFederatedProtocol{name: "protocol3"})

		protocols := federation.GetRegisteredProtocols()
		assert.Len(t, protocols, 3)
		assert.Contains(t, protocols, "protocol1")
		assert.Contains(t, protocols, "protocol2")
		assert.Contains(t, protocols, "protocol3")
	})
}

func TestProtocolFederation_GetProtocolCapabilities(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	protocol := &MockFederatedProtocol{
		name: "capable-protocol",
		capabilities: map[string]interface{}{
			"streaming": true,
			"tools":     5,
		},
	}
	federation.RegisterProtocol(protocol)

	t.Run("get existing protocol capabilities", func(t *testing.T) {
		caps, err := federation.GetProtocolCapabilities("capable-protocol")
		require.NoError(t, err)
		assert.Equal(t, true, caps["streaming"])
		assert.Equal(t, 5, caps["tools"])
	})

	t.Run("get non-existent protocol capabilities", func(t *testing.T) {
		caps, err := federation.GetProtocolCapabilities("non-existent")
		assert.Error(t, err)
		assert.Nil(t, caps)
	})
}

func TestProtocolFederation_SubscribeToEvents(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	protocol := &MockFederatedProtocol{name: "subscriber-protocol"}
	federation.RegisterProtocol(protocol)

	t.Run("subscribe to events", func(t *testing.T) {
		handler := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}

		err := federation.SubscribeToEvents("subscriber-protocol", "test-event", handler)
		require.NoError(t, err)
	})

	t.Run("subscribe with non-existent protocol", func(t *testing.T) {
		handler := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}

		err := federation.SubscribeToEvents("non-existent", "test-event", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestProtocolFederation_UnsubscribeFromEvents(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	protocol := &MockFederatedProtocol{name: "unsub-protocol"}
	federation.RegisterProtocol(protocol)

	handler := func(ctx context.Context, event *ProtocolEvent) error {
		return nil
	}
	federation.SubscribeToEvents("unsub-protocol", "test-event", handler)

	t.Run("unsubscribe from non-existent protocol", func(t *testing.T) {
		err := federation.UnsubscribeFromEvents("non-existent", "sub-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no subscriptions found")
	})
}

func TestProtocolFederation_AddDataTranslator(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	translator := &DataTranslator{
		SourceProtocol: "mcp",
		TargetProtocol: "lsp",
		Translations: map[string]TranslationRule{
			"name": {
				SourcePath: "toolName",
				TargetPath: "method",
				Transform:  IdentityTransform,
			},
		},
	}

	err := federation.AddDataTranslator(translator)
	require.NoError(t, err)
}

func TestProtocolFederation_BroadcastRequest(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	// Register multiple protocols
	for i := 1; i <= 3; i++ {
		protocol := &MockFederatedProtocol{
			name: "protocol" + string(rune('0'+i)),
			handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
				return &FederatedResponse{
					ID:      request.ID,
					Success: true,
				}, nil
			},
		}
		federation.RegisterProtocol(protocol)
	}

	results := federation.BroadcastRequest(context.Background(), "test-action", map[string]interface{}{"key": "value"})

	assert.Len(t, results, 3)
	for _, result := range results {
		assert.True(t, result.Success)
	}
}

func TestProtocolFederation_PublishEvent(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	event := &ProtocolEvent{
		ID:        "test-event",
		Type:      "test-type",
		Source:    "test-source",
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
	}

	err := federation.PublishEvent(context.Background(), event)
	assert.NoError(t, err) // Should not error even with no subscribers
}

func TestNewEventBus(t *testing.T) {
	log := newFederationTestLogger()
	eventBus := NewEventBus(log)

	require.NotNil(t, eventBus)
	assert.NotNil(t, eventBus.subscribers)
}

func TestEventBus_Subscribe(t *testing.T) {
	log := newFederationTestLogger()
	eventBus := NewEventBus(log)

	handler := func(ctx context.Context, event *ProtocolEvent) error {
		return nil
	}

	eventBus.Subscribe("test-event", handler)

	eventBus.mu.RLock()
	assert.Len(t, eventBus.subscribers["test-event"], 1)
	eventBus.mu.RUnlock()
}

func TestEventBus_Publish(t *testing.T) {
	log := newFederationTestLogger()
	eventBus := NewEventBus(log)

	t.Run("publish with no subscribers", func(t *testing.T) {
		event := &ProtocolEvent{
			ID:   "test",
			Type: "no-subscribers",
		}

		err := eventBus.Publish(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("publish with subscribers", func(t *testing.T) {
		handled := make(chan bool, 1)
		handler := func(ctx context.Context, event *ProtocolEvent) error {
			handled <- true
			return nil
		}

		eventBus.Subscribe("with-subscribers", handler)

		event := &ProtocolEvent{
			ID:   "test",
			Type: "with-subscribers",
		}

		err := eventBus.Publish(context.Background(), event)
		assert.NoError(t, err)

		// Wait for handler to be called
		select {
		case <-handled:
			// Success
		case <-time.After(time.Second):
			t.Error("Handler was not called")
		}
	})
}

func TestIdentityTransform(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"map", map[string]interface{}{"key": "value"}, map[string]interface{}{"key": "value"}},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IdentityTransform(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringToIntTransform(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    interface{}
		expectError bool
	}{
		{"true string", "true", 1, false},
		{"false string", "false", 0, false},
		{"invalid string", "invalid", 0, true},
		{"int passthrough", 42, 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StringToIntTransform(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJSONTransform(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		result, err := JSONTransform(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("primitive input", func(t *testing.T) {
		result, err := JSONTransform("hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})
}

func TestFederatedRequest_Structure(t *testing.T) {
	now := time.Now()
	request := &FederatedRequest{
		ID:            "req-123",
		Source:        "mcp",
		Target:        "lsp",
		Action:        "execute",
		Data:          map[string]interface{}{"key": "value"},
		Timestamp:     now,
		CorrelationID: "corr-456",
	}

	assert.Equal(t, "req-123", request.ID)
	assert.Equal(t, "mcp", request.Source)
	assert.Equal(t, "lsp", request.Target)
	assert.Equal(t, "execute", request.Action)
	assert.Equal(t, "corr-456", request.CorrelationID)
}

func TestFederatedResponse_Structure(t *testing.T) {
	now := time.Now()
	response := &FederatedResponse{
		ID:            "resp-123",
		Success:       true,
		Data:          map[string]interface{}{"result": "success"},
		Error:         "",
		Timestamp:     now,
		CorrelationID: "corr-456",
	}

	assert.Equal(t, "resp-123", response.ID)
	assert.True(t, response.Success)
	assert.Equal(t, "corr-456", response.CorrelationID)
}

func TestProtocolEvent_Structure(t *testing.T) {
	now := time.Now()
	event := &ProtocolEvent{
		ID:        "event-123",
		Type:      "tool-executed",
		Source:    "mcp",
		Data:      map[string]interface{}{"tool": "calculator"},
		Timestamp: now,
	}

	assert.Equal(t, "event-123", event.ID)
	assert.Equal(t, "tool-executed", event.Type)
	assert.Equal(t, "mcp", event.Source)
}

func TestEventSubscription_Structure(t *testing.T) {
	subscription := EventSubscription{
		ID:        "sub-123",
		Protocol:  "mcp",
		EventType: "tool-executed",
		Handler:   nil,
	}

	assert.Equal(t, "sub-123", subscription.ID)
	assert.Equal(t, "mcp", subscription.Protocol)
	assert.Equal(t, "tool-executed", subscription.EventType)
}

func TestDataTranslator_Structure(t *testing.T) {
	translator := &DataTranslator{
		SourceProtocol: "mcp",
		TargetProtocol: "lsp",
		Translations: map[string]TranslationRule{
			"name": {
				SourcePath: "toolName",
				TargetPath: "method",
				Transform:  IdentityTransform,
			},
		},
	}

	assert.Equal(t, "mcp", translator.SourceProtocol)
	assert.Equal(t, "lsp", translator.TargetProtocol)
	assert.Len(t, translator.Translations, 1)
}

func TestTranslationRule_Structure(t *testing.T) {
	rule := TranslationRule{
		SourcePath: "source.path",
		TargetPath: "target.path",
		Transform:  IdentityTransform,
	}

	assert.Equal(t, "source.path", rule.SourcePath)
	assert.Equal(t, "target.path", rule.TargetPath)
	assert.NotNil(t, rule.Transform)
}

func BenchmarkProtocolFederation_SendFederatedRequest(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	federation := NewProtocolFederation(log)

	protocol := &MockFederatedProtocol{
		name: "bench-protocol",
		handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
			return &FederatedResponse{
				ID:      request.ID,
				Success: true,
			}, nil
		},
	}
	federation.RegisterProtocol(protocol)

	request := &FederatedRequest{
		ID:     "bench-request",
		Source: "source",
		Target: "bench-protocol",
		Action: "test",
		Data:   map[string]interface{}{"key": "value"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = federation.SendFederatedRequest(ctx, request)
	}
}

func BenchmarkEventBus_Publish(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	eventBus := NewEventBus(log)

	handler := func(ctx context.Context, event *ProtocolEvent) error {
		return nil
	}
	eventBus.Subscribe("bench-event", handler)

	event := &ProtocolEvent{
		ID:   "bench",
		Type: "bench-event",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eventBus.Publish(ctx, event)
	}
}
