package browser

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.MaxInstances)
	assert.True(t, config.Headless)
	assert.Equal(t, 30000, config.Timeout)
	assert.True(t, config.AllowScreenshots)
	assert.Nil(t, config.AllowedDomains)
	assert.Nil(t, config.BlockedDomains)
}

func TestNewManager(t *testing.T) {
	config := DefaultConfig()

	// This test may fail if Playwright is not installed
	// Skip if PLAYWRIGHT_SKIP_TESTS is set
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	require.NotNil(t, manager)
	assert.NotNil(t, manager.pool)
	assert.Equal(t, config, manager.config)

	// Cleanup
	if manager != nil {
		manager.Close()
	}
}

func TestNewManager_InvalidConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	// Test with very large pool size (should still work but may fail on resource constraints)
	config := Config{
		MaxInstances: 1000,
		Headless:     true,
	}

	// This may or may not fail depending on system resources
	manager, err := NewManager(config)
	if err != nil {
		// Expected if resources are constrained
		assert.Contains(t, err.Error(), "failed to start Playwright")
	} else {
		manager.Close()
	}
}

func TestManager_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	actions := []Action{
		&NavigationAction{
			URL:     "about:blank",
			Timeout: 10 * time.Second,
		},
	}

	result, err := manager.Execute(ctx, actions)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Success)
	assert.Equal(t, "about:blank", result.URL)
	assert.NotEmpty(t, result.Title)
}

func TestManager_Execute_WithScreenshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	actions := []Action{
		&NavigationAction{
			URL:     "about:blank",
			Timeout: 10 * time.Second,
		},
		&ScreenshotAction{
			FullPage: true,
		},
	}

	result, err := manager.Execute(ctx, actions)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Success)
	if result.Screenshot != nil {
		assert.NotNil(t, result.Screenshot.Data)
	}
}

func TestManager_Execute_WithExtract(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	actions := []Action{
		&NavigationAction{
			URL:     "data:text/html,<html><body><h1>Test</h1></body></html>",
			Timeout: 10 * time.Second,
		},
		&ExtractAction{
			Selector: "h1",
			Type:     "text",
		},
	}

	result, err := manager.Execute(ctx, actions)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Success)
	if result.Extracted != nil {
		assert.Equal(t, "h1", result.Extracted.Selector)
	}
}

func TestManager_Execute_ActionError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// This action should fail
	actions := []Action{
		&NavigationAction{
			URL:     "invalid://url",
			Timeout: 1 * time.Second,
		},
	}

	result, err := manager.Execute(ctx, actions)
	// Should not return error, but result should indicate failure
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Error)
}

func TestManager_Close(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}

	err = manager.Close()
	assert.NoError(t, err)
}

// Pool Tests

func TestNewPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(2, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	require.NotNil(t, pool)
	assert.Equal(t, 2, pool.maxSize)
	assert.True(t, pool.headless)
	assert.NotNil(t, pool.instances)
	assert.NotNil(t, pool.pw)

	pool.Close()
}

func TestPool_Acquire(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(2, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	instance, err := pool.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, instance)
	assert.NotNil(t, instance.Browser)
	assert.NotNil(t, instance.Page)
	assert.NotNil(t, instance.Context)

	// Release back to pool
	pool.Release(instance)
}

func TestPool_Acquire_ContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	// Create pool with small size
	pool, err := NewPool(1, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	// Fill the pool
	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Release the instance first to make pool available
	pool.Release(instance)

	// Test with timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer timeoutCancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	// This might create a new instance if pool is empty
	instance2, err := pool.Acquire(timeoutCtx)
	if err != nil {
		assert.Equal(t, context.DeadlineExceeded, err)
	} else {
		pool.Release(instance2)
	}
}

func TestPool_Release(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(2, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Acquire and release
	instance, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pool.Release(instance)

	// Should be able to acquire again
	instance2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, instance2)

	pool.Release(instance2)
}

func TestPool_Release_Full(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	// Create pool with size 1
	pool, err := NewPool(1, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Acquire first instance
	instance1, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Acquire second instance (creates new one)
	instance2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Release first (should go to pool)
	pool.Release(instance1)

	// Release second (pool is full, should close)
	pool.Release(instance2)
}

func TestPool_Close(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(2, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}

	err = pool.Close()
	assert.NoError(t, err)
}

// Instance Tests

func TestInstance_Close(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(1, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Close should not panic
	instance.Close()
}

// Config Tests

func TestConfig_Struct(t *testing.T) {
	config := Config{
		MaxInstances:     5,
		Headless:         false,
		Timeout:          60000,
		AllowedDomains:   []string{"example.com", "test.com"},
		BlockedDomains:   []string{"blocked.com"},
		AllowScreenshots: false,
	}

	assert.Equal(t, 5, config.MaxInstances)
	assert.False(t, config.Headless)
	assert.Equal(t, 60000, config.Timeout)
	assert.Len(t, config.AllowedDomains, 2)
	assert.Len(t, config.BlockedDomains, 1)
	assert.False(t, config.AllowScreenshots)
}

// Instance struct tests

func TestInstance_Struct(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(1, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	instance, err := pool.Acquire(ctx)
	require.NoError(t, err)

	assert.NotNil(t, instance.Browser)
	assert.NotNil(t, instance.Page)
	assert.NotNil(t, instance.Context)

	pool.Release(instance)
}

// Manager struct tests

func TestManager_Struct(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	config := DefaultConfig()
	manager, err := NewManager(config)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer manager.Close()

	assert.NotNil(t, manager.pool)
	assert.Equal(t, config.MaxInstances, manager.config.MaxInstances)
	assert.Equal(t, config.Headless, manager.config.Headless)
}

// Pool struct tests

func TestPool_Struct(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser tests in short mode")  // SKIP-OK: #short-mode
	}

	pool, err := NewPool(2, true)
	if err != nil {
		t.Skipf("Skipping test - Playwright not available: %v", err)
	}
	defer pool.Close()

	assert.Equal(t, 2, pool.maxSize)
	assert.True(t, pool.headless)
	assert.NotNil(t, pool.instances)
	assert.Equal(t, 2, cap(pool.instances))
}
