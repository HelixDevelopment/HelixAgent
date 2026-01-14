package services

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/llm"
)

// Package-level metrics (registered once)
var (
	cbmMetricsOnce           sync.Once
	cbmCircuitStateGauge     *prometheus.GaugeVec
	cbmCircuitFailuresTotal  *prometheus.CounterVec
	cbmOpenCircuitsGauge     prometheus.Gauge
	cbmAlertsTotal           prometheus.Counter
)

func initCBMMetrics() {
	cbmMetricsOnce.Do(func() {
		cbmCircuitStateGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helixagent_circuit_breaker_state",
				Help: "Current state of circuit breakers (0=closed, 1=half_open, 2=open)",
			},
			[]string{"provider"},
		)

		cbmCircuitFailuresTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helixagent_circuit_breaker_failures_total",
				Help: "Total number of circuit breaker failures",
			},
			[]string{"provider"},
		)

		cbmOpenCircuitsGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "helixagent_circuit_breakers_open",
				Help: "Number of open circuit breakers",
			},
		)

		cbmAlertsTotal = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "helixagent_circuit_breaker_alerts_total",
				Help: "Total number of circuit breaker alerts",
			},
		)
	})
}

// CircuitBreakerMonitor monitors circuit breaker states and provides alerts
type CircuitBreakerMonitor struct {
	mu              sync.RWMutex
	manager         *llm.CircuitBreakerManager
	logger          *logrus.Logger
	checkInterval   time.Duration
	alertThreshold  int // Number of open circuits to trigger alert
	listeners       []CircuitBreakerAlertListener
	stopCh          chan struct{}
	running         bool
}

// CircuitBreakerAlertListener is called when circuit breaker alerts occur
type CircuitBreakerAlertListener func(alert CircuitBreakerAlert)

// CircuitBreakerAlert represents an alert from the monitor
type CircuitBreakerAlert struct {
	Type        string                        `json:"type"`
	ProviderID  string                        `json:"provider_id,omitempty"`
	OldState    llm.CircuitState              `json:"old_state,omitempty"`
	NewState    llm.CircuitState              `json:"new_state,omitempty"`
	Message     string                        `json:"message"`
	Timestamp   time.Time                     `json:"timestamp"`
	OpenCount   int                           `json:"open_count,omitempty"`
	AllStats    map[string]llm.CircuitBreakerStats `json:"all_stats,omitempty"`
}

// CircuitBreakerMonitorConfig configures the monitor
type CircuitBreakerMonitorConfig struct {
	CheckInterval  time.Duration
	AlertThreshold int
}

// DefaultCircuitBreakerMonitorConfig returns default configuration
func DefaultCircuitBreakerMonitorConfig() CircuitBreakerMonitorConfig {
	return CircuitBreakerMonitorConfig{
		CheckInterval:  10 * time.Second,
		AlertThreshold: 3, // Alert when 3+ circuits are open
	}
}

// NewCircuitBreakerMonitor creates a new circuit breaker monitor
func NewCircuitBreakerMonitor(manager *llm.CircuitBreakerManager, logger *logrus.Logger, config CircuitBreakerMonitorConfig) *CircuitBreakerMonitor {
	// Initialize package-level metrics (idempotent)
	initCBMMetrics()

	return &CircuitBreakerMonitor{
		manager:        manager,
		logger:         logger,
		checkInterval:  config.CheckInterval,
		alertThreshold: config.AlertThreshold,
		listeners:      make([]CircuitBreakerAlertListener, 0),
		stopCh:         make(chan struct{}),
	}
}

// AddAlertListener adds a listener for alerts
func (cbm *CircuitBreakerMonitor) AddAlertListener(listener CircuitBreakerAlertListener) {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	cbm.listeners = append(cbm.listeners, listener)
}

// Start starts the monitoring loop
func (cbm *CircuitBreakerMonitor) Start(ctx context.Context) {
	cbm.mu.Lock()
	if cbm.running {
		cbm.mu.Unlock()
		return
	}
	cbm.running = true
	cbm.stopCh = make(chan struct{})
	cbm.mu.Unlock()

	cbm.logger.Info("Circuit breaker monitor started")

	// Register state change listeners with the manager
	cbm.registerStateChangeListeners()

	ticker := time.NewTicker(cbm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cbm.logger.Info("Circuit breaker monitor stopped (context cancelled)")
			return
		case <-cbm.stopCh:
			cbm.logger.Info("Circuit breaker monitor stopped")
			return
		case <-ticker.C:
			cbm.checkCircuitBreakers()
		}
	}
}

// Stop stops the monitoring loop
func (cbm *CircuitBreakerMonitor) Stop() {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if cbm.running {
		close(cbm.stopCh)
		cbm.running = false
	}
}

// registerStateChangeListeners registers listeners for circuit state changes
func (cbm *CircuitBreakerMonitor) registerStateChangeListeners() {
	// Note: This would require access to individual circuit breakers
	// For now, we rely on periodic checks
}

