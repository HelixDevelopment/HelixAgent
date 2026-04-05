package agents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Adversarial Protocol Tests
// =============================================================================

// mockAdversarialLLM implements AdversarialLLMClient for testing.
type mockAdversarialLLM struct {
	responses []string
	callCount int
	err       error
}

func (m *mockAdversarialLLM) Complete(
	ctx context.Context, prompt string,
) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return resp, nil
	}
	return "", fmt.Errorf("no more responses")
}

func TestDefaultAdversarialConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultAdversarialConfig()

	assert.Equal(t, 3, cfg.MaxRounds)
	assert.Equal(t, 1, cfg.MinVulnerabilities)
	assert.Equal(t, 0.2, cfg.RiskThreshold)
	assert.Equal(t, 5*time.Minute, cfg.Timeout)
}

func TestNewAdversarialProtocol(t *testing.T) {
	t.Parallel()
	mock := &mockAdversarialLLM{}
	cfg := DefaultAdversarialConfig()

	ap := NewAdversarialProtocol(cfg, mock)
	require.NotNil(t, ap)

	assert.NotNil(t, ap.redTeam)
	assert.Equal(t, "red-team-agent", ap.redTeam.ID)
	assert.Equal(t, "attacker", ap.redTeam.Role)

	assert.NotNil(t, ap.blueTeam)
	assert.Equal(t, "blue-team-agent", ap.blueTeam.ID)
	assert.Equal(t, "defender", ap.blueTeam.Role)

	assert.Equal(t, mock, ap.llmClient)
	assert.Equal(t, cfg.MaxRounds, ap.config.MaxRounds)
}

func TestNewAdversarialProtocol_NilLLM(t *testing.T) {
	t.Parallel()
	cfg := DefaultAdversarialConfig()
	ap := NewAdversarialProtocol(cfg, nil)
	require.NotNil(t, ap)
	assert.Nil(t, ap.llmClient)
}

func TestAdversarialProtocol_Execute_WithMockLLM(t *testing.T) {
	t.Parallel()
	// The mock returns a structured attack with low risk, which should cause
	// early termination after the first round.
	attackResponse := `VULNERABILITIES
ID: VULN-001
Category: logic_error
Severity: low
Description: Minor issue
Evidence: some evidence
Exploit: some exploit
---

EDGE_CASES
ID: EDGE-001
Description: Empty input
Input: ""
Expected: Error returned
---

STRESS_SCENARIOS
ID: STRESS-001
Description: High load
Load: 500 rps
Expected: Stable
---

OVERALL_RISK: 0.1`

	defenseResponse := `PATCHED_VULNERABILITIES: VULN-001
PATCHES
VULN-001: Added input validation
---
REMAINING_RISKS: NONE
CONFIDENCE: 0.9
PATCHED_CODE
` + "```go\nfunc fixed() { return }\n```"

	mock := &mockAdversarialLLM{
		responses: []string{attackResponse, defenseResponse},
	}

	cfg := DefaultAdversarialConfig()
	cfg.MaxRounds = 3
	cfg.RiskThreshold = 0.2

	ap := NewAdversarialProtocol(cfg, mock)
	ctx := context.Background()

	result, err := ap.Execute(ctx, "func buggy() {}", "go")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Risk is 0.1 < threshold 0.2, so should stop after 1 round.
	assert.Equal(t, 1, result.Rounds)
	assert.NotEmpty(t, result.FinalCode)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.Len(t, result.AttackReports, 1)

	// Verify the attack report was parsed.
	attack := result.AttackReports[0]
	assert.Equal(t, 1, attack.Round)
	assert.LessOrEqual(t, attack.OverallRisk, 0.2)
}

func TestAdversarialProtocol_Execute_FallbackOnLLMFailure(t *testing.T) {
	t.Parallel()
	// Use nil LLM client to force fallback on every call.
	// The adversarial protocol handles nil by getting an error from Complete,
	// which triggers the fallback path.
	mock := &mockAdversarialLLM{
		err: fmt.Errorf("LLM unavailable"),
	}

	cfg := AdversarialConfig{
		MaxRounds:          2,
		MinVulnerabilities: 1,
		RiskThreshold:      0.1,
		Timeout:            30 * time.Second,
	}

	ap := NewAdversarialProtocol(cfg, mock)
	ctx := context.Background()

	// Code that triggers fallback vulnerability detection
	// (no sanitize/validate/escape, so "missing input validation" found).
	code := `func process(input string) string {
	return input
}
`

	result, err := ap.Execute(ctx, code, "go")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have run at least 1 round because fallback finds vulnerabilities.
	assert.GreaterOrEqual(t, result.Rounds, 1)
	assert.NotEmpty(t, result.FinalCode)
	assert.NotEmpty(t, result.AttackReports)
	assert.NotEmpty(t, result.DefenseReports)

	// Verify the fallback defense produced patches.
	lastDefense := result.DefenseReports[len(result.DefenseReports)-1]
	assert.NotEmpty(t, lastDefense.Patches)
	assert.Equal(t, 0.4, lastDefense.ConfidenceInDefense)
}

