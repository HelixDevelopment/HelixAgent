package reflexion

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccumulatedWisdom(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()
	require.NotNil(t, w)
	assert.Equal(t, 0, w.Size())
	assert.Empty(t, w.GetAll())
}

func TestAccumulatedWisdom_Store(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()

	t.Run("store valid wisdom", func(t *testing.T) {
			t.Parallel()
		wisdom := &Wisdom{
			ID:        "wis-1",
			Pattern:   "nil pointer check",
			Domain:    "code",
			Frequency: 3,
			Impact:    0.5,
			Tags:      []string{"nil", "pointer"},
			CreatedAt: time.Now(),
		}

		err := w.Store(wisdom)
		require.NoError(t, err)
		assert.Equal(t, 1, w.Size())

		all := w.GetAll()
		require.Len(t, all, 1)
		assert.Equal(t, "wis-1", all[0].ID)
	})

	t.Run("store nil wisdom", func(t *testing.T) {
			t.Parallel()
		err := w.Store(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be nil")
	})

	t.Run("auto-generate ID", func(t *testing.T) {
			t.Parallel()
		wisdom := &Wisdom{
			Pattern: "test pattern",
			Domain:  "testing",
		}

		err := w.Store(wisdom)
		require.NoError(t, err)
		assert.NotEmpty(t, wisdom.ID)
		assert.Contains(t, wisdom.ID, "wis-")
	})

	t.Run("auto-set CreatedAt", func(t *testing.T) {
			t.Parallel()
		wisdom := &Wisdom{
			ID:      "wis-auto-time",
			Pattern: "time pattern",
		}

		err := w.Store(wisdom)
		require.NoError(t, err)
		assert.False(t, wisdom.CreatedAt.IsZero())
	})

	t.Run("nil tags become empty slice", func(t *testing.T) {
			t.Parallel()
		wisdom := &Wisdom{
			ID:      "wis-nil-tags",
			Pattern: "tag test",
			Tags:    nil,
		}

		err := w.Store(wisdom)
		require.NoError(t, err)
		assert.NotNil(t, wisdom.Tags)
		assert.Empty(t, wisdom.Tags)
	})

	t.Run("domain indexing", func(t *testing.T) {
			t.Parallel()
		w2 := NewAccumulatedWisdom()

		_ = w2.Store(&Wisdom{
			ID: "w1", Pattern: "p1", Domain: "code",
		})
		_ = w2.Store(&Wisdom{
			ID: "w2", Pattern: "p2", Domain: "testing",
		})
		_ = w2.Store(&Wisdom{
			ID: "w3", Pattern: "p3", Domain: "code",
		})
		_ = w2.Store(&Wisdom{
			ID: "w4", Pattern: "p4", Domain: "",
		})

		code := w2.GetByDomain("code")
		assert.Len(t, code, 2)

		testing_ := w2.GetByDomain("testing")
		assert.Len(t, testing_, 1)

		empty := w2.GetByDomain("")
		assert.Empty(t, empty)
	})
}

func TestAccumulatedWisdom_ExtractFromEpisodes(t *testing.T) {
	t.Parallel()
	t.Run("empty episodes", func(t *testing.T) {
		w := NewAccumulatedWisdom()
		result, err := w.ExtractFromEpisodes([]*Episode{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("single episode per cause (no extraction)", func(t *testing.T) {
			t.Parallel()
		w := NewAccumulatedWisdom()
		episodes := []*Episode{
			{
				ID:      "ep-1",
				AgentID: "a",
				Reflection: &Reflection{
					RootCause: "unique cause 1",
				},
				Confidence: 0.5,
			},
			{
				ID:      "ep-2",
				AgentID: "a",
				Reflection: &Reflection{
					RootCause: "unique cause 2",
				},
				Confidence: 0.6,
			},
		}

		result, err := w.ExtractFromEpisodes(episodes)
		require.NoError(t, err)
		// Needs >= 2 episodes per cause to extract wisdom.
		assert.Empty(t, result)
	})

	t.Run("multiple episodes with same cause", func(t *testing.T) {
			t.Parallel()
		w := NewAccumulatedWisdom()
		episodes := []*Episode{
			{
				ID:              "ep-1",
				AgentID:         "a",
				SessionID:       "s1",
				AttemptNumber:   1,
				Code:            "func test() {}",
				FailureAnalysis: "test assertion failed",
				Reflection: &Reflection{
					RootCause: "nil pointer dereference at runtime",
				},
				Confidence: 0.3,
			},
			{
				ID:              "ep-2",
				AgentID:         "a",
				SessionID:       "s1",
				AttemptNumber:   2,
				Code:            "func test() { if x != nil {} }",
				FailureAnalysis: "test assertion failed",
				Reflection: &Reflection{
					RootCause: "nil pointer dereference at runtime",
				},
				Confidence: 0.6,
			},
			{
				ID:              "ep-3",
				AgentID:         "b",
				SessionID:       "s2",
				AttemptNumber:   1,
				Code:            "func other() {}",
				FailureAnalysis: "compile error",
				Reflection: &Reflection{
					RootCause: "nil pointer dereference at runtime",
				},
				Confidence: 0.4,
			},
		}

		result, err := w.ExtractFromEpisodes(episodes)
		require.NoError(t, err)
		require.Len(t, result, 1)

		wisdom := result[0]
		assert.Equal(t, "nil pointer dereference at runtime", wisdom.Pattern)
		assert.Equal(t, 3, wisdom.Frequency)
		assert.NotEmpty(t, wisdom.ID)
		assert.NotEmpty(t, wisdom.Source)
		assert.NotEmpty(t, wisdom.Tags)
		assert.NotEmpty(t, wisdom.Domain)
		assert.Equal(t, 0, wisdom.UseCount)
		assert.InDelta(t, 0.0, wisdom.SuccessRate, 0.001)

		// Wisdom should be stored.
		assert.Equal(t, 1, w.Size())
	})

	t.Run("episodes without reflections are skipped", func(t *testing.T) {
			t.Parallel()
		w := NewAccumulatedWisdom()
		episodes := []*Episode{
			{ID: "ep-1", AgentID: "a", Reflection: nil},
			{ID: "ep-2", AgentID: "a", Reflection: nil},
		}

		result, err := w.ExtractFromEpisodes(episodes)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("episodes with empty root cause are skipped", func(t *testing.T) {
			t.Parallel()
		w := NewAccumulatedWisdom()
		episodes := []*Episode{
			{
				ID: "ep-1", AgentID: "a",
				Reflection: &Reflection{RootCause: ""},
			},
			{
				ID: "ep-2", AgentID: "a",
				Reflection: &Reflection{RootCause: "  "},
			},
		}

		result, err := w.ExtractFromEpisodes(episodes)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestAccumulatedWisdom_GetRelevant(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()

	wisdoms := []*Wisdom{
		{
			ID:      "w1",
			Pattern: "nil pointer dereference check",
			Domain:  "code",
			Tags:    []string{"nil", "pointer", "dereference", "check"},
			Impact:  0.8,
		},
		{
			ID:      "w2",
			Pattern: "database connection timeout handling",
			Domain:  "infrastructure",
			Tags:    []string{"database", "connection", "timeout", "handling"},
			Impact:  0.6,
		},
		{
			ID:      "w3",
			Pattern: "test assertion mismatch pattern",
			Domain:  "testing",
			Tags:    []string{"test", "assertion", "mismatch", "pattern"},
			Impact:  0.5,
		},
	}

	for _, ws := range wisdoms {
		require.NoError(t, w.Store(ws))
	}

	t.Run("find relevant by keyword", func(t *testing.T) {
			t.Parallel()
		results := w.GetRelevant("nil pointer check in code", 2)
		require.NotEmpty(t, results)
		// w1 should be most relevant.
		assert.Equal(t, "w1", results[0].ID)
	})

	t.Run("find database related", func(t *testing.T) {
			t.Parallel()
		results := w.GetRelevant("database timeout issue", 1)
		require.Len(t, results, 1)
		assert.Equal(t, "w2", results[0].ID)
	})

	t.Run("empty query", func(t *testing.T) {
			t.Parallel()
		results := w.GetRelevant("", 5)
		assert.Empty(t, results)
	})

	t.Run("limit 0", func(t *testing.T) {
			t.Parallel()
		results := w.GetRelevant("pointer", 0)
		assert.Empty(t, results)
	})

	t.Run("no matches", func(t *testing.T) {
			t.Parallel()
		results := w.GetRelevant("kubernetes deployment", 5)
		assert.Empty(t, results)
	})

	t.Run("empty wisdom store", func(t *testing.T) {
			t.Parallel()
		empty := NewAccumulatedWisdom()
		results := empty.GetRelevant("pointer", 5)
		assert.Empty(t, results)
	})
}

func TestAccumulatedWisdom_RecordUsage(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()

	wisdom := &Wisdom{
		ID:          "wis-usage",
		Pattern:     "test pattern",
		UseCount:    0,
		SuccessRate: 0.0,
	}
	require.NoError(t, w.Store(wisdom))

	t.Run("record successful usage", func(t *testing.T) {
			t.Parallel()
		err := w.RecordUsage("wis-usage", true)
		require.NoError(t, err)

		all := w.GetAll()
		require.Len(t, all, 1)
		assert.Equal(t, 1, all[0].UseCount)
		assert.InDelta(t, 1.0, all[0].SuccessRate, 0.001)
		assert.False(t, all[0].LastUsedAt.IsZero())
	})

	t.Run("record failed usage", func(t *testing.T) {
			t.Parallel()
		err := w.RecordUsage("wis-usage", false)
		require.NoError(t, err)

		all := w.GetAll()
		require.Len(t, all, 1)
		assert.Equal(t, 2, all[0].UseCount)
		// Success rate: (1.0 * 1) / 2 = 0.5
		assert.InDelta(t, 0.5, all[0].SuccessRate, 0.001)
	})

	t.Run("record another success", func(t *testing.T) {
			t.Parallel()
		err := w.RecordUsage("wis-usage", true)
		require.NoError(t, err)

		all := w.GetAll()
		assert.Equal(t, 3, all[0].UseCount)
		// Success rate: (0.5 * 2 + 1.0) / 3 = 2.0/3 ~ 0.667
		assert.InDelta(t, 2.0/3.0, all[0].SuccessRate, 0.01)
	})

	t.Run("not found", func(t *testing.T) {
			t.Parallel()
		err := w.RecordUsage("nonexistent", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestAccumulatedWisdom_GetByDomain(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()

	for i := 0; i < 5; i++ {
		domain := "code"
		if i%2 == 0 {
			domain = "testing"
		}
		require.NoError(t, w.Store(&Wisdom{
			ID:      fmt.Sprintf("w-%d", i),
			Pattern: fmt.Sprintf("pattern %d", i),
			Domain:  domain,
		}))
	}

	code := w.GetByDomain("code")
	assert.Len(t, code, 2) // i=1, i=3

	testing_ := w.GetByDomain("testing")
	assert.Len(t, testing_, 3) // i=0, i=2, i=4

	none := w.GetByDomain("architecture")
	assert.Empty(t, none)
}

func TestAccumulatedWisdom_MarshalJSON_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	w := NewAccumulatedWisdom()

	wisdoms := []*Wisdom{
		{
			ID:          "w1",
			Pattern:     "nil check pattern",
			Source:      "ep-1,ep-2",
			Frequency:   5,
			Impact:      0.7,
			Domain:      "code",
			Tags:        []string{"nil", "check"},
			CreatedAt:   time.Now().Add(-time.Hour),
			LastUsedAt:  time.Now(),
			UseCount:    3,
			SuccessRate: 0.67,
		},
		{
			ID:          "w2",
			Pattern:     "timeout handling",
			Source:      "ep-3,ep-4",
			Frequency:   2,
			Impact:      0.5,
			Domain:      "infrastructure",
			Tags:        []string{"timeout"},
			CreatedAt:   time.Now(),
			LastUsedAt:  time.Time{},
			UseCount:    0,
			SuccessRate: 0.0,
		},
	}

	for _, ws := range wisdoms {
		require.NoError(t, w.Store(ws))
	}

	// Marshal.
	data, err := json.Marshal(w)
	require.NoError(t, err)
	assert.Contains(t, string(data), "nil check pattern")
	assert.Contains(t, string(data), "timeout handling")
	assert.Contains(t, string(data), `"insights"`)

	// Unmarshal into a new store.
	w2 := &AccumulatedWisdom{}
	err = json.Unmarshal(data, w2)
	require.NoError(t, err)

	assert.Equal(t, 2, w2.Size())

	all := w2.GetAll()
	assert.Equal(t, "w1", all[0].ID)
	assert.Equal(t, "w2", all[1].ID)

	// Domain index should be rebuilt.
	code := w2.GetByDomain("code")
	require.Len(t, code, 1)
	assert.Equal(t, "w1", code[0].ID)

	infra := w2.GetByDomain("infrastructure")
	require.Len(t, infra, 1)
	assert.Equal(t, "w2", infra[0].ID)

	// Tags should not be nil.
	for _, ws := range w2.GetAll() {
		assert.NotNil(t, ws.Tags)
	}
}

func TestAccumulatedWisdom_UnmarshalJSON_InvalidData(t *testing.T) {
	t.Parallel()
	w := &AccumulatedWisdom{}
	err := json.Unmarshal([]byte(`{bad json`), w)
	assert.Error(t, err)
}

func TestAccumulatedWisdom_UnmarshalJSON_NilInsights(t *testing.T) {
	t.Parallel()
	w := &AccumulatedWisdom{}
	err := json.Unmarshal([]byte(`{"insights":null}`), w)
	require.NoError(t, err)
	assert.NotNil(t, w.insights)
	assert.Empty(t, w.insights)
	assert.Equal(t, 0, w.Size())
}

func TestAccumulatedWisdom_UnmarshalJSON_NilTagsNormalized(t *testing.T) {
	t.Parallel()
	w := &AccumulatedWisdom{}
	data := `{"insights":[{"id":"w1","pattern":"p1","domain":"code","tags":null}]}`
	err := json.Unmarshal([]byte(data), w)
	require.NoError(t, err)

	all := w.GetAll()
	require.Len(t, all, 1)
	assert.NotNil(t, all[0].Tags)
	assert.Empty(t, all[0].Tags)
}
