package services

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/auth/oauth_credentials"
)

// Package-level metrics (registered once)
var (
	otmMetricsOnce       sync.Once
	otmTokenExpiryGauge  *prometheus.GaugeVec
	otmTokenValidGauge   *prometheus.GaugeVec
	otmTokenAlertsTotal  *prometheus.CounterVec
	otmTokenRefreshTotal *prometheus.CounterVec
)

func initOTMMetrics() {
	otmMetricsOnce.Do(func() {
		otmTokenExpiryGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helixagent_oauth_token_expiry_seconds",
				Help: "Seconds until OAuth token expires",
			},
			[]string{"provider"},
		)

		otmTokenValidGauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helixagent_oauth_token_valid",
				Help: "Whether OAuth token is valid (1=valid, 0=invalid/expired)",
			},
			[]string{"provider"},
		)

		otmTokenAlertsTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helixagent_oauth_token_alerts_total",
				Help: "Total number of OAuth token alerts by severity",
			},
			[]string{"provider", "severity"},
		)

		otmTokenRefreshTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helixagent_oauth_token_refresh_total",
				Help: "Total number of OAuth token refresh attempts",
			},
			[]string{"provider", "status"},
		)
	})
}

// OAuthTokenMonitor monitors OAuth token expiry and sends alerts.
//
// Concurrency model (CONST-029): listeners is *safe.Slice;
// tokenStatus is *safe.Store; running is atomic.Bool.
type OAuthTokenMonitor struct {
	logger          *logrus.Logger
	checkInterval   time.Duration
	expiryThreshold time.Duration // Alert when token expires within this duration
	listeners       *safe.Slice[OAuthTokenAlertListener]
	stopCh          chan struct{}
	running         atomic.Bool

	// Token status cache
	tokenStatus *safe.Store[string, *TokenStatus]
}

// OAuthTokenAlertListener is called when token alerts occur
type OAuthTokenAlertListener func(alert OAuthTokenAlert)

