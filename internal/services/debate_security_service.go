package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SecurityConfig holds security configuration
type SecurityConfig struct {
	MaxPromptLength      int
	MaxResponseLength    int
	BlockedPatterns      []string
	SensitivePatterns    []string
	RateLimitRequests    int
	RateLimitWindow      time.Duration
	AuditEnabled         bool
	ContentFilterEnabled bool
	PIIDetectionEnabled  bool
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxPromptLength:      100000,
		MaxResponseLength:    500000,
		BlockedPatterns:      []string{},
		SensitivePatterns:    []string{`\b\d{3}-\d{2}-\d{4}\b`, `\b\d{16}\b`}, // SSN, credit card patterns
		RateLimitRequests:    100,
		RateLimitWindow:      time.Minute,
		AuditEnabled:         true,
		ContentFilterEnabled: true,
		PIIDetectionEnabled:  true,
	}
}

// SecurityViolation represents a detected security issue
type SecurityViolation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`     // validation, content, pii, rate_limit
	Severity    string    `json:"severity"` // low, medium, high, critical
	Description string    `json:"description"`
	DebateID    string    `json:"debate_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID        string                 `json:"id"`
	DebateID  string                 `json:"debate_id"`
	Action    string                 `json:"action"`
	Actor     string                 `json:"actor,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Hash      string                 `json:"hash"` // Integrity hash
}

// RateLimitEntry tracks rate limiting
type RateLimitEntry struct {
	Key         string
	Count       int
	WindowStart time.Time
}

// DebateSecurityService provides security capabilities.
//
// Concurrent-safe by construction (CONST-029):
//   - violations / auditLog are safe.Slice[T]; Append, Snapshot, Len, Replace
//     are the only entry points.
//   - rateLimiter is safe.Store[string, RateLimitEntry]. Storing by value
//     (not *RateLimitEntry) lets Update atomically increment Count via the
//     single-write-lock read-modify-write path; without by-value semantics
//     the old code handed out pointers that were mutated outside the lock.
//   - blockedPatterns / sensitivePatterns are set once in the constructor
//     and never mutated, so they stay as bare slices (no mutex adjacency).
type DebateSecurityService struct {
	logger            *logrus.Logger
	config            *SecurityConfig
	violations        *safe.Slice[SecurityViolation]
	auditLog          *safe.Slice[AuditEntry]
	rateLimiter       *safe.Store[string, RateLimitEntry]
	blockedPatterns   []*regexp.Regexp
	sensitivePatterns []*regexp.Regexp
}

// NewDebateSecurityService creates a new security service
func NewDebateSecurityService(logger *logrus.Logger) *DebateSecurityService {
	return NewDebateSecurityServiceWithConfig(logger, DefaultSecurityConfig())
}

// NewDebateSecurityServiceWithConfig creates a security service with custom config
func NewDebateSecurityServiceWithConfig(logger *logrus.Logger, config *SecurityConfig) *DebateSecurityService {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	svc := &DebateSecurityService{
		logger:      logger,
		config:      config,
		violations:  safe.NewSlice[SecurityViolation](),
		auditLog:    safe.NewSlice[AuditEntry](),
		rateLimiter: safe.NewStore[string, RateLimitEntry](),
	}

	// Compile blocked patterns
	svc.blockedPatterns = make([]*regexp.Regexp, 0, len(config.BlockedPatterns))
	for _, pattern := range config.BlockedPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			svc.blockedPatterns = append(svc.blockedPatterns, re)
		}
	}

	// Compile sensitive patterns
	svc.sensitivePatterns = make([]*regexp.Regexp, 0, len(config.SensitivePatterns))
	for _, pattern := range config.SensitivePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			svc.sensitivePatterns = append(svc.sensitivePatterns, re)
		}
	}

	return svc
}

// ValidateDebateRequest validates a debate request
func (dss *DebateSecurityService) ValidateDebateRequest(ctx context.Context, config *DebateConfig) error {
	if config == nil {
		return dss.recordViolation("validation", "high", "Debate config is nil", "")
	}

	// Validate debate ID
	if config.DebateID == "" {
		return dss.recordViolation("validation", "medium", "Debate ID is empty", "")
	}

	// Validate topic length
	if len(config.Topic) > dss.config.MaxPromptLength {
		return dss.recordViolation("validation", "medium",
			fmt.Sprintf("Topic exceeds maximum length (%d > %d)", len(config.Topic), dss.config.MaxPromptLength),
			config.DebateID)
	}

	// Validate max rounds
	if config.MaxRounds <= 0 || config.MaxRounds > 100 {
		return dss.recordViolation("validation", "low",
			fmt.Sprintf("Invalid max rounds: %d (must be 1-100)", config.MaxRounds),
			config.DebateID)
	}

	// Validate participants
	if len(config.Participants) == 0 {
		return dss.recordViolation("validation", "medium", "No participants configured", config.DebateID)
	}

	for _, p := range config.Participants {
		if p.ParticipantID == "" || p.Name == "" {
			return dss.recordViolation("validation", "medium",
				fmt.Sprintf("Invalid participant: ID=%s, Name=%s", p.ParticipantID, p.Name),
				config.DebateID)
		}
	}

	// Content filtering on topic
	if dss.config.ContentFilterEnabled {
		if err := dss.checkBlockedContent(config.Topic, config.DebateID); err != nil {
			return err
		}
	}

	// PII detection on topic
	if dss.config.PIIDetectionEnabled {
		if err := dss.checkPII(config.Topic, config.DebateID); err != nil {
			return err
		}
	}

	// Audit the validation
	if dss.config.AuditEnabled {
		_ = dss.audit(config.DebateID, "validate_request", "", map[string]interface{}{ //nolint:errcheck
			"topic_length": len(config.Topic),
			"max_rounds":   config.MaxRounds,
			"participants": len(config.Participants),
		})
	}

	dss.logger.WithFields(logrus.Fields{
		"debate_id":    config.DebateID,
		"participants": len(config.Participants),
	}).Debug("Validated debate request")

	return nil
}

// SanitizeResponse sanitizes a response to remove sensitive content
func (dss *DebateSecurityService) SanitizeResponse(ctx context.Context, response string) (string, error) {
	if response == "" {
		return response, nil
	}

	// Check response length
	if len(response) > dss.config.MaxResponseLength {
		_ = dss.recordViolation("validation", "low", //nolint:errcheck
			fmt.Sprintf("Response exceeds maximum length (%d > %d)", len(response), dss.config.MaxResponseLength),
			"")
		response = response[:dss.config.MaxResponseLength]
	}

	sanitized := response

	// Remove blocked patterns
	if dss.config.ContentFilterEnabled {
		for _, re := range dss.blockedPatterns {
			sanitized = re.ReplaceAllString(sanitized, "[FILTERED]")
		}
	}

	// Mask PII
	if dss.config.PIIDetectionEnabled {
		for _, re := range dss.sensitivePatterns {
			sanitized = re.ReplaceAllStringFunc(sanitized, func(match string) string {
				if len(match) > 4 {
					return strings.Repeat("*", len(match)-4) + match[len(match)-4:]
				}
				return strings.Repeat("*", len(match))
			})
		}
	}

	// Log if content was modified
	if sanitized != response {
		dss.logger.Debug("Response content was sanitized")
		_ = dss.recordViolation("content", "low", "Response content was sanitized", "") //nolint:errcheck
	}

	return sanitized, nil
}

// AuditDebate creates an audit entry for a debate
func (dss *DebateSecurityService) AuditDebate(ctx context.Context, debateID string) error {
	if !dss.config.AuditEnabled {
		return nil
	}

	return dss.audit(debateID, "audit_debate", "", map[string]interface{}{
		"timestamp": time.Now(),
	})
}

// AuditAction creates an audit entry for a specific action
func (dss *DebateSecurityService) AuditAction(ctx context.Context, debateID, action, actor string, details map[string]interface{}) error {
	if !dss.config.AuditEnabled {
		return nil
	}

	return dss.audit(debateID, action, actor, details)
}

// audit creates an audit log entry
func (dss *DebateSecurityService) audit(debateID, action, actor string, details map[string]interface{}) error {
	entry := AuditEntry{
		ID:        "audit-" + uuid.New().String()[:8],
		DebateID:  debateID,
		Action:    action,
		Actor:     actor,
		Details:   details,
		Timestamp: time.Now(),
	}

	// Calculate integrity hash
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%v", entry.ID, entry.DebateID, entry.Action, entry.Timestamp, entry.Details)
	hash := sha256.Sum256([]byte(hashInput))
	entry.Hash = hex.EncodeToString(hash[:])

	dss.auditLog.Append(entry)

	dss.logger.WithFields(logrus.Fields{
		"audit_id":  entry.ID,
		"debate_id": debateID,
		"action":    action,
	}).Debug("Audit entry created")

	return nil
}

// recordViolation records a security violation
func (dss *DebateSecurityService) recordViolation(violationType, severity, description, debateID string) error {
	violation := SecurityViolation{
		ID:          "viol-" + uuid.New().String()[:8],
		Type:        violationType,
		Severity:    severity,
		Description: description,
		DebateID:    debateID,
		Timestamp:   time.Now(),
	}

	dss.violations.Append(violation)

	dss.logger.WithFields(logrus.Fields{
		"violation_id": violation.ID,
		"type":         violationType,
		"severity":     severity,
		"debate_id":    debateID,
	}).Warn("Security violation recorded: " + description)

	return fmt.Errorf("security violation: %s", description)
}

// checkBlockedContent checks for blocked content patterns
func (dss *DebateSecurityService) checkBlockedContent(content, debateID string) error {
	for _, re := range dss.blockedPatterns {
		if re.MatchString(content) {
			return dss.recordViolation("content", "high", "Blocked content pattern detected", debateID)
		}
	}
	return nil
}

// checkPII checks for PII patterns
func (dss *DebateSecurityService) checkPII(content, debateID string) error {
	for _, re := range dss.sensitivePatterns {
		if re.MatchString(content) {
			dss.logger.WithField("debate_id", debateID).Warn("PII pattern detected in content")
			// Don't block, just warn - PII will be masked during sanitization
		}
	}
	return nil
}

// CheckRateLimit checks if a request should be rate limited.
//
// The read-modify-write against rateLimiter is collapsed into a single
// Update callback so concurrent calls for the same key cannot race on
// Count. recordViolation runs outside the Update closure to keep
// cross-store locking strictly linear.
func (dss *DebateSecurityService) CheckRateLimit(ctx context.Context, key string) error {
	now := time.Now()
	var exceededCount int
	dss.rateLimiter.Update(key, func(cur RateLimitEntry, present bool) (RateLimitEntry, bool) {
		if !present || now.Sub(cur.WindowStart) > dss.config.RateLimitWindow {
			return RateLimitEntry{Key: key, Count: 1, WindowStart: now}, true
		}
		cur.Count++
		if cur.Count > dss.config.RateLimitRequests {
			exceededCount = cur.Count
		}
		return cur, true
	})

	if exceededCount > 0 {
		return dss.recordViolation("rate_limit", "medium",
			fmt.Sprintf("Rate limit exceeded for key: %s (%d requests in %v)",
				key, exceededCount, dss.config.RateLimitWindow),
			"")
	}

	return nil
}

// GetViolations returns all recorded violations
func (dss *DebateSecurityService) GetViolations() []SecurityViolation {
	return dss.violations.Snapshot()
}

// GetViolationsByDebate returns violations for a specific debate
func (dss *DebateSecurityService) GetViolationsByDebate(debateID string) []SecurityViolation {
	violations := make([]SecurityViolation, 0)
	for _, v := range dss.violations.Snapshot() {
		if v.DebateID == debateID {
			violations = append(violations, v)
		}
	}
	return violations
}

// GetAuditLog returns the audit log
func (dss *DebateSecurityService) GetAuditLog() []AuditEntry {
	return dss.auditLog.Snapshot()
}

// GetAuditLogByDebate returns audit entries for a specific debate
func (dss *DebateSecurityService) GetAuditLogByDebate(debateID string) []AuditEntry {
	entries := make([]AuditEntry, 0)
	for _, e := range dss.auditLog.Snapshot() {
		if e.DebateID == debateID {
			entries = append(entries, e)
		}
	}
	return entries
}

// VerifyAuditIntegrity verifies the integrity of audit entries
func (dss *DebateSecurityService) VerifyAuditIntegrity() (bool, []string) {
	invalidEntries := make([]string, 0)

	for _, entry := range dss.auditLog.Snapshot() {
		hashInput := fmt.Sprintf("%s|%s|%s|%s|%v", entry.ID, entry.DebateID, entry.Action, entry.Timestamp, entry.Details)
		hash := sha256.Sum256([]byte(hashInput))
		expectedHash := hex.EncodeToString(hash[:])

		if entry.Hash != expectedHash {
			invalidEntries = append(invalidEntries, entry.ID)
		}
	}

	return len(invalidEntries) == 0, invalidEntries
}

// ClearViolations clears all recorded violations
func (dss *DebateSecurityService) ClearViolations() {
	dss.violations.Clear()
}

// GetStats returns security service statistics
func (dss *DebateSecurityService) GetStats() map[string]interface{} {
	// Single Snapshot so the severity histogram is consistent with the count.
	violationSnapshot := dss.violations.Snapshot()

	severityCounts := map[string]int{
		"low":      0,
		"medium":   0,
		"high":     0,
		"critical": 0,
	}
	for _, v := range violationSnapshot {
		severityCounts[v.Severity]++
	}

	return map[string]interface{}{
		"total_violations":       len(violationSnapshot),
		"audit_entries":          dss.auditLog.Len(),
		"violations_by_severity": severityCounts,
		"audit_enabled":          dss.config.AuditEnabled,
		"content_filter":         dss.config.ContentFilterEnabled,
		"pii_detection":          dss.config.PIIDetectionEnabled,
	}
}
