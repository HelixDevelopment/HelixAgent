package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// ConcurrencyAlertManagerConfig configures the alert manager
type ConcurrencyAlertManagerConfig struct {
	Enabled         bool          `json:"enabled"`
	DefaultCooldown time.Duration `json:"default_cooldown"` // Default cooldown between alerts
	CleanupInterval time.Duration `json:"cleanup_interval"` // How often to clean old records
	MaxAlertAge     time.Duration `json:"max_alert_age"`    // Maximum age to keep alert records

	// Alert channels
	EnableLogging  bool          `json:"enable_logging"` // Log alerts to logger
	EnableWebhook  bool          `json:"enable_webhook"` // Send alerts via webhook
	WebhookURL     string        `json:"webhook_url"`    // Webhook endpoint
	WebhookTimeout time.Duration `json:"webhook_timeout"`

	// Slack webhook (specific)
	SlackWebhookURL string `json:"slack_webhook_url"` // Slack incoming webhook URL
	SlackChannel    string `json:"slack_channel"`     // Optional channel override

	// Email (future)
	EnableEmail     bool     `json:"enable_email"`
	SMTPHost        string   `json:"smtp_host"`
	SMTPPort        int      `json:"smtp_port"`
	SMTPUsername    string   `json:"smtp_username"`
	SMTPPassword    string   `json:"smtp_password"`
	SMTPFrom        string   `json:"smtp_from"`    // From address (defaults to SMTPUsername if not set)
	SMTPUseTLS      bool     `json:"smtp_use_tls"` // Use TLS for SMTP connection
	EmailRecipients []string `json:"email_recipients"`

	// Per-provider thresholds (optional, overrides monitor threshold)
	ProviderThresholds map[string]float64 `json:"provider_thresholds"` // provider -> saturation threshold

	// Escalation policies
	EscalationEnabled        bool             `json:"escalation_enabled"`         // Enable escalation policies
	EscalationWindow         time.Duration    `json:"escalation_window"`          // Time window for counting repeats (e.g., 1h)
	EscalationThresholds     []int            `json:"escalation_thresholds"`      // Repeat counts for each escalation level
	EscalationChannelRouting map[int][]string `json:"escalation_channel_routing"` // escalation level -> list of channels
	MaxEscalationLevel       int              `json:"max_escalation_level"`       // Maximum escalation level (0-based)

	// Rate limiting (per channel)
	RateLimitEnabled   bool          `json:"rate_limit_enabled"`    // Enable rate limiting
	RateLimitWindow    time.Duration `json:"rate_limit_window"`     // Time window for rate limiting (e.g., 1 minute)
	RateLimitMaxAlerts int           `json:"rate_limit_max_alerts"` // Maximum alerts per window per channel
	RateLimitBurstSize int           `json:"rate_limit_burst_size"` // Burst size for rate limiting

	// Circuit breaker settings
	CircuitBreakerEnabled   bool          `json:"circuit_breaker_enabled"`   // Enable circuit breaker
	CircuitBreakerFailures  int           `json:"circuit_breaker_failures"`  // Consecutive failures before opening circuit
	CircuitBreakerReset     time.Duration `json:"circuit_breaker_reset"`     // Time to wait before resetting circuit (half-open)
	CircuitBreakerSuccesses int           `json:"circuit_breaker_successes"` // Consecutive successes to close circuit

	// Retry policies
	RetryEnabled           bool          `json:"retry_enabled"`            // Enable retries for failed deliveries
	MaxRetries             int           `json:"max_retries"`              // Maximum retry attempts
	RetryInitialDelay      time.Duration `json:"retry_initial_delay"`      // Initial delay before first retry
	RetryMaxDelay          time.Duration `json:"retry_max_delay"`          // Maximum delay between retries
	RetryBackoffMultiplier float64       `json:"retry_backoff_multiplier"` // Multiplier for exponential backoff

	// Dead letter queue monitoring
	DeadLetterQueueMonitoringEnabled bool `json:"dead_letter_queue_monitoring_enabled"` // Enable dead letter queue monitoring
	DeadLetterQueueWarningThreshold  int  `json:"dead_letter_queue_warning_threshold"`  // Warning when queue size >= this value
	DeadLetterQueueCriticalThreshold int  `json:"dead_letter_queue_critical_threshold"` // Critical when queue size >= this value
}

// DefaultConcurrencyAlertManagerConfig returns default configuration
func DefaultConcurrencyAlertManagerConfig() ConcurrencyAlertManagerConfig {
	return ConcurrencyAlertManagerConfig{
		Enabled:         true,
		DefaultCooldown: 30 * time.Second,
		CleanupInterval: 5 * time.Minute,
		MaxAlertAge:     24 * time.Hour,

		EnableLogging:  true,
		EnableWebhook:  false,
		WebhookTimeout: 10 * time.Second,

		EnableEmail: false,
		SMTPPort:    587,  // Default to TLS port
		SMTPUseTLS:  true, // Use TLS by default for security

		EscalationEnabled:    true,
		EscalationWindow:     1 * time.Hour,
		EscalationThresholds: []int{1, 3, 5}, // Level 0: 1st alert, Level 1: after 3 repeats, Level 2: after 5 repeats
		MaxEscalationLevel:   3,
		EscalationChannelRouting: map[int][]string{
			0: {"logging"},
			1: {"logging", "email"},
			2: {"logging", "email", "slack"},
			3: {"logging", "email", "slack", "webhook"},
		},

		// Rate limiting defaults
		RateLimitEnabled:   true,
		RateLimitWindow:    1 * time.Minute,
		RateLimitMaxAlerts: 10,
		RateLimitBurstSize: 5,

		// Circuit breaker defaults
		CircuitBreakerEnabled:   true,
		CircuitBreakerFailures:  5,
		CircuitBreakerReset:     30 * time.Second,
		CircuitBreakerSuccesses: 3,

		// Retry defaults
		RetryEnabled:           true,
		MaxRetries:             3,
		RetryInitialDelay:      1 * time.Second,
		RetryMaxDelay:          30 * time.Second,
		RetryBackoffMultiplier: 2.0,

		// Dead letter queue monitoring defaults
		DeadLetterQueueMonitoringEnabled: true,
		DeadLetterQueueWarningThreshold:  10,
		DeadLetterQueueCriticalThreshold: 50,
	}
}

// alertTrackingInfo tracks information about alerts for escalation and deduplication
// CONCURRENCY FIX: Added mutex for thread-safe access to tracking fields
type alertTrackingInfo struct {
	mu              sync.RWMutex
	lastSentTime    time.Time
	repeatCount     int
	escalationLevel int
	firstSeenTime   time.Time
}

// rateLimitTracker tracks rate limiting state for a channel
type rateLimitTracker struct {
	count        int       // Number of alerts in current window
	windowStart  time.Time // Start of current window
	resetPending bool      // Whether a reset is pending
}

// circuitBreakerState tracks circuit breaker state for a channel
type circuitBreakerState struct {
	mu              sync.RWMutex
	state           string    // "closed", "open", "half-open"
	failures        int       // Consecutive failures
	successes       int       // Consecutive successes (for half-open)
	lastFailure     time.Time // Time of last failure
	lastStateChange time.Time // Time of last state change
}

// deliveryAttempt tracks a single delivery attempt with retry logic
type deliveryAttempt struct {
	channel     string
	alert       ConcurrencyAlert
	attempts    int
	lastAttempt time.Time
	nextRetry   time.Time
}

// deadLetterAlert represents an alert that has exceeded maximum retry attempts
type deadLetterAlert struct {
	channel      string
	alert        ConcurrencyAlert
	attempts     int
	lastAttempt  time.Time
	failureError string
	addedAt      time.Time
}

