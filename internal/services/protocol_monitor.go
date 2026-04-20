package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// systemMetricsCollector handles actual system metrics collection
type systemMetricsCollector struct {
	prevCPUStats cpuStats
	prevNetBytes int64
	initialized  bool
}

type cpuStats struct {
	user   uint64
	nice   uint64
	system uint64
	idle   uint64
	total  uint64
}

var metricsCollectorInstance = &systemMetricsCollector{}

// DefaultAlertHistoryLimit is the default maximum number of alerts to retain in history
const DefaultAlertHistoryLimit = 1000

// ProtocolMonitor provides performance monitoring and alerting for protocols.
//
// Concurrent-safe by construction (CONST-029):
//   - `metrics`, `alerts`, and `alertHistory` are safe.Store/safe.Slice.
//   - Per-*ProtocolMetrics field mutations are serialised by ProtocolMetrics.mu
//     (internal to that struct) so readers and writers of the same pointer
//     do not race even though the Store hands out the pointer directly.
//   - `alertLimitMu` is a narrow mutex that only serialises alertLimit
//     changes with the concurrent history-trim path in storeAlert; it
//     does not pair with any map/slice field (they are all safe.*).
type ProtocolMonitor struct {
	metrics      *safe.Store[string, *ProtocolMetrics]
	alerts       *safe.Slice[*AlertRule]
	alertChan    chan *Alert
	stopChan     chan struct{}
	logger       *logrus.Logger
	alertHistory *safe.Slice[*Alert]
	alertLimitMu sync.Mutex
	alertLimit   int
}

// ProtocolMetrics represents performance metrics for a protocol.
//
// Thread-safe: the unexported mu serialises all mutations and copying
// reads of the counter fields below. Callers obtain a pointer from the
// monitor's safe.Store and must take mu before mutating any field.
type ProtocolMetrics struct {
	mu                 sync.Mutex
	Protocol           string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	Throughput         float64 // requests per second
	LastRequestTime    time.Time
	ErrorRate          float64
	ActiveConnections  int
	CacheHitRate       float64
	ResourceUsage      SystemResourceUsage
}

// SystemResourceUsage represents system resource utilization
type SystemResourceUsage struct {
	MemoryMB     float64
	CPUPercent   float64
	NetworkBytes int64
	DiskUsageMB  float64
}

// AlertRule defines alerting conditions
type AlertRule struct {
	ID          string
	Name        string
	Description string
	Protocol    string
	Condition   AlertCondition
	Threshold   float64
	Severity    AlertSeverity
	Cooldown    time.Duration
	LastAlert   time.Time
	Enabled     bool
}

// AlertCondition defines when to trigger an alert
type AlertCondition int

const (
	ConditionGreaterThan AlertCondition = iota
	ConditionLessThan
	ConditionEqual
	ConditionRateAbove
	ConditionErrorRateAbove
	ConditionLatencyAbove
)

// AlertSeverity defines alert severity levels
type AlertSeverity int

