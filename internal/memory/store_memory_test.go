package memory

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// --- NewInMemoryStore ---

func TestNewInMemoryStore_Initialization(t *testing.T) {
	store := NewInMemoryStore()
	require.NotNil(t, store)
	assert.NotNil(t, store.memories)
	assert.NotNil(t, store.entities)
	assert.NotNil(t, store.relationships)
	assert.NotNil(t, store.userIndex)
	assert.NotNil(t, store.sessionIndex)
	assert.NotNil(t, store.entityIndex)
	assert.Equal(t, 0, store.memories.Len())
	assert.Equal(t, 0, store.entities.Len())
	assert.Equal(t, 0, store.relationships.Len())
	assert.Equal(t, 0, store.userIndex.Len())
	assert.Equal(t, 0, store.sessionIndex.Len())
	assert.Equal(t, 0, store.entityIndex.Len())
}

func TestNewInMemoryStore_ReturnsDistinctInstances(t *testing.T) {
	s1 := NewInMemoryStore()
	s2 := NewInMemoryStore()
	assert.NotSame(t, s1, s2)
}

// --- Add ---

func TestInMemoryStore_Add_BasicMemory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	mem := &Memory{
		ID:        "mem-add-1",
		UserID:    "user1",
		SessionID: "sess1",
		Content:   "test content",
		Type:      MemoryTypeSemantic,
	}

	err := store.Add(ctx, mem)
	require.NoError(t, err)

	assert.Equal(t, "mem-add-1", mem.ID)
	assert.True(t, store.memories.Has("mem-add-1"))
}

func TestInMemoryStore_Add_GeneratesID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	mem := &Memory{
		UserID:  "user1",
		Content: "no ID provided",
	}

	err := store.Add(ctx, mem)
	require.NoError(t, err)
	assert.NotEmpty(t, mem.ID)
}

func TestInMemoryStore_Add_UpdatesUserIndex(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "user1", Content: "a"})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "user1", Content: "b"})
	_ = store.Add(ctx, &Memory{ID: "m3", UserID: "user2", Content: "c"})

	u1, _ := store.userIndex.Get("user1")
	u2, _ := store.userIndex.Get("user2")
	assert.Len(t, u1, 2)
	assert.Len(t, u2, 1)
	assert.Contains(t, u1, "m1")
	assert.Contains(t, u1, "m2")
}

func TestInMemoryStore_Add_UpdatesSessionIndex(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "m1", SessionID: "s1", Content: "a"})
	_ = store.Add(ctx, &Memory{ID: "m2", SessionID: "s1", Content: "b"})
	_ = store.Add(ctx, &Memory{ID: "m3", SessionID: "s2", Content: "c"})

	s1, _ := store.sessionIndex.Get("s1")
	s2, _ := store.sessionIndex.Get("s2")
	assert.Len(t, s1, 2)
	assert.Len(t, s2, 1)
}

func TestInMemoryStore_Add_EmptyUserAndSession(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	mem := &Memory{ID: "m1", Content: "no user or session"}
	err := store.Add(ctx, mem)
	require.NoError(t, err)
	assert.Equal(t, 0, store.userIndex.Len())
	assert.Equal(t, 0, store.sessionIndex.Len())
}

func TestInMemoryStore_Add_MultipleMemoriesTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		userID    string
		sessionID string
		content   string
	}{
		{"WithAllFields", "m1", "user1", "s1", "content1"},
		{"OnlyUserID", "m2", "user2", "", "content2"},
		{"OnlySessionID", "m3", "", "s2", "content3"},
		{"NoUserOrSession", "m4", "", "", "content4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewInMemoryStore()
			ctx := context.Background()

			mem := &Memory{
				ID:        tt.id,
				UserID:    tt.userID,
				SessionID: tt.sessionID,
				Content:   tt.content,
			}

			err := store.Add(ctx, mem)
			require.NoError(t, err)
			assert.True(t, store.memories.Has(tt.id))
		})
	}
}

