//go:build performance
// +build performance

// Package performance contains benchmark and load tests for critical components.
// This file validates lazy loading patterns used throughout HelixAgent:
//   - sync.Once usage count across the codebase (meta-test)
//   - LazyProvider deferred initialization behaviour
//   - LazyServiceRegistry deferred initialization behaviour
package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/router"
)

// =============================================================================
// 1. Meta-test: verify sync.Once usage count across the codebase
// =============================================================================

// TestLazyLoading_SyncOnceCount walks the internal/ directory and counts
// occurrences of "sync.Once" in Go source files. The project documents 30+
// instances; we assert at least 30 to catch accidental removal.
func TestLazyLoading_SyncOnceCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping filesystem meta-test in short mode")
	}

	// Locate project root relative to this test file's package path.
	// Tests run from the module root, so "internal/" is directly accessible.
	internalDir := filepath.Join("..", "..", "internal")
	info, err := os.Stat(internalDir)
	require.NoError(t, err, "internal/ directory must exist at %s", internalDir)
	require.True(t, info.IsDir(), "%s must be a directory", internalDir)

	count := 0
	fileCount := 0

	err = filepath.Walk(internalDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		n := strings.Count(string(data), "sync.Once")
		if n > 0 {
			fileCount++
			count += n
		}
		return nil
	})
	require.NoError(t, err, "walking internal/ must succeed")

	t.Logf("sync.Once occurrences: %d across %d files", count, fileCount)

	const minExpected = 30
	assert.GreaterOrEqual(t, count, minExpected,
		"expected at least %d sync.Once instances (found %d across %d files); "+
			"lazy loading patterns may have been removed", minExpected, count, fileCount)
}

// =============================================================================
// 2. LazyProvider: verify initialization is deferred
// =============================================================================

// TestLazyProvider_InitDeferred verifies that:
//  1. A newly created LazyProvider reports IsInitialized() == false.
//  2. After Get() is called, IsInitialized() == true.
//  3. The factory is called exactly once even under concurrent access.
func TestLazyProvider_InitDeferred(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	var factoryCalls int64

	cfg := &llm.LazyProviderConfig{
		InitTimeout:   5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    0,
	}

	lp := llm.NewLazyProvider("test-lazy", func() (llm.LLMProvider, error) {
		atomic.AddInt64(&factoryCalls, 1)
		return &benchMockProvider{
			response: &models.LLMResponse{Content: "deferred init"},
		}, nil
	}, cfg)

	// --- Pre-access state ---
	assert.False(t, lp.IsInitialized(),
		"provider must not be initialized before first Get()")
	assert.Equal(t, int64(0), atomic.LoadInt64(&factoryCalls),
		"factory must not be called before first Get()")

	// --- First access ---
	provider, err := lp.Get()
	require.NoError(t, err, "Get() must succeed")
	require.NotNil(t, provider, "Get() must return a non-nil provider")

	assert.True(t, lp.IsInitialized(),
		"provider must be initialized after first Get()")
	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must be called exactly once after first Get()")

	// --- Subsequent accesses do not re-invoke the factory ---
	for i := 0; i < 10; i++ {
		_, err2 := lp.Get()
		require.NoError(t, err2)
	}
	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must still be called exactly once after multiple Get() calls")
}

// TestLazyProvider_ConcurrentInit verifies that exactly one factory call
// occurs when many goroutines race to call Get() simultaneously.
func TestLazyProvider_ConcurrentInit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	var factoryCalls int64

	cfg := &llm.LazyProviderConfig{
		InitTimeout:   5 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    0,
	}

	lp := llm.NewLazyProvider("concurrent-test", func() (llm.LLMProvider, error) {
		atomic.AddInt64(&factoryCalls, 1)
		// Brief sleep to amplify the race window.
		time.Sleep(5 * time.Millisecond)
		return &benchMockProvider{response: nil}, nil
	}, cfg)

	const goroutines = 50
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := lp.Get()
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("unexpected error from concurrent Get(): %v", err)
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must be called exactly once regardless of concurrent callers")
	assert.True(t, lp.IsInitialized(),
		"provider must be initialized after concurrent access")
}

// TestLazyProvider_ErrorPropagation verifies that when the factory returns an
// error, IsInitialized() remains false and Error() is non-nil.
func TestLazyProvider_ErrorPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	cfg := &llm.LazyProviderConfig{
		InitTimeout:   2 * time.Second,
		RetryAttempts: 1, // single attempt to keep the test fast
		RetryDelay:    0,
	}

	lp := llm.NewLazyProvider("error-provider", func() (llm.LLMProvider, error) {
		return nil, fmt.Errorf("simulated init failure")
	}, cfg)

	assert.False(t, lp.IsInitialized(), "must not be initialized before Get()")

	_, err := lp.Get()
	assert.Error(t, err, "Get() must return an error when factory fails")
	assert.False(t, lp.IsInitialized(),
		"provider must not report initialized after a factory error")
	assert.Error(t, lp.Error(),
		"Error() must return the factory error")
}

// =============================================================================
// 3. LazyServiceRegistry: verify deferred initialization
// =============================================================================

