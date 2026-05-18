// Package extended — round-79 §11.4 anti-bluff fix.
//
// This file provides a concrete, wireable PlanLLM implementation
// (LLMResponderBackedPlanLLM) that turns any LLMResponder (a single-
// method Generate-style interface) into a real PlanLLM. It closes
// the round-31 ErrPlanGenerationLLMNotWired sentinel by giving
// consumers a ready-made concrete implementation they can wire via
// (*PlanningHandlerExtensions).SetPlanLLM.
//
// Round-29 §11.4 audit (commit 93e63ada) replaced a hardcoded
// 5-step template ("Analyze requirements" / "Design solution
// approach" / "Implement the solution" / "Test and verify" /
// "Review and finalize") in generatePlanSteps with the PlanLLM
// interface + injection point + ErrPlanGenerationLLMNotWired
// sentinel. Round-31 documented the wiring contract. Round-79
// (this file, 2026-05-18) closes the gap by providing
// LLMResponderBackedPlanLLM, dependency-light and reusable from
// any LLMResponder source (LLMOps HTTPResponder per round-62,
// Ollama client, OpenAI client, ad-hoc test responders, etc.).
//
// The implementation:
//   - builds a structured plan-generation prompt template that
//     instructs the model to emit a numbered list of STEP markers
//     (`STEP 1: ...`, `STEP 2: ...`, ...) — no JSON / schema
//     coupling, no provider-specific format;
//   - invokes responder.Generate(ctx, prompt) — real call to a
//     real model, no simulation;
//   - parses the response with a permissive regex
//     (`^\s*STEP\s+(\d+)\s*[:.\-]?\s*(.+)$`) to extract action
//     text from each matching line;
//   - REFUSES to fabricate empty / synthetic steps when the
//     response carries zero STEP markers — returns
//     ErrPlanLLMResponseUnparseable so the caller sees the gap
//     honestly instead of silently receiving an empty plan.
//
// Constitutional anchors:
//   - CONST-035 / Article XI §11.9 (anti-bluff: no fabrication
//     from empty / unparseable model output);
//   - CONST-050(A) (production code never imports test mocks —
//     this file lives alongside planning.go in the production
//     package and imports nothing from a mock path);
//   - CONST-051(B) (decoupling — LLMResponder is the minimal
//     interface so helix_agent stays project-not-aware: any
//     consumer injects ANY implementation without bringing
//     HelixCode-specific context into this submodule);
//   - Round-31 ErrPlanGenerationLLMNotWired remains the sentinel
//     for the unwired-handler case — round-79 ADDS a wireable
//     impl rather than removing the sentinel.
package extended

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// LLMResponder is the minimal interface helix_agent's planning
// pipeline requires from any LLM client to back a real PlanLLM.
// Consumers inject their own implementation (e.g. an HTTP-backed
// responder from the LLMOps round-62 work, an Ollama client, an
// OpenAI client, a test fake, ...).
//
// Keeping this interface minimal (single Generate method) is a
// deliberate CONST-051(B) decoupling choice: helix_agent MUST stay
// project-not-aware and fully reusable; coupling to a concrete LLM
// SDK or HelixCode-internal type would violate that mandate. The
// shape intentionally mirrors HelixSpecifier's round-65
// speckit.LLMResponder so a single responder instance can back BOTH
// surfaces without adapter glue.
type LLMResponder interface {
	// Generate sends prompt to the underlying model and returns the
	// raw response text. Implementations MUST honour ctx cancellation
	// and MUST surface upstream errors verbatim (wrapping with %w is
	// acceptable; swallowing is not).
	Generate(ctx context.Context, prompt string) (string, error)
}

