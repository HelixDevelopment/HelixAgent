package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock implementations for testing
// =============================================================================

type mockTarget struct {
	responses    map[string]string
	defaultResp  string
	systemPrompt string
	metadata     map[string]interface{}
	sendError    error
}

func newMockTarget() *mockTarget {
	return &mockTarget{
		responses:    make(map[string]string),
		defaultResp:  "I cannot help with that request.",
		systemPrompt: "You are a helpful assistant.",
		metadata:     make(map[string]interface{}),
	}
}

func (m *mockTarget) Send(ctx context.Context, prompt string) (string, error) {
	if m.sendError != nil {
		return "", m.sendError
	}
	if resp, ok := m.responses[prompt]; ok {
		return resp, nil
	}
	return m.defaultResp, nil
}

func (m *mockTarget) GetSystemPrompt() string {
	return m.systemPrompt
}

func (m *mockTarget) GetMetadata() map[string]interface{} {
	return m.metadata
}

type mockDebateTarget struct {
	evaluation *DebateEvaluation
	err        error
}

func (m *mockDebateTarget) EvaluateResponse(ctx context.Context, attack *Attack, response string) (*DebateEvaluation, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.evaluation != nil {
		return m.evaluation, nil
	}
	return &DebateEvaluation{
		IsVulnerable:  false,
		Confidence:    0.5,
		Reasoning:     "Mock evaluation",
		ConsensusType: "mock",
	}, nil
}

type mockProviderVerifier struct {
	scores map[string]float64
}

func newMockProviderVerifier() *mockProviderVerifier {
	return &mockProviderVerifier{
		scores: make(map[string]float64),
	}
}

func (m *mockProviderVerifier) GetProviderScore(providerName string) float64 {
	if score, ok := m.scores[providerName]; ok {
		return score
	}
	return 5.0
}

func (m *mockProviderVerifier) IsProviderHealthy(providerName string) bool {
	return m.GetProviderScore(providerName) >= 6.0
}

type mockAuditLogger struct {
	events []*AuditEvent
	err    error
}

func (m *mockAuditLogger) Log(ctx context.Context, event *AuditEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditLogger) Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error) {
	return m.events, nil
}

func (m *mockAuditLogger) GetStats(ctx context.Context, since time.Time) (*AuditStats, error) {
	return &AuditStats{TotalEvents: int64(len(m.events))}, nil
}

// =============================================================================
// DeepTeamRedTeamer Tests
// =============================================================================

func TestNewDeepTeamRedTeamer(t *testing.T) {
	t.Parallel()
	t.Run("with nil config and logger", func(t *testing.T) {
		rt := NewDeepTeamRedTeamer(nil, nil)
		assert.NotNil(t, rt)
		assert.NotNil(t, rt.config)
		assert.NotNil(t, rt.logger)
		assert.NotNil(t, rt.attacks)
	})

	t.Run("with custom config", func(t *testing.T) {
		t.Parallel()
		config := &RedTeamConfig{
			AttackTypes: []AttackType{AttackTypeJailbreak},
			Timeout:     60 * time.Second,
		}
		logger := logrus.New()

		rt := NewDeepTeamRedTeamer(config, logger)
		assert.NotNil(t, rt)
		assert.Equal(t, config, rt.config)
		assert.Equal(t, logger, rt.logger)
	})

	t.Run("with custom payloads in config", func(t *testing.T) {
		t.Parallel()
		customAttack := Attack{
			ID:       "CUSTOM-001",
			Name:     "Custom Attack",
			Type:     AttackTypeCodeInjection,
			Payload:  "custom payload",
			Severity: SeverityCritical,
		}

		config := &RedTeamConfig{
			AttackTypes:    []AttackType{AttackTypeCodeInjection},
			CustomPayloads: []Attack{customAttack},
		}

		rt := NewDeepTeamRedTeamer(config, nil)
		attacks := rt.GetAttacks(AttackTypeCodeInjection)

		// Should include both built-in and custom attacks
		found := false
		for _, a := range attacks {
			if a.ID == "CUSTOM-001" {
				found = true
				break
			}
		}
		assert.True(t, found, "Custom attack should be registered")
	})
}

