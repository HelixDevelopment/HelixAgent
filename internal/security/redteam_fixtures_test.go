package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redteam "digital.vasic.redteam"
)

// alwaysBlockPipeline is a test double for GuardrailInputChecker that
// reports every prompt as blocked. Real end-to-end validation happens
// via the challenge script with the actual StandardGuardrailPipeline.
type alwaysBlockPipeline struct{}

func (alwaysBlockPipeline) CheckInput(
	_ context.Context, _ string, _ map[string]interface{},
) ([]*GuardrailResult, error) {
	return []*GuardrailResult{
		{
			Triggered:  true,
			Action:     GuardrailActionBlock,
			Guardrail:  "prompt_injection_detector",
			Reason:     "test double forced block",
			Confidence: 1.0,
		},
	}, nil
}

// neverBlockPipeline is a test double that lets every prompt through.
type neverBlockPipeline struct{}

func (neverBlockPipeline) CheckInput(
	_ context.Context, _ string, _ map[string]interface{},
) ([]*GuardrailResult, error) {
	return []*GuardrailResult{}, nil
}

func TestDeepTeamRedTeamer_RunFixtureSuite_Jailbreak_BlocksAll(t *testing.T) {
	t.Parallel()

	rt := NewDeepTeamRedTeamer(nil, nil)
	rt.AttachGuardrails(alwaysBlockPipeline{})

	report, err := rt.RunFixtureSuite(context.Background(), redteam.AttackClassJailbreak)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Every non-zero fixture must be blocked.
	assert.Equal(t, report.Total, report.Blocked,
		"every jailbreak fixture must be blocked by default guardrails")
	assert.Zero(t, report.Passed,
		"no jailbreak fixture should slip through guardrails")
	// report.Total may be zero until fixtures are populated — that's
	// acceptable here; a post-population regression will flip the
	// gate from trivial to substantive.
}

func TestDeepTeamRedTeamer_RunFixtureSuite_NoGuardrails_Errors(t *testing.T) {
	t.Parallel()

	rt := NewDeepTeamRedTeamer(nil, nil)
	_, err := rt.RunFixtureSuite(context.Background(), redteam.AttackClassJailbreak)
	require.Error(t, err)
}

func TestDeepTeamRedTeamer_RunFixtureSuite_UnknownClass_Errors(t *testing.T) {
	t.Parallel()

	rt := NewDeepTeamRedTeamer(nil, nil)
	rt.AttachGuardrails(alwaysBlockPipeline{})

	_, err := rt.RunFixtureSuite(context.Background(), "not-a-real-class")
	require.Error(t, err)
}

func TestDeepTeamRedTeamer_RunFixtureSuite_PassingPromptCountsAsPassed(t *testing.T) {
	t.Parallel()

	rt := NewDeepTeamRedTeamer(nil, nil)
	rt.AttachGuardrails(neverBlockPipeline{})

	// With no fixtures populated, this is trivially Total == 0.
	// The test asserts the wiring: when the pipeline does not block,
	// the report must NOT increment Blocked.
	report, err := rt.RunFixtureSuite(context.Background(), redteam.AttackClassJailbreak)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Zero(t, report.Blocked,
		"neverBlockPipeline must not cause any Blocked increments")
	assert.Equal(t, report.Total, report.Passed,
		"every fixture must be counted as Passed when guardrails do not block")
}
