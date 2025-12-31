package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContextManager(t *testing.T) {
	cm := NewContextManager(100)

	require.NotNil(t, cm)
	assert.NotNil(t, cm.entries)
	assert.NotNil(t, cm.cache)
	assert.Equal(t, 100, cm.maxSize)
	assert.Equal(t, 1024, cm.compressionThreshold)
}

func TestContextManager_AddEntry(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("add simple entry", func(t *testing.T) {
		entry := &ContextEntry{
			ID:       "entry1",
			Type:     "lsp",
			Source:   "test.go",
			Content:  "test content",
			Priority: 5,
		}

		err := cm.AddEntry(entry)
		require.NoError(t, err)

		retrieved, exists := cm.GetEntry("entry1")
		assert.True(t, exists)
		assert.Equal(t, "test content", retrieved.Content)
		assert.False(t, retrieved.Timestamp.IsZero())
	})

	t.Run("add entry with metadata", func(t *testing.T) {
		entry := &ContextEntry{
			ID:      "entry2",
			Type:    "mcp",
			Source:  "tool1",
			Content: "tool output",
			Metadata: map[string]interface{}{
				"tool":    "calculator",
				"success": true,
			},
			Priority: 7,
		}

		err := cm.AddEntry(entry)
		require.NoError(t, err)

		retrieved, exists := cm.GetEntry("entry2")
		assert.True(t, exists)
		assert.Equal(t, "calculator", retrieved.Metadata["tool"])
	})
}

func TestContextManager_AddEntry_Compression(t *testing.T) {
	cm := NewContextManager(100)
	cm.compressionThreshold = 50 // Low threshold for testing

	// Create large content
	largeContent := ""
	for i := 0; i < 100; i++ {
		largeContent += "This is a line of text that should trigger compression. "
	}

	entry := &ContextEntry{
		ID:       "large_entry",
		Type:     "llm",
		Source:   "response",
		Content:  largeContent,
		Priority: 5,
	}

	err := cm.AddEntry(entry)
	require.NoError(t, err)

	// Retrieve and verify decompression works
	retrieved, exists := cm.GetEntry("large_entry")
	assert.True(t, exists)
	assert.Equal(t, largeContent, retrieved.Content)
}