// calculateEscalationLevel determines the escalation level based on repeat count and configuration
func (am *ConcurrencyAlertManager) calculateEscalationLevel(repeatCount int) int {
	if !am.config.EscalationEnabled {
		return 0
	}

	// Find the highest threshold that repeatCount meets or exceeds
	level := 0
	for i, threshold := range am.config.EscalationThresholds {
		if repeatCount >= threshold {
			level = i + 1 // Level 1 corresponds to first threshold
		} else {
			break
		}
	}

	// Cap at max escalation level
	if level > am.config.MaxEscalationLevel {
		level = am.config.MaxEscalationLevel
	}
	return level
}

// checkRateLimit checks if a channel is allowed to send an alert based on rate limiting configuration
func (am *ConcurrencyAlertManager) checkRateLimit(channel string) bool {
	if !am.config.RateLimitEnabled {
		return true
	}

	allowed := true
	// Atomic read-modify-write through safe.Store.Update avoids the classic
	// get-then-put race when two goroutines race to increment the counter.
	am.rateLimitTrackers.Update(channel, func(tracker *rateLimitTracker, present bool) (*rateLimitTracker, bool) {
		if !present {
			tracker = &rateLimitTracker{
				count:       0,
				windowStart: time.Now(),
			}
		}

		now := time.Now()
		windowStart := now.Add(-am.config.RateLimitWindow)

		// Reset counter if window has passed
		if tracker.windowStart.Before(windowStart) {
			tracker.count = 0
			tracker.windowStart = now
		}

		// Check if we've reached the limit
		if tracker.count >= am.config.RateLimitMaxAlerts {
			// Check burst allowance
			if tracker.count >= am.config.RateLimitMaxAlerts+am.config.RateLimitBurstSize {
				// Record rate limit hit
				RecordRateLimitHit(channel)
				allowed = false
				return tracker, true
			}
			// Allow burst but mark as pending reset
			tracker.resetPending = true
		} else {
			tracker.resetPending = false
		}

		// Increment counter
		tracker.count++
		return tracker, true
	})
	return allowed
}

// recordRateLimitUsage records that an alert was sent (already accounted for in checkRateLimit, but separate for clarity)
func (am *ConcurrencyAlertManager) recordRateLimitUsage(channel string) {
	// Already handled in checkRateLimit by incrementing counter
	// This method exists for symmetry with other systems
}

// checkCircuitBreaker checks if a channel's circuit breaker allows sending
func (am *ConcurrencyAlertManager) checkCircuitBreaker(channel string) bool {
	if !am.config.CircuitBreakerEnabled {
		return true
	}

	// Atomic insert-if-absent so two racing callers see the same pointer.
	newCB := &circuitBreakerState{
		state:           "closed",
		failures:        0,
		successes:       0,
		lastFailure:     time.Time{},
		lastStateChange: time.Now(),
	}
	cb, stored := am.circuitBreakerStates.PutIfAbsent(channel, newCB)
	if stored {
		// First insertion — publish the initial closed state metric.
		UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("closed"))
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check current state
	switch cb.state {
	case "closed":
		return true
	case "open":
		// Check if reset time has passed
		if time.Since(cb.lastStateChange) >= am.config.CircuitBreakerReset {
			// Transition to half-open
			cb.state = "half-open"
			cb.lastStateChange = time.Now()
			cb.successes = 0
			// Update Prometheus metric
			UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("half-open"))
			return true // Allow one request to test
		}
		return false
	case "half-open":
		return true // Allow requests in half-open state
	default:
		return true
	}
}

// recordCircuitBreakerSuccess records a successful delivery for circuit breaker
func (am *ConcurrencyAlertManager) recordCircuitBreakerSuccess(channel string) {
	if !am.config.CircuitBreakerEnabled {
		return
	}

	cb, exists := am.circuitBreakerStates.Get(channel)
	if !exists {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "closed":
		// Reset consecutive failures on success
		cb.failures = 0
	case "half-open":
		cb.successes++
		if cb.successes >= am.config.CircuitBreakerSuccesses {
			// Transition back to closed
			cb.state = "closed"
			cb.lastStateChange = time.Now()
			cb.failures = 0
			cb.successes = 0
			// Update Prometheus metric
			UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("closed"))
		}
	case "open":
		// Should not happen, but if it does, transition to half-open
		cb.state = "half-open"
		cb.lastStateChange = time.Now()
		cb.successes = 1
		// Update Prometheus metric
		UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("half-open"))
	}
}

// recordCircuitBreakerFailure records a failed delivery for circuit breaker
func (am *ConcurrencyAlertManager) recordCircuitBreakerFailure(channel string) {
	if !am.config.CircuitBreakerEnabled {
		return
	}

	newCB := &circuitBreakerState{
		state:           "closed",
		failures:        0,
		successes:       0,
		lastFailure:     time.Time{},
		lastStateChange: time.Now(),
	}
	cb, stored := am.circuitBreakerStates.PutIfAbsent(channel, newCB)
	if stored {
		// First insertion — publish the initial closed state metric.
		UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("closed"))
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Record failure
	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case "closed":
		if cb.failures >= am.config.CircuitBreakerFailures {
			// Open the circuit
			cb.state = "open"
			cb.lastStateChange = time.Now()
			// Update Prometheus metric
			UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("open"))
		}
	case "half-open":
		// Single failure in half-open state opens circuit
		cb.state = "open"
		cb.lastStateChange = time.Now()
		// Update Prometheus metric
		UpdateCircuitBreakerState(channel, circuitBreakerStateToInt("open"))
	case "open":
		// Already open, do nothing
	}
}

// shouldSendToChannel checks if a channel should be used for given escalation level
func (am *ConcurrencyAlertManager) shouldSendToChannel(channel string, escalationLevel int) bool {
	// First check if channel is globally enabled and configured
	switch channel {
	case "logging":
		if !am.config.EnableLogging {
			return false
		}
	case "webhook":
		if !am.config.EnableWebhook || am.config.WebhookURL == "" {
			return false
		}
	case "slack":
		if am.config.SlackWebhookURL == "" {
			return false
		}
	case "email":
		if !am.config.EnableEmail || len(am.config.EmailRecipients) == 0 {
			return false
		}
	default:
		return false
	}

	if !am.config.EscalationEnabled {
		// If escalation disabled, channel is enabled (already verified above)
		return true
	}

	// Check if channel is in routing for this escalation level
	channels, ok := am.config.EscalationChannelRouting[escalationLevel]
	if !ok {
		// No routing defined for this level, use default (level 0)
		channels, ok = am.config.EscalationChannelRouting[0]
		if !ok {
			// No routing for level 0 either, allow all enabled channels
			return true
		}
	}

	// Check if channel is in the list
	for _, ch := range channels {
		if ch == channel {
			return true
		}
	}
	return false
}

// getOrCreateTracking gets tracking info for an alert key, creating if needed
// Returns repeat count and escalation level to avoid needing to read from tracking after release
func (am *ConcurrencyAlertManager) getOrCreateTracking(alertKey string) (repeatCount int, escalationLevel int) {
	// Atomic insert-if-absent: two racing callers both observe the same
	// *alertTrackingInfo and then synchronise on its inner mutex below.
	now := time.Now()
	fresh := &alertTrackingInfo{
		lastSentTime:    now,
		repeatCount:     1,
		escalationLevel: 0,
		firstSeenTime:   now,
	}
	// PutIfAbsent returns (value, stored): stored == true when we just
	// inserted `fresh` (first time for this key); stored == false when an
	// existing entry was already present and `value` is that existing one.
	tracking, stored := am.alertTracking.PutIfAbsent(alertKey, fresh)
	if stored {
		return 1, 0
	}

	// CONCURRENCY FIX: Lock tracking for safe access
	tracking.mu.Lock()
	defer tracking.mu.Unlock()

	// Update repeat count if within escalation window
	windowStart := time.Now().Add(-am.config.EscalationWindow)
	if tracking.firstSeenTime.Before(windowStart) {
		// Reset tracking if first seen outside escalation window
		reset := time.Now()
		tracking.repeatCount = 1
		tracking.escalationLevel = 0
		tracking.firstSeenTime = reset
		tracking.lastSentTime = reset
	} else {
		tracking.repeatCount++
	}

	return tracking.repeatCount, tracking.escalationLevel
}

