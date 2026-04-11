// Package kiro provides Kiro CLI agent integration for HelixAgent.
package kiro

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProjectMemory provides persistent project memory with semantic search.
// Ported from Kiro's memory implementation
type ProjectMemory struct {
	db        *pgxpool.Pool
	projectID string

	// Embedding generator
	embedder EmbeddingGenerator

	// Local cache for hot data
	shortTerm map[string]*MemoryEntry
}

// MemoryEntry represents a single memory.
type MemoryEntry struct {
	Key        string
	Value      interface{}
	Importance float64   // 0.0-1.0
	Embedding  []float32 // For semantic search
	Tags       []string
	Timestamp  time.Time
}

// EmbeddingGenerator generates embeddings for text.
type EmbeddingGenerator interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewProjectMemory creates a new ProjectMemory.
func NewProjectMemory(db *pgxpool.Pool, projectID string, embedder EmbeddingGenerator) *ProjectMemory {
	return &ProjectMemory{
		db:        db,
		projectID: projectID,
		embedder:  embedder,
		shortTerm: make(map[string]*MemoryEntry),
	}
}

// Remember stores a memory.
func (pm *ProjectMemory) Remember(
	ctx context.Context,
	key string,
	value interface{},
	importance float64,
	tags []string,
) error {
	entry := &MemoryEntry{
		Key:        key,
		Value:      value,
		Importance: importance,
		Tags:       tags,
		Timestamp:  time.Now(),
	}

	// Store in short-term memory
	pm.shortTerm[key] = entry

	// For important memories, persist to database
	if importance >= 0.3 {
		// Generate embedding
		valueJSON, _ := json.Marshal(value)
		embedding, err := pm.embedder.Embed(ctx, string(valueJSON))
		if err == nil {
			entry.Embedding = embedding
		}

		// Persist to database
		_, err = pm.db.Exec(ctx, `
			INSERT INTO project_memory (
				project_id, key, value, importance, embedding, tags, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (project_id, key) DO UPDATE
			SET value = EXCLUDED.value,
			    importance = EXCLUDED.importance,
			    embedding = EXCLUDED.embedding,
			    tags = EXCLUDED.tags,
			    updated_at = NOW()`,
			pm.projectID, key, valueJSON, importance, embedding, tags, entry.Timestamp,
		)

		if err != nil {
			return fmt.Errorf("persist memory: %w", err)
		}
	}

	return nil
}

// Recall retrieves a memory by key.
func (pm *ProjectMemory) Recall(ctx context.Context, key string) (interface{}, error) {
	// Check short-term memory first
	if entry, ok := pm.shortTerm[key]; ok {
		return entry.Value, nil
	}

	// Query database
	var valueJSON []byte
	err := pm.db.QueryRow(ctx, `
		SELECT value FROM project_memory
		WHERE project_id = $1 AND key = $2
	`, pm.projectID, key).Scan(&valueJSON)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(valueJSON, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// Search performs semantic search over memories.
func (pm *ProjectMemory) Search(ctx context.Context, query string, topK int) ([]*MemoryEntry, error) {
	if pm.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}

	// Generate query embedding
	queryEmb, err := pm.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Semantic search using pgvector cosine similarity
	rows, err := pm.db.Query(ctx, `
		SELECT key, value, importance, tags, created_at,
		       1 - (embedding <=> $3) as similarity
		FROM project_memory
		WHERE project_id = $1
		ORDER BY embedding <=> $3
		LIMIT $2
	`, pm.projectID, topK, queryEmb)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		var valueJSON []byte
		var similarity float64

		if err := rows.Scan(&entry.Key, &valueJSON, &entry.Importance,
			&entry.Tags, &entry.Timestamp, &similarity); err != nil {
			continue
		}

		if err := json.Unmarshal(valueJSON, &entry.Value); err != nil {
			continue
		}

		results = append(results, &entry)
	}

	return results, rows.Err()
}

// SearchByTags finds memories by tags.
func (pm *ProjectMemory) SearchByTags(ctx context.Context, tags []string) ([]*MemoryEntry, error) {
	rows, err := pm.db.Query(ctx, `
		SELECT key, value, importance, tags, created_at
		FROM project_memory
		WHERE project_id = $1 AND tags && $2
		ORDER BY importance DESC, created_at DESC
	`, pm.projectID, tags)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		var valueJSON []byte

		if err := rows.Scan(&entry.Key, &valueJSON, &entry.Importance,
			&entry.Tags, &entry.Timestamp); err != nil {
			continue
		}

		json.Unmarshal(valueJSON, &entry.Value)
		results = append(results, &entry)
	}

	return results, rows.Err()
}

// Delete removes a memory.
func (pm *ProjectMemory) Delete(ctx context.Context, key string) error {
	delete(pm.shortTerm, key)

	_, err := pm.db.Exec(ctx,
		"DELETE FROM project_memory WHERE project_id = $1 AND key = $2",
		pm.projectID, key,
	)

	return err
}

// GetRecent returns recent memories.
func (pm *ProjectMemory) GetRecent(ctx context.Context, limit int) ([]*MemoryEntry, error) {
	rows, err := pm.db.Query(ctx, `
		SELECT key, value, importance, tags, created_at
		FROM project_memory
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, pm.projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*MemoryEntry
	for rows.Next() {
		var entry MemoryEntry
		var valueJSON []byte

		if err := rows.Scan(&entry.Key, &valueJSON, &entry.Importance,
			&entry.Tags, &entry.Timestamp); err != nil {
			continue
		}

		json.Unmarshal(valueJSON, &entry.Value)
		results = append(results, &entry)
	}

	return results, rows.Err()
}

// Summarize returns a summary of project memory.
func (pm *ProjectMemory) Summarize(ctx context.Context) (*MemorySummary, error) {
	var summary MemorySummary

	err := pm.db.QueryRow(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours'),
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '7 days'),
			AVG(importance)
		FROM project_memory
		WHERE project_id = $1
	`, pm.projectID).Scan(
		&summary.TotalMemories,
		&summary.Last24Hours,
		&summary.Last7Days,
		&summary.AvgImportance,
	)

	if err != nil {
		return nil, err
	}

	return &summary, nil
}

// MemorySummary contains memory statistics.
type MemorySummary struct {
	TotalMemories int
	Last24Hours   int
	Last7Days     int
	AvgImportance float64
}

// SimpleEmbedder is a simple embedding generator for testing.
type SimpleEmbedder struct{}

// Embed generates a simple embedding (for testing).
func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Simple bag-of-words embedding
	words := make(map[string]int)
	for _, word := range splitWords(text) {
		words[word]++
	}

	// Create a simple vector
	embedding := make([]float32, 100)
	for word, count := range words {
		hash := hashString(word) % 100
		embedding[hash] = float32(count)
	}

	// Normalize
	var sum float32
	for _, v := range embedding {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(1.0 / float64(sum))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding, nil
}

func splitWords(text string) []string {
	// Simple word splitting
	var words []string
	var current []rune

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			current = append(current, r)
		} else if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

func hashString(s string) int {
	// Simple hash function
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
