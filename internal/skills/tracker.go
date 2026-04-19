package skills

import (
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// Tracker tracks skill usage across requests and sessions.
//
// Concurrent-safe by construction (CONST-029):
//   - All mutable tracking state (activeUsages, history, stats) is held
//     in a single trackerState struct stored under a sentinel key in a
//     safe.Store. Every read-modify-write therefore runs under one Update
//     callback — multi-collection invariants are atomic by construction.
//     This is Pattern Epsilon (joint atomicity via state struct).
type Tracker struct {
	state        *safe.Store[string, *trackerState]
	historyLimit int
	log          *logrus.Logger
}

const trackerStateKey = "_"

// trackerState is the single jointly-managed state. All fields are mutated
// only inside trackerState.Update callbacks. Readers obtain a *trackerState
// pointer via Get and must not mutate it; mutators always replace the value
// with a new *trackerState (semantically — though we mutate in place under
// the Store's write lock for performance, which is safe because no reader
// can be inside the callback).
type trackerState struct {
	activeUsages map[string]*SkillUsage // requestID -> usage
	history      []SkillUsage           // historical usage records
	stats        *UsageStats
}

// UsageStats provides aggregate usage statistics.
type UsageStats struct {
	TotalInvocations  int64                     `json:"total_invocations"`
	SuccessfulCount   int64                     `json:"successful_count"`
	FailedCount       int64                     `json:"failed_count"`
	BySkill           map[string]*SkillStats    `json:"by_skill"`
	ByCategory        map[string]*CategoryStats `json:"by_category"`
	ByMatchType       map[MatchType]int64       `json:"by_match_type"`
	AverageConfidence float64                   `json:"average_confidence"`
	AverageDuration   time.Duration             `json:"average_duration"`
	LastUpdated       time.Time                 `json:"last_updated"`
}

// SkillStats provides per-skill statistics.
type SkillStats struct {
	Name              string           `json:"name"`
	Category          string           `json:"category"`
	InvocationCount   int64            `json:"invocation_count"`
	SuccessCount      int64            `json:"success_count"`
	FailureCount      int64            `json:"failure_count"`
	AverageConfidence float64          `json:"average_confidence"`
	AverageDuration   time.Duration    `json:"average_duration"`
	TotalDuration     time.Duration    `json:"total_duration"`
	LastUsed          time.Time        `json:"last_used"`
	TriggersCounted   map[string]int64 `json:"triggers_counted"`
}

// CategoryStats provides per-category statistics.
type CategoryStats struct {
	Category        string `json:"category"`
	InvocationCount int64  `json:"invocation_count"`
	SuccessCount    int64  `json:"success_count"`
	UniqueSkills    int    `json:"unique_skills"`
}

func newTrackerState() *trackerState {
	return &trackerState{
		activeUsages: make(map[string]*SkillUsage),
		history:      make([]SkillUsage, 0, 1000),
		stats: &UsageStats{
			BySkill:     make(map[string]*SkillStats),
			ByCategory:  make(map[string]*CategoryStats),
			ByMatchType: make(map[MatchType]int64),
			LastUpdated: time.Now(),
		},
	}
}

// NewTracker creates a new skill usage tracker.
func NewTracker() *Tracker {
	store := safe.NewStore[string, *trackerState]()
	store.Put(trackerStateKey, newTrackerState())
	return &Tracker{
		state:        store,
		historyLimit: 10000,
		log:          logrus.New(),
	}
}

// SetLogger sets the logger for the tracker.
func (t *Tracker) SetLogger(log *logrus.Logger) {
	t.log = log
}

// withState runs fn under the state Store's write lock; fn may safely
// mutate *trackerState.
func (t *Tracker) withState(fn func(*trackerState)) {
	t.state.Update(trackerStateKey, func(s *trackerState, _ bool) (*trackerState, bool) {
		if s == nil {
			s = newTrackerState()
		}
		fn(s)
		return s, true
	})
}

// StartTracking begins tracking usage for a skill.
func (t *Tracker) StartTracking(requestID string, skill *Skill, match *SkillMatch) *SkillUsage {
	usage := &SkillUsage{
		SkillName:    skill.Name,
		Category:     skill.Category,
		TriggerUsed:  match.MatchedTrigger,
		MatchType:    match.MatchType,
		Confidence:   match.Confidence,
		ToolsInvoked: make([]string, 0),
		StartedAt:    time.Now(),
	}

	t.withState(func(s *trackerState) {
		s.activeUsages[requestID] = usage
	})

	t.log.WithFields(logrus.Fields{
		"request_id": requestID,
		"skill":      skill.Name,
		"trigger":    match.MatchedTrigger,
		"confidence": match.Confidence,
	}).Debug("Started tracking skill usage")

	return usage
}

// RecordToolUse records a tool being used by a skill.
func (t *Tracker) RecordToolUse(requestID, toolName string) {
	t.withState(func(s *trackerState) {
		if usage, ok := s.activeUsages[requestID]; ok {
			usage.ToolsInvoked = append(usage.ToolsInvoked, toolName)
		}
	})
}

// CompleteTracking marks skill usage as complete.
func (t *Tracker) CompleteTracking(requestID string, success bool, err string) *SkillUsage {
	var result *SkillUsage
	t.withState(func(s *trackerState) {
		usage, ok := s.activeUsages[requestID]
		if !ok {
			return
		}

		usage.CompletedAt = time.Now()
		usage.Success = success
		usage.Error = err

		updateStatsLocked(s.stats, usage)
		s.history = appendHistoryLocked(s.history, *usage, t.historyLimit)
		delete(s.activeUsages, requestID)

		result = usage
	})

	if result != nil {
		t.log.WithFields(logrus.Fields{
			"request_id": requestID,
			"skill":      result.SkillName,
			"success":    success,
			"duration":   result.CompletedAt.Sub(result.StartedAt),
		}).Debug("Completed tracking skill usage")
	}
	return result
}

// GetActiveUsage returns the active usage for a request.
func (t *Tracker) GetActiveUsage(requestID string) *SkillUsage {
	var result *SkillUsage
	t.withState(func(s *trackerState) {
		result = s.activeUsages[requestID]
	})
	return result
}

// GetActiveUsages returns all currently active usages.
func (t *Tracker) GetActiveUsages() []*SkillUsage {
	var usages []*SkillUsage
	t.withState(func(s *trackerState) {
		usages = make([]*SkillUsage, 0, len(s.activeUsages))
		for _, usage := range s.activeUsages {
			usages = append(usages, usage)
		}
	})
	return usages
}

// updateStatsLocked updates aggregate statistics. Caller must hold the
// state's write lock (i.e. be inside a withState callback).
func updateStatsLocked(stats *UsageStats, usage *SkillUsage) {
	stats.TotalInvocations++

	if usage.Success {
		stats.SuccessfulCount++
	} else {
		stats.FailedCount++
	}

	stats.ByMatchType[usage.MatchType]++

	skillStats, ok := stats.BySkill[usage.SkillName]
	if !ok {
		skillStats = &SkillStats{
			Name:            usage.SkillName,
			Category:        usage.Category,
			TriggersCounted: make(map[string]int64),
		}
		stats.BySkill[usage.SkillName] = skillStats
	}

	skillStats.InvocationCount++
	if usage.Success {
		skillStats.SuccessCount++
	} else {
		skillStats.FailureCount++
	}

	duration := usage.CompletedAt.Sub(usage.StartedAt)
	skillStats.TotalDuration += duration
	skillStats.AverageDuration = skillStats.TotalDuration / time.Duration(skillStats.InvocationCount)
	skillStats.LastUsed = usage.CompletedAt
	skillStats.TriggersCounted[usage.TriggerUsed]++

	oldConfidence := skillStats.AverageConfidence
	skillStats.AverageConfidence = ((oldConfidence * float64(skillStats.InvocationCount-1)) + usage.Confidence) / float64(skillStats.InvocationCount)

	catStats, ok := stats.ByCategory[usage.Category]
	if !ok {
		catStats = &CategoryStats{
			Category: usage.Category,
		}
		stats.ByCategory[usage.Category] = catStats
	}

	catStats.InvocationCount++
	if usage.Success {
		catStats.SuccessCount++
	}

	totalConfidence := float64(0)
	for _, ss := range stats.BySkill {
		totalConfidence += ss.AverageConfidence * float64(ss.InvocationCount)
	}
	stats.AverageConfidence = totalConfidence / float64(stats.TotalInvocations)

	totalDuration := time.Duration(0)
	for _, ss := range stats.BySkill {
		totalDuration += ss.TotalDuration
	}
	stats.AverageDuration = totalDuration / time.Duration(stats.TotalInvocations)

	stats.LastUpdated = time.Now()
}

// appendHistoryLocked appends and trims to historyLimit.
func appendHistoryLocked(history []SkillUsage, usage SkillUsage, limit int) []SkillUsage {
	history = append(history, usage)
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	return history
}

// GetHistory returns usage history.
func (t *Tracker) GetHistory(limit int) []SkillUsage {
	var result []SkillUsage
	t.withState(func(s *trackerState) {
		if limit <= 0 || limit > len(s.history) {
			limit = len(s.history)
		}
		start := len(s.history) - limit
		if start < 0 {
			start = 0
		}
		result = make([]SkillUsage, limit)
		copy(result, s.history[start:])
	})
	return result
}

// GetStats returns aggregate statistics.
func (t *Tracker) GetStats() *UsageStats {
	var stats *UsageStats
	t.withState(func(s *trackerState) {
		stats = &UsageStats{
			TotalInvocations:  s.stats.TotalInvocations,
			SuccessfulCount:   s.stats.SuccessfulCount,
			FailedCount:       s.stats.FailedCount,
			BySkill:           make(map[string]*SkillStats, len(s.stats.BySkill)),
			ByCategory:        make(map[string]*CategoryStats, len(s.stats.ByCategory)),
			ByMatchType:       make(map[MatchType]int64, len(s.stats.ByMatchType)),
			AverageConfidence: s.stats.AverageConfidence,
			AverageDuration:   s.stats.AverageDuration,
			LastUpdated:       s.stats.LastUpdated,
		}
		for k, v := range s.stats.BySkill {
			stats.BySkill[k] = v
		}
		for k, v := range s.stats.ByCategory {
			stats.ByCategory[k] = v
		}
		for k, v := range s.stats.ByMatchType {
			stats.ByMatchType[k] = v
		}
	})
	return stats
}

// GetSkillStats returns statistics for a specific skill.
func (t *Tracker) GetSkillStats(skillName string) *SkillStats {
	var result *SkillStats
	t.withState(func(s *trackerState) {
		result = s.stats.BySkill[skillName]
	})
	return result
}

// GetTopSkills returns the most frequently used skills.
func (t *Tracker) GetTopSkills(limit int) []*SkillStats {
	var skills []*SkillStats
	t.withState(func(s *trackerState) {
		skills = make([]*SkillStats, 0, len(s.stats.BySkill))
		for _, ss := range s.stats.BySkill {
			skills = append(skills, ss)
		}
	})

	// Sort by invocation count (outside the lock — `skills` is now ours)
	for i := 0; i < len(skills)-1; i++ {
		for j := i + 1; j < len(skills); j++ {
			if skills[j].InvocationCount > skills[i].InvocationCount {
				skills[i], skills[j] = skills[j], skills[i]
			}
		}
	}

	if limit > 0 && limit < len(skills) {
		skills = skills[:limit]
	}
	return skills
}

// Reset clears all tracking data.
func (t *Tracker) Reset() {
	t.state.Put(trackerStateKey, newTrackerState())
}
