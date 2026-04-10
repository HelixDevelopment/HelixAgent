// Package dream provides the memory consolidation system
// Inspired by Claude Code's Dream system for self-healing memory
package dream

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DreamerConfig configures the Dream system
type DreamerConfig struct {
	Enabled          bool          `json:"enabled"`
	MemoryDir        string        `json:"memory_dir"`
	TimeThreshold    time.Duration `json:"time_threshold"`    // 24 hours
	MinSessions      int           `json:"min_sessions"`      // 5 sessions
	ConsolidationInterval time.Duration `json:"consolidation_interval"`
}

// DefaultConfig returns default configuration
func DefaultConfig() DreamerConfig {
	homeDir, _ := os.UserHomeDir()
	return DreamerConfig{
		Enabled:               true,
		MemoryDir:             filepath.Join(homeDir, ".helixagent", "memory"),
		TimeThreshold:         24 * time.Hour,
		MinSessions:           5,
		ConsolidationInterval: 1 * time.Hour,
	}
}

// DreamPhase represents a phase of the dream process
type DreamPhase string

const (
	// PhaseOrientation - Read MEMORY.md, list directory
	PhaseOrientation DreamPhase = "ORIENTATION"
	// PhaseGather - Search for new information
	PhaseGather DreamPhase = "GATHER_SIGNALS"
	// PhaseConsolidate - Write/update memory files
	PhaseConsolidate DreamPhase = "CONSOLIDATION"
	// PhaseCleanup - Maintain MEMORY.md size
	PhaseCleanup DreamPhase = "CLEANUP_INDEXING"
)

// DreamState represents the current state of a dream
type DreamState string

const (
	DreamStatePending    DreamState = "pending"
	DreamStateRunning    DreamState = "running"
	DreamStateCompleted  DreamState = "completed"
	DreamStateFailed     DreamState = "failed"
	DreamStateCancelled  DreamState = "cancelled"
)

// DreamTrigger represents the three-gate trigger system
type DreamTrigger struct {
	LastDreamTime    time.Time     `json:"last_dream_time"`
	SessionCount     int           `json:"session_count"`
	SessionsSinceDream int         `json:"sessions_since_dream"`
	TimeThreshold    time.Duration `json:"time_threshold"`
	MinSessions      int           `json:"min_sessions"`
	Locked           bool          `json:"locked"`
}

// DreamSession represents a complete dream session
type DreamSession struct {
	ID              string                 `json:"id"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	State           DreamState             `json:"state"`
	CurrentPhase    DreamPhase             `json:"current_phase"`
	Phases          []PhaseResult          `json:"phases"`
	NewMemories     []MemoryEntry          `json:"new_memories"`
	UpdatedMemories []MemoryEntry          `json:"updated_memories"`
	RemovedMemories []string               `json:"removed_memories"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PhaseResult represents the result of a dream phase
type PhaseResult struct {
	Phase     DreamPhase    `json:"phase"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Success   bool          `json:"success"`
	Details   string        `json:"details,omitempty"`
}

// MemoryEntry represents a consolidated memory
type MemoryEntry struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"` // pattern, fact, preference, project
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Confidence  float64                `json:"confidence"` // 0.0 - 1.0
	Source      string                 `json:"source"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessCount int                    `json:"access_count"`
	LastAccess  time.Time              `json:"last_access"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MEMORY.md structure
type MemoryIndex struct {
	Version     string         `json:"version"`
	LastUpdated time.Time      `json:"last_updated"`
	Categories  []string       `json:"categories"`
	Entries     []MemorySummary `json:"entries"`
}

type MemorySummary struct {
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
}

// Dreamer is the memory consolidation engine
type Dreamer struct {
	config    DreamerConfig
	logger    *logrus.Logger
	trigger   DreamTrigger
	sessions  []DreamSession
	memories  map[string]MemoryEntry
	current   *DreamSession
	mu        sync.RWMutex
	running   bool
	stopCh    chan struct{}
	
	// Callbacks
	onPhaseStart   func(DreamPhase)
	onPhaseEnd     func(DreamPhase, bool, string)
	onMemoryAdded  func(MemoryEntry)
	onMemoryUpdated func(MemoryEntry)
}

// NewDreamer creates a new Dream system
func NewDreamer(config DreamerConfig, logger *logrus.Logger) *Dreamer {
	return &Dreamer{
		config:   config,
		logger:   logger,
		trigger:  DreamTrigger{
			TimeThreshold: config.TimeThreshold,
			MinSessions:   config.MinSessions,
		},
		sessions:  make([]DreamSession, 0),
		memories:  make(map[string]MemoryEntry),
		stopCh:    make(chan struct{}),
		onPhaseStart:    func(p DreamPhase) {},
		onPhaseEnd:      func(p DreamPhase, success bool, details string) {},
		onMemoryAdded:   func(m MemoryEntry) {},
		onMemoryUpdated: func(m MemoryEntry) {},
	}
}

// SetCallbacks sets the dreamer callbacks
func (d *Dreamer) SetCallbacks(
	onPhaseStart func(DreamPhase),
	onPhaseEnd func(DreamPhase, bool, string),
	onMemoryAdded func(MemoryEntry),
	onMemoryUpdated func(MemoryEntry),
) {
	d.onPhaseStart = onPhaseStart
	d.onPhaseEnd = onPhaseEnd
	d.onMemoryAdded = onMemoryAdded
	d.onMemoryUpdated = onMemoryUpdated
}

// Start starts the Dream system
func (d *Dreamer) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.running {
		return fmt.Errorf("Dreamer already running")
	}
	
	if !d.config.Enabled {
		d.logger.Info("Dream system is disabled")
		return nil
	}
	
	// Ensure memory directory exists
	if err := os.MkdirAll(d.config.MemoryDir, 0750); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}
	
	// Load existing memories
	d.loadMemories()
	
	d.running = true
	go d.run(ctx)
	
	d.logger.Info("Dream system started")
	return nil
}

