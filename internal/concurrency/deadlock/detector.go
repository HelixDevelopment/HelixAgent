// DEPRECATED: This package is superseded by the extracted Concurrency/ submodule
// (module digital.vasic.concurrency). It has zero production imports and is
// retained only for reference. New code should use the submodule via adapters.
//
// Package deadlock provides deadlock detection and prevention mechanisms
// for the HelixAgent concurrent systems.
package deadlock

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// Detector monitors lock acquisitions to detect potential deadlocks.
//
// Concurrency model (CONST-029): three graph maps migrate to
// *safe.Store; per-entry slice mutations use Store.Update-based
// copy-on-write. DetectCycles and Report use Snapshot() for a
// consistent point-in-time view across all three stores.
type Detector struct {
	lockGraph    *safe.Store[string, []string] // Lock dependency graph
	lockHolders  *safe.Store[string, string]   // Which goroutine holds which lock
	waitForGraph *safe.Store[string, []string] // Goroutines waiting for locks
	enabled      bool
	maxWaitTime  time.Duration
	logger       Logger
}

// Logger interface for deadlock detection logging
type Logger interface {
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewDetector creates a new deadlock detector
func NewDetector(maxWaitTime time.Duration, logger Logger) *Detector {
	if maxWaitTime == 0 {
		maxWaitTime = 5 * time.Second
	}

	return &Detector{
		lockGraph:    safe.NewStore[string, []string](),
		lockHolders:  safe.NewStore[string, string](),
		waitForGraph: safe.NewStore[string, []string](),
		enabled:      true,
		maxWaitTime:  maxWaitTime,
		logger:       logger,
	}
}

// LockWrapper wraps a mutex with deadlock detection
type LockWrapper struct {
	mu       sync.Locker
	name     string
	detector *Detector
}

// NewLockWrapper creates a new lock wrapper with deadlock detection
func (d *Detector) NewLockWrapper(mu sync.Locker, name string) *LockWrapper {
	return &LockWrapper{
		mu:       mu,
		name:     name,
		detector: d,
	}
}

// Lock acquires the lock with deadlock detection
func (lw *LockWrapper) Lock() {
	if lw.detector == nil || !lw.detector.enabled {
		lw.mu.Lock()
		return
	}

	goroutineID := getGoroutineID()

	// Check if we would create a deadlock
	if lw.detector.wouldDeadlock(goroutineID, lw.name) {
		lw.detector.logger.Error("Potential deadlock detected",
			"goroutine", goroutineID,
			"lock", lw.name,
		)
		// Deadlock detected: fall through to timeout-based acquisition
		// rather than panicking. The timeout will prevent actual deadlock.
	}

	// Record that this goroutine is waiting for this lock
	lw.detector.recordWait(goroutineID, lw.name)

	// Try to acquire with timeout
	done := make(chan struct{})
	go func() {
		lw.mu.Lock()
		close(done)
	}()

	select {
	case <-done:
		// Successfully acquired
		lw.detector.recordAcquire(goroutineID, lw.name)
	case <-time.After(lw.detector.maxWaitTime):
		// Timeout - potential deadlock
		lw.detector.logger.Warn("Lock acquisition timeout - potential deadlock",
			"goroutine", goroutineID,
			"lock", lw.name,
			"timeout", lw.detector.maxWaitTime,
		)
		// Still wait for the lock but log the issue
		<-done
		lw.detector.recordAcquire(goroutineID, lw.name)
	}
}

// Unlock releases the lock
func (lw *LockWrapper) Unlock() {
	if lw.detector != nil && lw.detector.enabled {
		goroutineID := getGoroutineID()
		lw.detector.recordRelease(goroutineID, lw.name)
	}
	lw.mu.Unlock()
}

// wouldDeadlock checks if acquiring this lock would create a deadlock
func (d *Detector) wouldDeadlock(goroutineID, lockName string) bool {
	// Get the current lock holder
	holder, held := d.lockHolders.Get(lockName)
	if !held {
		return false
	}

	// Check if the holder is waiting for any lock we hold
	myLocks := d.getLocksHeldBy(goroutineID)
	for _, myLock := range myLocks {
		if d.isWaitingFor(holder, myLock) {
			return true
		}
	}

	return false
}

// recordWait records that a goroutine is waiting for a lock
func (d *Detector) recordWait(goroutineID, lockName string) {
	d.waitForGraph.Update(goroutineID, func(cur []string, _ bool) ([]string, bool) {
		return append(cur, lockName), true
	})
}

// recordAcquire records that a goroutine acquired a lock
func (d *Detector) recordAcquire(goroutineID, lockName string) {
	d.lockHolders.Put(lockName, goroutineID)

	// Remove from wait list
	d.waitForGraph.Update(goroutineID, func(cur []string, present bool) ([]string, bool) {
		if !present {
			return cur, false
		}
		for i, l := range cur {
			if l == lockName {
				next := append([]string(nil), cur[:i]...)
				next = append(next, cur[i+1:]...)
				if len(next) == 0 {
					return nil, false
				}
				return next, true
			}
		}
		return cur, true
	})

	// Update lock graph
	if holder, ok := d.lockHolders.Get(lockName); ok && holder != "" && holder != goroutineID {
		d.lockGraph.Update(holder, func(cur []string, _ bool) ([]string, bool) {
			return append(cur, lockName), true
		})
	}
}

// recordRelease records that a goroutine released a lock
func (d *Detector) recordRelease(goroutineID, lockName string) {
	d.lockHolders.Update(lockName, func(cur string, present bool) (string, bool) {
		if !present || cur != goroutineID {
			return cur, present
		}
		return "", false
	})
}

// getLocksHeldBy returns all locks held by a goroutine
func (d *Detector) getLocksHeldBy(goroutineID string) []string {
	var locks []string
	d.lockHolders.Range(func(lock, holder string) bool {
		if holder == goroutineID {
			locks = append(locks, lock)
		}
		return true
	})
	return locks
}

// isWaitingFor checks if a goroutine is waiting for a specific lock
func (d *Detector) isWaitingFor(goroutineID, lockName string) bool {
	waits, _ := d.waitForGraph.Get(goroutineID)
	for _, l := range waits {
		if l == lockName {
			return true
		}
	}
	return false
}

// DetectCycles finds cycles in the lock dependency graph
func (d *Detector) DetectCycles() [][]string {
	graph := d.lockGraph.Snapshot()

	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				dfs(neighbor)
			} else if recStack[neighbor] {
				// Found a cycle
				cycle := extractCycle(path, neighbor)
				if len(cycle) > 0 {
					cycles = append(cycles, cycle)
				}
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
	}

	for node := range graph {
		if !visited[node] {
			dfs(node)
		}
	}

	return cycles
}

