//go:build !integration

package formatters

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry(t *testing.T) *FormatterRegistry {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return NewFormatterRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}, logger)
}

func newTestExecutor(t *testing.T, registry *FormatterRegistry, enableCache bool) *FormatterExecutor {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return NewFormatterExecutor(registry, &ExecutorConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		EnableCache:    enableCache,
		EnableMetrics:  false,
		EnableTracing:  false,
	}, logger)
}

func registerMock(t *testing.T, registry *FormatterRegistry, name string, langs []string) *mockFormatter {
	t.Helper()
	mock := newMockFormatter(name, "1.0.0", langs)
	metadata := &FormatterMetadata{
		Name:      name,
		Version:   "1.0.0",
		Languages: langs,
		Type:      FormatterTypeNative,
	}
	err := registry.Register(mock, metadata)
	require.NoError(t, err)
	return mock
}

func TestNewFormatterExecutor(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	assert.NotNil(t, executor)
}

func TestNewFormatterExecutor_WithCache(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, true)
	assert.NotNil(t, executor)
	assert.NotNil(t, executor.cache)
}

func TestFormatterExecutor_Execute_ByLanguage(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	registerMock(t, registry, "black", []string{"python"})

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	result, err := executor.Execute(ctx, &FormatRequest{
		Content:  "x=1",
		Language: "python",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "x=1 formatted", result.Content)
	assert.True(t, result.Changed)
}

func TestFormatterExecutor_Execute_ByFilePath(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	registerMock(t, registry, "gofmt", []string{"go"})

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	result, err := executor.Execute(ctx, &FormatRequest{
		Content:  "package main",
		FilePath: "main.go",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "formatted")
}

func TestFormatterExecutor_Execute_NoLanguageOrPath(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	_, err := executor.Execute(ctx, &FormatRequest{
		Content: "some code",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either language or file_path must be specified")
}

func TestFormatterExecutor_Execute_NoFormatterForLanguage(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	_, err := executor.Execute(ctx, &FormatRequest{
		Content:  "some code",
		Language: "cobol",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no formatters available for language")
}

func TestFormatterExecutor_Execute_FormatterError(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	mock := registerMock(t, registry, "errfmt", []string{"python"})
	mock.formatFunc = func(ctx context.Context, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("format failed")
	}

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	_, err := executor.Execute(ctx, &FormatRequest{
		Content:  "x=1",
		Language: "python",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "format failed")
}

func TestFormatterExecutor_Execute_DetectFromUnknownPath(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	_, err := executor.Execute(ctx, &FormatRequest{
		Content:  "data",
		FilePath: "noextension",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to detect formatter")
}

func TestFormatterExecutor_ExecuteBatch(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	registerMock(t, registry, "black", []string{"python"})
	registerMock(t, registry, "gofmt", []string{"go"})

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	reqs := []*FormatRequest{
		{Content: "x=1", Language: "python"},
		{Content: "package main", Language: "go"},
	}

	results, err := executor.ExecuteBatch(ctx, reqs)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	for _, r := range results {
		assert.NotNil(t, r)
		assert.Contains(t, r.Content, "formatted")
	}
}

func TestFormatterExecutor_ExecuteBatch_PartialFailure(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	mock := registerMock(t, registry, "black", []string{"python"})
	mock.formatFunc = func(ctx context.Context, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("fail")
	}
	registerMock(t, registry, "gofmt", []string{"go"})

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	reqs := []*FormatRequest{
		{Content: "x=1", Language: "python"},
		{Content: "package main", Language: "go"},
	}

	results, err := executor.ExecuteBatch(ctx, reqs)
	// Should return first error
	assert.Error(t, err)
	// Results still has all entries (some may be nil)
	assert.Len(t, results, 2)
}

func TestFormatterExecutor_ExecuteBatch_Empty(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	results, err := executor.ExecuteBatch(ctx, []*FormatRequest{})
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestFormatterExecutor_Use(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)

	called := false
	mw := func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
			called = true
			return next(ctx, f, req)
		}
	}

	executor.Use(mw)
	registerMock(t, registry, "black", []string{"python"})

	ctx := context.Background()
	_, err := executor.Execute(ctx, &FormatRequest{Content: "x=1", Language: "python"})
	assert.NoError(t, err)
	assert.True(t, called)
}

// --- Middleware Tests ---

func TestTimeoutMiddleware_Success(t *testing.T) {
	t.Parallel()
	mw := TimeoutMiddleware(5 * time.Second)

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "done", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.Equal(t, "done", result.Content)
}

func TestTimeoutMiddleware_Timeout(t *testing.T) {
	t.Parallel()
	mw := TimeoutMiddleware(10 * time.Millisecond)

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		time.Sleep(100 * time.Millisecond)
		return &FormatResult{Content: "done"}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestTimeoutMiddleware_UsesRequestTimeout(t *testing.T) {
	t.Parallel()
	mw := TimeoutMiddleware(5 * time.Second) // default is long

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		time.Sleep(100 * time.Millisecond)
		return &FormatResult{Content: "done"}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{
		Content: "code",
		Timeout: 10 * time.Millisecond, // short request timeout
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestRetryMiddleware_Success(t *testing.T) {
	t.Parallel()
	mw := RetryMiddleware(3)

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "ok", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.Equal(t, "ok", result.Content)
}

func TestRetryMiddleware_EventualSuccess(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping retry test with sleep in short mode") // SKIP-OK: #short-mode
	}
	attempts := 0
	mw := RetryMiddleware(3)

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		attempts++
		if attempts < 2 {
			return nil, fmt.Errorf("transient error")
		}
		return &FormatResult{Content: "ok", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.Equal(t, "ok", result.Content)
	assert.Equal(t, 2, attempts)
}

func TestRetryMiddleware_AllFail(t *testing.T) {
	t.Parallel()
	attempts := 0
	mw := RetryMiddleware(0) // no retries

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		attempts++
		return nil, fmt.Errorf("permanent error")
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permanent error")
	assert.Equal(t, 1, attempts) // 0 retries = 1 attempt
}

func TestRetryMiddleware_ContextCancellation(t *testing.T) {
	t.Parallel()
	mw := RetryMiddleware(3)
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		attempts++
		cancel() // cancel after first attempt
		return nil, fmt.Errorf("error")
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(ctx, mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetryMiddleware_MaxRetriesCapped(t *testing.T) {
	t.Parallel()
	// Verify that maxRetries > 30 is capped to 30.
	// We test this indirectly: use RetryMiddleware(100) and cancel immediately
	// so only 1 attempt executes, then verify the error message shows 30 (the cap).
	mw := RetryMiddleware(100)

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("fail")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so it breaks out of retry loop

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(ctx, mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	// The error message should say "30 retries" (capped from 100)
	assert.Contains(t, err.Error(), "30 retries")
}

func TestRetryMiddleware_NegativeRetries(t *testing.T) {
	t.Parallel()
	mw := RetryMiddleware(-5) // negative capped to 0
	attempts := 0

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		attempts++
		return nil, fmt.Errorf("fail")
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // 0 retries = 1 attempt
}

func TestValidationMiddleware_EmptyContent(t *testing.T) {
	t.Parallel()
	mw := ValidationMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "ok"}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: ""})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty content provided")
}

func TestValidationMiddleware_EmptyResult(t *testing.T) {
	t.Parallel()
	mw := ValidationMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formatter returned empty content")
}

func TestValidationMiddleware_Success(t *testing.T) {
	t.Parallel()
	mw := ValidationMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "formatted", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.Equal(t, "formatted", result.Content)
}

func TestValidationMiddleware_FormatterError(t *testing.T) {
	t.Parallel()
	mw := ValidationMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("inner error")
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inner error")
}

func TestValidationMiddleware_FailedResultWithEmptyContent(t *testing.T) {
	t.Parallel()
	mw := ValidationMiddleware()

	// When Success is false, empty content is OK
	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "", Success: false}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.False(t, result.Success)
}

func TestCacheMiddleware_CacheHit(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	cache := NewFormatterCache(&CacheConfig{
		TTL:         1 * time.Hour,
		MaxSize:     100,
		CleanupFreq: 1 * time.Hour,
	}, logger)
	defer cache.Stop()

	req := &FormatRequest{Content: "x=1", Language: "python"}
	cachedResult := &FormatResult{Content: "cached", Success: true}
	cache.Set(req, cachedResult)

	mw := CacheMiddleware(cache)

	callCount := 0
	base := func(ctx context.Context, f Formatter, r *FormatRequest) (*FormatResult, error) {
		callCount++
		return &FormatResult{Content: "fresh"}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("black", "1.0", []string{"python"})
	result, err := wrapped(context.Background(), mock, req)

	assert.NoError(t, err)
	assert.Equal(t, "cached", result.Content)
	assert.Equal(t, 0, callCount) // base should not be called
}

func TestCacheMiddleware_CacheMiss(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	cache := NewFormatterCache(&CacheConfig{
		TTL:         1 * time.Hour,
		MaxSize:     100,
		CleanupFreq: 1 * time.Hour,
	}, logger)
	defer cache.Stop()

	mw := CacheMiddleware(cache)

	base := func(ctx context.Context, f Formatter, r *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "fresh", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("black", "1.0", []string{"python"})
	req := &FormatRequest{Content: "x=1", Language: "python"}

	result, err := wrapped(context.Background(), mock, req)
	assert.NoError(t, err)
	assert.Equal(t, "fresh", result.Content)

	// Should now be in cache
	cached, found := cache.Get(req)
	assert.True(t, found)
	assert.Equal(t, "fresh", cached.Content)
}

func TestCacheMiddleware_SkipsCheckOnly(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	cache := NewFormatterCache(&CacheConfig{
		TTL:         1 * time.Hour,
		MaxSize:     100,
		CleanupFreq: 1 * time.Hour,
	}, logger)
	defer cache.Stop()

	mw := CacheMiddleware(cache)

	callCount := 0
	base := func(ctx context.Context, f Formatter, r *FormatRequest) (*FormatResult, error) {
		callCount++
		return &FormatResult{Content: "checked", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("black", "1.0", []string{"python"})
	req := &FormatRequest{Content: "x=1", Language: "python", CheckOnly: true}

	// First call
	_, err := wrapped(context.Background(), mock, req)
	assert.NoError(t, err)
	// Second call should also go through (not cached)
	_, err = wrapped(context.Background(), mock, req)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestCacheMiddleware_UnknownLanguage(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	cache := NewFormatterCache(&CacheConfig{
		TTL:         1 * time.Hour,
		MaxSize:     100,
		CleanupFreq: 1 * time.Hour,
	}, logger)
	defer cache.Stop()

	mw := CacheMiddleware(cache)

	base := func(ctx context.Context, f Formatter, r *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "done", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("fmt", "1.0", []string{"go"})
	req := &FormatRequest{Content: "code", Language: ""}

	result, err := wrapped(context.Background(), mock, req)
	assert.NoError(t, err)
	assert.Equal(t, "done", result.Content)
}

func TestMetricsMiddleware_Success(t *testing.T) {
	t.Parallel()
	mw := MetricsMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "ok", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("black", "1.0", []string{"python"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{
		Content:  "x=1",
		Language: "python",
	})

	assert.NoError(t, err)
	assert.Equal(t, "ok", result.Content)
}

func TestMetricsMiddleware_Failure(t *testing.T) {
	t.Parallel()
	mw := MetricsMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("error")
	}

	wrapped := mw(base)
	mock := newMockFormatter("black", "1.0", []string{"python"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{
		Content:  "x=1",
		Language: "python",
	})

	assert.Error(t, err)
}

func TestMetricsMiddleware_UnknownLanguage(t *testing.T) {
	t.Parallel()
	mw := MetricsMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "ok"}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("fmt", "1.0", []string{})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTracingMiddleware_Passthrough(t *testing.T) {
	t.Parallel()
	mw := TracingMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return &FormatResult{Content: "traced", Success: true}, nil
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	result, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.NoError(t, err)
	assert.Equal(t, "traced", result.Content)
}

func TestTracingMiddleware_Error(t *testing.T) {
	t.Parallel()
	mw := TracingMiddleware()

	base := func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
		return nil, fmt.Errorf("trace error")
	}

	wrapped := mw(base)
	mock := newMockFormatter("test", "1.0", []string{"go"})
	_, err := wrapped(context.Background(), mock, &FormatRequest{Content: "code"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trace error")
}

func TestFormatterExecutor_BuildChain_MultipleMiddleware(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	registerMock(t, registry, "black", []string{"python"})

	executor := newTestExecutor(t, registry, false)

	order := make([]string, 0)

	mw1 := func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
			order = append(order, "mw1-before")
			result, err := next(ctx, f, req)
			order = append(order, "mw1-after")
			return result, err
		}
	}

	mw2 := func(next ExecuteFunc) ExecuteFunc {
		return func(ctx context.Context, f Formatter, req *FormatRequest) (*FormatResult, error) {
			order = append(order, "mw2-before")
			result, err := next(ctx, f, req)
			order = append(order, "mw2-after")
			return result, err
		}
	}

	executor.Use(mw1, mw2)

	ctx := context.Background()
	_, err := executor.Execute(ctx, &FormatRequest{Content: "x=1", Language: "python"})
	assert.NoError(t, err)

	// Middleware applied in order: mw1 wraps mw2 wraps base
	assert.Equal(t, []string{"mw1-before", "mw2-before", "mw2-after", "mw1-after"}, order)
}
