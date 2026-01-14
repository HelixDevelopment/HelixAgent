package services

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

// Package-level metrics (registered once)
var (
	phmMetricsOnce            sync.Once
	phmHealthCheckGauge       *prometheus.GaugeVec
	phmHealthCheckDuration    *prometheus.HistogramVec
	phmUnhealthyProvidersGauge prometheus.Gauge
	phmHealthAlertsTotal      prometheus.Counter
)

func initPHMMetrics() {
	phmMetricsOnce.Do(func() {
		phmHealthCheckGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helixagent_provider_health",
				Help: "Health status of providers (1=healthy, 0=unhealthy)",
			},
			[]string{"provider"},
		)

		phmHealthCheckDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helixagent_provider_health_check_duration_seconds",
				Help:    "Duration of provider health checks",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider"},
		)

		phmUnhealthyProvidersGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "helixagent_unhealthy_providers",
				Help: "Number of unhealthy providers",
			},
		)

		phmHealthAlertsTotal = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "helixagent_provider_health_alerts_total",
				Help: "Total number of provider health alerts",
			},
		)
	})
}

// ProviderHealthMonitor performs periodic health checks on all providers
type ProviderHealthMonitor struct {
	mu              sync.RWMutex
	registry        *ProviderRegistry
	logger          *logrus.Logger
	checkInterval   time.Duration
	healthTimeout   time.Duration
	listeners       []ProviderHealthAlertListener
	stopCh          chan struct{}
	running         bool

	// Health status cache
	healthStatus map[string]*MonitoredProviderHealth
}

// ProviderHealthAlertListener is called when health alerts occur
type ProviderHealthAlertListener func(alert ProviderHealthAlert)

// ProviderHealthAlert represents a health alert
type ProviderHealthAlert struct {
	Type          string    `json:"type"`
	ProviderID    string    `json:"provider_id"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
	ConsecutiveFails int    `json:"consecutive_fails,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

// MonitoredProviderHealth represents the health status of a provider from the monitor
type MonitoredProviderHealth struct {
	ProviderID       string        `json:"provider_id"`
	Healthy          bool          `json:"healthy"`
	LastCheck        time.Time     `json:"last_check"`
	LastSuccess      time.Time     `json:"last_success,omitempty"`
	LastError        string        `json:"last_error,omitempty"`
	ConsecutiveFails int           `json:"consecutive_fails"`
	ResponseTime     time.Duration `json:"response_time,omitempty"`
	CheckCount       int64         `json:"check_count"`
	FailCount        int64         `json:"fail_count"`
}

// ProviderHealthMonitorConfig configures the monitor
type ProviderHealthMonitorConfig struct {
	CheckInterval time.Duration
	HealthTimeout time.Duration
	AlertAfterFails int // Alert after this many consecutive failures
}

// DefaultProviderHealthMonitorConfig returns default configuration
func DefaultProviderHealthMonitorConfig() ProviderHealthMonitorConfig {
	return ProviderHealthMonitorConfig{
		CheckInterval:   30 * time.Second,
		HealthTimeout:   10 * time.Second,
		AlertAfterFails: 3,
	}
}

// NewProviderHealthMonitor creates a new provider health monitor
func NewProviderHealthMonitor(registry *ProviderRegistry, logger *logrus.Logger, config ProviderHealthMonitorConfig) *ProviderHealthMonitor {
	// Initialize package-level metrics (idempotent)
	initPHMMetrics()

	return &ProviderHealthMonitor{
		registry:      registry,
		logger:        logger,
		checkInterval: config.CheckInterval,
		healthTimeout: config.HealthTimeout,
		listeners:     make([]ProviderHealthAlertListener, 0),
		stopCh:        make(chan struct{}),
		healthStatus:  make(map[string]*MonitoredProviderHealth),
	}
}

// AddAlertListener adds a listener for alerts
func (phm *ProviderHealthMonitor) AddAlertListener(listener ProviderHealthAlertListener) {
	phm.mu.Lock()
	defer phm.mu.Unlock()
	phm.listeners = append(phm.listeners, listener)
}

// Start starts the monitoring loop
func (phm *ProviderHealthMonitor) Start(ctx context.Context) {
	phm.mu.Lock()
	if phm.running {
		phm.mu.Unlock()
		return
	}
	phm.running = true
	phm.stopCh = make(chan struct{})
	phm.mu.Unlock()

	phm.logger.Info("Provider health monitor started")

	// Initial check
	phm.checkAllProviders(ctx)

	ticker := time.NewTicker(phm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			phm.logger.Info("Provider health monitor stopped (context cancelled)")
			return
		case <-phm.stopCh:
			phm.logger.Info("Provider health monitor stopped")
			return
		case <-ticker.C:
			phm.checkAllProviders(ctx)
		}
	}
}

// Stop stops the monitoring loop
func (phm *ProviderHealthMonitor) Stop() {
	phm.mu.Lock()
	defer phm.mu.Unlock()

	if phm.running {
		close(phm.stopCh)
		phm.running = false
	}
}

// checkAllProviders checks the health of all registered providers
func (phm *ProviderHealthMonitor) checkAllProviders(ctx context.Context) {
	if phm.registry == nil {
		return
	}

	providers := phm.registry.ListProviders()
	unhealthyCount := 0

	var wg sync.WaitGroup
	for _, providerID := range providers {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			phm.checkProvider(ctx, id)
		}(providerID)
	}
	wg.Wait()

	// Count unhealthy providers
	phm.mu.RLock()
	for _, status := range phm.healthStatus {
		if !status.Healthy {
			unhealthyCount++
		}
	}
	phm.mu.RUnlock()

	phmUnhealthyProvidersGauge.Set(float64(unhealthyCount))

	phm.logger.WithFields(logrus.Fields{
		"total":     len(providers),
		"unhealthy": unhealthyCount,
	}).Debug("Provider health check completed")
}

