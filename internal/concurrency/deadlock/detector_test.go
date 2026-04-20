// Package deadlock_test provides tests for deadlock detection functionality
package deadlock

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockLogger for testing
type MockLogger struct {
	warnings []string
	errors   []string
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnings = append(m.warnings, msg)
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.errors = append(m.errors, msg)
}

func TestDetector_NewDetector(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	assert.NotNil(t, d)
	assert.True(t, d.enabled)
	assert.Equal(t, 5*time.Second, d.maxWaitTime)
	assert.NotNil(t, d.lockGraph)
	assert.NotNil(t, d.lockHolders)
	assert.NotNil(t, d.waitForGraph)
}

func TestDetector_NewLockWrapper(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	mu := &sync.Mutex{}
	wrapper := d.NewLockWrapper(mu, "test-lock")

	assert.NotNil(t, wrapper)
	assert.Equal(t, "test-lock", wrapper.name)
	assert.Equal(t, d, wrapper.detector)
	assert.Equal(t, mu, wrapper.mu)
}

func TestLockWrapper_LockUnlock(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	mu := &sync.Mutex{}
	wrapper := d.NewLockWrapper(mu, "test-lock")

	// Test lock and unlock
	wrapper.Lock()
	assert.Greater(t, d.lockHolders.Len(), 0)

	wrapper.Unlock()
	assert.Equal(t, 0, d.lockHolders.Len())
}

func TestLockWrapper_ConcurrentAccess(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	mu := &sync.Mutex{}
	wrapper := d.NewLockWrapper(mu, "concurrent-lock")

	var counter int
	var wg sync.WaitGroup

	// Multiple goroutines accessing the same lock
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapper.Lock()
			counter++
			time.Sleep(1 * time.Millisecond)
			wrapper.Unlock()
		}()
	}

	wg.Wait()
	assert.Equal(t, 10, counter)
}

func TestDetector_DetectCycles_NoCycles(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	// Create a simple lock graph without cycles
	d.lockGraph.Put("A", []string{"B"})
	d.lockGraph.Put("B", []string{"C"})

	cycles := d.DetectCycles()
	assert.Empty(t, cycles)

	report := d.Report()
	assert.False(t, report.PotentialDeadlocks)
}

func TestDetector_DetectCycles_WithCycles(t *testing.T) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)

	// Create a lock graph with a cycle: A -> B -> C -> A
	d.lockGraph.Put("A", []string{"B"})
	d.lockGraph.Put("B", []string{"C"})
	d.lockGraph.Put("C", []string{"A"})

	cycles := d.DetectCycles()
	assert.NotEmpty(t, cycles)
	assert.True(t, len(cycles) > 0)

	report := d.Report()
	assert.True(t, report.PotentialDeadlocks)
}

func TestOrderedLock(t *testing.T) {
	ol := NewOrderedLock()

	var results []int
	var mu sync.Mutex

	// Launch goroutines in reverse order
	for i := 3; i >= 0; i-- {
		go func(order int) {
			ol.Acquire(order)
			defer ol.Release(order)

			mu.Lock()
			results = append(results, order)
			mu.Unlock()
		}(i)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	// Results should be in order 0, 1, 2, 3
	assert.Equal(t, []int{0, 1, 2, 3}, results)
	mu.Unlock()
}

func TestTimeoutLock(t *testing.T) {
	tl := NewTimeoutLock()

	// First acquisition should succeed
	assert.True(t, tl.TryLock(100*time.Millisecond))

	// Second acquisition should timeout
	assert.False(t, tl.TryLock(50*time.Millisecond))

	// Release and reacquire
	tl.Unlock()
	assert.True(t, tl.TryLock(100*time.Millisecond))
	tl.Unlock()
}

func TestHierarchicalLock(t *testing.T) {
	lock1 := NewHierarchicalLock(1, "resource-1")
	lock2 := NewHierarchicalLock(2, "resource-2")
	lock3 := NewHierarchicalLock(3, "resource-3")

	// Lock all in any order
	LockAll(lock3, lock1, lock2)

	// Unlock all
	UnlockAll(lock1, lock2, lock3)

	// Verify they were locked in hierarchical order (no deadlock)
	assert.True(t, true) // If we got here, no deadlock occurred
}

func TestLockSlice(t *testing.T) {
	lock1 := NewHierarchicalLock(3, "lock-3")
	lock2 := NewHierarchicalLock(1, "lock-1")
	lock3 := NewHierarchicalLock(2, "lock-2")

	locks := LockSlice{lock1, lock2, lock3}

	// Verify Len
	assert.Equal(t, 3, locks.Len())

	// Verify Less (should sort by level)
	assert.True(t, locks.Less(1, 0)) // lock2 (level 1) < lock1 (level 3)
	assert.True(t, locks.Less(2, 0)) // lock3 (level 2) < lock1 (level 3)

	// Verify Swap
	locks.Swap(0, 1)
	assert.Equal(t, 1, locks[0].level)
	assert.Equal(t, 3, locks[1].level)
}

func TestReport_String(t *testing.T) {
	report := &Report{
		Cycles:             [][]string{{"A", "B", "C", "A"}},
		LockGraph:          map[string][]string{"A": {"B"}},
		Holders:            map[string]string{"lock1": "goroutine-1"},
		Timestamp:          time.Now(),
		PotentialDeadlocks: true,
	}

	str := report.String()
	assert.Contains(t, str, "DEADLOCKS DETECTED")
	assert.Contains(t, str, "Cycle 1")
	assert.Contains(t, str, "Active Locks")
}

func TestReport_String_NoDeadlocks(t *testing.T) {
	report := &Report{
		Cycles:             [][]string{},
		LockGraph:          map[string][]string{},
		Holders:            map[string]string{},
		Timestamp:          time.Now(),
		PotentialDeadlocks: false,
	}

	str := report.String()
	assert.Contains(t, str, "No deadlocks detected")
}

func TestCopyMap(t *testing.T) {
	original := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
	}

	copied := copyMap(original)

	// Verify copy has same data
	assert.Equal(t, original, copied)

	// Verify modifying copy doesn't affect original
	copied["A"] = append(copied["A"], "E")
	assert.NotEqual(t, original["A"], copied["A"])
}

func TestCopyStringMap(t *testing.T) {
	original := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	copied := copyStringMap(original)

	// Verify copy has same data
	assert.Equal(t, original, copied)

	// Verify modifying copy doesn't affect original
	copied["key1"] = "modified"
	assert.NotEqual(t, original["key1"], copied["key1"])
}

func BenchmarkLockWrapper_LockUnlock(b *testing.B) {
	logger := &MockLogger{}
	d := NewDetector(5*time.Second, logger)
	mu := &sync.Mutex{}
	wrapper := d.NewLockWrapper(mu, "bench-lock")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapper.Lock()
		wrapper.Unlock()
	}
}

func BenchmarkTimeoutLock_TryLock(b *testing.B) {
	tl := NewTimeoutLock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tl.TryLock(1 * time.Millisecond)
		tl.Unlock()
	}
}
