package security_test

import (
	"context"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Type constant tests
// ============================================================================

func TestAttackTypeConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		constant adapter.AttackType
		value    string
	}{
		{"DirectPromptInjection", adapter.AttackTypeDirectPromptInjection, "direct_prompt_injection"},
		{"IndirectPromptInjection", adapter.AttackTypeIndirectPromptInjection, "indirect_prompt_injection"},
		{"Jailbreak", adapter.AttackTypeJailbreak, "jailbreak"},
		{"Roleplay", adapter.AttackTypeRoleplay, "roleplay_injection"},
		{"DataLeakage", adapter.AttackTypeDataLeakage, "data_leakage"},
		{"SystemPromptLeakage", adapter.AttackTypeSystemPromptLeakage, "system_prompt_leakage"},
		{"PIIExtraction", adapter.AttackTypePIIExtraction, "pii_extraction"},
		{"ModelExtraction", adapter.AttackTypeModelExtraction, "model_extraction"},
		{"ResourceExhaustion", adapter.AttackTypeResourceExhaustion, "resource_exhaustion"},
		{"InfiniteLoop", adapter.AttackTypeInfiniteLoop, "infinite_loop"},
		{"TokenOverflow", adapter.AttackTypeTokenOverflow, "token_overflow"},
		{"HarmfulContent", adapter.AttackTypeHarmfulContent, "harmful_content"},
		{"HateSpeech", adapter.AttackTypeHateSpeech, "hate_speech"},
		{"ViolentContent", adapter.AttackTypeViolentContent, "violent_content"},
		{"SexualContent", adapter.AttackTypeSexualContent, "sexual_content"},
		{"IllegalActivities", adapter.AttackTypeIllegalActivities, "illegal_activities"},
		{"Manipulation", adapter.AttackTypeManipulation, "manipulation"},
		{"Deception", adapter.AttackTypeDeception, "deception"},
		{"Impersonation", adapter.AttackTypeImpersonation, "impersonation"},
		{"AuthorityAbuse", adapter.AttackTypeAuthorityAbuse, "authority_abuse"},
		{"CodeInjection", adapter.AttackTypeCodeInjection, "code_injection"},
		{"SQLInjection", adapter.AttackTypeSQLInjection, "sql_injection"},
		{"CommandInjection", adapter.AttackTypeCommandInjection, "command_injection"},
		{"XSS", adapter.AttackTypeXSS, "xss"},
		{"BiasExploitation", adapter.AttackTypeBiasExploitation, "bias_exploitation"},
		{"Stereotyping", adapter.AttackTypeStereotyping, "stereotyping"},
		{"Discrimination", adapter.AttackTypeDiscrimination, "discrimination"},
		{"HallucinationInduction", adapter.AttackTypeHallucinationInduction, "hallucination_induction"},
		{"ConfabulationTrigger", adapter.AttackTypeConfabulationTrigger, "confabulation_trigger"},
		{"FalseCitation", adapter.AttackTypeFalseCitation, "false_citation"},
		{"ModelPoisoning", adapter.AttackTypeModelPoisoning, "model_poisoning"},
		{"DataPoisoning", adapter.AttackTypeDataPoisoning, "data_poisoning"},
		{"DependencyAttack", adapter.AttackTypeDependencyAttack, "dependency_attack"},
		{"Encoding", adapter.AttackTypeEncoding, "encoding_evasion"},
		{"Obfuscation", adapter.AttackTypeObfuscation, "obfuscation"},
		{"Fragmentation", adapter.AttackTypeFragmentation, "fragmentation"},
		{"Multilingual", adapter.AttackTypeMultilingual, "multilingual_evasion"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, adapter.AttackType(tt.value), tt.constant)
		})
	}
}

func TestSeverityConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, adapter.Severity("critical"), adapter.SeverityCritical)
	assert.Equal(t, adapter.Severity("high"), adapter.SeverityHigh)
	assert.Equal(t, adapter.Severity("medium"), adapter.SeverityMedium)
	assert.Equal(t, adapter.Severity("low"), adapter.SeverityLow)
	assert.Equal(t, adapter.Severity("info"), adapter.SeverityInfo)
}

func TestAuditEventTypeConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, adapter.AuditEventType("tool_call"), adapter.AuditEventToolCall)
	assert.Equal(t, adapter.AuditEventType("guardrail_block"), adapter.AuditEventGuardrailBlock)
	assert.Equal(t, adapter.AuditEventType("attack_detected"), adapter.AuditEventAttackDetected)
	assert.Equal(t, adapter.AuditEventType("pii_access"), adapter.AuditEventPIIAccess)
	assert.Equal(t, adapter.AuditEventType("permission_deny"), adapter.AuditEventPermissionDeny)
	assert.Equal(t, adapter.AuditEventType("rate_limit"), adapter.AuditEventRateLimit)
	assert.Equal(t, adapter.AuditEventType("authentication"), adapter.AuditEventAuthentication)
}