const (
	SeverityInfo AlertSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// Alert represents an alert event
type Alert struct {
	ID         string
	RuleID     string
	Protocol   string
	Message    string
	Severity   AlertSeverity
	Value      float64
	Threshold  float64
	Timestamp  time.Time
	Resolved   bool
	ResolvedAt *time.Time
}

// AlertFilter defines filtering criteria for retrieving alerts
type AlertFilter struct {
	// Severity filters alerts by severity level (nil means all severities)
	Severity *AlertSeverity
	// Protocol filters alerts by protocol name (empty string means all protocols)
	Protocol string
	// StartTime filters alerts that occurred after this time (zero time means no lower bound)
	StartTime time.Time
	// EndTime filters alerts that occurred before this time (zero time means no upper bound)
	EndTime time.Time
	// Limit is the maximum number of alerts to return (0 means no limit)
	Limit int
	// IncludeResolved includes resolved alerts in results (default false means only unresolved)
	IncludeResolved bool
}

// NewProtocolMonitor creates a new protocol monitor
func NewProtocolMonitor(logger *logrus.Logger) *ProtocolMonitor {
	monitor := &ProtocolMonitor{
		metrics:      safe.NewStore[string, *ProtocolMetrics](),
		alerts:       safe.NewSlice[*AlertRule](),
		alertChan:    make(chan *Alert, 100),
		stopChan:     make(chan struct{}),
		logger:       logger,
		alertHistory: safe.NewSlice[*Alert](),
		alertLimit:   DefaultAlertHistoryLimit,
	}

	// Start monitoring goroutines
	go monitor.metricsCollector()
	go monitor.alertChecker()

	return monitor
}

// RecordRequest records a protocol request
func (m *ProtocolMonitor) RecordRequest(ctx context.Context, protocol string, duration time.Duration, success bool, errorMsg string) {
	metrics := m.getOrCreateMetrics(protocol, func() *ProtocolMetrics {
		return &ProtocolMetrics{
			Protocol:   protocol,
			MinLatency: time.Hour, // Initialize to a large value
		}
	})

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.TotalRequests++
	metrics.LastRequestTime = time.Now()

	if success {
		metrics.SuccessfulRequests++
	} else {
		metrics.FailedRequests++
	}

	// Update latency statistics
	metrics.AverageLatency = time.Duration(
		(int64(metrics.AverageLatency)*int64(metrics.TotalRequests-1) + int64(duration)) / int64(metrics.TotalRequests),
	)

	if duration < metrics.MinLatency {
		metrics.MinLatency = duration
	}
	if duration > metrics.MaxLatency {
		metrics.MaxLatency = duration
	}

	// Calculate error rate
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	}

	// Calculate throughput (requests per second over last minute)
	// This is a simplified calculation
	metrics.Throughput = float64(metrics.TotalRequests) / 60.0

	m.logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"duration": duration,
		"success":  success,
		"latency":  duration,
	}).Debug("Protocol request recorded")
}

// getOrCreateMetrics atomically fetches or creates the *ProtocolMetrics
// entry for the protocol. The returned pointer is safe to mutate through
// its own mu after the call returns.
func (m *ProtocolMonitor) getOrCreateMetrics(protocol string, factory func() *ProtocolMetrics) *ProtocolMetrics {
	var result *ProtocolMetrics
	m.metrics.Update(protocol, func(existing *ProtocolMetrics, present bool) (*ProtocolMetrics, bool) {
		if present {
			result = existing
			return existing, true
		}
		result = factory()
		return result, true
	})
	return result
}

// UpdateConnections updates connection count for a protocol
func (m *ProtocolMonitor) UpdateConnections(protocol string, count int) {
	metrics := m.getOrCreateMetrics(protocol, func() *ProtocolMetrics {
		return &ProtocolMetrics{Protocol: protocol}
	})
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.ActiveConnections = count
}

// UpdateCacheStats updates cache statistics
func (m *ProtocolMonitor) UpdateCacheStats(protocol string, hitRate float64) {
	metrics := m.getOrCreateMetrics(protocol, func() *ProtocolMetrics {
		return &ProtocolMetrics{Protocol: protocol}
	})
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.CacheHitRate = hitRate
}

// UpdateResourceUsage updates resource usage statistics
func (m *ProtocolMonitor) UpdateResourceUsage(protocol string, usage SystemResourceUsage) {
	metrics := m.getOrCreateMetrics(protocol, func() *ProtocolMetrics {
		return &ProtocolMetrics{Protocol: protocol}
	})
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.ResourceUsage = usage
}

// GetMetrics returns metrics for a protocol
func (m *ProtocolMonitor) GetMetrics(protocol string) (*ProtocolMetrics, error) {
	metrics, exists := m.metrics.Get(protocol)
	if !exists {
		return nil, fmt.Errorf("no metrics found for protocol: %s", protocol)
	}

	return copyMetricsLocked(metrics), nil
}

// GetAllMetrics returns metrics for all protocols
func (m *ProtocolMonitor) GetAllMetrics() map[string]*ProtocolMetrics {
	result := make(map[string]*ProtocolMetrics)
	m.metrics.Range(func(protocol string, metrics *ProtocolMetrics) bool {
		result[protocol] = copyMetricsLocked(metrics)
		return true
	})
	return result
}

// copyMetricsLocked takes metrics.mu and returns a deep copy (excluding mu).
// The returned pointer is safe to return to a caller — the new ProtocolMetrics
// has its own zero-value mu.
func copyMetricsLocked(metrics *ProtocolMetrics) *ProtocolMetrics {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	return &ProtocolMetrics{
		Protocol:           metrics.Protocol,
		TotalRequests:      metrics.TotalRequests,
		SuccessfulRequests: metrics.SuccessfulRequests,
		FailedRequests:     metrics.FailedRequests,
		AverageLatency:     metrics.AverageLatency,
		MinLatency:         metrics.MinLatency,
		MaxLatency:         metrics.MaxLatency,
		Throughput:         metrics.Throughput,
		LastRequestTime:    metrics.LastRequestTime,
		ErrorRate:          metrics.ErrorRate,
		ActiveConnections:  metrics.ActiveConnections,
		CacheHitRate:       metrics.CacheHitRate,
		ResourceUsage:      metrics.ResourceUsage,
	}
}

