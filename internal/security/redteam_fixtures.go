package security

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	redteam "digital.vasic.redteam"
)

// FixtureSuiteReport summarises a RunFixtureSuite invocation. Total is
// the number of fixtures replayed; Blocked counts the fixtures whose
// prompt triggered at least one guardrail with Action = Block; Passed
// counts the rest. FailedReasons captures per-fixture notes for the
// fixtures that slipped through, so a regression surfaces the payload
// rather than only the count.
type FixtureSuiteReport struct {
	AttackClass   redteam.AttackClass
	Total         int
	Blocked       int
	Passed        int
	FailedReasons []FixtureFailure
}

// FixtureFailure identifies a fixture whose prompt was NOT blocked by
// the guardrail pipeline. Populated for every increment of Passed.
type FixtureFailure struct {
	FixtureID string
	Prompt    string
	Expected  string // expected_guardrail_trigger from the YAML
}

// AttachGuardrails wires a guardrail pipeline into the red-teamer for
// use by RunFixtureSuite. Passing nil detaches the pipeline. The
// argument is typed as the minimal GuardrailInputChecker interface so
// tests can supply fakes without constructing the full pipeline.
func (rt *DeepTeamRedTeamer) AttachGuardrails(pipeline GuardrailInputChecker) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.guardrails = pipeline
}

// RunFixtureSuite replays every fixture for the given attack class
// through the attached guardrail pipeline and reports how many were
// blocked. Returns an error if no pipeline is attached or the class
// is not a supported fixture class.
//
// Behaviour when the fixture file is empty (fixtures: [] in YAML):
// Total == 0, Blocked == 0, Passed == 0 — a trivially-passing report.
// Once the corpus is populated, the same assertions become substantive.
func (rt *DeepTeamRedTeamer) RunFixtureSuite(
	ctx context.Context, class redteam.AttackClass,
) (*FixtureSuiteReport, error) {
	rt.mu.RLock()
	pipeline := rt.guardrails
	rt.mu.RUnlock()

	if pipeline == nil {
		return nil, errors.New(
			"redteam: RunFixtureSuite requires a guardrail pipeline; " +
				"call AttachGuardrails first",
		)
	}

	loaded, err := redteam.LoadByClass(class)
	if err != nil {
		return nil, fmt.Errorf("redteam: load fixtures for %q: %w", class, err)
	}

	report := &FixtureSuiteReport{
		AttackClass: class,
		Total:       len(loaded),
	}

	for _, fixture := range loaded {
		blocked, reason := rt.runSingleFixture(ctx, pipeline, fixture)
		if blocked {
			report.Blocked++
			continue
		}
		report.Passed++
		report.FailedReasons = append(report.FailedReasons, FixtureFailure{
			FixtureID: fixture.ID,
			Prompt:    fixture.Prompt,
			Expected:  fixture.ExpectedGuardrailTrigger,
		})
		rt.logger.WithFields(logrus.Fields{
			"fixture_id":   fixture.ID,
			"attack_class": class,
			"expected":     fixture.ExpectedGuardrailTrigger,
			"failure_note": reason,
		}).Warn("redteam fixture slipped through guardrails")
	}

	return report, nil
}

// runSingleFixture returns whether the fixture prompt was blocked by
// any guardrail in the pipeline, plus a note used only for logging
// when the fixture passes (e.g., no results, pipeline error).
func (rt *DeepTeamRedTeamer) runSingleFixture(
	ctx context.Context, pipeline GuardrailInputChecker, fixture redteam.Fixture,
) (blocked bool, note string) {
	results, err := pipeline.CheckInput(ctx, fixture.Prompt, map[string]interface{}{
		"fixture_id":   fixture.ID,
		"attack_class": string(fixture.AttackClass),
	})
	if err != nil {
		// Pipeline failure is not "blocked" — the attack reached the
		// model path unreviewed. Surface the error as a failure note.
		return false, fmt.Sprintf("pipeline error: %v", err)
	}
	for _, r := range results {
		if r != nil && r.Triggered && r.Action == GuardrailActionBlock {
			return true, ""
		}
	}
	return false, "no guardrail blocked the prompt"
}
