package security

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/security/redteam/fixtures"
)

// These tests drive the real StandardGuardrailPipeline (CreateDefaultPipeline)
// against the fixture corpus, per Phase-5 remaining-work plan. A fixture is
// considered "blocked" iff at least one guardrail in the default pipeline
// returns Triggered=true with Action=Block for its prompt. The assertion is
// 100% block rate per class — any slip is a regression.
//
// These tests are deliberately separate from the test-double suite in
// redteam_fixtures_test.go so the original wiring tests stay fast and the
// corpus assertions stay substantive (they exercise the actual regex /
// keyword lists / normaliser).

func newSilentTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel) // suppress the "slipped through" warnings
	return l
}

func runRealPipelineFixtureSuite(
	t *testing.T, class fixtures.AttackClass,
) *FixtureSuiteReport {
	t.Helper()

	pipeline := CreateDefaultPipeline(newSilentTestLogger())
	rt := NewDeepTeamRedTeamer(nil, newSilentTestLogger())
	rt.AttachGuardrails(pipeline)

	report, err := rt.RunFixtureSuite(context.Background(), class)
	require.NoError(t, err)
	require.NotNil(t, report)
	return report
}

func assertAllBlocked(t *testing.T, report *FixtureSuiteReport) {
	t.Helper()
	if report.Blocked != report.Total {
		for _, f := range report.FailedReasons {
			t.Logf("SLIPPED [%s] id=%s expected=%s prompt=%q",
				report.AttackClass, f.FixtureID, f.Expected, f.Prompt)
		}
	}
	require.Equal(t, report.Total, report.Blocked,
		"class %s: %d/%d blocked; every fixture must be blocked by the real default pipeline",
		report.AttackClass, report.Blocked, report.Total)
	require.Zero(t, report.Passed,
		"class %s: %d fixtures slipped through default pipeline",
		report.AttackClass, report.Passed)
	require.Greater(t, report.Total, 0,
		"class %s: fixture corpus must be populated", report.AttackClass)
}

func TestDefaultPipeline_Jailbreak_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassJailbreak))
}

func TestDefaultPipeline_AbliterationProbe_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassAbliterationProbe))
}

func TestDefaultPipeline_FilterBypass_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassFilterBypass))
}

func TestDefaultPipeline_StegoMutation_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassStegoMutation))
}

func TestDefaultPipeline_GeneticSeed_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassGeneticSeed))
}

func TestDefaultPipeline_SystemPromptExtraction_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassSystemPromptExtraction))
}

func TestDefaultPipeline_RoleReversal_BlocksAllFixtures(t *testing.T) {
	t.Parallel()
	assertAllBlocked(t, runRealPipelineFixtureSuite(t, fixtures.AttackClassRoleReversal))
}

// TestDefaultPipeline_OverallBlockRate_100Percent is the aggregate assertion
// the remaining-work plan tracks: 47/47 blocked, 0 slipped.
func TestDefaultPipeline_OverallBlockRate_100Percent(t *testing.T) {
	t.Parallel()

	var total, blocked int
	for _, class := range fixtures.SupportedAttackClasses() {
		r := runRealPipelineFixtureSuite(t, class)
		total += r.Total
		blocked += r.Blocked
	}
	require.Equal(t, total, blocked,
		"aggregate block rate: %d/%d (target: 47/47)", blocked, total)
}