// TestLazyServiceRegistry_DeferredInit verifies that:
//  1. Registering a service does not call the factory.
//  2. The factory is called on the first Get().
//  3. Subsequent Get() calls return the cached value without re-invoking
//     the factory.
func TestLazyServiceRegistry_DeferredInit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	var factoryCalls int64

	reg := router.NewLazyServiceRegistry()
	svc := router.NewLazyService(func() (interface{}, error) {
		atomic.AddInt64(&factoryCalls, 1)
		return "my-deferred-service", nil
	})

	// Registration must not trigger initialization.
	reg.Register("deferred-svc", svc)

	ls, ok := reg.GetLazy("deferred-svc")
	require.True(t, ok, "GetLazy must find the registered service")

	assert.False(t, ls.IsInitialized(),
		"service must not be initialized after Register()")
	assert.Equal(t, int64(0), atomic.LoadInt64(&factoryCalls),
		"factory must not be called after Register()")

	// First Get() must trigger initialization.
	val, ok2 := reg.Get("deferred-svc")
	require.True(t, ok2, "Get() must return true for a registered service")
	require.NotNil(t, val, "Get() must return a non-nil value")
	assert.Equal(t, "my-deferred-service", val)

	assert.True(t, ls.IsInitialized(),
		"service must be initialized after first Get()")
	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must be called exactly once after first Get()")

	// Repeated Get() calls must not re-invoke the factory.
	for i := 0; i < 10; i++ {
		v, _ := reg.Get("deferred-svc")
		assert.Equal(t, "my-deferred-service", v)
	}
	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must still be called exactly once after repeated Get() calls")
}

// TestLazyServiceRegistry_MultipleServices verifies isolation between
// registered services: initializing one must not initialize others.
func TestLazyServiceRegistry_MultipleServices(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	var calls [3]int64

	reg := router.NewLazyServiceRegistry()
	for i := 0; i < 3; i++ {
		idx := i // capture for closure
		reg.Register(fmt.Sprintf("svc-%d", idx), router.NewLazyService(func() (interface{}, error) {
			atomic.AddInt64(&calls[idx], 1)
			return fmt.Sprintf("value-%d", idx), nil
		}))
	}

	// Access only svc-1.
	_, _ = reg.Get("svc-1")

	assert.Equal(t, int64(0), atomic.LoadInt64(&calls[0]), "svc-0 must not be initialized")
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls[1]), "svc-1 must be initialized exactly once")
	assert.Equal(t, int64(0), atomic.LoadInt64(&calls[2]), "svc-2 must not be initialized")

	// Now access svc-2.
	_, _ = reg.Get("svc-2")

	assert.Equal(t, int64(0), atomic.LoadInt64(&calls[0]), "svc-0 must still not be initialized")
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls[1]), "svc-1 factory must still be called exactly once")
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls[2]), "svc-2 must now be initialized exactly once")
}

// TestLazyServiceRegistry_ConcurrentAccess verifies that concurrent Get()
// calls on the same service result in exactly one factory invocation.
func TestLazyServiceRegistry_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	var factoryCalls int64

	reg := router.NewLazyServiceRegistry()
	reg.Register("concurrent-svc", router.NewLazyService(func() (interface{}, error) {
		atomic.AddInt64(&factoryCalls, 1)
		time.Sleep(5 * time.Millisecond) // amplify race window
		return "concurrent-value", nil
	}))

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.Get("concurrent-svc")
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&factoryCalls),
		"factory must be called exactly once regardless of concurrent callers")
}

// TestLazyServiceRegistry_UnknownService verifies that Get() returns false
// for a name that was never registered.
func TestLazyServiceRegistry_UnknownService(t *testing.T) {
	reg := router.NewLazyServiceRegistry()
	val, ok := reg.Get("nonexistent")
	assert.False(t, ok, "Get() must return false for unknown service")
	assert.Nil(t, val, "Get() value must be nil for unknown service")
}

// =============================================================================
// 4. LazyService direct usage
// =============================================================================

// TestLazyService_IsInitializedBeforeGet confirms IsInitialized() returns
// false before the first Get() call.
func TestLazyService_IsInitializedBeforeGet(t *testing.T) {
	ls := router.NewLazyService(func() (interface{}, error) {
		return "hello", nil
	})

	assert.False(t, ls.IsInitialized(),
		"IsInitialized() must be false before first Get()")

	_, err := ls.Get()
	require.NoError(t, err)

	assert.True(t, ls.IsInitialized(),
		"IsInitialized() must be true after first Get()")
}

// TestLazyService_FactoryCalledOnce verifies factory idempotency across
// sequential Get() calls.
func TestLazyService_FactoryCalledOnce(t *testing.T) {
	var calls int64
	ls := router.NewLazyService(func() (interface{}, error) {
		atomic.AddInt64(&calls, 1)
		return "singleton", nil
	})

	for i := 0; i < 100; i++ {
		v, err := ls.Get()
		require.NoError(t, err)
		assert.Equal(t, "singleton", v)
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&calls),
		"factory must be called exactly once across 100 sequential Get() calls")
}