// updateTrackingEscalationLevel updates the escalation level for an alert key thread-safely
func (am *ConcurrencyAlertManager) updateTrackingEscalationLevel(alertKey string, level int) {
	tracking, exists := am.alertTracking.Get(alertKey)
	if !exists {
		return
	}

	tracking.mu.Lock()
	tracking.escalationLevel = level
	tracking.mu.Unlock()
}

// ConcurrencyAlertManager handles sending concurrency alerts with deduplication and cooldown.
//
// Concurrent-safe by construction (CONST-029): the six collections below are
// each a safe.Store, so no outer sync.Mutex is required to coordinate their
// reads and writes. Per-entry pointer fields (e.g. alertTrackingInfo,
// circuitBreakerState) continue to carry their own sync.RWMutex — the
// safe.Store guarantee is about map integrity only, not inner mutation of
// pointer values.
//
// Lifecycle state (running / stopCh) is coordinated separately by stateMu —
// Start/Stop must observe and swap both fields under the same lock.
type ConcurrencyAlertManager struct {
	config     ConcurrencyAlertManagerConfig
	logger     *logrus.Logger
	httpClient *http.Client

	// Alert tracking for deduplication, cooldown, and escalation
	alertTracking *safe.Store[string, *alertTrackingInfo] // alertKey -> tracking info

	// Per-provider thresholds (overrides). Populated at construction from
	// config; only read thereafter, so safe.Store suffices.
	providerThresholds *safe.Store[string, float64]

	// Rate limiting trackers per channel
	rateLimitTrackers *safe.Store[string, *rateLimitTracker] // channel -> tracker

	// Circuit breaker states per channel
	circuitBreakerStates *safe.Store[string, *circuitBreakerState] // channel -> state

	// Delivery attempts for retry logic
	deliveryAttempts *safe.Store[string, *deliveryAttempt] // attempt key -> attempt

	// Dead letter queue for permanently failed alerts
	deadLetterAlerts *safe.Store[string, *deadLetterAlert] // channel:alertKey -> dead letter alert

	// Lifecycle: running is an atomic bool; stopCh is swapped under stateMu.
	stateMu sync.Mutex
	stopCh  chan struct{}
	running atomic.Bool
}

// NewConcurrencyAlertManager creates a new alert manager
func NewConcurrencyAlertManager(config ConcurrencyAlertManagerConfig, logger *logrus.Logger) *ConcurrencyAlertManager {
	if logger == nil {
		logger = logrus.New()
	}

	providerThresholds := safe.NewStore[string, float64]()
	for k, v := range config.ProviderThresholds {
		providerThresholds.Put(k, v)
	}

	return &ConcurrencyAlertManager{
		config:               config,
		logger:               logger,
		httpClient:           &http.Client{Timeout: config.WebhookTimeout},
		alertTracking:        safe.NewStore[string, *alertTrackingInfo](),
		providerThresholds:   providerThresholds,
		rateLimitTrackers:    safe.NewStore[string, *rateLimitTracker](),
		circuitBreakerStates: safe.NewStore[string, *circuitBreakerState](),
		deliveryAttempts:     safe.NewStore[string, *deliveryAttempt](),
		deadLetterAlerts:     safe.NewStore[string, *deadLetterAlert](),
		stopCh:               make(chan struct{}),
	}
}

// HandleAlert implements ConcurrencyAlertListener
func (am *ConcurrencyAlertManager) HandleAlert(alert ConcurrencyAlert) {
	if !am.config.Enabled {
		return
	}

	// Record alert handled (including those filtered by cooldown)
	RecordAlertHandled(alert.Type, alert.Provider, severityToString(alert.Severity))

	// Determine alert key for deduplication
	alertKey := am.generateAlertKey(alert)

	// Get or create tracking info (updates repeat count if within escalation window)
	// CONCURRENCY FIX: getOrCreateTracking now returns values to avoid race conditions
	repeatCount, _ := am.getOrCreateTracking(alertKey)

	// Calculate escalation level based on repeat count
	escalationLevel := am.calculateEscalationLevel(repeatCount)
	// Update tracking's escalation level thread-safely
	am.updateTrackingEscalationLevel(alertKey, escalationLevel)
	// Update Prometheus metric for escalation level
	UpdateEscalationLevel(alert.Type, alert.Provider, alertKey, escalationLevel)

	// Update alert with escalation level
	alert.EscalationLevel = escalationLevel

	// Check cooldown
	if am.isInCooldown(alertKey) {
		am.logger.Debug("Alert in cooldown period", logrus.Fields{
			"key":          alertKey,
			"type":         alert.Type,
			"provider":     alert.Provider,
			"escalation":   escalationLevel,
			"repeat_count": repeatCount,
		})
		return
	}

	am.logger.Info("Processing concurrency alert", logrus.Fields{
		"type":         alert.Type,
		"provider":     alert.Provider,
		"saturation":   alert.Saturation,
		"escalation":   escalationLevel,
		"repeat_count": repeatCount,
	})

	// Send to channels based on escalation level with resilience
	if am.shouldSendToChannel("logging", escalationLevel) {
		// Logging is synchronous but we still apply rate limiting and circuit breaker
		am.sendToChannelWithResilience("logging", alert, func() error {
			return am.sendToLog(alert)
		})
	}

	if am.shouldSendToChannel("webhook", escalationLevel) {
		// Webhook is async - run in goroutine
		go am.sendToChannelWithResilience("webhook", alert, func() error {
			return am.sendWebhook(alert)
		})
	}

	if am.shouldSendToChannel("slack", escalationLevel) {
		// Slack is async - run in goroutine
		go am.sendToChannelWithResilience("slack", alert, func() error {
			return am.sendSlackWebhook(alert)
		})
	}

	if am.shouldSendToChannel("email", escalationLevel) {
		// Email is async - run in goroutine
		go am.sendToChannelWithResilience("email", alert, func() error {
			return am.sendEmail(alert)
		})
	}

	// Record alert sent time (updates lastSentTime)
	am.recordAlertSent(alertKey)
}

// Start starts the cleanup background goroutine
func (am *ConcurrencyAlertManager) Start(ctx context.Context) {
	am.stateMu.Lock()
	if am.running.Load() {
		am.stateMu.Unlock()
		return
	}
	am.running.Store(true)
	am.stopCh = make(chan struct{})
	stopCh := am.stopCh
	am.stateMu.Unlock()

	am.logger.Info("Concurrency alert manager started")

	// Start cleanup ticker
	ticker := time.NewTicker(am.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			am.logger.Info("Concurrency alert manager stopped (context cancelled)")
			return
		case <-stopCh:
			am.logger.Info("Concurrency alert manager stopped")
			return
		case <-ticker.C:
			am.CleanupOldRecords(am.config.MaxAlertAge)
			am.processRetries()
			am.checkDeadLetterQueueThresholds()
		}
	}
}

// Stop stops the alert manager
func (am *ConcurrencyAlertManager) Stop() {
	am.stateMu.Lock()
	defer am.stateMu.Unlock()

	if am.running.Load() {
		close(am.stopCh)
		am.running.Store(false)
	}
}

// generateAlertKey creates a unique key for deduplication
func (am *ConcurrencyAlertManager) generateAlertKey(alert ConcurrencyAlert) string {
	// For provider-specific alerts, include provider
	if alert.Provider != "" {
		return fmt.Sprintf("concurrency_%s_%s_%.0f", alert.Type, alert.Provider, alert.Saturation)
	}
	// For summary alerts (multiple providers), use type and saturation threshold
	return fmt.Sprintf("concurrency_%s_%.0f", alert.Type, alert.Saturation)
}

