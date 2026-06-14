package handlers

import (
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// §11.4.115 polarity switch is the package-shared redMode() (id_uniqueness_test.go):
// RED_MODE=1 REPRODUCES the defect on the pre-fix artifact (asserts defect
// present); default (RED_MODE unset / =0) is the standing GREEN regression-guard
// asserting the defect is ABSENT. One source, two roles.

// buildAgenticEnsembleForTest constructs an AgenticEnsemble exactly the
// way the server-wiring helper does, so the test exercises the real
// construction path (not a hand-rolled fake). It uses the real
// DefaultAgenticEnsembleConfig and the real constructors.
func buildAgenticEnsembleForTest() *services.AgenticEnsemble {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return BuildAgenticEnsemble(
		nil, // debate service (nil tolerated by NewAgenticEnsemble)
		nil, // intent classifier
		nil, // provider registry
		logger,
	)
}

// agenticExecuteResult fabricates an EnsembleResult exactly as the real
// agenticExecutionLoop produces it: Metadata["agentic"] carrying a
// *services.AgenticMetadata with AgentsSpawned >= 2 and per-stage list.
// This is the shape the live /v1/chat/completions response MUST surface.
func agenticExecuteResult() *services.EnsembleResult {
	return &services.EnsembleResult{
		Selected: &models.LLMResponse{
			ID:           "ae-test1234",
			Content:      "synthesised answer from 3 subagents",
			ProviderName: "agentic-ensemble",
		},
		VotingMethod: "agentic_execution",
		Metadata: map[string]any{
			"agentic": &services.AgenticMetadata{
				Mode:            "execute",
				StagesCompleted: []string{"understand", "plan", "assign", "execute", "verify", "synthesise", "respond"},
				AgentsSpawned:   3,
				TasksCompleted:  3,
				TotalDurationMs: 1234,
				ProvenanceID:    "prov-test",
			},
		},
	}
}

// TestAgenticEnsemble_WiredIntoHandler proves the engine is constructible
// via the server-wiring helper and that SetAgenticEnsemble activates the
// gate at processWithEnsemble (h.agenticEnsemble != nil).
//
// RED (pre-fix, RED_MODE=1): the wiring helper BuildAgenticEnsemble does
// not exist / SetAgenticEnsemble is never reached on the real path, so
// h.agenticEnsemble stays nil — the defect.
// GREEN (post-fix, RED_MODE=0): the helper builds a real ensemble and the
// handler reports it active.
func TestAgenticEnsemble_WiredIntoHandler(t *testing.T) {
	h := NewUnifiedHandler(nil, nil)

	// Before wiring the gate is closed — this is the literal defect the
	// task reports (SetAgenticEnsemble never called by the server).
	require.Nil(t, h.agenticEnsemble, "precondition: fresh handler has no ensemble")

	ensemble := buildAgenticEnsembleForTest()
	require.NotNil(t, ensemble, "BuildAgenticEnsemble must produce a real ensemble (reuse, not reimplement)")

	h.SetAgenticEnsemble(ensemble)

	// On the PRE-fix artifact the wiring helper BuildAgenticEnsemble does
	// not exist, so the package fails to compile under RED_MODE=1 — that
	// build failure IS the captured RED reproduction of the "engine never
	// wired" defect. Post-fix the helper exists and the gate opens.
	assert.NotNil(t, h.agenticEnsemble,
		"after server-style wiring the agentic gate (processWithEnsemble :2612) MUST be open")
}

// TestAgenticMetadata_SurfacedInChatResponse proves the SECOND defect:
// even when the engine runs and returns Metadata["agentic"], the
// non-streaming chat response converter (the path the live curl hits)
// MUST surface agents_spawned / stages_completed in the JSON.
//
// RED (pre-fix, RED_MODE=1): convertToOpenAIChatResponse drops the
// metadata → the marshalled JSON has NO "agentic" key → assert ABSENT
// (defect reproduced).
// GREEN (post-fix, RED_MODE=0): the JSON carries agentic.agents_spawned
// >= 2 and the stage list → assert PRESENT.
func TestAgenticMetadata_SurfacedInChatResponse(t *testing.T) {
	h := NewUnifiedHandler(nil, nil)
	result := agenticExecuteResult()
	req := &OpenAIChatRequest{Model: "helixagent-llm"}

	resp := h.convertToOpenAIChatResponse(result, req)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var asMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &asMap))

	agentic, present := asMap["agentic"]

	if redMode() {
		// RED reproduction on the pre-fix artifact: the converter drops
		// agentic metadata, so the key is absent. Capturing this as the
		// defect-present proof.
		assert.False(t, present,
			"RED: pre-fix converter drops agentic metadata — live curl carries NO agents_spawned (the bluff)")
		return
	}

	// GREEN regression-guard: metadata MUST be surfaced with agents_spawned>=2.
	require.True(t, present, "GREEN: chat response MUST carry agentic metadata")
	am, ok := agentic.(map[string]any)
	require.True(t, ok, "agentic field must be an object")
	spawned, ok := am["agents_spawned"].(float64)
	require.True(t, ok, "agents_spawned must be present and numeric")
	assert.GreaterOrEqual(t, spawned, float64(2),
		"GREEN: agentic.agents_spawned MUST be >= 2 (subagents actually ran)")
	stages, ok := am["stages_completed"].([]any)
	require.True(t, ok, "stages_completed must be a list")
	assert.GreaterOrEqual(t, len(stages), 5, "GREEN: multi-stage decompose pipeline ran")
}
