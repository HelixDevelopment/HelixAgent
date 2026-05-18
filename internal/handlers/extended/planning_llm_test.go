package extended

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLMResponder is a CONST-050(A)-compliant unit-test fake for
// the LLMResponder interface. It returns a canned response (or
// canned error) and captures the prompt the production code sent
// it so paired-mutation assertions can confirm the prompt template
// actually included the caller's requirements / context / maxSteps.
//
// This stub is allowed under CONST-050(A) because it lives in a
// `_test.go` file invoked without an integration build tag.
// Production code (planning_llm.go) never imports it.
type stubLLMResponder struct {
	resp           string
	err            error
	capturedPrompt string
	calls          int
}

func (s *stubLLMResponder) Generate(_ context.Context, prompt string) (string, error) {
	s.calls++
	s.capturedPrompt = prompt
	if s.err != nil {
		return "", s.err
	}
	return s.resp, nil
}

// ctxAwareResponder respects context cancellation — used to prove
// GeneratePlanSteps honours ctx via responder pass-through and via
// its own pre-flight ctx check.
type ctxAwareResponder struct{}

func (ctxAwareResponder) Generate(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

// -----------------------------------------------------------------
// Constructor / configuration sentinels
// -----------------------------------------------------------------

func TestNewLLMResponderBackedPlanLLM_NilResponder_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	llm, err := NewLLMResponderBackedPlanLLM(nil)
	require.Nil(t, llm, "constructor must refuse to produce a half-wired instance")
	require.ErrorIs(t, err, ErrPlanLLMResponderNotConfigured, "constructor must return the round-79 sentinel exactly so callers can branch on it")
}

func TestNewLLMResponderBackedPlanLLM_DefaultsApplied(t *testing.T) {
	t.Parallel()
	r := &stubLLMResponder{resp: "STEP 1: do something concrete"}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)
	require.NotNil(t, llm)
	steps, err := llm.GeneratePlanSteps(context.Background(), "build a thing", nil, 5)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "task", steps[0].Type, "default step type must be 'task' when no override is configured")
}

func TestNewLLMResponderBackedPlanLLM_WithOptions(t *testing.T) {
	t.Parallel()
	r := &stubLLMResponder{resp: "STEP 1: implement caching layer"}
	llm, err := NewLLMResponderBackedPlanLLM(r,
		WithDefaultStepType("implement"),
		WithPlanPromptTemplate("CUSTOM: {requirements}\nMAX={max_steps}\nCTX={context}"),
	)
	require.NoError(t, err)
	steps, err := llm.GeneratePlanSteps(context.Background(), "ship feature X", []string{"existing api"}, 3)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "implement", steps[0].Type, "WithDefaultStepType must override the default")
	assert.Contains(t, r.capturedPrompt, "CUSTOM: ship feature X")
	assert.Contains(t, r.capturedPrompt, "MAX=3")
	assert.Contains(t, r.capturedPrompt, "CTX=existing api")
}

// -----------------------------------------------------------------
// Input-validation sentinels
// -----------------------------------------------------------------

func TestGeneratePlanSteps_EmptyRequirements_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	cases := []string{"", "   ", "\t\n  "}
	for _, in := range cases {
		in := in
		t.Run("input="+in, func(t *testing.T) {
			t.Parallel()
			r := &stubLLMResponder{resp: "STEP 1: should never run"}
			llm, err := NewLLMResponderBackedPlanLLM(r)
			require.NoError(t, err)
			steps, err := llm.GeneratePlanSteps(context.Background(), in, nil, 3)
			require.Nil(t, steps)
			require.ErrorIs(t, err, ErrPlanLLMRequirementsEmpty, "empty/whitespace requirements must surface the round-79 sentinel — never fabricate filler steps")
			assert.Equal(t, 0, r.calls, "responder must NOT be invoked when requirements are empty — wasting a real LLM round-trip is a §11.4.6 no-guessing violation")
		})
	}
}

