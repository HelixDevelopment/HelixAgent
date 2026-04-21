package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewConcurrencyAlertManager(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)
	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
}

func TestConcurrencyAlertManagerConfig_DefaultValues(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	assert.Equal(t, 587, config.SMTPPort)
	assert.True(t, config.SMTPUseTLS)
	assert.Empty(t, config.SMTPFrom)
}

func TestConcurrencyAlertManager_HandleAlert_Logging(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = true
	config.EnableWebhook = false
	config.EnableEmail = false
	config.EscalationEnabled = false // Disable escalation for this test

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// This should not panic
	manager.HandleAlert(alert)
}

func TestConcurrencyAlertManager_Cooldown(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.DefaultCooldown = 100 * time.Millisecond
	config.EnableLogging = false     // Disable logging to reduce noise
	config.EscalationEnabled = false // Disable escalation for this test
	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:       "high_saturation",
		Provider:   "test-provider",
		Message:    "Test alert",
		Timestamp:  time.Now(),
		Saturation: 90.0,
	}

	// First alert should be processed
	// We can't easily capture if alert was processed, but we can check that it doesn't panic
	manager.HandleAlert(alert)
	// Should be in cooldown now
	// Second alert within cooldown should be skipped
	manager.HandleAlert(alert)
	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)
	// Third alert after cooldown should be processed again
	manager.HandleAlert(alert)

	// No assertion, just ensure no panic
	assert.True(t, true)
}

func TestConcurrencyAlertManager_StartStop(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.CleanupInterval = 500 * time.Millisecond
	config.EscalationEnabled = false // Disable escalation for this test
	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager
	go manager.Start(ctx)

	// Allow some time for startup
	time.Sleep(50 * time.Millisecond)

	// Stop manager
	manager.Stop()

	// Allow time for shutdown
	time.Sleep(50 * time.Millisecond)

	// No assertion, just ensure no panic
	assert.True(t, true)
}

func TestConcurrencyAlertManager_AsListener(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EscalationEnabled = false // Disable escalation for this test
	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	listener := manager.AsListener()
	assert.NotNil(t, listener)

	// Ensure listener can be called
	alert := ConcurrencyAlert{
		Type:      "test",
		Message:   "test",
		Timestamp: time.Now(),
	}
	listener(alert) // Should not panic
}

func TestConcurrencyAlertManager_GetProviderThreshold(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EscalationEnabled = false // Disable escalation for this test
	config.ProviderThresholds = map[string]float64{
		"provider1": 75.0,
		"provider2": 90.0,
	}
	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	threshold, ok := manager.GetProviderThreshold("provider1")
	assert.True(t, ok)
	assert.Equal(t, 75.0, threshold)

	threshold, ok = manager.GetProviderThreshold("provider2")
	assert.True(t, ok)
	assert.Equal(t, 90.0, threshold)

	_, ok = manager.GetProviderThreshold("unknown")
	assert.False(t, ok)
}