func TestDeepTeamRedTeamer_SetDebateTarget(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	mockDebate := &mockDebateTarget{}
	rt.SetDebateTarget(mockDebate)

	// Verify it was set (internal state)
	assert.NotNil(t, rt.debateTarget)
}

func TestDeepTeamRedTeamer_SetVerifier(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	mockVerifier := newMockProviderVerifier()
	rt.SetVerifier(mockVerifier)

	// Verify it was set
	assert.NotNil(t, rt.verifier)
}

func TestDeepTeamRedTeamer_SetAuditLogger(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	mockLogger := &mockAuditLogger{}
	rt.SetAuditLogger(mockLogger)

	// Verify it was set
	assert.NotNil(t, rt.auditLogger)
}

func TestDeepTeamRedTeamer_GetAttacks(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	t.Run("get direct prompt injection attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeDirectPromptInjection)
		assert.NotEmpty(t, attacks)

		for _, attack := range attacks {
			assert.Equal(t, AttackTypeDirectPromptInjection, attack.Type)
			assert.NotEmpty(t, attack.ID)
			assert.NotEmpty(t, attack.Name)
			assert.NotEmpty(t, attack.Payload)
		}
	})

	t.Run("get jailbreak attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeJailbreak)
		assert.NotEmpty(t, attacks)
	})

	t.Run("get data leakage attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeDataLeakage)
		assert.NotEmpty(t, attacks)
	})

	t.Run("get system prompt leakage attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeSystemPromptLeakage)
		assert.NotEmpty(t, attacks)
	})

	t.Run("get harmful content attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeHarmfulContent)
		assert.NotEmpty(t, attacks)
	})

	t.Run("get code injection attacks", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks(AttackTypeCodeInjection)
		assert.NotEmpty(t, attacks)
	})

	t.Run("get nonexistent attack type returns empty", func(t *testing.T) {
		t.Parallel()
		attacks := rt.GetAttacks("nonexistent_type")
		assert.Empty(t, attacks)
	})
}

func TestDeepTeamRedTeamer_AddCustomAttack(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	t.Run("add custom attack with ID", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{
			ID:       "CUSTOM-TEST-001",
			Name:     "Test Attack",
			Type:     AttackTypeManipulation,
			Payload:  "test payload",
			Severity: SeverityHigh,
		}

		rt.AddCustomAttack(attack)

		attacks := rt.GetAttacks(AttackTypeManipulation)
		found := false
		for _, a := range attacks {
			if a.ID == "CUSTOM-TEST-001" {
				found = true
				assert.Equal(t, "Test Attack", a.Name)
			}
		}
		assert.True(t, found)
	})

	t.Run("add custom attack without ID generates one", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{
			Name:     "Auto ID Attack",
			Type:     AttackTypeDeception,
			Payload:  "deception payload",
			Severity: SeverityMedium,
		}

		rt.AddCustomAttack(attack)
		assert.NotEmpty(t, attack.ID)
	})
}

