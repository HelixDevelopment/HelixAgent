package smolagents

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — smolagents honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: Smolagents.run USED to return fabricated fixed steps ("Analyze
// task"/"Plan approach"/"Execute") + "Task completed successfully" WITHOUT
// running anything — a BLUFF-001 / CONST-035 false-success.
//
// FIX: smolagents (huggingface/smolagents) is a Python agent LIBRARY, not a
// headless binary, so run now returns an honest error rather than fabricate a
// run transcript. createAgent/listAgents/importTool are real JSON-persistence
// config ops and remain.
// ---------------------------------------------------------------------------

func TestD17_Smolagents_RunIsHonestError(t *testing.T) {
	s := New()
	ctx := context.Background()
	if err := s.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Create a real agent first (honest persistence path).
	ares, err := s.createAgent(ctx, map[string]interface{}{"name": "a1"})
	if err != nil {
		t.Fatalf("createAgent: %v", err)
	}
	agent := ares.(map[string]interface{})["agent"].(Agent)

	// run must NOT fabricate a transcript.
	res, err := s.run(ctx, map[string]interface{}{"agent_id": agent.ID, "task": "do x"})
	if err == nil {
		t.Fatalf("D-17 BLUFF: run returned success %v — must return an honest error (Python library, no headless CLI), never a fabricated transcript (BLUFF-001).", res)
	}
	if res != nil {
		t.Fatalf("D-17 BLUFF: run returned a result payload %v — must be nil.", res)
	}
}

// TestD17_Smolagents_RunMissingAgentStillErrors keeps the input-validation
// contract: an unknown agent_id is a different (also honest) error.
func TestD17_Smolagents_RunMissingAgentStillErrors(t *testing.T) {
	s := New()
	ctx := context.Background()
	if err := s.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if _, err := s.run(ctx, map[string]interface{}{"agent_id": "nope", "task": "x"}); err == nil {
		t.Fatal("D-17: run with unknown agent_id must error.")
	}
}
