package services

import (
	"context"
	"fmt"
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

	t.Run("send request with translator", func(t *testing.T) {
		fed := NewProtocolFederation(log)

		// Register source and target protocols
		sourceProto := &MockFederatedProtocol{name: "source-proto"}
		targetProto := &MockFederatedProtocol{
			name: "target-proto",
			handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
				return &FederatedResponse{
					ID:      request.ID,
					Success: true,
					Data:    request.Data,
				}, nil
			},
		}
		fed.RegisterProtocol(sourceProto)
		fed.RegisterProtocol(targetProto)

		// Add a translator from source to target
		translator := &DataTranslator{
			SourceProtocol: "source-proto",
			TargetProtocol: "target-proto",
			Translations: map[string]TranslationRule{
				"original_key": {
					SourcePath: "original_key",
					TargetPath: "translated_key",
					Transform: func(v interface{}) (interface{}, error) {
						return fmt.Sprintf("translated_%v", v), nil
					},
				},
			},
		}
		err := fed.AddDataTranslator(translator)
		require.NoError(t, err)

		request := &FederatedRequest{
			ID:     "trans-request",
			Source: "source-proto",
			Target: "target-proto",
			Action: "test",
			Data:   map[string]interface{}{"original_key": "value1"},
		}

		response, err := fed.SendFederatedRequest(context.Background(), request)
		require.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("send request with response translator", func(t *testing.T) {
		fed := NewProtocolFederation(log)

		targetProto := &MockFederatedProtocol{
			name: "target-proto2",
			handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
				return &FederatedResponse{
					ID:      request.ID,
					Success: true,
					Data:    map[string]interface{}{"response_key": "response_value"},
				}, nil
			},
		}
		fed.RegisterProtocol(targetProto)

		// Add a translator from target to source for response translation
		fed.mu.Lock()
		fed.translators["target-proto2-source-proto"] = &DataTranslator{
			SourceProtocol: "target-proto2",
			TargetProtocol: "source-proto",
			Translations: map[string]TranslationRule{
				"response_key": {
					SourcePath: "response_key",
					TargetPath: "translated_response",
					Transform: func(v interface{}) (interface{}, error) {
						return fmt.Sprintf("trans_%v", v), nil
					},
				},
			},
		}
		fed.mu.Unlock()

		request := &FederatedRequest{
			ID:     "resp-trans-request",
			Source: "source-proto",
			Target: "target-proto2",
			Action: "test",
			Data:   map[string]interface{}{"key": "value"},
		}

		response, err := fed.SendFederatedRequest(context.Background(), request)
		require.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("send request with translation error", func(t *testing.T) {
		fed := NewProtocolFederation(log)

		targetProto := &MockFederatedProtocol{
			name: "target-proto3",
		}
		fed.RegisterProtocol(targetProto)

		// Add a translator that returns an error
		fed.mu.Lock()
		fed.translators["source-proto-target-proto3"] = &DataTranslator{
			SourceProtocol: "source-proto",
			TargetProtocol: "target-proto3",
			Translations: map[string]TranslationRule{
				"fail_key": {
					SourcePath: "fail_key",
					TargetPath: "translated_fail",
					Transform: func(v interface{}) (interface{}, error) {
						return nil, fmt.Errorf("translation failed")
					},
				},
			},
		}
		fed.mu.Unlock()

		request := &FederatedRequest{
			ID:     "fail-trans-request",
			Source: "source-proto",
			Target: "target-proto3",
			Action: "test",
			Data:   map[string]interface{}{"fail_key": "value"},
		}

		response, err := fed.SendFederatedRequest(context.Background(), request)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to translate request")
	})

	t.Run("send request when handler returns error", func(t *testing.T) {
		fed := NewProtocolFederation(log)

		errorProto := &MockFederatedProtocol{
			name: "error-proto",
			handler: func(ctx context.Context, request *FederatedRequest) (*FederatedResponse, error) {
				return nil, fmt.Errorf("handler error")
			},
		}
		fed.RegisterProtocol(errorProto)

		request := &FederatedRequest{
			ID:     "error-request",
			Source: "source",
			Target: "error-proto",
			Action: "test",
			Data:   map[string]interface{}{},
		}

		response, err := fed.SendFederatedRequest(context.Background(), request)
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "handler error")
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

	t.Run("unsubscribe existing subscription", func(t *testing.T) {
		// Create a fresh federation for this test
		fed := NewProtocolFederation(log)
		proto := &MockFederatedProtocol{name: "test-proto"}
		fed.RegisterProtocol(proto)

		// Subscribe to get a subscription
		handlerFn := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}
		fed.SubscribeToEvents("test-proto", "my-event", handlerFn)

		// Get the subscription ID from the federation
		fed.mu.RLock()
		subs := fed.subscriptions["test-proto"]
		subID := ""
		if len(subs) > 0 {
			subID = subs[0].ID
		}
		fed.mu.RUnlock()

		// Now unsubscribe with a valid subscription ID
		err := fed.UnsubscribeFromEvents("test-proto", subID)
		require.NoError(t, err)

		// Verify subscription was removed
		fed.mu.RLock()
		remainingSubs := fed.subscriptions["test-proto"]
		fed.mu.RUnlock()
		assert.Len(t, remainingSubs, 0)
	})

	t.Run("unsubscribe with non-matching subscription ID", func(t *testing.T) {
		fed := NewProtocolFederation(log)
		proto := &MockFederatedProtocol{name: "test-proto-2"}
		fed.RegisterProtocol(proto)

		handlerFn := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}
		fed.SubscribeToEvents("test-proto-2", "event", handlerFn)

		// Unsubscribe with a non-matching ID - should not error but won't remove anything
		err := fed.UnsubscribeFromEvents("test-proto-2", "non-existent-sub-id")
		require.NoError(t, err) // No error even if ID doesn't match

		// Subscription should still exist
		fed.mu.RLock()
		subs := fed.subscriptions["test-proto-2"]
		fed.mu.RUnlock()
		assert.Len(t, subs, 1)
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

	t.Run("nil input", func(t *testing.T) {
		result, err := JSONTransform(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("numeric input", func(t *testing.T) {
		result, err := JSONTransform(123)
		require.NoError(t, err)
		assert.Equal(t, float64(123), result) // JSON unmarshals numbers to float64
	})

	t.Run("complex nested input", func(t *testing.T) {
		input := map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"nested": "value",
			},
			"key3": []interface{}{"a", "b", "c"},
		}
		result, err := JSONTransform(input)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("invalid input - channel", func(t *testing.T) {
		ch := make(chan int)
		result, err := JSONTransform(ch)
		assert.Error(t, err)
		assert.Nil(t, result)
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

// Tests for helper functions

func TestProtocolFederation_GetNestedValue(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("simple key", func(t *testing.T) {
		data := map[string]interface{}{"key": "value"}
		result, exists := federation.getNestedValue(data, "key")
		assert.True(t, exists)
		assert.Equal(t, "value", result)
	})

	t.Run("nested key", func(t *testing.T) {
		data := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "deep value",
			},
		}
		result, exists := federation.getNestedValue(data, "level1.level2")
		assert.True(t, exists)
		assert.Equal(t, "deep value", result)
	})

	t.Run("deeply nested key", func(t *testing.T) {
		data := map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": "deep",
				},
			},
		}
		result, exists := federation.getNestedValue(data, "a.b.c")
		assert.True(t, exists)
		assert.Equal(t, "deep", result)
	})

	t.Run("non-existent key", func(t *testing.T) {
		data := map[string]interface{}{"key": "value"}
		result, exists := federation.getNestedValue(data, "nonexistent")
		assert.False(t, exists)
		assert.Nil(t, result)
	})

	t.Run("non-existent nested key", func(t *testing.T) {
		data := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": "value",
			},
		}
		result, exists := federation.getNestedValue(data, "level1.level3")
		assert.False(t, exists)
		assert.Nil(t, result)
	})

	t.Run("invalid nested path", func(t *testing.T) {
		data := map[string]interface{}{
			"level1": "string value", // Not a map, so can't navigate further
		}
		result, exists := federation.getNestedValue(data, "level1.level2")
		assert.False(t, exists)
		assert.Nil(t, result)
	})
}

func TestProtocolFederation_SetNestedValue(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("simple key", func(t *testing.T) {
		data := make(map[string]interface{})
		federation.setNestedValue(data, "key", "value")
		assert.Equal(t, "value", data["key"])
	})

	t.Run("nested key creates intermediate maps", func(t *testing.T) {
		data := make(map[string]interface{})
		federation.setNestedValue(data, "level1.level2", "deep value")

		level1, ok := data["level1"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "deep value", level1["level2"])
	})

	t.Run("deeply nested key", func(t *testing.T) {
		data := make(map[string]interface{})
		federation.setNestedValue(data, "a.b.c", "deep")

		a, ok := data["a"].(map[string]interface{})
		require.True(t, ok)
		b, ok := a["b"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "deep", b["c"])
	})

	t.Run("overwrites non-map value", func(t *testing.T) {
		data := map[string]interface{}{
			"level1": "string value",
		}
		federation.setNestedValue(data, "level1.level2", "new value")

		level1, ok := data["level1"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "new value", level1["level2"])
	})

	t.Run("updates existing nested structure", func(t *testing.T) {
		data := map[string]interface{}{
			"level1": map[string]interface{}{
				"existing": "keep me",
			},
		}
		federation.setNestedValue(data, "level1.new", "added")

		level1 := data["level1"].(map[string]interface{})
		assert.Equal(t, "keep me", level1["existing"])
		assert.Equal(t, "added", level1["new"])
	})
}

func TestEventBus_Unsubscribe(t *testing.T) {
	log := newFederationTestLogger()
	eventBus := NewEventBus(log)

	t.Run("unsubscribe from empty list", func(t *testing.T) {
		handler := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}
		// Unsubscribe does not return error
		eventBus.Unsubscribe("non-existent-event", handler)
	})

	t.Run("unsubscribe with subscribers", func(t *testing.T) {
		handler := func(ctx context.Context, event *ProtocolEvent) error {
			return nil
		}
		eventBus.Subscribe("unsubscribe-test", handler)

		// Verify subscription exists
		eventBus.mu.RLock()
		assert.Len(t, eventBus.subscribers["unsubscribe-test"], 1)
		eventBus.mu.RUnlock()

		// Unsubscribe
		eventBus.Unsubscribe("unsubscribe-test", handler)

		// Verify subscription was removed
		eventBus.mu.RLock()
		assert.Len(t, eventBus.subscribers["unsubscribe-test"], 0)
		eventBus.mu.RUnlock()
	})
}

func TestProtocolFederation_TranslateRequest(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("translate with translator", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations: map[string]TranslationRule{
				"field": {
					SourcePath: "oldKey",
					TargetPath: "newKey",
					Transform:  IdentityTransform,
				},
			},
		}

		request := &FederatedRequest{
			ID:     "test",
			Source: "source",
			Target: "target",
			Data:   map[string]interface{}{"oldKey": "value"},
		}

		result, err := federation.translateRequest(translator, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.Data, "newKey")
		assert.Equal(t, "value", result.Data["newKey"])
	})

	t.Run("translate preserves original data", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations:   map[string]TranslationRule{},
		}

		request := &FederatedRequest{
			ID:     "test",
			Source: "source",
			Target: "target",
			Data:   map[string]interface{}{"key": "value", "other": 123},
		}

		result, err := federation.translateRequest(translator, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "value", result.Data["key"])
		assert.Equal(t, 123, result.Data["other"])
	})

	t.Run("translate with nested path", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations: map[string]TranslationRule{
				"nested": {
					SourcePath: "parent.child",
					TargetPath: "output.value",
					Transform:  IdentityTransform,
				},
			},
		}

		request := &FederatedRequest{
			ID:     "test",
			Source: "source",
			Target: "target",
			Data: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "nested-value",
				},
			},
		}

		result, err := federation.translateRequest(translator, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check nested output was set correctly
		output, ok := result.Data["output"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "nested-value", output["value"])
	})
}