// AddAlertRule adds an alert rule
func (m *ProtocolMonitor) AddAlertRule(rule *AlertRule) {
	m.alerts.Append(rule)
	m.logger.WithFields(logrus.Fields{
		"ruleId":   rule.ID,
		"name":     rule.Name,
		"protocol": rule.Protocol,
	}).Info("Alert rule added")
}

// RemoveAlertRule removes an alert rule
func (m *ProtocolMonitor) RemoveAlertRule(ruleID string) {
	if _, removed := m.alerts.Delete(func(r *AlertRule) bool { return r.ID == ruleID }); removed {
		m.logger.WithField("ruleId", ruleID).Info("Alert rule removed")
	}
}

// GetAlerts returns recent alerts with optional limit (for backward compatibility)
func (m *ProtocolMonitor) GetAlerts(limit int) []*Alert {
	filter := &AlertFilter{
		Limit:           limit,
		IncludeResolved: true,
	}
	return m.GetAlertsFiltered(filter)
}

// GetAlertsFiltered returns alerts from stored history with filtering support
func (m *ProtocolMonitor) GetAlertsFiltered(filter *AlertFilter) []*Alert {
	if filter == nil {
		filter = &AlertFilter{}
	}

	history := m.alertHistory.Snapshot()
	result := make([]*Alert, 0)

	// Iterate in reverse order to get most recent alerts first
	for i := len(history) - 1; i >= 0; i-- {
		alert := history[i]

		// Apply filters
		if !m.matchesFilter(alert, filter) {
			continue
		}

		// Create a copy to avoid race conditions
		alertCopy := &Alert{
			ID:         alert.ID,
			RuleID:     alert.RuleID,
			Protocol:   alert.Protocol,
			Message:    alert.Message,
			Severity:   alert.Severity,
			Value:      alert.Value,
			Threshold:  alert.Threshold,
			Timestamp:  alert.Timestamp,
			Resolved:   alert.Resolved,
			ResolvedAt: alert.ResolvedAt,
		}
		result = append(result, alertCopy)

		// Check limit
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result
}

// matchesFilter checks if an alert matches the given filter criteria
func (m *ProtocolMonitor) matchesFilter(alert *Alert, filter *AlertFilter) bool {
	// Filter by resolved status
	if !filter.IncludeResolved && alert.Resolved {
		return false
	}

	// Filter by severity
	if filter.Severity != nil && alert.Severity != *filter.Severity {
		return false
	}

	// Filter by protocol
	if filter.Protocol != "" && alert.Protocol != filter.Protocol {
		return false
	}

	// Filter by start time
	if !filter.StartTime.IsZero() && alert.Timestamp.Before(filter.StartTime) {
		return false
	}

	// Filter by end time
	if !filter.EndTime.IsZero() && alert.Timestamp.After(filter.EndTime) {
		return false
	}

	return true
}

// storeAlert adds an alert to the history with limit enforcement.
// alertLimitMu is held for the append-and-trim sequence so a concurrent
// SetAlertLimit cannot race with this trim.
func (m *ProtocolMonitor) storeAlert(alert *Alert) {
	m.alertLimitMu.Lock()
	defer m.alertLimitMu.Unlock()

	m.alertHistory.Append(alert)

	// Enforce limit by removing oldest alerts if exceeded
	snapshot := m.alertHistory.Snapshot()
	if len(snapshot) > m.alertLimit {
		excess := len(snapshot) - m.alertLimit
		m.alertHistory.Replace(snapshot[excess:])
	}
}

// GetAlertCount returns the current number of stored alerts
func (m *ProtocolMonitor) GetAlertCount() int {
	return m.alertHistory.Len()
}

// SetAlertLimit sets the maximum number of alerts to retain
func (m *ProtocolMonitor) SetAlertLimit(limit int) {
	if limit < 1 {
		limit = 1
	}

	m.alertLimitMu.Lock()
	defer m.alertLimitMu.Unlock()

	m.alertLimit = limit

	// Trim history if current size exceeds new limit
	snapshot := m.alertHistory.Snapshot()
	if len(snapshot) > limit {
		excess := len(snapshot) - limit
		m.alertHistory.Replace(snapshot[excess:])
	}
}

// ClearAlerts removes all alerts from history
func (m *ProtocolMonitor) ClearAlerts() {
	m.alertHistory.Clear()
}

// ResolveAlert marks an alert as resolved
func (m *ProtocolMonitor) ResolveAlert(alertID string) bool {
	resolved := false
	m.alertHistory.UpdateAt(
		func(a *Alert) bool { return a.ID == alertID && !a.Resolved },
		func(a *Alert) *Alert {
			a.Resolved = true
			now := time.Now()
			a.ResolvedAt = &now
			resolved = true
			return a
		},
	)
	return resolved
}

// Alerts returns a channel for receiving alerts
func (m *ProtocolMonitor) Alerts() <-chan *Alert {
	return m.alertChan
}

// Stop stops the monitor
func (m *ProtocolMonitor) Stop() {
	close(m.stopChan)
}

// Private methods

func (m *ProtocolMonitor) metricsCollector() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectSystemMetrics()
		}
	}
}

