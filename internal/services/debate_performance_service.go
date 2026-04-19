package services

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// PerformanceRecord holds a recorded performance entry
type PerformanceRecord struct {
	ID        string              `json:"id"`
	DebateID  string              `json:"debate_id"`
	Metrics   *PerformanceMetrics `json:"metrics"`
	CreatedAt time.Time           `json:"created_at"`
}

// PerformanceAggregation holds aggregated performance data
type PerformanceAggregation struct {
	TotalDebates      int           `json:"total_debates"`
	TotalDuration     time.Duration `json:"total_duration"`
	AverageDuration   time.Duration `json:"average_duration"`
	TotalRounds       int           `json:"total_rounds"`
	AverageRounds     float64       `json:"average_rounds"`
	AverageQuality    float64       `json:"average_quality"`
	AverageThroughput float64       `json:"average_throughput"`
	AverageLatency    time.Duration `json:"average_latency"`
	AverageErrorRate  float64       `json:"average_error_rate"`
	PeakCPU           float64       `json:"peak_cpu"`
	PeakMemory        uint64        `json:"peak_memory"`
	PerfTimeRange     TimeRange     `json:"time_range"`
}

// DebatePerformanceService provides performance metrics tracking and analysis.
// Concurrent-safe by construction (CONST-029 Tier 1): records is a safe.Store.
// PerformanceRecord values are immutable after creation (only read
// post-insert), so pointer-sharing does not risk field-mutation races.
type DebatePerformanceService struct {
	logger        *logrus.Logger
	records       *safe.Store[string, *PerformanceRecord]
	maxRecords    int
	systemMetrics *SystemMetricsCollector
}

// SystemMetricsCollector collects system metrics
type SystemMetricsCollector struct {
	lastCPUTime time.Time
}

// NewDebatePerformanceService creates a new performance service
func NewDebatePerformanceService(logger *logrus.Logger) *DebatePerformanceService {
	return &DebatePerformanceService{
		logger:     logger,
		records:    safe.NewStore[string, *PerformanceRecord](),
		maxRecords: 10000,
		systemMetrics: &SystemMetricsCollector{
			lastCPUTime: time.Now(),
		},
	}
}

// NewDebatePerformanceServiceWithMaxRecords creates a performance service with custom capacity
func NewDebatePerformanceServiceWithMaxRecords(logger *logrus.Logger, maxRecords int) *DebatePerformanceService {
	if maxRecords <= 0 {
		maxRecords = 10000
	}
	return &DebatePerformanceService{
		logger:     logger,
		records:    safe.NewStore[string, *PerformanceRecord](),
		maxRecords: maxRecords,
		systemMetrics: &SystemMetricsCollector{
			lastCPUTime: time.Now(),
		},
	}
}

// CalculateMetrics calculates performance metrics from a debate result
func (dps *DebatePerformanceService) CalculateMetrics(result *DebateResult) *PerformanceMetrics {
	if result == nil {
		return &PerformanceMetrics{}
	}

	// Calculate throughput (rounds per minute)
	throughput := float64(0)
	if result.Duration.Minutes() > 0 {
		throughput = float64(result.TotalRounds) / result.Duration.Minutes()
	}

	// Calculate average latency per round
	latency := time.Duration(0)
	if result.TotalRounds > 0 {
		latency = result.Duration / time.Duration(result.TotalRounds)
	}

	// Calculate error rate based on participant responses
	// Error rate is calculated from low-confidence responses as a proxy
	errorRate := float64(0)
	totalResponses := len(result.AllResponses)
	lowConfidenceResponses := 0
	for _, resp := range result.AllResponses {
		// Consider very low confidence responses as potential errors
		if resp.Confidence < 0.1 || resp.Response == "" {
			lowConfidenceResponses++
		}
	}
	if totalResponses > 0 {
		errorRate = float64(lowConfidenceResponses) / float64(totalResponses)
	}

	// Get current system resource usage
	resourceUsage := dps.collectResourceUsage()

	metrics := &PerformanceMetrics{
		Duration:      result.Duration,
		TotalRounds:   result.TotalRounds,
		QualityScore:  result.QualityScore,
		Throughput:    throughput,
		Latency:       latency,
		ErrorRate:     errorRate,
		ResourceUsage: resourceUsage,
	}

	return metrics
}

