package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTempDiskStore opens a DiskStore at a fresh temp-file path and registers
// cleanup. The temp dir is auto-removed by the testing framework.
func newTempDiskStore(t *testing.T) (*DiskStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := NewDiskStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

// --- ANTI-BLUFF DURABILITY PROOF ---
//
// This is the load-bearing test: it writes records, CLOSES the store, opens a
// BRAND-NEW DiskStore on the SAME path, and asserts every record reads back.
// An in-memory shortcut implementation would lose all data here. A PASS here is
// positive runtime proof the store is genuinely on disk and survives reopen.
func TestDiskStore_Durability_SurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "durable.db")

	// --- Session 1: write through one store, then close it entirely ---
	store1, err := NewDiskStore(path)
	require.NoError(t, err)

	mem := &Memory{
		ID:         "durable-mem-1",
		UserID:     "user-durable",
		SessionID:  "sess-durable",
		Content:    "the capital of France is Paris",
		Summary:    "geography fact",
		Type:       MemoryTypeSemantic,
		Category:   "geo",
		Metadata:   map[string]interface{}{"source": "test", "confidence": 0.9},
		Embedding:  []float32{0.1, 0.2, 0.3},
		Importance: 0.75,
	}
	mem.CreatedAt = time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store1.Add(ctx, mem))

	ent := &Entity{ID: "durable-ent-1", Name: "Paris", Type: "place",
		Properties: map[string]interface{}{"country": "France"}, Aliases: []string{"City of Light"}}
	require.NoError(t, store1.AddEntity(ctx, ent))

	rel := &Relationship{ID: "durable-rel-1", SourceID: "durable-ent-1", TargetID: "durable-ent-2",
		Type: "capital_of", Strength: 0.95, Properties: map[string]interface{}{"verified": true}}
	require.NoError(t, store1.AddRelationship(ctx, rel))

	// Close folds WAL into the main file — the process "ends" for this store.
	require.NoError(t, store1.Close())

	// --- Session 2: a FRESH store on the SAME path. No shared in-memory state. ---
	store2, err := NewDiskStore(path)
	require.NoError(t, err)
	defer func() { _ = store2.Close() }()

	gotMem, err := store2.Get(ctx, "durable-mem-1")
	require.NoError(t, err, "memory MUST survive store close+reopen — this is the durability proof")
	assert.Equal(t, "the capital of France is Paris", gotMem.Content)
	assert.Equal(t, "user-durable", gotMem.UserID)
	assert.Equal(t, MemoryTypeSemantic, gotMem.Type)
	assert.Equal(t, "geo", gotMem.Category)
	assert.InDelta(t, 0.75, gotMem.Importance, 0.0001)
	require.NotNil(t, gotMem.Metadata)
	assert.Equal(t, "test", gotMem.Metadata["source"])
	require.Len(t, gotMem.Embedding, 3)
	assert.InDelta(t, float32(0.2), gotMem.Embedding[1], 0.0001)
	assert.False(t, gotMem.CreatedAt.IsZero(), "created_at MUST round-trip across reopen")

	gotEnt, err := store2.GetEntity(ctx, "durable-ent-1")
	require.NoError(t, err, "entity MUST survive reopen")
	assert.Equal(t, "Paris", gotEnt.Name)
	assert.Equal(t, "France", gotEnt.Properties["country"])
	require.Len(t, gotEnt.Aliases, 1)
	assert.Equal(t, "City of Light", gotEnt.Aliases[0])

	rels, err := store2.GetRelationships(ctx, "durable-ent-1")
	require.NoError(t, err, "relationship MUST survive reopen")
	require.Len(t, rels, 1)
	assert.Equal(t, "capital_of", rels[0].Type)
	assert.InDelta(t, 0.95, rels[0].Strength, 0.0001)
	assert.Equal(t, true, rels[0].Properties["verified"])
}

// --- NewDiskStore ---

func TestNewDiskStore_EmptyPathRejected(t *testing.T) {
	store, err := NewDiskStore("   ")
	require.Error(t, err)
	assert.Nil(t, store)
}

func TestNewDiskStore_CreatesUsableStore(t *testing.T) {
	store, _ := newTempDiskStore(t)
	require.NotNil(t, store)
	// A no-op query proves the schema is live.
	_, err := store.GetByUser(context.Background(), "nobody", nil)
	require.NoError(t, err)
}

// --- Add / Get matrix ---

func TestDiskStore_Add_GeneratesIDWhenMissing(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	mem := &Memory{Content: "no id supplied", Type: MemoryTypeEpisodic}
	require.NoError(t, store.Add(ctx, mem))
	assert.NotEmpty(t, mem.ID, "Add must populate a generated UUID")

	got, err := store.Get(ctx, mem.ID)
	require.NoError(t, err)
	assert.Equal(t, "no id supplied", got.Content)
}

func TestDiskStore_Get_NotFound(t *testing.T) {
	store, _ := newTempDiskStore(t)
	got, err := store.Get(context.Background(), "does-not-exist")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiskStore_Get_IncrementsAccessCount(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, &Memory{ID: "acc-1", Content: "x", Type: MemoryTypeWorking}))

	g1, err := store.Get(ctx, "acc-1")
	require.NoError(t, err)
	assert.Equal(t, 1, g1.AccessCount)

	g2, err := store.Get(ctx, "acc-1")
	require.NoError(t, err)
	assert.Equal(t, 2, g2.AccessCount, "access count must persist+increment across Gets")
}