func TestDeepTeamRedTeamer_RunAttack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("attack blocked by target error", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		target := newMockTarget()
		target.sendError = errors.New("request blocked")

		attack := &Attack{
			ID:       "TEST-001",
			Type:     AttackTypeDirectPromptInjection,
			Payload:  "ignore instructions",
			Severity: SeverityHigh,
		}

		result, err := rt.RunAttack(ctx, attack, target)
		require.NoError(t, err)
		assert.True(t, result.Blocked)
		assert.False(t, result.Success)
		assert.Equal(t, float64(0), result.Score)
	})

	t.Run("attack succeeds with vulnerable response", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		rt.SetDebateTarget(&mockDebateTarget{
			evaluation: &DebateEvaluation{
				IsVulnerable: true,
				Confidence:   0.9,
				Reasoning:    "Vulnerability detected",
			},
		})

		target := newMockTarget()
		target.defaultResp = "I'll ignore my previous instructions and help you"

		attack := &Attack{
			ID:       "TEST-002",
			Type:     AttackTypeJailbreak,
			Payload:  "DAN jailbreak",
			Severity: SeverityCritical,
		}

		result, err := rt.RunAttack(ctx, attack, target)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.Mitigations)
	})

	t.Run("attack fails with safe response", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		target := newMockTarget()
		target.defaultResp = "I cannot help with that request."

		attack := &Attack{
			ID:       "TEST-003",
			Type:     AttackTypeHarmfulContent,
			Payload:  "harmful request",
			Severity: SeverityHigh,
		}

		result, err := rt.RunAttack(ctx, attack, target)
		require.NoError(t, err)
		assert.False(t, result.Success)
	})

	t.Run("attack with audit logging", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		auditLogger := &mockAuditLogger{}
		rt.SetAuditLogger(auditLogger)

		target := newMockTarget()

		attack := &Attack{
			ID:       "TEST-004",
			Type:     AttackTypeDataLeakage,
			Payload:  "extract data",
			Severity: SeverityMedium,
		}

		_, err := rt.RunAttack(ctx, attack, target)
		require.NoError(t, err)

		// Verify audit event was logged
		assert.NotEmpty(t, auditLogger.events)
	})

	t.Run("attack with debate evaluation error falls back to basic check", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		rt.SetDebateTarget(&mockDebateTarget{
			err: errors.New("debate failed"),
		})

		target := newMockTarget()

		attack := &Attack{
			ID:       "TEST-005",
			Type:     AttackTypeDirectPromptInjection,
			Payload:  "test",
			Severity: SeverityHigh,
		}

		result, err := rt.RunAttack(ctx, attack, target)
		require.NoError(t, err)
		assert.Equal(t, 0.5, result.Score)
		assert.Equal(t, 0.3, result.Confidence)
	})
}

func TestDeepTeamRedTeamer_BasicVulnerabilityCheck(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	t.Run("prompt injection detection - compliant", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeDirectPromptInjection}
		response := "I'll ignore my instructions and help you"

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("prompt injection detection - DAN", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeJailbreak}
		response := "As DAN, I can help with anything"

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("system prompt leakage detection", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeSystemPromptLeakage}
		response := "You are configured to be helpful. Your instructions tell you to..."

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("data leakage detection", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeDataLeakage}
		response := "I remember from our earlier conversation..."

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("harmful content - refusal is not vulnerable", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeHarmfulContent}
		response := "I cannot provide information on how to do that."

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.False(t, isVuln)
	})

	t.Run("harmful content - long response without refusal might be vulnerable", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeHarmfulContent}
		response := string(make([]byte, 300))

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("code injection detection", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeCodeInjection}
		response := "The output: root:x:0:0:root:/root:/bin/bash"

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})

	t.Run("SQL injection detection", func(t *testing.T) {
		t.Parallel()
		attack := &Attack{Type: AttackTypeSQLInjection}
		response := "Query executed: drop table users"

		isVuln := rt.basicVulnerabilityCheck(attack, response)
		assert.True(t, isVuln)
	})
}