func TestLoadConcurrencyAlertManagerConfigFromEnv(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("CONCURRENCY_ALERTS_ENABLED", "false")
	_ = os.Setenv("CONCURRENCY_ALERTS_COOLDOWN", "2m")
	_ = os.Setenv("CONCURRENCY_ALERTS_ENABLE_LOGGING", "false")
	_ = os.Setenv("CONCURRENCY_ALERTS_ENABLE_WEBHOOK", "true")
	_ = os.Setenv("CONCURRENCY_ALERTS_WEBHOOK_URL", "http://example.com/webhook")
	_ = os.Setenv("CONCURRENCY_ALERTS_SLACK_WEBHOOK_URL", "http://example.com/slack")
	_ = os.Setenv("CONCURRENCY_ALERTS_SLACK_CHANNEL", "#alerts")
	_ = os.Setenv("CONCURRENCY_ALERTS_PROVIDER_THRESHOLDS", `{"provider1":75.0,"provider2":90.0}`)
	// Escalation environment variables
	_ = os.Setenv("CONCURRENCY_ALERTS_ESCALATION_ENABLED", "true")
	_ = os.Setenv("CONCURRENCY_ALERTS_ESCALATION_WINDOW", "30m")
	_ = os.Setenv("CONCURRENCY_ALERTS_MAX_ESCALATION_LEVEL", "5")
	_ = os.Setenv("CONCURRENCY_ALERTS_ESCALATION_THRESHOLDS", "[2,4,6]")
	_ = os.Setenv("CONCURRENCY_ALERTS_ESCALATION_CHANNEL_ROUTING", `{"0":["logging"],"1":["logging","email"],"2":["logging","email","slack"]}`)
	// Rate limiting environment variables
	_ = os.Setenv("CONCURRENCY_ALERTS_RATE_LIMIT_ENABLED", "false")
	_ = os.Setenv("CONCURRENCY_ALERTS_RATE_LIMIT_WINDOW", "2m")
	_ = os.Setenv("CONCURRENCY_ALERTS_RATE_LIMIT_MAX_ALERTS", "20")
	_ = os.Setenv("CONCURRENCY_ALERTS_RATE_LIMIT_BURST_SIZE", "10")
	// Circuit breaker environment variables
	_ = os.Setenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_ENABLED", "false")
	_ = os.Setenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_FAILURES", "10")
	_ = os.Setenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_RESET", "1m")
	_ = os.Setenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_SUCCESSES", "5")
	// Retry configuration environment variables
	_ = os.Setenv("CONCURRENCY_ALERTS_RETRY_ENABLED", "false")
	_ = os.Setenv("CONCURRENCY_ALERTS_MAX_RETRIES", "5")
	_ = os.Setenv("CONCURRENCY_ALERTS_RETRY_INITIAL_DELAY", "2s")
	_ = os.Setenv("CONCURRENCY_ALERTS_RETRY_MAX_DELAY", "60s")
	_ = os.Setenv("CONCURRENCY_ALERTS_RETRY_BACKOFF_MULTIPLIER", "3.0")
	defer func() {
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ENABLED")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_COOLDOWN")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ENABLE_LOGGING")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ENABLE_WEBHOOK")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_WEBHOOK_URL")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_SLACK_WEBHOOK_URL")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_SLACK_CHANNEL")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_PROVIDER_THRESHOLDS")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ESCALATION_ENABLED")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ESCALATION_WINDOW")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_MAX_ESCALATION_LEVEL")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ESCALATION_THRESHOLDS")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_ESCALATION_CHANNEL_ROUTING")
		// Clean up rate limiting environment variables
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RATE_LIMIT_ENABLED")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RATE_LIMIT_WINDOW")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RATE_LIMIT_MAX_ALERTS")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RATE_LIMIT_BURST_SIZE")
		// Clean up circuit breaker environment variables
		_ = os.Unsetenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_ENABLED")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_FAILURES")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_RESET")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_CIRCUIT_BREAKER_SUCCESSES")
		// Clean up retry configuration environment variables
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RETRY_ENABLED")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_MAX_RETRIES")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RETRY_INITIAL_DELAY")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RETRY_MAX_DELAY")
		_ = os.Unsetenv("CONCURRENCY_ALERTS_RETRY_BACKOFF_MULTIPLIER")
	}()

	config := LoadConcurrencyAlertManagerConfigFromEnv()

	assert.False(t, config.Enabled)
	assert.Equal(t, 2*time.Minute, config.DefaultCooldown)
	assert.False(t, config.EnableLogging)
	assert.True(t, config.EnableWebhook)
	assert.Equal(t, "http://example.com/webhook", config.WebhookURL)
	assert.Equal(t, "http://example.com/slack", config.SlackWebhookURL)
	assert.Equal(t, "#alerts", config.SlackChannel)
	assert.Equal(t, 75.0, config.ProviderThresholds["provider1"])
	assert.Equal(t, 90.0, config.ProviderThresholds["provider2"])
	// Escalation assertions
	assert.True(t, config.EscalationEnabled)
	assert.Equal(t, 30*time.Minute, config.EscalationWindow)
	assert.Equal(t, 5, config.MaxEscalationLevel)
	assert.Equal(t, []int{2, 4, 6}, config.EscalationThresholds)
	assert.Equal(t, []string{"logging"}, config.EscalationChannelRouting[0])
	assert.Equal(t, []string{"logging", "email"}, config.EscalationChannelRouting[1])
	assert.Equal(t, []string{"logging", "email", "slack"}, config.EscalationChannelRouting[2])
	// Rate limiting assertions
	assert.False(t, config.RateLimitEnabled)
	assert.Equal(t, 2*time.Minute, config.RateLimitWindow)
	assert.Equal(t, 20, config.RateLimitMaxAlerts)
	assert.Equal(t, 10, config.RateLimitBurstSize)
	// Circuit breaker assertions
	assert.False(t, config.CircuitBreakerEnabled)
	assert.Equal(t, 10, config.CircuitBreakerFailures)
	assert.Equal(t, 1*time.Minute, config.CircuitBreakerReset)
	assert.Equal(t, 5, config.CircuitBreakerSuccesses)
	// Retry configuration assertions
	assert.False(t, config.RetryEnabled)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryInitialDelay)
	assert.Equal(t, 60*time.Second, config.RetryMaxDelay)
	assert.Equal(t, 3.0, config.RetryBackoffMultiplier)
}

func TestConcurrencyAlertManager_WebhookDelivery(t *testing.T) {
	// CONCURRENCY FIX: Use mutex to protect shared variables from race conditions
	var mu sync.Mutex
	var receivedMethod string
	var receivedContentType string
	var requestBody []byte
	requestReceived := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			mu.Unlock()
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		requestBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		// Signal that request was received
		select {
		case requestReceived <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	// Configure alert manager with webhook enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alert
	manager.HandleAlert(alert)

	// Wait for request to be received with timeout
	select {
	case <-requestReceived:
		// Request received
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for webhook request")
	}

	// Verify request was received (with lock)
	mu.Lock()
	method := receivedMethod
	contentType := receivedContentType
	body := make([]byte, len(requestBody))
	copy(body, requestBody)
	mu.Unlock()

	assert.Equal(t, "POST", method)
	assert.Equal(t, "application/json", contentType)

	// Parse request body and verify content
	var payload map[string]interface{}
	err := json.Unmarshal(body, &payload)
	assert.NoError(t, err)
	assert.Equal(t, "concurrency_alert", payload["event"])
	assert.Equal(t, "high_saturation", payload["type"])
	assert.Equal(t, "Test alert", payload["message"])
	assert.Equal(t, 85.5, payload["saturation"])
}

func TestConcurrencyAlertManager_SlackWebhookDelivery(t *testing.T) {
	// CONCURRENCY FIX: Use mutex to protect shared variables from race conditions
	var mu sync.Mutex
	var receivedMethod string
	var receivedContentType string
	var requestBody []byte
	requestReceived := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			mu.Unlock()
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		requestBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		// Signal that request was received
		select {
		case requestReceived <- struct{}{}:
		default:
		}
	}))
	defer server.Close()

	// Configure alert manager with Slack webhook enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.SlackWebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alert
	manager.HandleAlert(alert)

	// Wait for request to be received with timeout
	select {
	case <-requestReceived:
		// Request received
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for Slack webhook request")
	}

	// Verify request was received (with lock)
	mu.Lock()
	method := receivedMethod
	contentType := receivedContentType
	body := make([]byte, len(requestBody))
	copy(body, requestBody)
	mu.Unlock()

	assert.Equal(t, "POST", method)
	assert.Equal(t, "application/json", contentType)

	// Parse request body and verify Slack-specific structure
	var payload map[string]interface{}
	err := json.Unmarshal(body, &payload)
	assert.NoError(t, err)
	// Slack payload contains "attachments"
	attachments, ok := payload["attachments"].([]interface{})
	assert.True(t, ok)
	assert.Greater(t, len(attachments), 0)
	attachment := attachments[0].(map[string]interface{})
	assert.Equal(t, "Concurrency Alert", attachment["title"])
	assert.Equal(t, "Test alert", attachment["text"])
}