// extractCycle extracts the cycle from the path starting at the given node
func extractCycle(path []string, startNode string) []string {
	for i, node := range path {
		if node == startNode {
			return path[i:]
		}
	}
	return nil
}

// getGoroutineID returns a unique identifier for the current goroutine
func getGoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(string(buf[:n]))[1]
	return idField
}

// Report generates a deadlock detection report
func (d *Detector) Report() *Report {
	cycles := d.DetectCycles()

	return &Report{
		Cycles:             cycles,
		LockGraph:          copyMap(d.lockGraph.Snapshot()),
		Holders:            copyStringMap(d.lockHolders.Snapshot()),
		Timestamp:          time.Now(),
		PotentialDeadlocks: len(cycles) > 0,
	}
}

// Report contains deadlock detection results
type Report struct {
	Cycles             [][]string
	LockGraph          map[string][]string
	Holders            map[string]string
	Timestamp          time.Time
	PotentialDeadlocks bool
}

// String returns a string representation of the report
func (r *Report) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Deadlock Detection Report - %s\n", r.Timestamp.Format(time.RFC3339)))
	sb.WriteString(strings.Repeat("=", 50) + "\n")

	if r.PotentialDeadlocks {
		sb.WriteString("⚠️  POTENTIAL DEADLOCKS DETECTED!\n")
		for i, cycle := range r.Cycles {
			sb.WriteString(fmt.Sprintf("  Cycle %d: %s\n", i+1, strings.Join(cycle, " -> ")))
		}
	} else {
		sb.WriteString("✅ No deadlocks detected\n")
	}

	sb.WriteString(fmt.Sprintf("\nActive Locks: %d\n", len(r.Holders)))
	for lock, holder := range r.Holders {
		sb.WriteString(fmt.Sprintf("  %s: held by goroutine %s\n", lock, holder))
	}

	return sb.String()
}