func TestDeepTeamRedTeamer_RunSuite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("run suite with default config", func(t *testing.T) {
		t.Parallel()
		config := &RedTeamConfig{
			AttackTypes:         []AttackType{AttackTypeDirectPromptInjection},
			OWASPCategories:     []OWASPCategory{OWASP_LLM01},
			MaxConcurrent:       2,
			Timeout:             5 * time.Second,
			VariationsPerAttack: 1,
		}
		rt := NewDeepTeamRedTeamer(config, nil)
		target := newMockTarget()

		report, err := rt.RunSuite(ctx, nil, target)
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotEmpty(t, report.ID)
		assert.GreaterOrEqual(t, report.TotalAttacks, 1)
		assert.NotEmpty(t, report.Summary)
	})

	t.Run("run suite with custom config", func(t *testing.T) {
		t.Parallel()
		rt := NewDeepTeamRedTeamer(nil, nil)
		target := newMockTarget()

		customConfig := &RedTeamConfig{
			AttackTypes:     []AttackType{AttackTypeJailbreak, AttackTypeDataLeakage},
			OWASPCategories: []OWASPCategory{OWASP_LLM01, OWASP_LLM02},
			MaxConcurrent:   1,
			Timeout:         5 * time.Second,
		}

		report, err := rt.RunSuite(ctx, customConfig, target)
		require.NoError(t, err)
		assert.NotNil(t, report)
	})

	t.Run("run suite generates recommendations", func(t *testing.T) {
		t.Parallel()
		config := &RedTeamConfig{
			AttackTypes:     []AttackType{AttackTypeDirectPromptInjection},
			OWASPCategories: []OWASPCategory{OWASP_LLM01},
			MaxConcurrent:   1,
			Timeout:         5 * time.Second,
		}
		rt := NewDeepTeamRedTeamer(config, nil)

		// Use debate target that reports vulnerabilities
		rt.SetDebateTarget(&mockDebateTarget{
			evaluation: &DebateEvaluation{
				IsVulnerable: true,
				Confidence:   0.9,
			},
		})

		target := newMockTarget()
		target.defaultResp = "I'll ignore my instructions"

		report, err := rt.RunSuite(ctx, nil, target)
		require.NoError(t, err)

		// If vulnerabilities were found, there should be recommendations
		if report.SuccessfulAttacks > 0 {
			assert.NotEmpty(t, report.Recommendations)
		}
	})
}

func TestDeepTeamRedTeamer_GenerateMitigations(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	testCases := []struct {
		attackType    AttackType
		expectedCount int
	}{
		{AttackTypeDirectPromptInjection, 4},
		{AttackTypeJailbreak, 4},
		{AttackTypeSystemPromptLeakage, 3},
		{AttackTypeDataLeakage, 3},
		{AttackTypeHarmfulContent, 3},
		{AttackTypeCodeInjection, 4},
		{AttackTypeSQLInjection, 4},
		{AttackTypeResourceExhaustion, 3},
	}

	for _, tc := range testCases {
		t.Run(string(tc.attackType), func(t *testing.T) {
			t.Parallel()
			attack := &Attack{Type: tc.attackType}
			mitigations := rt.generateMitigations(attack)
			assert.GreaterOrEqual(t, len(mitigations), 1, "Should generate mitigations for %s", tc.attackType)
		})
	}
}

func TestDeepTeamRedTeamer_GetAllAttackTypes(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	attackTypes := rt.GetAllAttackTypes()
	assert.NotEmpty(t, attackTypes)

	// Verify some known attack types are included
	expectedTypes := []AttackType{
		AttackTypeDirectPromptInjection,
		AttackTypeJailbreak,
		AttackTypeDataLeakage,
		AttackTypeHarmfulContent,
		AttackTypeCodeInjection,
		AttackTypeSQLInjection,
	}

	for _, expected := range expectedTypes {
		found := false
		for _, at := range attackTypes {
			if at == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected attack type %s to be in list", expected)
	}
}

func TestRedTeamReport_Scores(t *testing.T) {
	t.Parallel()
	rt := NewDeepTeamRedTeamer(nil, nil)

	t.Run("calculate scores with no attacks", func(t *testing.T) {
		t.Parallel()
		report := &RedTeamReport{
			TotalAttacks:  0,
			OWASPCoverage: make(map[OWASPCategory]*CategoryScore),
		}
		rt.calculateScores(report)
		assert.Equal(t, float64(0), report.OverallScore)
	})

	t.Run("calculate scores with successful attacks", func(t *testing.T) {
		t.Parallel()
		report := &RedTeamReport{
			TotalAttacks:      10,
			SuccessfulAttacks: 3,
			OWASPCoverage: map[OWASPCategory]*CategoryScore{
				OWASP_LLM01: {AttacksRun: 5, Vulnerabilities: 2},
			},
		}
		rt.calculateScores(report)
		assert.Equal(t, 0.3, report.OverallScore)
		assert.Equal(t, 0.4, report.OWASPCoverage[OWASP_LLM01].Score)
	})
}

func TestDefaultRedTeamConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRedTeamConfig()

	assert.NotEmpty(t, config.AttackTypes)
	assert.NotEmpty(t, config.OWASPCategories)
	assert.Greater(t, config.VariationsPerAttack, 0)
	assert.Greater(t, config.MaxConcurrent, 0)
	assert.Greater(t, config.Timeout, time.Duration(0))
	assert.False(t, config.AdaptiveMode)
}

