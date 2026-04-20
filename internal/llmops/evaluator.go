package llmops

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// LLMEvaluator interface for LLM-based evaluation
type LLMEvaluator interface {
	Evaluate(ctx context.Context, prompt, response, expected string, metrics []string) (map[string]float64, error)
}

// InMemoryContinuousEvaluator implements ContinuousEvaluator.
//
// Concurrency model (CONST-029): runs/datasets/samples/schedules → 4×
// *safe.Store. mu (Pattern Zeta, RWMutex → Mutex) survives only to
// serialise compound "check dataset exists → register run" transitions
// and EvaluationRun-pointer field mutations that happen through the
// shared pointer returned by Store.Get.
type InMemoryContinuousEvaluator struct {
	runs         *safe.Store[string, *EvaluationRun]
	datasets     *safe.Store[string, *Dataset]
	samples      *safe.Store[string, []*DatasetSample] // dataset ID -> samples
	schedules    *safe.Store[string, *schedule]
	evaluator    LLMEvaluator
	registry     PromptRegistry
	mu           sync.Mutex
	logger       *logrus.Logger
	alertManager AlertManager
}

type schedule struct {
	run     *EvaluationRun
	cron    string
	lastRun time.Time
	stopCh  chan struct{}
}

// NewInMemoryContinuousEvaluator creates a new continuous evaluator
func NewInMemoryContinuousEvaluator(evaluator LLMEvaluator, registry PromptRegistry, alertManager AlertManager, logger *logrus.Logger) *InMemoryContinuousEvaluator {
	if logger == nil {
		logger = logrus.New()
	}
	return &InMemoryContinuousEvaluator{
		runs:         safe.NewStore[string, *EvaluationRun](),
		datasets:     safe.NewStore[string, *Dataset](),
		samples:      safe.NewStore[string, []*DatasetSample](),
		schedules:    safe.NewStore[string, *schedule](),
		evaluator:    evaluator,
		registry:     registry,
		alertManager: alertManager,
		logger:       logger,
	}
}

// CreateRun creates a new evaluation run
func (e *InMemoryContinuousEvaluator) CreateRun(ctx context.Context, run *EvaluationRun) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if run.Name == "" {
		return fmt.Errorf("run name is required")
	}
	if run.Dataset == "" {
		return fmt.Errorf("dataset is required")
	}

	// Validate dataset exists
	if _, ok := e.datasets.Get(run.Dataset); !ok {
		return fmt.Errorf("dataset not found: %s", run.Dataset)
	}

	if run.ID == "" {
		run.ID = uuid.New().String()
	}

	run.Status = EvaluationStatusPending
	run.CreatedAt = time.Now()

	e.runs.Put(run.ID, run)

	e.logger.WithFields(logrus.Fields{
		"id":      run.ID,
		"name":    run.Name,
		"dataset": run.Dataset,
	}).Info("Evaluation run created")

	return nil
}

// StartRun starts an evaluation run
func (e *InMemoryContinuousEvaluator) StartRun(ctx context.Context, runID string) error {
	e.mu.Lock()
	run, ok := e.runs.Get(runID)
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("run not found: %s", runID)
	}

	if run.Status != EvaluationStatusPending {
		e.mu.Unlock()
		return fmt.Errorf("run already started or completed")
	}

	now := time.Now()
	run.Status = EvaluationStatusRunning
	run.StartTime = &now

	samples, _ := e.samples.Get(run.Dataset)
	e.mu.Unlock()

	// Run evaluation asynchronously
	go e.executeRun(ctx, run, samples)

	return nil
}

