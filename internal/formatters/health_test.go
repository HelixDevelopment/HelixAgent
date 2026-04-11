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

func newTestHealthChecker(t *testing.T) (*HealthChecker, *FormatterRegistry) {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	registry := NewFormatterRegistry(&RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrent:  10,
	}, logger)
	hc := NewHealthChecker(registry, logger, 10*time.Second)
	return hc, registry
}

func TestNewHealthChecker(t *testing.T) {
	t.Parallel()
	hc, _ := newTestHealthChecker(t)
	assert.NotNil(t, hc)
}

func TestHealthChecker_CheckAll_AllHealthy(t *testing.T) {
	t.Parallel()
	hc, registry := newTestHealthChecker(t)

	// Register healthy formatters
	for _, name := range []string{"black", "gofmt"} {
		mock := newMockFormatter(name, "1.0", []string{"python"})
		mock.healthFunc = func(ctx context.Context) error { return nil }
		err := registry.Register(mock, &FormatterMetadata{
			Name: name, Version: "1.0", Languages: []string{"python"},
			Type: FormatterTypeNative,
		})
		require.NoError(t, err)
	}

	report := hc.CheckAll(context.Background())

	assert.Equal(t, 2, report.TotalFormatters)
	assert.Equal(t, 2, report.HealthyCount)
	assert.Equal(t, 0, report.UnhealthyCount)
	assert.True(t, report.IsHealthy())
	assert.Equal(t, 100.0, report.HealthPercentage())
	assert.NotZero(t, report.Timestamp)
}

func TestHealthChecker_CheckAll_SomeUnhealthy(t *testing.T) {
	t.Parallel()
	hc, registry := newTestHealthChecker(t)

	healthy := newMockFormatter("black", "1.0", []string{"python"})
	healthy.healthFunc = func(ctx context.Context) error { return nil }
	err := registry.Register(healthy, &FormatterMetadata{
		Name: "black", Version: "1.0", Languages: []string{"python"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)

	unhealthy := newMockFormatter("broken", "1.0", []string{"go"})
	unhealthy.healthFunc = func(ctx context.Context) error {
		return fmt.Errorf("service unavailable")
	}
	err = registry.Register(unhealthy, &FormatterMetadata{
		Name: "broken", Version: "1.0", Languages: []string{"go"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)

	report := hc.CheckAll(context.Background())

	assert.Equal(t, 2, report.TotalFormatters)
	assert.Equal(t, 1, report.HealthyCount)
	assert.Equal(t, 1, report.UnhealthyCount)
	assert.False(t, report.IsHealthy())
	assert.Equal(t, 50.0, report.HealthPercentage())
}

func TestHealthChecker_CheckAll_Empty(t *testing.T) {
	t.Parallel()
	hc, _ := newTestHealthChecker(t)

	report := hc.CheckAll(context.Background())

	assert.Equal(t, 0, report.TotalFormatters)
	assert.Equal(t, 0, report.HealthyCount)
	assert.Equal(t, 0, report.UnhealthyCount)
	assert.True(t, report.IsHealthy())
}

func TestHealthChecker_Check_Healthy(t *testing.T) {
	t.Parallel()
	hc, registry := newTestHealthChecker(t)

	mock := newMockFormatter("black", "1.0", []string{"python"})
	mock.healthFunc = func(ctx context.Context) error { return nil }
	err := registry.Register(mock, &FormatterMetadata{
		Name: "black", Version: "1.0", Languages: []string{"python"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)

	result, err := hc.Check(context.Background(), "black")
	require.NoError(t, err)
	assert.True(t, result.Healthy)
	assert.Equal(t, "black", result.Name)
	assert.Empty(t, result.Error)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestHealthChecker_Check_Unhealthy(t *testing.T) {
	t.Parallel()
	hc, registry := newTestHealthChecker(t)

	mock := newMockFormatter("broken", "1.0", []string{"python"})
	mock.healthFunc = func(ctx context.Context) error {
		return fmt.Errorf("connection refused")
	}
	err := registry.Register(mock, &FormatterMetadata{
		Name: "broken", Version: "1.0", Languages: []string{"python"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)

	result, err := hc.Check(context.Background(), "broken")
	require.NoError(t, err)
	assert.False(t, result.Healthy)
	assert.Contains(t, result.Error, "connection refused")
}

func TestHealthChecker_Check_NotFound(t *testing.T) {
	t.Parallel()
	hc, _ := newTestHealthChecker(t)

	_, err := hc.Check(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formatter not found")
}

func TestHealthReport_IsHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		unhealthy int
		expected  bool
	}{
		{"all healthy", 0, true},
		{"some unhealthy", 1, false},
		{"all unhealthy", 5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := &HealthReport{UnhealthyCount: tc.unhealthy}
			assert.Equal(t, tc.expected, report.IsHealthy())
		})
	}
}

func TestHealthReport_HealthPercentage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		total    int
		healthy  int
		expected float64
	}{
		{"all healthy", 4, 4, 100.0},
		{"half healthy", 4, 2, 50.0},
		{"none healthy", 4, 0, 0.0},
		{"empty", 0, 0, 100.0},
		{"one of three", 3, 1, 33.33333333333333},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := &HealthReport{
				TotalFormatters: tc.total,
				HealthyCount:    tc.healthy,
			}
			assert.InDelta(t, tc.expected, report.HealthPercentage(), 0.01)
		})
	}
}

func TestFormatterHealthResult_Fields(t *testing.T) {
	t.Parallel()
	result := FormatterHealthResult{
		Name:     "black",
		Healthy:  true,
		Error:    "",
		Duration: 5 * time.Millisecond,
	}

	assert.Equal(t, "black", result.Name)
	assert.True(t, result.Healthy)
	assert.Empty(t, result.Error)
	assert.Equal(t, 5*time.Millisecond, result.Duration)
}

func TestHealthChecker_CheckAll_FormatterResults(t *testing.T) {
	t.Parallel()
	hc, registry := newTestHealthChecker(t)

	mock := newMockFormatter("black", "1.0", []string{"python"})
	mock.healthFunc = func(ctx context.Context) error { return nil }
	err := registry.Register(mock, &FormatterMetadata{
		Name: "black", Version: "1.0", Languages: []string{"python"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)

	report := hc.CheckAll(context.Background())

	assert.Len(t, report.FormatterResults, 1)
	assert.Equal(t, "black", report.FormatterResults[0].Name)
	assert.True(t, report.FormatterResults[0].Healthy)
}
