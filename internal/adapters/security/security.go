// Package security provides adapter types to bridge between HelixAgent's
// internal security types and the extracted digital.vasic.security module.
// This allows HelixAgent to use the generic security module while maintaining
// its existing API contracts.
package security

import (
	"context"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	extsecurity "digital.vasic.security/pkg/guardrails"
	extpii "digital.vasic.security/pkg/pii"
	extpolicy "digital.vasic.security/pkg/policy"
)

// Re-export types from internal/security for backward compatibility
// These types are HelixAgent-specific and not in the extracted module

// AttackType represents a category of security attack
type AttackType string

// Attack types constants
const (
	AttackTypeDirectPromptInjection   AttackType = "direct_prompt_injection"
	AttackTypeIndirectPromptInjection AttackType = "indirect_prompt_injection"
	AttackTypeJailbreak               AttackType = "jailbreak"
	AttackTypeRoleplay                AttackType = "roleplay_injection"
	AttackTypeDataLeakage             AttackType = "data_leakage"
	AttackTypeSystemPromptLeakage     AttackType = "system_prompt_leakage"
	AttackTypePIIExtraction           AttackType = "pii_extraction"
	AttackTypeModelExtraction         AttackType = "model_extraction"
	AttackTypeResourceExhaustion      AttackType = "resource_exhaustion"
	AttackTypeInfiniteLoop            AttackType = "infinite_loop"
	AttackTypeTokenOverflow           AttackType = "token_overflow"
	AttackTypeHarmfulContent          AttackType = "harmful_content"
	AttackTypeHateSpeech              AttackType = "hate_speech"
	AttackTypeViolentContent          AttackType = "violent_content"
	AttackTypeSexualContent           AttackType = "sexual_content"
	AttackTypeIllegalActivities       AttackType = "illegal_activities"
	AttackTypeManipulation            AttackType = "manipulation"
	AttackTypeDeception               AttackType = "deception"
	AttackTypeImpersonation           AttackType = "impersonation"
	AttackTypeAuthorityAbuse          AttackType = "authority_abuse"
	AttackTypeCodeInjection           AttackType = "code_injection"
	AttackTypeSQLInjection            AttackType = "sql_injection"
	AttackTypeCommandInjection        AttackType = "command_injection"
	AttackTypeXSS                     AttackType = "xss"
	AttackTypeBiasExploitation        AttackType = "bias_exploitation"
	AttackTypeStereotyping            AttackType = "stereotyping"
	AttackTypeDiscrimination          AttackType = "discrimination"
	AttackTypeHallucinationInduction  AttackType = "hallucination_induction"
	AttackTypeConfabulationTrigger    AttackType = "confabulation_trigger"
	AttackTypeFalseCitation           AttackType = "false_citation"
	AttackTypeModelPoisoning          AttackType = "model_poisoning"
	AttackTypeDataPoisoning           AttackType = "data_poisoning"
	AttackTypeDependencyAttack        AttackType = "dependency_attack"
	AttackTypeEncoding                AttackType = "encoding_evasion"
	AttackTypeObfuscation             AttackType = "obfuscation"
	AttackTypeFragmentation           AttackType = "fragmentation"
	AttackTypeMultilingual            AttackType = "multilingual_evasion"
)

// Severity indicates the severity level of a security issue
type Severity string