// -----------------------------------------------------------------
// Happy path — real parsing, real PlanStep semantics
// -----------------------------------------------------------------

func TestGeneratePlanSteps_HappyPath_ParsesSteps(t *testing.T) {
	t.Parallel()
	canned := strings.Join([]string{
		"STEP 1: Add Redis cache adapter to internal/cache",
		"STEP 2: Wire adapter into request handler middleware",
		"STEP 3: Add integration test covering cache hit path",
	}, "\n")
	r := &stubLLMResponder{resp: canned}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)

	steps, err := llm.GeneratePlanSteps(context.Background(), "add request caching", []string{"http handlers"}, 10)
	require.NoError(t, err)
	require.Len(t, steps, 3)

	assert.Equal(t, 1, steps[0].Number)
	assert.Equal(t, "Add Redis cache adapter to internal/cache", steps[0].Description)
	assert.Equal(t, PlanStepStatusPending, steps[0].Status)
	assert.Equal(t, "task", steps[0].Type)

	assert.Equal(t, 2, steps[1].Number)
	assert.Equal(t, "Wire adapter into request handler middleware", steps[1].Description)

	assert.Equal(t, 3, steps[2].Number)
	assert.Equal(t, "Add integration test covering cache hit path", steps[2].Description)

	// Prompt must include the real requirements / context — proves
	// the dispatch wired the caller's input through, not a stale
	// template constant.
	assert.Contains(t, r.capturedPrompt, "add request caching")
	assert.Contains(t, r.capturedPrompt, "http handlers")
}

func TestGeneratePlanSteps_PermissiveSeparators(t *testing.T) {
	t.Parallel()
	// Different models drift on the separator after the number;
	// parser MUST accept ":", ".", "-", and bare-space variants.
	canned := strings.Join([]string{
		"STEP 1: colon form",
		"STEP 2. period form",
		"STEP 3 - dash form",
		"STEP 4 bare space form",
	}, "\n")
	r := &stubLLMResponder{resp: canned}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)
	steps, err := llm.GeneratePlanSteps(context.Background(), "drift-tolerance proof", nil, 10)
	require.NoError(t, err)
	require.Len(t, steps, 4)
	assert.Equal(t, "colon form", steps[0].Description)
	assert.Equal(t, "period form", steps[1].Description)
	assert.Equal(t, "dash form", steps[2].Description)
	assert.Equal(t, "bare space form", steps[3].Description)
}

func TestGeneratePlanSteps_MaxStepsCap(t *testing.T) {
	t.Parallel()
	// Model emits more steps than requested — parser MUST cap.
	canned := strings.Join([]string{
		"STEP 1: a",
		"STEP 2: b",
		"STEP 3: c",
		"STEP 4: d",
		"STEP 5: e",
	}, "\n")
	r := &stubLLMResponder{resp: canned}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)
	steps, err := llm.GeneratePlanSteps(context.Background(), "cap test", nil, 2)
	require.NoError(t, err)
	require.Len(t, steps, 2, "parser MUST cap at maxSteps even when model over-produces")
}

// -----------------------------------------------------------------
// Anti-bluff parser invariant — refuses to fabricate
// -----------------------------------------------------------------

func TestGeneratePlanSteps_UnparseableResponse_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"plain prose":          "I think a good plan would be to start by analysing the codebase carefully and then make some changes.",
		"json blob no markers": `{"plan": ["analyze", "design", "implement"]}`,
		"step keyword misuse":  "It would be a step in the right direction to refactor first.",
		"empty string":         "",
		"only whitespace":      "   \n\t  \n",
	}
	for name, canned := range cases {
		canned := canned
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := &stubLLMResponder{resp: canned}
			llm, err := NewLLMResponderBackedPlanLLM(r)
			require.NoError(t, err)
			steps, err := llm.GeneratePlanSteps(context.Background(), "valid objective", nil, 5)
			require.Nil(t, steps, "unparseable response MUST NOT yield a fabricated plan slice")
			require.ErrorIs(t, err, ErrPlanLLMResponseUnparseable, "round-79 anti-bluff: unparseable LLM output MUST surface the sentinel, not silently fall through to an empty success")
		})
	}
}