// OAuthTokenAlert represents an alert from the monitor
type OAuthTokenAlert struct {
	Type      string    `json:"type"`
	Provider  string    `json:"provider"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	ExpiresIn string    `json:"expires_in,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"` // warning, critical, expired
}

// TokenStatus represents the status of a token
type TokenStatus struct {
	Provider      string    `json:"provider"`
	Valid         bool      `json:"valid"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	ExpiresIn     string    `json:"expires_in,omitempty"`
	LastChecked   time.Time `json:"last_checked"`
	LastRefreshed time.Time `json:"last_refreshed,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// OAuthTokenMonitorConfig configures the monitor
type OAuthTokenMonitorConfig struct {
	CheckInterval   time.Duration
	ExpiryThreshold time.Duration // Alert when token expires within this duration
}

// DefaultOAuthTokenMonitorConfig returns default configuration
func DefaultOAuthTokenMonitorConfig() OAuthTokenMonitorConfig {
	return OAuthTokenMonitorConfig{
		CheckInterval:   5 * time.Minute,
		ExpiryThreshold: 10 * time.Minute, // Alert 10 minutes before expiry
	}
}

// NewOAuthTokenMonitor creates a new OAuth token monitor
func NewOAuthTokenMonitor(logger *logrus.Logger, config OAuthTokenMonitorConfig) *OAuthTokenMonitor {
	// Initialize package-level metrics (idempotent)
	initOTMMetrics()

	return &OAuthTokenMonitor{
		logger:          logger,
		checkInterval:   config.CheckInterval,
		expiryThreshold: config.ExpiryThreshold,
		listeners:       safe.NewSlice[OAuthTokenAlertListener](),
		stopCh:          make(chan struct{}),
		tokenStatus:     safe.NewStore[string, *TokenStatus](),
	}
}

// AddAlertListener adds a listener for alerts
func (otm *OAuthTokenMonitor) AddAlertListener(listener OAuthTokenAlertListener) {
	otm.listeners.Append(listener)
}

// Start starts the monitoring loop
func (otm *OAuthTokenMonitor) Start(ctx context.Context) {
	if !otm.running.CompareAndSwap(false, true) {
		return
	}

	otm.logger.Info("OAuth token monitor started")

	// Initial check
	otm.checkTokens()

	ticker := time.NewTicker(otm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			otm.logger.Info("OAuth token monitor stopped (context cancelled)")
			return
		case <-otm.stopCh:
			otm.logger.Info("OAuth token monitor stopped")
			return
		case <-ticker.C:
			otm.checkTokens()
		}
	}
}

// Stop stops the monitoring loop
func (otm *OAuthTokenMonitor) Stop() {
	if otm.running.CompareAndSwap(true, false) {
		close(otm.stopCh)
	}
}

// checkTokens checks all OAuth tokens
func (otm *OAuthTokenMonitor) checkTokens() {
	// Check Claude OAuth token
	if os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS") == "true" {
		otm.checkClaudeToken()
	}

	// Check Qwen OAuth token
	if os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS") == "true" {
		otm.checkQwenToken()
	}
}

// isNotConfiguredError checks if an error indicates OAuth is not configured (vs actual failure)
func isNotConfiguredError(errMsg string) bool {
	notConfiguredPhrases := []string{
		"file not found",
		"not logged in",
		"no such file",
		"credentials not found",
		"user may not be logged in",
	}
	errLower := strings.ToLower(errMsg)
	for _, phrase := range notConfiguredPhrases {
		if strings.Contains(errLower, phrase) {
			return true
		}
	}
	return false
}

// checkClaudeToken checks the Claude OAuth token
func (otm *OAuthTokenMonitor) checkClaudeToken() {
	reader := oauth_credentials.NewOAuthCredentialReader()
	creds, err := reader.ReadClaudeCredentials()

	status := &TokenStatus{
		Provider:    "claude",
		LastChecked: time.Now(),
	}

	if err != nil {
		status.Valid = false
		status.Error = err.Error()
		otmTokenValidGauge.WithLabelValues("claude").Set(0)

		// Determine severity: "not configured" is informational, actual errors are warnings
		severity := "info"
		alertType := "token_not_configured"
		if !isNotConfiguredError(err.Error()) {
			severity = "warning"
			alertType = "token_error"
		}

		otm.sendAlert(OAuthTokenAlert{
			Type:      alertType,
			Provider:  "claude",
			Message:   "Failed to read Claude OAuth credentials: " + err.Error(),
			Timestamp: time.Now(),
			Severity:  severity,
		})
	} else if creds == nil || creds.ClaudeAiOauth == nil {
		status.Valid = false
		status.Error = "Credentials structure is nil"
		otmTokenValidGauge.WithLabelValues("claude").Set(0)
	} else if oauth_credentials.IsExpired(creds.ClaudeAiOauth.ExpiresAt) {
		expiresAt := time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt)
		status.Valid = false
		status.ExpiresAt = expiresAt
		status.Error = "Token expired"
		otmTokenValidGauge.WithLabelValues("claude").Set(0)
		otmTokenExpiryGauge.WithLabelValues("claude").Set(0)
		otm.sendAlert(OAuthTokenAlert{
			Type:      "token_expired",
			Provider:  "claude",
			Message:   "Claude OAuth token has expired",
			ExpiresAt: expiresAt,
			Timestamp: time.Now(),
			Severity:  "expired",
		})
	} else {
		expiresAt := time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt)
		status.Valid = true
		status.ExpiresAt = expiresAt
		expiresIn := time.Until(expiresAt)
		status.ExpiresIn = expiresIn.Round(time.Second).String()

		otmTokenValidGauge.WithLabelValues("claude").Set(1)
		otmTokenExpiryGauge.WithLabelValues("claude").Set(expiresIn.Seconds())

		// Check if expiring soon
		if expiresIn <= otm.expiryThreshold {
			severity := "warning"
			if expiresIn <= 2*time.Minute {
				severity = "critical"
			}
			otm.sendAlert(OAuthTokenAlert{
				Type:      "token_expiring_soon",
				Provider:  "claude",
				Message:   "Claude OAuth token expires in " + status.ExpiresIn,
				ExpiresAt: expiresAt,
				ExpiresIn: status.ExpiresIn,
				Timestamp: time.Now(),
				Severity:  severity,
			})
		}
	}

	otm.updateTokenStatus("claude", status)
}

// checkQwenToken checks the Qwen OAuth token
func (otm *OAuthTokenMonitor) checkQwenToken() {
	reader := oauth_credentials.NewOAuthCredentialReader()
	creds, err := reader.ReadQwenCredentials()

	status := &TokenStatus{
		Provider:    "qwen",
		LastChecked: time.Now(),
	}

	if err != nil {
		status.Valid = false
		status.Error = err.Error()
		otmTokenValidGauge.WithLabelValues("qwen").Set(0)

		// Determine severity: "not configured" is informational, actual errors are warnings
		severity := "info"
		alertType := "token_not_configured"
		if !isNotConfiguredError(err.Error()) {
			severity = "warning"
			alertType = "token_error"
		}

		otm.sendAlert(OAuthTokenAlert{
			Type:      alertType,
			Provider:  "qwen",
			Message:   "Failed to read Qwen OAuth credentials: " + err.Error(),
			Timestamp: time.Now(),
			Severity:  severity,
		})
	} else if creds == nil {
		status.Valid = false
		status.Error = "Credentials is nil"
		otmTokenValidGauge.WithLabelValues("qwen").Set(0)
	} else if oauth_credentials.IsExpired(creds.ExpiryDate) {
		expiresAt := time.UnixMilli(creds.ExpiryDate)
		status.Valid = false
		status.ExpiresAt = expiresAt
		status.Error = "Token expired"
		otmTokenValidGauge.WithLabelValues("qwen").Set(0)
		otmTokenExpiryGauge.WithLabelValues("qwen").Set(0)
		otm.sendAlert(OAuthTokenAlert{
			Type:      "token_expired",
			Provider:  "qwen",
			Message:   "Qwen OAuth token has expired",
			ExpiresAt: expiresAt,
			Timestamp: time.Now(),
			Severity:  "expired",
		})
	} else {
		expiresAt := time.UnixMilli(creds.ExpiryDate)
		status.Valid = true
		status.ExpiresAt = expiresAt
		expiresIn := time.Until(expiresAt)
		status.ExpiresIn = expiresIn.Round(time.Second).String()

		otmTokenValidGauge.WithLabelValues("qwen").Set(1)
		otmTokenExpiryGauge.WithLabelValues("qwen").Set(expiresIn.Seconds())

		// Check if expiring soon
		if expiresIn <= otm.expiryThreshold {
			severity := "warning"
			if expiresIn <= 2*time.Minute {
				severity = "critical"
			}
			otm.sendAlert(OAuthTokenAlert{
				Type:      "token_expiring_soon",
				Provider:  "qwen",
				Message:   "Qwen OAuth token expires in " + status.ExpiresIn,
				ExpiresAt: expiresAt,
				ExpiresIn: status.ExpiresIn,
				Timestamp: time.Now(),
				Severity:  severity,
			})
		}
	}

	otm.updateTokenStatus("qwen", status)
}

// updateTokenStatus updates the token status cache
func (otm *OAuthTokenMonitor) updateTokenStatus(provider string, status *TokenStatus) {
	otm.tokenStatus.Put(provider, status)

	otm.logger.WithFields(logrus.Fields{
		"provider":   provider,
		"valid":      status.Valid,
		"expires_at": status.ExpiresAt,
		"expires_in": status.ExpiresIn,
		"error":      status.Error,
	}).Debug("OAuth token status updated")
}

// sendAlert sends an alert to all listeners
func (otm *OAuthTokenMonitor) sendAlert(alert OAuthTokenAlert) {
	otmTokenAlertsTotal.WithLabelValues(alert.Provider, alert.Severity).Inc()

	otm.listeners.Range(func(_ int, listener OAuthTokenAlertListener) bool {
		go listener(alert)
		return true
	})

	logLevel := logrus.WarnLevel
	if alert.Severity == "critical" || alert.Severity == "expired" {
		logLevel = logrus.ErrorLevel
	} else if alert.Severity == "info" {
		logLevel = logrus.InfoLevel
	}

	otm.logger.WithFields(logrus.Fields{
		"type":       alert.Type,
		"provider":   alert.Provider,
		"severity":   alert.Severity,
		"message":    alert.Message,
		"expires_at": alert.ExpiresAt,
	}).Log(logLevel, "OAuth token alert triggered")
}

// GetStatus returns the current status of all OAuth tokens
func (otm *OAuthTokenMonitor) GetStatus() OAuthTokenStatus {
	tokens := make(map[string]*TokenStatus)
	allValid := true
	expiringCount := 0

	otm.tokenStatus.Range(func(provider string, status *TokenStatus) bool {
		tokens[provider] = status
		if !status.Valid {
			allValid = false
		}
		if status.Valid && !status.ExpiresAt.IsZero() {
			if time.Until(status.ExpiresAt) <= otm.expiryThreshold {
				expiringCount++
			}
		}
		return true
	})

	return OAuthTokenStatus{
		Healthy:       allValid && expiringCount == 0,
		AllValid:      allValid,
		ExpiringCount: expiringCount,
		Tokens:        tokens,
		CheckedAt:     time.Now(),
	}
}

// OAuthTokenStatus represents the overall OAuth token status
type OAuthTokenStatus struct {
	Healthy       bool                    `json:"healthy"`
	AllValid      bool                    `json:"all_valid"`
	ExpiringCount int                     `json:"expiring_count"`
	Tokens        map[string]*TokenStatus `json:"tokens"`
	CheckedAt     time.Time               `json:"checked_at"`
}

// RefreshToken attempts to refresh a specific OAuth token
func (otm *OAuthTokenMonitor) RefreshToken(provider string) error {
	otm.logger.WithField("provider", provider).Info("Attempting to refresh OAuth token")

	switch provider {
	case "qwen":
		_, err := oauth_credentials.AutoRefreshQwenTokenViaCLI(context.Background())
		if err != nil {
			otmTokenRefreshTotal.WithLabelValues(provider, "failed").Inc()
			return err
		}
		otmTokenRefreshTotal.WithLabelValues(provider, "success").Inc()
		otm.checkQwenToken() // Re-check after refresh
		return nil
	case "claude":
		// Claude refresh requires user interaction via 'claude auth login'
		otmTokenRefreshTotal.WithLabelValues(provider, "manual_required").Inc()
		otm.logger.Warn("Claude token refresh requires manual login: run 'claude auth login'")
		return nil
	default:
		return nil
	}
}