// Stop stops the Dream system
func (d *Dreamer) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if !d.running {
		return nil
	}
	
	close(d.stopCh)
	d.running = false
	
	// Save memories
	d.saveMemories()
	
	d.logger.Info("Dream system stopped")
	return nil
}

// IsRunning returns true if the dreamer is running
func (d *Dreamer) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// run is the main dream loop
func (d *Dreamer) run(ctx context.Context) {
	ticker := time.NewTicker(d.config.ConsolidationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			d.Stop()
			return
		case <-ticker.C:
			if d.ShouldDream() {
				d.logger.Info("Triggering dream session")
				if _, err := d.Dream(ctx); err != nil {
					d.logger.WithError(err).Error("Dream session failed")
				}
			}
		}
	}
}

// ShouldDream checks if all three gates are open
func (d *Dreamer) ShouldDream() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	// Time Gate: 24 hours since last dream
	if time.Since(d.trigger.LastDreamTime) < d.trigger.TimeThreshold {
		return false
	}
	
	// Session Gate: Minimum 5 sessions since last dream
	if d.trigger.SessionsSinceDream < d.trigger.MinSessions {
		return false
	}
	
	// Lock Gate: Not already dreaming
	if d.trigger.Locked {
		return false
	}
	
	return true
}

// RecordSession records that a session occurred
func (d *Dreamer) RecordSession() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.trigger.SessionCount++
	d.trigger.SessionsSinceDream++
}

