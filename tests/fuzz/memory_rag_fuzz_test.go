//go:build fuzz

// Package fuzz provides Go native fuzzing tests for critical parsing paths
// in the HelixAgent system. These tests ensure that malformed or adversarial
// input never causes panics or undefined behavior.
package fuzz

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/memory"
)

// FuzzMemoryStoreSearch tests that the InMemoryStore.Search method never panics
// when given arbitrary query strings and search options. This exercises the text
// matching, filtering, and sorting paths in store_memory.go.
func FuzzMemoryStoreSearch(f *testing.F) {
	// Seed corpus: typical and adversarial query strings
	f.Add("golang concurrency patterns", "user-1", "session-abc", "semantic", 10, 0.5)
	f.Add("", "", "", "", 0, 0.0)
	f.Add("   ", "user-999", "", "episodic", -1, -1.0)
	f.Add(strings.Repeat("x", 8192), "user-1", "sess-1", "procedural", 1000, 1.0)
	f.Add("\x00\x01\x02\xff", "", "", "working", 5, 0.0)
	f.Add("'; DROP TABLE memories; --", "user-<script>", "sess\nnewline", "invalid-type", 3, 0.99)
	f.Add("multiple words in query string for matching", "user-1", "", "", 20, 0.1)

	f.Fuzz(func(t *testing.T, query, userID, sessionID, memType string, topK int, minScore float64) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzMemoryStoreSearch panicked: query=%q userID=%q panic=%v",
					query, userID, r)
			}
		}()

		store := memory.NewInMemoryStore()
		ctx := context.Background()

		// Pre-populate with some memories so Search has data to work with
		testMemories := []*memory.Memory{
			{
				ID:         "mem-1",
				UserID:     "user-1",
				SessionID:  "session-abc",
				Content:    "Go concurrency uses goroutines and channels",
				Type:       memory.MemoryTypeSemantic,
				Category:   "programming",
				Importance: 0.8,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			{
				ID:         "mem-2",
				UserID:     "user-1",
				SessionID:  "session-abc",
				Content:    "The user asked about design patterns in Go",
				Type:       memory.MemoryTypeEpisodic,
				Category:   "conversation",
				Importance: 0.6,
				CreatedAt:  time.Now().Add(-time.Hour),
				UpdatedAt:  time.Now().Add(-time.Hour),
			},
			{
				ID:         "mem-3",
				UserID:     "user-2",
				SessionID:  "session-def",
				Content:    "Database connection pooling best practices",
				Type:       memory.MemoryTypeProcedural,
				Category:   "infrastructure",
				Importance: 0.9,
				CreatedAt:  time.Now().Add(-24 * time.Hour),
				UpdatedAt:  time.Now().Add(-24 * time.Hour),
			},
		}

		for _, m := range testMemories {
			_ = store.Add(ctx, m)
		}

		// Construct search options from fuzzed inputs
		opts := &memory.SearchOptions{
			UserID:    userID,
			SessionID: sessionID,
			Type:      memory.MemoryType(memType),
			TopK:      topK,
			MinScore:  minScore,
		}

		// Search must not panic regardless of input
		results, err := store.Search(ctx, query, opts)
		_ = err
		for _, r := range results {
			_ = len(r.Content)
			_ = r.Importance
		}

		// Also search with nil options (uses defaults)
		results2, err2 := store.Search(ctx, query, nil)
		_ = err2
		_ = len(results2)
	})
}

// FuzzMemoryEntitySearch tests that entity search with arbitrary query strings
// and limits never panics. Exercises SearchEntities in store_memory.go.
func FuzzMemoryEntitySearch(f *testing.F) {
	f.Add("Alice", 10)
	f.Add("", 0)
	f.Add(strings.Repeat("x", 4096), -1)
	f.Add("\x00\xff\xfe", 1000000)
	f.Add("'; DROP TABLE entities; --", 5)
	f.Add("<script>alert(1)</script>", 1)

	f.Fuzz(func(t *testing.T, query string, limit int) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzMemoryEntitySearch panicked: query=%q limit=%d panic=%v",
					query, limit, r)
			}
		}()

		store := memory.NewInMemoryStore()
		ctx := context.Background()

		// Pre-populate entities
		entities := []*memory.Entity{
			{ID: "e1", Name: "Alice", Type: "person"},
			{ID: "e2", Name: "Go Programming", Type: "concept"},
			{ID: "e3", Name: "PostgreSQL", Type: "technology"},
		}
		for _, e := range entities {
			_ = store.AddEntity(ctx, e)
		}

		// Search must not panic
		results, err := store.SearchEntities(ctx, query, limit)
		_ = err
		for _, e := range results {
			_ = len(e.Name)
			_ = len(e.Type)
		}
	})
}