func TestAdversarialProtocol_Execute_MaxRoundsReached(t *testing.T) {
	t.Parallel()
	// Return high-risk attacks every time to force all rounds.
	attackResponse := `VULNERABILITIES
ID: VULN-001
Category: injection
Severity: critical
Description: SQL injection found
Evidence: string concatenation
Exploit: supply malicious SQL
---

OVERALL_RISK: 0.9`

	defenseResponse := `PATCHED_VULNERABILITIES: VULN-001
PATCHES
VULN-001: Use parameterized queries
---
REMAINING_RISKS: Incomplete fix
CONFIDENCE: 0.6
PATCHED_CODE
` + "```go\nfunc patched() { return }\n```"

	// 2 rounds * 2 calls per round (attack + defense) = 4 responses needed
	// Plus a third round attack.
	mock := &mockAdversarialLLM{
		responses: []string{
			attackResponse, defenseResponse,
			attackResponse, defenseResponse,
			attackResponse, defenseResponse,
		},
	}

	cfg := AdversarialConfig{
		MaxRounds:          3,
		MinVulnerabilities: 1,
		RiskThreshold:      0.05, // Very low threshold so it does not stop early.
		Timeout:            30 * time.Second,
	}

	ap := NewAdversarialProtocol(cfg, mock)
	ctx := context.Background()

	result, err := ap.Execute(ctx, "func vulnerable() {}", "go")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 3, result.Rounds)
	assert.Len(t, result.AttackReports, 3)
	assert.Len(t, result.DefenseReports, 3)
}

func TestAdversarialProtocol_Execute_NoVulnerabilitiesFound(t *testing.T) {
	t.Parallel()
	// The attack response reports no vulnerabilities.
	attackResponse := `VULNERABILITIES

OVERALL_RISK: 0.0`

	mock := &mockAdversarialLLM{
		responses: []string{attackResponse},
	}

	cfg := DefaultAdversarialConfig()
	ap := NewAdversarialProtocol(cfg, mock)

	result, err := ap.Execute(context.Background(), "safe code", "go")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Rounds)
	assert.True(t, result.AllResolved)
	assert.Empty(t, result.RemainingRisks)
	assert.Empty(t, result.DefenseReports)
}

func TestAdversarialProtocol_GenerateFallbackAttack(t *testing.T) {
	t.Parallel()
	ap := NewAdversarialProtocol(DefaultAdversarialConfig(), nil)

	tests := []struct {
		name     string
		code     string
		language string
		minVulns int
	}{
		{
			name:     "code with no validation",
			code:     "func process(x string) { return }",
			language: "go",
			minVulns: 1, // At least "no input validation" found.
		},
		{
			name: "code with concurrent access without sync",
			code: `func run() {
	go func() { shared++ }()
	goroutine()
}`,
			language: "go",
			minVulns: 1,
		},
		{
			name:     "safe code with validation",
			code:     "func safe(x string) { validate(x); sanitize(x) }",
			language: "go",
			minVulns: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			report := ap.generateFallbackAttack(tc.code, tc.language, 1)
			require.NotNil(t, report)
			assert.GreaterOrEqual(t, len(report.Vulnerabilities), tc.minVulns)
			assert.Equal(t, 1, report.Round)
			assert.NotEmpty(t, report.EdgeCases)
			assert.NotEmpty(t, report.StressScenarios)
			assert.GreaterOrEqual(t, report.OverallRisk, 0.0)
			assert.LessOrEqual(t, report.OverallRisk, 1.0)
		})
	}
}

func TestAdversarialProtocol_GenerateFallbackAttack_LanguageEdgeCases(
	t *testing.T,
) {
	ap := NewAdversarialProtocol(DefaultAdversarialConfig(), nil)

	tests := []struct {
		language    string
		edgeContain string
	}{
		{"go", "Nil pointer"},
		{"python", "None"},
		{"javascript", "Undefined"},
		{"unknown", "Empty input"},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
				t.Parallel()
			report := ap.generateFallbackAttack(
				"x = 1", tc.language, 1,
			)
			require.NotEmpty(t, report.EdgeCases)
			assert.Contains(t, report.EdgeCases[0].Description, tc.edgeContain)
		})
	}
}

func TestAdversarialProtocol_GenerateFallbackDefense(t *testing.T) {
	t.Parallel()
	ap := NewAdversarialProtocol(DefaultAdversarialConfig(), nil)

	attack := &AttackReport{
		Vulnerabilities: []Vulnerability{
			{
				ID:          "VULN-001",
				Category:    "injection",
				Severity:    "critical",
				Description: "SQL injection",
			},
			{
				ID:          "VULN-002",
				Category:    "race_condition",
				Severity:    "high",
				Description: "Race condition on shared state",
			},
		},
		EdgeCases: []EdgeCase{
			{
				ID:          "EDGE-001",
				Description: "Nil input",
			},
		},
	}

	report := ap.generateFallbackDefense("original code", attack)
	require.NotNil(t, report)

	// All vulnerabilities should be acknowledged.
	assert.Len(t, report.PatchedVulnerabilities, 2)
	assert.Contains(t, report.PatchedVulnerabilities, "VULN-001")
	assert.Contains(t, report.PatchedVulnerabilities, "VULN-002")

	// Patches should have appropriate descriptions.
	assert.Contains(t, report.Patches["VULN-001"], "parameterized")
	assert.Contains(t, report.Patches["VULN-002"], "mutex")

	// Edge cases become remaining risks.
	assert.NotEmpty(t, report.RemainingRisks)

	// Patched code should be the original (fallback cannot patch).
	assert.Equal(t, "original code", report.PatchedCode)

	// Low confidence for fallback.
	assert.Equal(t, 0.4, report.ConfidenceInDefense)
}