func TestDiskStore_Update(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, &Memory{ID: "upd-1", Content: "before", Type: MemoryTypeSemantic}))

	require.NoError(t, store.Update(ctx, &Memory{ID: "upd-1", Content: "after", Type: MemoryTypeSemantic}))
	got, err := store.Get(ctx, "upd-1")
	require.NoError(t, err)
	assert.Equal(t, "after", got.Content)
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestDiskStore_Update_NotFound(t *testing.T) {
	store, _ := newTempDiskStore(t)
	err := store.Update(context.Background(), &Memory{ID: "ghost", Content: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiskStore_Delete(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, &Memory{ID: "del-1", Content: "x", Type: MemoryTypeSemantic}))
	require.NoError(t, store.Delete(ctx, "del-1"))

	_, err := store.Get(ctx, "del-1")
	require.Error(t, err)
}

func TestDiskStore_Delete_NotFound(t *testing.T) {
	store, _ := newTempDiskStore(t)
	err := store.Delete(context.Background(), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Search matrix ---

func TestDiskStore_Search_TextMatch(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, &Memory{ID: "s1", UserID: "u", Content: "golang concurrency patterns", Type: MemoryTypeSemantic}))
	require.NoError(t, store.Add(ctx, &Memory{ID: "s2", UserID: "u", Content: "python data science", Type: MemoryTypeSemantic}))

	opts := DefaultSearchOptions()
	opts.MinScore = 0.5
	res, err := store.Search(ctx, "golang concurrency", opts)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "s1", res[0].ID)
}

func TestDiskStore_Search_UserFilter(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, &Memory{ID: "f1", UserID: "alice", Content: "shared topic", Type: MemoryTypeSemantic}))
	require.NoError(t, store.Add(ctx, &Memory{ID: "f2", UserID: "bob", Content: "shared topic", Type: MemoryTypeSemantic}))

	opts := DefaultSearchOptions()
	opts.UserID = "alice"
	opts.MinScore = 0.5
	res, err := store.Search(ctx, "shared topic", opts)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "f1", res[0].ID)
}

func TestDiskStore_Search_TopKLimit(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	for _, id := range []string{"k1", "k2", "k3"} {
		require.NoError(t, store.Add(ctx, &Memory{ID: id, Content: "common keyword here", Type: MemoryTypeSemantic}))
	}
	opts := DefaultSearchOptions()
	opts.MinScore = 0.0
	opts.TopK = 2
	res, err := store.Search(ctx, "common keyword", opts)
	require.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestDiskStore_GetByUser_SortAndPaginate(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	base := time.Now().UTC()
	for i, id := range []string{"p1", "p2", "p3"} {
		m := &Memory{ID: id, UserID: "pag", Content: id, Type: MemoryTypeSemantic}
		m.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, store.Add(ctx, m))
	}
	res, err := store.GetByUser(ctx, "pag", &ListOptions{SortBy: "created_at", Order: "desc", Limit: 2})
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, "p3", res[0].ID, "desc by created_at puts newest first")
	assert.Equal(t, "p2", res[1].ID)
}

func TestDiskStore_GetBySession_OrderedByCreation(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	base := time.Now().UTC()
	m2 := &Memory{ID: "sess-b", SessionID: "S", Content: "second", Type: MemoryTypeEpisodic}
	m2.CreatedAt = base.Add(2 * time.Minute)
	m1 := &Memory{ID: "sess-a", SessionID: "S", Content: "first", Type: MemoryTypeEpisodic}
	m1.CreatedAt = base.Add(1 * time.Minute)
	require.NoError(t, store.Add(ctx, m2))
	require.NoError(t, store.Add(ctx, m1))

	res, err := store.GetBySession(ctx, "S")
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, "sess-a", res[0].ID, "earliest created first")
	assert.Equal(t, "sess-b", res[1].ID)
}

// --- Entity / Relationship matrix ---

func TestDiskStore_Entity_AddGetSearch(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	e := &Entity{Name: "Ada Lovelace", Type: "person"}
	require.NoError(t, store.AddEntity(ctx, e))
	assert.NotEmpty(t, e.ID)
	assert.False(t, e.CreatedAt.IsZero())

	got, err := store.GetEntity(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", got.Name)

	found, err := store.SearchEntities(ctx, "ada", 10)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, e.ID, found[0].ID)
}

func TestDiskStore_GetEntity_NotFound(t *testing.T) {
	store, _ := newTempDiskStore(t)
	_, err := store.GetEntity(context.Background(), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiskStore_Relationship_AddAndQueryBothDirections(t *testing.T) {
	store, _ := newTempDiskStore(t)
	ctx := context.Background()
	require.NoError(t, store.AddRelationship(ctx, &Relationship{SourceID: "A", TargetID: "B", Type: "knows", Strength: 0.5}))

	// Reachable from the source...
	src, err := store.GetRelationships(ctx, "A")
	require.NoError(t, err)
	require.Len(t, src, 1)
	assert.Equal(t, "knows", src[0].Type)

	// ...and from the target.
	tgt, err := store.GetRelationships(ctx, "B")
	require.NoError(t, err)
	require.Len(t, tgt, 1)
}

// satisfies-interface sanity (compile + runtime).
func TestDiskStore_SatisfiesMemoryStore(t *testing.T) {
	store, _ := newTempDiskStore(t)
	var ms MemoryStore = store
	require.NotNil(t, ms)
}
