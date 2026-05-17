package dream

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 24*time.Hour, config.TimeThreshold)
	assert.Equal(t, 5, config.MinSessions)
	assert.Equal(t, 1*time.Hour, config.ConsolidationInterval)
	assert.NotEmpty(t, config.MemoryDir)
}

func TestNewDreamer(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	assert.NotNil(t, dreamer)
	assert.Equal(t, config, dreamer.config)
	assert.NotNil(t, dreamer.trigger)
	assert.NotNil(t, dreamer.sessions)
	assert.NotNil(t, dreamer.memories)
	assert.NotNil(t, dreamer.stopCh)
	assert.False(t, dreamer.running.Load())
}

func TestDreamer_SetCallbacks(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	phaseStartCalled := false
	phaseEndCalled := false
	memoryAddedCalled := false
	memoryUpdatedCalled := false

	dreamer.SetCallbacks(
		func(p DreamPhase) { phaseStartCalled = true },
		func(p DreamPhase, success bool, details string) { phaseEndCalled = true },
		func(m MemoryEntry) { memoryAddedCalled = true },
		func(m MemoryEntry) { memoryUpdatedCalled = true },
	)

	// Test that callbacks are set
	dreamer.onPhaseStart(PhaseOrientation)
	dreamer.onPhaseEnd(PhaseGather, true, "")
	dreamer.onMemoryAdded(MemoryEntry{})
	dreamer.onMemoryUpdated(MemoryEntry{})

	assert.True(t, phaseStartCalled)
	assert.True(t, phaseEndCalled)
	assert.True(t, memoryAddedCalled)
	assert.True(t, memoryUpdatedCalled)
}

func TestDreamer_StartStop(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.Enabled = true
	config.ConsolidationInterval = 100 * time.Millisecond

	dreamer := NewDreamer(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start dreamer
	err := dreamer.Start(ctx)
	require.NoError(t, err)
	assert.True(t, dreamer.IsRunning())

	// Stop dreamer
	err = dreamer.Stop()
	require.NoError(t, err)
	assert.False(t, dreamer.IsRunning())
}

func TestDreamer_Start_AlreadyRunning(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.Enabled = true
	config.ConsolidationInterval = 100 * time.Millisecond

	dreamer := NewDreamer(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start dreamer
	err := dreamer.Start(ctx)
	require.NoError(t, err)

	// Try to start again
	err = dreamer.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	dreamer.Stop()
}

func TestDreamer_Start_Disabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	config.Enabled = false

	dreamer := NewDreamer(config, logger)

	ctx := context.Background()
	err := dreamer.Start(ctx)
	require.NoError(t, err)
	assert.False(t, dreamer.IsRunning())
}

func TestDreamer_ShouldDream(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Initially should not dream (no time elapsed, no sessions)
	assert.False(t, dreamer.ShouldDream())

	// Set up conditions for dreaming
	dreamer.trigger.LastDreamTime = time.Now().Add(-25 * time.Hour) // > 24 hours ago
	dreamer.trigger.SessionsSinceDream = 10                         // >= 5 sessions
	dreamer.trigger.Locked = false

	assert.True(t, dreamer.ShouldDream())
}

func TestDreamer_ShouldDream_TimeGate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Not enough time elapsed
	dreamer.trigger.LastDreamTime = time.Now().Add(-12 * time.Hour) // < 24 hours
	dreamer.trigger.SessionsSinceDream = 10
	dreamer.trigger.Locked = false

	assert.False(t, dreamer.ShouldDream())
}

func TestDreamer_ShouldDream_SessionGate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Not enough sessions
	dreamer.trigger.LastDreamTime = time.Now().Add(-25 * time.Hour)
	dreamer.trigger.SessionsSinceDream = 3 // < 5 sessions
	dreamer.trigger.Locked = false

	assert.False(t, dreamer.ShouldDream())
}

func TestDreamer_ShouldDream_LockGate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Already locked
	dreamer.trigger.LastDreamTime = time.Now().Add(-25 * time.Hour)
	dreamer.trigger.SessionsSinceDream = 10
	dreamer.trigger.Locked = true

	assert.False(t, dreamer.ShouldDream())
}