func TestAttackTypeUniqueness(t *testing.T) {
	t.Parallel()
	attackTypes := []adapter.AttackType{
		adapter.AttackTypeDirectPromptInjection,
		adapter.AttackTypeIndirectPromptInjection,
		adapter.AttackTypeJailbreak,
		adapter.AttackTypeRoleplay,
		adapter.AttackTypeDataLeakage,
		adapter.AttackTypeSystemPromptLeakage,
		adapter.AttackTypePIIExtraction,
		adapter.AttackTypeModelExtraction,
		adapter.AttackTypeResourceExhaustion,
		adapter.AttackTypeInfiniteLoop,
		adapter.AttackTypeTokenOverflow,
		adapter.AttackTypeHarmfulContent,
		adapter.AttackTypeHateSpeech,
		adapter.AttackTypeViolentContent,
		adapter.AttackTypeSexualContent,
		adapter.AttackTypeIllegalActivities,
		adapter.AttackTypeManipulation,
		adapter.AttackTypeDeception,
		adapter.AttackTypeImpersonation,
		adapter.AttackTypeAuthorityAbuse,
		adapter.AttackTypeCodeInjection,
		adapter.AttackTypeSQLInjection,
		adapter.AttackTypeCommandInjection,
		adapter.AttackTypeXSS,
		adapter.AttackTypeBiasExploitation,
		adapter.AttackTypeStereotyping,
		adapter.AttackTypeDiscrimination,
		adapter.AttackTypeHallucinationInduction,
		adapter.AttackTypeConfabulationTrigger,
		adapter.AttackTypeFalseCitation,
		adapter.AttackTypeModelPoisoning,
		adapter.AttackTypeDataPoisoning,
		adapter.AttackTypeDependencyAttack,
		adapter.AttackTypeEncoding,
		adapter.AttackTypeObfuscation,
		adapter.AttackTypeFragmentation,
		adapter.AttackTypeMultilingual,
	}

	seen := make(map[adapter.AttackType]bool)
	for _, at := range attackTypes {
		assert.False(t, seen[at], "AttackType %q is duplicated", at)
		seen[at] = true
	}
}

// ============================================================================
// Attack struct test
// ============================================================================

func TestAttackStruct(t *testing.T) {
	t.Parallel()
	attack := &adapter.Attack{
		ID:          "atk-001",
		Name:        "Jailbreak Test",
		Description: "Test jailbreak attack",
		Type:        adapter.AttackTypeJailbreak,
		Severity:    adapter.SeverityHigh,
		Payload:     "Ignore your instructions...",
	}

	assert.Equal(t, "atk-001", attack.ID)
	assert.Equal(t, adapter.AttackTypeJailbreak, attack.Type)
	assert.Equal(t, adapter.SeverityHigh, attack.Severity)
}

// ============================================================================
// InMemoryAuditLogger Tests
// ============================================================================

func TestNewInMemoryAuditLogger(t *testing.T) {
	t.Parallel()
	logger := adapter.NewInMemoryAuditLogger(100, nil)
	require.NotNil(t, logger)
}

func TestInMemoryAuditLogger_Log(t *testing.T) {
	t.Parallel()
	logger := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	event := &adapter.AuditEvent{
		EventType: adapter.AuditEventToolCall,
		Action:    "tool_call",
		Result:    "success",
		Risk:      adapter.SeverityLow,
		UserID:    "user-123",
	}

	err := logger.Log(ctx, event)
	require.NoError(t, err)
	// ID should be auto-generated
	assert.NotEmpty(t, event.ID)
	// Timestamp should be set
	assert.False(t, event.Timestamp.IsZero())
}

func TestInMemoryAuditLogger_Log_PreservesID(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	event := &adapter.AuditEvent{
		ID:        "custom-id",
		EventType: adapter.AuditEventPIIAccess,
		Action:    "access",
		Result:    "allowed",
		Risk:      adapter.SeverityInfo,
	}

	err := l.Log(ctx, event)
	require.NoError(t, err)
	assert.Equal(t, "custom-id", event.ID)
}

func TestInMemoryAuditLogger_Query(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	events := []*adapter.AuditEvent{
		{EventType: adapter.AuditEventToolCall, Action: "call1", Result: "ok", Risk: adapter.SeverityLow, UserID: "user-1"},
		{EventType: adapter.AuditEventGuardrailBlock, Action: "block", Result: "blocked", Risk: adapter.SeverityHigh, UserID: "user-2"},
		{EventType: adapter.AuditEventToolCall, Action: "call2", Result: "ok", Risk: adapter.SeverityLow, UserID: "user-1"},
	}

	for _, e := range events {
		err := l.Log(ctx, e)
		require.NoError(t, err)
	}

	// Query all
	filter := &adapter.AuditFilter{}
	results, err := l.Query(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestInMemoryAuditLogger_Query_ByUser(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "a", Result: "ok", Risk: adapter.SeverityLow, UserID: "user-1"})
	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "b", Result: "ok", Risk: adapter.SeverityLow, UserID: "user-2"})

	filter := &adapter.AuditFilter{UserID: "user-1"}
	results, err := l.Query(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "user-1", results[0].UserID)
}