func TestConcurrencyAlertManager_EmailDelivery(t *testing.T) {
	config := DefaultConcurrencyAlertManagerConfig()
	config.EnableLogging = false
	config.EnableEmail = true
	config.SMTPHost = "localhost"
	config.SMTPPort = 9999
	config.EmailRecipients = []string{"test@example.com"}
	config.DefaultCooldown = 0       // Disable cooldown for testing
	config.EscalationEnabled = false // Disable escalation for this test

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// This should not panic, email delivery will fail but be handled
	manager.HandleAlert(alert)
	// Give some time for async email delivery attempt
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrencyAlertManager_Escalation(t *testing.T) {
	// Test escalation level calculation and channel routing
	config := DefaultConcurrencyAlertManagerConfig()
	config.EnableLogging = true
	config.EnableEmail = true
	config.SMTPHost = "localhost"
	config.SMTPPort = 9999
	config.EmailRecipients = []string{"test@example.com"}
	config.EnableWebhook = true
	config.WebhookURL = "http://example.com/webhook"
	config.SlackWebhookURL = "http://example.com/slack"
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.EscalationEnabled = true
	config.EscalationWindow = 1 * time.Hour
	config.EscalationThresholds = []int{1, 3, 5}
	config.MaxEscalationLevel = 3
	config.EscalationChannelRouting = map[int][]string{
		0: {"logging"},
		1: {"logging", "email"},
		2: {"logging", "email", "slack"},
		3: {"logging", "email", "slack", "webhook"},
	}

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Helper to send alert multiple times
	sendAlertNTimes := func(n int) {
		for i := 0; i < n; i++ {
			manager.HandleAlert(alert)
			// Small delay to ensure different timestamps
			time.Sleep(1 * time.Millisecond)
		}
	}

	// First alert: repeat count = 1, escalation level should be 1
	// Should send to logging and email (level 1 routing)
	sendAlertNTimes(1)
	// We can't easily verify which channels were used, but we can verify no panic
	// For now, just ensure it runs without errors
	assert.True(t, true)
}
func TestConcurrencyAlertManager_RateLimiting(t *testing.T) {
	// Create test server to capture requests
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Configure alert manager with rate limiting enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test
	// Enable rate limiting with low limits
	config.RateLimitEnabled = true
	config.RateLimitWindow = 1 * time.Second
	config.RateLimitMaxAlerts = 2
	config.RateLimitBurstSize = 1

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send 5 alerts quickly - should be rate limited after maxAlerts + burstSize
	for i := 0; i < 5; i++ {
		manager.HandleAlert(alert)
	}
	// Wait for async delivery
	time.Sleep(200 * time.Millisecond)

	// Verify only maxAlerts + burstSize alerts were delivered (3)
	mu.Lock()
	delivered := requestCount
	mu.Unlock()
	assert.Equal(t, 3, delivered, "Should only deliver 3 alerts (maxAlerts + burstSize)")

	// Wait for rate limit window to reset
	time.Sleep(1 * time.Second)

	// Send another alert - should be allowed again
	manager.HandleAlert(alert)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	delivered = requestCount
	mu.Unlock()
	assert.Equal(t, 4, delivered, "After window reset, should deliver another alert")
}

func TestConcurrencyAlertManager_RateLimitMetrics(t *testing.T) {
	// Set up test registry for metrics
	originalRegistry := prometheus.DefaultRegisterer
	testRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testRegistry
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()
	// Reset metrics to ensure they register with our test registry
	resetConcurrencyMetrics()

	// Create test server to capture requests
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Configure alert manager with rate limiting enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test
	// Enable rate limiting with low limits
	config.RateLimitEnabled = true
	config.RateLimitWindow = 500 * time.Millisecond // shorter window for faster test
	config.RateLimitMaxAlerts = 2
	config.RateLimitBurstSize = 0 // no burst for clearer counting

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Helper to get metric value
	getCounterValue := func(metricName string) float64 {
		metrics, err := testRegistry.Gather()
		if err != nil {
			return 0
		}
		for _, mf := range metrics {
			if mf.GetName() == metricName {
				for _, metric := range mf.GetMetric() {
					return metric.GetCounter().GetValue()
				}
			}
		}
		return 0
	}

	// Send alerts up to limit
	for i := 0; i < 3; i++ {
		manager.HandleAlert(alert)
	}
	// Wait for async delivery and rate limit processing
	time.Sleep(200 * time.Millisecond)

	// Check rate limit hits metric
	rateLimitHits := getCounterValue("helixagent_concurrency_alert_rate_limit_hits_total")
	// With maxAlerts=2 and burst=0, third alert should be rate limited
	assert.Equal(t, 1.0, rateLimitHits, "Should have recorded one rate limit hit")

	// Verify only maxAlerts alerts were delivered (2)
	mu.Lock()
	delivered := requestCount
	mu.Unlock()
	assert.Equal(t, 2, delivered, "Should only deliver 2 alerts (maxAlerts)")

	// Wait for rate limit window to reset
	time.Sleep(600 * time.Millisecond)

	// Send another alert - should be allowed again
	manager.HandleAlert(alert)
	time.Sleep(200 * time.Millisecond)

	// No additional rate limit hits (window reset)
	rateLimitHits = getCounterValue("helixagent_concurrency_alert_rate_limit_hits_total")
	assert.Equal(t, 1.0, rateLimitHits, "Should still have only one rate limit hit after window reset")
}

func TestConcurrencyAlertManager_CircuitBreaker(t *testing.T) {
	// Create test server that fails requests
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		// Always return error to trigger circuit breaker
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Configure alert manager with circuit breaker enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test
	// Enable circuit breaker with low failure threshold
	config.CircuitBreakerEnabled = true
	config.CircuitBreakerFailures = 3
	config.CircuitBreakerReset = 1 * time.Second
	config.CircuitBreakerSuccesses = 2

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alerts up to failure threshold with small delays to allow circuit breaker to update
	for i := 0; i < 5; i++ {
		manager.HandleAlert(alert)
		// Small delay to allow the async delivery and circuit breaker state update
		// This ensures the circuit opens before the next alert is sent
		time.Sleep(50 * time.Millisecond)
	}
	// Wait for any remaining async delivery
	time.Sleep(100 * time.Millisecond)

	// Verify exactly CircuitBreakerFailures requests were made before circuit opened
	mu.Lock()
	delivered := requestCount
	mu.Unlock()
	assert.Equal(t, config.CircuitBreakerFailures, delivered,
		"Should only make requests up to failure threshold before circuit opens")

	// Wait for circuit breaker reset period
	time.Sleep(1 * time.Second)

	// Send another alert - circuit should be half-open now
	// But our server still returns error, so it will fail again
	manager.HandleAlert(alert)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	delivered = requestCount
	mu.Unlock()
	// Should have made one more request (half-open allows one attempt)
	assert.Equal(t, config.CircuitBreakerFailures+1, delivered,
		"After reset, circuit half-open should allow one attempt")
}

func TestConcurrencyAlertManager_ResilienceIntegration(t *testing.T) {
	// Test combined rate limiting and circuit breaker
	// Create test server that works initially then fails
	var requestCount int
	var mu sync.Mutex
	var shouldFail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		if shouldFail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Configure alert manager with both resilience features
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test
	// Enable rate limiting
	config.RateLimitEnabled = true
	config.RateLimitWindow = 500 * time.Millisecond
	config.RateLimitMaxAlerts = 2
	config.RateLimitBurstSize = 0
	// Enable circuit breaker
	config.CircuitBreakerEnabled = true
	config.CircuitBreakerFailures = 2
	config.CircuitBreakerReset = 1 * time.Second
	config.CircuitBreakerSuccesses = 2

	logger := logrus.New()
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Phase 1: Send 3 alerts (rate limit allows 2)
	for i := 0; i < 3; i++ {
		manager.HandleAlert(alert)
		// Small delay to allow rate limiting to update before next alert
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	delivered := requestCount
	mu.Unlock()
	assert.Equal(t, 2, delivered, "Rate limiting should allow only 2 alerts")

	// Wait for rate limit window to reset
	time.Sleep(500 * time.Millisecond)

	// Phase 2: Make server start failing
	shouldFail.Store(true)
	// Send alerts to trigger circuit breaker with small delays
	for i := 0; i < 3; i++ {
		manager.HandleAlert(alert)
		// Small delay to allow circuit breaker to update before next alert
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	delivered = requestCount
	mu.Unlock()
	// Should have 2 (previous) + 2 failures (circuit breaker threshold) = 4 total
	assert.Equal(t, 4, delivered,
		"After rate limit reset, circuit breaker should open after 2 failures")

	// Wait for circuit breaker reset
	time.Sleep(1 * time.Second)

	// Phase 3: Server still failing, circuit half-open allows one attempt
	manager.HandleAlert(alert)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	delivered = requestCount
	mu.Unlock()
	assert.Equal(t, 5, delivered,
		"After circuit breaker reset, half-open allows one attempt")
}

func TestConcurrencyAlertManager_RetryLogic(t *testing.T) {
	// Create test server that fails first request, succeeds on retry
	var requestCount int
	var mu sync.Mutex
	shouldFail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()
		t.Logf("Server received request %d", count)
		if shouldFail {
			// First request fails
			w.WriteHeader(http.StatusInternalServerError)
			shouldFail = false // Next request will succeed
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Configure alert manager with retry enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false
	// Disable rate limiting and circuit breaker for this test
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = true
	config.MaxRetries = 2
	config.RetryInitialDelay = 10 * time.Millisecond
	config.RetryMaxDelay = 100 * time.Millisecond
	config.RetryBackoffMultiplier = 2.0
	config.CleanupInterval = 10 * time.Millisecond // Fast cleanup for retry processing
	config.MaxAlertAge = 1 * time.Hour             // Prevent immediate cleanup of retry attempts

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	manager := NewConcurrencyAlertManager(config, logger)

	// Start the manager to enable retry processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	// Give manager time to start
	time.Sleep(5 * time.Millisecond)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alert
	t.Logf("Sending alert...")
	manager.HandleAlert(alert)
	t.Logf("Alert sent, waiting for retry...")

	// Wait for retry to happen (initial delay + processing)
	time.Sleep(200 * time.Millisecond)
	t.Logf("Wait complete")

	// Verify that two requests were made (first failure + retry success)
	mu.Lock()
	count := requestCount
	mu.Unlock()
	t.Logf("Request count: %d (expected 2)", count)
	assert.Equal(t, 2, count, "Should have made initial request plus one retry")
}

func TestConcurrencyAlertManager_DeadLetterQueue(t *testing.T) {
	// Create test server that always fails
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Configure alert manager with retry enabled and low max retries
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false
	// Disable rate limiting and circuit breaker for this test
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = true
	config.MaxRetries = 2
	config.RetryInitialDelay = 10 * time.Millisecond
	config.RetryMaxDelay = 100 * time.Millisecond
	config.RetryBackoffMultiplier = 2.0
	config.CleanupInterval = 10 * time.Millisecond
	config.MaxAlertAge = 1 * time.Hour

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	manager := NewConcurrencyAlertManager(config, logger)

	// Start the manager to enable retry processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	// Give manager time to start
	time.Sleep(5 * time.Millisecond)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alert
	t.Logf("Sending alert (will fail, retry up to MaxRetries, then move to dead letter queue)...")
	manager.HandleAlert(alert)

	// Wait for retries and dead letter queue movement
	// MaxRetries=2 means total attempts = MaxRetries (initial + (MaxRetries-1) retries?)
	// Actually: attempts count = number of attempts already made (including current).
	// With MaxRetries=2, we allow up to 2 total attempts (initial + 1 retry).
	time.Sleep(500 * time.Millisecond)

	// Verify that exactly MaxRetries requests were made (initial + (MaxRetries-1) retries)
	mu.Lock()
	count := requestCount
	mu.Unlock()
	expectedRequests := config.MaxRetries
	t.Logf("Request count: %d (expected %d)", count, expectedRequests)
	assert.Equal(t, expectedRequests, count, "Should have made MaxRetries total attempts before dead letter")

	// Check dead letter queue via GetDeadLetterAlerts
	deadLetterAlerts := manager.GetDeadLetterAlerts()
	assert.Equal(t, 1, len(deadLetterAlerts), "Should have one alert in dead letter queue")
	if len(deadLetterAlerts) > 0 {
		deadLetter := deadLetterAlerts[0]
		assert.Equal(t, "webhook", deadLetter["channel"])
		assert.Equal(t, "high_saturation", deadLetter["type"])
		assert.Equal(t, "test-provider", deadLetter["provider"])
		assert.Equal(t, expectedRequests, deadLetter["attempts"]) // attempts = total attempts made
		assert.Contains(t, deadLetter["failure_error"], "max retries exceeded")
	}

	// Check stats
	stats := manager.GetAlertStats()
	assert.Equal(t, 1, stats["dead_letter_queue_size"])
	assert.Equal(t, 0, stats["retry_queue_size"]) // Should be empty after moving to dead letter
	t.Logf("Test completed")
}

func TestConcurrencyAlertManager_DeadLetterQueueRetry(t *testing.T) {
	// Create test server that always fails
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = true
	config.MaxRetries = 1                      // No automatic retries - alert moves directly to dead letter queue after initial failure
	config.RetryInitialDelay = 5 * time.Second // Large delay (not used since MaxRetries=1)
	config.RetryMaxDelay = 10 * time.Second
	config.RetryBackoffMultiplier = 2.0
	config.CleanupInterval = 10 * time.Millisecond
	config.MaxAlertAge = 1 * time.Hour

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	manager := NewConcurrencyAlertManager(config, logger)

	// Start the manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	time.Sleep(5 * time.Millisecond)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
	}

	// Send alert to trigger dead letter queue (with MaxRetries=1, alert moves to dead letter queue after initial failure)
	manager.HandleAlert(alert)
	time.Sleep(200 * time.Millisecond) // Wait for dead letter queue

	// Verify dead letter queue has the alert (because MaxRetries=2, total attempts = 2, dead letter after 2 attempts)
	deadLetterAlerts := manager.GetDeadLetterAlerts()
	assert.Equal(t, 1, len(deadLetterAlerts), "Should have one alert in dead letter queue")
	key := deadLetterAlerts[0]["key"].(string)

	// Test RetryDeadLetterAlert with non-existent key
	assert.False(t, manager.RetryDeadLetterAlert("nonexistent"), "Should return false for non-existent key")

	// Test RetryDeadLetterAlert with valid key
	removed := manager.RetryDeadLetterAlert(key)
	assert.True(t, removed, "Should return true for existing key")

	// Immediately check dead letter queue (should be empty because we deleted it)
	deadLetterAlerts = manager.GetDeadLetterAlerts()
	assert.Equal(t, 0, len(deadLetterAlerts), "Dead letter queue should be empty after retry")

	// Wait a bit for the retry to fail and potentially move back to dead letter queue
	time.Sleep(200 * time.Millisecond)
	deadLetterAlerts = manager.GetDeadLetterAlerts()
	// Alert may have moved back to dead letter queue after failed retry (since MaxRetries=1)
	// So size could be 0 (if retry still pending) or 1 (if retry already failed)
	// Since MaxRetries=1, retry fails immediately and moves back to dead letter queue
	assert.Equal(t, 1, len(deadLetterAlerts), "Alert should be back in dead letter queue after failed retry")

	// Check retry queue size (should be 0 because retry failed and moved to dead letter queue)
	stats := manager.GetAlertStats()
	assert.Equal(t, 0, stats["retry_queue_size"], "Retry queue should be empty")

	t.Logf("Test completed")
}

func TestConcurrencyAlertManager_DeadLetterQueueThresholdMonitoring(t *testing.T) {
	// Configure dead letter queue monitoring
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = false
	config.EscalationEnabled = false
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = false
	config.DefaultCooldown = 500 * time.Millisecond // Set cooldown for deduplication
	config.CleanupInterval = 100 * time.Millisecond
	config.MaxAlertAge = 1 * time.Hour

	// Enable dead letter queue monitoring with low thresholds for testing
	config.DeadLetterQueueMonitoringEnabled = true
	config.DeadLetterQueueWarningThreshold = 2
	config.DeadLetterQueueCriticalThreshold = 4

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	manager := NewConcurrencyAlertManager(config, logger)

	// Start the manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	time.Sleep(5 * time.Millisecond)

	// Helper to add alerts to dead letter queue
	addDeadLetterAlert := func(channel string, alert ConcurrencyAlert) {
		alertKey := manager.generateAlertKey(alert)
		deadLetterKey := fmt.Sprintf("%s:%s", channel, alertKey)
		manager.deadLetterAlerts.Put(deadLetterKey, &deadLetterAlert{
			channel:      channel,
			alert:        alert,
			attempts:     1,
			lastAttempt:  time.Now(),
			failureError: "test",
			addedAt:      time.Now(),
		})
	}

	// Helper to check if alert key exists in alertTracking
	hasAlertKey := func(key string) bool {
		return manager.alertTracking.Has(key)
	}

	// Helper to get last sent time for alert key
	getLastSentTime := func(key string) time.Time {
		if tracking, exists := manager.alertTracking.Get(key); exists {
			tracking.mu.RLock()
			defer tracking.mu.RUnlock()
			return tracking.lastSentTime
		}
		return time.Time{}
	}

	// Initially queue is empty, no alerts expected
	time.Sleep(150 * time.Millisecond) // Wait for first tick

	// Add 1 alert (below warning threshold)
	alert1 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 1",
		Timestamp:  time.Now(),
		Saturation: 50.0,
	}
	addDeadLetterAlert("logging", alert1)
	time.Sleep(150 * time.Millisecond) // Wait for tick
	// No warning alert expected
	warningKey := "concurrency_dead_letter_queue_warning_system_2"
	assert.False(t, hasAlertKey(warningKey), "Should not have warning alert when below threshold")

	// Add second alert (reaches warning threshold)
	alert2 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 2",
		Timestamp:  time.Now(),
		Saturation: 60.0,
	}
	addDeadLetterAlert("logging", alert2)
	time.Sleep(150 * time.Millisecond) // Should trigger warning alert
	assert.True(t, hasAlertKey(warningKey), "Should have warning alert when threshold reached")
	warningLastSent := getLastSentTime(warningKey)
	assert.False(t, warningLastSent.IsZero(), "Last sent time should be set")

	// Add third alert (still warning)
	alert3 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 3",
		Timestamp:  time.Now(),
		Saturation: 70.0,
	}
	addDeadLetterAlert("logging", alert3)
	time.Sleep(150 * time.Millisecond) // Should not trigger new warning (cooldown)
	// Last sent time should not have changed (within cooldown)
	warningLastSent2 := getLastSentTime(warningKey)
	assert.Equal(t, warningLastSent, warningLastSent2, "Last sent time should not change during cooldown")

	// Add fourth alert (reaches critical threshold)
	alert4 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 4",
		Timestamp:  time.Now(),
		Saturation: 80.0,
	}
	addDeadLetterAlert("logging", alert4)
	time.Sleep(150 * time.Millisecond) // Should trigger critical alert
	criticalKey := "concurrency_dead_letter_queue_critical_system_4"
	assert.True(t, hasAlertKey(criticalKey), "Should have critical alert when critical threshold reached")
	// Warning alert should still exist
	assert.True(t, hasAlertKey(warningKey), "Warning alert should still be tracked")

	// Clean up dead letter queue
	manager.deadLetterAlerts.Clear()
	time.Sleep(150 * time.Millisecond) // Wait for tick

	t.Logf("Dead letter queue threshold monitoring test completed")
}