// Round-79 sentinels — distinguishable from the round-31
// ErrPlanGenerationLLMNotWired so callers can branch precisely on
// the misconfiguration / input-validation / parse-failure paths.
//
//   - ErrPlanLLMResponderNotConfigured: NewLLMResponderBackedPlanLLM
//     was called with a nil LLMResponder. The constructor itself
//     rejects nil; this sentinel is reserved for any future invariant
//     bypass and for paired-mutation testing.
//   - ErrPlanLLMRequirementsEmpty: GeneratePlanSteps was invoked
//     with an empty / whitespace-only objective. We refuse to ask
//     the model to plan for "nothing" — that path silently fabricates
//     filler.
//   - ErrPlanLLMResponseUnparseable: the LLM responded with text that
//     contained zero `STEP N: ...` markers. Per CONST-035 we refuse
//     to fabricate plan steps from an unparseable response; the
//     honest failure surfaces upstream so the caller can retry,
//     reprompt, or fail loudly.
var (
	ErrPlanLLMResponderNotConfigured = errors.New("planning: LLMResponderBackedPlanLLM constructed (or invoked) with nil LLMResponder — pass a non-nil responder to NewLLMResponderBackedPlanLLM (round-79 §11.4 anti-bluff: refusing to fabricate plan steps without a real model behind the wire)")
	ErrPlanLLMRequirementsEmpty      = errors.New("planning: GeneratePlanSteps invoked with empty / whitespace-only requirements — refusing to ask the LLM to plan for nothing (round-79 §11.4 anti-bluff: an empty objective produces filler 'Analyze requirements'-style fabrication regardless of the model)")
	ErrPlanLLMResponseUnparseable    = errors.New("planning: LLMResponder.Generate returned text with zero parseable STEP markers (expected lines matching `STEP N: <action>`) — refusing to fabricate an empty or synthetic plan from an unparseable response (round-79 §11.4 anti-bluff: CONST-035 — silent fallback to a hardcoded template was the original round-29 bluff this code path replaces)")
)

// PlanLLMOption is a functional option for
// NewLLMResponderBackedPlanLLM.
type PlanLLMOption func(*planLLMConfig)

// planLLMConfig holds the tunable knobs for the responder-backed
// PlanLLM. Defaults work out-of-the-box with any LLMResponder;
// callers tune via the functional options below.
type planLLMConfig struct {
	promptTemplate string
	defaultType    string
}

// WithPlanPromptTemplate replaces the default plan-generation
// prompt template. The template MUST contain the placeholders
// {requirements} (the objective text supplied by the caller),
// {context} (newline-joined context snippets, empty string if
// none), and {max_steps} (the caller-requested step ceiling).
func WithPlanPromptTemplate(template string) PlanLLMOption {
	return func(c *planLLMConfig) {
		if strings.TrimSpace(template) != "" {
			c.promptTemplate = template
		}
	}
}

// WithDefaultStepType sets the PlanStep.Type assigned to parsed
// steps when the model output does not encode an explicit type.
// Defaults to "task".
func WithDefaultStepType(t string) PlanLLMOption {
	return func(c *planLLMConfig) {
		if strings.TrimSpace(t) != "" {
			c.defaultType = t
		}
	}
}

// defaultPlanPromptTemplate produces a structured plan-generation
// prompt. Placeholders are substituted at invocation time. The
// `STEP N:` marker is intentionally simple so a permissive regex
// can parse responses from any provider without JSON-mode coupling.
const defaultPlanPromptTemplate = `You are a software planning assistant. Given these requirements, generate a concrete, ordered, numbered list of implementation steps.

Requirements:
{requirements}

Additional context:
{context}

Constraints:
  - Produce AT MOST {max_steps} steps.
  - Each step MUST start on its own line with the exact prefix "STEP N:" where N is the 1-indexed step number.
  - Each step description MUST be a single concrete action the implementer can take, not a generic phase name.
  - Do NOT emit any other prose, headings, or commentary outside the STEP lines.

Respond now with the numbered STEP lines.`

// stepLineRegex matches a plan-step line emitted by the LLM under
// the default prompt template. Permissive on separators after the
// number ("STEP 1:", "STEP 1.", "STEP 1 - ", "STEP 1 ") because
// different models drift; strict on the leading `STEP N` token so
// stray prose (e.g. "Step in the right direction") is rejected.
var stepLineRegex = regexp.MustCompile(`(?im)^\s*STEP\s+(\d+)\s*[:.\-]?\s+(\S.*?)\s*$`)

// LLMResponderBackedPlanLLM is the concrete PlanLLM implementation
// added in round-79. It adapts any LLMResponder into a PlanLLM by
// rendering a structured prompt, invoking responder.Generate, and
// parsing the response into a []PlanStep.
//
// Safe for concurrent use provided the underlying responder is.
type LLMResponderBackedPlanLLM struct {
	responder LLMResponder
	cfg       planLLMConfig
}