// CalculateMetricsWithID calculates performance metrics and returns with debate ID
func (dps *DebatePerformanceService) CalculateMetricsWithID(result *DebateResult) (*PerformanceMetrics, string) {
	metrics := dps.CalculateMetrics(result)
	debateID := ""
	if result != nil {
		debateID = result.DebateID
	}
	return metrics, debateID
}

// collectResourceUsage collects current system resource usage
func (dps *DebatePerformanceService) collectResourceUsage() ResourceUsage {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Estimate CPU usage (simplified)
	numCPU := runtime.NumCPU()
	numGoroutine := runtime.NumGoroutine()
	cpuEstimate := float64(numGoroutine) / float64(numCPU) * 100
	if cpuEstimate > 100 {
		cpuEstimate = 100
	}

	return ResourceUsage{
		CPU:     cpuEstimate,
		Memory:  memStats.Alloc,
		Network: 0, // Network tracking would require more infrastructure
	}
}

// RecordMetrics records performance metrics for a debate
func (dps *DebatePerformanceService) RecordMetrics(ctx context.Context, debateID string, metrics *PerformanceMetrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics cannot be nil")
	}

	recordID := fmt.Sprintf("perf-%s-%d", debateID, time.Now().UnixNano())

	if dps.records.Len() >= dps.maxRecords {
		dps.evictOldestRecord()
	}

	record := &PerformanceRecord{
		ID:        recordID,
		DebateID:  debateID,
		Metrics:   metrics,
		CreatedAt: time.Now(),
	}

	dps.records.Put(recordID, record)

	dps.logger.WithFields(logrus.Fields{
		"record_id":     recordID,
		"debate_id":     debateID,
		"duration":      metrics.Duration,
		"quality_score": metrics.QualityScore,
		"throughput":    metrics.Throughput,
	}).Info("Recorded performance metrics")

	return nil
}

// evictOldestRecord removes the oldest record
func (dps *DebatePerformanceService) evictOldestRecord() {
	var oldestID string
	var oldestTime time.Time

	for id, record := range dps.records.Snapshot() {
		if oldestID == "" || record.CreatedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = record.CreatedAt
		}
	}

	if oldestID != "" {
		dps.records.Delete(oldestID)
	}
}

// GetMetrics retrieves performance metrics for a time range
func (dps *DebatePerformanceService) GetMetrics(ctx context.Context, timeRange TimeRange) (*PerformanceMetrics, error) {
	aggregation, err := dps.GetAggregatedMetrics(ctx, timeRange)
	if err != nil {
		return nil, err
	}

	// Convert aggregation to single metrics
	return &PerformanceMetrics{
		Duration:     aggregation.AverageDuration,
		TotalRounds:  aggregation.TotalRounds,
		QualityScore: aggregation.AverageQuality,
		Throughput:   aggregation.AverageThroughput,
		Latency:      aggregation.AverageLatency,
		ErrorRate:    aggregation.AverageErrorRate,
		ResourceUsage: ResourceUsage{
			CPU:    aggregation.PeakCPU,
			Memory: aggregation.PeakMemory,
		},
	}, nil
}