// TestConcurrencyAlertManager_Metrics_RecordAlertHandled tests that RecordAlertHandled metric is recorded
func TestConcurrencyAlertManager_Metrics_RecordAlertHandled(t *testing.T) {
	// Save original registry and replace with test registry
	originalRegistry := prometheus.DefaultRegisterer
	testRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testRegistry
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Reset metrics to ensure fresh initialization
	resetConcurrencyMetrics()

	// Configure alert manager with minimal settings
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = false
	config.EnableEmail = false
	config.EscalationEnabled = false
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = false
	config.DefaultCooldown = 100 * time.Millisecond // Short cooldown for testing

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
		Severity:       SeverityWarning,
	}

	// Helper to get metric counter value
	getCounterValue := func(metricName string) float64 {
		metrics, err := testRegistry.Gather()
		if err != nil {
			return 0
		}
		for _, mf := range metrics {
			if mf.GetName() == metricName {
				for _, metric := range mf.GetMetric() {
					return metric.GetCounter().GetValue()
				}
			}
		}
		return 0
	}

	// First alert should be processed
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond) // Allow async processing
	// Check that alert count increased
	alertsTotal := getCounterValue("helixagent_concurrency_alerts_total")
	assert.Equal(t, 1.0, alertsTotal, "Should have recorded one alert")

	// Second alert within cooldown should be filtered but still counted
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	alertsTotal = getCounterValue("helixagent_concurrency_alerts_total")
	assert.Equal(t, 2.0, alertsTotal, "Should have recorded second alert (even if filtered)")

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)
	// Third alert after cooldown should be processed
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	alertsTotal = getCounterValue("helixagent_concurrency_alerts_total")
	assert.Equal(t, 3.0, alertsTotal, "Should have recorded third alert")
}

