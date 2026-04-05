package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Provenance Tracker Tests
// =============================================================================

func TestNewProvenanceTracker(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	require.NotNil(t, tracker)
	assert.NotNil(t, tracker.entries)
	assert.Empty(t, tracker.entries)
	assert.Equal(t, int64(0), tracker.counter)
}

func TestProvenanceTracker_Record(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	now := time.Now()
	entry := &AuditEntry{
		ID:        "entry-1",
		Timestamp: now,
		EventType: EventPromptSent,
		AgentID:   "agent-a",
		Phase:     "proposal",
		Round:     1,
		Data: map[string]interface{}{
			"model":    "gpt-4",
			"provider": "openai",
		},
	}

	tracker.Record("session-1", entry)

	entries := tracker.GetEntries("session-1")
	require.Len(t, entries, 1)
	assert.Equal(t, "entry-1", entries[0].ID)
	assert.Equal(t, "session-1", entries[0].SessionID)
	assert.Equal(t, EventPromptSent, entries[0].EventType)
	assert.Equal(t, "agent-a", entries[0].AgentID)
	assert.Equal(t, "proposal", entries[0].Phase)
	assert.Equal(t, 1, entries[0].Round)
}

func TestProvenanceTracker_Record_NilEntry(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	tracker.Record("session-1", nil)

	entries := tracker.GetEntries("session-1")
	assert.Nil(t, entries)
}

func TestProvenanceTracker_Record_AutoID(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	e1 := &AuditEntry{EventType: EventPromptSent}
	e2 := &AuditEntry{EventType: EventResponseReceived}
	e3 := &AuditEntry{EventType: EventToolCalled}

	tracker.Record("session-1", e1)
	tracker.Record("session-1", e2)
	tracker.Record("session-1", e3)

	entries := tracker.GetEntries("session-1")
	require.Len(t, entries, 3)

	assert.Equal(t, "audit-1", entries[0].ID)
	assert.Equal(t, "audit-2", entries[1].ID)
	assert.Equal(t, "audit-3", entries[2].ID)
}

func TestProvenanceTracker_Record_AutoTimestamp(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	before := time.Now()
	entry := &AuditEntry{
		ID:        "ts-entry",
		EventType: EventDebateStarted,
	}
	tracker.Record("session-1", entry)
	after := time.Now()

	entries := tracker.GetEntries("session-1")
	require.Len(t, entries, 1)
	assert.False(t, entries[0].Timestamp.IsZero())
	assert.True(t, entries[0].Timestamp.After(before) ||
		entries[0].Timestamp.Equal(before))
	assert.True(t, entries[0].Timestamp.Before(after) ||
		entries[0].Timestamp.Equal(after))
}

func TestProvenanceTracker_Record_SessionIDOverwritten(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	entry := &AuditEntry{
		ID:        "sid-test",
		EventType: EventPromptSent,
		SessionID: "wrong-session",
	}
	tracker.Record("correct-session", entry)

	entries := tracker.GetEntries("correct-session")
	require.Len(t, entries, 1)
	assert.Equal(t, "correct-session", entries[0].SessionID)
}

func TestProvenanceTracker_GetEntries(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	// Add entries with different timestamps out of order.
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)

	tracker.Record("session-1", &AuditEntry{
		ID: "a", EventType: EventPromptSent, Timestamp: t1,
	})
	tracker.Record("session-1", &AuditEntry{
		ID: "b", EventType: EventResponseReceived, Timestamp: t2,
	})
	tracker.Record("session-1", &AuditEntry{
		ID: "c", EventType: EventToolCalled, Timestamp: t3,
	})

	entries := tracker.GetEntries("session-1")
	require.Len(t, entries, 3)

	// Entries should be sorted by timestamp.
	assert.Equal(t, "b", entries[0].ID)
	assert.Equal(t, "a", entries[1].ID)
	assert.Equal(t, "c", entries[2].ID)
}

func TestProvenanceTracker_GetEntries_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	entries := tracker.GetEntries("no-such-session")
	assert.Nil(t, entries)
}

func TestProvenanceTracker_GetEntriesByType(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "2", EventType: EventResponseReceived, Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "3", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "4", EventType: EventErrorOccurred, Timestamp: time.Now(),
	})

	prompts := tracker.GetEntriesByType("s1", EventPromptSent)
	require.Len(t, prompts, 2)
	for _, e := range prompts {
		assert.Equal(t, EventPromptSent, e.EventType)
	}

	errors := tracker.GetEntriesByType("s1", EventErrorOccurred)
	require.Len(t, errors, 1)
	assert.Equal(t, "4", errors[0].ID)
}

