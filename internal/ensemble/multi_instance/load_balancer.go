// Package multi_instance provides multi-instance ensemble coordination for HelixAgent.
package multi_instance

import (
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis"
)

// LoadBalancer distributes requests across instances.
type LoadBalancer interface {
	// SelectInstance chooses an instance from the available pool.
	SelectInstance(instances []*clis.AgentInstance) (*clis.AgentInstance, error)

	// ReportResult reports the result of using an instance.
	ReportResult(instanceID string, success bool, duration int64)

	// GetStats returns load balancer statistics.
	GetStats() map[string]interface{}
}

// RoundRobinBalancer implements round-robin load balancing.
type RoundRobinBalancer struct {
	counter uint64
}

// NewRoundRobinBalancer creates a new round-robin balancer.
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// SelectInstance selects an instance using round-robin.
func (b *RoundRobinBalancer) SelectInstance(
	instances []*clis.AgentInstance,
) (*clis.AgentInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstancesAvailable
	}

	// Filter to only healthy, active instances
	var healthy []*clis.AgentInstance
	for _, inst := range instances {
		if inst.CanAcceptWork() {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) == 0 {
		return nil, ErrNoHealthyInstances
	}

	// Round-robin selection
	idx := atomic.AddUint64(&b.counter, 1) % uint64(len(healthy))
	return healthy[idx], nil
}

// ReportResult is a no-op for round-robin.
func (b *RoundRobinBalancer) ReportResult(instanceID string, success bool, duration int64) {
	// Round-robin doesn't track results
}

// GetStats returns balancer statistics.
func (b *RoundRobinBalancer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":    "round_robin",
		"counter": atomic.LoadUint64(&b.counter),
	}
}

// LeastConnectionsBalancer selects instances with fewest active connections.
// Pattern Alpha at the map level: each *int64 counter is immutable
// as a pointer; concurrent updates happen via atomic ops.
type LeastConnectionsBalancer struct {
	connections *safe.Store[string, *int64]
}

// NewLeastConnectionsBalancer creates a new least-connections balancer.
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		connections: safe.NewStore[string, *int64](),
	}
}

// SelectInstance selects the instance with least connections.
func (b *LeastConnectionsBalancer) SelectInstance(
	instances []*clis.AgentInstance,
) (*clis.AgentInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstancesAvailable
	}

	var selected *clis.AgentInstance
	var minConn int64 = -1

	for _, inst := range instances {
		if !inst.CanAcceptWork() {
			continue
		}

		var conn int64
		if connPtr, ok := b.connections.Get(inst.ID); ok {
			conn = atomic.LoadInt64(connPtr)
		}

		if minConn == -1 || conn < minConn {
			minConn = conn
			selected = inst
		}
	}

	if selected == nil {
		return nil, ErrNoHealthyInstances
	}

	// Increment connection count. PutIfAbsent makes get-or-create
	// atomic; racing first-callers converge on the same counter.
	zero := int64(0)
	connPtr, _ := b.connections.PutIfAbsent(selected.ID, &zero)
	atomic.AddInt64(connPtr, 1)

	return selected, nil
}

// ReportResult updates connection count.
func (b *LeastConnectionsBalancer) ReportResult(instanceID string, success bool, duration int64) {
	if connPtr, ok := b.connections.Get(instanceID); ok {
		atomic.AddInt64(connPtr, -1)
	}
}

// GetStats returns balancer statistics.
func (b *LeastConnectionsBalancer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	b.connections.Range(func(id string, connPtr *int64) bool {
		stats[id] = atomic.LoadInt64(connPtr)
		return true
	})

	return map[string]interface{}{
		"type":        "least_connections",
		"connections": stats,
	}
}

// WeightedResponseTimeBalancer selects based on response time performance.
// Pattern Alpha at the map level: each *responseTimeStats pointer is
// immutable; fields are updated via atomic ops.
type WeightedResponseTimeBalancer struct {
	responseTimes *safe.Store[string, *responseTimeStats]
}

type responseTimeStats struct {
	totalTime int64
	count     int64
	successes int64
	failures  int64
}

// NewWeightedResponseTimeBalancer creates a new weighted response time balancer.
func NewWeightedResponseTimeBalancer() *WeightedResponseTimeBalancer {
	return &WeightedResponseTimeBalancer{
		responseTimes: safe.NewStore[string, *responseTimeStats](),
	}
}