// -----------------------------------------------------------------
// Responder error / context propagation
// -----------------------------------------------------------------

func TestGeneratePlanSteps_ResponderError_PropagatesAsError(t *testing.T) {
	t.Parallel()
	upstream := errors.New("LLM 503")
	r := &stubLLMResponder{err: upstream}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)
	steps, err := llm.GeneratePlanSteps(context.Background(), "obj", nil, 3)
	require.Nil(t, steps)
	require.ErrorIs(t, err, upstream, "responder errors MUST propagate verbatim (wrapped is fine; swallowed is a §11.4 bluff)")
}

func TestGeneratePlanSteps_HonoursContextCancel(t *testing.T) {
	t.Parallel()
	llm, err := NewLLMResponderBackedPlanLLM(ctxAwareResponder{})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	steps, err := llm.GeneratePlanSteps(ctx, "obj", nil, 3)
	require.Nil(t, steps)
	require.Error(t, err, "context cancellation MUST surface as an error")
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled), "wrapped error MUST chain back to context.DeadlineExceeded/Canceled")
}

func TestGeneratePlanSteps_PreflightCtxCheck(t *testing.T) {
	t.Parallel()
	r := &stubLLMResponder{resp: "STEP 1: never reached"}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	steps, err := llm.GeneratePlanSteps(ctx, "obj", nil, 3)
	require.Nil(t, steps)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "pre-flight ctx check must short-circuit before responder is invoked")
	assert.Equal(t, 0, r.calls, "pre-flight ctx-cancelled path MUST NOT consume an LLM round-trip")
}

// -----------------------------------------------------------------
// Paired-mutation: different inputs MUST yield different outputs
// -----------------------------------------------------------------

func TestGeneratePlanSteps_NotFabricated(t *testing.T) {
	t.Parallel()
	// Same llm instance, two different LLM responses — produced
	// PlanStep slices MUST diverge. This is the round-79 forensic
	// anchor: if a future change reintroduces a hardcoded template,
	// THIS test (which feeds two different "model" responses through
	// the same dispatch path) will detect it because the produced
	// slices will become identical regardless of the canned input.
	rA := &stubLLMResponder{resp: "STEP 1: alpha-action-A\nSTEP 2: alpha-action-B"}
	rB := &stubLLMResponder{resp: "STEP 1: beta-action-X\nSTEP 2: beta-action-Y\nSTEP 3: beta-action-Z"}

	llmA, err := NewLLMResponderBackedPlanLLM(rA)
	require.NoError(t, err)
	llmB, err := NewLLMResponderBackedPlanLLM(rB)
	require.NoError(t, err)

	stepsA, err := llmA.GeneratePlanSteps(context.Background(), "same objective", nil, 10)
	require.NoError(t, err)
	stepsB, err := llmB.GeneratePlanSteps(context.Background(), "same objective", nil, 10)
	require.NoError(t, err)

	require.Len(t, stepsA, 2)
	require.Len(t, stepsB, 3)
	assert.NotEqual(t, stepsA[0].Description, stepsB[0].Description, "two distinct LLM responses MUST produce two distinct first steps — identical output would prove the parser fabricates regardless of input (round-29 bluff regression)")
	assert.Equal(t, "alpha-action-A", stepsA[0].Description)
	assert.Equal(t, "beta-action-X", stepsB[0].Description)
}