// --- Get ---

func TestInMemoryStore_Get_ExistingMemory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "get-m1", Content: "Test Get", UserID: "u1"})

	mem, err := store.Get(ctx, "get-m1")
	require.NoError(t, err)
	assert.Equal(t, "get-m1", mem.ID)
	assert.Equal(t, "Test Get", mem.Content)
}

func TestInMemoryStore_Get_IncrementsAccessCount(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "get-m2", Content: "Test"})

	for i := 0; i < 5; i++ {
		mem, err := store.Get(ctx, "get-m2")
		require.NoError(t, err)
		assert.Equal(t, i+1, mem.AccessCount)
	}
}

func TestInMemoryStore_Get_UpdatesLastAccess(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "get-m3", Content: "Test"})

	before := time.Now().Add(-time.Millisecond)
	mem, err := store.Get(ctx, "get-m3")
	require.NoError(t, err)
	assert.True(t, mem.LastAccess.After(before) || mem.LastAccess.Equal(before))
}

func TestInMemoryStore_Get_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory not found")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestInMemoryStore_Get_EmptyID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory not found")
}

// --- Update ---

func TestInMemoryStore_Update_ExistingMemory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "upd-m1", Content: "Original"})

	err := store.Update(ctx, &Memory{ID: "upd-m1", Content: "Updated"})
	require.NoError(t, err)

	mem, _ := store.Get(ctx, "upd-m1")
	assert.Equal(t, "Updated", mem.Content)
}

func TestInMemoryStore_Update_SetsUpdatedAt(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "upd-m2", Content: "Original"})

	before := time.Now()
	err := store.Update(ctx, &Memory{ID: "upd-m2", Content: "Updated"})
	require.NoError(t, err)

	mem, _ := store.Get(ctx, "upd-m2")
	assert.True(t, mem.UpdatedAt.After(before) || mem.UpdatedAt.Equal(before))
}

func TestInMemoryStore_Update_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.Update(ctx, &Memory{ID: "nonexistent", Content: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory not found")
}

func TestInMemoryStore_Update_ReplacesEntireMemory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	original := &Memory{
		ID:         "upd-m3",
		UserID:     "user1",
		Content:    "Original",
		Importance: 0.5,
	}
	_ = store.Add(ctx, original)

	replacement := &Memory{
		ID:         "upd-m3",
		UserID:     "user2",
		Content:    "Replaced",
		Importance: 0.9,
	}
	err := store.Update(ctx, replacement)
	require.NoError(t, err)

	mem, _ := store.Get(ctx, "upd-m3")
	assert.Equal(t, "user2", mem.UserID)
	assert.Equal(t, "Replaced", mem.Content)
	assert.InDelta(t, 0.9, mem.Importance, 0.001)
}

// --- Delete ---

func TestInMemoryStore_Delete_ExistingMemory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{
		ID:        "del-m1",
		UserID:    "user1",
		SessionID: "sess1",
		Content:   "To Delete",
	})

	err := store.Delete(ctx, "del-m1")
	require.NoError(t, err)

	_, err = store.Get(ctx, "del-m1")
	require.Error(t, err)
}

func TestInMemoryStore_Delete_RemovesFromUserIndex(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "del-m2", UserID: "user1", Content: "a"})
	_ = store.Add(ctx, &Memory{ID: "del-m3", UserID: "user1", Content: "b"})

	err := store.Delete(ctx, "del-m2")
	require.NoError(t, err)

	u1, _ := store.userIndex.Get("user1")
	assert.Len(t, u1, 1)
	assert.NotContains(t, u1, "del-m2")
	assert.Contains(t, u1, "del-m3")
}