// isInCooldown checks if an alert is in cooldown period
func (am *ConcurrencyAlertManager) isInCooldown(alertKey string) bool {
	tracking, exists := am.alertTracking.Get(alertKey)
	if !exists {
		return false
	}

	// CONCURRENCY FIX: Lock tracking for safe access to lastSentTime
	tracking.mu.RLock()
	lastSentTime := tracking.lastSentTime
	tracking.mu.RUnlock()

	return time.Since(lastSentTime) < am.config.DefaultCooldown
}

// recordAlertSent updates the last sent time for an alert
func (am *ConcurrencyAlertManager) recordAlertSent(alertKey string) {
	// PutIfAbsent handles the "tracking missing" edge case atomically; if
	// it was already present, we fall through and update its inner mutex.
	now := time.Now()
	fresh := &alertTrackingInfo{
		lastSentTime:    now,
		repeatCount:     1,
		escalationLevel: 0,
		firstSeenTime:   now,
	}
	tracking, stored := am.alertTracking.PutIfAbsent(alertKey, fresh)
	if stored {
		// We just inserted the fresh tracking with lastSentTime==now.
		return
	}

	// CONCURRENCY FIX: Lock tracking for safe access to lastSentTime
	tracking.mu.Lock()
	tracking.lastSentTime = time.Now()
	tracking.mu.Unlock()
}

// CleanupOldRecords removes old alert records.
//
// cleanupOldEntries walks three independent safe.Stores. A
// concurrent insert between pass 1 and pass 2 is acceptable
// because cleanup is advisory, not a correctness invariant —
// the entry will simply be inspected in the next cleanup cycle.
func (am *ConcurrencyAlertManager) CleanupOldRecords(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	alertCount := 0
	attemptCount := 0
	deadLetterCount := 0

	// Pass 1: alert tracking. Snapshot first so per-entry mutex acquisition
	// below does not contend with the Store's read lock.
	for key, tracking := range am.alertTracking.Snapshot() {
		tracking.mu.RLock()
		lastSent := tracking.lastSentTime
		tracking.mu.RUnlock()
		if lastSent.Before(cutoff) {
			if _, existed := am.alertTracking.Delete(key); existed {
				alertCount++
			}
		}
	}

	// Pass 2: delivery attempts (should be removed by processRetries, but
	// re-check here as a defensive sweep).
	for key, attempt := range am.deliveryAttempts.Snapshot() {
		if attempt.lastAttempt.Before(cutoff) {
			if _, existed := am.deliveryAttempts.Delete(key); existed {
				attemptCount++
			}
		}
	}

	// Pass 3: dead letter alerts.
	for key, deadLetter := range am.deadLetterAlerts.Snapshot() {
		if deadLetter.addedAt.Before(cutoff) {
			if _, existed := am.deadLetterAlerts.Delete(key); existed {
				deadLetterCount++
			}
		}
	}

	if alertCount > 0 || attemptCount > 0 || deadLetterCount > 0 {
		am.logger.Debug("Cleaned up old alert records", logrus.Fields{
			"alert_count":       alertCount,
			"attempt_count":     attemptCount,
			"dead_letter_count": deadLetterCount,
			"max_age":           maxAge,
		})
		// Update metrics for changed queues
		if attemptCount > 0 {
			am.updateRetryQueueMetrics()
		}
		if deadLetterCount > 0 {
			am.updateDeadLetterQueueMetrics()
		}
	}
}

// GetAlertStats returns statistics about sent alerts.
//
// Independent snapshot passes: acceptable for advisory stats even if a
// concurrent insert lands between passes (same visibility contract as
// CleanupOldRecords).
func (am *ConcurrencyAlertManager) GetAlertStats() map[string]interface{} {
	alertSnap := am.alertTracking.Snapshot()
	attemptsSnap := am.deliveryAttempts.Snapshot()
	deadLetterSnap := am.deadLetterAlerts.Snapshot()

	stats := map[string]interface{}{
		"total_alerts":           len(alertSnap),
		"providers":              make(map[string]int),
		"types":                  make(map[string]int),
		"retry_queue_size":       len(attemptsSnap),
		"dead_letter_queue_size": len(deadLetterSnap),
		"channels":               make(map[string]map[string]int),
	}

	// Count alerts by provider and type
	for key := range alertSnap {
		// Simple parsing - could be enhanced
		if len(key) > 0 {
			// key format: concurrency_type_provider_saturation
			parts := splitAlertKey(key)
			if len(parts) >= 3 {
				provider := parts[2]
				if provider != "" && provider != "summary" {
					stats["providers"].(map[string]int)[provider]++ //nolint:errcheck
				}
				stats["types"].(map[string]int)[parts[1]]++ //nolint:errcheck
			}
		}
	}

	// Count retry queue by channel
	retryByChannel := make(map[string]int)
	for _, attempt := range attemptsSnap {
		retryByChannel[attempt.channel]++
	}
	stats["retry_queue_by_channel"] = retryByChannel

	// Count dead letter queue by channel
	deadLetterByChannel := make(map[string]int)
	for _, deadLetter := range deadLetterSnap {
		deadLetterByChannel[deadLetter.channel]++
	}
	stats["dead_letter_queue_by_channel"] = deadLetterByChannel

	return stats
}
func (am *ConcurrencyAlertManager) calculateRetryDelay(attempts int) time.Duration {
	if attempts <= 0 {
		return am.config.RetryInitialDelay
	}

	// Calculate exponential backoff: initialDelay * multiplier^attempts
	delay := float64(am.config.RetryInitialDelay) * math.Pow(am.config.RetryBackoffMultiplier, float64(attempts-1))

	// Apply jitter: ±20% random variation
	jitter := 0.8 + 0.4*rand.Float64() // #nosec G404 -- jitter for backoff timing is not security-sensitive
	delay = delay * jitter

	// Convert to time.Duration and cap at max delay
	result := time.Duration(delay)
	if result > am.config.RetryMaxDelay {
		result = am.config.RetryMaxDelay
	}
	if result < am.config.RetryInitialDelay {
		result = am.config.RetryInitialDelay
	}
	return result
}

// scheduleRetry schedules a retry for a failed delivery
func (am *ConcurrencyAlertManager) scheduleRetry(channel string, alert ConcurrencyAlert, sendFunc func() error, attempts int, lastError error) {
	if !am.config.RetryEnabled {
		return
	}
	if attempts >= am.config.MaxRetries {
		am.logger.Debug("Max retries reached for alert", logrus.Fields{
			"channel":  channel,
			"type":     alert.Type,
			"provider": alert.Provider,
			"attempts": attempts,
		})
		// Move to dead letter queue
		failureError := "max retries exceeded"
		if lastError != nil {
			failureError = fmt.Sprintf("max retries exceeded: %v", lastError)
		}
		am.addToDeadLetterQueue(channel, alert, attempts, failureError)
		return
	}

	alertKey := am.generateAlertKey(alert)
	retryKey := fmt.Sprintf("%s:%s", channel, alertKey)

	// Atomic upsert — Update either inserts a new attempt or overwrites
	// the existing one under a single write lock.
	var nextRetry time.Time
	am.deliveryAttempts.Update(retryKey, func(cur *deliveryAttempt, present bool) (*deliveryAttempt, bool) {
		now := time.Now()
		nr := now.Add(am.calculateRetryDelay(attempts))
		if !present {
			cur = &deliveryAttempt{
				channel:     channel,
				alert:       alert,
				attempts:    attempts,
				lastAttempt: now,
				nextRetry:   nr,
			}
		} else {
			cur.attempts = attempts
			cur.lastAttempt = now
			cur.nextRetry = nr
		}
		nextRetry = cur.nextRetry
		return cur, true
	})

	am.logger.Debug("Scheduled retry for alert", logrus.Fields{
		"channel":    channel,
		"type":       alert.Type,
		"provider":   alert.Provider,
		"attempts":   attempts,
		"next_retry": nextRetry,
	})
	am.updateRetryQueueMetrics()
}

