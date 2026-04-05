package reflexion

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEpisodicMemoryBuffer_DefaultSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		inputSize   int
		expectedCap int
	}{
		{"zero defaults to 1000", 0, 1000},
		{"negative defaults to 1000", -5, 1000},
		{"positive stays as given", 50, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			buf := NewEpisodicMemoryBuffer(tc.inputSize)
			require.NotNil(t, buf)
			assert.Equal(t, tc.expectedCap, buf.maxSize)
			assert.Equal(t, 0, buf.Size())
		})
	}
}

func TestEpisodicMemoryBuffer_Store(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	ep := &Episode{
		ID:              "ep-1",
		AgentID:         "agent-1",
		SessionID:       "session-1",
		TaskDescription: "implement sorting",
		AttemptNumber:   1,
		Code:            "func sort() {}",
		Confidence:      0.8,
	}

	err := buf.Store(ep)
	require.NoError(t, err)
	assert.Equal(t, 1, buf.Size())

	// Verify the episode is retrievable.
	all := buf.GetAll()
	require.Len(t, all, 1)
	assert.Equal(t, "ep-1", all[0].ID)
	assert.Equal(t, "agent-1", all[0].AgentID)
	assert.False(t, all[0].Timestamp.IsZero(), "timestamp should be auto-set")
}

func TestEpisodicMemoryBuffer_Store_AutoGenerateID(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	ep := &Episode{
		AgentID: "agent-1",
	}

	err := buf.Store(ep)
	require.NoError(t, err)
	assert.NotEmpty(t, ep.ID, "ID should be auto-generated")
	assert.Contains(t, ep.ID, "ep-")
}

func TestEpisodicMemoryBuffer_Store_Validation(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	tests := []struct {
		name    string
		episode *Episode
		errMsg  string
	}{
		{
			name:    "nil episode",
			episode: nil,
			errMsg:  "must not be nil",
		},
		{
			name:    "empty agent ID",
			episode: &Episode{ID: "ep-1", AgentID: ""},
			errMsg:  "AgentID must not be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			err := buf.Store(tc.episode)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestEpisodicMemoryBuffer_GetByAgent(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	episodes := []*Episode{
		{ID: "ep-1", AgentID: "agent-a", SessionID: "s1"},
		{ID: "ep-2", AgentID: "agent-b", SessionID: "s1"},
		{ID: "ep-3", AgentID: "agent-a", SessionID: "s2"},
		{ID: "ep-4", AgentID: "agent-c", SessionID: "s1"},
	}

	for _, ep := range episodes {
		require.NoError(t, buf.Store(ep))
	}

	agentA := buf.GetByAgent("agent-a")
	assert.Len(t, agentA, 2)
	assert.Equal(t, "ep-1", agentA[0].ID)
	assert.Equal(t, "ep-3", agentA[1].ID)

	agentB := buf.GetByAgent("agent-b")
	assert.Len(t, agentB, 1)

	nonexistent := buf.GetByAgent("agent-x")
	assert.Empty(t, nonexistent)
}

func TestEpisodicMemoryBuffer_GetBySession(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	episodes := []*Episode{
		{ID: "ep-1", AgentID: "a", SessionID: "session-1"},
		{ID: "ep-2", AgentID: "a", SessionID: "session-2"},
		{ID: "ep-3", AgentID: "b", SessionID: "session-1"},
		{ID: "ep-4", AgentID: "b", SessionID: ""},
	}

	for _, ep := range episodes {
		require.NoError(t, buf.Store(ep))
	}

	sess1 := buf.GetBySession("session-1")
	assert.Len(t, sess1, 2)
	assert.Equal(t, "ep-1", sess1[0].ID)
	assert.Equal(t, "ep-3", sess1[1].ID)

	sess2 := buf.GetBySession("session-2")
	assert.Len(t, sess2, 1)

	// Episode with empty session should not appear in any session lookup.
	empty := buf.GetBySession("")
	assert.Empty(t, empty)

	nonexistent := buf.GetBySession("session-x")
	assert.Empty(t, nonexistent)
}

func TestEpisodicMemoryBuffer_GetRecent(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	for i := 1; i <= 5; i++ {
		require.NoError(t, buf.Store(&Episode{
			ID:      fmt.Sprintf("ep-%d", i),
			AgentID: "a",
		}))
	}

	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"get last 3", 3, []string{"ep-5", "ep-4", "ep-3"}},
		{"get last 1", 1, []string{"ep-5"}},
		{"get more than available", 10, []string{"ep-5", "ep-4", "ep-3", "ep-2", "ep-1"}},
		{"zero returns empty", 0, nil},
		{"negative returns empty", -1, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			recent := buf.GetRecent(tc.n)
			if tc.expected == nil {
				assert.Empty(t, recent)
			} else {
				require.Len(t, recent, len(tc.expected))
				for i, id := range tc.expected {
					assert.Equal(t, id, recent[i].ID)
				}
			}
		})
	}
}