func TestProtocolFederation_TranslateResponse(t *testing.T) {
	log := newFederationTestLogger()
	federation := NewProtocolFederation(log)

	t.Run("translate with translator", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations: map[string]TranslationRule{
				"field": {
					SourcePath: "oldKey",
					TargetPath: "newKey",
					Transform:  IdentityTransform,
				},
			},
		}

		response := &FederatedResponse{
			ID:      "test",
			Success: true,
			Data:    map[string]interface{}{"oldKey": "value"},
		}

		result, err := federation.translateResponse(translator, response)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Contains(t, result.Data, "newKey")
	})

	t.Run("translate preserves response metadata", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations:   map[string]TranslationRule{},
		}

		response := &FederatedResponse{
			ID:            "test-id",
			Success:       true,
			Data:          map[string]interface{}{"key": "value"},
			Error:         "",
			CorrelationID: "corr-123",
		}

		result, err := federation.translateResponse(translator, response)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-id", result.ID)
		assert.True(t, result.Success)
		assert.Equal(t, "corr-123", result.CorrelationID)
	})

	t.Run("translate with nested path", func(t *testing.T) {
		translator := &DataTranslator{
			SourceProtocol: "source",
			TargetProtocol: "target",
			Translations: map[string]TranslationRule{
				"nested": {
					SourcePath: "parent.child",
					TargetPath: "output.value",
					Transform:  IdentityTransform,
				},
			},
		}

		response := &FederatedResponse{
			ID:      "test",
			Success: true,
			Data: map[string]interface{}{
				"parent": map[string]interface{}{
					"child": "nested-value",
				},
			},
		}

		result, err := federation.translateResponse(translator, response)
		require.NoError(t, err)
		require.NotNil(t, result)

		output, ok := result.Data["output"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "nested-value", output["value"])
	})
}

