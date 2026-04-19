package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// DebateHistorySummary provides a summary of historical debate data
type DebateHistorySummary struct {
	TotalDebates      int           `json:"total_debates"`
	SuccessfulDebates int           `json:"successful_debates"`
	FailedDebates     int           `json:"failed_debates"`
	AverageQuality    float64       `json:"average_quality"`
	AverageDuration   time.Duration `json:"average_duration"`
	TotalRounds       int           `json:"total_rounds"`
	CogneeEnhanced    int           `json:"cognee_enhanced"`
	MemoryUsed        int           `json:"memory_used"`
	HistoryTimeRange  TimeRange     `json:"time_range"`
}

// DebateHistoryService provides historical debate data with persistent storage.
//
// Concurrent-safe by construction (CONST-029 Tier 1): history is a
// safe.Store. Eviction under concurrent Save may briefly overshoot
// maxEntries (each saver evicts one, net same count) — acceptable LRU
// approximation.
type DebateHistoryService struct {
	logger     *logrus.Logger
	history    *safe.Store[string, *DebateResult]
	maxEntries int
}

// NewDebateHistoryService creates a new history service
func NewDebateHistoryService(logger *logrus.Logger) *DebateHistoryService {
	return &DebateHistoryService{
		logger:     logger,
		history:    safe.NewStore[string, *DebateResult](),
		maxEntries: 10000,
	}
}

// NewDebateHistoryServiceWithMaxEntries creates a history service with custom max entries
func NewDebateHistoryServiceWithMaxEntries(logger *logrus.Logger, maxEntries int) *DebateHistoryService {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &DebateHistoryService{
		logger:     logger,
		history:    safe.NewStore[string, *DebateResult](),
		maxEntries: maxEntries,
	}
}

// SaveDebateResult saves a debate result to history
func (dhs *DebateHistoryService) SaveDebateResult(ctx context.Context, result *DebateResult) error {
	if result == nil {
		return fmt.Errorf("cannot save nil debate result")
	}

	if result.DebateID == "" {
		return fmt.Errorf("debate result must have a debate ID")
	}

	if dhs.history.Len() >= dhs.maxEntries {
		dhs.evictOldestEntry()
	}

	// Make a copy to prevent external modification
	resultCopy := *result
	dhs.history.Put(result.DebateID, &resultCopy)

	dhs.logger.WithFields(logrus.Fields{
		"debate_id":     result.DebateID,
		"quality_score": result.QualityScore,
		"success":       result.Success,
		"duration":      result.Duration,
	}).Info("Saved debate result to history")

	return nil
}

// evictOldestEntry removes the oldest entry from history
func (dhs *DebateHistoryService) evictOldestEntry() {
	var oldestID string
	var oldestTime time.Time

	for id, result := range dhs.history.Snapshot() {
		if oldestID == "" || result.StartTime.Before(oldestTime) {
			oldestID = id
			oldestTime = result.StartTime
		}
	}

	if oldestID != "" {
		dhs.history.Delete(oldestID)
		dhs.logger.Debugf("Evicted oldest debate from history: %s", oldestID)
	}
}

// QueryHistory queries historical debate data with filters
func (dhs *DebateHistoryService) QueryHistory(ctx context.Context, filters *HistoryFilters) ([]*DebateResult, error) {
	if filters == nil {
		filters = &HistoryFilters{}
	}

	results := make([]*DebateResult, 0)

	for _, result := range dhs.history.Snapshot() {
		if dhs.matchesFilters(result, filters) {
			// Make a copy
			resultCopy := *result
			results = append(results, &resultCopy)
		}
	}

	// Sort results
	// Sort results by start time (default)
	dhs.sortResults(results, "start_time", "desc")

	// Apply offset and limit
	if filters.Offset > 0 {
		if filters.Offset >= len(results) {
			return []*DebateResult{}, nil
		}
		results = results[filters.Offset:]
	}

	if filters.Limit > 0 && filters.Limit < len(results) {
		results = results[:filters.Limit]
	}

	dhs.logger.WithFields(logrus.Fields{
		"results_count": len(results),
		"filters":       filters,
	}).Debug("Queried debate history")

	return results, nil
}

