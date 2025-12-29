package services

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolAnalyticsService provides comprehensive analytics for protocol usage
type ProtocolAnalyticsService struct {
	mu               sync.RWMutex
	metrics          map[string]*AnalyticsProtocolMetrics
	performanceData  map[string]*AnalyticsPerformanceStats
	usagePatterns    map[string]*AnalyticsUsagePattern
	logger           *logrus.Logger
	collectionWindow time.Duration
	retentionPeriod  time.Duration
}

// AnalyticsProtocolMetrics represents usage metrics for a protocol
type AnalyticsProtocolMetrics struct {
	Protocol           string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
	PeakLatency        time.Duration
	MinLatency         time.Duration
	ErrorRate          float64
	Throughput         float64 // requests per second
	LastUsed           time.Time
	ActiveConnections  int32
}

// AnalyticsPerformanceStats represents detailed performance statistics
type AnalyticsPerformanceStats struct {
	Protocol      string
	ResponseTimes []time.Duration
	ErrorCounts   map[string]int64
	SuccessCounts map[string]int64
	PeakUsageTime time.Time
	LowUsageTime  time.Time
	AverageLoad   float64
}

// AnalyticsUsagePattern represents usage patterns over time
type AnalyticsUsagePattern struct {
	Protocol       string
	HourlyUsage    [24]int64
	DailyUsage     [7]int64
	MonthlyUsage   [12]int64
	PeakHours      []int
	PeakDays       []time.Weekday
	TrendDirection string // "increasing", "decreasing", "stable"
}

// AnalyticsConfig represents configuration for the analytics service
type AnalyticsConfig struct {
	CollectionWindow  time.Duration
	RetentionPeriod   time.Duration
	MaxMetricsHistory int
	SamplingRate      float64
	EnableRealTime    bool
}

// NewProtocolAnalyticsService creates a new protocol analytics service
func NewProtocolAnalyticsService(config *AnalyticsConfig, logger *logrus.Logger) *ProtocolAnalyticsService {
	if config == nil {
		config = &AnalyticsConfig{
			CollectionWindow:  1 * time.Hour,
			RetentionPeriod:   30 * 24 * time.Hour,
			MaxMetricsHistory: 1000,
			SamplingRate:      1.0,
			EnableRealTime:    true,
		}
	}

	return &ProtocolAnalyticsService{
		metrics:          make(map[string]*AnalyticsProtocolMetrics),
		performanceData:  make(map[string]*AnalyticsPerformanceStats),
		usagePatterns:    make(map[string]*AnalyticsUsagePattern),
		logger:           logger,
		collectionWindow: config.CollectionWindow,
		retentionPeriod:  config.RetentionPeriod,
	}
}

// RecordRequest records a protocol request for analytics
func (a *ProtocolAnalyticsService) RecordRequest(ctx context.Context, protocol string, method string, duration time.Duration, success bool, errorType string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Initialize metrics if not exists
	if _, exists := a.metrics[protocol]; !exists {
		a.metrics[protocol] = &AnalyticsProtocolMetrics{
			Protocol:   protocol,
			MinLatency: time.Hour, // Initialize to high value
			LastUsed:   time.Now(),
		}
		a.performanceData[protocol] = &AnalyticsPerformanceStats{
			Protocol:      protocol,
			ResponseTimes: make([]time.Duration, 0, 1000),
			ErrorCounts:   make(map[string]int64),
			SuccessCounts: make(map[string]int64),
		}
		a.usagePatterns[protocol] = &AnalyticsUsagePattern{
			Protocol: protocol,
		}
	}

	metrics := a.metrics[protocol]
	perfStats := a.performanceData[protocol]
	usagePattern := a.usagePatterns[protocol]

	// Update basic metrics
	metrics.TotalRequests++
	metrics.LastUsed = time.Now()

	if success {
		metrics.SuccessfulRequests++
		perfStats.SuccessCounts[method]++
	} else {
		metrics.FailedRequests++
		perfStats.ErrorCounts[errorType]++
	}

	// Update latency statistics
	if duration > 0 {
		metrics.AverageLatency = time.Duration((int64(metrics.AverageLatency)*int64(metrics.TotalRequests-1) + int64(duration)) / int64(metrics.TotalRequests))

		if duration > metrics.PeakLatency {
			metrics.PeakLatency = duration
		}

		if duration < metrics.MinLatency {
			metrics.MinLatency = duration
		}

		// Keep response times for detailed analysis (limit to last 1000)
		perfStats.ResponseTimes = append(perfStats.ResponseTimes, duration)
		if len(perfStats.ResponseTimes) > 1000 {
			perfStats.ResponseTimes = perfStats.ResponseTimes[1:]
		}
	}

	// Update error rate
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	}

	// Update usage patterns
	now := time.Now()
	hour := now.Hour()
	day := int(now.Weekday())
	month := int(now.Month()) - 1

	usagePattern.HourlyUsage[hour]++
	usagePattern.DailyUsage[day]++
	usagePattern.MonthlyUsage[month]++

	// Update throughput (requests per second over last minute)
	// This is a simplified calculation - in production you'd want more sophisticated tracking
	metrics.Throughput = float64(metrics.TotalRequests) / time.Since(metrics.LastUsed).Seconds()

	a.logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"method":   method,
		"duration": duration,
		"success":  success,
	}).Debug("Recorded protocol request")

	return nil
}