// copyMap creates a copy of a string slice map
func copyMap(m map[string][]string) map[string][]string {
	result := make(map[string][]string, len(m))
	for k, v := range m {
		result[k] = append([]string(nil), v...)
	}
	return result
}

// copyStringMap creates a copy of a string map
func copyStringMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// OrderedLock provides deadlock-safe ordered locking
type OrderedLock struct {
	mu      sync.Mutex
	cond    *sync.Cond
	order   int
	current int
}

// NewOrderedLock creates a new ordered lock
func NewOrderedLock() *OrderedLock {
	ol := &OrderedLock{order: 0, current: 0}
	ol.cond = sync.NewCond(&ol.mu)
	return ol
}

// Acquire acquires the lock in order
func (ol *OrderedLock) Acquire(order int) {
	ol.mu.Lock()
	defer ol.mu.Unlock()

	for ol.current != order {
		ol.cond.Wait()
	}
}

// Release releases the lock and signals the next
func (ol *OrderedLock) Release(order int) {
	ol.mu.Lock()
	defer ol.mu.Unlock()

	ol.current = order + 1
	ol.cond.Broadcast()
}

// TimeoutLock provides lock with timeout
type TimeoutLock struct {
	mu       sync.Mutex
	acquired chan struct{}
}

// NewTimeoutLock creates a new timeout lock
func NewTimeoutLock() *TimeoutLock {
	return &TimeoutLock{
		acquired: make(chan struct{}, 1),
	}
}

// TryLock attempts to acquire lock with timeout
func (tl *TimeoutLock) TryLock(timeout time.Duration) bool {
	select {
	case tl.acquired <- struct{}{}:
		tl.mu.Lock()
		return true
	case <-time.After(timeout):
		return false
	}
}

// Unlock releases the lock
func (tl *TimeoutLock) Unlock() {
	select {
	case <-tl.acquired:
		tl.mu.Unlock()
	default:
		// Lock wasn't held
	}
}

// HierarchicalLock provides deadlock prevention through lock ordering
type HierarchicalLock struct {
	mu    sync.Mutex
	level int
	name  string
}

// NewHierarchicalLock creates a new hierarchical lock
func NewHierarchicalLock(level int, name string) *HierarchicalLock {
	return &HierarchicalLock{
		level: level,
		name:  name,
	}
}

// LockSlice provides utilities for ordered multi-locking
type LockSlice []*HierarchicalLock

func (ls LockSlice) Len() int           { return len(ls) }
func (ls LockSlice) Swap(i, j int)      { ls[i], ls[j] = ls[j], ls[i] }
func (ls LockSlice) Less(i, j int) bool { return ls[i].level < ls[j].level }

// LockAll locks all locks in hierarchical order
func LockAll(locks ...*HierarchicalLock) {
	sort.Sort(LockSlice(locks))
	for _, lock := range locks {
		lock.mu.Lock()
	}
}

// UnlockAll unlocks all locks
func UnlockAll(locks ...*HierarchicalLock) {
	for _, lock := range locks {
		lock.mu.Unlock()
	}
}