// matchesFilters checks if a result matches the given filters
func (dhs *DebateHistoryService) matchesFilters(result *DebateResult, filters *HistoryFilters) bool {
	// Filter by time range
	if filters.StartTime != nil && result.StartTime.Before(*filters.StartTime) {
		return false
	}
	if filters.EndTime != nil && result.EndTime.After(*filters.EndTime) {
		return false
	}

	// Filter by minimum quality
	if filters.MinQualityScore != nil && result.QualityScore < *filters.MinQualityScore {
		return false
	}

	// Filter by maximum quality
	if filters.MaxQualityScore != nil && result.QualityScore > *filters.MaxQualityScore {
		return false
	}

	// Filter by participant IDs
	if len(filters.ParticipantIDs) > 0 {
		hasMatch := false
		for _, responder := range result.Participants {
			for _, filterID := range filters.ParticipantIDs {
				if responder.ParticipantID == filterID {
					hasMatch = true
					break
				}
			}
			if hasMatch {
				break
			}
		}
		if !hasMatch {
			return false
		}
	}

	return true
}

// sortResults sorts the results by the specified field
func (dhs *DebateHistoryService) sortResults(results []*DebateResult, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "start_time"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "quality_score":
			less = results[i].QualityScore < results[j].QualityScore
		case "duration":
			less = results[i].Duration < results[j].Duration
		case "start_time":
			fallthrough
		default:
			less = results[i].StartTime.Before(results[j].StartTime)
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// GetDebateByID retrieves a specific debate by ID
func (dhs *DebateHistoryService) GetDebateByID(ctx context.Context, debateID string) (*DebateResult, error) {
	result, exists := dhs.history.Get(debateID)
	if !exists {
		return nil, fmt.Errorf("debate not found: %s", debateID)
	}

	// Return a copy
	resultCopy := *result
	return &resultCopy, nil
}

// DeleteDebate removes a debate from history
func (dhs *DebateHistoryService) DeleteDebate(ctx context.Context, debateID string) error {
	if _, ok := dhs.history.Delete(debateID); !ok {
		return fmt.Errorf("debate not found: %s", debateID)
	}
	dhs.logger.Infof("Deleted debate from history: %s", debateID)
	return nil
}

// GetSummary returns a summary of historical debate data
func (dhs *DebateHistoryService) GetSummary(ctx context.Context, filters *HistoryFilters) (*DebateHistorySummary, error) {
	results, err := dhs.QueryHistory(ctx, filters)
	if err != nil {
		return nil, err
	}

	summary := &DebateHistorySummary{
		TotalDebates: len(results),
	}

	if len(results) == 0 {
		return summary, nil
	}

	var totalQuality float64
	var totalDuration time.Duration
	var minTime, maxTime time.Time

	for i, result := range results {
		if result.Success {
			summary.SuccessfulDebates++
		} else {
			summary.FailedDebates++
		}

		totalQuality += result.QualityScore
		totalDuration += result.Duration
		summary.TotalRounds += result.TotalRounds

		if result.CogneeEnhanced {
			summary.CogneeEnhanced++
		}
		if result.MemoryUsed {
			summary.MemoryUsed++
		}

		if i == 0 || result.StartTime.Before(minTime) {
			minTime = result.StartTime
		}
		if i == 0 || result.EndTime.After(maxTime) {
			maxTime = result.EndTime
		}
	}

	summary.AverageQuality = totalQuality / float64(len(results))
	summary.AverageDuration = totalDuration / time.Duration(len(results))
	summary.HistoryTimeRange = TimeRange{
		StartTime: minTime,
		EndTime:   maxTime,
	}

	return summary, nil
}

// CleanupOldEntries removes entries older than the given duration
func (dhs *DebateHistoryService) CleanupOldEntries(ctx context.Context, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, result := range dhs.history.Snapshot() {
		if result.EndTime.Before(cutoff) {
			if _, ok := dhs.history.Delete(id); ok {
				removed++
			}
		}
	}

	if removed > 0 {
		dhs.logger.Infof("Cleaned up %d old entries from debate history", removed)
	}

	return removed, nil
}

// GetCount returns the total number of entries in history
func (dhs *DebateHistoryService) GetCount() int {
	return dhs.history.Len()
}