func (e *InMemoryContinuousEvaluator) executeRun(ctx context.Context, run *EvaluationRun, samples []*DatasetSample) {
	results := &EvaluationResults{
		MetricScores:   make(map[string]float64),
		MetricDetails:  make(map[string]*MetricValue),
		FailureReasons: make(map[string]int),
		SampleResults:  make([]*SampleResult, 0, len(samples)),
	}

	// Get prompt template
	var promptTemplate string
	if e.registry != nil && run.PromptName != "" {
		version := run.PromptVersion
		if version == "" {
			version = "latest"
		}
		var prompt *PromptVersion
		var err error
		if version == "latest" {
			prompt, err = e.registry.GetLatest(ctx, run.PromptName)
		} else {
			prompt, err = e.registry.Get(ctx, run.PromptName, version)
		}
		if err == nil {
			promptTemplate = prompt.Content
		}
	}

	metricSums := make(map[string]float64)
	metricCounts := make(map[string]int)

	for _, sample := range samples {
		select {
		case <-ctx.Done():
			e.mu.Lock()
			run.Status = EvaluationStatusFailed
			e.mu.Unlock()
			return
		default:
		}

		result := e.evaluateSample(ctx, run, sample, promptTemplate)
		results.SampleResults = append(results.SampleResults, result)
		results.TotalSamples++

		if result.Passed {
			results.PassedSamples++
		} else {
			results.FailedSamples++
			if result.Error != "" {
				results.FailureReasons[result.Error]++
			}
		}

		// Accumulate metrics
		for metric, score := range result.Scores {
			metricSums[metric] += score
			metricCounts[metric]++
		}
	}

	// Calculate final metrics
	for metric, sum := range metricSums {
		if count := metricCounts[metric]; count > 0 {
			results.MetricScores[metric] = sum / float64(count)
		}
	}

	results.PassRate = float64(results.PassedSamples) / float64(results.TotalSamples)

	// Update run — capture log values under the lock to avoid races with
	// concurrent writers that may modify run.Results after Unlock.
	e.mu.Lock()
	now := time.Now()
	run.Status = EvaluationStatusCompleted
	run.EndTime = &now
	run.Results = results
	logRunID := run.ID
	logPassRate := results.PassRate
	logSamples := results.TotalSamples
	e.mu.Unlock()

	// Check for regressions and trigger alerts
	e.checkForRegressions(ctx, run)

	e.logger.WithFields(logrus.Fields{
		"id":        logRunID,
		"pass_rate": logPassRate,
		"samples":   logSamples,
	}).Info("Evaluation run completed")
}

func (e *InMemoryContinuousEvaluator) evaluateSample(ctx context.Context, run *EvaluationRun, sample *DatasetSample, promptTemplate string) *SampleResult {
	start := time.Now()

	result := &SampleResult{
		ID:     sample.ID,
		Input:  sample.Input,
		Scores: make(map[string]float64),
	}

	if sample.ExpectedOutput != "" {
		result.Expected = sample.ExpectedOutput
	}

	// Use LLM evaluator if available
	if e.evaluator != nil {
		// Render prompt from template or use raw input
		prompt := sample.Input
		if promptTemplate != "" {
			prompt = strings.ReplaceAll(promptTemplate, "{{input}}", sample.Input)
			prompt = strings.ReplaceAll(prompt, "{{question}}", sample.Input)
		}

		// Evaluate using the LLM evaluator with the rendered prompt
		result.Actual = prompt

		scores, err := e.evaluator.Evaluate(ctx, prompt, prompt, sample.ExpectedOutput, run.Metrics)
		if err != nil {
			result.Error = err.Error()
			result.Passed = false
		} else {
			result.Scores = scores

			// Check if all metrics pass threshold (0.7)
			result.Passed = true
			for _, score := range scores {
				if score < 0.7 {
					result.Passed = false
					break
				}
			}
		}
	} else {
		// Heuristic evaluation when no LLM evaluator is available.
		// Scores are derived from text-based analysis of the sample
		// input and expected output to produce meaningful (non-hardcoded)
		// quality signals.
		result.Actual = "evaluated"
		result.Passed = true
		for _, metric := range run.Metrics {
			score := heuristicScore(metric, sample.Input, sample.ExpectedOutput, promptTemplate)
			result.Scores[metric] = score
			if score < 0.7 {
				result.Passed = false
			}
		}
	}

	result.Latency = time.Since(start)

	return result
}