// Dream initiates a dream session
func (d *Dreamer) Dream(ctx context.Context) (*DreamSession, error) {
	d.mu.Lock()
	
	// Acquire lock
	if d.trigger.Locked {
		d.mu.Unlock()
		return nil, fmt.Errorf("dream already in progress")
	}
	d.trigger.Locked = true
	
	// Create session
	session := &DreamSession{
		ID:           generateDreamID(),
		StartedAt:    time.Now(),
		State:        DreamStateRunning,
		CurrentPhase: PhaseOrientation,
		Phases:       make([]PhaseResult, 0),
		NewMemories:  make([]MemoryEntry, 0),
		UpdatedMemories: make([]MemoryEntry, 0),
		RemovedMemories: make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
	d.current = session
	d.mu.Unlock()
	
	d.logger.Infof("Starting dream session %s", session.ID)
	
	// Execute phases
	d.executePhase(ctx, session, PhaseOrientation, d.orientationPhase)
	d.executePhase(ctx, session, PhaseGather, d.gatherPhase)
	d.executePhase(ctx, session, PhaseConsolidate, d.consolidationPhase)
	d.executePhase(ctx, session, PhaseCleanup, d.cleanupPhase)
	
	// Complete session
	d.mu.Lock()
	session.State = DreamStateCompleted
	now := time.Now()
	session.CompletedAt = &now
	d.sessions = append(d.sessions, *session)
	
	// Update trigger
	d.trigger.LastDreamTime = now
	d.trigger.SessionsSinceDream = 0
	d.trigger.Locked = false
	
	d.current = nil
	d.mu.Unlock()
	
	d.logger.Infof("Dream session %s completed", session.ID)
	
	// Save memories
	d.saveMemories()
	
	return session, nil
}

// executePhase executes a single dream phase
func (d *Dreamer) executePhase(ctx context.Context, session *DreamSession, phase DreamPhase, fn func(context.Context, *DreamSession) error) {
	d.mu.Lock()
	session.CurrentPhase = phase
	phaseResult := PhaseResult{
		Phase:     phase,
		StartedAt: time.Now(),
	}
	d.mu.Unlock()
	
	d.onPhaseStart(phase)
	d.logger.Debugf("Dream phase: %s", phase)
	
	err := fn(ctx, session)
	
	now := time.Now()
	phaseResult.EndedAt = &now
	phaseResult.Success = err == nil
	if err != nil {
		phaseResult.Details = err.Error()
	}
	
	d.mu.Lock()
	session.Phases = append(session.Phases, phaseResult)
	d.mu.Unlock()
	
	d.onPhaseEnd(phase, phaseResult.Success, phaseResult.Details)
}

// orientationPhase: Phase 1 - Read MEMORY.md, list directory
func (d *Dreamer) orientationPhase(ctx context.Context, session *DreamSession) error {
	// Read MEMORY.md
	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")
	content, err := os.ReadFile(memoryMdPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	
	session.Metadata["memory_md_size"] = len(content)
	
	// List memory directory
	entries, err := os.ReadDir(d.config.MemoryDir)
	if err != nil {
		return err
	}
	
	var memoryFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			memoryFiles = append(memoryFiles, entry.Name())
		}
	}
	
	session.Metadata["memory_file_count"] = len(memoryFiles)
	d.logger.Debugf("Found %d memory files", len(memoryFiles))
	
	return nil
}

// gatherPhase: Phase 2 - Search for new information
func (d *Dreamer) gatherPhase(ctx context.Context, session *DreamSession) error {
	// This would search for:
	// - Daily logs from KAIROS
	// - Drifting memories (unsaved observations)
	// - Transcript search
	
	// For now, this is a placeholder that would integrate with other systems
	d.logger.Debug("Gathering fresh signals")
	
	return nil
}

// consolidationPhase: Phase 3 - Write/update memory files
func (d *Dreamer) consolidationPhase(ctx context.Context, session *DreamSession) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// Process gathered information and create/update memories
	// This would:
	// - Extract patterns from sessions
	// - Identify facts worth preserving
	// - Update existing memories with new information
	// - Translate relative dates to absolute
	// - Remove disproven facts
	
	d.logger.Debug("Consolidating memories")
	
	return nil
}

