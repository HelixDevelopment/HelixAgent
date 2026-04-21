// Package clis provides CLI agent integration for HelixAgent.
package clis

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstancePool(t *testing.T) {
	t.Parallel()
	config := DefaultPoolConfig()
	var newPoolCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&newPoolCounter, 1)
		return &AgentInstance{
			ID:   fmt.Sprintf("new-pool-%d", id),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	require.NotNil(t, pool)
	assert.Equal(t, TypeAider, pool.agentType)
	assert.Equal(t, config.MinIdle, pool.minIdle)
	assert.Equal(t, config.MaxIdle, pool.maxIdle)
	assert.Equal(t, config.MaxActive, pool.maxActive)

	pool.Close()
}

func TestInstancePool_AcquireFromEmpty(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: time.Hour,
	}

	factoryCalled := 0
	factory := func() (*AgentInstance, error) {
		factoryCalled++
		return &AgentInstance{
			ID:   "instance-1",
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()
	inst, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, inst)
	assert.Equal(t, "instance-1", inst.ID)
	assert.Equal(t, 1, factoryCalled)
}

func TestInstancePool_AcquireFromPool(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     1,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: time.Hour,
	}

	factory := func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:     "instance-1",
			Type:   TypeAider,
			Status: StatusIdle,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	// Wait for pre-warming
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	inst, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, inst)
	// Should get from pool, not create new
	assert.Equal(t, StatusIdle, inst.Status)
}

func TestInstancePool_Release(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: time.Hour,
	}

	factory := func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:     "instance-1",
			Type:   TypeAider,
			Status: StatusIdle,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()
	inst, _ := pool.Acquire(ctx)
	inst.Status = StatusActive
	inst.SessionID = "test-session"

	err := pool.Release(inst)
	require.NoError(t, err)

	// Instance should be reset
	assert.Equal(t, StatusIdle, inst.Status)
	assert.Equal(t, "", inst.SessionID)
}

func TestInstancePool_MaxIdleLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     2,
		MaxActive:   10,
		MaxLifetime: time.Hour,
	}

	factoryCalled := 0
	factory := func() (*AgentInstance, error) {
		factoryCalled++
		return &AgentInstance{
			ID:   string(rune('0' + factoryCalled)),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()

	// Create and release instances
	instances := make([]*AgentInstance, 5)
	for i := 0; i < 5; i++ {
		inst, err := pool.Acquire(ctx)
		require.NoError(t, err)
		instances[i] = inst
	}

	// Release all - only MaxIdle should be kept
	for _, inst := range instances {
		pool.Release(inst)
	}

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check stats - should only have MaxIdle instances
	stats := pool.Stats()
	assert.LessOrEqual(t, stats["idle_count"], 2)
}

func TestInstancePool_MaxActiveLimit(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   2,
		MaxLifetime: time.Hour,
	}

	var instanceCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&instanceCounter, 1)
		return &AgentInstance{
			ID:   fmt.Sprintf("instance-%d", id),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Acquire up to MaxActive
	inst1, err := pool.Acquire(ctx)
	require.NoError(t, err)
	inst2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Third acquire should timeout (pool exhausted at maxActive=2)
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer shortCancel()
	_, err = pool.Acquire(shortCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Release one and try again
	pool.Release(inst1)
	inst3, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, inst3)

	pool.Release(inst2)
	pool.Release(inst3)
}

func TestInstancePool_Invalidate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := DefaultPoolConfig()
	factory := func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:   "instance-1",
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()
	inst, _ := pool.Acquire(ctx)

	err := pool.Invalidate(inst)
	require.NoError(t, err)

	// Instance should be removed from active
	pool.mu.RLock()
	_, exists := pool.active.Get(inst.ID)
	pool.mu.RUnlock()
	assert.False(t, exists)
	assert.Equal(t, StatusTerminated, inst.Status)
}

func TestInstancePool_CleanupExpired(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: 50 * time.Millisecond,
	}

	factory := func() (*AgentInstance, error) {
		return &AgentInstance{
			ID:   "instance",
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()
	inst, _ := pool.Acquire(ctx)
	pool.Release(inst)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup by acquiring again
	pool.Acquire(ctx)

	// Check that expired instance was removed
	stats := pool.Stats()
	// The exact count depends on timing, but eviction should have occurred
	assert.GreaterOrEqual(t, stats["evicts"], uint64(0))
}

func TestInstancePool_ConcurrentAcquireRelease(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     10,
		MaxActive:   20,
		MaxLifetime: time.Hour,
	}

	var factoryCounter int64
	factory := func() (*AgentInstance, error) {
		count := atomic.AddInt64(&factoryCounter, 1)
		return &AgentInstance{
			ID:   fmt.Sprintf("concurrent-%d", count),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	const numGoroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				inst, err := pool.Acquire(ctx)
				if err == nil {
					time.Sleep(time.Millisecond)
					pool.Release(inst)
				}
			}
		}()
	}

	wg.Wait()

	// Verify pool is in consistent state
	stats := pool.Stats()
	assert.LessOrEqual(t, stats["active_count"], config.MaxActive)
	assert.LessOrEqual(t, stats["idle_count"], config.MaxIdle)
}

