//go:build stress
// +build stress

package stress

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/formatters"
)

// stressFormatter is a simple mock formatter for stress testing
type stressFormatter struct {
	*formatters.BaseFormatter
	delay time.Duration
}

func newStressFormatter(name string, languages []string, delay time.Duration) *stressFormatter {
	metadata := &formatters.FormatterMetadata{
		Name:      name,
		Type:      formatters.FormatterTypeNative,
		Version:   "1.0.0",
		Languages: languages,
	}
	return &stressFormatter{
		BaseFormatter: formatters.NewBaseFormatter(metadata),
		delay:         delay,
	}
}

func (f *stressFormatter) Format(
	ctx context.Context, req *formatters.FormatRequest,
) (*formatters.FormatResult, error) {
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	return &formatters.FormatResult{
		Content:          req.Content,
		Changed:          false,
		FormatterName:    f.Name(),
		FormatterVersion: f.Version(),
		Duration:         f.delay,
		Success:          true,
	}, nil
}

func (f *stressFormatter) FormatBatch(
	ctx context.Context, reqs []*formatters.FormatRequest,
) ([]*formatters.FormatResult, error) {
	results := make([]*formatters.FormatResult, len(reqs))
	for i, req := range reqs {
		result, err := f.Format(ctx, req)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

func (f *stressFormatter) HealthCheck(ctx context.Context) error {
	return nil
}

func newStressLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return logger
}

// TestFormatters_ConcurrentCacheAccess tests 100 goroutines doing
// Get/Set on FormatterCache concurrently
func TestFormatters_ConcurrentCacheAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	logger := newStressLogger()
	cache := formatters.NewFormatterCache(&formatters.CacheConfig{
		TTL:         time.Minute,
		MaxSize:     1000,
		CleanupFreq: 10 * time.Second,
	}, logger)
	defer cache.Stop()

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					req := &formatters.FormatRequest{
						Content:  fmt.Sprintf("content-%d-%d", idx, j%10),
						Language: "go",
						FilePath: fmt.Sprintf("file_%d.go", j%10),
					}
					result := &formatters.FormatResult{
						Content:       req.Content,
						FormatterName: "stress-fmt",
						Success:       true,
						Duration:      time.Millisecond,
					}
					cache.Set(req, result)
					cache.Get(req)
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrent cache access timed out")
	}

	assert.True(t, cache.Size() > 0, "Cache should have entries")
}

// TestFormatters_ConcurrentHealthChecks tests 50 goroutines running
// HealthCheckAll on a registry with mock formatters
func TestFormatters_ConcurrentHealthChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	logger := newStressLogger()
	registry := formatters.NewFormatterRegistry(&formatters.RegistryConfig{
		DefaultTimeout: 5 * time.Second,
		MaxConcurrent:  10,
	}, logger)

	// Register several formatters
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("stress-fmt-%d", i)
		f := newStressFormatter(name, []string{"go", "python"}, time.Millisecond)
		meta := &formatters.FormatterMetadata{
			Name:      name,
			Type:      formatters.FormatterTypeNative,
			Version:   "1.0.0",
			Languages: []string{"go", "python"},
		}
		err := registry.Register(f, meta)
		assert.NoError(t, err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		ctx := context.Background()

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results := registry.HealthCheckAll(ctx)
				assert.Equal(t, 20, len(results))
				for _, err := range results {
					assert.NoError(t, err)
				}
			}()
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrent health checks timed out")
	}
}

// TestFormatters_ConcurrentExecution tests 50 goroutines executing
// format requests simultaneously
func TestFormatters_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	logger := newStressLogger()
	registry := formatters.NewFormatterRegistry(&formatters.RegistryConfig{
		DefaultTimeout: 5 * time.Second,
		MaxConcurrent:  50,
	}, logger)

	f := newStressFormatter("go-fmt", []string{"go"}, time.Millisecond)
	meta := &formatters.FormatterMetadata{
		Name:      "go-fmt",
		Type:      formatters.FormatterTypeNative,
		Version:   "1.0.0",
		Languages: []string{"go"},
	}
	err := registry.Register(f, meta)
	assert.NoError(t, err)

	done := make(chan struct{})
	var successCount int64

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		ctx := context.Background()

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					formatter, fErr := registry.Get("go-fmt")
					if fErr != nil {
						t.Errorf("Failed to get formatter: %v", fErr)
						return
					}
					req := &formatters.FormatRequest{
						Content:  fmt.Sprintf("package main\n// %d-%d\n", idx, j),
						Language: "go",
						FilePath: "main.go",
					}
					result, fErr := formatter.Format(ctx, req)
					if fErr != nil {
						t.Errorf("Format failed: %v", fErr)
						return
					}
					if result.Success {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrent execution timed out")
	}

	total := atomic.LoadInt64(&successCount)
	assert.Equal(t, int64(2500), total,
		"Expected 2500 successful formats, got %d", total)
}