// Tests for MCPFederatedProtocol

func TestNewMCPFederatedProtocol(t *testing.T) {
	log := newFederationTestLogger()

	t.Run("create with nil client", func(t *testing.T) {
		protocol := NewMCPFederatedProtocol(nil)
		require.NotNil(t, protocol)
		assert.Equal(t, "mcp", protocol.Name())
	})

	t.Run("create with client", func(t *testing.T) {
		client := NewMCPClient(log)
		protocol := NewMCPFederatedProtocol(client)
		require.NotNil(t, protocol)
		assert.Equal(t, "mcp", protocol.Name())
	})
}

func TestMCPFederatedProtocol_GetCapabilities(t *testing.T) {
	log := newFederationTestLogger()
	client := NewMCPClient(log)
	protocol := NewMCPFederatedProtocol(client)

	caps := protocol.GetCapabilities()
	require.NotNil(t, caps)
	assert.Contains(t, caps, "type")
	assert.Contains(t, caps, "tools")
	assert.Contains(t, caps, "servers")
	assert.Equal(t, "mcp", caps["type"])
}

func TestMCPFederatedProtocol_HandleFederatedRequest(t *testing.T) {
	log := newFederationTestLogger()
	client := NewMCPClient(log)
	protocol := NewMCPFederatedProtocol(client)
	ctx := context.Background()

	t.Run("handle list_tools request", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "list_tools",
			Data:   map[string]interface{}{},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		// Response may or may not be success depending on client state
	})

	t.Run("handle call_tool request", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "call_tool",
			Data: map[string]interface{}{
				"serverID": "test-server",
				"toolName": "test-tool",
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handle unknown action", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "unknown_action",
			Data:   map[string]interface{}{},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("no servers available", func(t *testing.T) {
		request := &FederatedRequest{
			ID:            "test-no-servers",
			Action:        "some-tool",
			Data:          map[string]interface{}{"param": "value"},
			CorrelationID: "corr-123",
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "no MCP servers available")
		assert.Equal(t, "test-no-servers", response.ID)
		// Note: CorrelationID is only set in successful call path, not in the "no servers" error path
	})
}

func TestMCPFederatedProtocol_PublishEvent(t *testing.T) {
	log := newFederationTestLogger()
	client := NewMCPClient(log)
	protocol := NewMCPFederatedProtocol(client)
	ctx := context.Background()

	event := &ProtocolEvent{
		ID:   "test-event",
		Type: "tool-executed",
		Data: map[string]interface{}{"tool": "test"},
	}

	err := protocol.PublishEvent(ctx, event)
	assert.NoError(t, err)
}

// Tests for LSPFederatedProtocol

func TestNewLSPFederatedProtocol(t *testing.T) {
	log := newFederationTestLogger()

	t.Run("create with nil client", func(t *testing.T) {
		protocol := NewLSPFederatedProtocol(nil)
		require.NotNil(t, protocol)
		assert.Equal(t, "lsp", protocol.Name())
	})

	t.Run("create with client", func(t *testing.T) {
		client := NewLSPClient(log)
		protocol := NewLSPFederatedProtocol(client)
		require.NotNil(t, protocol)
		assert.Equal(t, "lsp", protocol.Name())
	})
}

func TestLSPFederatedProtocol_GetCapabilities(t *testing.T) {
	log := newFederationTestLogger()
	client := NewLSPClient(log)
	protocol := NewLSPFederatedProtocol(client)

	caps := protocol.GetCapabilities()
	require.NotNil(t, caps)
	assert.Contains(t, caps, "type")
	assert.Contains(t, caps, "languages")
	assert.Contains(t, caps, "servers")
	assert.Equal(t, "lsp", caps["type"])
}

func TestLSPFederatedProtocol_HandleFederatedRequest(t *testing.T) {
	log := newFederationTestLogger()
	client := NewLSPClient(log)
	protocol := NewLSPFederatedProtocol(client)
	ctx := context.Background()

	t.Run("handle hover request", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "hover",
			Data: map[string]interface{}{
				"serverID": "test-server",
				"fileURI":  "file:///test.go",
				"line":     10,
				"column":   5,
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handle completion request", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "completion",
			Data: map[string]interface{}{
				"serverID": "test-server",
				"fileURI":  "file:///test.go",
				"line":     10,
				"column":   5,
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handle unknown action", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "unknown_action",
			Data:   map[string]interface{}{},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Contains(t, response.Error, "unsupported LSP action")
	})

	t.Run("completion missing uri parameter", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "completion",
			Data:   map[string]interface{}{},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "missing uri parameter")
	})

	t.Run("completion missing line parameter", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "completion",
			Data: map[string]interface{}{
				"uri": "file:///test.go",
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "missing line parameter")
	})

	t.Run("completion missing character parameter", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "completion",
			Data: map[string]interface{}{
				"uri":  "file:///test.go",
				"line": float64(10),
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "missing character parameter")
	})

	t.Run("completion with all parameters but no servers", func(t *testing.T) {
		request := &FederatedRequest{
			ID:     "test-request",
			Action: "completion",
			Data: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      float64(10),
				"character": float64(5),
			},
		}

		response, err := protocol.HandleFederatedRequest(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "no LSP servers available")
	})
}