func TestInMemoryStore_Delete_RemovesFromSessionIndex(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "del-m4", SessionID: "s1", Content: "a"})
	_ = store.Add(ctx, &Memory{ID: "del-m5", SessionID: "s1", Content: "b"})

	err := store.Delete(ctx, "del-m4")
	require.NoError(t, err)

	s1, _ := store.sessionIndex.Get("s1")
	assert.Len(t, s1, 1)
	assert.NotContains(t, s1, "del-m4")
}

func TestInMemoryStore_Delete_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memory not found")
}

func TestInMemoryStore_Delete_EmptyUserAndSession(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "del-m6", Content: "no user/session"})

	err := store.Delete(ctx, "del-m6")
	require.NoError(t, err)
}

// --- Search ---

func TestInMemoryStore_Search_NilOptions(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "programming basics"})
	results, err := store.Search(ctx, "programming", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestInMemoryStore_Search_FilterByUserID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", UserID: "u1", Content: "Go programming"})
	_ = store.Add(ctx, &Memory{ID: "s2", UserID: "u2", Content: "Go programming"})

	opts := &SearchOptions{UserID: "u1", MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, "u1", r.UserID)
	}
}

func TestInMemoryStore_Search_FilterBySessionID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", SessionID: "sess1", Content: "Go programming"})
	_ = store.Add(ctx, &Memory{ID: "s2", SessionID: "sess2", Content: "Go programming"})

	opts := &SearchOptions{SessionID: "sess1", MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, "sess1", r.SessionID)
	}
}

func TestInMemoryStore_Search_FilterByType(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "Go programming", Type: MemoryTypeSemantic})
	_ = store.Add(ctx, &Memory{ID: "s2", Content: "Go programming", Type: MemoryTypeEpisodic})

	opts := &SearchOptions{Type: MemoryTypeSemantic, MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, MemoryTypeSemantic, r.Type)
	}
}

func TestInMemoryStore_Search_FilterByCategory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "Go programming", Category: "tech"})
	_ = store.Add(ctx, &Memory{ID: "s2", Content: "Go programming", Category: "other"})

	opts := &SearchOptions{Category: "tech", MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, "tech", r.Category)
	}
}

func TestInMemoryStore_Search_FilterByTimeRange(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{
		ID: "s1", Content: "recent programming", CreatedAt: now.Add(-10 * time.Minute),
	})
	_ = store.Add(ctx, &Memory{
		ID: "s2", Content: "old programming", CreatedAt: now.Add(-2 * time.Hour),
	})

	opts := &SearchOptions{
		MinScore: 0,
		TimeRange: &TimeRange{
			Start: now.Add(-30 * time.Minute),
			End:   now.Add(time.Minute),
		},
	}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	for _, r := range results {
		assert.True(t, r.CreatedAt.After(now.Add(-30*time.Minute)))
	}
}

func TestInMemoryStore_Search_TopKLimit(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("m%d", i),
			Content: "programming language basics tutorial",
		})
	}

	opts := &SearchOptions{TopK: 3, MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 3)
}

func TestInMemoryStore_Search_SortedByScore(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "python machine learning"})
	_ = store.Add(ctx, &Memory{ID: "s2", Content: "go programming language tutorial"})

	opts := &SearchOptions{MinScore: 0, TopK: 10}
	results, err := store.Search(ctx, "go programming language", opts)
	require.NoError(t, err)

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].Importance, results[i].Importance)
	}
}