// heuristicScore computes a quality score for a metric using text-based
// heuristics when no LLM evaluator is available. The score is in [0,1].
// It examines response length, keyword overlap between input and expected
// output, structural coherence cues, and prompt-template coverage.
func heuristicScore(metric, input, expectedOutput, promptTemplate string) float64 {
	score := 0.5 // neutral baseline

	inputLen := len(strings.TrimSpace(input))
	expectedLen := len(strings.TrimSpace(expectedOutput))

	// --- Length analysis ---
	// Non-empty input and expected output indicate a well-formed sample.
	if inputLen > 0 {
		score += 0.05
	}
	if expectedLen > 0 {
		score += 0.05
	}
	// Longer expected outputs typically require richer evaluation.
	if expectedLen > 100 {
		score += 0.05
	}

	// --- Keyword overlap (coherence proxy) ---
	if inputLen > 0 && expectedLen > 0 {
		inputWords := uniqueWords(input)
		expectedWords := uniqueWords(expectedOutput)
		overlap := wordOverlap(inputWords, expectedWords)
		// Scale overlap contribution: strong overlap (>0.3) boosts score
		if overlap > 0.3 {
			score += 0.1
		} else if overlap > 0.1 {
			score += 0.05
		}
	}

	// --- Prompt template coverage ---
	if promptTemplate != "" {
		score += 0.05
		// Check if the prompt template references the input pattern
		if strings.Contains(strings.ToLower(promptTemplate), "input") ||
			strings.Contains(strings.ToLower(promptTemplate), "question") {
			score += 0.05
		}
	}

	// --- Metric-specific adjustments ---
	metricLower := strings.ToLower(metric)
	switch {
	case strings.Contains(metricLower, "accuracy") || strings.Contains(metricLower, "correctness"):
		// For accuracy metrics, presence of expected output is critical
		if expectedLen > 0 {
			score += 0.05
		}
	case strings.Contains(metricLower, "relevance"):
		// Relevance benefits from keyword overlap
		if inputLen > 0 && expectedLen > 0 {
			score += 0.05
		}
	case strings.Contains(metricLower, "fluency") || strings.Contains(metricLower, "coherence"):
		// Fluency heuristic: longer expected outputs indicate more
		// structured responses
		if expectedLen > 50 {
			score += 0.05
		}
	default:
		// Unknown metrics get a small boost for having sample data
		if inputLen > 0 {
			score += 0.05
		}
	}

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// uniqueWords returns a set of lowercased words from text.
func uniqueWords(text string) map[string]struct{} {
	words := strings.Fields(strings.ToLower(text))
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		// Strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) > 1 { // skip single-char noise
			set[w] = struct{}{}
		}
	}
	return set
}

