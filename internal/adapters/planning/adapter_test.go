package planning_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	planningadapter "dev.helix.agent/internal/adapters/planning"
	extplanning "digital.vasic.planning/planning"
)

func TestAdapter_New_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	require.NotNil(t, adapter)
}

func TestAdapter_DefaultHiPlanConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultHiPlanConfig()
	assert.NotZero(t, config.MaxMilestones)
	assert.NotZero(t, config.MaxStepsPerMilestone)
}

func TestAdapter_DefaultMCTSConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultMCTSConfig()
	assert.NotZero(t, config.MaxIterations)
}

func TestAdapter_DefaultTreeOfThoughtsConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultTreeOfThoughtsConfig()
	assert.NotZero(t, config.MaxDepth)
	assert.NotZero(t, config.MaxBranches)
}

func TestAdapter_NewHiPlan_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultHiPlanConfig()

	generator := adapter.NewLLMMilestoneGenerator(nil)
	executor := &noopStepExecutor{}

	hp := adapter.NewHiPlan(config, generator, executor)
	require.NotNil(t, hp)
}

func TestAdapter_NewMCTS_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultMCTSConfig()

	actionGen := extplanning.NewCodeActionGenerator(nil, logger)
	rewardFunc := extplanning.NewCodeRewardFunction(nil, nil, logger)
	rolloutPolicy := extplanning.NewDefaultRolloutPolicy(actionGen, rewardFunc)

	m := adapter.NewMCTS(config, actionGen, rewardFunc, rolloutPolicy)
	require.NotNil(t, m)
}

func TestAdapter_NewTreeOfThoughts_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := planningadapter.New(logger)
	config := adapter.DefaultTreeOfThoughtsConfig()

	generator := adapter.NewLLMThoughtGenerator(nil, 0.7)
	evaluator := adapter.NewLLMThoughtEvaluator(nil)

	tot := adapter.NewTreeOfThoughts(config, generator, evaluator)
	require.NotNil(t, tot)
}

// noopStepExecutor satisfies the extplanning.StepExecutor interface with no-op implementations.
type noopStepExecutor struct{}

func (e *noopStepExecutor) Execute(
	_ context.Context,
	_ *extplanning.PlanStep,
	_ []string,
) (*extplanning.StepResult, error) {
	return &extplanning.StepResult{Success: true}, nil
}

func (e *noopStepExecutor) Validate(
	_ context.Context,
	_ *extplanning.PlanStep,
	_ *extplanning.StepResult,
) error {
	return nil
}