// GetAggregatedMetrics retrieves aggregated metrics for a time range
func (dps *DebatePerformanceService) GetAggregatedMetrics(ctx context.Context, timeRange TimeRange) (*PerformanceAggregation, error) {
	snap := dps.records.Snapshot()

	aggregation := &PerformanceAggregation{
		PerfTimeRange: timeRange,
	}

	var totalDuration time.Duration
	var totalQuality float64
	var totalThroughput float64
	var totalLatency time.Duration
	var totalErrorRate float64
	var peakCPU float64
	var peakMemory uint64

	for _, record := range snap {
		// Filter by time range
		if !timeRange.StartTime.IsZero() && record.CreatedAt.Before(timeRange.StartTime) {
			continue
		}
		if !timeRange.EndTime.IsZero() && record.CreatedAt.After(timeRange.EndTime) {
			continue
		}

		aggregation.TotalDebates++
		totalDuration += record.Metrics.Duration
		aggregation.TotalRounds += record.Metrics.TotalRounds
		totalQuality += record.Metrics.QualityScore
		totalThroughput += record.Metrics.Throughput
		totalLatency += record.Metrics.Latency
		totalErrorRate += record.Metrics.ErrorRate

		if record.Metrics.ResourceUsage.CPU > peakCPU {
			peakCPU = record.Metrics.ResourceUsage.CPU
		}
		if record.Metrics.ResourceUsage.Memory > peakMemory {
			peakMemory = record.Metrics.ResourceUsage.Memory
		}
	}

	if aggregation.TotalDebates > 0 {
		aggregation.TotalDuration = totalDuration
		aggregation.AverageDuration = totalDuration / time.Duration(aggregation.TotalDebates)
		aggregation.AverageRounds = float64(aggregation.TotalRounds) / float64(aggregation.TotalDebates)
		aggregation.AverageQuality = totalQuality / float64(aggregation.TotalDebates)
		aggregation.AverageThroughput = totalThroughput / float64(aggregation.TotalDebates)
		aggregation.AverageLatency = totalLatency / time.Duration(aggregation.TotalDebates)
		aggregation.AverageErrorRate = totalErrorRate / float64(aggregation.TotalDebates)
	}

	aggregation.PeakCPU = peakCPU
	aggregation.PeakMemory = peakMemory

	return aggregation, nil
}

// GetMetricsByDebateID retrieves metrics for a specific debate
func (dps *DebatePerformanceService) GetMetricsByDebateID(ctx context.Context, debateID string) ([]*PerformanceMetrics, error) {
	metrics := make([]*PerformanceMetrics, 0)
	for _, record := range dps.records.Snapshot() {
		if record.DebateID == debateID {
			metricsCopy := *record.Metrics
			metrics = append(metrics, &metricsCopy)
		}
	}
	return metrics, nil
}

// GetCurrentResourceUsage returns current system resource usage
func (dps *DebatePerformanceService) GetCurrentResourceUsage() ResourceUsage {
	return dps.collectResourceUsage()
}

// CleanupOldRecords removes records older than maxAge
func (dps *DebatePerformanceService) CleanupOldRecords(ctx context.Context, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, record := range dps.records.Snapshot() {
		if record.CreatedAt.Before(cutoff) {
			if _, ok := dps.records.Delete(id); ok {
				removed++
			}
		}
	}

	if removed > 0 {
		dps.logger.Infof("Cleaned up %d old performance records", removed)
	}

	return removed, nil
}

// GetRecordCount returns the total number of records
func (dps *DebatePerformanceService) GetRecordCount() int {
	return dps.records.Len()
}

// GetStats returns performance service statistics
func (dps *DebatePerformanceService) GetStats() map[string]interface{} {
	currentUsage := dps.collectResourceUsage()
	recordCount := dps.records.Len()

	stats := map[string]interface{}{
		"total_records":  recordCount,
		"max_records":    dps.maxRecords,
		"current_cpu":    currentUsage.CPU,
		"current_memory": currentUsage.Memory,
	}

	// Calculate some aggregate stats
	var totalQuality float64
	var totalDuration time.Duration
	snap := dps.records.Snapshot()
	for _, record := range snap {
		totalQuality += record.Metrics.QualityScore
		totalDuration += record.Metrics.Duration
	}

	if n := len(snap); n > 0 {
		stats["average_quality"] = totalQuality / float64(n)
		stats["average_duration_ms"] = totalDuration.Milliseconds() / int64(n)
	}

	return stats
}