func TestInMemoryStore_Search_NoResults(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "Go programming"})

	results, err := store.Search(ctx, "completely unrelated zzyzzy", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestInMemoryStore_Search_EmptyStore(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	results, err := store.Search(ctx, "anything", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestInMemoryStore_Search_CombinedFilters(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{
		ID: "s1", UserID: "u1", SessionID: "sess1",
		Content: "Go programming", Type: MemoryTypeSemantic,
		Category: "tech", CreatedAt: now,
	})
	_ = store.Add(ctx, &Memory{
		ID: "s2", UserID: "u1", SessionID: "sess2",
		Content: "Go programming", Type: MemoryTypeEpisodic,
		Category: "tech", CreatedAt: now,
	})
	_ = store.Add(ctx, &Memory{
		ID: "s3", UserID: "u2", SessionID: "sess1",
		Content: "Go programming", Type: MemoryTypeSemantic,
		Category: "tech", CreatedAt: now,
	})

	opts := &SearchOptions{
		UserID:    "u1",
		SessionID: "sess1",
		Type:      MemoryTypeSemantic,
		Category:  "tech",
		MinScore:  0,
		TopK:      10,
	}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s1", results[0].ID)
}

func TestInMemoryStore_Search_ReturnsCopies(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "s1", Content: "Go programming", Importance: 0.5})

	opts := &SearchOptions{MinScore: 0}
	results, err := store.Search(ctx, "programming", opts)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	// Modifying result should not affect stored memory
	results[0].Content = "Modified"

	stored, _ := store.memories.Get("s1")
	assert.Equal(t, "Go programming", stored.Content)
}

// --- GetByUser ---

func TestInMemoryStore_GetByUser_ReturnsAll(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("m%d", i),
			UserID:  "user1",
			Content: fmt.Sprintf("Memory %d", i),
		})
	}

	results, err := store.GetByUser(ctx, "user1", nil)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestInMemoryStore_GetByUser_EmptyForUnknownUser(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	results, err := store.GetByUser(ctx, "unknown-user", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.NotNil(t, results) // Should return empty slice, not nil
}

func TestInMemoryStore_GetByUser_Pagination(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("m%d", i),
			UserID:  "user1",
			Content: fmt.Sprintf("Memory %d", i),
		})
	}

	tests := []struct {
		name     string
		limit    int
		offset   int
		expected int
	}{
		{"FirstPage", 3, 0, 3},
		{"SecondPage", 3, 3, 3},
		{"LastPage", 3, 9, 1},
		{"OffsetExceeds", 3, 100, 0},
		{"ZeroLimit", 0, 0, 10},
		{"LargeLimit", 100, 0, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ListOptions{Limit: tt.limit, Offset: tt.offset}
			results, err := store.GetByUser(ctx, "user1", opts)
			require.NoError(t, err)
			assert.Len(t, results, tt.expected)
		})
	}
}

func TestInMemoryStore_GetByUser_SortByCreatedAt(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "u1", Content: "a", CreatedAt: now.Add(time.Hour)})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "u1", Content: "b", CreatedAt: now})
	_ = store.Add(ctx, &Memory{ID: "m3", UserID: "u1", Content: "c", CreatedAt: now.Add(-time.Hour)})

	opts := &ListOptions{SortBy: "created_at", Order: "asc"}
	results, err := store.GetByUser(ctx, "u1", opts)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "m3", results[0].ID)
	assert.Equal(t, "m2", results[1].ID)
	assert.Equal(t, "m1", results[2].ID)
}

func TestInMemoryStore_GetByUser_SortByImportanceDesc(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "u1", Content: "a", Importance: 0.3})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "u1", Content: "b", Importance: 0.9})
	_ = store.Add(ctx, &Memory{ID: "m3", UserID: "u1", Content: "c", Importance: 0.6})

	opts := &ListOptions{SortBy: "importance", Order: "desc"}
	results, err := store.GetByUser(ctx, "u1", opts)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "m2", results[0].ID)
	assert.Equal(t, "m3", results[1].ID)
	assert.Equal(t, "m1", results[2].ID)
}

func TestInMemoryStore_GetByUser_SortByAccessCount(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "u1", Content: "a", AccessCount: 10})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "u1", Content: "b", AccessCount: 5})
	_ = store.Add(ctx, &Memory{ID: "m3", UserID: "u1", Content: "c", AccessCount: 15})

	opts := &ListOptions{SortBy: "access_count", Order: "desc"}
	results, err := store.GetByUser(ctx, "u1", opts)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "m3", results[0].ID)
}