func (m *ProtocolMonitor) collectSystemMetrics() {
	// Collect actual system-level metrics
	usage := collectRealSystemMetrics()

	for _, protocol := range m.metrics.Keys() {
		m.UpdateResourceUsage(protocol, usage)
	}
}

// collectRealSystemMetrics gathers actual system resource usage
func collectRealSystemMetrics() SystemResourceUsage {
	usage := SystemResourceUsage{}

	// Collect memory metrics using Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	usage.MemoryMB = float64(memStats.Alloc) / (1024 * 1024)

	// Collect CPU percentage
	usage.CPUPercent = collectCPUPercent()

	// Collect network bytes
	usage.NetworkBytes = collectNetworkBytes()

	// Collect disk usage
	usage.DiskUsageMB = collectDiskUsage()

	return usage
}

// collectCPUPercent reads CPU usage from /proc/stat on Linux
func collectCPUPercent() float64 {
	if runtime.GOOS != "linux" {
		// For non-Linux systems, return a simple estimate based on goroutines
		return float64(runtime.NumGoroutine()) * 0.1
	}

	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0.0
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0.0
			}

			user, _ := strconv.ParseUint(fields[1], 10, 64)   //nolint:errcheck
			nice, _ := strconv.ParseUint(fields[2], 10, 64)   //nolint:errcheck
			system, _ := strconv.ParseUint(fields[3], 10, 64) //nolint:errcheck
			idle, _ := strconv.ParseUint(fields[4], 10, 64)   //nolint:errcheck
			total := user + nice + system + idle

			currentStats := cpuStats{
				user:   user,
				nice:   nice,
				system: system,
				idle:   idle,
				total:  total,
			}

			if !metricsCollectorInstance.initialized {
				metricsCollectorInstance.prevCPUStats = currentStats
				metricsCollectorInstance.initialized = true
				return 0.0
			}

			// Calculate CPU percentage based on delta
			totalDelta := currentStats.total - metricsCollectorInstance.prevCPUStats.total
			idleDelta := currentStats.idle - metricsCollectorInstance.prevCPUStats.idle

			metricsCollectorInstance.prevCPUStats = currentStats

			if totalDelta == 0 {
				return 0.0
			}

			cpuPercent := 100.0 * float64(totalDelta-idleDelta) / float64(totalDelta)
			return cpuPercent
		}
	}

	return 0.0
}

// collectNetworkBytes reads network usage from /proc/net/dev on Linux
func collectNetworkBytes() int64 {
	if runtime.GOOS != "linux" {
		return 0
	}

	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0
	}
	defer func() { _ = file.Close() }()

	var totalBytes int64
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip header lines
		if lineNum <= 2 {
			continue
		}

		line := scanner.Text()
		// Format: "interface: rx_bytes rx_packets ... tx_bytes tx_packets ..."
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		// Skip loopback interface
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rxBytes, _ := strconv.ParseInt(fields[0], 10, 64) //nolint:errcheck
		txBytes, _ := strconv.ParseInt(fields[8], 10, 64) //nolint:errcheck
		totalBytes += rxBytes + txBytes
	}

	// Calculate delta from previous collection
	delta := totalBytes - metricsCollectorInstance.prevNetBytes
	metricsCollectorInstance.prevNetBytes = totalBytes

	// Return delta (bytes since last collection)
	if delta < 0 {
		return totalBytes // Counter wrapped or first collection
	}
	return delta
}

