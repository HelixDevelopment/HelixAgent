package services

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecDrivenDefault_DevelopmentRoutesThroughSpec is the §11.4.115
// RED→GREEN polarity-switch regression guard for "spec-driven development is
// the DEFAULT planning flow".
//
//   - RED_MODE=1 (explicit, run against a PRE-FIX checkout) reproduces the
//     historical defect: ordinary development/implementation/feature/creation
//     requests did NOT route through the HelixSpecifier 7-phase spec flow — they
//     were treated as plain code work. Asserting RequiresSpecKit==false for these
//     requests reproduces the broken-for-the-user state on the PRE-FIX engine.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard that runs in the
//     normal suite) asserts the fix: those same development requests now default
//     to the spec-driven path (RequiresSpecKit==true), while genuinely trivial
//     conversational / single-action / pure-analysis turns still bypass it.
//
// One source, two roles: the bug-catcher IS the regression guard. The standing
// suite role is GREEN (default); RED_MODE=1 is the explicit pre-fix reproduction.
func TestSpecDrivenDefault_DevelopmentRoutesThroughSpec(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	classifier := NewEnhancedIntentClassifier(registry, logger)

	redMode := os.Getenv("RED_MODE") == "1" // default = GREEN standing guard; RED_MODE=1 = explicit pre-fix reproduction

	// Development / planning class requests that a user would expect to be
	// spec-driven by default. These are deliberately "ordinary" — NOT
	// whole-functionality / major-refactor / big-creation — to prove the
	// default routing, not the old opt-in heuristic.
	devRequests := []*EnhancedIntentResult{
		{ // ordinary "implement feature X"
			Granularity: GranularitySmallCreation, GranularityScore: 0.6,
			ActionType: ActionCreation, ActionTypeScore: 0.7,
		},
		{ // small feature build
			Granularity: GranularityBigCreation, GranularityScore: 0.6,
			ActionType: ActionCreation, ActionTypeScore: 0.7,
		},
		{ // an improvement / enhancement
			Granularity: GranularitySmallCreation, GranularityScore: 0.6,
			ActionType: ActionImprovements, ActionTypeScore: 0.7,
		},
		{ // a small fix that still produces code
			Granularity: GranularitySmallCreation, GranularityScore: 0.6,
			ActionType: ActionFixing, ActionTypeScore: 0.7,
		},
	}

	for i, r := range devRequests {
		got := classifier.shouldUseSpecKit(r)
		if redMode {
			// PRE-FIX: development requests did NOT route through spec.
			require.Falsef(t, got,
				"RED_MODE=1: dev request #%d (gran=%s action=%s) must NOT route through spec on the pre-fix engine (defect reproduction)",
				i, r.Granularity, r.ActionType)
		} else {
			// POST-FIX: spec-driven is the DEFAULT for development requests.
			require.Truef(t, got,
				"RED_MODE=0 (GREEN guard): dev request #%d (gran=%s action=%s) MUST default to the spec-driven 7-phase flow",
				i, r.Granularity, r.ActionType)
		}
	}

	// Trivial-bypass invariant holds in BOTH polarities: conversational,
	// single-action, and pure-analysis turns must NEVER spec-drive (don't
	// spec "what is 2+2"). This must stay true after the default-on fix.
	trivial := []*EnhancedIntentResult{
		{ActionType: ActionConversation, Granularity: GranularitySingleAction},
		{ActionType: ActionAnalysis, Granularity: GranularitySingleAction},
		{ActionType: ActionSingleOp, Granularity: GranularitySingleAction},
	}
	for i, r := range trivial {
		assert.Falsef(t, classifier.shouldUseSpecKit(r),
			"trivial turn #%d (gran=%s action=%s) must always bypass the spec flow",
			i, r.Granularity, r.ActionType)
	}
}
