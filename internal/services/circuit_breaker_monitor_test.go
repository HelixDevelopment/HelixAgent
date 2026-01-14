package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
)

func TestCircuitBreakerMonitor_Creation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("NewCircuitBreakerMonitor creates monitor with default config", func(t *testing.T) {
		manager := llm.NewDefaultCircuitBreakerManager()
		config := DefaultCircuitBreakerMonitorConfig()

		monitor := NewCircuitBreakerMonitor(manager, logger, config)

		require.NotNil(t, monitor)
		assert.Equal(t, 10*time.Second, monitor.checkInterval)
		assert.Equal(t, 3, monitor.alertThreshold)
	})

	t.Run("DefaultConfig has sensible values", func(t *testing.T) {
		config := DefaultCircuitBreakerMonitorConfig()

		assert.Equal(t, 10*time.Second, config.CheckInterval)
		assert.Equal(t, 3, config.AlertThreshold)
	})
}

func TestCircuitBreakerMonitor_AlertListener(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("AddAlertListener adds listener", func(t *testing.T) {
		manager := llm.NewDefaultCircuitBreakerManager()
		monitor := NewCircuitBreakerMonitor(manager, logger, DefaultCircuitBreakerMonitorConfig())

		alertReceived := make(chan CircuitBreakerAlert, 1)
		monitor.AddAlertListener(func(alert CircuitBreakerAlert) {
			alertReceived <- alert
		})

		assert.Len(t, monitor.listeners, 1)
	})
}

func TestCircuitBreakerMonitor_GetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetStatus returns healthy when no circuits", func(t *testing.T) {
		manager := llm.NewDefaultCircuitBreakerManager()
		monitor := NewCircuitBreakerMonitor(manager, logger, DefaultCircuitBreakerMonitorConfig())

		status := monitor.GetStatus()

		assert.True(t, status.Healthy)
		assert.Equal(t, 0, status.OpenCount)
		assert.Equal(t, 0, status.TotalCount)
	})

	t.Run("GetStatus returns status when manager is nil", func(t *testing.T) {
		monitor := NewCircuitBreakerMonitor(nil, logger, DefaultCircuitBreakerMonitorConfig())

		status := monitor.GetStatus()

		assert.True(t, status.Healthy)
		assert.Empty(t, status.Providers)
	})
}

func TestCircuitBreakerMonitor_StartStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("Start and Stop work correctly", func(t *testing.T) {
		manager := llm.NewDefaultCircuitBreakerManager()
		config := CircuitBreakerMonitorConfig{
			CheckInterval:  100 * time.Millisecond,
			AlertThreshold: 3,
		}
		monitor := NewCircuitBreakerMonitor(manager, logger, config)

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
		manager := llm.NewDefaultCircuitBreakerManager()
		config := CircuitBreakerMonitorConfig{
			CheckInterval:  100 * time.Millisecond,
			AlertThreshold: 3,
		}
		monitor := NewCircuitBreakerMonitor(manager, logger, config)

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

func TestCircuitBreakerMonitor_ResetCircuitBreaker(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("ResetCircuitBreaker works for nil manager", func(t *testing.T) {
		monitor := NewCircuitBreakerMonitor(nil, logger, DefaultCircuitBreakerMonitorConfig())

		err := monitor.ResetCircuitBreaker("test-provider")
		assert.NoError(t, err)
	})

	t.Run("ResetAllCircuitBreakers works for nil manager", func(t *testing.T) {
		monitor := NewCircuitBreakerMonitor(nil, logger, DefaultCircuitBreakerMonitorConfig())

		// Should not panic
		monitor.ResetAllCircuitBreakers()
	})
}

func TestCalculateSuccessRate(t *testing.T) {
	t.Run("Returns 100% for no requests", func(t *testing.T) {
		stat := llm.CircuitBreakerStats{
			TotalRequests: 0,
		}
		rate := calculateSuccessRate(stat)
		assert.Equal(t, 100.0, rate)
	})

	t.Run("Calculates correct rate", func(t *testing.T) {
		stat := llm.CircuitBreakerStats{
			TotalRequests:  100,
			TotalSuccesses: 75,
		}
		rate := calculateSuccessRate(stat)
		assert.Equal(t, 75.0, rate)
	})

	t.Run("Returns 0% for all failures", func(t *testing.T) {
		stat := llm.CircuitBreakerStats{
			TotalRequests:  100,
			TotalSuccesses: 0,
		}
		rate := calculateSuccessRate(stat)
		assert.Equal(t, 0.0, rate)
	})
}