// SelectInstance selects based on weighted response time.
func (b *WeightedResponseTimeBalancer) SelectInstance(
	instances []*clis.AgentInstance,
) (*clis.AgentInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstancesAvailable
	}

	var selected *clis.AgentInstance
	var bestScore float64 = -1

	for _, inst := range instances {
		if !inst.CanAcceptWork() {
			continue
		}

		score := b.calculateScore(inst.ID)
		if bestScore == -1 || score > bestScore {
			bestScore = score
			selected = inst
		}
	}

	if selected == nil {
		return nil, ErrNoHealthyInstances
	}

	return selected, nil
}

func (b *WeightedResponseTimeBalancer) calculateScore(instanceID string) float64 {
	stats, ok := b.responseTimes.Get(instanceID)
	if !ok || atomic.LoadInt64(&stats.count) == 0 {
		// No data, give average score
		return 0.5
	}

	// Load atomically so a concurrent ReportResult cannot race us.
	count := atomic.LoadInt64(&stats.count)
	totalTime := atomic.LoadInt64(&stats.totalTime)
	successes := atomic.LoadInt64(&stats.successes)

	// Calculate metrics
	avgTime := float64(totalTime) / float64(count)
	successRate := float64(successes) / float64(count)

	// Score: higher is better
	// Factor in both response time and success rate
	// Normalize: assume 5s (5000ms) is worst case
	timeScore := 1.0 - (avgTime / 5000.0)
	if timeScore < 0 {
		timeScore = 0
	}

	// Weight success rate more heavily
	return (timeScore * 0.3) + (successRate * 0.7)
}

// ReportResult updates statistics.
func (b *WeightedResponseTimeBalancer) ReportResult(instanceID string, success bool, duration int64) {
	// Atomic get-or-create: racing first-callers converge on the same
	// stats pointer, and all subsequent updates are lock-free via
	// atomic ops on the pointed-to struct.
	fresh := &responseTimeStats{}
	stats, _ := b.responseTimes.PutIfAbsent(instanceID, fresh)

	atomic.AddInt64(&stats.totalTime, duration)
	atomic.AddInt64(&stats.count, 1)
	if success {
		atomic.AddInt64(&stats.successes, 1)
	} else {
		atomic.AddInt64(&stats.failures, 1)
	}
}

// GetStats returns balancer statistics.
func (b *WeightedResponseTimeBalancer) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	b.responseTimes.Range(func(id string, rt *responseTimeStats) bool {
		count := atomic.LoadInt64(&rt.count)
		if count > 0 {
			totalTime := atomic.LoadInt64(&rt.totalTime)
			stats[id] = map[string]interface{}{
				"avg_response_time_ms": float64(totalTime) / float64(count),
				"success_rate":         float64(atomic.LoadInt64(&rt.successes)) / float64(count),
				"total_requests":       count,
			}
		}
		return true
	})

	return map[string]interface{}{
		"type":  "weighted_response_time",
		"stats": stats,
	}
}

// PriorityBalancer selects instances based on priority configuration.
type PriorityBalancer struct {
	// Priority order of agent types (first = highest priority)
	priorityOrder []clis.AgentType
}

// NewPriorityBalancer creates a new priority-based balancer.
func NewPriorityBalancer(priorityOrder []clis.AgentType) *PriorityBalancer {
	return &PriorityBalancer{
		priorityOrder: priorityOrder,
	}
}

// SelectInstance selects based on priority order.
func (b *PriorityBalancer) SelectInstance(
	instances []*clis.AgentInstance,
) (*clis.AgentInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstancesAvailable
	}

	// Build type -> instances map
	byType := make(map[clis.AgentType][]*clis.AgentInstance)
	for _, inst := range instances {
		if inst.CanAcceptWork() {
			byType[inst.Type] = append(byType[inst.Type], inst)
		}
	}

	// Check priority order
	for _, agentType := range b.priorityOrder {
		if typeInstances, ok := byType[agentType]; ok && len(typeInstances) > 0 {
			// Round-robin within type
			return typeInstances[0], nil
		}
	}

	// No priority match, return first available
	for _, inst := range instances {
		if inst.CanAcceptWork() {
			return inst, nil
		}
	}

	return nil, ErrNoHealthyInstances
}

// ReportResult is a no-op for priority balancer.
func (b *PriorityBalancer) ReportResult(instanceID string, success bool, duration int64) {
}

// GetStats returns balancer statistics.
func (b *PriorityBalancer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":           "priority",
		"priority_order": b.priorityOrder,
	}
}

// Error definitions
var (
	ErrNoInstancesAvailable = fmtError("no instances available")
	ErrNoHealthyInstances   = fmtError("no healthy instances available")
)

func fmtError(msg string) error {
	return &balancerError{msg: msg}
}

type balancerError struct {
	msg string
}

func (e *balancerError) Error() string {
	return e.msg
}