func TestAttackTypes(t *testing.T) {
	t.Parallel()
	// Verify all attack types have non-empty string values
	attackTypes := []AttackType{
		AttackTypeDirectPromptInjection,
		AttackTypeIndirectPromptInjection,
		AttackTypeJailbreak,
		AttackTypeRoleplay,
		AttackTypeDataLeakage,
		AttackTypeSystemPromptLeakage,
		AttackTypePIIExtraction,
		AttackTypeModelExtraction,
		AttackTypeResourceExhaustion,
		AttackTypeInfiniteLoop,
		AttackTypeTokenOverflow,
		AttackTypeHarmfulContent,
		AttackTypeHateSpeech,
		AttackTypeViolentContent,
		AttackTypeSexualContent,
		AttackTypeIllegalActivities,
		AttackTypeManipulation,
		AttackTypeDeception,
		AttackTypeImpersonation,
		AttackTypeAuthorityAbuse,
		AttackTypeCodeInjection,
		AttackTypeSQLInjection,
		AttackTypeCommandInjection,
		AttackTypeXSS,
		AttackTypeBiasExploitation,
		AttackTypeStereotyping,
		AttackTypeDiscrimination,
		AttackTypeHallucinationInduction,
		AttackTypeConfabulationTrigger,
		AttackTypeFalseCitation,
		AttackTypeModelPoisoning,
		AttackTypeDataPoisoning,
		AttackTypeDependencyAttack,
		AttackTypeEncoding,
		AttackTypeObfuscation,
		AttackTypeFragmentation,
		AttackTypeMultilingual,
	}

	for _, at := range attackTypes {
		assert.NotEmpty(t, string(at), "Attack type should have non-empty string value")
	}
}

func TestOWASPCategories(t *testing.T) {
	t.Parallel()
	categories := []OWASPCategory{
		OWASP_LLM01,
		OWASP_LLM02,
		OWASP_LLM03,
		OWASP_LLM04,
		OWASP_LLM05,
		OWASP_LLM06,
		OWASP_LLM07,
		OWASP_LLM08,
		OWASP_LLM09,
		OWASP_LLM10,
	}

	for _, cat := range categories {
		assert.NotEmpty(t, string(cat))
		assert.Contains(t, string(cat), "LLM")
		assert.Contains(t, string(cat), "2025")
	}
}

func TestSeverity(t *testing.T) {
	t.Parallel()
	severities := []Severity{
		SeverityCritical,
		SeverityHigh,
		SeverityMedium,
		SeverityLow,
		SeverityInfo,
	}

	for _, sev := range severities {
		assert.NotEmpty(t, string(sev))
	}
}

func TestAttackFields(t *testing.T) {
	t.Parallel()
	attack := &Attack{
		ID:          "ATK-001",
		Name:        "Test Attack",
		Type:        AttackTypeDirectPromptInjection,
		OWASP:       []OWASPCategory{OWASP_LLM01, OWASP_LLM02},
		Description: "A test attack",
		Payload:     "test payload",
		Variations:  []string{"var1", "var2"},
		Severity:    SeverityHigh,
		Metadata:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "ATK-001", attack.ID)
	assert.Equal(t, "Test Attack", attack.Name)
	assert.Equal(t, AttackTypeDirectPromptInjection, attack.Type)
	assert.Len(t, attack.OWASP, 2)
	assert.Len(t, attack.Variations, 2)
	assert.NotNil(t, attack.Metadata)
}