// GetProtocolMetrics returns metrics for a specific protocol
func (a *ProtocolAnalyticsService) GetProtocolMetrics(protocol string) (*AnalyticsProtocolMetrics, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics, exists := a.metrics[protocol]
	if !exists {
		return nil, fmt.Errorf("no metrics found for protocol: %s", protocol)
	}

	// Return a copy to avoid race conditions
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetAllProtocolMetrics returns metrics for all protocols
func (a *ProtocolAnalyticsService) GetAllProtocolMetrics() map[string]*AnalyticsProtocolMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]*AnalyticsProtocolMetrics)
	for protocol, metrics := range a.metrics {
		metricsCopy := *metrics
		result[protocol] = &metricsCopy
	}

	return result
}

// GetPerformanceStats returns detailed performance statistics
func (a *ProtocolAnalyticsService) GetPerformanceStats(protocol string) (*AnalyticsPerformanceStats, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats, exists := a.performanceData[protocol]
	if !exists {
		return nil, fmt.Errorf("no performance stats found for protocol: %s", protocol)
	}

	// Return a copy
	statsCopy := *stats
	statsCopy.ResponseTimes = make([]time.Duration, len(stats.ResponseTimes))
	copy(statsCopy.ResponseTimes, stats.ResponseTimes)

	statsCopy.ErrorCounts = make(map[string]int64)
	for k, v := range stats.ErrorCounts {
		statsCopy.ErrorCounts[k] = v
	}

	statsCopy.SuccessCounts = make(map[string]int64)
	for k, v := range stats.SuccessCounts {
		statsCopy.SuccessCounts[k] = v
	}

	return &statsCopy, nil
}

// GetUsagePatterns returns usage patterns for a protocol
func (a *ProtocolAnalyticsService) GetUsagePatterns(protocol string) (*AnalyticsUsagePattern, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	pattern, exists := a.usagePatterns[protocol]
	if !exists {
		return nil, fmt.Errorf("no usage patterns found for protocol: %s", protocol)
	}

	// Return a copy
	patternCopy := *pattern
	patternCopy.PeakHours = make([]int, len(pattern.PeakHours))
	copy(patternCopy.PeakHours, pattern.PeakHours)

	patternCopy.PeakDays = make([]time.Weekday, len(pattern.PeakDays))
	copy(patternCopy.PeakDays, pattern.PeakDays)

	return &patternCopy, nil
}

// AnalyzePerformance provides performance analysis and recommendations
func (a *ProtocolAnalyticsService) AnalyzePerformance(protocol string) (*PerformanceAnalysis, error) {
	metrics, err := a.GetProtocolMetrics(protocol)
	if err != nil {
		return nil, err
	}

	perfStats, err := a.GetPerformanceStats(protocol)
	if err != nil {
		return nil, err
	}

	analysis := &PerformanceAnalysis{
		Protocol:          protocol,
		OverallHealth:     "healthy",
		Bottlenecks:       []string{},
		Recommendations:   []string{},
		OptimizationScore: 100,
	}

	// Analyze error rate
	if metrics.ErrorRate > 0.1 {
		analysis.Bottlenecks = append(analysis.Bottlenecks, "High error rate")
		analysis.Recommendations = append(analysis.Recommendations, "Investigate error patterns and improve error handling")
		analysis.OptimizationScore -= 20
		analysis.OverallHealth = "degraded"
	}

	// Analyze latency
	if metrics.AverageLatency > 5*time.Second {
		analysis.Bottlenecks = append(analysis.Bottlenecks, "High average latency")
		analysis.Recommendations = append(analysis.Recommendations, "Consider implementing caching or optimizing request processing")
		analysis.OptimizationScore -= 15
	}

	if metrics.PeakLatency > 30*time.Second {
		analysis.Bottlenecks = append(analysis.Bottlenecks, "Extreme latency spikes")
		analysis.Recommendations = append(analysis.Recommendations, "Implement circuit breakers and request timeouts")
		analysis.OptimizationScore -= 25
		analysis.OverallHealth = "critical"
	}

	// Analyze throughput
	if metrics.Throughput < 10 {
		analysis.Recommendations = append(analysis.Recommendations, "Consider implementing connection pooling and request batching")
		analysis.OptimizationScore -= 10
	}

	// Check for error patterns
	if len(perfStats.ErrorCounts) > 0 {
		analysis.Recommendations = append(analysis.Recommendations, "Review error distribution and implement targeted fixes")
	}

	// Ensure score doesn't go below 0
	if analysis.OptimizationScore < 0 {
		analysis.OptimizationScore = 0
	}

	return analysis, nil
}

