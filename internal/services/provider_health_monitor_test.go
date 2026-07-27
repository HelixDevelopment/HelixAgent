package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitUntilProviderHealthMonitorRunning waits until the monitor has entered its run loop.
func waitUntilProviderHealthMonitorRunning(t *testing.T, monitor *ProviderHealthMonitor) {
	t.Helper()
	require.Eventually(t, func() bool {
		return monitor.running.Load()
	}, 2*time.Second, time.Millisecond)
}

func TestProviderHealthMonitor_Creation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("NewProviderHealthMonitor creates monitor with default config", func(t *testing.T) {
		config := DefaultProviderHealthMonitorConfig()

		monitor := NewProviderHealthMonitor(nil, logger, config)

		require.NotNil(t, monitor)
		assert.Equal(t, 30*time.Second, monitor.checkInterval)
		assert.Equal(t, 10*time.Second, monitor.healthTimeout)
	})

	t.Run("DefaultConfig has sensible values", func(t *testing.T) {
		config := DefaultProviderHealthMonitorConfig()

		assert.Equal(t, 30*time.Second, config.CheckInterval)
		assert.Equal(t, 10*time.Second, config.HealthTimeout)
		assert.Equal(t, 3, config.AlertAfterFails)
	})
}

func TestProviderHealthMonitor_AlertListener(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("AddAlertListener adds listener", func(t *testing.T) {
		monitor := NewProviderHealthMonitor(nil, logger, DefaultProviderHealthMonitorConfig())

		alertReceived := make(chan ProviderHealthAlert, 1)
		monitor.AddAlertListener(func(alert ProviderHealthAlert) {
			alertReceived <- alert
		})

		assert.Equal(t, 1, monitor.listeners.Len())
	})
}

func TestProviderHealthMonitor_GetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetStatus returns healthy when no providers", func(t *testing.T) {
		monitor := NewProviderHealthMonitor(nil, logger, DefaultProviderHealthMonitorConfig())

		status := monitor.GetStatus()

		assert.True(t, status.Healthy)
		assert.Equal(t, 0, status.HealthyCount)
		assert.Equal(t, 0, status.UnhealthyCount)
		assert.Equal(t, 0, status.TotalCount)
	})
}

func TestProviderHealthMonitor_GetProviderStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetProviderStatus returns false for non-existent provider", func(t *testing.T) {
		monitor := NewProviderHealthMonitor(nil, logger, DefaultProviderHealthMonitorConfig())

		status, exists := monitor.GetProviderStatus("non-existent")

		assert.False(t, exists)
		assert.Nil(t, status)
	})
}

func TestProviderHealthMonitor_StartStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("Start and Stop work correctly", func(t *testing.T) {
		config := ProviderHealthMonitorConfig{
			CheckInterval:   100 * time.Millisecond,
			HealthTimeout:   50 * time.Millisecond,
			AlertAfterFails: 3,
		}
		monitor := NewProviderHealthMonitor(nil, logger, config)

		ctx, cancel := context.WithCancel(context.Background())

		// Start in goroutine
		done := make(chan struct{})
		go func() {
			monitor.Start(ctx)
			close(done)
		}()

		// Stop via context
		cancel()

		// Wait for completion
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Monitor did not stop within timeout")
		}
	})

	t.Run("Stop method works", func(t *testing.T) {
		config := ProviderHealthMonitorConfig{
			CheckInterval:   100 * time.Millisecond,
			HealthTimeout:   50 * time.Millisecond,
			AlertAfterFails: 3,
		}
		monitor := NewProviderHealthMonitor(nil, logger, config)

		ctx := context.Background()

		// Start in goroutine
		done := make(chan struct{})
		go func() {
			monitor.Start(ctx)
			close(done)
		}()

		// Wait until monitor is running before stopping
		waitUntilProviderHealthMonitorRunning(t, monitor)

		// Stop via method
		monitor.Stop()

		// Wait for completion
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Monitor did not stop within timeout")
		}
	})
}

func TestProviderHealthMonitor_ForceCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("ForceCheck works with nil registry", func(t *testing.T) {
		monitor := NewProviderHealthMonitor(nil, logger, DefaultProviderHealthMonitorConfig())

		// Should not panic
		monitor.ForceCheck(context.Background())
	})

	t.Run("ForceCheckProvider works with nil registry", func(t *testing.T) {
		monitor := NewProviderHealthMonitor(nil, logger, DefaultProviderHealthMonitorConfig())

		// Should not panic
		monitor.ForceCheckProvider(context.Background(), "test-provider")
	})
}

