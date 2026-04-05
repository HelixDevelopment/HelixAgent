package memory_test

import (
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/memory"
	helixmem "dev.helix.agent/internal/memory"
	modmem "digital.vasic.memory/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ToModuleMemory / ToHelixMemory conversion tests (no store needed)
// ============================================================================

func TestToModuleMemory_Nil(t *testing.T) {
	t.Parallel()
	result := adapter.ToModuleMemory(nil)
	assert.Nil(t, result)
}

func TestToModuleMemory_Basic(t *testing.T) {
	t.Parallel()
	now := time.Now()
	helix := &helixmem.Memory{
		ID:          "mem-001",
		UserID:      "user-123",
		SessionID:   "session-456",
		Content:     "Test memory content",
		Type:        helixmem.MemoryTypeEpisodic,
		Category:    "work",
		Importance:  0.8,
		AccessCount: 3,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]interface{}{"tag": "important"},
	}

	mod := adapter.ToModuleMemory(helix)
	require.NotNil(t, mod)
	assert.Equal(t, "mem-001", mod.ID)
	assert.Equal(t, "Test memory content", mod.Content)
	assert.Equal(t, now, mod.CreatedAt)
	assert.Equal(t, now, mod.UpdatedAt)

	// Metadata should contain user_id, session_id, etc.
	assert.Equal(t, "user-123", mod.Metadata["user_id"])
	assert.Equal(t, "session-456", mod.Metadata["session_id"])
	assert.Equal(t, "work", mod.Metadata["category"])
	assert.Equal(t, 0.8, mod.Metadata["importance"])
	assert.Equal(t, 3, mod.Metadata["access_count"])
}

func TestToHelixMemory_Nil(t *testing.T) {
	t.Parallel()
	result := adapter.ToHelixMemory(nil)
	assert.Nil(t, result)
}