// TestFormatters_CachePressure fills cache to capacity and then keeps
// setting/getting under pressure from 100 goroutines
func TestFormatters_CachePressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	logger := newStressLogger()
	maxSize := 500
	cache := formatters.NewFormatterCache(&formatters.CacheConfig{
		TTL:         time.Minute,
		MaxSize:     maxSize,
		CleanupFreq: 10 * time.Second,
	}, logger)
	defer cache.Stop()

	// Pre-fill cache to capacity
	for i := 0; i < maxSize; i++ {
		req := &formatters.FormatRequest{
			Content:  fmt.Sprintf("prefill-%d", i),
			Language: "go",
			FilePath: fmt.Sprintf("prefill_%d.go", i),
		}
		result := &formatters.FormatResult{
			Content:       req.Content,
			FormatterName: "stress-fmt",
			Success:       true,
		}
		cache.Set(req, result)
	}
	assert.Equal(t, maxSize, cache.Size(), "Cache should be full after prefill")

	done := make(chan struct{})
	var ops int64

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 200; j++ {
					req := &formatters.FormatRequest{
						Content:  fmt.Sprintf("pressure-%d-%d", idx, j),
						Language: "python",
						FilePath: fmt.Sprintf("pressure_%d_%d.py", idx, j%20),
					}
					result := &formatters.FormatResult{
						Content:       req.Content,
						FormatterName: "stress-fmt",
						Success:       true,
					}
					cache.Set(req, result)
					cache.Get(req)
					atomic.AddInt64(&ops, 2)
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Cache pressure test timed out")
	}

	totalOps := atomic.LoadInt64(&ops)
	t.Logf("Cache pressure test completed: %d operations, final size: %d",
		totalOps, cache.Size())
	assert.True(t, cache.Size() <= maxSize,
		"Cache size %d should not exceed max %d", cache.Size(), maxSize)
	assert.Equal(t, int64(40000), totalOps,
		"Expected 40000 operations, got %d", totalOps)
}

// TestFormatters_RegistryStress tests 50 goroutines registering/getting
// formatters simultaneously
func TestFormatters_RegistryStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	logger := newStressLogger()
	registry := formatters.NewFormatterRegistry(&formatters.RegistryConfig{
		DefaultTimeout: 5 * time.Second,
		MaxConcurrent:  50,
	}, logger)

	done := make(chan struct{})
	var registered int64

	go func() {
		defer close(done)

		var wg sync.WaitGroup

		// 25 goroutines registering unique formatters
		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					name := fmt.Sprintf("reg-fmt-%d-%d", idx, j)
					f := newStressFormatter(
						name,
						[]string{"go", "python", "rust"},
						0,
					)
					meta := &formatters.FormatterMetadata{
						Name:      name,
						Type:      formatters.FormatterTypeNative,
						Version:   "1.0.0",
						Languages: []string{"go", "python", "rust"},
					}
					if err := registry.Register(f, meta); err == nil {
						atomic.AddInt64(&registered, 1)
					}
				}
			}(i)
		}

		// 25 goroutines reading from the registry concurrently
		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					// Try getting formatters that may or may not exist yet
					name := fmt.Sprintf("reg-fmt-%d-%d", idx%25, j%10)
					registry.Get(name)
					registry.List()
					registry.GetByLanguage("go")
					registry.Count()
				}
			}(i)
		}

		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Registry stress test timed out")
	}

	totalRegistered := atomic.LoadInt64(&registered)
	t.Logf("Registry stress test completed: %d formatters registered, "+
		"registry count: %d", totalRegistered, registry.Count())
	assert.True(t, totalRegistered > 0,
		"Should have registered at least some formatters")
	assert.Equal(t, int(totalRegistered), registry.Count(),
		"Registry count should match registered count")
}