func TestSentinelsAreDistinct(t *testing.T) {
	t.Parallel()
	// Paired-mutation guard: every round-79 sentinel must be
	// distinguishable from every other sentinel via errors.Is,
	// AND distinguishable from the round-31 sentinel they layer
	// alongside. Collapsing two sentinels into one would make
	// caller branch logic silently lie.
	pairs := []struct {
		a, b error
	}{
		{ErrPlanLLMResponderNotConfigured, ErrPlanLLMRequirementsEmpty},
		{ErrPlanLLMResponderNotConfigured, ErrPlanLLMResponseUnparseable},
		{ErrPlanLLMRequirementsEmpty, ErrPlanLLMResponseUnparseable},
		{ErrPlanLLMResponderNotConfigured, ErrPlanGenerationLLMNotWired},
		{ErrPlanLLMRequirementsEmpty, ErrPlanGenerationLLMNotWired},
		{ErrPlanLLMResponseUnparseable, ErrPlanGenerationLLMNotWired},
	}
	for _, p := range pairs {
		assert.False(t, errors.Is(p.a, p.b), "sentinels %v and %v MUST be distinct via errors.Is", p.a, p.b)
		assert.False(t, errors.Is(p.b, p.a), "sentinels %v and %v MUST be distinct via errors.Is (reverse)", p.a, p.b)
	}
}

// -----------------------------------------------------------------
// End-to-end: wire into PlanningHandlerExtensions via SetPlanLLM
// and exercise the real generatePlanSteps dispatch path
// -----------------------------------------------------------------

func TestEndToEnd_WireIntoHandlerAndDispatch(t *testing.T) {
	t.Parallel()
	canned := strings.Join([]string{
		"STEP 1: Add planning round-79 unit tests",
		"STEP 2: Wire LLMResponderBackedPlanLLM in main()",
		"STEP 3: Document the LLMResponder injection point",
	}, "\n")
	r := &stubLLMResponder{resp: canned}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)

	h := NewPlanningHandlerExtensions(logrus.New())
	h.SetPlanLLM(llm)

	steps, err := h.generatePlanSteps(context.Background(), "round-79 closure", []string{"helix_agent/internal/handlers/extended"}, 5)
	require.NoError(t, err, "wired LLMResponderBackedPlanLLM must dispatch through PlanningHandlerExtensions.generatePlanSteps cleanly")
	require.Len(t, steps, 3, "all three canned STEP lines must be parsed and surfaced through the handler boundary")

	// generatePlanSteps normalises IDs / numbers — proves the
	// concrete LLM output flowed through the existing
	// normalisation pipeline rather than bypassing it.
	for i, s := range steps {
		assert.NotEmpty(t, s.ID, "step %d ID must be normalised by generatePlanSteps", i)
		assert.NotZero(t, s.Number, "step %d Number must be normalised by generatePlanSteps", i)
		assert.Equal(t, PlanStepStatusPending, s.Status, "step %d Status must be normalised to Pending", i)
	}

	// Per-step content sanity — proves the canned responder text
	// (not a hardcoded template) is what surfaced.
	assert.Contains(t, steps[0].Description, "Add planning round-79 unit tests")
	assert.Contains(t, steps[1].Description, "Wire LLMResponderBackedPlanLLM")
	assert.Contains(t, steps[2].Description, "Document the LLMResponder injection point")
}

func TestEndToEnd_UnparseableLLMSurfacesAsHandlerError(t *testing.T) {
	t.Parallel()
	// Wired concrete impl + unparseable LLM response MUST flow as
	// an error through the handler boundary — NOT silently mask
	// as an empty success slice.
	r := &stubLLMResponder{resp: "I am not going to follow your format and will write a paragraph instead."}
	llm, err := NewLLMResponderBackedPlanLLM(r)
	require.NoError(t, err)

	h := NewPlanningHandlerExtensions(logrus.New())
	h.SetPlanLLM(llm)

	steps, err := h.generatePlanSteps(context.Background(), "obj", nil, 3)
	require.Nil(t, steps)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPlanLLMResponseUnparseable, "unparseable response from a wired LLMResponderBackedPlanLLM MUST surface as ErrPlanLLMResponseUnparseable through the handler boundary — silent empty success would be a §11.4 bluff regression")
}