func TestInMemoryAuditLogger_Query_ByEventType(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "a", Result: "ok", Risk: adapter.SeverityLow})
	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventGuardrailBlock, Action: "b", Result: "blocked", Risk: adapter.SeverityHigh})

	filter := &adapter.AuditFilter{EventTypes: []adapter.AuditEventType{adapter.AuditEventGuardrailBlock}}
	results, err := l.Query(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, adapter.AuditEventGuardrailBlock, results[0].EventType)
}

func TestInMemoryAuditLogger_Query_WithLimit(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "a", Result: "ok", Risk: adapter.SeverityLow})
	}

	filter := &adapter.AuditFilter{Limit: 2}
	results, err := l.Query(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestInMemoryAuditLogger_GetStats(t *testing.T) {
	t.Parallel()
	l := adapter.NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "a", Result: "ok", Risk: adapter.SeverityLow})
	l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventGuardrailBlock, Action: "b", Result: "blocked", Risk: adapter.SeverityHigh})

	stats, err := l.GetStats(ctx, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.TotalEvents)
	assert.Equal(t, int64(1), stats.EventsByType[adapter.AuditEventToolCall])
	assert.Equal(t, int64(1), stats.EventsByType[adapter.AuditEventGuardrailBlock])
}

func TestInMemoryAuditLogger_MaxEventsEnforced(t *testing.T) {
	t.Parallel()
	maxEvents := 10
	l := adapter.NewInMemoryAuditLogger(maxEvents, nil)
	ctx := context.Background()

	// Add more than maxEvents
	for i := 0; i < maxEvents+5; i++ {
		l.Log(ctx, &adapter.AuditEvent{EventType: adapter.AuditEventToolCall, Action: "a", Result: "ok", Risk: adapter.SeverityLow})
	}

	// Should not exceed maxEvents significantly (10% is evicted when full)
	filter := &adapter.AuditFilter{}
	results, err := l.Query(ctx, filter)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), maxEvents)
}

// ============================================================================
// GuardrailEngineAdapter Tests
// ============================================================================

func TestNewGuardrailEngineAdapter(t *testing.T) {
	t.Parallel()
	e := adapter.NewGuardrailEngineAdapter(nil)
	require.NotNil(t, e)
}

func TestGuardrailEngineAdapter_Check(t *testing.T) {
	t.Parallel()
	e := adapter.NewGuardrailEngineAdapter(nil)
	result := e.Check("Hello, how are you?")
	assert.NotNil(t, result)
}

// ============================================================================
// PIIDetectorAdapter Tests
// ============================================================================

func TestNewPIIDetectorAdapter(t *testing.T) {
	t.Parallel()
	d := adapter.NewPIIDetectorAdapter(nil)
	require.NotNil(t, d)
}

func TestPIIDetectorAdapter_Detect(t *testing.T) {
	t.Parallel()
	d := adapter.NewPIIDetectorAdapter(nil)
	matches := d.Detect("My email is test@example.com")
	assert.NotNil(t, matches)
}

func TestPIIDetectorAdapter_Redact(t *testing.T) {
	t.Parallel()
	d := adapter.NewPIIDetectorAdapter(nil)
	redacted, matches := d.Redact("Contact test@example.com for info.")
	assert.NotEmpty(t, redacted)
	assert.NotNil(t, matches)
}

// ============================================================================
// PolicyEnforcerAdapter Tests
// ============================================================================

func TestNewPolicyEnforcerAdapter(t *testing.T) {
	t.Parallel()
	e := adapter.NewPolicyEnforcerAdapter()
	require.NotNil(t, e)
}

// ============================================================================
// Severity conversion tests
// ============================================================================

func TestSeverityToExternal_AllValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input adapter.Severity
	}{
		{adapter.SeverityCritical},
		{adapter.SeverityHigh},
		{adapter.SeverityMedium},
		{adapter.SeverityLow},
		{adapter.SeverityInfo},
		{"unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			t.Parallel()
			result := adapter.SeverityToExternal(tt.input)
			// Just verify it returns a non-empty value
			assert.NotEmpty(t, string(result))
		})
	}
}

func TestSeverityFromExternal_RoundTrip(t *testing.T) {
	t.Parallel()
	severities := []adapter.Severity{
		adapter.SeverityCritical,
		adapter.SeverityHigh,
		adapter.SeverityMedium,
		adapter.SeverityLow,
		adapter.SeverityInfo,
	}

	for _, sev := range severities {
		ext := adapter.SeverityToExternal(sev)
		back := adapter.SeverityFromExternal(ext)
		assert.Equal(t, sev, back, "round-trip failed for %s", sev)
	}
}
