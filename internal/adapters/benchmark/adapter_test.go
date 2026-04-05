package benchmark_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	benchmarkadapter "dev.helix.agent/internal/adapters/benchmark"
	extbenchmark "digital.vasic.benchmark/benchmark"
)

// mockLLMProvider implements extbenchmark.LLMProvider for unit tests.
type mockLLMProvider struct{ name string }

func (m *mockLLMProvider) Complete(_ context.Context, _, _ string) (string, int, error) {
	return "mock response", 10, nil
}

func (m *mockLLMProvider) GetName() string { return m.name }

// mockProviderService implements extbenchmark.ProviderServiceForBenchmark for unit tests.
type mockProviderService struct{}

func (m *mockProviderService) Complete(_ context.Context, _, _, _, _ string) (string, int, error) {
	return "mock response", 10, nil
}

func (m *mockProviderService) GetProvider(name string) extbenchmark.LLMProvider {
	return &mockLLMProvider{name: name}
}

func TestAdapter_New_NilLogger(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	require.NotNil(t, adapter, "New with nil logger should return a non-nil adapter")
}

func TestAdapter_New_WithLogger(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := benchmarkadapter.New(logger)
	require.NotNil(t, adapter)
}

func TestAdapter_ListBenchmarks_BeforeInit(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	// Before Initialize, GetRunner returns nil — ListBenchmarks should return nil, nil
	benchmarks, err := adapter.ListBenchmarks(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, benchmarks)
}

func TestAdapter_Initialize(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	err := adapter.Initialize(svc, "test-provider", "test-model")
	require.NoError(t, err)
}

func TestAdapter_GetRunner_AfterInit(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	require.NoError(t, adapter.Initialize(svc, "test-provider", "test-model"))

	runner := adapter.GetRunner()
	require.NotNil(t, runner, "GetRunner should return a non-nil runner after Initialize")
}

func TestAdapter_ListBenchmarks_AfterInit(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	require.NoError(t, adapter.Initialize(svc, "test-provider", "test-model"))

	benchmarks, err := adapter.ListBenchmarks(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, benchmarks, "expected built-in benchmarks to be listed after initialization")
}

func TestAdapter_RunBenchmarkWithBestProvider_NilConfig(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	require.NoError(t, adapter.Initialize(svc, "test-provider", "test-model"))

	// nil config should use defaults without panicking
	run, err := adapter.RunBenchmarkWithBestProvider(
		context.Background(),
		extbenchmark.BenchmarkTypeMMLU,
		nil,
	)
	// Run may be nil if no verifier is set, but no panic is expected
	_ = run
	_ = err
}

func TestAdapter_CompareProviders_EmptyList(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	require.NoError(t, adapter.Initialize(svc, "test-provider", "test-model"))

	runs, err := adapter.CompareProviders(
		context.Background(),
		extbenchmark.BenchmarkTypeGSM8K,
		[]string{},
		nil,
	)
	_ = err
	assert.Empty(t, runs)
}

func TestAdapter_GenerateLeaderboard(t *testing.T) {
	t.Parallel()
	adapter := benchmarkadapter.New(nil)
	svc := &mockProviderService{}
	require.NoError(t, adapter.Initialize(svc, "test-provider", "test-model"))

	lb, err := adapter.GenerateLeaderboard(context.Background(), extbenchmark.BenchmarkTypeGSM8K)
	// No runs completed yet — leaderboard may be empty but should not panic
	_ = lb
	_ = err
}
