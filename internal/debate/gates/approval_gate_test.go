package gates

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/debate/topology"
)

func TestNewApprovalGate_DefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultGateConfig()

	assert.False(t, cfg.Enabled)
	assert.Nil(t, cfg.GatePoints)
	assert.Equal(t, 30*time.Minute, cfg.Timeout)
	assert.Nil(t, cfg.NotificationChannels)

	gate := NewApprovalGate(cfg)
	require.NotNil(t, gate)
	assert.False(t, gate.IsEnabled())
	assert.Empty(t, gate.GetPendingRequests("any"))
}

func TestApprovalGate_CheckGate_Disabled(t *testing.T) {
	t.Parallel()
	// Gates disabled: should auto-approve regardless of phase.
	gate := NewApprovalGate(GateConfig{
		Enabled:    false,
		GatePoints: []topology.DebatePhase{topology.PhaseProposal},
		Timeout:    time.Second,
	})

	decision, err := gate.CheckGate(
		context.Background(),
		"debate-1", "session-1",
		topology.PhaseProposal,
		"test summary",
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Equal(t, GateStatusApproved, decision.Decision)
	assert.Equal(t, "auto", decision.Reviewer)
	assert.Equal(t, "gate not enabled for this phase", decision.Reason)
	assert.Empty(t, decision.RequestID)
}

func TestApprovalGate_CheckGate_Disabled_PhaseNotInGatePoints(t *testing.T) {
	t.Parallel()
	// Gates enabled but phase not in gate points: should auto-approve.
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseCritique},
		Timeout:    time.Second,
	})

	decision, err := gate.CheckGate(
		context.Background(),
		"debate-1", "session-1",
		topology.PhaseProposal,
		"test summary",
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Equal(t, GateStatusApproved, decision.Decision)
	assert.Equal(t, "auto", decision.Reviewer)
}

func TestApprovalGate_CheckGate_Enabled(t *testing.T) {
	t.Parallel()
	// Gates enabled with matching phase: blocks and creates a pending request.
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseProposal},
		Timeout:    5 * time.Second,
	})

	// Run CheckGate in a goroutine since it blocks.
	type result struct {
		decision *GateDecision
		err      error
	}
	ch := make(chan result, 1)

	go func() {
		d, err := gate.CheckGate(
			context.Background(),
			"debate-1", "session-1",
			topology.PhaseProposal,
			"needs review",
			map[string]interface{}{"key": "value"},
		)
		ch <- result{d, err}
	}()

	// Allow the goroutine to create the request.
	time.Sleep(100 * time.Millisecond)

	// Verify pending request was created.
	pending := gate.GetPendingRequests("debate-1")
	require.Len(t, pending, 1)
	assert.Equal(t, "debate-1", pending[0].DebateID)
	assert.Equal(t, "session-1", pending[0].SessionID)
	assert.Equal(t, topology.PhaseProposal, pending[0].Phase)
	assert.Equal(t, "needs review", pending[0].Summary)
	assert.Equal(t, GateStatusPending, pending[0].Status)

	// Approve it to unblock.
	err := gate.Approve(pending[0].ID, "reviewer-1", "looks good")
	require.NoError(t, err)

	res := <-ch
	require.NoError(t, res.err)
	require.NotNil(t, res.decision)
	assert.Equal(t, GateStatusApproved, res.decision.Decision)
	assert.Equal(t, "reviewer-1", res.decision.Reviewer)
	assert.Equal(t, "looks good", res.decision.Reason)
}