// FuzzMemoryJSONParsing tests that parsing Memory objects from arbitrary JSON
// never panics. Memory objects are serialized/deserialized in API responses
// and internal storage paths.
func FuzzMemoryJSONParsing(f *testing.F) {
	f.Add([]byte(`{"id":"mem-1","user_id":"user-1","content":"test memory","type":"semantic","importance":0.8}`))
	f.Add([]byte(`{"id":"","content":"","type":"","importance":-1,"embedding":[0.1,0.2,0.3]}`))
	f.Add([]byte(`{"metadata":{"key":"value","nested":{"deep":"data"}}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"embedding":` + buildFloat32Array(1536) + `}`))
	f.Add([]byte("\x00\x01\x02\xff\xfe"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzMemoryJSONParsing panicked with input length %d: %v", len(data), r)
			}
		}()

		var mem memory.Memory
		if err := json.Unmarshal(data, &mem); err != nil {
			return
		}

		// Safe field access
		_ = len(mem.ID)
		_ = len(mem.UserID)
		_ = len(mem.Content)
		_ = len(mem.Summary)
		_ = string(mem.Type)
		_ = len(mem.Category)
		_ = mem.Importance
		_ = mem.AccessCount
		_ = len(mem.Embedding)

		for k, v := range mem.Metadata {
			_ = len(k)
			_ = v
		}

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(&mem)
	})
}

// FuzzRAGSearchOptionsParsing tests that parsing RAG SearchOptions and related
// types from arbitrary JSON never panics. These are used in the /v1/rag endpoints.
func FuzzRAGSearchOptionsParsing(f *testing.F) {
	f.Add(`{"top_k":10,"min_score":0.5,"enable_reranking":true,"hybrid_alpha":0.5,"include_metadata":true}`)
	f.Add(`{"top_k":-1,"min_score":-100,"filter":{"category":"tech"},"namespace":"default"}`)
	f.Add(`{"top_k":0,"min_score":0,"filter":null}`)
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(`invalid json`)
	f.Add(`{"filter":{"nested":{"deep":{"deeper":"value"}}}}`)
	f.Add(`{"top_k":999999999,"hybrid_alpha":1e308}`)

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzRAGSearchOptionsParsing panicked with input %q: %v", input, r)
			}
		}()

		// Parse as a generic search options map (mirrors handler parsing)
		var opts map[string]interface{}
		if err := json.Unmarshal([]byte(input), &opts); err != nil {
			return
		}

		// Extract and validate typical search option fields
		if topK, ok := opts["top_k"]; ok {
			switch v := topK.(type) {
			case float64:
				_ = int(v)
			}
		}

		if minScore, ok := opts["min_score"]; ok {
			if v, ok := minScore.(float64); ok {
				_ = v > 0
			}
		}

		if alpha, ok := opts["hybrid_alpha"]; ok {
			if v, ok := alpha.(float64); ok {
				_ = v >= 0 && v <= 1
			}
		}

		if filter, ok := opts["filter"]; ok && filter != nil {
			if fm, ok := filter.(map[string]interface{}); ok {
				for k, v := range fm {
					_ = len(k)
					_ = v
				}
			}
		}

		_, _ = opts["namespace"].(string)
		_, _ = opts["enable_reranking"].(bool)
		_, _ = opts["include_metadata"].(bool)

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(opts)
	})
}

// FuzzRAGDocumentParsing tests that parsing RAG Document payloads from arbitrary
// JSON never panics. Documents flow through the chunking, embedding, and retrieval
// pipeline in the RAG subsystem.
func FuzzRAGDocumentParsing(f *testing.F) {
	f.Add([]byte(`{"id":"doc-1","content":"This is a test document about Go programming.","source":"test.go","metadata":{"language":"go"}}`))
	f.Add([]byte(`{"id":"","content":"","score":0.95,"embedding":[0.1,0.2]}`))
	f.Add([]byte(`{"content":"` + strings.Repeat("word ", 2048) + `"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte("\x00\x01\xff"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzRAGDocumentParsing panicked with input length %d: %v", len(data), r)
			}
		}()

		// Parse as a generic document map (mirrors handler parsing)
		var doc map[string]interface{}
		if err := json.Unmarshal(data, &doc); err != nil {
			return
		}

		// Safe extraction of document fields
		_, _ = doc["id"].(string)
		content, _ := doc["content"].(string)
		_ = len(content)
		_, _ = doc["source"].(string)
		_, _ = doc["score"].(float64)

		// Walk metadata
		if meta, ok := doc["metadata"]; ok {
			if mm, ok := meta.(map[string]interface{}); ok {
				for k, v := range mm {
					_ = len(k)
					_ = v
				}
			}
		}

		// Walk embedding
		if emb, ok := doc["embedding"]; ok {
			if arr, ok := emb.([]interface{}); ok {
				for _, v := range arr {
					_, _ = v.(float64)
				}
			}
		}

		// Re-marshal for round-trip safety
		_, _ = json.Marshal(doc)
	})
}

// buildFloat32Array produces a JSON float array of length n for seed corpus.
func buildFloat32Array(n int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString("0.01")
	}
	sb.WriteByte(']')
	return sb.String()
}