func TestInMemoryStore_GetByUser_SortByUpdatedAt(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "u1", Content: "a", UpdatedAt: now.Add(time.Hour)})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "u1", Content: "b", UpdatedAt: now.Add(-time.Hour)})
	_ = store.Add(ctx, &Memory{ID: "m3", UserID: "u1", Content: "c", UpdatedAt: now})

	opts := &ListOptions{SortBy: "updated_at", Order: "asc"}
	results, err := store.GetByUser(ctx, "u1", opts)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "m2", results[0].ID)
}

func TestInMemoryStore_GetByUser_UnknownSortField(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{ID: "m1", UserID: "u1", Content: "a", CreatedAt: now.Add(time.Hour)})
	_ = store.Add(ctx, &Memory{ID: "m2", UserID: "u1", Content: "b", CreatedAt: now.Add(-time.Hour)})

	opts := &ListOptions{SortBy: "unknown_field", Order: "asc"}
	results, err := store.GetByUser(ctx, "u1", opts)
	require.NoError(t, err)
	// Should default to created_at sort
	assert.Equal(t, "m2", results[0].ID)
}

// --- GetBySession ---

func TestInMemoryStore_GetBySession_ReturnsAll(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, &Memory{ID: "m1", SessionID: "sess1", CreatedAt: time.Now()})
	_ = store.Add(ctx, &Memory{ID: "m2", SessionID: "sess1", CreatedAt: time.Now().Add(time.Second)})
	_ = store.Add(ctx, &Memory{ID: "m3", SessionID: "sess2", CreatedAt: time.Now()})

	results, err := store.GetBySession(ctx, "sess1")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryStore_GetBySession_SortedByCreationTime(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Add(ctx, &Memory{ID: "m1", SessionID: "sess1", CreatedAt: now.Add(time.Hour)})
	_ = store.Add(ctx, &Memory{ID: "m2", SessionID: "sess1", CreatedAt: now})
	_ = store.Add(ctx, &Memory{ID: "m3", SessionID: "sess1", CreatedAt: now.Add(-time.Hour)})

	results, err := store.GetBySession(ctx, "sess1")
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.True(t, results[0].CreatedAt.Before(results[1].CreatedAt))
	assert.True(t, results[1].CreatedAt.Before(results[2].CreatedAt))
}

func TestInMemoryStore_GetBySession_EmptyForUnknownSession(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	results, err := store.GetBySession(ctx, "unknown-session")
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.NotNil(t, results) // Should return empty slice, not nil
}

// --- AddEntity ---

func TestInMemoryStore_AddEntity_Success(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	entity := &Entity{Name: "Test Entity", Type: "concept"}
	err := store.AddEntity(ctx, entity)
	require.NoError(t, err)
	assert.NotEmpty(t, entity.ID)
	assert.False(t, entity.CreatedAt.IsZero())
	assert.False(t, entity.UpdatedAt.IsZero())
}

func TestInMemoryStore_AddEntity_WithExistingID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	entity := &Entity{ID: "e-pre-set", Name: "Entity", Type: "person"}
	err := store.AddEntity(ctx, entity)
	require.NoError(t, err)
	assert.Equal(t, "e-pre-set", entity.ID)
}

func TestInMemoryStore_AddEntity_SetsTimestamps(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	before := time.Now()
	entity := &Entity{Name: "Test", Type: "concept"}
	err := store.AddEntity(ctx, entity)
	require.NoError(t, err)

	assert.True(t, entity.CreatedAt.After(before) || entity.CreatedAt.Equal(before))
	assert.True(t, entity.UpdatedAt.After(before) || entity.UpdatedAt.Equal(before))
}

func TestInMemoryStore_AddEntity_MultipleEntities(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := store.AddEntity(ctx, &Entity{
			Name: fmt.Sprintf("Entity %d", i),
			Type: "concept",
		})
		require.NoError(t, err)
	}

	assert.Equal(t, 5, store.entities.Len())
}