func TestEpisodicMemoryBuffer_GetRecent_EmptyBuffer(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)
	recent := buf.GetRecent(5)
	assert.Empty(t, recent)
}

func TestEpisodicMemoryBuffer_GetRelevant(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	episodes := []*Episode{
		{ID: "ep-1", AgentID: "a", TaskDescription: "implement binary search algorithm"},
		{ID: "ep-2", AgentID: "a", TaskDescription: "fix database connection pooling"},
		{ID: "ep-3", AgentID: "a", TaskDescription: "optimize search performance"},
		{ID: "ep-4", AgentID: "a", TaskDescription: "write unit tests for sorting"},
	}

	for _, ep := range episodes {
		require.NoError(t, buf.Store(ep))
	}

	// Search for "search" related tasks.
	relevant := buf.GetRelevant("search algorithm optimization", 2)
	require.NotEmpty(t, relevant)
	assert.LessOrEqual(t, len(relevant), 2)

	// Empty query returns empty.
	empty := buf.GetRelevant("", 5)
	assert.Empty(t, empty)

	// Short words (<=2 chars) only returns empty.
	short := buf.GetRelevant("a b c", 5)
	assert.Empty(t, short)

	// Limit 0 returns empty.
	zero := buf.GetRelevant("search", 0)
	assert.Empty(t, zero)
}

func TestEpisodicMemoryBuffer_GetRelevant_EmptyBuffer(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)
	relevant := buf.GetRelevant("search algorithm", 5)
	assert.Empty(t, relevant)
}

func TestEpisodicMemoryBuffer_FIFO_Eviction(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(3)

	// Fill the buffer.
	for i := 1; i <= 3; i++ {
		require.NoError(t, buf.Store(&Episode{
			ID:        fmt.Sprintf("ep-%d", i),
			AgentID:   "a",
			SessionID: "s1",
		}))
	}
	assert.Equal(t, 3, buf.Size())

	// Adding a fourth should evict the first.
	require.NoError(t, buf.Store(&Episode{
		ID:        "ep-4",
		AgentID:   "a",
		SessionID: "s1",
	}))
	assert.Equal(t, 3, buf.Size())

	all := buf.GetAll()
	require.Len(t, all, 3)
	assert.Equal(t, "ep-2", all[0].ID)
	assert.Equal(t, "ep-3", all[1].ID)
	assert.Equal(t, "ep-4", all[2].ID)

	// Agent index should reflect eviction.
	agentEps := buf.GetByAgent("a")
	assert.Len(t, agentEps, 3)
	assert.Equal(t, "ep-2", agentEps[0].ID)

	// Add two more to test continued eviction.
	require.NoError(t, buf.Store(&Episode{
		ID:        "ep-5",
		AgentID:   "b",
		SessionID: "s2",
	}))
	require.NoError(t, buf.Store(&Episode{
		ID:        "ep-6",
		AgentID:   "b",
		SessionID: "s2",
	}))
	assert.Equal(t, 3, buf.Size())

	all = buf.GetAll()
	assert.Equal(t, "ep-4", all[0].ID)
	assert.Equal(t, "ep-5", all[1].ID)
	assert.Equal(t, "ep-6", all[2].ID)

	// Agent "a" should have only ep-4 left.
	agentA := buf.GetByAgent("a")
	assert.Len(t, agentA, 1)
	assert.Equal(t, "ep-4", agentA[0].ID)

	// Session s1 should have only ep-4 left.
	s1 := buf.GetBySession("s1")
	assert.Len(t, s1, 1)
	assert.Equal(t, "ep-4", s1[0].ID)
}

