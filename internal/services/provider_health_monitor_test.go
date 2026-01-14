package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		assert.Len(t, monitor.listeners, 1)
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

		// Wait a bit
		time.Sleep(200 * time.Millisecond)

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

		// Wait a bit
		time.Sleep(200 * time.Millisecond)

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