func TestDreamer_RecordSession(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	assert.Equal(t, 0, dreamer.trigger.SessionCount)
	assert.Equal(t, 0, dreamer.trigger.SessionsSinceDream)

	dreamer.RecordSession()
	assert.Equal(t, 1, dreamer.trigger.SessionCount)
	assert.Equal(t, 1, dreamer.trigger.SessionsSinceDream)

	dreamer.RecordSession()
	assert.Equal(t, 2, dreamer.trigger.SessionCount)
	assert.Equal(t, 2, dreamer.trigger.SessionsSinceDream)
}

func TestDreamer_AddMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	memoryAddedCalled := false
	dreamer.SetCallbacks(
		func(p DreamPhase) {},
		func(p DreamPhase, success bool, details string) {},
		func(m MemoryEntry) { memoryAddedCalled = true },
		func(m MemoryEntry) {},
	)

	entry := MemoryEntry{
		Category:   "pattern",
		Title:      "Test Pattern",
		Content:    "This is a test pattern",
		Confidence: 0.85,
		Source:     "test",
		Tags:       []string{"test", "pattern"},
	}

	err := dreamer.AddMemory(entry)
	require.NoError(t, err)

	assert.True(t, memoryAddedCalled)
	assert.Equal(t, 1, len(dreamer.GetAllMemories()))
}

func TestDreamer_UpdateMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	memoryUpdatedCalled := false
	dreamer.SetCallbacks(
		func(p DreamPhase) {},
		func(p DreamPhase, success bool, details string) {},
		func(m MemoryEntry) {},
		func(m MemoryEntry) { memoryUpdatedCalled = true },
	)

	// Add a memory first
	entry := MemoryEntry{
		ID:         "mem_123",
		Category:   "fact",
		Title:      "Test Fact",
		Content:    "Original content",
		Confidence: 0.7,
	}
	dreamer.AddMemory(entry)

	// Update the memory
	updates := map[string]interface{}{
		"content":    "Updated content",
		"confidence": 0.9,
	}
	err := dreamer.UpdateMemory("mem_123", updates)
	require.NoError(t, err)

	assert.True(t, memoryUpdatedCalled)

	memories := dreamer.GetAllMemories()
	require.Len(t, memories, 1)
	assert.Equal(t, "Updated content", memories[0].Content)
	assert.Equal(t, 0.9, memories[0].Confidence)
}

func TestDreamer_UpdateMemory_NotFound(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	err := dreamer.UpdateMemory("nonexistent", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDreamer_GetMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	entry := MemoryEntry{
		ID:    "mem_123",
		Title: "Test Memory",
	}
	dreamer.AddMemory(entry)

	// Get existing memory
	memory, exists := dreamer.GetMemory("mem_123")
	assert.True(t, exists)
	assert.Equal(t, "Test Memory", memory.Title)

	// Get non-existent memory
	_, exists = dreamer.GetMemory("nonexistent")
	assert.False(t, exists)
}

func TestDreamer_GetMemoriesByCategory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Add memories of different categories
	dreamer.AddMemory(MemoryEntry{Category: "pattern", Title: "Pattern 1"})
	dreamer.AddMemory(MemoryEntry{Category: "pattern", Title: "Pattern 2"})
	dreamer.AddMemory(MemoryEntry{Category: "fact", Title: "Fact 1"})

	patterns := dreamer.GetMemoriesByCategory("pattern")
	assert.Len(t, patterns, 2)

	facts := dreamer.GetMemoriesByCategory("fact")
	assert.Len(t, facts, 1)

	none := dreamer.GetMemoriesByCategory("nonexistent")
	assert.Len(t, none, 0)
}

func TestDreamer_GetAllMemories(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Initially empty
	memories := dreamer.GetAllMemories()
	assert.Len(t, memories, 0)

	// Add memories
	dreamer.AddMemory(MemoryEntry{Title: "Memory 1"})
	dreamer.AddMemory(MemoryEntry{Title: "Memory 2"})

	memories = dreamer.GetAllMemories()
	assert.Len(t, memories, 2)
}

func TestDreamer_GetCurrentSession(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// No current session initially
	session := dreamer.GetCurrentSession()
	assert.Nil(t, session)
}

