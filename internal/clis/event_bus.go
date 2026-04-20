// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"sync"
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"
)

// EventBus provides pub/sub event routing between agent instances.
//
// Concurrency model (CONST-029):
//   - subscribers/topics → *safe.Store with Update-based COW
//     for the per-key slice appends and filtered removes.
//   - wildcards → *safe.Slice.
//   - dispatchMu (Pattern Zeta, RWMutex) serialises dispatch
//     (RLock) against Close's channel-close pass (Lock) so
//     senders cannot race with channel closers.
//   - closed atomic.Bool short-circuits sendToSub once Close
//     starts, and closeOnce keeps Close idempotent.
type EventBus struct {
	// Subscribers by event type
	subscribers *safe.Store[EventType, []*Subscription]

	// Wildcard subscribers (receive all events)
	wildcards *safe.Slice[*Subscription]

	// Topic-based subscribers
	topics *safe.Store[string, []*Subscription]

	// Serialises dispatch vs channel-close in Close.
	dispatchMu sync.RWMutex

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
		subscribers: safe.NewStore[EventType, []*Subscription](),
		topics:      safe.NewStore[string, []*Subscription](),
		wildcards:   safe.NewSlice[*Subscription](),
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
	sub := &Subscription{
		ID:        generateEventID(),
		EventType: eventType,
		Ch:        make(chan *Event, bufferSize),
	}

	eb.subscribers.Update(eventType, func(cur []*Subscription, _ bool) ([]*Subscription, bool) {
		return append(cur, sub), true
	})

	return sub
}

// SubscribeTopic creates a subscription for a topic.
func (eb *EventBus) SubscribeTopic(topic string, bufferSize int) *Subscription {
	sub := &Subscription{
		ID:    generateEventID(),
		Topic: topic,
		Ch:    make(chan *Event, bufferSize),
	}

	eb.topics.Update(topic, func(cur []*Subscription, _ bool) ([]*Subscription, bool) {
		return append(cur, sub), true
	})

	return sub
}

// SubscribeWildcard creates a subscription for all events.
func (eb *EventBus) SubscribeWildcard(bufferSize int) *Subscription {
	sub := &Subscription{
		ID: generateEventID(),
		Ch: make(chan *Event, bufferSize),
	}

	eb.wildcards.Append(sub)

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
	var closed *Subscription

	// Remove from event type subscribers
	if sub.EventType != "" {
		eb.subscribers.Update(sub.EventType, func(cur []*Subscription, present bool) ([]*Subscription, bool) {
			if !present {
				return cur, false
			}
			for i, s := range cur {
				if s.ID == sub.ID {
					closed = s
					next := make([]*Subscription, 0, len(cur)-1)
					next = append(next, cur[:i]...)
					next = append(next, cur[i+1:]...)
					if len(next) == 0 {
						return nil, false
					}
					return next, true
				}
			}
			return cur, true
		})
	}

	// Remove from topic subscribers
	if closed == nil && sub.Topic != "" {
		eb.topics.Update(sub.Topic, func(cur []*Subscription, present bool) ([]*Subscription, bool) {
			if !present {
				return cur, false
			}
			for i, s := range cur {
				if s.ID == sub.ID {
					closed = s
					next := make([]*Subscription, 0, len(cur)-1)
					next = append(next, cur[:i]...)
					next = append(next, cur[i+1:]...)
					if len(next) == 0 {
						return nil, false
					}
					return next, true
				}
			}
			return cur, true
		})
	}

	// Remove from wildcards
	if closed == nil {
		if removed, ok := eb.wildcards.Delete(func(s *Subscription) bool { return s.ID == sub.ID }); ok {
			closed = removed
		}
	}

	if closed != nil {
		close(closed.Ch)
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
	eb.dispatchMu.RLock()
	defer eb.dispatchMu.RUnlock()

	// Send to type-specific subscribers
	if subs, ok := eb.subscribers.Get(event.Type); ok {
		for _, sub := range subs {
			eb.sendToSub(sub, event)
		}
	}

	// Send to topic subscribers
	if topic, ok := event.Metadata["topic"].(string); ok {
		if subs, ok := eb.topics.Get(topic); ok {
			for _, sub := range subs {
				eb.sendToSub(sub, event)
			}
		}
	}

	// Send to wildcards
	for _, sub := range eb.wildcards.Snapshot() {
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

		// 4. Now safe to close all subscriber channels. Take dispatchMu
		//    in write mode — any in-flight dispatch() has already
		//    bailed via the closed.Load check in sendToSub, so this is
		//    just a fence for the race detector.
		eb.dispatchMu.Lock()
		defer eb.dispatchMu.Unlock()
		eb.subscribers.Range(func(_ EventType, subs []*Subscription) bool {
			for _, sub := range subs {
				close(sub.Ch)
			}
			return true
		})
		eb.topics.Range(func(_ string, subs []*Subscription) bool {
			for _, sub := range subs {
				close(sub.Ch)
			}
			return true
		})
		for _, sub := range eb.wildcards.Snapshot() {
			close(sub.Ch)
		}
	})

	return nil
}

// GetStats returns statistics about the event bus.
func (eb *EventBus) GetStats() map[string]interface{} {
	totalSubs := 0
	eb.subscribers.Range(func(_ EventType, subs []*Subscription) bool {
		totalSubs += len(subs)
		return true
	})
	eb.topics.Range(func(_ string, subs []*Subscription) bool {
		totalSubs += len(subs)
		return true
	})
	wildcardLen := eb.wildcards.Len()
	totalSubs += wildcardLen

	return map[string]interface{}{
		"total_subscriptions": totalSubs,
		"event_types":         eb.subscribers.Len(),
		"topics":              eb.topics.Len(),
		"wildcards":           wildcardLen,
	}
}

// Helper function
func generateEventID() string {
	return "evt_" + generateShortID()
}