// collectDiskUsage gets disk usage for the root filesystem
// This is a platform-specific function implemented in protocol_monitor_unix.go and protocol_monitor_windows.go

func (m *ProtocolMonitor) alertChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAlerts()
		}
	}
}

func (m *ProtocolMonitor) checkAlerts() {
	alerts := m.alerts.Snapshot()

	for _, rule := range alerts {
		if !rule.Enabled {
			continue
		}

		// Check cooldown
		if time.Since(rule.LastAlert) < rule.Cooldown {
			continue
		}

		metrics, exists := m.metrics.Get(rule.Protocol)
		if !exists {
			continue
		}

		var currentValue float64
		var triggered bool

		metrics.mu.Lock()
		switch rule.Condition {
		case ConditionErrorRateAbove:
			currentValue = metrics.ErrorRate
			triggered = currentValue > rule.Threshold
		case ConditionLatencyAbove:
			currentValue = float64(metrics.AverageLatency.Nanoseconds()) / 1e6 // Convert to milliseconds
			triggered = currentValue > rule.Threshold
		case ConditionGreaterThan:
			currentValue = float64(metrics.TotalRequests)
			triggered = currentValue > rule.Threshold
		}
		metrics.mu.Unlock()

		if triggered {
			alert := &Alert{
				ID:        fmt.Sprintf("%s-%d", rule.ID, time.Now().Unix()),
				RuleID:    rule.ID,
				Protocol:  rule.Protocol,
				Message:   fmt.Sprintf("%s: %s (%.2f > %.2f)", rule.Name, rule.Description, currentValue, rule.Threshold),
				Severity:  rule.Severity,
				Value:     currentValue,
				Threshold: rule.Threshold,
				Timestamp: time.Now(),
			}

			// Store alert in history (always persisted)
			m.storeAlert(alert)

			// Also send to channel for real-time consumers
			select {
			case m.alertChan <- alert:
				rule.LastAlert = time.Now()
				m.logger.WithFields(logrus.Fields{
					"alertId":   alert.ID,
					"ruleId":    rule.ID,
					"protocol":  rule.Protocol,
					"value":     currentValue,
					"threshold": rule.Threshold,
				}).Warn("Alert triggered")
			default:
				// Channel full, but alert is still stored in history
				rule.LastAlert = time.Now()
				m.logger.Warn("Alert channel full, alert stored in history only")
			}
		}
	}
}

// Predefined alert rules

// NewErrorRateAlertRule creates an alert rule for high error rates
func NewErrorRateAlertRule(protocol string, threshold float64) *AlertRule {
	return &AlertRule{
		ID:          fmt.Sprintf("error-rate-%s", protocol),
		Name:        fmt.Sprintf("%s Error Rate Alert", protocol),
		Description: "Error rate exceeded threshold",
		Protocol:    protocol,
		Condition:   ConditionErrorRateAbove,
		Threshold:   threshold,
		Severity:    SeverityError,
		Cooldown:    5 * time.Minute,
		Enabled:     true,
	}
}

// NewLatencyAlertRule creates an alert rule for high latency
func NewLatencyAlertRule(protocol string, thresholdMs float64) *AlertRule {
	return &AlertRule{
		ID:          fmt.Sprintf("latency-%s", protocol),
		Name:        fmt.Sprintf("%s Latency Alert", protocol),
		Description: "Average latency exceeded threshold",
		Protocol:    protocol,
		Condition:   ConditionLatencyAbove,
		Threshold:   thresholdMs,
		Severity:    SeverityWarning,
		Cooldown:    2 * time.Minute,
		Enabled:     true,
	}
}

// NewHighTrafficAlertRule creates an alert rule for high traffic
func NewHighTrafficAlertRule(protocol string, threshold int64) *AlertRule {
	return &AlertRule{
		ID:          fmt.Sprintf("traffic-%s", protocol),
		Name:        fmt.Sprintf("%s High Traffic Alert", protocol),
		Description: "Request volume exceeded threshold",
		Protocol:    protocol,
		Condition:   ConditionGreaterThan,
		Threshold:   float64(threshold),
		Severity:    SeverityInfo,
		Cooldown:    10 * time.Minute,
		Enabled:     true,
	}
}