func TestToHelixMemory_Basic(t *testing.T) {
	t.Parallel()
	now := time.Now()
	modMemory := &modmem.Memory{
		ID:      "mem-001",
		Content: "Test content",
		Metadata: map[string]any{
			"user_id":      "user-123",
			"session_id":   "session-456",
			"type":         "short_term",
			"category":     "work",
			"importance":   0.8,
			"access_count": 3,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	helix := adapter.ToHelixMemory(modMemory)
	require.NotNil(t, helix)
	assert.Equal(t, "mem-001", helix.ID)
	assert.Equal(t, "user-123", helix.UserID)
	assert.Equal(t, "session-456", helix.SessionID)
	assert.Equal(t, "Test content", helix.Content)
	assert.Equal(t, "work", helix.Category)
}

func TestToHelixMemories(t *testing.T) {
	t.Parallel()
	now := time.Now()
	modMems := []*modmem.Memory{
		{ID: "m1", Content: "Memory 1", Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now},
		{ID: "m2", Content: "Memory 2", Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now},
		{ID: "m3", Content: "Memory 3", Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now},
	}

	result := adapter.ToHelixMemories(modMems)
	require.Len(t, result, 3)
	assert.Equal(t, "m1", result[0].ID)
	assert.Equal(t, "m2", result[1].ID)
	assert.Equal(t, "m3", result[2].ID)
}

func TestToHelixMemories_Empty(t *testing.T) {
	t.Parallel()
	result := adapter.ToHelixMemories(nil)
	assert.Len(t, result, 0)

	result = adapter.ToHelixMemories([]*modmem.Memory{})
	assert.Len(t, result, 0)
}

func TestRoundTrip_Memory(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	original := &helixmem.Memory{
		ID:          "round-trip-001",
		UserID:      "user-999",
		SessionID:   "session-888",
		Content:     "Round trip test content",
		Type:        helixmem.MemoryTypeSemantic,
		Category:    "personal",
		Importance:  0.5,
		AccessCount: 7,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	modMem := adapter.ToModuleMemory(original)
	require.NotNil(t, modMem)

	recovered := adapter.ToHelixMemory(modMem)
	require.NotNil(t, recovered)

	assert.Equal(t, original.ID, recovered.ID)
	assert.Equal(t, original.UserID, recovered.UserID)
	assert.Equal(t, original.SessionID, recovered.SessionID)
	assert.Equal(t, original.Content, recovered.Content)
	assert.Equal(t, original.Category, recovered.Category)
	assert.Equal(t, original.Importance, recovered.Importance)
}

func TestToModuleMemory_WithEmbedding(t *testing.T) {
	t.Parallel()
	now := time.Now()
	embedding := []float32{0.1, 0.2, 0.3}
	helix := &helixmem.Memory{
		ID:        "emb-001",
		Content:   "embedded content",
		Embedding: embedding,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mod := adapter.ToModuleMemory(helix)
	require.NotNil(t, mod)
	assert.Equal(t, embedding, mod.Embedding)
}

// ============================================================================
// SearchOptions conversion tests
// ============================================================================

func TestToModuleSearchOptions_Nil(t *testing.T) {
	t.Parallel()
	opts := adapter.ToModuleSearchOptions(nil)
	require.NotNil(t, opts) // Should return default
}

func TestToModuleSearchOptions_WithValues(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	helixOpts := &helixmem.SearchOptions{
		TopK:     20,
		MinScore: 0.7,
		TimeRange: &helixmem.TimeRange{
			Start: start,
			End:   end,
		},
	}

	modOpts := adapter.ToModuleSearchOptions(helixOpts)
	require.NotNil(t, modOpts)
	assert.Equal(t, 20, modOpts.TopK)
	assert.Equal(t, 0.7, modOpts.MinScore)
	require.NotNil(t, modOpts.TimeRange)
	assert.Equal(t, start, modOpts.TimeRange.Start)
	assert.Equal(t, end, modOpts.TimeRange.End)
}

func TestToModuleSearchOptions_NoTimeRange(t *testing.T) {
	t.Parallel()
	helixOpts := &helixmem.SearchOptions{
		TopK:     5,
		MinScore: 0.5,
	}

	modOpts := adapter.ToModuleSearchOptions(helixOpts)
	require.NotNil(t, modOpts)
	assert.Equal(t, 5, modOpts.TopK)
	assert.Nil(t, modOpts.TimeRange)
}

// ============================================================================
// ListOptions conversion tests
// ============================================================================

func TestToModuleListOptions_Nil(t *testing.T) {
	t.Parallel()
	opts := adapter.ToModuleListOptions(nil)
	require.NotNil(t, opts) // Should return default
}

func TestToModuleListOptions_WithValues(t *testing.T) {
	t.Parallel()
	helixOpts := &helixmem.ListOptions{
		Offset: 10,
		Limit:  50,
		SortBy: "created_at",
	}

	modOpts := adapter.ToModuleListOptions(helixOpts)
	require.NotNil(t, modOpts)
	assert.Equal(t, 10, modOpts.Offset)
	assert.Equal(t, 50, modOpts.Limit)
	assert.Equal(t, "created_at", modOpts.OrderBy)
}

// ============================================================================
// EntityExtractorAdapter Tests
// ============================================================================

func TestNewEntityExtractorAdapter(t *testing.T) {
	t.Parallel()
	ea := adapter.NewEntityExtractorAdapter()
	require.NotNil(t, ea)
}

func TestEntityExtractorAdapter_ExtractEntities(t *testing.T) {
	t.Parallel()
	ea := adapter.NewEntityExtractorAdapter()

	entities, err := ea.ExtractEntities("John works at Google in New York.")
	require.NoError(t, err)
	assert.NotNil(t, entities)
}

func TestEntityExtractorAdapter_ExtractRelations(t *testing.T) {
	t.Parallel()
	ea := adapter.NewEntityExtractorAdapter()

	relations, err := ea.ExtractRelations("John works at Google.")
	require.NoError(t, err)
	assert.NotNil(t, relations)
}

func TestEntityExtractorAdapter_ExtractEntities_EmptyText(t *testing.T) {
	t.Parallel()
	ea := adapter.NewEntityExtractorAdapter()

	entities, err := ea.ExtractEntities("")
	require.NoError(t, err)
	assert.Empty(t, entities)
}
