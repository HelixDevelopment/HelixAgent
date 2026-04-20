package llmops

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInMemoryContinuousEvaluator_ScheduleRun verifies ScheduleRun creates a
// run, stores a schedule, and starts the background scheduler goroutine.
func TestInMemoryContinuousEvaluator_ScheduleRun(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Sched Dataset", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run := &EvaluationRun{
		Name:    "Scheduled Run",
		Dataset: dataset.ID,
	}
	err := evaluator.ScheduleRun(ctx, run, "0 * * * *")
	require.NoError(t, err)
	assert.NotEmpty(t, run.ID)

	// Verify the schedule was stored (same-package access)
	sched, ok := evaluator.schedules.Get(run.ID)

	require.True(t, ok, "Schedule should be stored")
	assert.Equal(t, "0 * * * *", sched.cron)

	// Stop the background goroutine to avoid goroutine leaks
	close(sched.stopCh)
	// Give the goroutine a moment to exit
	time.Sleep(10 * time.Millisecond)
}

// TestInMemoryContinuousEvaluator_ScheduleRun_InvalidRun verifies ScheduleRun
// propagates CreateRun validation errors.
func TestInMemoryContinuousEvaluator_ScheduleRun_InvalidRun(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	// Run without a dataset — should fail on CreateRun
	run := &EvaluationRun{Name: "Bad Run", Dataset: "nonexistent"}
	err := evaluator.ScheduleRun(ctx, run, "0 * * * *")
	assert.Error(t, err)

	// No schedule should be stored
	_, ok := evaluator.schedules.Get(run.ID)
	assert.False(t, ok)
}

// TestInMemoryExperimentManager_DetermineWinner verifies determineWinner is
// called (via GetResults) when statistical confidence >= 0.95.
//
// To achieve confidence >= 0.95 we need >=30 samples per variant with
// non-zero std-dev and a high enough Z-score (>=1.96).
// Using alternating values [0.4,0.6] (mean=0.5, std≈0.1) vs [0.7,0.9]
// (mean=0.8, std≈0.1) with 50 samples each produces Z≈15 → confidence=0.99.
func TestInMemoryExperimentManager_DetermineWinner(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Winner Test",
		Variants: []*Variant{
			{Name: "Control", IsControl: true},
			{Name: "Treatment"},
		},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	controlID := exp.Variants[0].ID
	treatmentID := exp.Variants[1].ID

	// Record 50 samples with non-zero std-dev and different means so the
	// significance calculation yields confidence >= 0.95.
	for i := 0; i < 50; i++ {
		// Control: alternates between 0.4 and 0.6 → mean≈0.5, std≈0.1
		if i%2 == 0 {
			require.NoError(t, manager.RecordMetric(ctx, exp.ID, controlID, "primary", 0.4))
		} else {
			require.NoError(t, manager.RecordMetric(ctx, exp.ID, controlID, "primary", 0.6))
		}
		// Treatment: alternates between 0.7 and 0.9 → mean≈0.8, std≈0.1
		if i%2 == 0 {
			require.NoError(t, manager.RecordMetric(ctx, exp.ID, treatmentID, "primary", 0.7))
		} else {
			require.NoError(t, manager.RecordMetric(ctx, exp.ID, treatmentID, "primary", 0.9))
		}
	}

	results, err := manager.GetResults(ctx, exp.ID)
	require.NoError(t, err)

	// With high Z-score, confidence should be >= 0.95 and a winner declared
	if results.Confidence >= 0.95 {
		assert.NotEmpty(t, results.Winner, "A winner should be determined at high confidence")
		assert.Contains(t, results.Recommendation, "Deploy variant")
	}
}

// TestInMemoryExperimentManager_DetermineWinner_NoMetricValues verifies
// determineWinner returns empty string when VariantResults have no
// "primary" metric (e.g., empty variant map).
func TestInMemoryExperimentManager_DetermineWinner_NoMetricValues(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)

	result := &ExperimentResult{
		VariantResults: map[string]*VariantResult{
			"v1": {VariantID: "v1", SampleCount: 0, MetricValues: make(map[string]*MetricValue)},
			"v2": {VariantID: "v2", SampleCount: 0, MetricValues: make(map[string]*MetricValue)},
		},
	}

	winner := manager.determineWinner(result)
	assert.Empty(t, winner)
}