// checkProvider checks the health of a specific provider
func (phm *ProviderHealthMonitor) checkProvider(ctx context.Context, providerID string) {
	if phm.registry == nil {
		phm.updateStatus(providerID, false, "registry is nil", 0)
		return
	}

	provider, err := phm.registry.GetProvider(providerID)
	if err != nil {
		phm.updateStatus(providerID, false, err.Error(), 0)
		return
	}

	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, phm.healthTimeout)
	defer cancel()

	startTime := time.Now()

	// Perform health check
	var healthErr error
	done := make(chan error, 1)
	go func() {
		done <- provider.HealthCheck()
	}()

	select {
	case healthErr = <-done:
		// Health check completed
	case <-checkCtx.Done():
		healthErr = checkCtx.Err()
	}

	responseTime := time.Since(startTime)
	phmHealthCheckDuration.WithLabelValues(providerID).Observe(responseTime.Seconds())

	if healthErr != nil {
		phm.updateStatus(providerID, false, healthErr.Error(), responseTime)
	} else {
		phm.updateStatus(providerID, true, "", responseTime)
	}
}

// updateStatus updates the health status of a provider
func (phm *ProviderHealthMonitor) updateStatus(providerID string, healthy bool, errMsg string, responseTime time.Duration) {
	phm.mu.Lock()
	defer phm.mu.Unlock()

	status, exists := phm.healthStatus[providerID]
	if !exists {
		status = &MonitoredProviderHealth{
			ProviderID: providerID,
		}
		phm.healthStatus[providerID] = status
	}

	status.LastCheck = time.Now()
	status.CheckCount++
	status.ResponseTime = responseTime

	if healthy {
		status.Healthy = true
		status.LastSuccess = time.Now()
		status.LastError = ""
		status.ConsecutiveFails = 0
		phmHealthCheckGauge.WithLabelValues(providerID).Set(1)
	} else {
		status.Healthy = false
		status.LastError = errMsg
		status.ConsecutiveFails++
		status.FailCount++
		phmHealthCheckGauge.WithLabelValues(providerID).Set(0)

		// Send alert after threshold
		if status.ConsecutiveFails == 3 {
			phm.sendAlert(ProviderHealthAlert{
				Type:             "provider_unhealthy",
				ProviderID:       providerID,
				Message:          "Provider has failed " + string(rune(status.ConsecutiveFails)) + " consecutive health checks",
				Timestamp:        time.Now(),
				ConsecutiveFails: status.ConsecutiveFails,
				LastError:        errMsg,
			})
		}
	}

	phm.logger.WithFields(logrus.Fields{
		"provider":          providerID,
		"healthy":           healthy,
		"response_time_ms":  responseTime.Milliseconds(),
		"consecutive_fails": status.ConsecutiveFails,
		"error":             errMsg,
	}).Debug("Provider health status updated")
}

// sendAlert sends an alert to all listeners
func (phm *ProviderHealthMonitor) sendAlert(alert ProviderHealthAlert) {
	phmHealthAlertsTotal.Inc()

	phm.mu.RLock()
	listeners := phm.listeners
	phm.mu.RUnlock()

	for _, listener := range listeners {
		go listener(alert)
	}

	phm.logger.WithFields(logrus.Fields{
		"type":       alert.Type,
		"provider":   alert.ProviderID,
		"message":    alert.Message,
		"last_error": alert.LastError,
	}).Error("Provider health alert triggered")
}

// GetStatus returns the current health status of all providers
func (phm *ProviderHealthMonitor) GetStatus() ProviderHealthOverallStatus {
	phm.mu.RLock()
	defer phm.mu.RUnlock()

	providers := make(map[string]*MonitoredProviderHealth)
	healthyCount := 0
	unhealthyCount := 0

	for providerID, status := range phm.healthStatus {
		statusCopy := *status
		providers[providerID] = &statusCopy
		if status.Healthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	return ProviderHealthOverallStatus{
		Healthy:        unhealthyCount == 0,
		HealthyCount:   healthyCount,
		UnhealthyCount: unhealthyCount,
		TotalCount:     len(providers),
		Providers:      providers,
		CheckedAt:      time.Now(),
	}
}

// ProviderHealthOverallStatus represents the overall health status
type ProviderHealthOverallStatus struct {
	Healthy        bool                            `json:"healthy"`
	HealthyCount   int                             `json:"healthy_count"`
	UnhealthyCount int                             `json:"unhealthy_count"`
	TotalCount     int                             `json:"total_count"`
	Providers      map[string]*MonitoredProviderHealth `json:"providers"`
	CheckedAt      time.Time                       `json:"checked_at"`
}

// GetProviderStatus returns the health status of a specific provider
func (phm *ProviderHealthMonitor) GetProviderStatus(providerID string) (*MonitoredProviderHealth, bool) {
	phm.mu.RLock()
	defer phm.mu.RUnlock()

	status, exists := phm.healthStatus[providerID]
	if !exists {
		return nil, false
	}

	statusCopy := *status
	return &statusCopy, true
}

// ForceCheck forces an immediate health check of all providers
func (phm *ProviderHealthMonitor) ForceCheck(ctx context.Context) {
	phm.logger.Info("Forcing provider health check")
	phm.checkAllProviders(ctx)
}

// ForceCheckProvider forces an immediate health check of a specific provider
func (phm *ProviderHealthMonitor) ForceCheckProvider(ctx context.Context, providerID string) {
	phm.logger.WithField("provider", providerID).Info("Forcing provider health check")
	phm.checkProvider(ctx, providerID)
}