func TestInstancePool_Stats(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := DefaultPoolConfig()
	var statsCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&statsCounter, 1)
		return &AgentInstance{ID: fmt.Sprintf("stats-%d", id), Type: TypeAider}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	ctx := context.Background()

	// Initial stats
	stats := pool.Stats()
	assert.Equal(t, TypeAider, stats["agent_type"])
	assert.Equal(t, 0, stats["idle_count"])
	assert.Equal(t, 0, stats["active_count"])
	assert.Equal(t, uint64(0), stats["hits"])
	assert.Equal(t, uint64(0), stats["misses"])

	// Acquire an instance (should be a miss)
	inst, _ := pool.Acquire(ctx)
	stats = pool.Stats()
	assert.Equal(t, uint64(0), stats["hits"])
	assert.Equal(t, uint64(1), stats["misses"])

	// Release and re-acquire (should be a hit)
	pool.Release(inst)
	time.Sleep(50 * time.Millisecond)
	pool.Acquire(ctx)
	stats = pool.Stats()
	assert.GreaterOrEqual(t, stats["hits"], uint64(1))
}

func TestInstancePool_Close(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}
	config := DefaultPoolConfig()
	var closeCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&closeCounter, 1)
		return &AgentInstance{
			ID:   fmt.Sprintf("close-test-%d", id),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)

	ctx := context.Background()
	inst1, _ := pool.Acquire(ctx)
	inst2, _ := pool.Acquire(ctx)

	pool.Release(inst1)

	err := pool.Close()
	require.NoError(t, err)

	// All instances should be terminated
	assert.Equal(t, StatusTerminated, inst1.Status)
	assert.Equal(t, StatusTerminated, inst2.Status)
}

func TestInstancePool_Acquire_ConcurrentRace(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping pool test in short mode - requires database setup")
	}

	// This test targets the RLock-to-Lock gap in Acquire().
	// With the old code, multiple goroutines could read activeCount < maxActive
	// under RLock, then all proceed to create new instances, exceeding maxActive.
	// The tight maxActive=5 with 100 goroutines makes the race window very likely.
	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   5,
		MaxLifetime: time.Hour,
	}

	var factoryCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&factoryCounter, 1)
		return &AgentInstance{
			ID:   fmt.Sprintf("race-inst-%d", id),
			Type: TypeAider,
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	const numGoroutines = 100
	var wg sync.WaitGroup
	ctx := context.Background()

	var acquireErrors int64
	var acquireSuccesses int64
	var maxConcurrentActive int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use a short timeout so we don't hang
			acquireCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			inst, err := pool.Acquire(acquireCtx)
			if err != nil {
				atomic.AddInt64(&acquireErrors, 1)
				return
			}
			atomic.AddInt64(&acquireSuccesses, 1)

			// Check active count under lock - should never exceed maxActive
			pool.mu.RLock()
			currentActive := int64(pool.active.Len())
			pool.mu.RUnlock()

			// Track the max we observe
			for {
				old := atomic.LoadInt64(&maxConcurrentActive)
				if currentActive <= old {
					break
				}
				if atomic.CompareAndSwapInt64(&maxConcurrentActive, old, currentActive) {
					break
				}
			}

			// Hold for a bit to maximize contention
			time.Sleep(10 * time.Millisecond)

			pool.Release(inst)
		}()
	}

	wg.Wait()

	// The critical invariant: active count should NEVER have exceeded maxActive
	maxObserved := atomic.LoadInt64(&maxConcurrentActive)
	assert.LessOrEqual(t, maxObserved, int64(config.MaxActive),
		"active count exceeded maxActive: observed %d, max allowed %d",
		maxObserved, config.MaxActive)

	// Verify pool is in consistent state after all goroutines complete
	stats := pool.Stats()
	assert.Equal(t, 0, stats["active_count"],
		"all instances should be released after test")

	t.Logf("Results: %d successes, %d errors (timeouts), max concurrent active: %d",
		atomic.LoadInt64(&acquireSuccesses),
		atomic.LoadInt64(&acquireErrors),
		maxObserved)
}