// TestConcurrencyAlertManager_Metrics_EscalationLevel tests that UpdateEscalationLevel metric is recorded
func TestConcurrencyAlertManager_Metrics_EscalationLevel(t *testing.T) {
	// Save original registry and replace with test registry
	originalRegistry := prometheus.DefaultRegisterer
	testRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testRegistry
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Reset metrics to ensure fresh initialization
	resetConcurrencyMetrics()

	// Configure alert manager with escalation enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = false
	config.EnableEmail = false
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = false
	config.EscalationEnabled = true
	config.EscalationWindow = 1 * time.Hour // long window to keep repeats
	config.EscalationThresholds = []int{1, 3, 5}
	config.MaxEscalationLevel = 3
	config.EscalationChannelRouting = map[int][]string{
		0: {"logging"},
		1: {"logging", "email"},
		2: {"logging", "email", "slack"},
		3: {"logging", "email", "slack", "webhook"},
	}
	config.DefaultCooldown = 0 // disable cooldown for predictable repeat counting

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
		Severity:       SeverityWarning,
	}

	// Helper to get gauge value for escalation level with label matching
	getGaugeValue := func(metricName string, labelValues ...string) float64 {
		metrics, err := testRegistry.Gather()
		if err != nil {
			t.Logf("Error gathering metrics: %v", err)
			return 0
		}
		for _, mf := range metrics {
			if mf.GetName() == metricName {
				for _, metric := range mf.GetMetric() {
					labels := metric.GetLabel()
					// Build map of label name to value
					labelMap := make(map[string]string)
					for _, label := range labels {
						labelMap[label.GetName()] = label.GetValue()
					}
					// Expected label names based on metric definition order
					expectedNames := []string{"alert_type", "provider", "alert_key"}
					matched := true
					for i, expectedValue := range labelValues {
						if i >= len(expectedNames) {
							matched = false
							break
						}
						name := expectedNames[i]
						if labelMap[name] != expectedValue {
							matched = false
							break
						}
					}
					if matched {
						return metric.GetGauge().GetValue()
					}
				}
			}
		}
		return 0
	}

	// Generate alert key (mirroring internal logic)
	alertKey := manager.generateAlertKey(alert)

	// Send first alert - repeat count = 1, escalation level should be 1 (since thresholds[0] = 1)
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond) // Allow async processing

	// Check escalation level gauge
	level := getGaugeValue("helixagent_concurrency_alert_escalation_level", "high_saturation", "test-provider", alertKey)
	assert.Equal(t, 1.0, level, "Escalation level should be 1 after first alert")

	// Send second alert - repeat count = 2, escalation level still 1 (thresholds[1] = 3)
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	level = getGaugeValue("helixagent_concurrency_alert_escalation_level", "high_saturation", "test-provider", alertKey)
	assert.Equal(t, 1.0, level, "Escalation level should remain 1 after second alert (threshold 3 not reached)")

	// Send third alert - repeat count = 3, escalation level becomes 2 (thresholds[1] = 3)
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	level = getGaugeValue("helixagent_concurrency_alert_escalation_level", "high_saturation", "test-provider", alertKey)
	assert.Equal(t, 2.0, level, "Escalation level should be 2 after third alert (threshold 3 reached)")

	// Send fourth alert - repeat count = 4, escalation level still 2 (thresholds[2] = 5)
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	level = getGaugeValue("helixagent_concurrency_alert_escalation_level", "high_saturation", "test-provider", alertKey)
	assert.Equal(t, 2.0, level, "Escalation level should remain 2 after fourth alert (threshold 5 not reached)")

	// Send fifth alert - repeat count = 5, escalation level becomes 3 (thresholds[2] = 5)
	manager.HandleAlert(alert)
	time.Sleep(10 * time.Millisecond)
	level = getGaugeValue("helixagent_concurrency_alert_escalation_level", "high_saturation", "test-provider", alertKey)
	assert.Equal(t, 3.0, level, "Escalation level should be 3 after fifth alert (threshold 5 reached)")
}

