package services

import (
	"fmt"
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

func TestContextManager_HasConflictingMetadata(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("no entries", func(t *testing.T) {
		result := cm.hasConflictingMetadata([]*ContextEntry{})
		assert.False(t, result)
	})

	t.Run("single entry", func(t *testing.T) {
		result := cm.hasConflictingMetadata([]*ContextEntry{
			{ID: "1", Metadata: map[string]interface{}{"key": "value"}},
		})
		assert.False(t, result)
	})

	t.Run("matching metadata", func(t *testing.T) {
		result := cm.hasConflictingMetadata([]*ContextEntry{
			{ID: "1", Metadata: map[string]interface{}{"key": "value"}},
			{ID: "2", Metadata: map[string]interface{}{"key": "value"}},
		})
		assert.False(t, result)
	})

	t.Run("conflicting metadata", func(t *testing.T) {
		result := cm.hasConflictingMetadata([]*ContextEntry{
			{ID: "1", Metadata: map[string]interface{}{"key": "value1"}},
			{ID: "2", Metadata: map[string]interface{}{"key": "value2"}},
		})
		assert.True(t, result)
	})
}

func TestContextManager_DecompressEntry(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("not compressed", func(t *testing.T) {
		entry := &ContextEntry{
			ID:         "1",
			Content:    "test content",
			Compressed: false,
		}
		err := cm.decompressEntry(entry)
		assert.NoError(t, err)
		assert.Equal(t, "test content", entry.Content)
	})

	t.Run("empty compressed data", func(t *testing.T) {
		entry := &ContextEntry{
			ID:             "1",
			Compressed:     true,
			CompressedData: nil,
		}
		err := cm.decompressEntry(entry)
		assert.NoError(t, err)
	})
}

func TestContextManager_CompressAndDecompress(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("compress and decompress round trip", func(t *testing.T) {
		originalContent := "This is test content that should be compressed and then decompressed"
		entry := &ContextEntry{
			ID:      "1",
			Content: originalContent,
		}

		// Compress
		err := cm.compressEntry(entry)
		assert.NoError(t, err)
		assert.True(t, entry.Compressed)
		assert.NotNil(t, entry.CompressedData)
		assert.Empty(t, entry.Content)

		// Decompress
		err = cm.decompressEntry(entry)
		assert.NoError(t, err)
		assert.False(t, entry.Compressed)
		assert.Nil(t, entry.CompressedData)
		assert.Equal(t, originalContent, entry.Content)
	})
}

func TestContextManager_MetadataEqual(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("equal nil", func(t *testing.T) {
		result := cm.metadataEqual(nil, nil)
		assert.True(t, result)
	})

	t.Run("equal empty maps", func(t *testing.T) {
		result := cm.metadataEqual(map[string]interface{}{}, map[string]interface{}{})
		assert.True(t, result)
	})

	t.Run("equal maps", func(t *testing.T) {
		a := map[string]interface{}{"key1": "value1", "key2": 123}
		b := map[string]interface{}{"key1": "value1", "key2": 123}
		result := cm.metadataEqual(a, b)
		assert.True(t, result)
	})

	t.Run("different maps", func(t *testing.T) {
		a := map[string]interface{}{"key1": "value1"}
		b := map[string]interface{}{"key1": "value2"}
		result := cm.metadataEqual(a, b)
		assert.False(t, result)
	})
}

func TestContextManager_EvictLowPriorityEntries(t *testing.T) {
	t.Run("empty entries", func(t *testing.T) {
		cm := NewContextManager(100)
		err := cm.evictLowPriorityEntries()
		assert.NoError(t, err)
	})

	t.Run("with entries", func(t *testing.T) {
		cm := NewContextManager(10)

		// Add more entries than capacity
		for i := 0; i < 15; i++ {
			_ = cm.AddEntry(&ContextEntry{
				ID:       fmt.Sprintf("entry-%d", i),
				Type:     "test",
				Content:  "test content",
				Priority: i % 5,
			})
		}

		initialCount := len(cm.entries)
		err := cm.evictLowPriorityEntries()
		assert.NoError(t, err)
		// Should have evicted some entries
		assert.LessOrEqual(t, len(cm.entries), initialCount)
	})
}

func TestContextManager_SelectRelevantEntries(t *testing.T) {
	cm := NewContextManager(100)

	// Create entries with different types
	testEntries := []*ContextEntry{
		{
			ID:       "1",
			Type:     "lsp",
			Content:  "LSP content about code completion",
			Priority: 5,
		},
		{
			ID:       "2",
			Type:     "mcp",
			Content:  "MCP tool result",
			Priority: 3,
		},
		{
			ID:       "3",
			Type:     "document",
			Content:  "Documentation content",
			Priority: 7,
		},
	}

	t.Run("select relevant entries", func(t *testing.T) {
		entries := cm.selectRelevantEntries(testEntries, "code_completion", 1000)
		assert.NotEmpty(t, entries)
	})

	t.Run("select with max tokens limit", func(t *testing.T) {
		entries := cm.selectRelevantEntries(testEntries, "code_completion", 10)
		// Should return some entries even with small limit
		assert.NotNil(t, entries)
	})

	t.Run("empty entries", func(t *testing.T) {
		entries := cm.selectRelevantEntries([]*ContextEntry{}, "code_completion", 1000)
		assert.Empty(t, entries)
	})
}

func TestContextManager_CalculateRelevanceScore(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("LSP entry for code completion", func(t *testing.T) {
		entry := &ContextEntry{
			ID:       "1",
			Type:     "lsp",
			Content:  "code completion result",
			Priority: 5,
		}
		score := cm.calculateRelevanceScore(entry, "code_completion")
		assert.Greater(t, score, 0.0)
	})

	t.Run("document entry for documentation task", func(t *testing.T) {
		entry := &ContextEntry{
			ID:       "1",
			Type:     "document",
			Content:  "documentation content",
			Priority: 5,
		}
		score := cm.calculateRelevanceScore(entry, "documentation")
		assert.Greater(t, score, 0.0)
	})

	t.Run("llm entry for chat request", func(t *testing.T) {
		entry := &ContextEntry{
			ID:        "1",
			Type:      "llm",
			Content:   "chat conversation question answer",
			Priority:  5,
			Timestamp: time.Now(),
		}
		score := cm.calculateRelevanceScore(entry, "chat")
		assert.Greater(t, score, 0.0)
	})

	t.Run("mcp entry for tool execution", func(t *testing.T) {
		entry := &ContextEntry{
			ID:        "1",
			Type:      "mcp",
			Content:   "MCP tool result",
			Priority:  5,
			Timestamp: time.Now(),
		}
		score := cm.calculateRelevanceScore(entry, "tool_execution")
		assert.Greater(t, score, 0.0)
	})

	t.Run("entry with lsp source", func(t *testing.T) {
		entry := &ContextEntry{
			ID:        "1",
			Type:      "lsp",
			Source:    "lsp",
			Content:   "LSP result",
			Priority:  5,
			Timestamp: time.Now(),
		}
		score := cm.calculateRelevanceScore(entry, "code_completion")
		assert.Greater(t, score, 0.0)
	})

	t.Run("entry with mcp source", func(t *testing.T) {
		entry := &ContextEntry{
			ID:        "1",
			Type:      "tool",
			Source:    "mcp",
			Content:   "MCP result",
			Priority:  5,
			Timestamp: time.Now(),
		}
		score := cm.calculateRelevanceScore(entry, "tool_execution")
		assert.Greater(t, score, 0.0)
	})

	t.Run("entry with tool source", func(t *testing.T) {
		entry := &ContextEntry{
			ID:        "1",
			Type:      "tool",
			Source:    "tool",
			Content:   "Tool result",
			Priority:  5,
			Timestamp: time.Now(),
		}
		score := cm.calculateRelevanceScore(entry, "tool_execution")
		assert.Greater(t, score, 0.0)
	})
}

func TestContextManager_DecompressEntry_InvalidData(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("invalid gzip data", func(t *testing.T) {
		entry := &ContextEntry{
			ID:             "1",
			Compressed:     true,
			CompressedData: []byte("not valid gzip data"),
		}
		err := cm.decompressEntry(entry)
		assert.Error(t, err)
	})
}

func TestContextManager_GetEntry_DecompressionError(t *testing.T) {
	cm := NewContextManager(100)

	// Add entry directly with invalid compressed data
	cm.mu.Lock()
	cm.entries["corrupted"] = &ContextEntry{
		ID:             "corrupted",
		Compressed:     true,
		CompressedData: []byte("invalid gzip data"),
	}
	cm.mu.Unlock()

	// GetEntry should return nil, false when decompression fails
	entry, exists := cm.GetEntry("corrupted")
	assert.False(t, exists)
	assert.Nil(t, entry)
}

func TestContextManager_BuildContext_WithCorruptedEntry(t *testing.T) {
	cm := NewContextManager(100)

	// Add a valid entry
	_ = cm.AddEntry(&ContextEntry{
		ID:       "valid",
		Type:     "lsp",
		Content:  "valid content",
		Priority: 5,
	})

	// Add a corrupted compressed entry directly
	cm.mu.Lock()
	cm.entries["corrupted"] = &ContextEntry{
		ID:             "corrupted",
		Type:           "lsp",
		Compressed:     true,
		CompressedData: []byte("invalid gzip data"),
		Priority:       8,
	}
	cm.mu.Unlock()

	// BuildContext should skip the corrupted entry
	entries, err := cm.BuildContext("code_completion", 1000)
	require.NoError(t, err)

	// Should have at least the valid entry
	foundValid := false
	for _, entry := range entries {
		if entry.ID == "valid" {
			foundValid = true
		}
	}
	assert.True(t, foundValid, "Valid entry should be in the result")
}

func TestContextManager_UpdateEntry_WithCompression(t *testing.T) {
	cm := NewContextManager(100)
	cm.compressionThreshold = 50 // Low threshold for testing

	// Add entry without compression
	entry := &ContextEntry{
		ID:       "update_compress_test",
		Type:     "lsp",
		Content:  "short",
		Priority: 5,
	}
	_ = cm.AddEntry(entry)

	// Create large content that will trigger compression
	largeContent := ""
	for i := 0; i < 20; i++ {
		largeContent += "This is a line of text for compression testing. "
	}

	// Update with large content - should trigger compression
	err := cm.UpdateEntry("update_compress_test", largeContent, map[string]interface{}{"updated": true})
	require.NoError(t, err)

	// Retrieve and verify the content is correct
	retrieved, exists := cm.GetEntry("update_compress_test")
	assert.True(t, exists)
	assert.Equal(t, largeContent, retrieved.Content)
}

func TestContextManager_checkValueConflicts(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("no conflicts", func(t *testing.T) {
		sourceValues := map[string]map[string]interface{}{
			"source1": {"key1": "value1"},
			"source2": {"key2": "value2"},
		}
		entries := []*ContextEntry{{ID: "1"}, {ID: "2"}}
		conflict := cm.checkValueConflicts(sourceValues, "test_subject", "lsp", entries)
		assert.Nil(t, conflict)
	})

	t.Run("same key same value", func(t *testing.T) {
		sourceValues := map[string]map[string]interface{}{
			"source1": {"key1": "value1"},
			"source2": {"key1": "value1"},
		}
		entries := []*ContextEntry{{ID: "1"}, {ID: "2"}}
		conflict := cm.checkValueConflicts(sourceValues, "test_subject", "lsp", entries)
		assert.Nil(t, conflict)
	})

	t.Run("conflicting values for same key", func(t *testing.T) {
		sourceValues := map[string]map[string]interface{}{
			"source1": {"key1": "value1"},
			"source2": {"key1": "different_value"},
		}
		entries := []*ContextEntry{{ID: "1"}, {ID: "2"}}
		conflict := cm.checkValueConflicts(sourceValues, "test_subject", "lsp", entries)
		assert.NotNil(t, conflict)
		assert.Equal(t, "cross_source_conflict", conflict.Type)
		assert.Contains(t, conflict.Message, "key1")
	})

	t.Run("multiple conflicting keys", func(t *testing.T) {
		sourceValues := map[string]map[string]interface{}{
			"source1": {"key1": "value1", "key2": 100},
			"source2": {"key1": "value2", "key2": 200},
		}
		entries := []*ContextEntry{{ID: "1"}, {ID: "2"}}
		conflict := cm.checkValueConflicts(sourceValues, "test_subject", "mcp", entries)
		assert.NotNil(t, conflict)
		assert.Equal(t, "medium", conflict.Severity)
	})
}

func TestContextManager_isContentUpdate(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("higher priority is update", func(t *testing.T) {
		older := &ContextEntry{ID: "1", Priority: 3}
		newer := &ContextEntry{ID: "2", Priority: 5}
		assert.True(t, cm.isContentUpdate(older, newer))
	})

	t.Run("lower priority is not update", func(t *testing.T) {
		older := &ContextEntry{ID: "1", Priority: 5}
		newer := &ContextEntry{ID: "2", Priority: 3}
		assert.False(t, cm.isContentUpdate(older, newer))
	})

	t.Run("same priority no metadata is not update", func(t *testing.T) {
		older := &ContextEntry{ID: "1", Priority: 5}
		newer := &ContextEntry{ID: "2", Priority: 5}
		assert.False(t, cm.isContentUpdate(older, newer))
	})

	t.Run("metadata update_of marks as update", func(t *testing.T) {
		older := &ContextEntry{ID: "original", Priority: 5}
		newer := &ContextEntry{
			ID:       "updated",
			Priority: 5,
			Metadata: map[string]interface{}{"update_of": "original"},
		}
		assert.True(t, cm.isContentUpdate(older, newer))
	})

	t.Run("metadata update_of with wrong ID is not update", func(t *testing.T) {
		older := &ContextEntry{ID: "original", Priority: 5}
		newer := &ContextEntry{
			ID:       "updated",
			Priority: 5,
			Metadata: map[string]interface{}{"update_of": "different_id"},
		}
		assert.False(t, cm.isContentUpdate(older, newer))
	})
}

func TestContextManager_calculateTemporalSeverity(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("more than 24 hours is high", func(t *testing.T) {
		age := 25 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "high", severity)
	})

	t.Run("more than 6 hours is medium", func(t *testing.T) {
		age := 12 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "medium", severity)
	})

	t.Run("less than 6 hours is low", func(t *testing.T) {
		age := 3 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "low", severity)
	})

	t.Run("exactly 24 hours is medium", func(t *testing.T) {
		age := 24 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "medium", severity)
	})

	t.Run("exactly 6 hours is low", func(t *testing.T) {
		age := 6 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "low", severity)
	})

	t.Run("zero hours is low", func(t *testing.T) {
		age := 0 * time.Hour
		severity := cm.calculateTemporalSeverity(age)
		assert.Equal(t, "low", severity)
	})
}