func TestAdversarialProtocol_GenerateFallbackDefense_AllCategories(
	t *testing.T,
) {
	ap := NewAdversarialProtocol(DefaultAdversarialConfig(), nil)

	categories := []struct {
		category      string
		patchContains string
	}{
		{"injection", "parameterized"},
		{"overflow", "bounds checking"},
		{"race_condition", "mutex"},
		{"logic_error", "validation"},
		{"auth", "authentication"},
		{"xss", "Escape"},
		{"other_category", "defensive"},
	}

	for _, tc := range categories {
		t.Run(tc.category, func(t *testing.T) {
				t.Parallel()
			attack := &AttackReport{
				Vulnerabilities: []Vulnerability{
					{
						ID:       "VULN-001",
						Category: tc.category,
						Severity: "medium",
					},
				},
			}

			report := ap.generateFallbackDefense("code", attack)
			require.NotNil(t, report)
			require.Contains(t, report.Patches, "VULN-001")
			assert.Contains(t, report.Patches["VULN-001"], tc.patchContains)
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestContainsAny(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		s        string
		subs     []string
		expected bool
	}{
		{"match first", "hello world", []string{"hello"}, true},
		{"match last", "hello world", []string{"foo", "world"}, true},
		{"no match", "hello world", []string{"foo", "bar"}, false},
		{"empty string", "", []string{"foo"}, false},
		{"empty subs", "hello", []string{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected, containsAny(tc.s, tc.subs...))
		})
	}
}

func TestCalculateFallbackRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		vulns    []Vulnerability
		expected float64
	}{
		{
			name:     "no vulnerabilities",
			vulns:    []Vulnerability{},
			expected: 0.0,
		},
		{
			name: "single critical",
			vulns: []Vulnerability{
				{Severity: "critical"},
			},
			expected: 1.0,
		},
		{
			name: "single low",
			vulns: []Vulnerability{
				{Severity: "low"},
			},
			expected: 0.2,
		},
		{
			name: "mixed severities",
			vulns: []Vulnerability{
				{Severity: "critical"},
				{Severity: "low"},
			},
			expected: 0.6, // (1.0 + 0.2) / 2
		},
		{
			name: "unknown severity",
			vulns: []Vulnerability{
				{Severity: "unknown"},
			},
			expected: 0.3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			report := &AttackReport{Vulnerabilities: tc.vulns}
			risk := calculateFallbackRisk(report)
			assert.InDelta(t, tc.expected, risk, 0.001)
		})
	}
}

func TestExtractField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line     string
		key      string
		expected string
		ok       bool
	}{
		{"ID: VULN-001", "ID:", "VULN-001", true},
		{"Category: injection", "Category:", "injection", true},
		{"Something else", "ID:", "", false},
		{"ID:", "ID:", "", true},
		{"ID:  spaced  ", "ID:", "spaced", true},
	}

	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
				t.Parallel()
			val, ok := extractField(tc.line, tc.key)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.expected, val)
		})
	}
}

func TestCollectRemainingRisks(t *testing.T) {
	t.Parallel()
	// Case: defense has remaining risks.
	result := &AdversarialResult{
		AttackReports: []*AttackReport{
			{Vulnerabilities: []Vulnerability{{ID: "V1", Severity: "high"}}},
		},
		DefenseReports: []*DefenseReport{
			{RemainingRisks: []string{"risk A", "risk B"}},
		},
	}

	risks := collectRemainingRisks(result)
	assert.Contains(t, risks, "risk A")
	assert.Contains(t, risks, "risk B")

	// Case: more attack reports than defense reports (undefended vulns).
	result2 := &AdversarialResult{
		AttackReports: []*AttackReport{
			{Vulnerabilities: []Vulnerability{
				{ID: "V1", Severity: "high", Description: "desc1"},
			}},
			{Vulnerabilities: []Vulnerability{
				{ID: "V2", Severity: "critical", Description: "desc2"},
			}},
		},
		DefenseReports: []*DefenseReport{
			{RemainingRisks: []string{}},
		},
	}

	risks2 := collectRemainingRisks(result2)
	assert.NotEmpty(t, risks2)
	assert.Contains(t, risks2[0], "V2")
}

func TestCollectRemainingRisks_Empty(t *testing.T) {
	t.Parallel()
	result := &AdversarialResult{
		AttackReports:  []*AttackReport{},
		DefenseReports: []*DefenseReport{},
	}

	risks := collectRemainingRisks(result)
	assert.Empty(t, risks)
}
