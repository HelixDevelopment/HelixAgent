// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"sync"
	"sync/atomic"
)

// EventBus provides pub/sub event routing between agent instances.
type EventBus struct {
	// Subscribers by event type
	subscribers map[EventType][]*Subscription

	// Wildcard subscribers (receive all events)
	wildcards []*Subscription

	// Topic-based subscribers
	topics map[string][]*Subscription

	mu sync.RWMutex

	// Event channel for async publishing
	eventCh chan *Event

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Close safety: prevent send-on-closed-channel panic
	closed    atomic.Bool
	closeOnce sync.Once
}

// Subscription represents an event subscription.
type Subscription struct {
	ID        string
	EventType EventType
	Topic     string
	Ch        chan *Event
	Filter    func(*Event) bool
	Once      bool
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	ctx, cancel := context.WithCancel(context.Background())

	eb := &EventBus{
		subscribers: make(map[EventType][]*Subscription),
		topics:      make(map[string][]*Subscription),
		eventCh:     make(chan *Event, 1000),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start event dispatcher
	eb.wg.Add(1)
	go eb.dispatchLoop()

	return eb
}

// Subscribe creates a subscription for a specific event type.
func (eb *EventBus) Subscribe(eventType EventType, bufferSize int) *Subscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscription{
		ID:        generateEventID(),
		EventType: eventType,
		Ch:        make(chan *Event, bufferSize),
	}

	eb.subscribers[eventType] = append(eb.subscribers[eventType], sub)

	return sub
}

// SubscribeTopic creates a subscription for a topic.
func (eb *EventBus) SubscribeTopic(topic string, bufferSize int) *Subscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscription{
		ID:    generateEventID(),
		Topic: topic,
		Ch:    make(chan *Event, bufferSize),
	}

	eb.topics[topic] = append(eb.topics[topic], sub)

	return sub
}

// SubscribeWildcard creates a subscription for all events.
func (eb *EventBus) SubscribeWildcard(bufferSize int) *Subscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscription{
		ID: generateEventID(),
		Ch: make(chan *Event, bufferSize),
	}

	eb.wildcards = append(eb.wildcards, sub)

	return sub
}

// SubscribeFiltered creates a subscription with a filter function.
func (eb *EventBus) SubscribeFiltered(
	eventType EventType,
	bufferSize int,
	filter func(*Event) bool,
) *Subscription {
	sub := eb.Subscribe(eventType, bufferSize)
	sub.Filter = filter
	return sub
}

// Unsubscribe removes a subscription.
func (eb *EventBus) Unsubscribe(sub *Subscription) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Remove from event type subscribers
	if sub.EventType != "" {
		subs := eb.subscribers[sub.EventType]
		for i, s := range subs {
			if s.ID == sub.ID {
				eb.subscribers[sub.EventType] = append(subs[:i], subs[i+1:]...)
				close(s.Ch)
				break
			}
		}
	}

	// Remove from topic subscribers
	if sub.Topic != "" {
		subs := eb.topics[sub.Topic]
		for i, s := range subs {
			if s.ID == sub.ID {
				eb.topics[sub.Topic] = append(subs[:i], subs[i+1:]...)
				close(s.Ch)
				break
			}
		}
	}

	// Remove from wildcards
	for i, s := range eb.wildcards {
		if s.ID == sub.ID {
			eb.wildcards = append(eb.wildcards[:i], eb.wildcards[i+1:]...)
			close(s.Ch)
			break
		}
	}
}

// Publish publishes an event to all subscribers.
func (eb *EventBus) Publish(event *Event) {
	select {
	case eb.eventCh <- event:
		// Event queued
	case <-eb.ctx.Done():
		// Bus is closed
	default:
		// Channel full, drop event (could also block or log)
	}
}

// PublishSync publishes an event synchronously (blocks until dispatched).
func (eb *EventBus) PublishSync(event *Event) {
	eb.dispatch(event)
}

// dispatch routes an event to all matching subscribers.
func (eb *EventBus) dispatch(event *Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Send to type-specific subscribers
	if subs, ok := eb.subscribers[event.Type]; ok {
		for _, sub := range subs {
			eb.sendToSub(sub, event)
		}
	}

	// Send to topic subscribers
	if topic, ok := event.Metadata["topic"].(string); ok {
		if subs, ok := eb.topics[topic]; ok {
			for _, sub := range subs {
				eb.sendToSub(sub, event)
			}
		}
	}

	// Send to wildcards
	for _, sub := range eb.wildcards {
		eb.sendToSub(sub, event)
	}
}

// sendToSub sends an event to a subscriber, respecting filters.
func (eb *EventBus) sendToSub(sub *Subscription, event *Event) {
	// Bail out if bus is closed — channels may already be closed.
	if eb.closed.Load() {
		return
	}

	// Apply filter if present
	if sub.Filter != nil && !sub.Filter(event) {
		return
	}

	select {
	case sub.Ch <- event:
		// Event sent
		if sub.Once {
			go eb.Unsubscribe(sub)
		}
	default:
		// Subscriber buffer full, drop event
	}
}

// dispatchLoop processes events from the queue.
func (eb *EventBus) dispatchLoop() {
	defer eb.wg.Done()

	for {
		select {
		case event := <-eb.eventCh:
			eb.dispatch(event)
		case <-eb.ctx.Done():
			return
		}
	}
}

// Close shuts down the event bus. It is safe to call multiple times.
func (eb *EventBus) Close() error {
	eb.closeOnce.Do(func() {
		// 1. Signal closed — sendToSub will bail out immediately.
		eb.closed.Store(true)

		// 2. Cancel context so dispatchLoop's select sees ctx.Done().
		eb.cancel()

		// 3. Wait for dispatchLoop to exit. This guarantees no goroutine
		//    is inside dispatch()/sendToSub() when we close channels below.
		eb.wg.Wait()

		// 4. Now safe to close all subscriber channels under lock.
		eb.mu.Lock()
		for _, subs := range eb.subscribers {
			for _, sub := range subs {
				close(sub.Ch)
			}
		}
		for _, subs := range eb.topics {
			for _, sub := range subs {
				close(sub.Ch)
			}
		}
		for _, sub := range eb.wildcards {
			close(sub.Ch)
		}
		eb.mu.Unlock()
	})

	return nil
}

// GetStats returns statistics about the event bus.
func (eb *EventBus) GetStats() map[string]interface{} {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	totalSubs := 0
	for _, subs := range eb.subscribers {
		totalSubs += len(subs)
	}
	for _, subs := range eb.topics {
		totalSubs += len(subs)
	}
	totalSubs += len(eb.wildcards)

	return map[string]interface{}{
		"total_subscriptions": totalSubs,
		"event_types":         len(eb.subscribers),
		"topics":              len(eb.topics),
		"wildcards":           len(eb.wildcards),
	}
}

// Helper function
func generateEventID() string {
	return "evt_" + generateShortID()
}