func TestProvenanceTracker_GetEntriesByType_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	entries := tracker.GetEntriesByType("no-session", EventPromptSent)
	assert.Nil(t, entries)
}

func TestProvenanceTracker_GetEntriesByType_NoMatches(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: time.Now(),
	})

	entries := tracker.GetEntriesByType("s1", EventErrorOccurred)
	assert.Empty(t, entries)
}

func TestProvenanceTracker_GetEntriesByAgent(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, AgentID: "agent-a",
		Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "2", EventType: EventResponseReceived, AgentID: "agent-b",
		Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "3", EventType: EventToolCalled, AgentID: "agent-a",
		Timestamp: time.Now(),
	})

	agentA := tracker.GetEntriesByAgent("s1", "agent-a")
	require.Len(t, agentA, 2)
	for _, e := range agentA {
		assert.Equal(t, "agent-a", e.AgentID)
	}

	agentB := tracker.GetEntriesByAgent("s1", "agent-b")
	require.Len(t, agentB, 1)
	assert.Equal(t, "2", agentB[0].ID)
}

func TestProvenanceTracker_GetEntriesByAgent_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	entries := tracker.GetEntriesByAgent("no-session", "agent-a")
	assert.Nil(t, entries)
}

func TestProvenanceTracker_GetAuditTrail(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	now := time.Now()
	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventDebateStarted, Timestamp: now,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "2", EventType: EventPromptSent, Timestamp: now.Add(time.Second),
		Data: map[string]interface{}{
			"model": "claude-3", "provider": "anthropic",
		},
	})
	tracker.Record("s1", &AuditEntry{
		ID: "3", EventType: EventResponseReceived,
		Timestamp: now.Add(2 * time.Second),
	})

	trail := tracker.GetAuditTrail("s1")
	require.NotNil(t, trail)
	assert.Equal(t, "s1", trail.SessionID)
	assert.Len(t, trail.Entries, 3)
	require.NotNil(t, trail.Summary)
	assert.Equal(t, "s1", trail.Summary.SessionID)
}

func TestProvenanceTracker_GetAuditTrail_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	trail := tracker.GetAuditTrail("no-session")
	assert.Nil(t, trail)
}

func TestProvenanceTracker_GetSummary(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC)
	t4 := time.Date(2026, 1, 1, 10, 3, 0, 0, time.UTC)
	t5 := time.Date(2026, 1, 1, 10, 4, 0, 0, time.UTC)
	t6 := time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC)
	t7 := time.Date(2026, 1, 1, 10, 6, 0, 0, time.UTC)
	t8 := time.Date(2026, 1, 1, 10, 7, 0, 0, time.UTC)

	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: t1,
		Data: map[string]interface{}{
			"model": "gpt-4", "provider": "openai",
		},
	})
	tracker.Record("s1", &AuditEntry{
		ID: "2", EventType: EventPromptSent, Timestamp: t2,
		Data: map[string]interface{}{
			"model": "claude-3", "provider": "anthropic",
		},
	})
	tracker.Record("s1", &AuditEntry{
		ID: "3", EventType: EventResponseReceived, Timestamp: t3,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "4", EventType: EventToolCalled, Timestamp: t4,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "5", EventType: EventVoteCast, Timestamp: t5,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "6", EventType: EventReflectionGenerated, Timestamp: t6,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "7", EventType: EventErrorOccurred, Timestamp: t7,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "8", EventType: EventPhaseStarted, Phase: "proposal",
		Timestamp: t8,
		Data: map[string]interface{}{
			"phase": "proposal",
		},
	})

	summary := tracker.GetSummary("s1")
	require.NotNil(t, summary)

	assert.Equal(t, "s1", summary.SessionID)
	assert.Equal(t, 2, summary.TotalPrompts)
	assert.Equal(t, 1, summary.TotalResponses)
	assert.Equal(t, 1, summary.TotalToolCalls)
	assert.Equal(t, 1, summary.TotalVotes)
	assert.Equal(t, 1, summary.TotalReflections)
	assert.Equal(t, 1, summary.TotalErrors)

	assert.Contains(t, summary.ModelsUsed, "gpt-4")
	assert.Contains(t, summary.ModelsUsed, "claude-3")
	assert.Contains(t, summary.ProvidersUsed, "openai")
	assert.Contains(t, summary.ProvidersUsed, "anthropic")
	assert.Contains(t, summary.PhasesExecuted, "proposal")

	assert.Equal(t, t1, summary.StartTime)
	assert.Equal(t, t8, summary.EndTime)
	assert.Equal(t, t8.Sub(t1), summary.Duration)
}