// processRetries processes pending retry attempts
func (am *ConcurrencyAlertManager) processRetries() {
	if !am.config.RetryEnabled {
		return
	}

	now := time.Now()
	// Snapshot first: the Delete below would deadlock inside Range.
	changed := false
	for key, attempt := range am.deliveryAttempts.Snapshot() {
		if attempt.nextRetry.After(now) {
			continue
		}
		// Ready for retry. Copy values to avoid data race.
		channel := attempt.channel
		alert := attempt.alert
		attempts := attempt.attempts
		// Remove from store before attempting (will be re-added if fails).
		// Delete returns false if another goroutine got there first — safe to skip.
		if _, existed := am.deliveryAttempts.Delete(key); !existed {
			continue
		}
		changed = true
		// Execute retry asynchronously
		go am.retryDelivery(channel, alert, attempts)
	}
	if changed {
		am.updateRetryQueueMetrics()
	}
}

// retryDelivery attempts a retry delivery (called from processRetries)
func (am *ConcurrencyAlertManager) retryDelivery(channel string, alert ConcurrencyAlert, attempts int) {
	// Determine sendFunc based on channel
	var sendFunc func() error
	switch channel {
	case "logging":
		sendFunc = func() error { return am.sendToLog(alert) }
	case "webhook":
		sendFunc = func() error { return am.sendWebhook(alert) }
	case "slack":
		sendFunc = func() error { return am.sendSlackWebhook(alert) }
	case "email":
		sendFunc = func() error { return am.sendEmail(alert) }
	default:
		am.logger.Error("Unknown channel for retry", logrus.Fields{"channel": channel})
		return
	}

	// Use the resilience wrapper with current attempt count
	am.sendToChannelWithResilienceInternal(channel, alert, sendFunc, attempts)
}

// sendToChannelWithResilienceInternal sends an alert to a channel with rate limiting, circuit breaker, and retry support
func (am *ConcurrencyAlertManager) sendToChannelWithResilienceInternal(channel string, alert ConcurrencyAlert, sendFunc func() error, attempts int) {

	am.logger.Debug("Sending alert via channel", logrus.Fields{
		"channel":  channel,
		"type":     alert.Type,
		"provider": alert.Provider,
		"attempts": attempts,
	})
	// Record metrics
	RecordAlertDelivery(channel, alert.Provider, alert.Type)
	if attempts > 0 {
		RecordRetryAttempt(channel, alert.Provider, alert.Type)
	}
	// Check rate limiting
	if !am.checkRateLimit(channel) {
		am.logger.Debug("Rate limit exceeded for channel", logrus.Fields{
			"channel":  channel,
			"type":     alert.Type,
			"provider": alert.Provider,
		})
		return
	}

	// Check circuit breaker
	if !am.checkCircuitBreaker(channel) {
		am.logger.Debug("Circuit breaker open for channel", logrus.Fields{
			"channel":  channel,
			"type":     alert.Type,
			"provider": alert.Provider,
		})
		return
	}

	// Attempt delivery
	err := sendFunc()
	if err != nil {
		RecordAlertDeliveryError(channel, alert.Provider, alert.Type)
		am.logger.Error("Failed to send alert via channel", logrus.Fields{
			"channel":  channel,
			"type":     alert.Type,
			"provider": alert.Provider,
			"error":    err.Error(),
		})
		am.recordCircuitBreakerFailure(channel)
		// Schedule retry if enabled
		if am.config.RetryEnabled && attempts < am.config.MaxRetries {
			am.scheduleRetry(channel, alert, sendFunc, attempts+1, err)
		}
	} else {
		am.recordCircuitBreakerSuccess(channel)
		am.recordRateLimitUsage(channel)
		// Record successful retry if this was a retry attempt
		if attempts > 0 {
			RecordRetrySuccess(channel, alert.Provider, alert.Type)
		}
		// Remove any pending retry for this alert
		am.removeDeliveryAttempt(channel, alert)
	}
}

// sendToChannelWithResilience sends an alert to a channel with rate limiting and circuit breaker protection
func (am *ConcurrencyAlertManager) sendToChannelWithResilience(channel string, alert ConcurrencyAlert, sendFunc func() error) {
	am.sendToChannelWithResilienceInternal(channel, alert, sendFunc, 0)
}

// removeDeliveryAttempt removes a delivery attempt from the retry queue (e.g., on success)
func (am *ConcurrencyAlertManager) removeDeliveryAttempt(channel string, alert ConcurrencyAlert) {
	alertKey := am.generateAlertKey(alert)
	retryKey := fmt.Sprintf("%s:%s", channel, alertKey)

	if _, existed := am.deliveryAttempts.Delete(retryKey); existed {
		am.updateRetryQueueMetrics()
	}
}

// updateRetryQueueMetrics updates Prometheus metrics for retry queue sizes
// per channel. Snapshots the delivery attempts store; acceptable for metric
// reporting even if a concurrent insert lands between the two passes.
func (am *ConcurrencyAlertManager) updateRetryQueueMetrics() {
	// Count delivery attempts per channel
	channelCounts := make(map[string]int)
	for _, attempt := range am.deliveryAttempts.Snapshot() {
		channelCounts[attempt.channel]++
	}

	// Update metrics for each channel
	for channel, count := range channelCounts {
		UpdateRetryQueueSize(channel, count)
	}

	// Channels with zero will naturally have no gauge update (they keep last
	// value). In a more complete implementation, we'd track all known channels.
}

// updateDeadLetterQueueMetrics updates Prometheus metrics for dead letter
// queue sizes per channel.
func (am *ConcurrencyAlertManager) updateDeadLetterQueueMetrics() {
	// Count dead letter alerts per channel
	channelCounts := make(map[string]int)
	for _, deadLetter := range am.deadLetterAlerts.Snapshot() {
		channelCounts[deadLetter.channel]++
	}

	// Update metrics for each channel
	for channel, count := range channelCounts {
		UpdateDeadLetterQueueSize(channel, count)
	}
}

// addToDeadLetterQueue adds an alert to the dead letter queue
func (am *ConcurrencyAlertManager) addToDeadLetterQueue(channel string, alert ConcurrencyAlert, attempts int, failureError string) {
	alertKey := am.generateAlertKey(alert)
	deadLetterKey := fmt.Sprintf("%s:%s", channel, alertKey)

	inserted := false
	// Atomic upsert: if already present, refresh fields; otherwise insert.
	am.deadLetterAlerts.Update(deadLetterKey, func(cur *deadLetterAlert, present bool) (*deadLetterAlert, bool) {
		now := time.Now()
		if present {
			cur.lastAttempt = now
			cur.addedAt = now
			cur.failureError = failureError
			cur.attempts = attempts
			return cur, true
		}
		inserted = true
		return &deadLetterAlert{
			channel:      channel,
			alert:        alert,
			attempts:     attempts,
			lastAttempt:  now,
			failureError: failureError,
			addedAt:      now,
		}, true
	})

	if !inserted {
		return
	}

	am.logger.Warning("Alert moved to dead letter queue", logrus.Fields{
		"channel":  channel,
		"type":     alert.Type,
		"provider": alert.Provider,
		"attempts": attempts,
		"error":    failureError,
	})

	// Update metrics
	am.updateDeadLetterQueueMetrics()
}

