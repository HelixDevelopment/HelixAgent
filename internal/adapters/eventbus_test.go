package adapters_test

import (
	"context"
	"testing"
	"time"

	adapters "dev.helix.agent/internal/adapters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventBus(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()
}

func TestEventBus_PublishAndSubscribe(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	ch := bus.Subscribe(adapters.EventProviderRegistered)
	require.NotNil(t, ch)

	evt := adapters.NewEvent(adapters.EventProviderRegistered, "test", map[string]string{"name": "provider-1"})
	bus.Publish(evt)

	select {
	case received := <-ch:
		assert.Equal(t, adapters.EventProviderRegistered, received.Type)
		assert.Equal(t, "test", received.Source)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_PublishAsync(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	ch := bus.Subscribe(adapters.EventDebateStarted)
	require.NotNil(t, ch)

	evt := adapters.NewEvent(adapters.EventDebateStarted, "debate", nil)
	bus.PublishAsync(evt)

	select {
	case received := <-ch:
		assert.Equal(t, adapters.EventDebateStarted, received.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async event")
	}
}

func TestEventBus_SubscribeMultiple(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	ch := bus.SubscribeMultiple(adapters.EventCacheHit, adapters.EventCacheMiss)
	require.NotNil(t, ch)

	evt := adapters.NewEvent(adapters.EventCacheHit, "cache", nil)
	bus.Publish(evt)

	select {
	case received := <-ch:
		assert.Equal(t, adapters.EventCacheHit, received.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_SubscribeWithFilter(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	// Only receive events with "critical" in payload
	ch := bus.SubscribeWithFilter(adapters.EventSystemError, func(e *adapters.Event) bool {
		payload, ok := e.Payload.(string)
		return ok && payload == "critical"
	})
	require.NotNil(t, ch)

	// Non-matching event should be filtered
	bus.Publish(adapters.NewEvent(adapters.EventSystemError, "system", "non-critical"))
	// Matching event
	bus.Publish(adapters.NewEvent(adapters.EventSystemError, "system", "critical"))

	select {
	case received := <-ch:
		assert.Equal(t, "critical", received.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for filtered event")
	}
}

func TestEventBus_Wait(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(adapters.NewEvent(adapters.EventSystemStartup, "system", nil))
	}()

	evt, err := bus.Wait(ctx, adapters.EventSystemStartup)
	require.NoError(t, err)
	assert.Equal(t, adapters.EventSystemStartup, evt.Type)
}

func TestEventBus_Wait_ContextCancelled(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := bus.Wait(ctx, adapters.EventSystemStartup)
	assert.Error(t, err)
}

func TestEventBus_SubscriberCount(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	initialCount := bus.SubscriberCount(adapters.EventProviderRegistered)
	assert.Equal(t, 0, initialCount)

	ch := bus.Subscribe(adapters.EventProviderRegistered)
	require.NotNil(t, ch)

	newCount := bus.SubscriberCount(adapters.EventProviderRegistered)
	assert.Equal(t, 1, newCount)

	bus.Unsubscribe(ch)
	finalCount := bus.SubscriberCount(adapters.EventProviderRegistered)
	assert.Equal(t, 0, finalCount)
}

func TestEventBus_TotalSubscribers(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	assert.Equal(t, 0, bus.TotalSubscribers())

	ch1 := bus.Subscribe(adapters.EventProviderRegistered)
	ch2 := bus.Subscribe(adapters.EventDebateStarted)
	require.NotNil(t, ch1)
	require.NotNil(t, ch2)

	assert.Equal(t, 2, bus.TotalSubscribers())
}

func TestEventBus_Metrics(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	metrics := bus.Metrics()
	assert.NotNil(t, metrics)
}

func TestEventBus_Inner(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultBusConfig()
	bus := adapters.NewEventBus(cfg)
	require.NotNil(t, bus)
	defer bus.Close()

	inner := bus.Inner()
	assert.NotNil(t, inner)
}

func TestNewEvent(t *testing.T) {
	t.Parallel()
	payload := map[string]string{"key": "value"}
	evt := adapters.NewEvent(adapters.EventProviderRegistered, "test-source", payload)

	require.NotNil(t, evt)
	assert.Equal(t, adapters.EventProviderRegistered, evt.Type)
	assert.Equal(t, "test-source", evt.Source)
	assert.Equal(t, payload, evt.Payload)
}

func TestEventTypeConstants(t *testing.T) {
	t.Parallel()
	// Verify all event type constants have expected string values
	assert.Equal(t, adapters.EventType("provider.registered"), adapters.EventProviderRegistered)
	assert.Equal(t, adapters.EventType("provider.unregistered"), adapters.EventProviderUnregistered)
	assert.Equal(t, adapters.EventType("provider.health.changed"), adapters.EventProviderHealthChanged)
	assert.Equal(t, adapters.EventType("provider.score.updated"), adapters.EventProviderScoreUpdated)
	assert.Equal(t, adapters.EventType("mcp.server.connected"), adapters.EventMCPServerConnected)
	assert.Equal(t, adapters.EventType("debate.started"), adapters.EventDebateStarted)
	assert.Equal(t, adapters.EventType("cache.hit"), adapters.EventCacheHit)
	assert.Equal(t, adapters.EventType("system.startup"), adapters.EventSystemStartup)
	assert.Equal(t, adapters.EventType("request.received"), adapters.EventRequestReceived)
}

func TestGlobalBus_InitAndEmit(t *testing.T) {
	// NOT t.Parallel — this test mutates adapters.GlobalBus, a global
	// singleton shared with TestGlobalBus_EmitAsync and TestOn_
	// NilGlobalBus. Running them in parallel races on
	// InitGlobalBus (replaces the global) vs GlobalBus = nil in
	// TestOn_NilGlobalBus (race-debt BUGFIX #35).
	cfg := adapters.DefaultBusConfig()
	adapters.InitGlobalBus(cfg)

	ch := adapters.On(adapters.EventSystemStartup)
	require.NotNil(t, ch)

	adapters.Emit(adapters.NewEvent(adapters.EventSystemStartup, "global", nil))

	select {
	case evt := <-ch:
		assert.Equal(t, adapters.EventSystemStartup, evt.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for global event")
	}
}

func TestGlobalBus_EmitAsync(t *testing.T) {
	// NOT t.Parallel — see TestGlobalBus_InitAndEmit for rationale
	// (BUGFIX #35).
	cfg := adapters.DefaultBusConfig()
	adapters.InitGlobalBus(cfg)

	ch := adapters.On(adapters.EventSystemShutdown)
	require.NotNil(t, ch)

	adapters.EmitAsync(adapters.NewEvent(adapters.EventSystemShutdown, "global", nil))

	select {
	case evt := <-ch:
		assert.Equal(t, adapters.EventSystemShutdown, evt.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async global event")
	}
}

func TestOn_NilGlobalBus(t *testing.T) {
	// NOT t.Parallel — mutates the adapters.GlobalBus singleton
	// shared with TestGlobalBus_InitAndEmit + _EmitAsync (BUGFIX #35).
	// When GlobalBus is nil, On should return a closed channel
	adapters.GlobalBus = nil
	ch := adapters.On(adapters.EventSystemStartup)
	require.NotNil(t, ch)

	// Channel should be closed (readable immediately)
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed")
	default:
		t.Error("expected closed channel to be immediately readable")
	}

	// Reset GlobalBus for other tests
	cfg := adapters.DefaultBusConfig()
	adapters.InitGlobalBus(cfg)
}

func TestDefaultLazyProviderConfig(t *testing.T) {
	t.Parallel()
	cfg := adapters.DefaultLazyProviderConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 30*time.Second, cfg.InitTimeout)
	assert.Equal(t, 3, cfg.RetryAttempts)
	assert.Equal(t, 1*time.Second, cfg.RetryDelay)
	assert.False(t, cfg.PrewarmOnAccess)
}