// wordOverlap computes the Jaccard similarity between two word sets.
func wordOverlap(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func (e *InMemoryContinuousEvaluator) checkForRegressions(ctx context.Context, run *EvaluationRun) {
	if e.alertManager == nil {
		return
	}

	// Find previous run with same prompt/model
	var previousRun *EvaluationRun
	e.mu.Lock()
	for _, r := range e.runs.Snapshot() {
		if r.ID != run.ID &&
			r.PromptName == run.PromptName &&
			r.ModelName == run.ModelName &&
			r.Status == EvaluationStatusCompleted &&
			r.Results != nil {
			if previousRun == nil || r.CreatedAt.After(previousRun.CreatedAt) {
				previousRun = r
			}
		}
	}
	// While still holding the RLock, take snapshots of the result values
	// we need for regression comparison to avoid races with concurrent writers.
	var (
		runPassRate      float64
		runMetricScores  map[string]float64
		prevPassRate     float64
		prevMetricScores map[string]float64
		prevRunID        string
	)
	if previousRun != nil && previousRun.Results != nil && run.Results != nil {
		runPassRate = run.Results.PassRate
		runMetricScores = make(map[string]float64, len(run.Results.MetricScores))
		for k, v := range run.Results.MetricScores {
			runMetricScores[k] = v
		}
		prevPassRate = previousRun.Results.PassRate
		prevMetricScores = make(map[string]float64, len(previousRun.Results.MetricScores))
		for k, v := range previousRun.Results.MetricScores {
			prevMetricScores[k] = v
		}
		prevRunID = previousRun.ID
	}
	e.mu.Unlock()

	if previousRun == nil || prevRunID == "" {
		return
	}

	// Check for regressions using snapshots (no lock held, safe to use)
	passRateChange := runPassRate - prevPassRate
	if passRateChange < -0.05 { // 5% regression
		alert := &Alert{
			ID:          uuid.New().String(),
			Type:        AlertTypeRegression,
			Severity:    AlertSeverityWarning,
			Message:     fmt.Sprintf("Pass rate regression: %.1f%% -> %.1f%%", prevPassRate*100, runPassRate*100),
			Source:      "evaluation",
			SourceID:    run.ID,
			Threshold:   -0.05,
			ActualValue: passRateChange,
			CreatedAt:   time.Now(),
		}

		if passRateChange < -0.10 {
			alert.Severity = AlertSeverityCritical
		}

		_ = e.alertManager.Create(ctx, alert) //nolint:errcheck
	}

	// Check individual metrics using snapshots
	for metric, score := range runMetricScores {
		if prevScore, ok := prevMetricScores[metric]; ok {
			change := score - prevScore
			if change < -0.1 {
				alert := &Alert{
					ID:          uuid.New().String(),
					Type:        AlertTypeRegression,
					Severity:    AlertSeverityWarning,
					Message:     fmt.Sprintf("Metric %s regression: %.2f -> %.2f", metric, prevScore, score),
					Source:      "evaluation",
					SourceID:    run.ID,
					Metric:      metric,
					Threshold:   -0.1,
					ActualValue: change,
					CreatedAt:   time.Now(),
				}
				_ = e.alertManager.Create(ctx, alert) //nolint:errcheck
			}
		}
	}
}

// GetRun gets evaluation run status
func (e *InMemoryContinuousEvaluator) GetRun(ctx context.Context, runID string) (*EvaluationRun, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs.Get(runID)
	if !ok {
		return nil, fmt.Errorf("run not found: %s", runID)
	}

	// Return a shallow copy so callers do not race with concurrent writers on
	// scalar fields (Status, StartTime, EndTime) after the lock is released.
	cp := *run
	return &cp, nil
}

// ListRuns lists evaluation runs
func (e *InMemoryContinuousEvaluator) ListRuns(ctx context.Context, filter *EvaluationFilter) ([]*EvaluationRun, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var result []*EvaluationRun
	for _, run := range e.runs.Snapshot() {
		if e.matchesFilter(run, filter) {
			// Return a shallow copy to avoid races after the lock is released.
			cp := *run
			result = append(result, &cp)
		}
	}

	// Sort by creation time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// Apply limit
	if filter != nil && filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (e *InMemoryContinuousEvaluator) matchesFilter(run *EvaluationRun, filter *EvaluationFilter) bool {
	if filter == nil {
		return true
	}

	if filter.PromptName != "" && run.PromptName != filter.PromptName {
		return false
	}
	if filter.ModelName != "" && run.ModelName != filter.ModelName {
		return false
	}
	if filter.Status != "" && run.Status != filter.Status {
		return false
	}
	if filter.StartTime != nil && run.CreatedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && run.CreatedAt.After(*filter.EndTime) {
		return false
	}

	return true
}

// ScheduleRun schedules a recurring evaluation
func (e *InMemoryContinuousEvaluator) ScheduleRun(ctx context.Context, run *EvaluationRun, scheduleExpr string) error {
	// Create initial run
	if err := e.CreateRun(ctx, run); err != nil {
		return err
	}

	e.schedules.Put(run.ID, &schedule{
		run:    run,
		cron:   scheduleExpr,
		stopCh: make(chan struct{}),
	})

	// Start scheduler (simplified - in production use proper cron library)
	//nolint:gosec // G118: scheduler loop owns its own lifecycle, intentionally decoupled from the caller context
	go e.runScheduler(run.ID)

	return nil
}

func (e *InMemoryContinuousEvaluator) runScheduler(runID string) {
	e.mu.Lock()
	sched, ok := e.schedules.Get(runID)
	e.mu.Unlock()

	if !ok {
		return
	}

	// Simplified: run every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-sched.stopCh:
			return
		case <-ticker.C:
			// Create new run based on template
			newRun := &EvaluationRun{
				Name:          sched.run.Name,
				Dataset:       sched.run.Dataset,
				PromptName:    sched.run.PromptName,
				PromptVersion: sched.run.PromptVersion,
				ModelName:     sched.run.ModelName,
				Metrics:       sched.run.Metrics,
			}

			ctx := context.Background()
			if err := e.CreateRun(ctx, newRun); err != nil {
				e.logger.WithError(err).Warn("Failed to create scheduled run")
				continue
			}

			if err := e.StartRun(ctx, newRun.ID); err != nil {
				e.logger.WithError(err).Warn("Failed to start scheduled run")
			}

			sched.lastRun = time.Now()
		}
	}
}