// GetDeadLetterAlerts returns all alerts currently in the dead letter queue
func (am *ConcurrencyAlertManager) GetDeadLetterAlerts() []map[string]interface{} {
	snap := am.deadLetterAlerts.Snapshot()
	alerts := make([]map[string]interface{}, 0, len(snap))
	for key, deadLetter := range snap {
		alert := map[string]interface{}{
			"key":           key,
			"channel":       deadLetter.channel,
			"type":          deadLetter.alert.Type,
			"provider":      deadLetter.alert.Provider,
			"message":       deadLetter.alert.Message,
			"attempts":      deadLetter.attempts,
			"last_attempt":  deadLetter.lastAttempt,
			"failure_error": deadLetter.failureError,
			"added_at":      deadLetter.addedAt,
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// RetryDeadLetterAlert attempts to retry an alert from the dead letter queue
func (am *ConcurrencyAlertManager) RetryDeadLetterAlert(key string) bool {
	// Atomic remove — only one caller wins the retry.
	deadLetter, existed := am.deadLetterAlerts.Delete(key)
	if !existed {
		return false
	}
	am.updateDeadLetterQueueMetrics()

	// Determine sendFunc based on channel
	var sendFunc func() error
	switch deadLetter.channel {
	case "logging":
		sendFunc = func() error { return am.sendToLog(deadLetter.alert) }
	case "webhook":
		sendFunc = func() error { return am.sendWebhook(deadLetter.alert) }
	case "slack":
		sendFunc = func() error { return am.sendSlackWebhook(deadLetter.alert) }
	case "email":
		sendFunc = func() error { return am.sendEmail(deadLetter.alert) }
	default:
		am.logger.Error("Unknown channel for dead letter retry", logrus.Fields{"channel": deadLetter.channel})
		return false
	}

	// Retry with zero attempts (fresh start)
	go am.sendToChannelWithResilienceInternal(deadLetter.channel, deadLetter.alert, sendFunc, 0)
	return true
}

// GetRetryQueueAlerts returns all alerts currently in the retry queue
func (am *ConcurrencyAlertManager) GetRetryQueueAlerts() []map[string]interface{} {
	snap := am.deliveryAttempts.Snapshot()
	alerts := make([]map[string]interface{}, 0, len(snap))
	for key, attempt := range snap {
		alert := map[string]interface{}{
			"key":          key,
			"channel":      attempt.channel,
			"type":         attempt.alert.Type,
			"provider":     attempt.alert.Provider,
			"message":      attempt.alert.Message,
			"attempts":     attempt.attempts,
			"last_attempt": attempt.lastAttempt,
			"next_retry":   attempt.nextRetry,
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// CancelRetryAttempt cancels a scheduled retry attempt
func (am *ConcurrencyAlertManager) CancelRetryAttempt(key string) bool {
	if _, existed := am.deliveryAttempts.Delete(key); !existed {
		return false
	}
	am.updateRetryQueueMetrics()
	return true
}

// checkDeadLetterQueueThresholds checks the dead letter queue size against configured thresholds
// and generates alerts when thresholds are exceeded
func (am *ConcurrencyAlertManager) checkDeadLetterQueueThresholds() {
	if !am.config.DeadLetterQueueMonitoringEnabled {
		return
	}

	queueSize := am.deadLetterAlerts.Len()

	// Determine which threshold(s) are crossed
	warningThreshold := am.config.DeadLetterQueueWarningThreshold
	criticalThreshold := am.config.DeadLetterQueueCriticalThreshold

	// Log monitoring activity for debugging
	am.logger.Debug("Checking dead letter queue thresholds", logrus.Fields{
		"queue_size":         queueSize,
		"warning_threshold":  warningThreshold,
		"critical_threshold": criticalThreshold,
		"monitoring_enabled": am.config.DeadLetterQueueMonitoringEnabled,
	})

	// Check critical threshold first (more severe)
	if criticalThreshold > 0 && queueSize >= criticalThreshold {
		// Generate critical alert
		// Use threshold as saturation for consistent alert key (same threshold = same key)
		alert := ConcurrencyAlert{
			Type:       "dead_letter_queue_critical",
			Message:    fmt.Sprintf("Dead letter queue size (%d) exceeds critical threshold (%d)", queueSize, criticalThreshold),
			Timestamp:  time.Now(),
			Saturation: float64(criticalThreshold),
			Severity:   SeverityCritical,
			Provider:   "system",
		}
		am.logger.Warning("Dead letter queue critical threshold exceeded", logrus.Fields{
			"queue_size":         queueSize,
			"critical_threshold": criticalThreshold,
		})
		// Record threshold breach metric only if not in cooldown
		alertKey := am.generateAlertKey(alert)
		if !am.isInCooldown(alertKey) {
			RecordThresholdBreach("critical", "dead_letter_queue", "system")
		}
		am.HandleAlert(alert)
		return // Don't send warning if critical already triggered
	}

	// Check warning threshold
	if warningThreshold > 0 && queueSize >= warningThreshold {
		// Generate warning alert
		// Use threshold as saturation for consistent alert key
		alert := ConcurrencyAlert{
			Type:       "dead_letter_queue_warning",
			Message:    fmt.Sprintf("Dead letter queue size (%d) exceeds warning threshold (%d)", queueSize, warningThreshold),
			Timestamp:  time.Now(),
			Saturation: float64(warningThreshold),
			Severity:   SeverityWarning,
			Provider:   "system",
		}
		am.logger.Warning("Dead letter queue warning threshold exceeded", logrus.Fields{
			"queue_size":        queueSize,
			"warning_threshold": warningThreshold,
		})
		// Record threshold breach metric only if not in cooldown
		alertKey := am.generateAlertKey(alert)
		if !am.isInCooldown(alertKey) {
			RecordThresholdBreach("warning", "dead_letter_queue", "system")
		}
		am.HandleAlert(alert)
	}
}

// splitAlertKey splits alert key into parts
func splitAlertKey(key string) []string {
	// Simple split by underscore
	var parts []string
	start := 0
	for i, ch := range key {
		if ch == '_' {
			parts = append(parts, key[start:i])
			start = i + 1
		}
	}
	if start < len(key) {
		parts = append(parts, key[start:])
	}
	return parts
}

// sendToLog logs the alert
func (am *ConcurrencyAlertManager) sendToLog(alert ConcurrencyAlert) error {
	fields := logrus.Fields{
		"type":             alert.Type,
		"timestamp":        alert.Timestamp,
		"saturation":       alert.Saturation,
		"severity":         alert.Severity,
		"escalation_level": alert.EscalationLevel,
	}

	if alert.Provider != "" {
		fields["provider"] = alert.Provider
		fields["active_requests"] = alert.ActiveRequests
		fields["total_permits"] = alert.TotalPermits
		fields["available"] = alert.Available
	}

	if alert.AllStats != nil {
		fields["providers_affected"] = len(alert.AllStats)
	}

	am.logger.WithFields(fields).Error(alert.Message)
	return nil
}

// sendWebhook sends alert via HTTP webhook
func (am *ConcurrencyAlertManager) sendWebhook(alert ConcurrencyAlert) error {
	payload, err := am.buildWebhookPayload(alert)
	if err != nil {
		return fmt.Errorf("failed to build webhook payload: %w", err)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", am.config.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}
	return nil
}

// sendSlackWebhook sends alert to Slack via incoming webhook
func (am *ConcurrencyAlertManager) sendSlackWebhook(alert ConcurrencyAlert) error {
	payload := am.buildSlackPayload(alert)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequest("POST", am.config.SlackWebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Slack webhook returned error status: %d", resp.StatusCode)
	}
	return nil
}

// buildWebhookPayload builds generic webhook payload
func (am *ConcurrencyAlertManager) buildWebhookPayload(alert ConcurrencyAlert) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"event":            "concurrency_alert",
		"type":             alert.Type,
		"message":          alert.Message,
		"timestamp":        alert.Timestamp.Format(time.RFC3339),
		"saturation":       alert.Saturation,
		"severity":         alert.Severity,
		"escalation_level": alert.EscalationLevel,
		"data":             alert,
	}
	return payload, nil
}

// buildSlackPayload builds Slack webhook payload
func (am *ConcurrencyAlertManager) buildSlackPayload(alert ConcurrencyAlert) map[string]interface{} {
	// Determine color based on severity
	var color string
	switch alert.Severity {
	case SeverityCritical:
		color = "danger"
	case SeverityError:
		color = "danger"
	case SeverityWarning:
		color = "warning"
	case SeverityInfo:
		color = "good"
	default:
		// Fallback to saturation-based color
		if alert.Saturation >= 95 {
			color = "danger"
		} else if alert.Saturation >= 80 {
			color = "warning"
		} else {
			color = "good"
		}
	}

	// Build fields
	fields := []map[string]interface{}{
		{
			"title": "Type",
			"value": alert.Type,
			"short": true,
		},
		{
			"title": "Saturation",
			"value": fmt.Sprintf("%.1f%%", alert.Saturation),
			"short": true,
		},
	}

	if alert.Provider != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Provider",
			"value": alert.Provider,
			"short": true,
		})
		fields = append(fields, map[string]interface{}{
			"title": "Active Requests",
			"value": fmt.Sprintf("%d", alert.ActiveRequests),
			"short": true,
		})
	}

	if alert.AllStats != nil {
		fields = append(fields, map[string]interface{}{
			"title": "Providers Affected",
			"value": fmt.Sprintf("%d", len(alert.AllStats)),
			"short": true,
		})
	}

	return map[string]interface{}{
		"channel": am.config.SlackChannel,
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  "Concurrency Alert",
				"text":   alert.Message,
				"fields": fields,
				"footer": "HelixAgent Monitoring",
				"ts":     alert.Timestamp.Unix(),
			},
		},
	}
}

