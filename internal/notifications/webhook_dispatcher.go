package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// WebhookDispatcher handles webhook delivery with retry logic
type WebhookDispatcher struct {
	// Registered webhooks
	webhooks   map[string]*WebhookRegistration
	webhooksMu sync.RWMutex

	// Delivery queue
	deliveryQueue chan *WebhookDelivery

	// Configuration
	config *WebhookConfig

	// HTTP client
	client *http.Client

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	deliveredCount int64
	failedCount    int64
	metricsMu      sync.RWMutex
}

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	MaxRetries     int           `yaml:"max_retries"`
	RetryBackoff   time.Duration `yaml:"retry_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
	Timeout        time.Duration `yaml:"timeout"`
	WorkerCount    int           `yaml:"worker_count"`
	QueueSize      int           `yaml:"queue_size"`
	SignatureHeader string       `yaml:"signature_header"`
}

// DefaultWebhookConfig returns default webhook configuration
func DefaultWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		MaxRetries:     5,
		RetryBackoff:   time.Second,
		MaxBackoff:     5 * time.Minute,
		Timeout:        30 * time.Second,
		WorkerCount:    5,
		QueueSize:      1000,
		SignatureHeader: "X-HelixAgent-Signature",
	}
}

// WebhookRegistration represents a registered webhook
type WebhookRegistration struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	Secret      string            `json:"secret,omitempty"`
	Events      []string          `json:"events,omitempty"`      // Empty = all events
	TaskTypes   []string          `json:"task_types,omitempty"`  // Empty = all task types
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	LastSuccess *time.Time        `json:"last_success,omitempty"`
	LastFailure *time.Time        `json:"last_failure,omitempty"`
	FailCount   int               `json:"fail_count"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID             string                 `json:"id"`
	WebhookID      string                 `json:"webhook_id"`
	TaskID         string                 `json:"task_id"`
	Event          string                 `json:"event"`
	Payload        map[string]interface{} `json:"payload"`
	RetryCount     int                    `json:"retry_count"`
	ScheduledAt    time.Time              `json:"scheduled_at"`
	DeliveredAt    *time.Time             `json:"delivered_at,omitempty"`
	ResponseStatus int                    `json:"response_status,omitempty"`
	ResponseBody   string                 `json:"response_body,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// NewWebhookDispatcher creates a new webhook dispatcher
func NewWebhookDispatcher(config *WebhookConfig, logger *logrus.Logger) *WebhookDispatcher {
	if config == nil {
		config = DefaultWebhookConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	dispatcher := &WebhookDispatcher{
		webhooks:      make(map[string]*WebhookRegistration),
		deliveryQueue: make(chan *WebhookDelivery, config.QueueSize),
		config:        config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start delivery workers
	for i := 0; i < config.WorkerCount; i++ {
		dispatcher.wg.Add(1)
		go dispatcher.deliveryWorker()
	}

	return dispatcher
}

// Start starts the webhook dispatcher
func (d *WebhookDispatcher) Start() error {
	d.logger.Info("Webhook dispatcher started")
	return nil
}

// Stop stops the webhook dispatcher
func (d *WebhookDispatcher) Stop() error {
	d.logger.Info("Stopping webhook dispatcher")
	d.cancel()
	close(d.deliveryQueue)
	d.wg.Wait()
	return nil
}

// RegisterWebhook registers a new webhook
func (d *WebhookDispatcher) RegisterWebhook(webhook *WebhookRegistration) error {
	if webhook.ID == "" {
		webhook.ID = uuid.New().String()
	}
	if webhook.CreatedAt.IsZero() {
		webhook.CreatedAt = time.Now()
	}
	webhook.Enabled = true

	d.webhooksMu.Lock()
	d.webhooks[webhook.ID] = webhook
	d.webhooksMu.Unlock()

	d.logger.WithFields(logrus.Fields{
		"webhook_id": webhook.ID,
		"url":        webhook.URL,
	}).Info("Webhook registered")

	return nil
}

// UnregisterWebhook removes a webhook
func (d *WebhookDispatcher) UnregisterWebhook(id string) error {
	d.webhooksMu.Lock()
	delete(d.webhooks, id)
	d.webhooksMu.Unlock()

	d.logger.WithField("webhook_id", id).Info("Webhook unregistered")
	return nil
}

// GetWebhook returns a webhook by ID
func (d *WebhookDispatcher) GetWebhook(id string) (*WebhookRegistration, bool) {
	d.webhooksMu.RLock()
	defer d.webhooksMu.RUnlock()

	webhook, exists := d.webhooks[id]
	return webhook, exists
}

// ListWebhooks returns all registered webhooks
func (d *WebhookDispatcher) ListWebhooks() []*WebhookRegistration {
	d.webhooksMu.RLock()
	defer d.webhooksMu.RUnlock()

	webhooks := make([]*WebhookRegistration, 0, len(d.webhooks))
	for _, webhook := range d.webhooks {
		webhooks = append(webhooks, webhook)
	}
	return webhooks
}

// Dispatch dispatches a notification to matching webhooks
func (d *WebhookDispatcher) Dispatch(notification *TaskNotification) {
	d.webhooksMu.RLock()
	webhooks := make([]*WebhookRegistration, 0)
	for _, webhook := range d.webhooks {
		if d.matchesWebhook(webhook, notification) {
			webhooks = append(webhooks, webhook)
		}
	}
	d.webhooksMu.RUnlock()

	for _, webhook := range webhooks {
		delivery := &WebhookDelivery{
			ID:          uuid.New().String(),
			WebhookID:   webhook.ID,
			TaskID:      notification.TaskID,
			Event:       notification.EventType,
			Payload:     d.buildPayload(notification),
			ScheduledAt: time.Now(),
		}

		select {
		case d.deliveryQueue <- delivery:
		default:
			d.logger.WithField("webhook_id", webhook.ID).Warn("Delivery queue full, dropping webhook")
		}
	}
}

// matchesWebhook checks if a notification matches a webhook's filters
func (d *WebhookDispatcher) matchesWebhook(webhook *WebhookRegistration, notification *TaskNotification) bool {
	if !webhook.Enabled {
		return false
	}

	// Check event filter
	if len(webhook.Events) > 0 {
		matched := false
		for _, event := range webhook.Events {
			if event == notification.EventType || event == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check task type filter
	if len(webhook.TaskTypes) > 0 && notification.Task != nil {
		matched := false
		for _, taskType := range webhook.TaskTypes {
			if taskType == notification.Task.TaskType || taskType == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// buildPayload builds the webhook payload
func (d *WebhookDispatcher) buildPayload(notification *TaskNotification) map[string]interface{} {
	payload := map[string]interface{}{
		"event":     notification.EventType,
		"task_id":   notification.TaskID,
		"timestamp": notification.Timestamp.Format(time.RFC3339),
	}

	if notification.Data != nil {
		payload["data"] = notification.Data
	}

	if notification.Task != nil {
		payload["task"] = map[string]interface{}{
			"id":       notification.Task.ID,
			"type":     notification.Task.TaskType,
			"name":     notification.Task.TaskName,
			"status":   notification.Task.Status,
			"progress": notification.Task.Progress,
		}
	}

	return payload
}

// deliveryWorker processes webhook deliveries
func (d *WebhookDispatcher) deliveryWorker() {
	defer d.wg.Done()

	for delivery := range d.deliveryQueue {
		d.deliver(delivery)
	}
}

// deliver attempts to deliver a webhook
func (d *WebhookDispatcher) deliver(delivery *WebhookDelivery) {
	d.webhooksMu.RLock()
	webhook, exists := d.webhooks[delivery.WebhookID]
	d.webhooksMu.RUnlock()

	if !exists {
		return
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(delivery.Payload)
	if err != nil {
		d.logger.WithError(err).Error("Failed to serialize webhook payload")
		return
	}

	// Create request
	req, err := http.NewRequestWithContext(d.ctx, "POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		d.logger.WithError(err).Error("Failed to create webhook request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HelixAgent-Event", delivery.Event)
	req.Header.Set("X-HelixAgent-Delivery", delivery.ID)
	req.Header.Set("User-Agent", "HelixAgent-Webhook/1.0")

	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Add signature if secret is configured
	if webhook.Secret != "" {
		signature := d.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set(d.config.SignatureHeader, signature)
	}

	// Send request
	resp, err := d.client.Do(req)
	if err != nil {
		d.handleDeliveryFailure(delivery, webhook, err.Error(), 0)
		return
	}
	defer resp.Body.Close()

	// Read response body (limited to 1KB)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	delivery.ResponseStatus = resp.StatusCode
	delivery.ResponseBody = string(respBody)

	// Check if successful
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		d.handleDeliverySuccess(delivery, webhook)
	} else {
		d.handleDeliveryFailure(delivery, webhook, fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode)
	}
}

// handleDeliverySuccess handles successful webhook delivery
func (d *WebhookDispatcher) handleDeliverySuccess(delivery *WebhookDelivery, webhook *WebhookRegistration) {
	now := time.Now()
	delivery.DeliveredAt = &now

	d.webhooksMu.Lock()
	webhook.LastSuccess = &now
	webhook.FailCount = 0
	d.webhooksMu.Unlock()

	d.metricsMu.Lock()
	d.deliveredCount++
	d.metricsMu.Unlock()

	d.logger.WithFields(logrus.Fields{
		"webhook_id":  webhook.ID,
		"delivery_id": delivery.ID,
		"status":      delivery.ResponseStatus,
	}).Debug("Webhook delivered successfully")
}

// handleDeliveryFailure handles failed webhook delivery
func (d *WebhookDispatcher) handleDeliveryFailure(delivery *WebhookDelivery, webhook *WebhookRegistration, errorMsg string, status int) {
	delivery.Error = errorMsg

	d.webhooksMu.Lock()
	now := time.Now()
	webhook.LastFailure = &now
	webhook.FailCount++

	// Disable webhook after too many failures
	if webhook.FailCount > 10 {
		webhook.Enabled = false
		d.logger.WithField("webhook_id", webhook.ID).Warn("Webhook disabled due to repeated failures")
	}
	d.webhooksMu.Unlock()

	// Retry if possible
	if delivery.RetryCount < d.config.MaxRetries {
		delivery.RetryCount++
		backoff := d.calculateBackoff(delivery.RetryCount)
		delivery.ScheduledAt = time.Now().Add(backoff)

		// Re-queue with delay
		go func() {
			select {
			case <-d.ctx.Done():
				return
			case <-time.After(backoff):
				select {
				case d.deliveryQueue <- delivery:
				default:
				}
			}
		}()

		d.logger.WithFields(logrus.Fields{
			"webhook_id":  webhook.ID,
			"delivery_id": delivery.ID,
			"retry_count": delivery.RetryCount,
			"backoff":     backoff,
		}).Debug("Scheduling webhook retry")
	} else {
		d.metricsMu.Lock()
		d.failedCount++
		d.metricsMu.Unlock()

		d.logger.WithFields(logrus.Fields{
			"webhook_id":  webhook.ID,
			"delivery_id": delivery.ID,
			"error":       errorMsg,
		}).Warn("Webhook delivery failed after max retries")
	}
}

// calculateBackoff calculates exponential backoff
func (d *WebhookDispatcher) calculateBackoff(retryCount int) time.Duration {
	backoff := d.config.RetryBackoff * time.Duration(1<<uint(retryCount-1))
	if backoff > d.config.MaxBackoff {
		backoff = d.config.MaxBackoff
	}
	return backoff
}

// generateSignature generates HMAC-SHA256 signature
func (d *WebhookDispatcher) generateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// GetStats returns webhook dispatcher statistics
func (d *WebhookDispatcher) GetStats() map[string]interface{} {
	d.metricsMu.RLock()
	delivered := d.deliveredCount
	failed := d.failedCount
	d.metricsMu.RUnlock()

	d.webhooksMu.RLock()
	webhookCount := len(d.webhooks)
	d.webhooksMu.RUnlock()

	return map[string]interface{}{
		"webhooks_registered": webhookCount,
		"deliveries_success":  delivered,
		"deliveries_failed":   failed,
		"queue_size":          len(d.deliveryQueue),
	}
}

// LoadWebhooksFromTask loads webhooks from task notification config
func (d *WebhookDispatcher) LoadWebhooksFromTask(task *models.BackgroundTask) {
	if len(task.NotificationConfig.Webhooks) == 0 {
		return
	}

	for _, whConfig := range task.NotificationConfig.Webhooks {
		webhook := &WebhookRegistration{
			ID:        uuid.New().String(),
			URL:       whConfig.URL,
			Secret:    whConfig.Secret,
			Events:    whConfig.Events,
			Headers:   whConfig.Headers,
			Enabled:   true,
			CreatedAt: time.Now(),
		}
		d.RegisterWebhook(webhook)
	}
}

// WebhookSubscriber implements the Subscriber interface for webhooks
type WebhookSubscriber struct {
	id         string
	taskID     string
	dispatcher *WebhookDispatcher
	webhook    *WebhookRegistration
	active     bool
	mu         sync.RWMutex
}

// NewWebhookSubscriber creates a new webhook subscriber
func NewWebhookSubscriber(id, taskID string, dispatcher *WebhookDispatcher, webhook *WebhookRegistration) *WebhookSubscriber {
	return &WebhookSubscriber{
		id:         id,
		taskID:     taskID,
		dispatcher: dispatcher,
		webhook:    webhook,
		active:     true,
	}
}

func (s *WebhookSubscriber) Notify(ctx context.Context, notification *TaskNotification) error {
	s.dispatcher.Dispatch(notification)
	return nil
}

func (s *WebhookSubscriber) Type() NotificationType {
	return NotificationTypeWebhook
}

func (s *WebhookSubscriber) ID() string {
	return s.id
}

func (s *WebhookSubscriber) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *WebhookSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	return nil
}
