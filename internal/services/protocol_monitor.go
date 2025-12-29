package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolMonitor provides performance monitoring and alerting for protocols
type ProtocolMonitor struct {
	mu        sync.RWMutex
	metrics   map[string]*ProtocolMetrics
	alerts    []*AlertRule
	alertChan chan *Alert
	stopChan  chan struct{}
	logger    *logrus.Logger
}

// ProtocolMetrics represents performance metrics for a protocol
type ProtocolMetrics struct {
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

// NewProtocolMonitor creates a new protocol monitor
func NewProtocolMonitor(logger *logrus.Logger) *ProtocolMonitor {
	monitor := &ProtocolMonitor{
		metrics:   make(map[string]*ProtocolMetrics),
		alerts:    []*AlertRule{},
		alertChan: make(chan *Alert, 100),
		stopChan:  make(chan struct{}),
		logger:    logger,
	}

	// Start monitoring goroutines
	go monitor.metricsCollector()
	go monitor.alertChecker()

	return monitor
}

// RecordRequest records a protocol request
func (m *ProtocolMonitor) RecordRequest(ctx context.Context, protocol string, duration time.Duration, success bool, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[protocol]
	if !exists {
		metrics = &ProtocolMetrics{
			Protocol:   protocol,
			MinLatency: time.Hour, // Initialize to a large value
		}
		m.metrics[protocol] = metrics
	}

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

// UpdateConnections updates connection count for a protocol
func (m *ProtocolMonitor) UpdateConnections(protocol string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[protocol]
	if !exists {
		metrics = &ProtocolMetrics{Protocol: protocol}
		m.metrics[protocol] = metrics
	}

	metrics.ActiveConnections = count
}

// UpdateCacheStats updates cache statistics
func (m *ProtocolMonitor) UpdateCacheStats(protocol string, hitRate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[protocol]
	if !exists {
		metrics = &ProtocolMetrics{Protocol: protocol}
		m.metrics[protocol] = metrics
	}

	metrics.CacheHitRate = hitRate
}

// UpdateResourceUsage updates resource usage statistics
func (m *ProtocolMonitor) UpdateResourceUsage(protocol string, usage SystemResourceUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[protocol]
	if !exists {
		metrics = &ProtocolMetrics{Protocol: protocol}
		m.metrics[protocol] = metrics
	}

	metrics.ResourceUsage = usage
}

// GetMetrics returns metrics for a protocol
func (m *ProtocolMonitor) GetMetrics(protocol string) (*ProtocolMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[protocol]
	if !exists {
		return nil, fmt.Errorf("no metrics found for protocol: %s", protocol)
	}

	// Return a copy to avoid race conditions
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
	}, nil
}

// GetAllMetrics returns metrics for all protocols
func (m *ProtocolMonitor) GetAllMetrics() map[string]*ProtocolMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ProtocolMetrics)
	for protocol, metrics := range m.metrics {
		result[protocol] = &ProtocolMetrics{
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

	return result
}

// AddAlertRule adds an alert rule
func (m *ProtocolMonitor) AddAlertRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts = append(m.alerts, rule)
	m.logger.WithFields(logrus.Fields{
		"ruleId":   rule.ID,
		"name":     rule.Name,
		"protocol": rule.Protocol,
	}).Info("Alert rule added")
}

// RemoveAlertRule removes an alert rule
func (m *ProtocolMonitor) RemoveAlertRule(ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, rule := range m.alerts {
		if rule.ID == ruleID {
			m.alerts = append(m.alerts[:i], m.alerts[i+1:]...)
			m.logger.WithField("ruleId", ruleID).Info("Alert rule removed")
			return
		}
	}
}

// GetAlerts returns recent alerts
func (m *ProtocolMonitor) GetAlerts(limit int) []*Alert {
	// For simplicity, return alerts from the channel
	// In a real implementation, you'd store alerts in a database
	alerts := make([]*Alert, 0, limit)

	// Non-blocking read from channel
	for i := 0; i < limit; i++ {
		select {
		case alert := <-m.alertChan:
			alerts = append(alerts, alert)
		default:
			break
		}
	}

	return alerts
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
	// Collect system-level metrics
	// This is a placeholder - in a real implementation, you'd collect
	// actual system metrics like memory usage, CPU, etc.

	for protocol := range m.metrics {
		usage := SystemResourceUsage{
			MemoryMB:   100.0, // Placeholder
			CPUPercent: 5.0,   // Placeholder
		}
		m.UpdateResourceUsage(protocol, usage)
	}
}

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
	m.mu.RLock()
	alerts := make([]*AlertRule, len(m.alerts))
	copy(alerts, m.alerts)
	m.mu.RUnlock()

	for _, rule := range alerts {
		if !rule.Enabled {
			continue
		}

		// Check cooldown
		if time.Since(rule.LastAlert) < rule.Cooldown {
			continue
		}

		metrics, exists := m.metrics[rule.Protocol]
		if !exists {
			continue
		}

		var currentValue float64
		var triggered bool

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
				m.logger.Warn("Alert channel full, dropping alert")
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
