package services

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContextEntry represents a single context item
type ContextEntry struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`   // "lsp", "mcp", "tool", "llm"
	Source         string                 `json:"source"` // file path, tool name, etc.
	Content        string                 `json:"content"`
	Metadata       map[string]interface{} `json:"metadata"`
	Timestamp      time.Time              `json:"timestamp"`
	Priority       int                    `json:"priority"` // 1-10, higher is more important
	Compressed     bool                   `json:"compressed"`
	CompressedData []byte                 `json:"compressed_data,omitempty"`
}

// ContextCacheEntry represents a cached context item
type ContextCacheEntry struct {
	Data      interface{}
	Timestamp time.Time
	TTL       time.Duration
}

// ContextManager manages context for LLM requests
type ContextManager struct {
	mu                   sync.RWMutex
	entries              map[string]*ContextEntry
	cache                map[string]*ContextCacheEntry // For LSP, MCP, tool results
	cacheMu              sync.RWMutex
	maxSize              int
	compressionThreshold int // Compress entries larger than this
}

// NewContextManager creates a new context manager
func NewContextManager(maxSize int) *ContextManager {
	return &ContextManager{
		entries:              make(map[string]*ContextEntry),
		cache:                make(map[string]*ContextCacheEntry),
		maxSize:              maxSize,
		compressionThreshold: 1024, // 1KB
	}
}

// AddEntry adds a context entry
func (cm *ContextManager) AddEntry(entry *ContextEntry) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Compress if needed
	if len(entry.Content) > cm.compressionThreshold {
		if err := cm.compressEntry(entry); err != nil {
			return fmt.Errorf("failed to compress entry: %w", err)
		}
	}

	// Check size limits
	if len(cm.entries) >= cm.maxSize {
		if err := cm.evictLowPriorityEntries(); err != nil {
			return fmt.Errorf("failed to evict entries: %w", err)
		}
	}

	entry.Timestamp = time.Now()
	cm.entries[entry.ID] = entry

	return nil
}

// GetEntry retrieves a context entry
func (cm *ContextManager) GetEntry(id string) (*ContextEntry, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, exists := cm.entries[id]
	if !exists {
		return nil, false
	}

	// Decompress if needed
	if entry.Compressed {
		if err := cm.decompressEntry(entry); err != nil {
			return nil, false
		}
	}

	return entry, true
}

// UpdateEntry updates an existing entry
func (cm *ContextManager) UpdateEntry(id string, content string, metadata map[string]interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entry, exists := cm.entries[id]
	if !exists {
		return fmt.Errorf("entry %s not found", id)
	}

	entry.Content = content
	entry.Metadata = metadata
	entry.Timestamp = time.Now()

	// Re-compress if needed
	if len(entry.Content) > cm.compressionThreshold && !entry.Compressed {
		if err := cm.compressEntry(entry); err != nil {
			return fmt.Errorf("failed to compress entry: %w", err)
		}
	}

	return nil
}

// RemoveEntry removes a context entry
func (cm *ContextManager) RemoveEntry(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.entries, id)
}

// BuildContext builds optimized context for an LLM request
func (cm *ContextManager) BuildContext(requestType string, maxTokens int) ([]*ContextEntry, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Get all entries
	entries := make([]*ContextEntry, 0, len(cm.entries))
	for _, entry := range cm.entries {
		entries = append(entries, entry)
	}

	// Sort by priority and recency
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Priority != entries[j].Priority {
			return entries[i].Priority > entries[j].Priority
		}
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Filter and select relevant entries
	selected := cm.selectRelevantEntries(entries, requestType, maxTokens)

	// Decompress selected entries
	for _, entry := range selected {
		if entry.Compressed {
			if err := cm.decompressEntry(entry); err != nil {
				continue // Skip corrupted entries
			}
		}
	}

	return selected, nil
}

// CacheResult caches a result from LSP, MCP, or tool execution
func (cm *ContextManager) CacheResult(key string, result interface{}, ttl time.Duration) {
	cm.cacheMu.Lock()
	defer cm.cacheMu.Unlock()

	cm.cache[key] = &ContextCacheEntry{
		Data:      result,
		Timestamp: time.Now(),
		TTL:       ttl,
	}
}

// GetCachedResult retrieves a cached result
func (cm *ContextManager) GetCachedResult(key string) (interface{}, bool) {
	cm.cacheMu.RLock()
	defer cm.cacheMu.RUnlock()

	entry, exists := cm.cache[key]
	if !exists {
		return nil, false
	}

	if time.Since(entry.Timestamp) > entry.TTL {
		// Expired, will be cleaned up by cleanup routine
		return nil, false
	}

	return entry.Data, true
}

// DetectConflicts detects conflicting information in context
func (cm *ContextManager) DetectConflicts() []Conflict {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conflicts := []Conflict{}

	// Group entries by source
	sourceMap := make(map[string][]*ContextEntry)
	for _, entry := range cm.entries {
		sourceMap[entry.Source] = append(sourceMap[entry.Source], entry)
	}

	// Check for conflicts within each source
	for source, entries := range sourceMap {
		if conflict := cm.detectSourceConflicts(source, entries); conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
	}

	// Check for cross-source conflicts
	crossSourceConflicts := cm.detectCrossSourceConflicts()
	conflicts = append(conflicts, crossSourceConflicts...)

	// Check for temporal conflicts
	temporalConflicts := cm.detectTemporalConflicts()
	conflicts = append(conflicts, temporalConflicts...)

	// Check for semantic conflicts
	semanticConflicts := cm.detectSemanticConflicts()
	conflicts = append(conflicts, semanticConflicts...)

	return conflicts
}

// detectCrossSourceConflicts finds conflicts between different sources
func (cm *ContextManager) detectCrossSourceConflicts() []Conflict {
	conflicts := []Conflict{}

	// Group entries by type
	typeMap := make(map[string][]*ContextEntry)
	for _, entry := range cm.entries {
		typeMap[entry.Type] = append(typeMap[entry.Type], entry)
	}

	// For each type, check if entries from different sources conflict
	for entryType, entries := range typeMap {
		if len(entries) < 2 {
			continue
		}

		// Group by a common subject/topic identifier from metadata
		subjectMap := make(map[string][]*ContextEntry)
		for _, entry := range entries {
			subject := cm.extractSubject(entry)
			if subject != "" {
				subjectMap[subject] = append(subjectMap[subject], entry)
			}
		}

		// Check each subject group for conflicts
		for subject, subjectEntries := range subjectMap {
			if len(subjectEntries) < 2 {
				continue
			}

			// Check if entries from different sources have contradicting values
			sourceValues := make(map[string]map[string]interface{})
			for _, entry := range subjectEntries {
				if _, exists := sourceValues[entry.Source]; !exists {
					sourceValues[entry.Source] = make(map[string]interface{})
				}
				for k, v := range entry.Metadata {
					sourceValues[entry.Source][k] = v
				}
			}

			if len(sourceValues) > 1 {
				// Check for value conflicts across sources
				if conflict := cm.checkValueConflicts(sourceValues, subject, entryType, subjectEntries); conflict != nil {
					conflicts = append(conflicts, *conflict)
				}
			}
		}
	}

	return conflicts
}

// detectTemporalConflicts finds conflicts due to stale data
func (cm *ContextManager) detectTemporalConflicts() []Conflict {
	conflicts := []Conflict{}
	now := time.Now()
	staleThreshold := 1 * time.Hour

	// Group entries by subject
	subjectMap := make(map[string][]*ContextEntry)
	for _, entry := range cm.entries {
		subject := cm.extractSubject(entry)
		if subject != "" {
			subjectMap[subject] = append(subjectMap[subject], entry)
		}
	}

	for subject, entries := range subjectMap {
		if len(entries) < 2 {
			continue
		}

		// Sort by timestamp
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.Before(entries[j].Timestamp)
		})

		// Check if old entries conflict with newer ones
		newest := entries[len(entries)-1]
		for _, older := range entries[:len(entries)-1] {
			age := now.Sub(older.Timestamp)
			if age > staleThreshold {
				// Check if content differs significantly
				if older.Content != newest.Content && !cm.isContentUpdate(older, newest) {
					conflicts = append(conflicts, Conflict{
						Type:     "temporal_conflict",
						Source:   subject,
						Entries:  []*ContextEntry{older, newest},
						Severity: cm.calculateTemporalSeverity(age),
						Message:  fmt.Sprintf("Stale entry (%s old) may conflict with newer data for subject: %s", age.Round(time.Minute), subject),
					})
				}
			}
		}
	}

	return conflicts
}

// detectSemanticConflicts finds contradictory semantic information
func (cm *ContextManager) detectSemanticConflicts() []Conflict {
	conflicts := []Conflict{}

	// Define known semantic fields that should be consistent
	semanticFields := []string{"status", "state", "enabled", "active", "version", "value"}

	// Group entries by subject
	subjectMap := make(map[string][]*ContextEntry)
	for _, entry := range cm.entries {
		subject := cm.extractSubject(entry)
		if subject != "" {
			subjectMap[subject] = append(subjectMap[subject], entry)
		}
	}

	for subject, entries := range subjectMap {
		if len(entries) < 2 {
			continue
		}

		// Check each semantic field for conflicts
		for _, field := range semanticFields {
			values := make(map[string][]*ContextEntry)
			for _, entry := range entries {
				if val, ok := entry.Metadata[field]; ok {
					valStr := fmt.Sprintf("%v", val)
					values[valStr] = append(values[valStr], entry)
				}
			}

			// If we have multiple different values for the same field, it's a conflict
			if len(values) > 1 {
				var conflictingEntries []*ContextEntry
				var valueList []string
				for val, entryList := range values {
					conflictingEntries = append(conflictingEntries, entryList...)
					valueList = append(valueList, val)
				}

				conflicts = append(conflicts, Conflict{
					Type:     "semantic_conflict",
					Source:   subject,
					Entries:  conflictingEntries,
					Severity: "high",
					Message:  fmt.Sprintf("Conflicting %s values for subject '%s': %v", field, subject, valueList),
				})
			}
		}
	}

	return conflicts
}

// extractSubject extracts a common subject identifier from entry metadata
func (cm *ContextManager) extractSubject(entry *ContextEntry) string {
	// Try common subject identifiers
	subjectFields := []string{"subject", "topic", "id", "name", "key", "file", "resource"}
	for _, field := range subjectFields {
		if val, ok := entry.Metadata[field]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	// Fall back to source if no subject found
	return entry.Source
}

// checkValueConflicts checks if different sources have conflicting values
func (cm *ContextManager) checkValueConflicts(sourceValues map[string]map[string]interface{}, subject, entryType string, entries []*ContextEntry) *Conflict {
	// Compare values across sources
	allKeys := make(map[string]bool)
	for _, values := range sourceValues {
		for k := range values {
			allKeys[k] = true
		}
	}

	var conflictingKeys []string
	for key := range allKeys {
		values := make(map[string]bool)
		for _, sourceVals := range sourceValues {
			if val, ok := sourceVals[key]; ok {
				valStr := fmt.Sprintf("%v", val)
				values[valStr] = true
			}
		}
		if len(values) > 1 {
			conflictingKeys = append(conflictingKeys, key)
		}
	}

	if len(conflictingKeys) > 0 {
		return &Conflict{
			Type:     "cross_source_conflict",
			Source:   subject,
			Entries:  entries,
			Severity: "medium",
			Message:  fmt.Sprintf("Cross-source conflict for %s entries on subject '%s': conflicting keys: %v", entryType, subject, conflictingKeys),
		}
	}

	return nil
}

// isContentUpdate checks if newer entry is a legitimate update to older entry
func (cm *ContextManager) isContentUpdate(older, newer *ContextEntry) bool {
	// Check if newer has higher priority (indicating intentional update)
	if newer.Priority > older.Priority {
		return true
	}
	// Check if metadata indicates an update
	if v, ok := newer.Metadata["update_of"]; ok {
		if updateID, ok := v.(string); ok && updateID == older.ID {
			return true
		}
	}
	return false
}

// calculateTemporalSeverity calculates severity based on age of stale entry
func (cm *ContextManager) calculateTemporalSeverity(age time.Duration) string {
	switch {
	case age > 24*time.Hour:
		return "high"
	case age > 6*time.Hour:
		return "medium"
	default:
		return "low"
	}
}

// Cleanup removes expired entries
func (cm *ContextManager) Cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.cacheMu.Lock()
	defer cm.cacheMu.Unlock()

	now := time.Now()

	// Clean context entries (keep recent ones)
	for id, entry := range cm.entries {
		if now.Sub(entry.Timestamp) > 24*time.Hour {
			delete(cm.entries, id)
		}
	}

	// Clean cache
	for key, entry := range cm.cache {
		if now.Sub(entry.Timestamp) > entry.TTL {
			delete(cm.cache, key)
		}
	}
}

// Helper methods

func (cm *ContextManager) compressEntry(entry *ContextEntry) error {
	var buf strings.Builder
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write([]byte(entry.Content)); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	entry.CompressedData = []byte(buf.String())
	entry.Compressed = true
	entry.Content = "" // Clear original content

	return nil
}

func (cm *ContextManager) decompressEntry(entry *ContextEntry) error {
	if !entry.Compressed || len(entry.CompressedData) == 0 {
		return nil
	}

	gr, err := gzip.NewReader(strings.NewReader(string(entry.CompressedData)))
	if err != nil {
		return err
	}
	defer gr.Close()

	content, err := io.ReadAll(gr)
	if err != nil {
		return err
	}

	entry.Content = string(content)
	entry.Compressed = false
	entry.CompressedData = nil

	return nil
}

func (cm *ContextManager) evictLowPriorityEntries() error {
	if len(cm.entries) == 0 {
		return nil
	}

	// Find lowest priority entries
	type entryWithID struct {
		id    string
		entry *ContextEntry
	}

	var candidates []entryWithID
	minPriority := 10

	for id, entry := range cm.entries {
		if entry.Priority < minPriority {
			minPriority = entry.Priority
			candidates = []entryWithID{{id, entry}}
		} else if entry.Priority == minPriority {
			candidates = append(candidates, entryWithID{id, entry})
		}
	}

	// Remove oldest from lowest priority
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].entry.Timestamp.Before(candidates[j].entry.Timestamp)
		})

		delete(cm.entries, candidates[0].id)
	}

	return nil
}

func (cm *ContextManager) selectRelevantEntries(entries []*ContextEntry, requestType string, maxTokens int) []*ContextEntry {
	// Score entries by relevance
	type scoredEntry struct {
		entry *ContextEntry
		score float64
	}

	var scoredEntries []scoredEntry
	for _, entry := range entries {
		score := cm.calculateRelevanceScore(entry, requestType)
		scoredEntries = append(scoredEntries, scoredEntry{entry: entry, score: score})
	}

	// Sort by score (descending) then by recency
	sort.Slice(scoredEntries, func(i, j int) bool {
		if scoredEntries[i].score != scoredEntries[j].score {
			return scoredEntries[i].score > scoredEntries[j].score
		}
		return scoredEntries[i].entry.Timestamp.After(scoredEntries[j].entry.Timestamp)
	})

	// Select entries within token limit
	selected := []*ContextEntry{}
	totalTokens := 0

	for _, scored := range scoredEntries {
		estimatedTokens := len(scored.entry.Content) / 4
		if totalTokens+estimatedTokens > maxTokens {
			break
		}

		selected = append(selected, scored.entry)
		totalTokens += estimatedTokens
	}

	return selected
}

// calculateRelevanceScore calculates ML-based relevance score
func (cm *ContextManager) calculateRelevanceScore(entry *ContextEntry, requestType string) float64 {
	score := 0.0

	// Base score from priority
	score += float64(entry.Priority) * 10.0

	// Recency bonus (newer entries get higher score)
	hoursOld := time.Since(entry.Timestamp).Hours()
	recencyScore := 1.0 / (1.0 + hoursOld/24.0) // Exponential decay
	score += recencyScore * 5.0

	// Content-based scoring
	contentLower := strings.ToLower(entry.Content)

	// Keyword matching
	keywords := cm.extractKeywords(requestType)
	keywordMatches := 0
	for _, keyword := range keywords {
		if strings.Contains(contentLower, keyword) {
			keywordMatches++
		}
	}
	score += float64(keywordMatches) * 2.0

	// Type-specific scoring
	switch requestType {
	case "code_completion":
		if entry.Type == "lsp" {
			score += 15.0
		}
	case "tool_execution":
		if entry.Type == "tool" || entry.Type == "mcp" {
			score += 15.0
		}
	case "chat":
		if entry.Type == "llm" {
			score += 10.0
		}
	}

	// Source reliability scoring
	switch entry.Source {
	case "lsp":
		score += 8.0
	case "mcp":
		score += 7.0
	case "tool":
		score += 6.0
	}

	return score
}

// extractKeywords extracts keywords from request type
func (cm *ContextManager) extractKeywords(requestType string) []string {
	// Simple keyword extraction - can be enhanced with NLP
	keywords := []string{requestType}

	switch requestType {
	case "code_completion":
		keywords = append(keywords, "function", "class", "variable", "import", "syntax")
	case "tool_execution":
		keywords = append(keywords, "run", "execute", "command", "script")
	case "chat":
		keywords = append(keywords, "conversation", "question", "answer")
	}

	return keywords
}

func (cm *ContextManager) isRelevant(entry *ContextEntry, requestType string) bool {
	// Simple relevance logic - can be enhanced with ML
	switch requestType {
	case "code_completion":
		return entry.Type == "lsp" || entry.Type == "tool"
	case "chat":
		return entry.Type == "llm" || entry.Type == "memory"
	case "tool_execution":
		return entry.Type == "tool" || entry.Type == "mcp"
	default:
		return true
	}
}

func (cm *ContextManager) detectSourceConflicts(source string, entries []*ContextEntry) *Conflict {
	// Detect conflicts within entries from the same source

	contentMap := make(map[string][]*ContextEntry)
	for _, entry := range entries {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(entry.Content)))
		contentMap[hash] = append(contentMap[hash], entry)
	}

	// If we have multiple entries with same content but different metadata,
	// it indicates a metadata conflict within the same source
	for _, group := range contentMap {
		if len(group) > 1 {
			// Check if metadata differs significantly
			if cm.hasConflictingMetadata(group) {
				// Identify which metadata keys differ
				differingKeys := cm.findDifferingMetadataKeys(group)
				return &Conflict{
					Type:     "metadata_conflict",
					Source:   source,
					Entries:  group,
					Severity: "medium",
					Message:  fmt.Sprintf("Entries from source '%s' have identical content but conflicting metadata on keys: %v", source, differingKeys),
				}
			}
		}
	}

	// Also check for entries with same type but contradicting content
	typeMap := make(map[string][]*ContextEntry)
	for _, entry := range entries {
		typeMap[entry.Type] = append(typeMap[entry.Type], entry)
	}

	for entryType, typeEntries := range typeMap {
		if len(typeEntries) > 1 {
			// Check for content conflicts within same type
			if conflict := cm.detectContentConflict(source, entryType, typeEntries); conflict != nil {
				return conflict
			}
		}
	}

	return nil
}

// findDifferingMetadataKeys identifies which metadata keys differ between entries
func (cm *ContextManager) findDifferingMetadataKeys(entries []*ContextEntry) []string {
	if len(entries) < 2 {
		return nil
	}

	allKeys := make(map[string]bool)
	for _, entry := range entries {
		for k := range entry.Metadata {
			allKeys[k] = true
		}
	}

	var differingKeys []string
	for key := range allKeys {
		values := make(map[string]bool)
		for _, entry := range entries {
			if val, ok := entry.Metadata[key]; ok {
				valStr := fmt.Sprintf("%v", val)
				values[valStr] = true
			} else {
				values["<missing>"] = true
			}
		}
		if len(values) > 1 {
			differingKeys = append(differingKeys, key)
		}
	}

	return differingKeys
}

// detectContentConflict checks for contradicting content within same type entries
func (cm *ContextManager) detectContentConflict(source, entryType string, entries []*ContextEntry) *Conflict {
	// Group by subject within the same type
	subjectContent := make(map[string][]string)
	subjectEntries := make(map[string][]*ContextEntry)

	for _, entry := range entries {
		subject := cm.extractSubject(entry)
		subjectContent[subject] = append(subjectContent[subject], entry.Content)
		subjectEntries[subject] = append(subjectEntries[subject], entry)
	}

	// Check if same subject has different content
	for subject, contents := range subjectContent {
		uniqueContents := make(map[string]bool)
		for _, content := range contents {
			uniqueContents[content] = true
		}

		if len(uniqueContents) > 1 && len(contents) > 1 {
			return &Conflict{
				Type:     "content_conflict",
				Source:   source,
				Entries:  subjectEntries[subject],
				Severity: "high",
				Message:  fmt.Sprintf("Multiple %s entries for subject '%s' have conflicting content within source '%s'", entryType, subject, source),
			}
		}
	}

	return nil
}

func (cm *ContextManager) hasConflictingMetadata(entries []*ContextEntry) bool {
	if len(entries) < 2 {
		return false
	}

	// Compare metadata between entries
	base := entries[0].Metadata
	for _, entry := range entries[1:] {
		if !cm.metadataEqual(base, entry.Metadata) {
			return true
		}
	}

	return false
}

func (cm *ContextManager) metadataEqual(a, b map[string]interface{}) bool {
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return string(aBytes) == string(bBytes)
}

// ContextContextCacheEntry represents a cached context item
type ContextContextCacheEntry struct {
	Data      interface{}
	Timestamp time.Time
	TTL       time.Duration
}

// Conflict represents a detected conflict in context
type Conflict struct {
	Type     string          `json:"type"`
	Source   string          `json:"source"`
	Entries  []*ContextEntry `json:"entries"`
	Severity string          `json:"severity"`
	Message  string          `json:"message"`
}