// NewLLMResponderBackedPlanLLM constructs an LLMResponderBackedPlanLLM
// from any LLMResponder. Passing a nil responder returns
// ErrPlanLLMResponderNotConfigured — the constructor refuses to
// produce a half-wired instance that would surface the same failure
// later during a request.
func NewLLMResponderBackedPlanLLM(responder LLMResponder, opts ...PlanLLMOption) (*LLMResponderBackedPlanLLM, error) {
	if responder == nil {
		return nil, ErrPlanLLMResponderNotConfigured
	}
	cfg := planLLMConfig{
		promptTemplate: defaultPlanPromptTemplate,
		defaultType:    "task",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &LLMResponderBackedPlanLLM{
		responder: responder,
		cfg:       cfg,
	}, nil
}

// GeneratePlanSteps satisfies the PlanLLM interface (planning.go).
// It builds a structured prompt from objective + context + maxSteps,
// invokes the wired LLMResponder, and parses the response into a
// []PlanStep. Returns:
//
//   - ErrPlanLLMResponderNotConfigured if the responder is nil
//     (defensive — the constructor rejects nil, so this only fires
//     if a caller built the struct directly with the zero value).
//   - ErrPlanLLMRequirementsEmpty if objective is empty/whitespace.
//   - ctx.Err() (wrapped) if context cancelled before invocation.
//   - The responder's error (wrapped) if Generate failed.
//   - ErrPlanLLMResponseUnparseable if the response contains zero
//     `STEP N:` markers. We do NOT fabricate empty / synthetic
//     steps — that path was the round-29 §11.4 bluff this entire
//     subsystem exists to prevent.
//
// On success the returned slice contains one PlanStep per parsed
// STEP line, in the order the LLM emitted them, capped at maxSteps.
// Step.Number is taken from the model's "STEP N" prefix; Status is
// PlanStepStatusPending; Type is the configured default (overridable
// via WithDefaultStepType).
func (p *LLMResponderBackedPlanLLM) GeneratePlanSteps(
	ctx context.Context,
	objective string,
	context_ []string,
	maxSteps int,
) ([]PlanStep, error) {
	if p == nil || p.responder == nil {
		return nil, ErrPlanLLMResponderNotConfigured
	}

	trimmed := strings.TrimSpace(objective)
	if trimmed == "" {
		return nil, ErrPlanLLMRequirementsEmpty
	}

	// Honour ctx cancellation before issuing any network I/O.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("planning: LLMResponderBackedPlanLLM: context cancelled before LLM invocation: %w", err)
	}

	if maxSteps <= 0 {
		maxSteps = 10
	}

	prompt := renderPlanPrompt(p.cfg.promptTemplate, trimmed, context_, maxSteps)

	raw, err := p.responder.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("planning: LLMResponderBackedPlanLLM: LLMResponder.Generate failed for objective %q: %w", trimmed, err)
	}

	steps := parsePlanSteps(raw, p.cfg.defaultType, maxSteps)
	if len(steps) == 0 {
		return nil, fmt.Errorf("planning: LLMResponderBackedPlanLLM: objective %q: response length=%d: %w", trimmed, len(raw), ErrPlanLLMResponseUnparseable)
	}

	return steps, nil
}

// renderPlanPrompt substitutes the three documented placeholders.
// No templating engine is used — kept dependency-light per
// CONST-051(B) (decoupling) and to mirror HelixSpecifier round-65.
func renderPlanPrompt(tpl, requirements string, context_ []string, maxSteps int) string {
	ctxJoined := strings.TrimSpace(strings.Join(context_, "\n"))
	if ctxJoined == "" {
		ctxJoined = "(none provided)"
	}
	out := strings.ReplaceAll(tpl, "{requirements}", requirements)
	out = strings.ReplaceAll(out, "{context}", ctxJoined)
	out = strings.ReplaceAll(out, "{max_steps}", fmt.Sprintf("%d", maxSteps))
	return out
}

// parsePlanSteps applies stepLineRegex to the raw LLM response and
// extracts PlanStep entries. The function is deliberately strict:
// it refuses to invent steps from prose between STEP markers and
// returns an empty slice when the regex finds nothing — the caller
// (GeneratePlanSteps) translates that into ErrPlanLLMResponseUnparseable.
//
// The maxSteps cap is applied here rather than relying on the model
// to honour the prompt-level constraint — defence-in-depth against
// chatty models.
func parsePlanSteps(raw string, defaultType string, maxSteps int) []PlanStep {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	matches := stepLineRegex.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]PlanStep, 0, len(matches))
	for i, m := range matches {
		if len(m) < 3 {
			continue
		}
		// m[1] = step number from the model, m[2] = description text.
		number := i + 1
		var parsedN int
		if _, err := fmt.Sscanf(m[1], "%d", &parsedN); err == nil && parsedN > 0 {
			number = parsedN
		}
		desc := strings.TrimSpace(m[2])
		if desc == "" {
			// Skip empty-description lines rather than fabricate.
			continue
		}
		out = append(out, PlanStep{
			Number:      number,
			Description: desc,
			Type:        defaultType,
			Status:      PlanStepStatusPending,
		})
		if maxSteps > 0 && len(out) >= maxSteps {
			break
		}
	}
	return out
}
