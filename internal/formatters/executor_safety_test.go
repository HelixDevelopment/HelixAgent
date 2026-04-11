//go:build !integration

package formatters

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteBatch_PanicRecovery verifies that a panicking formatter in ExecuteBatch
// does not hang the call or crash the process — the panic is recovered and returned
// as an error in the result set.
func TestExecuteBatch_PanicRecovery(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)

	// Register a formatter that panics on Format
	panicMock := newMockFormatter("panicker", "1.0.0", []string{"python"})
	panicMock.formatFunc = func(ctx context.Context, req *FormatRequest) (*FormatResult, error) {
		panic("simulated formatter panic")
	}
	metadata := &FormatterMetadata{
		Name:      "panicker",
		Version:   "1.0.0",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}
	err := registry.Register(panicMock, metadata)
	require.NoError(t, err)

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	reqs := []*FormatRequest{
		{Content: "x=1", Language: "python"},
	}

	// Must return without hanging; panic must be converted to an error
	results, err := executor.ExecuteBatch(ctx, reqs)

	assert.Error(t, err, "expected an error from the panicking formatter")
	assert.True(t, strings.Contains(err.Error(), "panic"),
		"error message should mention panic, got: %s", err.Error())
	assert.Len(t, results, 1)
	assert.Nil(t, results[0], "result entry for panicked formatter should be nil")
}

// TestExecuteBatch_PanicRecovery_MixedBatch verifies that a panic in one goroutine
// does not prevent the other formatters from completing — all slots are filled.
func TestExecuteBatch_PanicRecovery_MixedBatch(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)

	// Healthy formatter for "go"
	registerMock(t, registry, "gofmt", []string{"go"})

	// Panicking formatter for "python"
	panicMock := newMockFormatter("panicker", "1.0.0", []string{"python"})
	panicMock.formatFunc = func(ctx context.Context, req *FormatRequest) (*FormatResult, error) {
		panic("boom")
	}
	panicMeta := &FormatterMetadata{
		Name:      "panicker",
		Version:   "1.0.0",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}
	err := registry.Register(panicMock, panicMeta)
	require.NoError(t, err)

	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	reqs := []*FormatRequest{
		{Content: "package main", Language: "go"},
		{Content: "x=1", Language: "python"},
	}

	results, err := executor.ExecuteBatch(ctx, reqs)

	// At least one formatter panicked so an error must be returned
	assert.Error(t, err)
	// All result slots must be present (len == number of requests)
	assert.Len(t, results, 2)
	// The go result should be non-nil and successful
	goIdx, pyIdx := 0, 1
	assert.NotNil(t, results[goIdx], "go formatter result should not be nil")
	assert.Contains(t, results[goIdx].Content, "formatted")
	// The python/panic slot should be nil
	assert.Nil(t, results[pyIdx], "panicking formatter result should be nil")
}

// TestExecuteBatch_EmptyBatch verifies that an empty (nil or zero-length) slice
// returns immediately with no error and an empty result slice.
func TestExecuteBatch_EmptyBatch(t *testing.T) {
	t.Parallel()
	registry := newTestRegistry(t)
	executor := newTestExecutor(t, registry, false)
	ctx := context.Background()

	t.Run("nil slice", func(t *testing.T) {
		t.Parallel()
		results, err := executor.ExecuteBatch(ctx, nil)
		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		results, err := executor.ExecuteBatch(ctx, []*FormatRequest{})
		assert.NoError(t, err)
		assert.Empty(t, results)
	})
}