// checkCircuitBreakers checks all circuit breakers and sends alerts if needed
func (cbm *CircuitBreakerMonitor) checkCircuitBreakers() {
	if cbm.manager == nil {
		return
	}

	stats := cbm.manager.GetAllStats()
	openCount := 0
	halfOpenCount := 0

	for providerID, stat := range stats {
		// Update Prometheus metrics
		stateValue := float64(0)
		switch stat.State {
		case llm.CircuitClosed:
			stateValue = 0
		case llm.CircuitHalfOpen:
			stateValue = 1
			halfOpenCount++
		case llm.CircuitOpen:
			stateValue = 2
			openCount++
		}
		cbmCircuitStateGauge.WithLabelValues(providerID).Set(stateValue)
		cbmCircuitFailuresTotal.WithLabelValues(providerID).Add(float64(stat.TotalFailures))

		// Log warning for open circuits
		if stat.State == llm.CircuitOpen {
			cbm.logger.WithFields(logrus.Fields{
				"provider":             providerID,
				"state":                stat.State,
				"consecutive_failures": stat.ConsecutiveFailures,
				"last_failure":         stat.LastFailure,
			}).Warn("Circuit breaker is OPEN")
		}
	}

	cbmOpenCircuitsGauge.Set(float64(openCount))

	// Send alert if threshold exceeded
	if openCount >= cbm.alertThreshold {
		cbm.sendAlert(CircuitBreakerAlert{
			Type:      "threshold_exceeded",
			Message:   "Multiple circuit breakers are open",
			Timestamp: time.Now(),
			OpenCount: openCount,
			AllStats:  stats,
		})
	}

	cbm.logger.WithFields(logrus.Fields{
		"total":     len(stats),
		"open":      openCount,
		"half_open": halfOpenCount,
		"closed":    len(stats) - openCount - halfOpenCount,
	}).Debug("Circuit breaker status check completed")
}

// sendAlert sends an alert to all listeners
func (cbm *CircuitBreakerMonitor) sendAlert(alert CircuitBreakerAlert) {
	cbmAlertsTotal.Inc()

	cbm.mu.RLock()
	listeners := cbm.listeners
	cbm.mu.RUnlock()

	for _, listener := range listeners {
		go listener(alert)
	}

	cbm.logger.WithFields(logrus.Fields{
		"type":       alert.Type,
		"message":    alert.Message,
		"open_count": alert.OpenCount,
	}).Error("Circuit breaker alert triggered")
}

// GetStatus returns the current status of all circuit breakers
func (cbm *CircuitBreakerMonitor) GetStatus() CircuitBreakerStatus {
	if cbm.manager == nil {
		return CircuitBreakerStatus{
			Healthy:   true,
			Providers: make(map[string]CircuitBreakerProviderStatus),
		}
	}

	stats := cbm.manager.GetAllStats()
	providers := make(map[string]CircuitBreakerProviderStatus)
	openCount := 0
	halfOpenCount := 0

	for providerID, stat := range stats {
		status := CircuitBreakerProviderStatus{
			State:                string(stat.State),
			TotalRequests:        stat.TotalRequests,
			TotalSuccesses:       stat.TotalSuccesses,
			TotalFailures:        stat.TotalFailures,
			ConsecutiveFailures:  stat.ConsecutiveFailures,
			ConsecutiveSuccesses: stat.ConsecutiveSuccesses,
			LastFailure:          stat.LastFailure,
			LastStateChange:      stat.LastStateChange,
			SuccessRate:          calculateSuccessRate(stat),
		}
		providers[providerID] = status

		if stat.State == llm.CircuitOpen {
			openCount++
		} else if stat.State == llm.CircuitHalfOpen {
			halfOpenCount++
		}
	}

	return CircuitBreakerStatus{
		Healthy:       openCount < cbm.alertThreshold,
		OpenCount:     openCount,
		HalfOpenCount: halfOpenCount,
		ClosedCount:   len(stats) - openCount - halfOpenCount,
		TotalCount:    len(stats),
		Providers:     providers,
		CheckedAt:     time.Now(),
	}
}

// CircuitBreakerStatus represents the overall status
type CircuitBreakerStatus struct {
	Healthy       bool                                   `json:"healthy"`
	OpenCount     int                                    `json:"open_count"`
	HalfOpenCount int                                    `json:"half_open_count"`
	ClosedCount   int                                    `json:"closed_count"`
	TotalCount    int                                    `json:"total_count"`
	Providers     map[string]CircuitBreakerProviderStatus `json:"providers"`
	CheckedAt     time.Time                              `json:"checked_at"`
}

// CircuitBreakerProviderStatus represents a single provider's status
type CircuitBreakerProviderStatus struct {
	State                string    `json:"state"`
	TotalRequests        int64     `json:"total_requests"`
	TotalSuccesses       int64     `json:"total_successes"`
	TotalFailures        int64     `json:"total_failures"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	ConsecutiveSuccesses int       `json:"consecutive_successes"`
	LastFailure          time.Time `json:"last_failure,omitempty"`
	LastStateChange      time.Time `json:"last_state_change"`
	SuccessRate          float64   `json:"success_rate"`
}

// ResetCircuitBreaker resets a specific circuit breaker
func (cbm *CircuitBreakerMonitor) ResetCircuitBreaker(providerID string) error {
	if cbm.manager == nil {
		return nil
	}

	cb, exists := cbm.manager.Get(providerID)
	if !exists {
		return nil
	}

	cb.Reset()
	cbm.logger.WithField("provider", providerID).Info("Circuit breaker reset")
	return nil
}

// ResetAllCircuitBreakers resets all circuit breakers
func (cbm *CircuitBreakerMonitor) ResetAllCircuitBreakers() {
	if cbm.manager == nil {
		return
	}

	cbm.manager.ResetAll()
	cbm.logger.Info("All circuit breakers reset")
}

// calculateSuccessRate calculates the success rate from stats
func calculateSuccessRate(stat llm.CircuitBreakerStats) float64 {
	if stat.TotalRequests == 0 {
		return 100.0
	}
	return float64(stat.TotalSuccesses) / float64(stat.TotalRequests) * 100.0
}