func TestLSPFederatedProtocol_PublishEvent(t *testing.T) {
	log := newFederationTestLogger()
	client := NewLSPClient(log)
	protocol := NewLSPFederatedProtocol(client)
	ctx := context.Background()

	event := &ProtocolEvent{
		ID:   "test-event",
		Type: "file-opened",
		Data: map[string]interface{}{"uri": "file:///test.go"},
	}

	err := protocol.PublishEvent(ctx, event)
	assert.NoError(t, err)
}

// Benchmarks for helper functions

func BenchmarkProtocolFederation_GetNestedValue(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	federation := NewProtocolFederation(log)

	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "value",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = federation.getNestedValue(data, "level1.level2.level3")
	}
}

func BenchmarkProtocolFederation_SetNestedValue(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	federation := NewProtocolFederation(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := make(map[string]interface{})
		federation.setNestedValue(data, "level1.level2.level3", "value")
	}
}

func TestMCPFederatedProtocol_HandleFederatedRequest_WithServer(t *testing.T) {
	log := newFederationTestLogger()
	client := NewMCPClient(log)
	protocol := NewMCPFederatedProtocol(client)
	ctx := context.Background()

	// Create a mock transport
	mockTransport := NewMockMCPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Tool execution result",
					},
				},
			},
		}, nil
	}

	// Register the mock server with the client
	client.mu.Lock()
	client.servers["test-server"] = &MCPServerConnection{
		ID:        "test-server",
		Name:      "Test Server",
		Transport: mockTransport,
		Connected: true,
		Tools: []*MCPTool{
			{Name: "test-tool", Description: "Test Tool"},
		},
	}
	client.mu.Unlock()

	// Now test with a registered server
	request := &FederatedRequest{
		ID:            "test-with-server",
		Action:        "test-tool",
		Data:          map[string]interface{}{"param": "value"},
		CorrelationID: "corr-456",
	}

	response, err := protocol.HandleFederatedRequest(ctx, request)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-with-server", response.ID)
	assert.Equal(t, "corr-456", response.CorrelationID)
	// Response may succeed or fail depending on mock transport behavior
}

func TestLSPFederatedProtocol_HandleFederatedRequest_WithServer(t *testing.T) {
	log := newFederationTestLogger()
	client := NewLSPClient(log)
	protocol := NewLSPFederatedProtocol(client)
	ctx := context.Background()

	// Create a mock transport for LSP
	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"isIncomplete": false,
				"items":        []interface{}{},
			},
		}, nil
	}

	// Register the mock server with the client
	client.mu.Lock()
	client.servers["test-lsp-server"] = &LSPServerConnection{
		ID:        "test-lsp-server",
		Name:      "Test LSP Server",
		Transport: mockTransport,
		Connected: true,
		Capabilities: &LSPCapabilities{
			CompletionProvider: &CompletionOptions{},
		},
	}
	client.mu.Unlock()

	// Test completion with a registered server
	request := &FederatedRequest{
		ID:     "test-lsp-with-server",
		Action: "completion",
		Data: map[string]interface{}{
			"uri":       "file:///test.go",
			"line":      float64(10),
			"character": float64(5),
		},
		CorrelationID: "corr-789",
	}

	response, err := protocol.HandleFederatedRequest(ctx, request)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-lsp-with-server", response.ID)
	assert.Equal(t, "corr-789", response.CorrelationID)
}
