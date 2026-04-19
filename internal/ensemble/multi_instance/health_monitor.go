// Package multi_instance provides multi-instance ensemble coordination for HelixAgent.
package multi_instance

import (
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis"
)

// HealthMonitor tracks instance health and makes routing decisions.
//
// Concurrent-safe by construction (CONST-029): history is a safe.Store;
// HealthHistory field mutations route through Update callbacks
// (Pattern Beta).
type HealthMonitor struct {
	history *safe.Store[string, *HealthHistory]

	// Thresholds
	failureThreshold  int
	degradedThreshold float64
	recoveryThreshold float64

	// Window size for health calculations
	historyWindow time.Duration
}

// HealthHistory tracks health metrics over time.
type HealthHistory struct {
	InstanceID string

	// Health checks
	Checks []*HealthCheck

	// Calculated metrics
	FailureRate     float64
	AvgResponseTime time.Duration
	LastCheck       time.Time

	// State
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	Timestamp time.Time
	Healthy   bool
	Duration  time.Duration
	Error     error
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		history:           safe.NewStore[string, *HealthHistory](),
		failureThreshold:  3,
		degradedThreshold: 0.5,
		recoveryThreshold: 0.8,
		historyWindow:     5 * time.Minute,
	}
}

// RecordCheck records a health check result.
func (m *HealthMonitor) RecordCheck(instanceID string, healthy bool, duration time.Duration, err error) {
	m.history.Update(instanceID, func(history *HealthHistory, _ bool) (*HealthHistory, bool) {
		if history == nil {
			history = &HealthHistory{
				InstanceID: instanceID,
				Checks:     make([]*HealthCheck, 0),
			}
		}

		check := &HealthCheck{
			Timestamp: time.Now(),
			Healthy:   healthy,
			Duration:  duration,
			Error:     err,
		}
		history.Checks = append(history.Checks, check)
		history.LastCheck = time.Now()

		if healthy {
			history.ConsecutiveSuccesses++
			history.ConsecutiveFailures = 0
		} else {
			history.ConsecutiveFailures++
			history.ConsecutiveSuccesses = 0
		}

		m.cleanupOldChecks(history)
		m.recalculateMetrics(history)
		return history, true
	})
}

// GetHealth returns the current health status for an instance.
func (m *HealthMonitor) GetHealth(instanceID string) clis.HealthStatus {
	var status clis.HealthStatus = clis.HealthUnknown
	m.history.Update(instanceID, func(history *HealthHistory, ok bool) (*HealthHistory, bool) {
		if !ok {
			return nil, false
		}
		status = m.calculateHealthStatus(history)
		return history, true
	})
	return status
}

// GetRecommendation returns routing recommendation for an instance.
func (m *HealthMonitor) GetRecommendation(instanceID string) HealthRecommendation {
	var rec HealthRecommendation = HealthRecommendation{
		CanRoute: true,
		Weight:   1.0,
		Reason:   "no health history",
	}
	m.history.Update(instanceID, func(history *HealthHistory, ok bool) (*HealthHistory, bool) {
		if !ok {
			return nil, false
		}
		status := m.calculateHealthStatus(history)
		switch status {
		case clis.HealthHealthy:
			rec = HealthRecommendation{CanRoute: true, Weight: 1.0, Reason: "healthy"}
		case clis.HealthDegraded:
			weight := 1.0 - history.FailureRate
			if weight < 0.1 {
				weight = 0.1
			}
			rec = HealthRecommendation{CanRoute: true, Weight: weight, Reason: "degraded"}
		case clis.HealthUnhealthy:
			rec = HealthRecommendation{CanRoute: false, Weight: 0.0, Reason: "unhealthy"}
		default:
			rec = HealthRecommendation{CanRoute: true, Weight: 0.5, Reason: "unknown health"}
		}
		return history, true
	})
	return rec
}

// HealthRecommendation provides routing guidance.
type HealthRecommendation struct {
	CanRoute bool
	Weight   float64
	Reason   string
}

// GetStats returns health statistics for all instances.
func (m *HealthMonitor) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	m.history.Range(func(id string, history *HealthHistory) bool {
		stats[id] = map[string]interface{}{
			"failure_rate":          history.FailureRate,
			"avg_response_time_ms":  history.AvgResponseTime.Milliseconds(),
			"consecutive_failures":  history.ConsecutiveFailures,
			"consecutive_successes": history.ConsecutiveSuccesses,
			"total_checks":          len(history.Checks),
			"last_check":            history.LastCheck,
		}
		return true
	})
	return stats
}

// RemoveInstance removes an instance from monitoring.
func (m *HealthMonitor) RemoveInstance(instanceID string) {
	m.history.Delete(instanceID)
}

// cleanupOldChecks removes checks outside the history window.
func (m *HealthMonitor) cleanupOldChecks(history *HealthHistory) {
	cutoff := time.Now().Add(-m.historyWindow)
	var kept []*HealthCheck
	for _, check := range history.Checks {
		if check.Timestamp.After(cutoff) {
			kept = append(kept, check)
		}
	}
	history.Checks = kept
}