func TestProvenanceTracker_GetSummary_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	summary := tracker.GetSummary("no-session")
	assert.Nil(t, summary)
}

func TestProvenanceTracker_GetSummary_EmptyData(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	now := time.Now()
	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: now,
	})

	summary := tracker.GetSummary("s1")
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.TotalPrompts)
	assert.Empty(t, summary.ModelsUsed)
	assert.Empty(t, summary.ProvidersUsed)
}

func TestProvenanceTracker_MarshalSessionJSON(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	now := time.Now()
	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventDebateStarted, Timestamp: now,
	})
	tracker.Record("s1", &AuditEntry{
		ID: "2", EventType: EventPromptSent,
		Timestamp: now.Add(time.Second),
		Data: map[string]interface{}{
			"model": "gpt-4", "provider": "openai",
		},
	})

	data, err := tracker.MarshalSessionJSON("s1")
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it is valid JSON.
	var trail AuditTrail
	err = json.Unmarshal(data, &trail)
	require.NoError(t, err)
	assert.Equal(t, "s1", trail.SessionID)
	assert.Len(t, trail.Entries, 2)
	require.NotNil(t, trail.Summary)
}

func TestProvenanceTracker_MarshalSessionJSON_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	data, err := tracker.MarshalSessionJSON("no-session")
	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "not found")
}

func TestProvenanceTracker_Clear(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("s2", &AuditEntry{
		ID: "2", EventType: EventPromptSent, Timestamp: time.Now(),
	})

	// Verify both sessions exist.
	require.NotNil(t, tracker.GetEntries("s1"))
	require.NotNil(t, tracker.GetEntries("s2"))

	// Clear s1.
	tracker.Clear("s1")

	assert.Nil(t, tracker.GetEntries("s1"))
	assert.NotNil(t, tracker.GetEntries("s2"))
}

func TestProvenanceTracker_Clear_NonexistentSession(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	// Should not panic or error.
	tracker.Clear("no-session")

	ids := tracker.GetSessionIDs()
	assert.Empty(t, ids)
}

func TestProvenanceTracker_GetSessionIDs(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	tracker.Record("beta", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("alpha", &AuditEntry{
		ID: "2", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("gamma", &AuditEntry{
		ID: "3", EventType: EventPromptSent, Timestamp: time.Now(),
	})

	ids := tracker.GetSessionIDs()
	require.Len(t, ids, 3)

	// IDs should be sorted.
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, ids)
}

func TestProvenanceTracker_GetSessionIDs_Empty(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()
	ids := tracker.GetSessionIDs()
	assert.Empty(t, ids)
}

func TestProvenanceTracker_MultipleSessions(t *testing.T) {
	t.Parallel()
	tracker := NewProvenanceTracker()

	tracker.Record("s1", &AuditEntry{
		ID: "1", EventType: EventPromptSent, Timestamp: time.Now(),
	})
	tracker.Record("s2", &AuditEntry{
		ID: "2", EventType: EventToolCalled, Timestamp: time.Now(),
	})
	tracker.Record("s1", &AuditEntry{
		ID: "3", EventType: EventResponseReceived, Timestamp: time.Now(),
	})

	s1Entries := tracker.GetEntries("s1")
	require.Len(t, s1Entries, 2)

	s2Entries := tracker.GetEntries("s2")
	require.Len(t, s2Entries, 1)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestSortedKeys(t *testing.T) {
	t.Parallel()
	m := map[string]struct{}{
		"cherry": {},
		"apple":  {},
		"banana": {},
	}
	keys := sortedKeys(m)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, keys)
}

func TestSortedKeys_Empty(t *testing.T) {
	t.Parallel()
	m := map[string]struct{}{}
	keys := sortedKeys(m)
	assert.Equal(t, []string{}, keys)
}

func TestExtractStringField(t *testing.T) {
	t.Parallel()
	target := make(map[string]struct{})

	// Nil data should not panic.
	extractStringField(nil, "key", target)
	assert.Empty(t, target)

	// Missing key.
	data := map[string]interface{}{"other": "value"}
	extractStringField(data, "key", target)
	assert.Empty(t, target)

	// Present key with string value.
	data = map[string]interface{}{"key": "hello"}
	extractStringField(data, "key", target)
	assert.Contains(t, target, "hello")

	// Present key with empty string (should not be added).
	target2 := make(map[string]struct{})
	data = map[string]interface{}{"key": ""}
	extractStringField(data, "key", target2)
	assert.Empty(t, target2)

	// Present key with non-string value.
	target3 := make(map[string]struct{})
	data = map[string]interface{}{"key": 123}
	extractStringField(data, "key", target3)
	assert.Empty(t, target3)
}