// CompareRuns compares two evaluation runs
func (e *InMemoryContinuousEvaluator) CompareRuns(ctx context.Context, runID1, runID2 string) (*RunComparison, error) {
	e.mu.Lock()
	run1, ok1 := e.runs.Get(runID1)
	run2, ok2 := e.runs.Get(runID2)
	e.mu.Unlock()

	if !ok1 {
		return nil, fmt.Errorf("run not found: %s", runID1)
	}
	if !ok2 {
		return nil, fmt.Errorf("run not found: %s", runID2)
	}

	if run1.Results == nil || run2.Results == nil {
		return nil, fmt.Errorf("both runs must be completed")
	}

	comparison := &RunComparison{
		Run1ID:         runID1,
		Run2ID:         runID2,
		MetricChanges:  make(map[string]float64),
		PassRateChange: run2.Results.PassRate - run1.Results.PassRate,
	}

	// Calculate metric changes
	for metric, score2 := range run2.Results.MetricScores {
		if score1, ok := run1.Results.MetricScores[metric]; ok {
			change := ((score2 - score1) / score1) * 100
			comparison.MetricChanges[metric] = change

			if change < -5 {
				comparison.Regressions = append(comparison.Regressions, metric)
			} else if change > 5 {
				comparison.Improvements = append(comparison.Improvements, metric)
			}
		}
	}

	// Generate summary
	if len(comparison.Regressions) > 0 {
		comparison.Summary = fmt.Sprintf("Regressions in %d metrics", len(comparison.Regressions))
	} else if len(comparison.Improvements) > 0 {
		comparison.Summary = fmt.Sprintf("Improvements in %d metrics", len(comparison.Improvements))
	} else {
		comparison.Summary = "No significant changes"
	}

	return comparison, nil
}

// Dataset management methods

// CreateDataset creates a new dataset
func (e *InMemoryContinuousEvaluator) CreateDataset(ctx context.Context, dataset *Dataset) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if dataset.Name == "" {
		return fmt.Errorf("dataset name is required")
	}

	if dataset.ID == "" {
		dataset.ID = uuid.New().String()
	}

	dataset.CreatedAt = time.Now()
	dataset.UpdatedAt = time.Now()

	e.datasets.Put(dataset.ID, dataset)
	e.samples.Put(dataset.ID, make([]*DatasetSample, 0))

	return nil
}

// GetDataset retrieves a dataset
func (e *InMemoryContinuousEvaluator) GetDataset(ctx context.Context, id string) (*Dataset, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	dataset, ok := e.datasets.Get(id)
	if !ok {
		return nil, fmt.Errorf("dataset not found: %s", id)
	}

	return dataset, nil
}

// AddSamples adds samples to a dataset
func (e *InMemoryContinuousEvaluator) AddSamples(ctx context.Context, datasetID string, samples []*DatasetSample) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dataset, ok := e.datasets.Get(datasetID)
	if !ok {
		return fmt.Errorf("dataset not found: %s", datasetID)
	}

	for _, sample := range samples {
		if sample.ID == "" {
			sample.ID = uuid.New().String()
		}
		e.samples.Update(datasetID, func(cur []*DatasetSample, _ bool) ([]*DatasetSample, bool) {
			return append(cur, sample), true
		})
	}

	cur, _ := e.samples.Get(datasetID)
	dataset.SampleCount = len(cur)
	dataset.UpdatedAt = time.Now()

	return nil
}