// sendEmail sends alert via email
func (am *ConcurrencyAlertManager) sendEmail(alert ConcurrencyAlert) error {
	if len(am.config.EmailRecipients) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	if am.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured for email alerts")
	}

	// Determine from address
	fromAddress := am.config.SMTPFrom
	if fromAddress == "" {
		fromAddress = am.config.SMTPUsername
	}
	if fromAddress == "" {
		return fmt.Errorf("no from address configured for email alerts")
	}

	subject := am.formatEmailSubject(alert)
	body := am.formatConcurrencyEmail(alert)

	am.logger.Info("Sending email alert", logrus.Fields{
		"subject":    subject,
		"recipients": len(am.config.EmailRecipients),
		"smtp_host":  am.config.SMTPHost,
	})

	// Build email message
	message := am.buildEmailMessage(fromAddress, subject, body)

	// Send via SMTP
	if err := am.sendSMTPEmail(fromAddress, message); err != nil {
		return fmt.Errorf("failed to send email alert: %w", err)
	}

	am.logger.Info("Email alert sent successfully", logrus.Fields{
		"subject":    subject,
		"recipients": am.config.EmailRecipients,
	})
	return nil
}

func (am *ConcurrencyAlertManager) formatEmailSubject(alert ConcurrencyAlert) string {
	severity := "Unknown"
	switch alert.Severity {
	case SeverityInfo:
		severity = "Info"
	case SeverityWarning:
		severity = "Warning"
	case SeverityError:
		severity = "Error"
	case SeverityCritical:
		severity = "Critical"
	}
	return fmt.Sprintf("[HelixAgent] Concurrency Alert: %s - %s", severity, alert.Type)
}

func (am *ConcurrencyAlertManager) formatConcurrencyEmail(alert ConcurrencyAlert) string {
	var builder strings.Builder

	// Convert severity to string
	severityStr := "Unknown"
	switch alert.Severity {
	case SeverityInfo:
		severityStr = "Info"
	case SeverityWarning:
		severityStr = "Warning"
	case SeverityError:
		severityStr = "Error"
	case SeverityCritical:
		severityStr = "Critical"
	}

	builder.WriteString(fmt.Sprintf("Concurrency Alert: %s\n", alert.Type))
	builder.WriteString(fmt.Sprintf("Severity: %s\n", severityStr))
	builder.WriteString(fmt.Sprintf("Time: %s\n", alert.Timestamp.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))

	if alert.Provider != "" {
		builder.WriteString(fmt.Sprintf("Provider: %s\n", alert.Provider))
		builder.WriteString(fmt.Sprintf("Saturation: %.1f%%\n", alert.Saturation))
		builder.WriteString(fmt.Sprintf("Active Requests: %d\n", alert.ActiveRequests))
		builder.WriteString(fmt.Sprintf("Total Permits: %d\n", alert.TotalPermits))
		builder.WriteString(fmt.Sprintf("Available: %d\n", alert.Available))
	}

	if alert.AllStats != nil {
		builder.WriteString(fmt.Sprintf("Providers Affected: %d\n", len(alert.AllStats)))
	}

	builder.WriteString(fmt.Sprintf("Escalation Level: %d\n", alert.EscalationLevel))
	builder.WriteString("\n---\n")
	builder.WriteString("This alert was generated by HelixAgent Concurrency Monitor.\n")
	builder.WriteString("Check the system logs for more details.\n")

	return builder.String()
}

func (am *ConcurrencyAlertManager) buildEmailMessage(fromAddress, subject, body string) []byte {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromAddress))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(am.config.EmailRecipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return []byte(msg.String())
}

func (am *ConcurrencyAlertManager) sendSMTPEmail(fromAddress string, message []byte) error {
	addr := fmt.Sprintf("%s:%d", am.config.SMTPHost, am.config.SMTPPort)

	// Create authentication if credentials provided
	var auth smtp.Auth
	if am.config.SMTPUsername != "" && am.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", am.config.SMTPUsername, am.config.SMTPPassword, am.config.SMTPHost)
	}

	if am.config.SMTPUseTLS {
		return am.sendEmailWithTLS(addr, auth, fromAddress, message)
	}

	return smtp.SendMail(addr, auth, fromAddress, am.config.EmailRecipients, message)
}

func (am *ConcurrencyAlertManager) sendEmailWithTLS(addr string, auth smtp.Auth, fromAddress string, message []byte) error {
	// Connect to the SMTP server with TLS
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: am.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		// Fallback to STARTTLS
		return am.sendEmailWithSTARTTLS(addr, auth, fromAddress, message)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, am.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Authenticate if credentials provided
	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("SMTP authentication failed: %w", authErr)
		}
	}

	// Set sender and recipients
	if mailErr := client.Mail(fromAddress); mailErr != nil {
		return fmt.Errorf("failed to set sender: %w", mailErr)
	}

	for _, recipient := range am.config.EmailRecipients {
		if rcptErr := client.Rcpt(recipient); rcptErr != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, rcptErr)
		}
	}

	// Send the message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data transfer: %w", err)
	}

	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

func (am *ConcurrencyAlertManager) sendEmailWithSTARTTLS(addr string, auth smtp.Auth, fromAddress string, message []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Try STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName: am.config.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}
		if tlsErr := client.StartTLS(config); tlsErr != nil {
			am.logger.Warning("STARTTLS failed, continuing without TLS", logrus.Fields{
				"error": tlsErr.Error(),
			})
		}
	}

	// Authenticate if credentials provided
	if auth != nil {
		if authErr := client.Auth(auth); authErr != nil {
			return fmt.Errorf("SMTP authentication failed: %w", authErr)
		}
	}

	// Set sender and recipients
	if mailErr := client.Mail(fromAddress); mailErr != nil {
		return fmt.Errorf("failed to set sender: %w", mailErr)
	}

	for _, recipient := range am.config.EmailRecipients {
		if rcptErr := client.Rcpt(recipient); rcptErr != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, rcptErr)
		}
	}

	// Send the message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data transfer: %w", err)
	}

	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// AsListener returns a ConcurrencyAlertListener that calls HandleAlert
func (am *ConcurrencyAlertManager) AsListener() ConcurrencyAlertListener {
	return func(alert ConcurrencyAlert) {
		am.HandleAlert(alert)
	}
}

// GetProviderThreshold returns the alert threshold for a provider
func (am *ConcurrencyAlertManager) GetProviderThreshold(provider string) (float64, bool) {
	return am.providerThresholds.Get(provider)
}

