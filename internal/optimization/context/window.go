// Package context provides context window management for LLM interactions.
package context

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrContextOverflow indicates the context window limit was exceeded.
	ErrContextOverflow = errors.New("context window overflow")
	// ErrInvalidTokenLimit indicates an invalid token limit.
	ErrInvalidTokenLimit = errors.New("invalid token limit")
	// ErrEmptyContext indicates the context is empty.
	ErrEmptyContext = errors.New("context is empty")
)

// windowState is the immutable state published under ContextWindow.state.
// Every mutation produces a fresh *windowState via CAS swap; readers load
// a stable snapshot without any lock. The invariant
// `tokenCount == Σ entries[i].TokenCount` is preserved structurally —
// each writer constructs a consistent state before publication.
type windowState struct {
	entries    []ContextEntry
	tokenCount int
	lastAccess time.Time
}

// ContextWindow manages the context window for LLM interactions.
//
// CONST-029: backed by atomic.Pointer[windowState] instead of a mu +
// bare slice + int triple. Writes are a CAS-loop over a fresh
// windowState; reads are a single Load. No bare mutex to forget.
type ContextWindow struct {
	state        atomic.Pointer[windowState]
	config       *WindowConfig                          // constructor-set, read-only
	eventHandler atomic.Pointer[WindowEventHandler] // cold-path swap
}

// ContextEntry represents an entry in the context window.
type ContextEntry struct {
	// ID is the unique identifier for this entry.
	ID string `json:"id"`
	// Role is the message role (system, user, assistant, tool).
	Role string `json:"role"`
	// Content is the entry content.
	Content string `json:"content"`
	// TokenCount is the number of tokens in this entry.
	TokenCount int `json:"token_count"`
	// Timestamp is when this entry was added.
	Timestamp time.Time `json:"timestamp"`
	// Priority determines importance for eviction.
	Priority Priority `json:"priority"`
	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Pinned indicates the entry should not be evicted.
	Pinned bool `json:"pinned"`
}

// Priority levels for context entries.
type Priority int

const (
	// PriorityLow is low priority (evicted first).
	PriorityLow Priority = 0
	// PriorityNormal is normal priority.
	PriorityNormal Priority = 1
	// PriorityHigh is high priority.
	PriorityHigh Priority = 2
	// PriorityCritical is critical priority (evicted last).
	PriorityCritical Priority = 3
)

// WindowConfig holds configuration for the context window.
type WindowConfig struct {
	// MaxTokens is the maximum tokens allowed.
	MaxTokens int `json:"max_tokens"`
	// ReserveTokens is the number of tokens to reserve for output.
	ReserveTokens int `json:"reserve_tokens"`
	// EvictionPolicy determines how to evict entries.
	EvictionPolicy EvictionPolicy `json:"eviction_policy"`
	// EvictionThreshold triggers eviction when usage exceeds this.
	EvictionThreshold float64 `json:"eviction_threshold"`
	// PreserveSystemPrompt keeps the system prompt from eviction.
	PreserveSystemPrompt bool `json:"preserve_system_prompt"`
	// PreserveLastN keeps the last N entries from eviction.
	PreserveLastN int `json:"preserve_last_n"`
}

// EvictionPolicy defines how entries are evicted.
type EvictionPolicy string

const (
	// EvictionPolicyFIFO evicts oldest entries first.
	EvictionPolicyFIFO EvictionPolicy = "fifo"
	// EvictionPolicyLRU evicts least recently used.
	EvictionPolicyLRU EvictionPolicy = "lru"
	// EvictionPolicyPriority evicts lowest priority first.
	EvictionPolicyPriority EvictionPolicy = "priority"
	// EvictionPolicySummarize summarizes older entries.
	EvictionPolicySummarize EvictionPolicy = "summarize"
)

// DefaultWindowConfig returns a default configuration.
func DefaultWindowConfig() *WindowConfig {
	return &WindowConfig{
		MaxTokens:            4096,
		ReserveTokens:        512,
		EvictionPolicy:       EvictionPolicyFIFO,
		EvictionThreshold:    0.9,
		PreserveSystemPrompt: true,
		PreserveLastN:        2,
	}
}

// WindowEventHandler handles window events.
type WindowEventHandler func(event *WindowEvent)

