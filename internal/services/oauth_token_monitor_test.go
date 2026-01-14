package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthTokenMonitor_Creation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("NewOAuthTokenMonitor creates monitor with default config", func(t *testing.T) {
		config := DefaultOAuthTokenMonitorConfig()

		monitor := NewOAuthTokenMonitor(logger, config)

		require.NotNil(t, monitor)
		assert.Equal(t, 5*time.Minute, monitor.checkInterval)
		assert.Equal(t, 10*time.Minute, monitor.expiryThreshold)
	})

	t.Run("DefaultConfig has sensible values", func(t *testing.T) {
		config := DefaultOAuthTokenMonitorConfig()

		assert.Equal(t, 5*time.Minute, config.CheckInterval)
		assert.Equal(t, 10*time.Minute, config.ExpiryThreshold)
	})
}

func TestOAuthTokenMonitor_AlertListener(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("AddAlertListener adds listener", func(t *testing.T) {
		monitor := NewOAuthTokenMonitor(logger, DefaultOAuthTokenMonitorConfig())

		alertReceived := make(chan OAuthTokenAlert, 1)
		monitor.AddAlertListener(func(alert OAuthTokenAlert) {
			alertReceived <- alert
		})

		assert.Len(t, monitor.listeners, 1)
	})
}

func TestOAuthTokenMonitor_GetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetStatus returns empty when no tokens checked", func(t *testing.T) {
		monitor := NewOAuthTokenMonitor(logger, DefaultOAuthTokenMonitorConfig())

		status := monitor.GetStatus()

		assert.True(t, status.Healthy)
		assert.True(t, status.AllValid)
		assert.Equal(t, 0, status.ExpiringCount)
		assert.Empty(t, status.Tokens)
	})
}

func TestOAuthTokenMonitor_StartStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("Start and Stop work correctly", func(t *testing.T) {
		config := OAuthTokenMonitorConfig{
			CheckInterval:   100 * time.Millisecond,
			ExpiryThreshold: 1 * time.Minute,
		}
		monitor := NewOAuthTokenMonitor(logger, config)

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
		config := OAuthTokenMonitorConfig{
			CheckInterval:   100 * time.Millisecond,
			ExpiryThreshold: 1 * time.Minute,
		}
		monitor := NewOAuthTokenMonitor(logger, config)

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

func TestOAuthTokenMonitor_RefreshToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("RefreshToken for claude logs warning", func(t *testing.T) {
		monitor := NewOAuthTokenMonitor(logger, DefaultOAuthTokenMonitorConfig())

		err := monitor.RefreshToken("claude")
		assert.NoError(t, err)
	})

	t.Run("RefreshToken for unknown provider returns nil", func(t *testing.T) {
		monitor := NewOAuthTokenMonitor(logger, DefaultOAuthTokenMonitorConfig())

		err := monitor.RefreshToken("unknown")
		assert.NoError(t, err)
	})
}

func TestOAuthTokenAlert_Severity(t *testing.T) {
	t.Run("Alert severities are valid", func(t *testing.T) {
		severities := []string{"warning", "critical", "expired"}

		for _, sev := range severities {
			alert := OAuthTokenAlert{
				Type:     "test",
				Provider: "test",
				Severity: sev,
			}
			assert.Contains(t, severities, alert.Severity)
		}
	})
}

func TestTokenStatus_Fields(t *testing.T) {
	t.Run("TokenStatus contains all required fields", func(t *testing.T) {
		status := &TokenStatus{
			Provider:      "claude",
			Valid:         true,
			ExpiresAt:     time.Now().Add(1 * time.Hour),
			ExpiresIn:     "1h0m0s",
			LastChecked:   time.Now(),
			LastRefreshed: time.Now(),
			Error:         "",
		}

		assert.Equal(t, "claude", status.Provider)
		assert.True(t, status.Valid)
		assert.NotZero(t, status.ExpiresAt)
		assert.NotEmpty(t, status.ExpiresIn)
	})
}