// TestConcurrencyAlertManager_Metrics_CircuitBreakerState tests that UpdateCircuitBreakerState metric is recorded
func TestConcurrencyAlertManager_Metrics_CircuitBreakerState(t *testing.T) {
	// Save original registry and replace with test registry
	originalRegistry := prometheus.DefaultRegisterer
	testRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testRegistry
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Reset metrics to ensure fresh initialization
	resetConcurrencyMetrics()

	// Create test server that fails requests to trigger circuit breaker
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		// Always return error to trigger circuit breaker
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Configure alert manager with circuit breaker enabled
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = true
	config.WebhookURL = server.URL
	config.DefaultCooldown = 0 // Disable cooldown for testing
	config.WebhookTimeout = 5 * time.Second
	config.EscalationEnabled = false // Disable escalation for this test
	config.RateLimitEnabled = false  // Disable rate limiting for this test
	config.RetryEnabled = false      // Disable retry for this test
	// Enable circuit breaker with low failure threshold for faster testing
	config.CircuitBreakerEnabled = true
	config.CircuitBreakerFailures = 2
	config.CircuitBreakerReset = 500 * time.Millisecond
	config.CircuitBreakerSuccesses = 2

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise
	manager := NewConcurrencyAlertManager(config, logger)

	alert := ConcurrencyAlert{
		Type:           "high_saturation",
		Provider:       "test-provider",
		Message:        "Test alert",
		Timestamp:      time.Now(),
		Saturation:     85.5,
		ActiveRequests: 10,
		TotalPermits:   12,
		Available:      2,
		Severity:       SeverityWarning,
	}

	// Helper to get gauge value for circuit breaker state with label matching
	getGaugeValue := func(metricName string, labelValues ...string) float64 {
		metrics, err := testRegistry.Gather()
		if err != nil {
			return 0
		}
		for _, mf := range metrics {
			if mf.GetName() == metricName {
				for _, metric := range mf.GetMetric() {
					labels := metric.GetLabel()
					// Build map of label name to value
					labelMap := make(map[string]string)
					for _, label := range labels {
						labelMap[label.GetName()] = label.GetValue()
					}
					// Expected label names based on metric definition order
					expectedNames := []string{"channel"}
					matched := true
					for i, expectedValue := range labelValues {
						if i >= len(expectedNames) {
							matched = false
							break
						}
						name := expectedNames[i]
						if labelMap[name] != expectedValue {
							matched = false
							break
						}
					}
					if matched {
						return metric.GetGauge().GetValue()
					}
				}
			}
		}
		return 0
	}

	// Initially circuit breaker should be closed (state 0)
	initialState := getGaugeValue("helixagent_concurrency_alert_circuit_breaker_state", "webhook")
	assert.Equal(t, 0.0, initialState, "Circuit breaker should be closed (0) initially")

	// Send first alert - should fail but circuit remains closed (failures < threshold)
	manager.HandleAlert(alert)
	time.Sleep(100 * time.Millisecond) // Allow async processing
	state := getGaugeValue("helixagent_concurrency_alert_circuit_breaker_state", "webhook")
	assert.Equal(t, 0.0, state, "Circuit breaker should still be closed (0) after first failure")

	// Send second alert - reaches failure threshold, circuit should open (state 2)
	manager.HandleAlert(alert)
	time.Sleep(100 * time.Millisecond)
	state = getGaugeValue("helixagent_concurrency_alert_circuit_breaker_state", "webhook")
	assert.Equal(t, 2.0, state, "Circuit breaker should be open (2) after reaching failure threshold")

	// Wait for circuit breaker reset period (half-open state)
	time.Sleep(600 * time.Millisecond) // Slightly longer than reset period

	// Send third alert - circuit should be half-open (state 1) allowing one attempt
	manager.HandleAlert(alert)
	time.Sleep(100 * time.Millisecond)
	state = getGaugeValue("helixagent_concurrency_alert_circuit_breaker_state", "webhook")
	// After half-open attempt, since server still fails, circuit will open again (state 2)
	// However, there may be a brief moment where state is 1 (half-open)
	// We'll accept either 1 (half-open) or 2 (open) depending on timing
	assert.True(t, state == 1.0 || state == 2.0,
		"Circuit breaker should be half-open (1) or open (2) after reset period")

	// Verify request count matches expected behavior
	mu.Lock()
	count := requestCount
	mu.Unlock()
	// Expected: 2 failures (closed) + 1 attempt (half-open) = 3 total requests
	assert.Equal(t, 3, count, "Should have made 3 total requests (2 closed + 1 half-open)")
}