// cleanupPhase: Phase 4 - Maintain MEMORY.md size
func (d *Dreamer) cleanupPhase(ctx context.Context, session *DreamSession) error {
	// Keep MEMORY.md within 200 lines (~25KB)
	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")
	
	content, err := os.ReadFile(memoryMdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	lines := strings.Split(string(content), "\n")
	if len(lines) > 200 {
		// Remove stale pointers and resolve contradictions
		// For now, just trim to last 200 lines
		lines = lines[len(lines)-200:]
		newContent := strings.Join(lines, "\n")
		
		if err := os.WriteFile(memoryMdPath, []byte(newContent), 0644); err != nil {
			return err
		}
		
		d.logger.Debug("Trimmed MEMORY.md to 200 lines")
	}
	
	return nil
}

// AddMemory adds a new memory
func (d *Dreamer) AddMemory(entry MemoryEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if entry.ID == "" {
		entry.ID = generateMemoryID()
	}
	
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt
	
	d.memories[entry.ID] = entry
	d.onMemoryAdded(entry)
	
	return nil
}

// UpdateMemory updates an existing memory
func (d *Dreamer) UpdateMemory(id string, updates map[string]interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	memory, exists := d.memories[id]
	if !exists {
		return fmt.Errorf("memory not found: %s", id)
	}
	
	// Apply updates
	if content, ok := updates["content"].(string); ok {
		memory.Content = content
	}
	if confidence, ok := updates["confidence"].(float64); ok {
		memory.Confidence = confidence
	}
	if tags, ok := updates["tags"].([]string); ok {
		memory.Tags = tags
	}
	
	memory.UpdatedAt = time.Now()
	d.memories[id] = memory
	d.onMemoryUpdated(memory)
	
	return nil
}

// GetMemory retrieves a memory by ID
func (d *Dreamer) GetMemory(id string) (MemoryEntry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	memory, exists := d.memories[id]
	return memory, exists
}

// GetMemoriesByCategory returns memories in a category
func (d *Dreamer) GetMemoriesByCategory(category string) []MemoryEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	var result []MemoryEntry
	for _, memory := range d.memories {
		if memory.Category == category {
			result = append(result, memory)
		}
	}
	
	return result
}

// GetAllMemories returns all memories
func (d *Dreamer) GetAllMemories() []MemoryEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	result := make([]MemoryEntry, 0, len(d.memories))
	for _, memory := range d.memories {
		result = append(result, memory)
	}
	
	return result
}

// GetCurrentSession returns the current dream session
func (d *Dreamer) GetCurrentSession() *DreamSession {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	if d.current == nil {
		return nil
	}
	
	session := *d.current
	return &session
}

// GetSessions returns all dream sessions
func (d *Dreamer) GetSessions() []DreamSession {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	result := make([]DreamSession, len(d.sessions))
	copy(result, d.sessions)
	return result
}

// loadMemories loads memories from disk
func (d *Dreamer) loadMemories() {
	entries, err := os.ReadDir(d.config.MemoryDir)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		
		path := filepath.Join(d.config.MemoryDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		
		var memory MemoryEntry
		if err := json.Unmarshal(data, &memory); err != nil {
			continue
		}
		
		d.memories[memory.ID] = memory
	}
	
	d.logger.Infof("Loaded %d memories", len(d.memories))
}

// saveMemories saves memories to disk
func (d *Dreamer) saveMemories() {
	for id, memory := range d.memories {
		path := filepath.Join(d.config.MemoryDir, fmt.Sprintf("%s.json", id))
		
		data, err := json.MarshalIndent(memory, "", "  ")
		if err != nil {
			continue
		}
		
		os.WriteFile(path, data, 0644)
	}
	
	// Update MEMORY.md index
	d.updateMemoryIndex()
}

// updateMemoryIndex updates the MEMORY.md file
func (d *Dreamer) updateMemoryIndex() {
	index := MemoryIndex{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Categories:  []string{"pattern", "fact", "preference", "project"},
		Entries:     make([]MemorySummary, 0, len(d.memories)),
	}
	
	for _, memory := range d.memories {
		index.Entries = append(index.Entries, MemorySummary{
			ID:       memory.ID,
			Category: memory.Category,
			Title:    memory.Title,
			Tags:     memory.Tags,
		})
	}
	
	// Generate markdown content
	var content strings.Builder
	content.WriteString("# HelixAgent Memory\n\n")
	content.WriteString(fmt.Sprintf("Last Updated: %s\n\n", index.LastUpdated.Format(time.RFC3339)))
	
	for _, category := range index.Categories {
		content.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(category)))
		
		for _, entry := range index.Entries {
			if entry.Category == category {
				content.WriteString(fmt.Sprintf("- **%s** (%s)\n", entry.Title, strings.Join(entry.Tags, ", ")))
			}
		}
		
		content.WriteString("\n")
	}
	
	memoryMdPath := filepath.Join(d.config.MemoryDir, "MEMORY.md")
	os.WriteFile(memoryMdPath, []byte(content.String()), 0644)
}

// generateDreamID generates a unique dream ID
func generateDreamID() string {
	return fmt.Sprintf("dream_%d", time.Now().UnixNano())
}

// generateMemoryID generates a unique memory ID
func generateMemoryID() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixNano())
}