// PerformanceAnalysis represents the result of performance analysis
type PerformanceAnalysis struct {
	Protocol          string
	OverallHealth     string
	Bottlenecks       []string
	Recommendations   []string
	OptimizationScore int // 0-100
}

// GetTopProtocols returns the most used protocols
func (a *ProtocolAnalyticsService) GetTopProtocols(limit int) []*AnalyticsProtocolMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var protocols []*AnalyticsProtocolMetrics
	for _, metrics := range a.metrics {
		metricsCopy := *metrics
		protocols = append(protocols, &metricsCopy)
	}

	// Sort by total requests (descending)
	sort.Slice(protocols, func(i, j int) bool {
		return protocols[i].TotalRequests > protocols[j].TotalRequests
	})

	if len(protocols) > limit {
		protocols = protocols[:limit]
	}

	return protocols
}

// GenerateUsageReport generates a comprehensive usage report
func (a *ProtocolAnalyticsService) GenerateUsageReport() *UsageReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &UsageReport{
		GeneratedAt:     time.Now(),
		TotalProtocols:  len(a.metrics),
		ProtocolMetrics: make(map[string]*AnalyticsProtocolMetrics),
		TopProtocols:    make([]*AnalyticsProtocolMetrics, 0),
		Summary:         &UsageSummary{},
	}

	totalRequests := int64(0)
	totalErrors := int64(0)
	totalLatency := time.Duration(0)

	for protocol, metrics := range a.metrics {
		metricsCopy := *metrics
		report.ProtocolMetrics[protocol] = &metricsCopy

		totalRequests += metrics.TotalRequests
		totalErrors += metrics.FailedRequests
		totalLatency += metrics.AverageLatency
	}

	report.Summary.TotalRequests = totalRequests
	report.Summary.TotalErrors = totalErrors
	report.Summary.AverageLatency = totalLatency / time.Duration(len(a.metrics))

	// Get top 5 protocols
	report.TopProtocols = a.GetTopProtocols(5)

	return report
}

// UsageReport represents a comprehensive usage report
type UsageReport struct {
	GeneratedAt     time.Time
	TotalProtocols  int
	ProtocolMetrics map[string]*AnalyticsProtocolMetrics
	TopProtocols    []*AnalyticsProtocolMetrics
	Summary         *UsageSummary
}

// UsageSummary represents summary statistics
type UsageSummary struct {
	TotalRequests  int64
	TotalErrors    int64
	AverageLatency time.Duration
	ErrorRate      float64
}

// UpdateConnectionCount updates the active connection count for a protocol
func (a *ProtocolAnalyticsService) UpdateConnectionCount(protocol string, count int32) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if metrics, exists := a.metrics[protocol]; exists {
		metrics.ActiveConnections = count
	}
}

// CleanOldData removes old analytics data beyond retention period
func (a *ProtocolAnalyticsService) CleanOldData() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for protocol, metrics := range a.metrics {
		if time.Since(metrics.LastUsed) > a.retentionPeriod {
			delete(a.metrics, protocol)
			delete(a.performanceData, protocol)
			delete(a.usagePatterns, protocol)

			a.logger.WithField("protocol", protocol).Info("Cleaned old analytics data")
		}
	}

	return nil
}

// GetHealthStatus returns overall health status of protocol ecosystem
func (a *ProtocolAnalyticsService) GetHealthStatus() *AnalyticsHealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := &AnalyticsHealthStatus{
		OverallStatus:    "healthy",
		ProtocolStatuses: make(map[string]string),
		Alerts:           []string{},
		LastUpdated:      time.Now(),
	}

	totalErrorRate := 0.0
	protocolCount := 0

	for protocol, metrics := range a.metrics {
		protocolCount++

		// Determine protocol health
		protocolStatus := "healthy"
		if metrics.ErrorRate > 0.1 {
			protocolStatus = "degraded"
			status.Alerts = append(status.Alerts, fmt.Sprintf("%s has high error rate: %.2f%%", protocol, metrics.ErrorRate*100))
		}
		if metrics.AverageLatency > 10*time.Second {
			protocolStatus = "critical"
			status.Alerts = append(status.Alerts, fmt.Sprintf("%s has high latency: %v", protocol, metrics.AverageLatency))
		}

		status.ProtocolStatuses[protocol] = protocolStatus
		totalErrorRate += metrics.ErrorRate
	}

	// Determine overall status
	if protocolCount > 0 {
		avgErrorRate := totalErrorRate / float64(protocolCount)
		if avgErrorRate > 0.05 {
			status.OverallStatus = "degraded"
		}
		if avgErrorRate > 0.15 {
			status.OverallStatus = "critical"
		}
	}

	return status
}

// AnalyticsHealthStatus represents the health status of the protocol ecosystem
type AnalyticsHealthStatus struct {
	OverallStatus    string
	ProtocolStatuses map[string]string
	Alerts           []string
	LastUpdated      time.Time
}