func TestEpisodicMemoryBuffer_MarshalJSON_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(50)

	episodes := []*Episode{
		{
			ID:              "ep-1",
			AgentID:         "agent-a",
			SessionID:       "session-1",
			TaskDescription: "task one",
			AttemptNumber:   1,
			Code:            "code-1",
			Confidence:      0.7,
			Timestamp:       time.Now().Add(-time.Hour),
		},
		{
			ID:              "ep-2",
			AgentID:         "agent-b",
			SessionID:       "session-2",
			TaskDescription: "task two",
			AttemptNumber:   2,
			Code:            "code-2",
			Confidence:      0.9,
			Timestamp:       time.Now(),
		},
	}

	for _, ep := range episodes {
		require.NoError(t, buf.Store(ep))
	}

	// Marshal.
	data, err := json.Marshal(buf)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ep-1")
	assert.Contains(t, string(data), "ep-2")
	assert.Contains(t, string(data), `"max_size":50`)

	// Unmarshal into a new buffer.
	buf2 := &EpisodicMemoryBuffer{}
	err = json.Unmarshal(data, buf2)
	require.NoError(t, err)

	assert.Equal(t, 50, buf2.maxSize)
	assert.Equal(t, 2, buf2.Size())

	// Verify indexes were rebuilt.
	agentA := buf2.GetByAgent("agent-a")
	require.Len(t, agentA, 1)
	assert.Equal(t, "ep-1", agentA[0].ID)

	sess2 := buf2.GetBySession("session-2")
	require.Len(t, sess2, 1)
	assert.Equal(t, "ep-2", sess2[0].ID)
}

func TestEpisodicMemoryBuffer_UnmarshalJSON_InvalidData(t *testing.T) {
	t.Parallel()
	buf := &EpisodicMemoryBuffer{}
	err := json.Unmarshal([]byte(`{invalid json`), buf)
	assert.Error(t, err)
}

func TestEpisodicMemoryBuffer_UnmarshalJSON_DefaultMaxSize(t *testing.T) {
	t.Parallel()
	// max_size of 0 should default to 1000.
	buf := &EpisodicMemoryBuffer{}
	err := json.Unmarshal([]byte(`{"episodes":[],"max_size":0}`), buf)
	require.NoError(t, err)
	assert.Equal(t, 1000, buf.maxSize)
}

func TestEpisodicMemoryBuffer_Clear(t *testing.T) {
	t.Parallel()
	buf := NewEpisodicMemoryBuffer(100)

	for i := 0; i < 5; i++ {
		require.NoError(t, buf.Store(&Episode{
			ID:        fmt.Sprintf("ep-%d", i),
			AgentID:   "a",
			SessionID: "s",
		}))
	}
	assert.Equal(t, 5, buf.Size())

	buf.Clear()

	assert.Equal(t, 0, buf.Size())
	assert.Empty(t, buf.GetAll())
	assert.Empty(t, buf.GetByAgent("a"))
	assert.Empty(t, buf.GetBySession("s"))
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkEpisodicMemoryBuffer_Store(b *testing.B) {
	buf := NewEpisodicMemoryBuffer(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Store(&Episode{ //nolint:errcheck
			ID:              fmt.Sprintf("ep-%d", i),
			AgentID:         "agent-a",
			SessionID:       "session-1",
			TaskDescription: "benchmark task",
		})
	}
}

func BenchmarkEpisodicMemoryBuffer_GetByAgent(b *testing.B) {
	buf := NewEpisodicMemoryBuffer(10000)
	for i := 0; i < 500; i++ {
		agentID := fmt.Sprintf("agent-%d", i%5)
		buf.Store(&Episode{ //nolint:errcheck
			ID:      fmt.Sprintf("ep-%d", i),
			AgentID: agentID,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.GetByAgent("agent-0")
	}
}

func BenchmarkEpisodicMemoryBuffer_GetRecent(b *testing.B) {
	buf := NewEpisodicMemoryBuffer(10000)
	for i := 0; i < 500; i++ {
		buf.Store(&Episode{ //nolint:errcheck
			ID:      fmt.Sprintf("ep-%d", i),
			AgentID: "a",
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.GetRecent(10)
	}
}

func BenchmarkEpisodicMemoryBuffer_GetRelevant(b *testing.B) {
	buf := NewEpisodicMemoryBuffer(10000)
	descriptions := []string{
		"implement binary search algorithm",
		"fix database connection pooling",
		"optimize search performance",
		"write unit tests for sorting",
		"refactor authentication service",
	}
	for i := 0; i < 500; i++ {
		buf.Store(&Episode{ //nolint:errcheck
			ID:              fmt.Sprintf("ep-%d", i),
			AgentID:         "a",
			TaskDescription: descriptions[i%len(descriptions)],
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.GetRelevant("search algorithm optimization", 5)
	}
}

func BenchmarkEpisodicMemoryBuffer_MarshalJSON(b *testing.B) {
	buf := NewEpisodicMemoryBuffer(1000)
	for i := 0; i < 100; i++ {
		buf.Store(&Episode{ //nolint:errcheck
			ID:              fmt.Sprintf("ep-%d", i),
			AgentID:         "agent-a",
			SessionID:       "session-1",
			TaskDescription: "benchmark task description",
			Confidence:      0.85,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(buf) //nolint:errcheck
	}
}