func TestDreamer_GetSessions(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	sessions := dreamer.GetSessions()
	assert.Len(t, sessions, 0)
}

func TestGenerateDreamID(t *testing.T) {
	t.Parallel()
	id1 := generateDreamID()
	id2 := generateDreamID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "dream_")
}

func TestGenerateMemoryID(t *testing.T) {
	t.Parallel()
	id1 := generateMemoryID()
	id2 := generateMemoryID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "mem_")
}

func TestDreamer_Dream_Locked(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Manually lock the dreamer
	dreamer.trigger.Locked = true

	ctx := context.Background()

	// Dream should fail (already locked)
	_, err := dreamer.Dream(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

// TestDreamer_Dream_HappyPath fills the gap that close-out⁷⁴ found:
// the existing TestDreamer_Dream_Locked only exercises the lock-rejection
// branch (before any phase work). NO test exercised the happy-path Dream()
// execution. A mutation that replaced all 4 d.executePhase(...) calls with
// `_ = ctx` left ALL 22 dream tests passing. This test catches that bluff.
func TestDreamer_Dream_HappyPath(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Wire phase-callback counters via SetCallbacks so we can prove
	// the 4 dispatches actually fired.
	var startCount, endCount int32
	dreamer.SetCallbacks(
		func(p DreamPhase) {
			atomic.AddInt32(&startCount, 1)
		},
		func(p DreamPhase, _ bool, _ string) {
			atomic.AddInt32(&endCount, 1)
		},
		func(MemoryEntry) {}, // onMemoryAdded — not under test here
		func(MemoryEntry) {}, // onMemoryUpdated — not under test here
	)

	session, err := dreamer.Dream(context.Background())

	// Anti-bluff (close-out⁷⁴): every assertion below catches a different
	// dimension of the 4-phase-skip mutation pattern.
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, DreamStateCompleted, session.State,
		"state must reach Completed after all 4 phases — proves end-of-Dream code ran")
	assert.NotNil(t, session.CompletedAt,
		"CompletedAt must be set — proves completion path executed")
	// Concrete-count: exactly 4 phases executed (Orientation, Gather,
	// Consolidate, Cleanup) — catches mutation that skips any of them.
	assert.Len(t, session.Phases, 4,
		"session must record exactly 4 phase results (Orientation/Gather/Consolidate/Cleanup) — fewer = phase-dispatch skipped")
	// Concrete-order: each phase must appear in the right slot.
	if len(session.Phases) == 4 {
		assert.Equal(t, PhaseOrientation, session.Phases[0].Phase, "phase[0]")
		assert.Equal(t, PhaseGather, session.Phases[1].Phase, "phase[1]")
		assert.Equal(t, PhaseConsolidate, session.Phases[2].Phase, "phase[2]")
		assert.Equal(t, PhaseCleanup, session.Phases[3].Phase, "phase[3]")
	}
	// Callback invocation count: each phase fires onPhaseStart + onPhaseEnd
	// exactly once. With the phase-skip mutation, both stay at 0.
	assert.Equal(t, int32(4), atomic.LoadInt32(&startCount),
		"onPhaseStart must fire exactly 4 times — skipping phases drops this to 0")
	assert.Equal(t, int32(4), atomic.LoadInt32(&endCount),
		"onPhaseEnd must fire exactly 4 times — skipping phases drops this to 0")
	// Trigger-unlock must happen at end of Dream — catches a mutation
	// that returns early after lock acquisition.
	assert.False(t, dreamer.trigger.Locked,
		"trigger.Locked must be released after Dream completes")
}

func BenchmarkDreamer_AddMemory(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	entry := MemoryEntry{
		Category:   "pattern",
		Title:      "Benchmark Pattern",
		Content:    "Benchmark content",
		Confidence: 0.85,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dreamer.AddMemory(entry)
	}
}

func BenchmarkDreamer_GetMemory(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	dreamer := NewDreamer(config, logger)

	// Add some memories first
	for i := 0; i < 100; i++ {
		dreamer.AddMemory(MemoryEntry{
			ID:    fmt.Sprintf("mem_%d", i),
			Title: fmt.Sprintf("Memory %d", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dreamer.GetMemory(fmt.Sprintf("mem_%d", i%100))
	}
}