func TestApprovalGate_Approve(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseCritique},
		Timeout:    5 * time.Second,
	})

	type result struct {
		decision *GateDecision
		err      error
	}
	ch := make(chan result, 1)

	go func() {
		d, err := gate.CheckGate(
			context.Background(),
			"debate-2", "session-2",
			topology.PhaseCritique,
			"critique phase",
			nil,
		)
		ch <- result{d, err}
	}()

	time.Sleep(100 * time.Millisecond)

	pending := gate.GetPendingRequests("debate-2")
	require.Len(t, pending, 1)

	// Approve the pending request.
	err := gate.Approve(pending[0].ID, "admin", "approved by admin")
	require.NoError(t, err)

	res := <-ch
	require.NoError(t, res.err)
	require.NotNil(t, res.decision)
	assert.Equal(t, GateStatusApproved, res.decision.Decision)
	assert.Equal(t, "admin", res.decision.Reviewer)

	// Request status should be updated.
	req, found := gate.GetRequest(pending[0].ID)
	require.True(t, found)
	assert.Equal(t, GateStatusApproved, req.Status)

	// Approving again should fail (not pending).
	err = gate.Approve(pending[0].ID, "admin", "double approve")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestApprovalGate_Approve_NotFound(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(DefaultGateConfig())

	err := gate.Approve("nonexistent-id", "reviewer", "reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApprovalGate_Reject(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseReview},
		Timeout:    5 * time.Second,
	})

	type result struct {
		decision *GateDecision
		err      error
	}
	ch := make(chan result, 1)

	go func() {
		d, err := gate.CheckGate(
			context.Background(),
			"debate-3", "session-3",
			topology.PhaseReview,
			"review phase",
			nil,
		)
		ch <- result{d, err}
	}()

	time.Sleep(100 * time.Millisecond)

	pending := gate.GetPendingRequests("debate-3")
	require.Len(t, pending, 1)

	// Reject the pending request.
	err := gate.Reject(pending[0].ID, "admin", "does not meet standards")
	require.NoError(t, err)

	res := <-ch
	require.NoError(t, res.err)
	require.NotNil(t, res.decision)
	assert.Equal(t, GateStatusRejected, res.decision.Decision)
	assert.Equal(t, "admin", res.decision.Reviewer)
	assert.Equal(t, "does not meet standards", res.decision.Reason)

	// Request status should be updated.
	req, found := gate.GetRequest(pending[0].ID)
	require.True(t, found)
	assert.Equal(t, GateStatusRejected, req.Status)
}

func TestApprovalGate_Reject_NotFound(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(DefaultGateConfig())

	err := gate.Reject("nonexistent-id", "reviewer", "reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApprovalGate_GetPendingRequests(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(GateConfig{
		Enabled: true,
		GatePoints: []topology.DebatePhase{
			topology.PhaseProposal,
			topology.PhaseCritique,
		},
		Timeout: 5 * time.Second,
	})

	// Launch two requests for the same debate.
	for _, phase := range []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
	} {
		go func(p topology.DebatePhase) {
			_, _ = gate.CheckGate(
				context.Background(),
				"debate-4", "session-4",
				p,
				"summary for "+string(p),
				nil,
			)
		}(phase)
	}

	time.Sleep(200 * time.Millisecond)

	pending := gate.GetPendingRequests("debate-4")
	assert.Len(t, pending, 2)

	// Different debate should return nothing.
	other := gate.GetPendingRequests("debate-other")
	assert.Empty(t, other)

	// Approve one, now only 1 pending remains.
	err := gate.Approve(pending[0].ID, "r", "ok")
	require.NoError(t, err)

	remaining := gate.GetPendingRequests("debate-4")
	assert.Len(t, remaining, 1)
}

func TestApprovalGate_IsEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			gate := NewApprovalGate(GateConfig{Enabled: tc.enabled})
			assert.Equal(t, tc.expected, gate.IsEnabled())
		})
	}
}

func TestApprovalGate_CheckGate_ContextCancelled(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseProposal},
		Timeout:    30 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())

	type result struct {
		decision *GateDecision
		err      error
	}
	ch := make(chan result, 1)

	go func() {
		d, err := gate.CheckGate(
			ctx,
			"debate-ctx", "session-ctx",
			topology.PhaseProposal,
			"will be cancelled",
			nil,
		)
		ch <- result{d, err}
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	res := <-ch
	assert.Nil(t, res.decision)
	assert.Error(t, res.err)
	assert.Contains(t, res.err.Error(), "cancelled")
}

func TestApprovalGate_CheckGate_Timeout(t *testing.T) {
	t.Parallel()
	gate := NewApprovalGate(GateConfig{
		Enabled:    true,
		GatePoints: []topology.DebatePhase{topology.PhaseProposal},
		Timeout:    200 * time.Millisecond,
	})

	decision, err := gate.CheckGate(
		context.Background(),
		"debate-to", "session-to",
		topology.PhaseProposal,
		"will timeout",
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.Equal(t, GateStatusTimedOut, decision.Decision)
	assert.Contains(t, decision.Reason, "timed out")
}