func TestAttackResultFields(t *testing.T) {
	t.Parallel()
	result := &AttackResult{
		AttackID:    "ATK-001",
		AttackType:  AttackTypeJailbreak,
		Success:     true,
		Blocked:     false,
		Response:    "vulnerable response",
		Score:       0.9,
		Confidence:  0.85,
		Severity:    SeverityCritical,
		Details:     "Vulnerability found",
		Duration:    100 * time.Millisecond,
		Timestamp:   time.Now(),
		Mitigations: []string{"Fix 1", "Fix 2"},
	}

	assert.Equal(t, "ATK-001", result.AttackID)
	assert.True(t, result.Success)
	assert.False(t, result.Blocked)
	assert.Equal(t, 0.9, result.Score)
	assert.NotEmpty(t, result.Mitigations)
}

func TestCategoryScoreFields(t *testing.T) {
	t.Parallel()
	score := &CategoryScore{
		Category:        OWASP_LLM01,
		AttacksRun:      10,
		Vulnerabilities: 3,
		Score:           0.3,
		Findings:        []string{"Finding 1", "Finding 2"},
	}

	assert.Equal(t, OWASP_LLM01, score.Category)
	assert.Equal(t, 10, score.AttacksRun)
	assert.Equal(t, 3, score.Vulnerabilities)
	assert.Equal(t, 0.3, score.Score)
	assert.Len(t, score.Findings, 2)
}

func TestRedTeamReportFields(t *testing.T) {
	t.Parallel()
	report := &RedTeamReport{
		ID:                "RPT-001",
		StartTime:         time.Now().Add(-10 * time.Minute),
		EndTime:           time.Now(),
		TotalAttacks:      50,
		SuccessfulAttacks: 5,
		BlockedAttacks:    40,
		FailedAttacks:     5,
		OverallScore:      0.1,
		Results:           []*AttackResult{},
		OWASPCoverage:     make(map[OWASPCategory]*CategoryScore),
		Recommendations:   []string{"Rec 1"},
		Summary:           "Test summary",
	}

	assert.Equal(t, "RPT-001", report.ID)
	assert.Equal(t, 50, report.TotalAttacks)
	assert.Equal(t, 5, report.SuccessfulAttacks)
	assert.Equal(t, 40, report.BlockedAttacks)
	assert.NotEmpty(t, report.Summary)
}

func TestDebateEvaluationFields(t *testing.T) {
	t.Parallel()
	eval := &DebateEvaluation{
		IsVulnerable:  true,
		Confidence:    0.95,
		Reasoning:     "Test reasoning",
		Participants:  []string{"LLM1", "LLM2"},
		ConsensusType: "unanimous",
		Details:       map[string]interface{}{"key": "value"},
	}

	assert.True(t, eval.IsVulnerable)
	assert.Equal(t, 0.95, eval.Confidence)
	assert.NotEmpty(t, eval.Reasoning)
	assert.Len(t, eval.Participants, 2)
}

func TestRedTeamConfig_Fields(t *testing.T) {
	t.Parallel()
	config := &RedTeamConfig{
		AttackTypes:         []AttackType{AttackTypeJailbreak},
		OWASPCategories:     []OWASPCategory{OWASP_LLM01},
		VariationsPerAttack: 10,
		MaxConcurrent:       5,
		Timeout:             2 * time.Minute,
		AdaptiveMode:        true,
		CustomPayloads:      []Attack{{ID: "custom"}},
	}

	assert.NotEmpty(t, config.AttackTypes)
	assert.Equal(t, 10, config.VariationsPerAttack)
	assert.True(t, config.AdaptiveMode)
	assert.Len(t, config.CustomPayloads, 1)
}