// WindowEvent represents an event in the context window.
type WindowEvent struct {
	// Type is the event type.
	Type WindowEventType `json:"type"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Entry is the affected entry (if applicable).
	Entry *ContextEntry `json:"entry,omitempty"`
	// TokenCount is the current token count.
	TokenCount int `json:"token_count"`
	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// WindowEventType defines types of window events.
type WindowEventType string

const (
	// EventTypeEntryAdded indicates an entry was added.
	EventTypeEntryAdded WindowEventType = "entry_added"
	// EventTypeEntryEvicted indicates an entry was evicted.
	EventTypeEntryEvicted WindowEventType = "entry_evicted"
	// EventTypeEntryUpdated indicates an entry was updated.
	EventTypeEntryUpdated WindowEventType = "entry_updated"
	// EventTypeOverflow indicates context overflow.
	EventTypeOverflow WindowEventType = "overflow"
	// EventTypeSummarized indicates context was summarized.
	EventTypeSummarized WindowEventType = "summarized"
)

// NewContextWindow creates a new context window.
func NewContextWindow(config *WindowConfig) *ContextWindow {
	if config == nil {
		config = DefaultWindowConfig()
	}
	w := &ContextWindow{
		config: config,
	}
	w.state.Store(&windowState{
		entries:    make([]ContextEntry, 0),
		tokenCount: 0,
		lastAccess: time.Now(),
	})
	return w
}

// SetEventHandler sets the event handler.
func (w *ContextWindow) SetEventHandler(handler WindowEventHandler) {
	if handler == nil {
		w.eventHandler.Store(nil)
		return
	}
	w.eventHandler.Store(&handler)
}

// Add adds an entry to the context window.
func (w *ContextWindow) Add(entry ContextEntry) error {
	if entry.TokenCount == 0 {
		entry.TokenCount = estimateTokens(entry.Content)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.ID == "" {
		entry.ID = generateID()
	}

	availableTokens := w.config.MaxTokens - w.config.ReserveTokens

	for {
		prev := w.state.Load()

		// Decide eviction plan against the observed snapshot.
		entries := prev.entries
		tokenCount := prev.tokenCount
		overflow := false

		if tokenCount+entry.TokenCount > availableTokens {
			needed := tokenCount + entry.TokenCount - availableTokens
			newEntries, evicted, ok := w.planEviction(entries, needed)
			if !ok {
				overflow = true
			} else {
				entries = newEntries
				tokenCount -= evicted
			}
		}

		if overflow {
			w.emitEvent(EventTypeOverflow, nil, prev.tokenCount)
			return ErrContextOverflow
		}

		nextEntries := make([]ContextEntry, 0, len(entries)+1)
		nextEntries = append(nextEntries, entries...)
		nextEntries = append(nextEntries, entry)

		next := &windowState{
			entries:    nextEntries,
			tokenCount: tokenCount + entry.TokenCount,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			w.emitEvent(EventTypeEntryAdded, &entry, next.tokenCount)
			return nil
		}
	}
}

// AddMessage adds a message to the context window.
func (w *ContextWindow) AddMessage(role, content string) error {
	return w.Add(ContextEntry{
		Role:     role,
		Content:  content,
		Priority: PriorityNormal,
	})
}

// AddSystemPrompt adds a system prompt (pinned).
func (w *ContextWindow) AddSystemPrompt(content string) error {
	return w.Add(ContextEntry{
		Role:     "system",
		Content:  content,
		Priority: PriorityCritical,
		Pinned:   w.config.PreserveSystemPrompt,
	})
}

// Get returns all entries in the context window.
func (w *ContextWindow) Get() []ContextEntry {
	snap := w.state.Load()
	result := make([]ContextEntry, len(snap.entries))
	copy(result, snap.entries)
	return result
}

// GetMessages returns entries formatted as messages.
func (w *ContextWindow) GetMessages() []map[string]string {
	snap := w.state.Load()
	messages := make([]map[string]string, len(snap.entries))
	for i, entry := range snap.entries {
		messages[i] = map[string]string{
			"role":    entry.Role,
			"content": entry.Content,
		}
	}
	return messages
}

// TokenCount returns the current token count.
func (w *ContextWindow) TokenCount() int {
	return w.state.Load().tokenCount
}

// AvailableTokens returns the number of tokens available.
func (w *ContextWindow) AvailableTokens() int {
	return w.config.MaxTokens - w.config.ReserveTokens - w.state.Load().tokenCount
}

// UsageRatio returns the context window usage ratio (0-1).
func (w *ContextWindow) UsageRatio() float64 {
	maxUsable := w.config.MaxTokens - w.config.ReserveTokens
	if maxUsable <= 0 {
		return 1.0
	}
	return float64(w.state.Load().tokenCount) / float64(maxUsable)
}

// Clear clears all entries from the context window.
func (w *ContextWindow) Clear() {
	for {
		prev := w.state.Load()
		next := &windowState{
			entries:    make([]ContextEntry, 0),
			tokenCount: 0,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			return
		}
	}
}

// ClearExceptPinned clears all except pinned entries.
func (w *ContextWindow) ClearExceptPinned() {
	for {
		prev := w.state.Load()
		var preserved []ContextEntry
		preservedTokens := 0
		for _, entry := range prev.entries {
			if entry.Pinned {
				preserved = append(preserved, entry)
				preservedTokens += entry.TokenCount
			}
		}
		next := &windowState{
			entries:    preserved,
			tokenCount: preservedTokens,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			return
		}
	}
}

// RemoveEntry removes an entry by ID.
func (w *ContextWindow) RemoveEntry(id string) bool {
	for {
		prev := w.state.Load()
		idx := -1
		for i, entry := range prev.entries {
			if entry.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return false
		}
		removed := prev.entries[idx]
		nextEntries := make([]ContextEntry, 0, len(prev.entries)-1)
		nextEntries = append(nextEntries, prev.entries[:idx]...)
		nextEntries = append(nextEntries, prev.entries[idx+1:]...)
		next := &windowState{
			entries:    nextEntries,
			tokenCount: prev.tokenCount - removed.TokenCount,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			w.emitEvent(EventTypeEntryEvicted, &removed, next.tokenCount)
			return true
		}
	}
}

// UpdateEntry updates an existing entry.
func (w *ContextWindow) UpdateEntry(id string, newContent string) error {
	for {
		prev := w.state.Load()
		idx := -1
		for i, entry := range prev.entries {
			if entry.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return errors.New("entry not found")
		}
		oldTokens := prev.entries[idx].TokenCount
		newTokens := estimateTokens(newContent)
		tokenDiff := newTokens - oldTokens
		if tokenDiff > 0 && prev.tokenCount+tokenDiff > w.config.MaxTokens-w.config.ReserveTokens {
			return ErrContextOverflow
		}

		nextEntries := make([]ContextEntry, len(prev.entries))
		copy(nextEntries, prev.entries)
		nextEntries[idx].Content = newContent
		nextEntries[idx].TokenCount = newTokens
		updated := nextEntries[idx]

		next := &windowState{
			entries:    nextEntries,
			tokenCount: prev.tokenCount + tokenDiff,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			w.emitEvent(EventTypeEntryUpdated, &updated, next.tokenCount)
			return nil
		}
	}
}

// planEviction computes the post-eviction entry list for the currently
// configured eviction policy. Returns (newEntries, evictedTokens, ok).
// ok == false means eviction could not free enough tokens — caller
// signals overflow.
func (w *ContextWindow) planEviction(entries []ContextEntry, tokensNeeded int) ([]ContextEntry, int, bool) {
	switch w.config.EvictionPolicy {
	case EvictionPolicyPriority:
		return w.planEvictByPriority(entries, tokensNeeded)
	case EvictionPolicyLRU:
		// LRU currently approximated by FIFO (see legacy implementation).
		return w.planEvictFIFO(entries, tokensNeeded)
	case EvictionPolicyFIFO:
		fallthrough
	default:
		return w.planEvictFIFO(entries, tokensNeeded)
	}
}

func (w *ContextWindow) planEvictFIFO(entries []ContextEntry, tokensNeeded int) ([]ContextEntry, int, bool) {
	evicted := 0
	preserveFrom := len(entries) - w.config.PreserveLastN
	remaining := make([]ContextEntry, 0, len(entries))

	for i, entry := range entries {
		if entry.Pinned || (w.config.PreserveSystemPrompt && entry.Role == "system") || i >= preserveFrom {
			remaining = append(remaining, entry)
			continue
		}
		if evicted >= tokensNeeded {
			remaining = append(remaining, entry)
			continue
		}
		evicted += entry.TokenCount
		evictedEntry := entry
		w.emitEvent(EventTypeEntryEvicted, &evictedEntry, 0)
	}

	if evicted < tokensNeeded {
		return entries, 0, false
	}
	return remaining, evicted, true
}

func (w *ContextWindow) planEvictByPriority(entries []ContextEntry, tokensNeeded int) ([]ContextEntry, int, bool) {
	evicted := 0
	current := entries
	preserveFrom := len(entries) - w.config.PreserveLastN

	for priority := PriorityLow; priority <= PriorityCritical && evicted < tokensNeeded; priority++ {
		remaining := make([]ContextEntry, 0, len(current))
		for i, entry := range current {
			if entry.Priority != priority || entry.Pinned || i >= preserveFrom {
				remaining = append(remaining, entry)
				continue
			}
			if evicted >= tokensNeeded {
				remaining = append(remaining, entry)
				continue
			}
			evicted += entry.TokenCount
			evictedEntry := entry
			w.emitEvent(EventTypeEntryEvicted, &evictedEntry, 0)
		}
		current = remaining
	}

	if evicted < tokensNeeded {
		return entries, 0, false
	}
	return current, evicted, true
}

func (w *ContextWindow) emitEvent(eventType WindowEventType, entry *ContextEntry, tokenCount int) {
	handlerPtr := w.eventHandler.Load()
	if handlerPtr == nil {
		return
	}
	handler := *handlerPtr
	if handler == nil {
		return
	}
	handler(&WindowEvent{
		Type:       eventType,
		Timestamp:  time.Now(),
		Entry:      entry,
		TokenCount: tokenCount,
	})
}

// Snapshot returns a snapshot of the context window.
func (w *ContextWindow) Snapshot() *WindowSnapshot {
	snap := w.state.Load()
	entries := make([]ContextEntry, len(snap.entries))
	copy(entries, snap.entries)
	return &WindowSnapshot{
		Entries:    entries,
		TokenCount: snap.tokenCount,
		Timestamp:  time.Now(),
		Config:     *w.config,
	}
}

// WindowSnapshot represents a point-in-time snapshot of the context window.
type WindowSnapshot struct {
	// Entries are the context entries.
	Entries []ContextEntry `json:"entries"`
	// TokenCount is the total token count.
	TokenCount int `json:"token_count"`
	// Timestamp is when the snapshot was taken.
	Timestamp time.Time `json:"timestamp"`
	// Config is the window configuration.
	Config WindowConfig `json:"config"`
}

// RestoreFromSnapshot restores the context window from a snapshot.
func (w *ContextWindow) RestoreFromSnapshot(snapshot *WindowSnapshot) {
	for {
		prev := w.state.Load()
		entries := make([]ContextEntry, len(snapshot.Entries))
		copy(entries, snapshot.Entries)
		next := &windowState{
			entries:    entries,
			tokenCount: snapshot.TokenCount,
			lastAccess: time.Now(),
		}
		if w.state.CompareAndSwap(prev, next) {
			return
		}
	}
}

// Helper functions

func estimateTokens(text string) int {
	// Simple approximation: ~4 characters per token
	return len(text) / 4
}

var idCounter int64
var idMu sync.Mutex

func generateID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return strings.ReplaceAll(time.Now().Format("20060102150405"), ".", "") + "_" + strconv.FormatInt(idCounter, 10)
}

// WindowStats contains statistics about the context window.
type WindowStats struct {
	// TotalEntries is the total number of entries.
	TotalEntries int `json:"total_entries"`
	// TotalTokens is the total token count.
	TotalTokens int `json:"total_tokens"`
	// AvailableTokens is the available token count.
	AvailableTokens int `json:"available_tokens"`
	// UsageRatio is the usage ratio (0-1).
	UsageRatio float64 `json:"usage_ratio"`
	// PinnedEntries is the number of pinned entries.
	PinnedEntries int `json:"pinned_entries"`
	// MessagesByRole counts messages by role.
	MessagesByRole map[string]int `json:"messages_by_role"`
	// AverageEntrySize is the average entry token count.
	AverageEntrySize float64 `json:"average_entry_size"`
}

// Stats returns statistics about the context window.
func (w *ContextWindow) Stats() *WindowStats {
	snap := w.state.Load()

	stats := &WindowStats{
		TotalEntries:    len(snap.entries),
		TotalTokens:     snap.tokenCount,
		AvailableTokens: w.config.MaxTokens - w.config.ReserveTokens - snap.tokenCount,
		MessagesByRole:  make(map[string]int),
	}

	if len(snap.entries) > 0 {
		stats.AverageEntrySize = float64(snap.tokenCount) / float64(len(snap.entries))
	}

	maxUsable := w.config.MaxTokens - w.config.ReserveTokens
	if maxUsable > 0 {
		stats.UsageRatio = float64(snap.tokenCount) / float64(maxUsable)
	}

	for _, entry := range snap.entries {
		stats.MessagesByRole[entry.Role]++
		if entry.Pinned {
			stats.PinnedEntries++
		}
	}

	return stats
}