func TestMonitoredProviderHealth_Fields(t *testing.T) {
	t.Run("MonitoredProviderHealth contains all required fields", func(t *testing.T) {
		status := &MonitoredProviderHealth{
			ProviderID:       "test-provider",
			Healthy:          true,
			LastCheck:        time.Now(),
			LastSuccess:      time.Now(),
			LastError:        "",
			ConsecutiveFails: 0,
			ResponseTime:     100 * time.Millisecond,
			CheckCount:       10,
			FailCount:        0,
		}

		assert.Equal(t, "test-provider", status.ProviderID)
		assert.True(t, status.Healthy)
		assert.NotZero(t, status.LastCheck)
		assert.Equal(t, int64(10), status.CheckCount)
	})
}

func TestProviderHealthAlert_Fields(t *testing.T) {
	t.Run("ProviderHealthAlert contains all required fields", func(t *testing.T) {
		alert := ProviderHealthAlert{
			Type:             "provider_unhealthy",
			ProviderID:       "test-provider",
			Message:          "Provider failed health check",
			Timestamp:        time.Now(),
			ConsecutiveFails: 3,
			LastError:        "connection refused",
		}

		assert.Equal(t, "provider_unhealthy", alert.Type)
		assert.Equal(t, "test-provider", alert.ProviderID)
		assert.NotEmpty(t, alert.Message)
		assert.Equal(t, 3, alert.ConsecutiveFails)
	})
}

// TestDeriveTier covers the verifier filter taxonomy (CONST-032 +
// "LLMsVerifier MUST be capable of filtering providers and models
// properly"). Each provider is sorted into one of:
//   - "verified":   has a recorded LastSuccess
//   - "configured": registered but not yet probed (or transient down)
//   - "dead":       terminal auth signal in LastError, or 5+ failures
//     with no prior success
func TestDeriveTier(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name string
		in   *MonitoredProviderHealth
		want string
	}{
		{"nil → unknown", nil, "unknown"},
		{
			name: "fresh registration → configured",
			in:   &MonitoredProviderHealth{ProviderID: "x"},
			want: "configured",
		},
		{
			name: "had a success → verified",
			in:   &MonitoredProviderHealth{ProviderID: "x", LastSuccess: now},
			want: "verified",
		},
		{
			name: "401 → dead",
			in:   &MonitoredProviderHealth{ProviderID: "x", LastError: "auth failed: status 401"},
			want: "dead",
		},
		{
			name: "403 → dead",
			in:   &MonitoredProviderHealth{ProviderID: "x", LastError: "remote returned 403 Forbidden"},
			want: "dead",
		},
		{
			name: "discontinued tier → dead",
			in:   &MonitoredProviderHealth{ProviderID: "qwen-oauth", LastError: "qwen CLI failed: Qwen OAuth free tier was discontinued on 2026-04-15"},
			want: "dead",
		},
		{
			name: "JetBrains subscription → dead",
			in:   &MonitoredProviderHealth{ProviderID: "junie", LastError: "Junie: 403 Forbidden: No active JetBrains AI subscription found"},
			want: "dead",
		},
		{
			name: "insufficient balance → dead",
			in:   &MonitoredProviderHealth{ProviderID: "zai", LastError: "Zhipu GLM API error: insufficient balance - please recharge"},
			want: "dead",
		},
		{
			name: "5+ fails no prior success → dead",
			in:   &MonitoredProviderHealth{ProviderID: "x", ConsecutiveFails: 5},
			want: "dead",
		},
		{
			name: "5+ fails but had prior success → verified (transient)",
			in:   &MonitoredProviderHealth{ProviderID: "x", ConsecutiveFails: 5, LastSuccess: now.Add(-time.Hour)},
			want: "verified",
		},
		{
			name: "transient 500 (no auth signal, < 5 fails) → configured",
			in:   &MonitoredProviderHealth{ProviderID: "x", LastError: "internal server error", ConsecutiveFails: 2},
			want: "configured",
		},
	}
	for _, c := range cases {
		got := deriveTier(c.in)
		assert.Equal(t, c.want, got, c.name)
	}
}