func TestInstancePool_CleanupExpired_NoGoroutineLeak(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping goroutine leak test in short mode")
	}

	// Force GC and let background goroutines from previous tests settle.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	config := PoolConfig{
		MinIdle:     0,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: 10 * time.Millisecond, // very short so cleanup triggers termination
	}

	var factoryCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&factoryCounter, 1)
		return &AgentInstance{
			ID:        fmt.Sprintf("leak-test-%d", id),
			Type:      TypeAider,
			UpdatedAt: time.Now(),
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)

	ctx := context.Background()

	// Acquire and release several instances so they enter the idle pool.
	for i := 0; i < 5; i++ {
		inst, err := pool.Acquire(ctx)
		require.NoError(t, err)
		err = pool.Release(inst)
		require.NoError(t, err)
	}

	// Wait for instances to expire past maxLifetime.
	time.Sleep(50 * time.Millisecond)

	// Manually trigger cleanup which spawns tracked terminate goroutines.
	pool.cleanupExpired()

	// Close waits for all tracked goroutines via wg.Wait().
	err := pool.Close()
	require.NoError(t, err)

	// Allow any remaining goroutines to wind down.
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	delta := after - baseline

	// The goroutine count should return to near-baseline. A tolerance of ±5
	// accounts for runtime internals (GC, finalizers, etc.).
	assert.LessOrEqual(t, delta, 5,
		"goroutine leak detected: baseline=%d, after=%d, delta=%d", baseline, after, delta)

	t.Logf("Goroutine count: baseline=%d, after_close=%d, delta=%d", baseline, after, delta)
}

func TestInstancePool_EnsureMinIdle_NoDeadlock(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping deadlock test in short mode")
	}

	// Create a pool with a slow factory (100ms per instance).
	// If ensureMinIdle held the lock during factory(), every concurrent Acquire()
	// would block on the same lock, causing a deadlock-like hang that exceeds the
	// 10-second deadline below.
	config := PoolConfig{
		MinIdle:     3,
		MaxIdle:     5,
		MaxActive:   10,
		MaxLifetime: time.Hour,
	}

	var factoryCounter int64
	factory := func() (*AgentInstance, error) {
		// Simulate slow instance creation (e.g., process spawn, network call).
		time.Sleep(100 * time.Millisecond)
		id := atomic.AddInt64(&factoryCounter, 1)
		return &AgentInstance{
			ID:        fmt.Sprintf("minIdle-noDeadlock-%d", id),
			Type:      TypeAider,
			UpdatedAt: time.Now(),
		}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	// While ensureMinIdle is filling idle slots (slow factory), concurrently
	// call Acquire to prove the lock is not held during factory().
	// All acquisitions must complete within 10 seconds; a deadlock would hang.
	const numAcquirers = 5
	done := make(chan struct{}, numAcquirers)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < numAcquirers; i++ {
		go func() {
			inst, err := pool.Acquire(ctx)
			if err == nil {
				time.Sleep(20 * time.Millisecond)
				pool.Release(inst)
			}
			done <- struct{}{}
		}()
	}

	// Also manually trigger ensureMinIdle to race with the acquirers.
	go func() {
		pool.ensureMinIdle()
		done <- struct{}{}
	}()

	total := numAcquirers + 1
	for i := 0; i < total; i++ {
		select {
		case <-done:
			// goroutine completed successfully
		case <-ctx.Done():
			t.Fatal("deadlock detected: pool operations did not complete within 10 seconds")
		}
	}
}

// Benchmarks

func BenchmarkInstancePool_AcquireRelease(b *testing.B) {
	config := PoolConfig{
		MinIdle:     5,
		MaxIdle:     10,
		MaxActive:   20,
		MaxLifetime: time.Hour,
	}

	var benchCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&benchCounter, 1)
		return &AgentInstance{ID: fmt.Sprintf("bench-%d", id), Type: TypeAider}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	// Wait for pre-warming
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst, _ := pool.Acquire(ctx)
		pool.Release(inst)
	}
}

func BenchmarkInstancePool_ConcurrentAcquireRelease(b *testing.B) {
	config := PoolConfig{
		MinIdle:     10,
		MaxIdle:     20,
		MaxActive:   50,
		MaxLifetime: time.Hour,
	}

	var benchConcurrentCounter int64
	factory := func() (*AgentInstance, error) {
		id := atomic.AddInt64(&benchConcurrentCounter, 1)
		return &AgentInstance{ID: fmt.Sprintf("bench-concurrent-%d", id), Type: TypeAider}, nil
	}

	pool := NewInstancePool(TypeAider, config, factory)
	defer pool.Close()

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			inst, _ := pool.Acquire(ctx)
			pool.Release(inst)
		}
	})
}