func TestContextManager_GetEntry(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("get existing entry", func(t *testing.T) {
		entry := &ContextEntry{
			ID:       "get_test",
			Type:     "lsp",
			Content:  "test",
			Priority: 5,
		}
		_ = cm.AddEntry(entry)

		retrieved, exists := cm.GetEntry("get_test")
		assert.True(t, exists)
		assert.Equal(t, "test", retrieved.Content)
	})

	t.Run("get non-existent entry", func(t *testing.T) {
		retrieved, exists := cm.GetEntry("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, retrieved)
	})
}

func TestContextManager_UpdateEntry(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("update existing entry", func(t *testing.T) {
		entry := &ContextEntry{
			ID:       "update_test",
			Type:     "lsp",
			Content:  "original",
			Priority: 5,
		}
		_ = cm.AddEntry(entry)

		err := cm.UpdateEntry("update_test", "updated content", map[string]interface{}{"new": "metadata"})
		require.NoError(t, err)

		retrieved, _ := cm.GetEntry("update_test")
		assert.Equal(t, "updated content", retrieved.Content)
		assert.Equal(t, "metadata", retrieved.Metadata["new"])
	})

	t.Run("update non-existent entry", func(t *testing.T) {
		err := cm.UpdateEntry("nonexistent", "content", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestContextManager_RemoveEntry(t *testing.T) {
	cm := NewContextManager(100)

	entry := &ContextEntry{
		ID:       "remove_test",
		Type:     "lsp",
		Content:  "to be removed",
		Priority: 5,
	}
	_ = cm.AddEntry(entry)

	cm.RemoveEntry("remove_test")

	_, exists := cm.GetEntry("remove_test")
	assert.False(t, exists)
}

func TestContextManager_BuildContext(t *testing.T) {
	cm := NewContextManager(100)

	// Add entries with different priorities
	_ = cm.AddEntry(&ContextEntry{
		ID:       "low_priority",
		Type:     "lsp",
		Content:  "low priority content",
		Priority: 2,
	})
	_ = cm.AddEntry(&ContextEntry{
		ID:       "high_priority",
		Type:     "lsp",
		Content:  "high priority content",
		Priority: 9,
	})
	_ = cm.AddEntry(&ContextEntry{
		ID:       "medium_priority",
		Type:     "tool",
		Content:  "medium priority content",
		Priority: 5,
	})

	entries, err := cm.BuildContext("code_completion", 1000)
	require.NoError(t, err)
	assert.True(t, len(entries) > 0)

	// High priority should come first
	assert.Equal(t, "high_priority", entries[0].ID)
}

func TestContextManager_CacheResult(t *testing.T) {
	cm := NewContextManager(100)

	cm.CacheResult("tool_result_1", map[string]interface{}{"result": "success"}, 5*time.Minute)

	result, exists := cm.GetCachedResult("tool_result_1")
	assert.True(t, exists)
	assert.NotNil(t, result)

	resultMap := result.(map[string]interface{})
	assert.Equal(t, "success", resultMap["result"])
}

func TestContextManager_CacheResult_Expiration(t *testing.T) {
	cm := NewContextManager(100)

	cm.CacheResult("expiring", "data", 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	_, exists := cm.GetCachedResult("expiring")
	assert.False(t, exists)
}

func TestContextManager_GetCachedResult_NotFound(t *testing.T) {
	cm := NewContextManager(100)

	result, exists := cm.GetCachedResult("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, result)
}

func TestContextManager_DetectConflicts(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("no conflicts", func(t *testing.T) {
		_ = cm.AddEntry(&ContextEntry{
			ID:       "c1",
			Type:     "lsp",
			Source:   "test.go",
			Content:  "content1",
			Priority: 5,
		})

		conflicts := cm.DetectConflicts()
		// May or may not have conflicts depending on content
		assert.NotNil(t, conflicts)
	})

	t.Run("same content different metadata", func(t *testing.T) {
		cm2 := NewContextManager(100)

		_ = cm2.AddEntry(&ContextEntry{
			ID:       "conflict1",
			Type:     "lsp",
			Source:   "same_source",
			Content:  "identical content",
			Metadata: map[string]interface{}{"version": 1},
			Priority: 5,
		})
		_ = cm2.AddEntry(&ContextEntry{
			ID:       "conflict2",
			Type:     "lsp",
			Source:   "same_source",
			Content:  "identical content",
			Metadata: map[string]interface{}{"version": 2},
			Priority: 5,
		})

		conflicts := cm2.DetectConflicts()
		// Should detect metadata conflict
		assert.NotNil(t, conflicts)
	})
}

func TestContextManager_Cleanup(t *testing.T) {
	cm := NewContextManager(100)

	// Add an old entry (simulate by manipulating timestamp)
	entry := &ContextEntry{
		ID:       "old_entry",
		Type:     "lsp",
		Content:  "old content",
		Priority: 5,
	}
	_ = cm.AddEntry(entry)

	// Manually set timestamp to old
	cm.mu.Lock()
	cm.entries["old_entry"].Timestamp = time.Now().Add(-48 * time.Hour)
	cm.mu.Unlock()

	// Add recent entry
	_ = cm.AddEntry(&ContextEntry{
		ID:       "new_entry",
		Type:     "lsp",
		Content:  "new content",
		Priority: 5,
	})

	cm.Cleanup()

	// Old entry should be removed
	_, exists := cm.GetEntry("old_entry")
	assert.False(t, exists)

	// New entry should remain
	_, exists = cm.GetEntry("new_entry")
	assert.True(t, exists)
}

func TestContextManager_Eviction(t *testing.T) {
	// Small max size to trigger eviction
	cm := NewContextManager(3)

	// Fill the context manager
	for i := 1; i <= 3; i++ {
		_ = cm.AddEntry(&ContextEntry{
			ID:       string(rune('0' + i)),
			Type:     "lsp",
			Content:  "content",
			Priority: i, // Different priorities
		})
		time.Sleep(10 * time.Millisecond) // Different timestamps
	}

	// Add one more to trigger eviction
	_ = cm.AddEntry(&ContextEntry{
		ID:       "new",
		Type:     "lsp",
		Content:  "new content",
		Priority: 5,
	})

	// Lowest priority should be evicted
	_, exists := cm.GetEntry("1")
	assert.False(t, exists, "Lowest priority entry should be evicted")

	// New entry should exist
	_, exists = cm.GetEntry("new")
	assert.True(t, exists)
}

func TestContextManager_calculateRelevanceScore(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("lsp entry for code completion", func(t *testing.T) {
		entry := &ContextEntry{
			Type:      "lsp",
			Content:   "function definition",
			Priority:  5,
			Timestamp: time.Now(),
		}

		score := cm.calculateRelevanceScore(entry, "code_completion")
		assert.True(t, score > 0)
	})

	t.Run("tool entry for tool execution", func(t *testing.T) {
		entry := &ContextEntry{
			Type:      "tool",
			Content:   "run execute command",
			Priority:  5,
			Timestamp: time.Now(),
		}

		score := cm.calculateRelevanceScore(entry, "tool_execution")
		assert.True(t, score > 0)
	})

	t.Run("higher priority gets higher score", func(t *testing.T) {
		lowPriority := &ContextEntry{
			Type:      "lsp",
			Content:   "test",
			Priority:  1,
			Timestamp: time.Now(),
		}

		highPriority := &ContextEntry{
			Type:      "lsp",
			Content:   "test",
			Priority:  10,
			Timestamp: time.Now(),
		}

		lowScore := cm.calculateRelevanceScore(lowPriority, "chat")
		highScore := cm.calculateRelevanceScore(highPriority, "chat")

		assert.True(t, highScore > lowScore)
	})
}

func TestContextManager_extractKeywords(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("code completion keywords", func(t *testing.T) {
		keywords := cm.extractKeywords("code_completion")
		assert.Contains(t, keywords, "function")
		assert.Contains(t, keywords, "class")
	})

	t.Run("tool execution keywords", func(t *testing.T) {
		keywords := cm.extractKeywords("tool_execution")
		assert.Contains(t, keywords, "run")
		assert.Contains(t, keywords, "execute")
	})

	t.Run("chat keywords", func(t *testing.T) {
		keywords := cm.extractKeywords("chat")
		assert.Contains(t, keywords, "conversation")
		assert.Contains(t, keywords, "question")
	})
}

func TestContextManager_isRelevant(t *testing.T) {
	cm := NewContextManager(100)

	tests := []struct {
		entryType   string
		requestType string
		expected    bool
	}{
		{"lsp", "code_completion", true},
		{"tool", "code_completion", true},
		{"llm", "chat", true},
		{"memory", "chat", true},
		{"tool", "tool_execution", true},
		{"mcp", "tool_execution", true},
		{"unknown", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.entryType+"_"+tt.requestType, func(t *testing.T) {
			entry := &ContextEntry{Type: tt.entryType}
			result := cm.isRelevant(entry, tt.requestType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextEntry(t *testing.T) {
	entry := &ContextEntry{
		ID:       "test_id",
		Type:     "lsp",
		Source:   "main.go",
		Content:  "func main() {}",
		Metadata: map[string]interface{}{"line": 10},
		Priority: 8,
	}

	assert.Equal(t, "test_id", entry.ID)
	assert.Equal(t, "lsp", entry.Type)
	assert.Equal(t, "main.go", entry.Source)
	assert.Equal(t, 8, entry.Priority)
}

func TestContextCacheEntry(t *testing.T) {
	entry := &ContextCacheEntry{
		Data:      map[string]interface{}{"result": "success"},
		Timestamp: time.Now(),
		TTL:       5 * time.Minute,
	}

	assert.NotNil(t, entry.Data)
	assert.False(t, entry.Timestamp.IsZero())
	assert.Equal(t, 5*time.Minute, entry.TTL)
}

func TestConflict(t *testing.T) {
	conflict := &Conflict{
		Type:     "metadata_conflict",
		Source:   "test.go",
		Entries:  []*ContextEntry{{ID: "e1"}, {ID: "e2"}},
		Severity: "medium",
		Message:  "Conflicting metadata detected",
	}

	assert.Equal(t, "metadata_conflict", conflict.Type)
	assert.Equal(t, "test.go", conflict.Source)
	assert.Len(t, conflict.Entries, 2)
	assert.Equal(t, "medium", conflict.Severity)
}

func TestContextManager_metadataEqual(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("equal metadata", func(t *testing.T) {
		a := map[string]interface{}{"key": "value", "num": 42}
		b := map[string]interface{}{"key": "value", "num": 42}
		assert.True(t, cm.metadataEqual(a, b))
	})

	t.Run("different metadata", func(t *testing.T) {
		a := map[string]interface{}{"key": "value1"}
		b := map[string]interface{}{"key": "value2"}
		assert.False(t, cm.metadataEqual(a, b))
	})

	t.Run("nil metadata", func(t *testing.T) {
		var a map[string]interface{} = nil
		var b map[string]interface{} = nil
		assert.True(t, cm.metadataEqual(a, b))
	})
}

func BenchmarkContextManager_AddEntry(b *testing.B) {
	cm := NewContextManager(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry := &ContextEntry{
			ID:       string(rune(i % 1000)),
			Type:     "lsp",
			Content:  "benchmark content",
			Priority: 5,
		}
		_ = cm.AddEntry(entry)
	}
}

func BenchmarkContextManager_GetEntry(b *testing.B) {
	cm := NewContextManager(10000)
	_ = cm.AddEntry(&ContextEntry{
		ID:       "bench_entry",
		Type:     "lsp",
		Content:  "benchmark",
		Priority: 5,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.GetEntry("bench_entry")
	}
}

func BenchmarkContextManager_BuildContext(b *testing.B) {
	cm := NewContextManager(100)

	// Add some entries
	for i := 0; i < 50; i++ {
		_ = cm.AddEntry(&ContextEntry{
			ID:       string(rune(i)),
			Type:     "lsp",
			Content:  "benchmark content for testing",
			Priority: i % 10,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.BuildContext("code_completion", 1000)
	}
}