// recalculateMetrics recalculates health metrics.
func (m *HealthMonitor) recalculateMetrics(history *HealthHistory) {
	if len(history.Checks) == 0 {
		history.FailureRate = 0
		history.AvgResponseTime = 0
		return
	}

	// Calculate failure rate
	failures := 0
	var totalDuration time.Duration

	for _, check := range history.Checks {
		if !check.Healthy {
			failures++
		}
		totalDuration += check.Duration
	}

	history.FailureRate = float64(failures) / float64(len(history.Checks))
	history.AvgResponseTime = totalDuration / time.Duration(len(history.Checks))
}

// calculateHealthStatus determines health status from metrics.
func (m *HealthMonitor) calculateHealthStatus(history *HealthHistory) clis.HealthStatus {
	// Check consecutive failures first
	if history.ConsecutiveFailures >= m.failureThreshold {
		return clis.HealthUnhealthy
	}

	// Check failure rate
	if history.FailureRate >= m.degradedThreshold {
		// Check if recovering
		if history.ConsecutiveSuccesses >= m.failureThreshold {
			return clis.HealthDegraded // Still degraded but recovering
		}
		return clis.HealthDegraded
	}

	// Check recovery
	if history.ConsecutiveSuccesses >= m.failureThreshold {
		return clis.HealthHealthy
	}

	// Default based on failure rate
	if history.FailureRate < 0.1 {
		return clis.HealthHealthy
	}

	return clis.HealthDegraded
}

// CircuitBreaker implements circuit breaker pattern for instance routing.
type CircuitBreaker struct {
	instanceID string

	// Thresholds
	failureThreshold int
	successThreshold int
	timeout          time.Duration

	// State
	state           CircuitState
	failures        int
	successes       int
	lastFailure     time.Time
	lastStateChange time.Time

	mu sync.RWMutex
}

// CircuitState represents circuit breaker state.
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Failing fast
	StateHalfOpen                     // Testing recovery
)

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(instanceID string) *CircuitBreaker {
	return &CircuitBreaker{
		instanceID:       instanceID,
		failureThreshold: 5,
		successThreshold: 3,
		timeout:          30 * time.Second,
		state:            StateClosed,
		lastStateChange:  time.Now(),
	}
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			cb.failures = 0
			cb.successes = 0
			cb.lastStateChange = time.Now()
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successes++

	switch cb.state {
	case StateHalfOpen:
		if cb.successes >= cb.successThreshold {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
			cb.lastStateChange = time.Now()
		}

	case StateClosed:
		// Reset failures on success in closed state
		if cb.failures > 0 {
			cb.failures = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateHalfOpen:
		// Immediately trip in half-open state
		cb.state = StateOpen
		cb.successes = 0
		cb.lastStateChange = time.Now()

	case StateClosed:
		if cb.failures >= cb.failureThreshold {
			cb.state = StateOpen
			cb.successes = 0
			cb.lastStateChange = time.Now()
		}
	}
}

// GetState returns current circuit state.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics.
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stateName := "closed"
	switch cb.state {
	case StateOpen:
		stateName = "open"
	case StateHalfOpen:
		stateName = "half_open"
	}

	return map[string]interface{}{
		"instance_id":       cb.instanceID,
		"state":             stateName,
		"failures":          cb.failures,
		"successes":         cb.successes,
		"last_failure":      cb.lastFailure,
		"last_state_change": cb.lastStateChange,
	}
}

// CircuitBreakerManager manages circuit breakers for all instances.
//
// Concurrent-safe by construction (CONST-029): breakers is a safe.Store.
// CircuitBreaker has its own internal mutex (no collection adjacent —
// audit-clean).
type CircuitBreakerManager struct {
	breakers *safe.Store[string, *CircuitBreaker]
}

// NewCircuitBreakerManager creates a new manager.
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: safe.NewStore[string, *CircuitBreaker](),
	}
}

// GetBreaker gets or creates a circuit breaker.
func (m *CircuitBreakerManager) GetBreaker(instanceID string) *CircuitBreaker {
	if cb, ok := m.breakers.Get(instanceID); ok {
		return cb
	}
	cb, _ := m.breakers.PutIfAbsent(instanceID, NewCircuitBreaker(instanceID))
	return cb
}

// RemoveBreaker removes a circuit breaker.
func (m *CircuitBreakerManager) RemoveBreaker(instanceID string) {
	m.breakers.Delete(instanceID)
}

// GetAllStats returns stats for all breakers.
func (m *CircuitBreakerManager) GetAllStats() map[string]interface{} {
	stats := make(map[string]interface{})
	m.breakers.Range(func(id string, cb *CircuitBreaker) bool {
		stats[id] = cb.GetStats()
		return true
	})
	return stats
}