// Severity levels
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Attack represents a security attack test case
type Attack struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        AttackType             `json:"type"`
	Description string                 `json:"description"`
	Payload     string                 `json:"payload"`
	Variations  []string               `json:"variations,omitempty"`
	Severity    Severity               `json:"severity"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DebateEvaluation holds the result of debate-based security evaluation
type DebateEvaluation struct {
	IsVulnerable  bool     `json:"is_vulnerable"`
	Confidence    float64  `json:"confidence"`
	Reasoning     string   `json:"reasoning"`
	ConsensusType string   `json:"consensus_type"`
	Participants  []string `json:"participants,omitempty"`
}

// ContentEvaluation holds the result of content safety evaluation
type ContentEvaluation struct {
	IsSafe       bool     `json:"is_safe"`
	Confidence   float64  `json:"confidence"`
	Categories   []string `json:"categories,omitempty"`
	Reasoning    string   `json:"reasoning"`
	Participants []string `json:"participants,omitempty"`
}

// AuditEventType indicates the type of audit event
type AuditEventType string

// Audit event types
const (
	AuditEventToolCall       AuditEventType = "tool_call"
	AuditEventGuardrailBlock AuditEventType = "guardrail_block"
	AuditEventAttackDetected AuditEventType = "attack_detected"
	AuditEventPIIAccess      AuditEventType = "pii_access"
	AuditEventPermissionDeny AuditEventType = "permission_deny"
	AuditEventRateLimit      AuditEventType = "rate_limit"
	AuditEventAuthentication AuditEventType = "authentication"
)

// AuditEvent represents a security audit event
type AuditEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType AuditEventType         `json:"event_type"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource,omitempty"`
	Result    string                 `json:"result"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Risk      Severity               `json:"risk"`
}

// AuditFilter filters audit events
type AuditFilter struct {
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	EventTypes []AuditEventType `json:"event_types,omitempty"`
	UserID     string           `json:"user_id,omitempty"`
	MinRisk    Severity         `json:"min_risk,omitempty"`
	Limit      int              `json:"limit,omitempty"`
}

// AuditStats contains audit statistics
type AuditStats struct {
	TotalEvents     int64                    `json:"total_events"`
	EventsByType    map[AuditEventType]int64 `json:"events_by_type"`
	EventsByRisk    map[Severity]int64       `json:"events_by_risk"`
	TopUsers        []UserAuditStat          `json:"top_users"`
	TrendingThreats []string                 `json:"trending_threats"`
}

// UserAuditStat contains audit stats for a user
type UserAuditStat struct {
	UserID    string  `json:"user_id"`
	Events    int64   `json:"events"`
	Blocks    int64   `json:"blocks"`
	RiskScore float64 `json:"risk_score"`
}

// AuditLogger logs security audit events
type AuditLogger interface {
	Log(ctx context.Context, event *AuditEvent) error
	Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error)
	GetStats(ctx context.Context, since time.Time) (*AuditStats, error)
}

// InMemoryAuditLogger provides an in-memory audit log implementation.
//
// Concurrent-safe by construction (CONST-029): events lives inside a
// single-key sseState-style state held by safe.Store; Log atomically
// appends + trims under one Update callback (Pattern Epsilon for the
// single-slice-needing-bounded-trim case).
type InMemoryAuditLogger struct {
	state     *safe.Store[string, []*AuditEvent]
	maxEvents int
	logger    *logrus.Logger
}

const auditStateKey = "_"

// NewInMemoryAuditLogger creates a new in-memory audit logger
func NewInMemoryAuditLogger(maxEvents int, logger *logrus.Logger) *InMemoryAuditLogger {
	if maxEvents <= 0 {
		maxEvents = 10000
	}
	if logger == nil {
		logger = logrus.New()
	}
	store := safe.NewStore[string, []*AuditEvent]()
	store.Put(auditStateKey, make([]*AuditEvent, 0, maxEvents))
	return &InMemoryAuditLogger{
		state:     store,
		maxEvents: maxEvents,
		logger:    logger,
	}
}

// Log logs an audit event
func (l *InMemoryAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	l.state.Update(auditStateKey, func(events []*AuditEvent, _ bool) ([]*AuditEvent, bool) {
		if len(events) >= l.maxEvents {
			removeCount := l.maxEvents / 10
			events = events[removeCount:]
		}
		events = append(events, event)
		return events, true
	})

	l.logger.WithFields(logrus.Fields{
		"audit_id":   event.ID,
		"event_type": event.EventType,
		"action":     event.Action,
		"result":     event.Result,
		"risk":       event.Risk,
		"user_id":    event.UserID,
	}).Info("Audit event logged")

	return nil
}

// Query queries audit events with filtering
func (l *InMemoryAuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	events, _ := l.state.Get(auditStateKey)

	var results []*AuditEvent

	for _, event := range events {
		if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
			continue
		}
		if filter.UserID != "" && event.UserID != filter.UserID {
			continue
		}
		if len(filter.EventTypes) > 0 {
			found := false
			for _, t := range filter.EventTypes {
				if event.EventType == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		results = append(results, event)
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// GetStats returns audit statistics
func (l *InMemoryAuditLogger) GetStats(ctx context.Context, since time.Time) (*AuditStats, error) {
	events, _ := l.state.Get(auditStateKey)

	stats := &AuditStats{
		EventsByType: make(map[AuditEventType]int64),
		EventsByRisk: make(map[Severity]int64),
		TopUsers:     make([]UserAuditStat, 0),
	}

	for _, event := range events {
		if event.Timestamp.Before(since) {
			continue
		}
		stats.TotalEvents++
		stats.EventsByType[event.EventType]++
		stats.EventsByRisk[event.Risk]++
	}

	return stats, nil
}

// GuardrailEngineAdapter wraps the extracted guardrails engine
type GuardrailEngineAdapter struct {
	engine *extsecurity.Engine
}

// NewGuardrailEngineAdapter creates a new adapter wrapping the external guardrails engine
func NewGuardrailEngineAdapter(config *extsecurity.Config) *GuardrailEngineAdapter {
	return &GuardrailEngineAdapter{
		engine: extsecurity.NewEngine(config),
	}
}

// Check runs guardrail checks on content
func (a *GuardrailEngineAdapter) Check(content string) *extsecurity.Result {
	return a.engine.Check(content)
}

// AddRule adds a rule to the guardrail engine
func (a *GuardrailEngineAdapter) AddRule(rule extsecurity.Rule) {
	a.engine.AddRule(rule)
}

// PIIDetectorAdapter wraps the extracted PII redactor
type PIIDetectorAdapter struct {
	redactor *extpii.Redactor
}

// NewPIIDetectorAdapter creates a new adapter wrapping the external PII redactor
func NewPIIDetectorAdapter(config *extpii.Config) *PIIDetectorAdapter {
	return &PIIDetectorAdapter{
		redactor: extpii.NewRedactor(config),
	}
}

// Detect detects PII in text
func (a *PIIDetectorAdapter) Detect(text string) []extpii.Match {
	return a.redactor.Detect(text)
}

// Redact redacts PII in text
func (a *PIIDetectorAdapter) Redact(text string) (string, []extpii.Match) {
	return a.redactor.Redact(text)
}

// PolicyEnforcerAdapter wraps the extracted policy enforcer
type PolicyEnforcerAdapter struct {
	enforcer *extpolicy.Enforcer
}

// NewPolicyEnforcerAdapter creates a new adapter wrapping the external policy enforcer
func NewPolicyEnforcerAdapter() *PolicyEnforcerAdapter {
	return &PolicyEnforcerAdapter{
		enforcer: extpolicy.NewEnforcer(),
	}
}

// LoadPolicy loads a policy into the enforcer
func (a *PolicyEnforcerAdapter) LoadPolicy(policy *extpolicy.Policy) error {
	return a.enforcer.LoadPolicy(policy)
}

// Evaluate evaluates a policy
func (a *PolicyEnforcerAdapter) Evaluate(
	ctx context.Context,
	policyName string,
	evalCtx *extpolicy.EvaluationContext,
) (*extpolicy.EvaluationResult, error) {
	return a.enforcer.Evaluate(ctx, policyName, evalCtx)
}

// SeverityToExternal converts internal Severity to external guardrails.Severity
func SeverityToExternal(s Severity) extsecurity.Severity {
	switch s {
	case SeverityCritical:
		return extsecurity.SeverityCritical
	case SeverityHigh:
		return extsecurity.SeverityHigh
	case SeverityMedium:
		return extsecurity.SeverityMedium
	case SeverityLow:
		return extsecurity.SeverityLow
	case SeverityInfo:
		return extsecurity.SeverityInfo
	default:
		return extsecurity.SeverityMedium
	}
}

// SeverityFromExternal converts external guardrails.Severity to internal Severity
func SeverityFromExternal(s extsecurity.Severity) Severity {
	switch s {
	case extsecurity.SeverityCritical:
		return SeverityCritical
	case extsecurity.SeverityHigh:
		return SeverityHigh
	case extsecurity.SeverityMedium:
		return SeverityMedium
	case extsecurity.SeverityLow:
		return SeverityLow
	case extsecurity.SeverityInfo:
		return SeverityInfo
	default:
		return SeverityMedium
	}
}