// getEnv returns environment variable or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv returns boolean environment variable or default
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// getDurationEnv returns duration environment variable or default
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// getFloat64Env returns float64 environment variable or default
func getFloat64Env(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

// getIntEnv returns int environment variable or default
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// getStringMapEnv returns map[string]float64 from JSON environment variable
func getStringMapEnv(key string) map[string]float64 {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	var m map[string]float64
	if err := json.Unmarshal([]byte(value), &m); err != nil {
		return nil
	}
	return m
}

// getIntSliceEnv returns []int from JSON environment variable
func getIntSliceEnv(key string) []int {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	var slice []int
	if err := json.Unmarshal([]byte(value), &slice); err != nil {
		return nil
	}
	return slice
}

// getEscalationChannelRoutingEnv returns map[int][]string from JSON environment variable
func getEscalationChannelRoutingEnv(key string) map[int][]string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	// Parse as map[string][]string first, then convert keys to int
	var m map[string][]string
	if err := json.Unmarshal([]byte(value), &m); err != nil {
		return nil
	}
	result := make(map[int][]string)
	for k, v := range m {
		intKey, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		result[intKey] = v
	}
	return result
}

// LoadConcurrencyAlertManagerConfigFromEnv loads configuration from environment variables
func LoadConcurrencyAlertManagerConfigFromEnv() ConcurrencyAlertManagerConfig {
	config := DefaultConcurrencyAlertManagerConfig()

	config.Enabled = getBoolEnv("CONCURRENCY_ALERTS_ENABLED", config.Enabled)
	config.DefaultCooldown = getDurationEnv("CONCURRENCY_ALERTS_COOLDOWN", config.DefaultCooldown)
	config.CleanupInterval = getDurationEnv("CONCURRENCY_ALERTS_CLEANUP_INTERVAL", config.CleanupInterval)
	config.MaxAlertAge = getDurationEnv("CONCURRENCY_ALERTS_MAX_AGE", config.MaxAlertAge)

	config.EnableLogging = getBoolEnv("CONCURRENCY_ALERTS_ENABLE_LOGGING", config.EnableLogging)
	config.EnableWebhook = getBoolEnv("CONCURRENCY_ALERTS_ENABLE_WEBHOOK", config.EnableWebhook)
	config.WebhookURL = getEnv("CONCURRENCY_ALERTS_WEBHOOK_URL", config.WebhookURL)
	config.WebhookTimeout = getDurationEnv("CONCURRENCY_ALERTS_WEBHOOK_TIMEOUT", config.WebhookTimeout)

	config.SlackWebhookURL = getEnv("CONCURRENCY_ALERTS_SLACK_WEBHOOK_URL", config.SlackWebhookURL)
	config.SlackChannel = getEnv("CONCURRENCY_ALERTS_SLACK_CHANNEL", config.SlackChannel)

	config.EnableEmail = getBoolEnv("CONCURRENCY_ALERTS_ENABLE_EMAIL", config.EnableEmail)
	config.SMTPHost = getEnv("CONCURRENCY_ALERTS_SMTP_HOST", config.SMTPHost)
	config.SMTPPort = getIntEnv("CONCURRENCY_ALERTS_SMTP_PORT", config.SMTPPort)
	config.SMTPUsername = getEnv("CONCURRENCY_ALERTS_SMTP_USERNAME", config.SMTPUsername)
	config.SMTPPassword = getEnv("CONCURRENCY_ALERTS_SMTP_PASSWORD", config.SMTPPassword)
	config.SMTPFrom = getEnv("CONCURRENCY_ALERTS_SMTP_FROM", config.SMTPFrom)
	config.SMTPUseTLS = getBoolEnv("CONCURRENCY_ALERTS_SMTP_USE_TLS", config.SMTPUseTLS)
	// EmailRecipients could be comma-separated list
	recipients := getEnv("CONCURRENCY_ALERTS_EMAIL_RECIPIENTS", "")
	if recipients != "" {
		config.EmailRecipients = strings.Split(recipients, ",")
	}

	// Provider thresholds as JSON map
	thresholds := getStringMapEnv("CONCURRENCY_ALERTS_PROVIDER_THRESHOLDS")
	if thresholds != nil {
		config.ProviderThresholds = thresholds
	}

	// Escalation configuration
	config.EscalationEnabled = getBoolEnv("CONCURRENCY_ALERTS_ESCALATION_ENABLED", config.EscalationEnabled)
	config.EscalationWindow = getDurationEnv("CONCURRENCY_ALERTS_ESCALATION_WINDOW", config.EscalationWindow)
	config.MaxEscalationLevel = getIntEnv("CONCURRENCY_ALERTS_MAX_ESCALATION_LEVEL", config.MaxEscalationLevel)

	// Escalation thresholds as JSON array
	escalationThresholds := getIntSliceEnv("CONCURRENCY_ALERTS_ESCALATION_THRESHOLDS")
	if escalationThresholds != nil {
		config.EscalationThresholds = escalationThresholds
	}

	// Escalation channel routing as JSON map
	escalationRouting := getEscalationChannelRoutingEnv("CONCURRENCY_ALERTS_ESCALATION_CHANNEL_ROUTING")
	if escalationRouting != nil {
		config.EscalationChannelRouting = escalationRouting
	}

	// Rate limiting configuration
	config.RateLimitEnabled = getBoolEnv("CONCURRENCY_ALERTS_RATE_LIMIT_ENABLED", config.RateLimitEnabled)
	config.RateLimitWindow = getDurationEnv("CONCURRENCY_ALERTS_RATE_LIMIT_WINDOW", config.RateLimitWindow)
	config.RateLimitMaxAlerts = getIntEnv("CONCURRENCY_ALERTS_RATE_LIMIT_MAX_ALERTS", config.RateLimitMaxAlerts)
	config.RateLimitBurstSize = getIntEnv("CONCURRENCY_ALERTS_RATE_LIMIT_BURST_SIZE", config.RateLimitBurstSize)

	// Circuit breaker configuration
	config.CircuitBreakerEnabled = getBoolEnv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_ENABLED", config.CircuitBreakerEnabled)
	config.CircuitBreakerFailures = getIntEnv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_FAILURES", config.CircuitBreakerFailures)
	config.CircuitBreakerReset = getDurationEnv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_RESET", config.CircuitBreakerReset)
	config.CircuitBreakerSuccesses = getIntEnv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_SUCCESSES", config.CircuitBreakerSuccesses)

	// Retry configuration
	config.RetryEnabled = getBoolEnv("CONCURRENCY_ALERTS_RETRY_ENABLED", config.RetryEnabled)
	config.MaxRetries = getIntEnv("CONCURRENCY_ALERTS_MAX_RETRIES", config.MaxRetries)
	config.RetryInitialDelay = getDurationEnv("CONCURRENCY_ALERTS_RETRY_INITIAL_DELAY", config.RetryInitialDelay)
	config.RetryMaxDelay = getDurationEnv("CONCURRENCY_ALERTS_RETRY_MAX_DELAY", config.RetryMaxDelay)
	config.RetryBackoffMultiplier = getFloat64Env("CONCURRENCY_ALERTS_RETRY_BACKOFF_MULTIPLIER", config.RetryBackoffMultiplier)

	// Dead letter queue monitoring configuration
	config.DeadLetterQueueMonitoringEnabled = getBoolEnv("CONCURRENCY_ALERTS_DEAD_LETTER_QUEUE_MONITORING_ENABLED", config.DeadLetterQueueMonitoringEnabled)
	config.DeadLetterQueueWarningThreshold = getIntEnv("CONCURRENCY_ALERTS_DEAD_LETTER_QUEUE_WARNING_THRESHOLD", config.DeadLetterQueueWarningThreshold)
	config.DeadLetterQueueCriticalThreshold = getIntEnv("CONCURRENCY_ALERTS_DEAD_LETTER_QUEUE_CRITICAL_THRESHOLD", config.DeadLetterQueueCriticalThreshold)

	return config
}

// severityToString converts AlertSeverity to string representation
func severityToString(severity AlertSeverity) string {
	switch severity {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// circuitBreakerStateToInt converts circuit breaker state string to numeric representation
func circuitBreakerStateToInt(state string) int {
	switch state {
	case "closed":
		return 0
	case "half-open":
		return 1
	case "open":
		return 2
	default:
		return -1
	}
}