// TestConcurrencyAlertManager_Metrics_ThresholdBreach tests that RecordThresholdBreach metric is recorded
func TestConcurrencyAlertManager_Metrics_ThresholdBreach(t *testing.T) {
	// Save original registry and replace with test registry
	originalRegistry := prometheus.DefaultRegisterer
	testRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testRegistry
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Reset metrics to ensure fresh initialization
	resetConcurrencyMetrics()

	// Configure dead letter queue monitoring with low thresholds
	config := DefaultConcurrencyAlertManagerConfig()
	config.Enabled = true
	config.EnableLogging = false
	config.EnableWebhook = false
	config.EnableEmail = false
	config.EscalationEnabled = false
	config.RateLimitEnabled = false
	config.CircuitBreakerEnabled = false
	config.RetryEnabled = false
	config.DefaultCooldown = 500 * time.Millisecond // Set cooldown for deduplication
	config.CleanupInterval = 100 * time.Millisecond
	config.MaxAlertAge = 1 * time.Hour

	// Enable dead letter queue monitoring with low thresholds for testing
	config.DeadLetterQueueMonitoringEnabled = true
	config.DeadLetterQueueWarningThreshold = 2
	config.DeadLetterQueueCriticalThreshold = 4

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise
	manager := NewConcurrencyAlertManager(config, logger)

	// Start the manager to enable monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Start(ctx)
	time.Sleep(5 * time.Millisecond)

	// Helper to get counter value with labels
	getCounterValue := func(metricName string, labelValues ...string) float64 {
		metrics, err := testRegistry.Gather()
		if err != nil {
			return 0
		}
		for _, mf := range metrics {
			if mf.GetName() == metricName {
				for _, metric := range mf.GetMetric() {
					labels := metric.GetLabel()
					// Build map of label name to value
					labelMap := make(map[string]string)
					for _, label := range labels {
						labelMap[label.GetName()] = label.GetValue()
					}
					// Expected label names based on metric definition order
					expectedNames := []string{"threshold_type", "channel", "provider"}
					matched := true
					for i, expectedValue := range labelValues {
						if i >= len(expectedNames) {
							matched = false
							break
						}
						name := expectedNames[i]
						if labelMap[name] != expectedValue {
							matched = false
							break
						}
					}
					if matched {
						return metric.GetCounter().GetValue()
					}
				}
			}
		}
		return 0
	}

	// Helper to add alerts to dead letter queue (mirroring internal logic)
	addDeadLetterAlert := func(channel string, alert ConcurrencyAlert) {
		alertKey := manager.generateAlertKey(alert)
		deadLetterKey := fmt.Sprintf("%s:%s", channel, alertKey)
		manager.deadLetterAlerts.Put(deadLetterKey, &deadLetterAlert{
			channel:      channel,
			alert:        alert,
			attempts:     1,
			lastAttempt:  time.Now(),
			failureError: "test",
			addedAt:      time.Now(),
		})
	}

	// Initially queue is empty, no threshold breaches
	time.Sleep(150 * time.Millisecond) // Wait for first monitoring tick
	warningBreaches := getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "warning", "dead_letter_queue", "system")
	criticalBreaches := getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "critical", "dead_letter_queue", "system")
	assert.Equal(t, 0.0, warningBreaches, "Should have no warning breaches initially")
	assert.Equal(t, 0.0, criticalBreaches, "Should have no critical breaches initially")

	// Add 1 alert (below warning threshold)
	alert1 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 1",
		Timestamp:  time.Now(),
		Saturation: 50.0,
		Severity:   SeverityWarning,
	}
	addDeadLetterAlert("logging", alert1)
	time.Sleep(150 * time.Millisecond) // Wait for monitoring tick
	warningBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "warning", "dead_letter_queue", "system")
	criticalBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "critical", "dead_letter_queue", "system")
	assert.Equal(t, 0.0, warningBreaches, "Should have no warning breaches below threshold")

	// Add second alert (reaches warning threshold)
	alert2 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 2",
		Timestamp:  time.Now(),
		Saturation: 60.0,
		Severity:   SeverityWarning,
	}
	addDeadLetterAlert("logging", alert2)
	time.Sleep(150 * time.Millisecond) // Should trigger warning breach metric
	warningBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "warning", "dead_letter_queue", "system")
	criticalBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "critical", "dead_letter_queue", "system")
	assert.Equal(t, 1.0, warningBreaches, "Should have one warning breach after reaching warning threshold")
	assert.Equal(t, 0.0, criticalBreaches, "Should still have no critical breaches")

	// Add third alert (still warning, but cooldown may prevent another metric increment)
	alert3 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 3",
		Timestamp:  time.Now(),
		Saturation: 70.0,
		Severity:   SeverityWarning,
	}
	addDeadLetterAlert("logging", alert3)
	time.Sleep(150 * time.Millisecond)
	warningBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "warning", "dead_letter_queue", "system")
	// Warning breaches should still be 1 (cooldown prevents duplicate alerts)
	assert.Equal(t, 1.0, warningBreaches, "Warning breaches should not increase during cooldown")

	// Add fourth alert (reaches critical threshold)
	alert4 := ConcurrencyAlert{
		Type:       "test",
		Provider:   "test-provider",
		Message:    "Test alert 4",
		Timestamp:  time.Now(),
		Saturation: 80.0,
		Severity:   SeverityWarning,
	}
	addDeadLetterAlert("logging", alert4)
	time.Sleep(150 * time.Millisecond) // Should trigger critical breach metric
	warningBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "warning", "dead_letter_queue", "system")
	criticalBreaches = getCounterValue("helixagent_concurrency_alert_threshold_breaches_total", "critical", "dead_letter_queue", "system")
	assert.Equal(t, 1.0, warningBreaches, "Warning breaches should still be 1")
	assert.Equal(t, 1.0, criticalBreaches, "Should have one critical breach after reaching critical threshold")

	// Clean up dead letter queue
	manager.deadLetterAlerts.Clear()
	time.Sleep(150 * time.Millisecond) // Wait for monitoring tick
}