// --- GetEntity ---

func TestInMemoryStore_GetEntity_Success(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddEntity(ctx, &Entity{ID: "ge-e1", Name: "Test Entity", Type: "person"})

	entity, err := store.GetEntity(ctx, "ge-e1")
	require.NoError(t, err)
	assert.Equal(t, "ge-e1", entity.ID)
	assert.Equal(t, "Test Entity", entity.Name)
}

func TestInMemoryStore_GetEntity_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetEntity(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity not found")
}

// --- SearchEntities ---

func TestInMemoryStore_SearchEntities_ByName(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddEntity(ctx, &Entity{ID: "e1", Name: "Alice Johnson"})
	_ = store.AddEntity(ctx, &Entity{ID: "e2", Name: "Bob Smith"})
	_ = store.AddEntity(ctx, &Entity{ID: "e3", Name: "Alice Cooper"})

	results, err := store.SearchEntities(ctx, "alice", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryStore_SearchEntities_CaseInsensitive(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddEntity(ctx, &Entity{ID: "e1", Name: "Test Entity"})

	results, err := store.SearchEntities(ctx, "TEST", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestInMemoryStore_SearchEntities_WithLimit(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_ = store.AddEntity(ctx, &Entity{
			Name: fmt.Sprintf("Entity %d", i),
		})
	}

	results, err := store.SearchEntities(ctx, "entity", 3)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 3)
}

func TestInMemoryStore_SearchEntities_ZeroLimit(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = store.AddEntity(ctx, &Entity{Name: fmt.Sprintf("Entity %d", i)})
	}

	// limit=0 means no limit
	results, err := store.SearchEntities(ctx, "entity", 0)
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestInMemoryStore_SearchEntities_NoResults(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddEntity(ctx, &Entity{ID: "e1", Name: "Alice"})

	results, err := store.SearchEntities(ctx, "zzyzzy", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- AddRelationship ---

func TestInMemoryStore_AddRelationship_Success(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rel := &Relationship{
		SourceID: "e1",
		TargetID: "e2",
		Type:     "knows",
		Strength: 0.8,
	}
	err := store.AddRelationship(ctx, rel)
	require.NoError(t, err)
	assert.NotEmpty(t, rel.ID)
	assert.False(t, rel.CreatedAt.IsZero())
	assert.False(t, rel.UpdatedAt.IsZero())
}

func TestInMemoryStore_AddRelationship_WithExistingID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rel := &Relationship{ID: "r-pre-set", SourceID: "e1", TargetID: "e2"}
	err := store.AddRelationship(ctx, rel)
	require.NoError(t, err)
	assert.Equal(t, "r-pre-set", rel.ID)
}

func TestInMemoryStore_AddRelationship_UpdatesEntityIndex(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rel := &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2"}
	err := store.AddRelationship(ctx, rel)
	require.NoError(t, err)

	// Both source and target should be in the entity index
	e1, _ := store.entityIndex.Get("e1")
	e2, _ := store.entityIndex.Get("e2")
	assert.Contains(t, e1, "r1")
	assert.Contains(t, e2, "r1")
}

func TestInMemoryStore_AddRelationship_MultipleRelationships(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddRelationship(ctx, &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2"})
	_ = store.AddRelationship(ctx, &Relationship{ID: "r2", SourceID: "e1", TargetID: "e3"})
	_ = store.AddRelationship(ctx, &Relationship{ID: "r3", SourceID: "e2", TargetID: "e3"})

	assert.Equal(t, 3, store.relationships.Len())
	e1, _ := store.entityIndex.Get("e1")
	e2, _ := store.entityIndex.Get("e2")
	e3, _ := store.entityIndex.Get("e3")
	assert.Len(t, e1, 2) // source of r1 and r2
	assert.Len(t, e2, 2) // target of r1, source of r3
	assert.Len(t, e3, 2) // target of r2 and r3
}

// --- GetRelationships ---

func TestInMemoryStore_GetRelationships_ForEntity(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddRelationship(ctx, &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2", Type: "knows"})
	_ = store.AddRelationship(ctx, &Relationship{ID: "r2", SourceID: "e1", TargetID: "e3", Type: "works_at"})
	_ = store.AddRelationship(ctx, &Relationship{ID: "r3", SourceID: "e4", TargetID: "e5", Type: "owns"})

	results, err := store.GetRelationships(ctx, "e1")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryStore_GetRelationships_AsTarget(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.AddRelationship(ctx, &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2"})
	_ = store.AddRelationship(ctx, &Relationship{ID: "r2", SourceID: "e3", TargetID: "e2"})

	results, err := store.GetRelationships(ctx, "e2")
	require.NoError(t, err)
	assert.Len(t, results, 2) // e2 is target in both
}

func TestInMemoryStore_GetRelationships_Empty(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	results, err := store.GetRelationships(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- calculateMatchScore ---

func TestInMemoryStore_CalculateMatchScore_TableDriven(t *testing.T) {
	store := NewInMemoryStore()

	tests := []struct {
		name     string
		query    string
		content  string
		expected float64
	}{
		{"FullMatch_SingleWord", "go", "Go programming", 1.0},
		{"FullMatch_MultiWord", "go programming", "Go Programming Language", 1.0},
		{"PartialMatch", "go python", "Go programming language", 0.5},
		{"NoMatch", "rust java", "Go programming language", 0.0},
		{"EmptyQuery", "", "Go programming language", 0.0},
		{"AlreadyLowered", "go", "Go Programming", 1.0},
		{"SubstringMatch", "program", "Go programming", 1.0},
		{"MultiWordPartial", "go python rust", "Go programming language", float64(1) / float64(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mem := &Memory{Content: tt.content}
			score := store.calculateMatchScore(tt.query, mem)
			assert.InDelta(t, tt.expected, score, 0.01)
		})
	}
}

// --- sortMemories ---

func TestInMemoryStore_SortMemories_AllFields(t *testing.T) {
	store := NewInMemoryStore()
	now := time.Now()

	memories := []*Memory{
		{ID: "a", CreatedAt: now, UpdatedAt: now.Add(time.Hour), Importance: 0.3, AccessCount: 5},
		{ID: "b", CreatedAt: now.Add(-time.Hour), UpdatedAt: now, Importance: 0.9, AccessCount: 1},
		{ID: "c", CreatedAt: now.Add(time.Hour), UpdatedAt: now.Add(-time.Hour), Importance: 0.6, AccessCount: 10},
	}

	tests := []struct {
		name    string
		sortBy  string
		order   string
		firstID string
	}{
		{"CreatedAt_Asc", "created_at", "asc", "b"},
		{"CreatedAt_Desc", "created_at", "desc", "c"},
		{"UpdatedAt_Asc", "updated_at", "asc", "c"},
		{"UpdatedAt_Desc", "updated_at", "desc", "a"},
		{"Importance_Asc", "importance", "asc", "a"},
		{"Importance_Desc", "importance", "desc", "b"},
		{"AccessCount_Asc", "access_count", "asc", "b"},
		{"AccessCount_Desc", "access_count", "desc", "c"},
		{"Default_Asc", "unknown", "asc", "b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mems := make([]*Memory, len(memories))
			copy(mems, memories)
			store.sortMemories(mems, tt.sortBy, tt.order)
			assert.Equal(t, tt.firstID, mems[0].ID)
		})
	}
}

// --- removeFromSlice ---

func TestRemoveFromSlice_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected []string
	}{
		{"RemoveFirst", []string{"a", "b", "c"}, "a", []string{"b", "c"}},
		{"RemoveMiddle", []string{"a", "b", "c"}, "b", []string{"a", "c"}},
		{"RemoveLast", []string{"a", "b", "c"}, "c", []string{"a", "b"}},
		{"ItemNotFound", []string{"a", "b", "c"}, "d", []string{"a", "b", "c"}},
		{"EmptySlice", []string{}, "a", []string{}},
		{"SingleElement_Remove", []string{"a"}, "a", []string{}},
		{"SingleElement_NoMatch", []string{"a"}, "b", []string{"a"}},
		{"Duplicates_RemovesFirst", []string{"a", "b", "a"}, "a", []string{"b", "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeFromSlice(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Concurrent access tests ---

func TestInMemoryStore_ConcurrentAdd(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := store.Add(ctx, &Memory{
				ID:      fmt.Sprintf("concurrent-m%d", idx),
				UserID:  "user1",
				Content: fmt.Sprintf("Memory %d", idx),
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 100, store.memories.Len())
}

func TestInMemoryStore_ConcurrentGetAndAdd(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate some memories
	for i := 0; i < 50; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("pre-m%d", i),
			UserID:  "user1",
			Content: fmt.Sprintf("Pre-existing memory %d", i),
		})
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = store.Get(ctx, fmt.Sprintf("pre-m%d", idx))
		}(i)
	}

	// Concurrent adds
	for i := 50; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = store.Add(ctx, &Memory{
				ID:      fmt.Sprintf("new-m%d", idx),
				UserID:  "user1",
				Content: fmt.Sprintf("New memory %d", idx),
			})
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 100, store.memories.Len())
}

func TestInMemoryStore_ConcurrentSearch(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 20; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("search-m%d", i),
			UserID:  "user1",
			Content: fmt.Sprintf("Go programming tutorial %d", i),
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := &SearchOptions{MinScore: 0, TopK: 5}
			results, err := store.Search(ctx, "programming", opts)
			assert.NoError(t, err)
			assert.NotEmpty(t, results)
		}()
	}

	wg.Wait()
}

func TestInMemoryStore_ConcurrentEntityOperations(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent entity adds
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := store.AddEntity(ctx, &Entity{
				Name: fmt.Sprintf("Entity %d", idx),
				Type: "concept",
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 50, store.entities.Len())
}

func TestInMemoryStore_ConcurrentRelationshipOperations(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent relationship adds
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := store.AddRelationship(ctx, &Relationship{
				SourceID: fmt.Sprintf("e%d", idx),
				TargetID: fmt.Sprintf("e%d", idx+1),
				Type:     "related",
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 50, store.relationships.Len())
}

func TestInMemoryStore_ConcurrentDeleteAndGet(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 100; i++ {
		_ = store.Add(ctx, &Memory{
			ID:        fmt.Sprintf("cd-m%d", i),
			UserID:    "user1",
			SessionID: "sess1",
			Content:   fmt.Sprintf("Memory %d", i),
		})
	}

	var wg sync.WaitGroup

	// Delete first 50 concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = store.Delete(ctx, fmt.Sprintf("cd-m%d", idx))
		}(i)
	}

	// Get last 50 concurrently
	for i := 50; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mem, err := store.Get(ctx, fmt.Sprintf("cd-m%d", idx))
			if err == nil {
				assert.NotNil(t, mem)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 50, store.memories.Len())
}

func TestInMemoryStore_ConcurrentUpdateAndSearch(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 20; i++ {
		_ = store.Add(ctx, &Memory{
			ID:      fmt.Sprintf("cus-m%d", i),
			UserID:  "user1",
			Content: fmt.Sprintf("Go programming tutorial %d", i),
		})
	}

	var wg sync.WaitGroup

	// Concurrent updates
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = store.Update(ctx, &Memory{
				ID:      fmt.Sprintf("cus-m%d", idx),
				Content: fmt.Sprintf("Updated Go programming tutorial %d", idx),
			})
		}(i)
	}

	// Concurrent searches
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Search(ctx, "programming", &SearchOptions{MinScore: 0})
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}