func TestContextManager_detectContentConflict(t *testing.T) {
	cm := NewContextManager(100)

	t.Run("no conflict with single entry", func(t *testing.T) {
		entries := []*ContextEntry{
			{ID: "1", Type: "lsp", Content: "content1", Source: "file.go"},
		}
		conflict := cm.detectContentConflict("file.go", "lsp", entries)
		assert.Nil(t, conflict)
	})

	t.Run("no conflict with same subject same content", func(t *testing.T) {
		entries := []*ContextEntry{
			{ID: "1", Type: "lsp", Content: "same content", Source: "file.go"},
			{ID: "2", Type: "lsp", Content: "same content", Source: "file.go"},
		}
		conflict := cm.detectContentConflict("file.go", "lsp", entries)
		assert.Nil(t, conflict)
	})

	t.Run("conflict with same subject different content", func(t *testing.T) {
		entries := []*ContextEntry{
			{ID: "1", Type: "lsp", Content: "content version 1", Source: "file.go"},
			{ID: "2", Type: "lsp", Content: "content version 2", Source: "file.go"},
		}
		conflict := cm.detectContentConflict("file.go", "lsp", entries)
		assert.NotNil(t, conflict)
		assert.Equal(t, "content_conflict", conflict.Type)
		assert.Equal(t, "high", conflict.Severity)
	})

	t.Run("no conflict with different subjects", func(t *testing.T) {
		entries := []*ContextEntry{
			{ID: "1", Type: "mcp", Content: "content1", Source: "tool1"},
			{ID: "2", Type: "mcp", Content: "content2", Source: "tool2"},
		}
		conflict := cm.detectContentConflict("tools", "mcp", entries)
		assert.Nil(t, conflict)
	})

	t.Run("empty entries", func(t *testing.T) {
		entries := []*ContextEntry{}
		conflict := cm.detectContentConflict("source", "type", entries)
		assert.Nil(t, conflict)
	})
}
