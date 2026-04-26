package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

// Package-level metrics (registered once)
var (
	phmMetricsOnce             sync.Once
	phmHealthCheckGauge        *prometheus.GaugeVec
	phmHealthCheckDuration     *prometheus.HistogramVec
	phmUnhealthyProvidersGauge prometheus.Gauge
	phmHealthAlertsTotal       prometheus.Counter
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

// ProviderHealthMonitor performs periodic health checks on all providers.
//
// Concurrency model (CONST-029): listeners is a *safe.Slice;
// healthStatus is a *safe.Store; running is atomic.Bool. All per-
// provider status mutations in updateStatus run through Store.Update
// with a copy-on-write closure so concurrent checks never see a
// half-written MonitoredProviderHealth.
type ProviderHealthMonitor struct {
	registry      *ProviderRegistry
	logger        *logrus.Logger
	checkInterval time.Duration
	healthTimeout time.Duration
	listeners     *safe.Slice[ProviderHealthAlertListener]
	stopCh        chan struct{}
	running       atomic.Bool

	// Health status cache
	healthStatus *safe.Store[string, *MonitoredProviderHealth]
}

// ProviderHealthAlertListener is called when health alerts occur
type ProviderHealthAlertListener func(alert ProviderHealthAlert)

// ProviderHealthAlert represents a health alert
type ProviderHealthAlert struct {
	Type             string    `json:"type"`
	ProviderID       string    `json:"provider_id"`
	Message          string    `json:"message"`
	Timestamp        time.Time `json:"timestamp"`
	ConsecutiveFails int       `json:"consecutive_fails,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
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
	// Tier categorizes the provider's verifier status (CONST-032 +
	// user requirement: "LLMsVerifier MUST be capable of filtering
	// providers and models properly"). Computed by deriveTier() from
	// the other fields. Possible values:
	//   - "verified":   has at least one successful health check;
	//                   primary chain candidate
	//   - "configured": registered + last_success unset and < N
	//                   consecutive fails; still attempted as fallback
	//                   (transient down OR not-yet-probed)
	//   - "dead":       LastError matches a terminal-auth pattern
	//                   (401/403/quota_exceeded/discontinued/no
	//                   subscription) OR consecutive_fails ≥ 5 with no
	//                   prior success; excluded from rotation; operator
	//                   should rotate the credential
	Tier string `json:"tier"`
}

// deriveTier categorizes a provider for operator triage.
// See MonitoredProviderHealth.Tier for the taxonomy.
func deriveTier(h *MonitoredProviderHealth) string {
	if h == nil {
		return "unknown"
	}
	// Dead: terminal auth signal in last error message.
	if h.LastError != "" {
		low := strings.ToLower(h.LastError)
		if strings.Contains(low, "401") ||
			strings.Contains(low, "403") ||
			strings.Contains(low, "unauthorized") ||
			strings.Contains(low, "forbidden") ||
			strings.Contains(low, "no active") && strings.Contains(low, "subscription") ||
			strings.Contains(low, "discontinued") ||
			strings.Contains(low, "quota_exceeded") ||
			strings.Contains(low, "tokens per day") ||
			strings.Contains(low, "insufficient balance") {
			return "dead"
		}
	}
	// Dead by sustained-failure heuristic: 5+ consecutive fails AND no
	// success ever recorded. Genuinely transient providers either
	// recover or have a prior LastSuccess.
	if h.ConsecutiveFails >= 5 && h.LastSuccess.IsZero() {
		return "dead"
	}
	// Verified: at least one successful health check on record.
	if !h.LastSuccess.IsZero() {
		return "verified"
	}
	// Configured: registered, no success yet, but not (yet) terminal.
	return "configured"
}

// ProviderHealthMonitorConfig configures the monitor
type ProviderHealthMonitorConfig struct {
	CheckInterval   time.Duration
	HealthTimeout   time.Duration
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
		listeners:     safe.NewSlice[ProviderHealthAlertListener](),
		stopCh:        make(chan struct{}),
		healthStatus:  safe.NewStore[string, *MonitoredProviderHealth](),
	}
}

// AddAlertListener adds a listener for alerts
func (phm *ProviderHealthMonitor) AddAlertListener(listener ProviderHealthAlertListener) {
	phm.listeners.Append(listener)
}

// Start starts the monitoring loop
func (phm *ProviderHealthMonitor) Start(ctx context.Context) {
	if !phm.running.CompareAndSwap(false, true) {
		return
	}

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
	if phm.running.CompareAndSwap(true, false) {
		close(phm.stopCh)
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
	phm.healthStatus.Range(func(_ string, status *MonitoredProviderHealth) bool {
		if !status.Healthy {
			unhealthyCount++
		}
		return true
	})

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
	var shouldAlert bool
	var alertData ProviderHealthAlert
	var consecutiveFails int

	// Update status under Store.Update closure — atomic read-mutate-commit.
	phm.healthStatus.Update(providerID, func(cur *MonitoredProviderHealth, present bool) (*MonitoredProviderHealth, bool) {
		var status MonitoredProviderHealth
		if present {
			status = *cur
		} else {
			status.ProviderID = providerID
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

			// Prepare alert after threshold (will send after Update commits).
			if status.ConsecutiveFails == 3 {
				shouldAlert = true
				alertData = ProviderHealthAlert{
					Type:             "provider_unhealthy",
					ProviderID:       providerID,
					Message:          fmt.Sprintf("Provider has failed %d consecutive health checks", status.ConsecutiveFails),
					Timestamp:        time.Now(),
					ConsecutiveFails: status.ConsecutiveFails,
					LastError:        errMsg,
				}
			}
		}
		consecutiveFails = status.ConsecutiveFails
		return &status, true
	})

	// Send alert outside of lock to prevent deadlock
	if shouldAlert {
		phm.sendAlert(alertData)
	}

	phm.logger.WithFields(logrus.Fields{
		"provider":          providerID,
		"healthy":           healthy,
		"response_time_ms":  responseTime.Milliseconds(),
		"consecutive_fails": consecutiveFails,
		"error":             errMsg,
	}).Debug("Provider health status updated")
}

// isUnconfiguredError checks if an error indicates the provider is not configured (vs actual failure)
func isProviderUnconfiguredError(errMsg string) bool {
	unconfiguredPhrases := []string{
		"api key is required",
		"api key not set",
		"api key is invalid or expired",
		"key not configured",
		"credentials not found",
		"unauthorized",
		"401",
	}
	errLower := strings.ToLower(errMsg)
	for _, phrase := range unconfiguredPhrases {
		if strings.Contains(errLower, phrase) {
			return true
		}
	}
	return false
}

// sendAlert sends an alert to all listeners
func (phm *ProviderHealthMonitor) sendAlert(alert ProviderHealthAlert) {
	phmHealthAlertsTotal.Inc()

	phm.listeners.Range(func(_ int, listener ProviderHealthAlertListener) bool {
		go listener(alert)
		return true
	})

	// Use appropriate log level based on error type
	// Unconfigured providers are warnings, not errors
	logLevel := logrus.ErrorLevel
	if isProviderUnconfiguredError(alert.LastError) {
		logLevel = logrus.WarnLevel
		alert.Type = "provider_unconfigured"
	}

	phm.logger.WithFields(logrus.Fields{
		"type":       alert.Type,
		"provider":   alert.ProviderID,
		"message":    alert.Message,
		"last_error": alert.LastError,
	}).Log(logLevel, "Provider health alert triggered")
}

// GetStatus returns the current health status of all providers
func (phm *ProviderHealthMonitor) GetStatus() ProviderHealthOverallStatus {
	providers := make(map[string]*MonitoredProviderHealth)
	healthyCount := 0
	unhealthyCount := 0
	tierCounts := VerifierTierSummary{}

	phm.healthStatus.Range(func(providerID string, status *MonitoredProviderHealth) bool {
		statusCopy := *status
		statusCopy.Tier = deriveTier(&statusCopy)
		providers[providerID] = &statusCopy
		if status.Healthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
		switch statusCopy.Tier {
		case "verified":
			tierCounts.Verified++
		case "configured":
			tierCounts.Configured++
		case "dead":
			tierCounts.Dead++
		default:
			tierCounts.Unknown++
		}
		return true
	})

	tierCounts.Total = len(providers)

	return ProviderHealthOverallStatus{
		Healthy:         unhealthyCount == 0,
		HealthyCount:    healthyCount,
		UnhealthyCount:  unhealthyCount,
		TotalCount:      len(providers),
		Providers:       providers,
		CheckedAt:       time.Now(),
		VerifierSummary: tierCounts,
	}
}

// VerifierTierSummary is the operator-facing roll-up of verifier
// classifications across all providers. Surfaced at the top level of
// /v1/monitoring/status (as `verifier_summary`) so operators can see
// at a glance how many keys need rotation.
type VerifierTierSummary struct {
	Verified   int `json:"verified"`
	Configured int `json:"configured"`
	Dead       int `json:"dead"`
	Unknown    int `json:"unknown,omitempty"`
	Total      int `json:"total"`
}

// ProviderHealthOverallStatus represents the overall health status
type ProviderHealthOverallStatus struct {
	Healthy         bool                                `json:"healthy"`
	HealthyCount    int                                 `json:"healthy_count"`
	UnhealthyCount  int                                 `json:"unhealthy_count"`
	TotalCount      int                                 `json:"total_count"`
	Providers       map[string]*MonitoredProviderHealth `json:"providers"`
	CheckedAt       time.Time                           `json:"checked_at"`
	VerifierSummary VerifierTierSummary                 `json:"verifier_summary"`
}

// GetProviderStatus returns the health status of a specific provider
func (phm *ProviderHealthMonitor) GetProviderStatus(providerID string) (*MonitoredProviderHealth, bool) {
	status, exists := phm.healthStatus.Get(providerID)
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
