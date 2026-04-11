package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.debate/agents"
)

// mockAdversarialLLM implements agents.AdversarialLLMClient for integration
// testing. It returns pre-defined structured responses so the adversarial
// protocol can parse attack and defense reports without a live LLM.
type mockAdversarialLLM struct {
	attackResponses  []string
	defenseResponses []string
	attackCall       int
	defenseCall      int
}

func (m *mockAdversarialLLM) Complete(ctx context.Context, prompt string) (string, error) {
	// Detect whether this is an attack or defense prompt. The defense
	// prompt also contains the word "vulnerabilities" (in the
	// "Vulnerabilities found:" section echoed back to the LLM), so
	// the mock must key on the unique "Blue Team" / "Red Team" role
	// markers rather than a content keyword.
	switch {
	case containsStr(prompt, "Blue Team"):
		if m.defenseCall < len(m.defenseResponses) {
			resp := m.defenseResponses[m.defenseCall]
			m.defenseCall++
			return resp, nil
		}
		m.defenseCall++
		return "", fmt.Errorf("no more defense responses")
	case containsStr(prompt, "Red Team"):
		if m.attackCall < len(m.attackResponses) {
			resp := m.attackResponses[m.attackCall]
			m.attackCall++
			return resp, nil
		}
		m.attackCall++
		return "", fmt.Errorf("no more attack responses")
	}
	// Unknown prompt — fall back to empty to exercise the protocol's
	// deterministic fallback path.
	return "", fmt.Errorf("unknown prompt type")
}

// containsStr is a simple helper to avoid importing strings in tests.
func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestAdversarial_AttackDefendCycle verifies the complete red team attack
// and blue team defense cycle using the adversarial protocol.
func TestAdversarial_AttackDefendCycle(t *testing.T) {

	llm := &mockAdversarialLLM{
		attackResponses: []string{
			// Round 1 attack: structured response with vulnerabilities
			"VULNERABILITIES\n" +
				"ID: VULN-001\n" +
				"Category: injection\n" +
				"Severity: critical\n" +
				"Description: SQL injection via string concatenation\n" +
				"Evidence: query uses fmt.Sprintf\n" +
				"Exploit: supply malicious SQL in input\n" +
				"---\n" +
				"EDGE_CASES\n" +
				"ID: EDGE-001\n" +
				"Description: Empty input string\n" +
				"Input: empty string\n" +
				"Expected: graceful error\n" +
				"---\n" +
				"STRESS_SCENARIOS\n" +
				"ID: STRESS-001\n" +
				"Description: High concurrency\n" +
				"Load: 1000 concurrent requests\n" +
				"Expected: no crashes\n" +
				"---\n" +
				"OVERALL_RISK: 0.8\n",
			// Round 2 attack: fewer issues after patches
			"VULNERABILITIES\n" +
				"---\n" +
				"EDGE_CASES\n" +
				"---\n" +
				"STRESS_SCENARIOS\n" +
				"---\n" +
				"OVERALL_RISK: 0.1\n",
		},
		defenseResponses: []string{
			// Round 1 defense: patch the injection vulnerability
			"PATCHED_VULNERABILITIES: VULN-001\n" +
				"PATCHES\n" +
				"VULN-001: Use parameterized queries instead of string concatenation\n" +
				"---\n" +
				"REMAINING_RISKS: NONE\n" +
				"CONFIDENCE: 0.85\n" +
				"PATCHED_CODE\n" +
				"```go\n" +
				"func query(db *sql.DB, id string) error {\n" +
				"  _, err := db.Query(\"SELECT * FROM users WHERE id = $1\", id)\n" +
				"  return err\n" +
				"}\n" +
				"```\n",
		},
	}

	config := agents.AdversarialConfig{
		MaxRounds:          3,
		MinVulnerabilities: 1,
		RiskThreshold:      0.2,
		Timeout:            30 * time.Second,
	}

	protocol := agents.NewAdversarialProtocol(config, llm)

	solution := `func query(db *sql.DB, id string) error {
  q := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", id)
  _, err := db.Query(q)
  return err
}`

	ctx := context.Background()
	result, err := protocol.Execute(ctx, solution, "go")

	require.NoError(t, err, "Adversarial protocol should not error")
	require.NotNil(t, result, "Should return a result")

	// Verify attack/defense cycle occurred
	assert.Greater(t, len(result.AttackReports), 0,
		"Should have at least one attack report")
	assert.Greater(t, len(result.DefenseReports), 0,
		"Should have at least one defense report")
	assert.Greater(t, result.Rounds, 0,
		"Should have completed at least one round")

	// The first attack should have found the SQL injection
	firstAttack := result.AttackReports[0]
	assert.Greater(t, len(firstAttack.Vulnerabilities), 0,
		"First attack should find vulnerabilities")
	assert.Equal(t, "VULN-001", firstAttack.Vulnerabilities[0].ID,
		"Should identify VULN-001")
	assert.Equal(t, "injection", firstAttack.Vulnerabilities[0].Category,
		"Should categorize as injection")
	assert.Equal(t, "critical", firstAttack.Vulnerabilities[0].Severity,
		"Should rate as critical severity")

	// The defense should have patched the vulnerability
	firstDefense := result.DefenseReports[0]
	assert.Contains(t, firstDefense.PatchedVulnerabilities, "VULN-001",
		"Defense should patch VULN-001")
	assert.NotEmpty(t, firstDefense.Patches,
		"Defense should include patch descriptions")
	assert.Greater(t, firstDefense.ConfidenceInDefense, 0.0,
		"Defense should have non-zero confidence")

	// Final code should be updated
	assert.NotEmpty(t, result.FinalCode,
		"Final code should not be empty")

	t.Logf("Adversarial: %d rounds, %d attacks, %d defenses, "+
		"all resolved: %v, duration: %v",
		result.Rounds, len(result.AttackReports),
		len(result.DefenseReports), result.AllResolved, result.Duration)
}

// TestAdversarial_FallbackMode verifies that the adversarial protocol
// uses deterministic fallback when the LLM is unavailable.
func TestAdversarial_FallbackMode(t *testing.T) {

	// LLM client that always fails
	failingLLM := &failingAdversarialLLM{}

	config := agents.DefaultAdversarialConfig()
	config.MaxRounds = 2
	config.Timeout = 15 * time.Second

	protocol := agents.NewAdversarialProtocol(config, failingLLM)

	solution := `func processInput(input string) string {
  // No validation
  result := fmt.Sprintf("SELECT * FROM data WHERE value = '%s'", input)
  go func() { sharedState++ }()
  return result
}`

	ctx := context.Background()
	result, err := protocol.Execute(ctx, solution, "go")

	require.NoError(t, err, "Fallback mode should not error")
	require.NotNil(t, result, "Should return a result")

	// Fallback should still find deterministic vulnerabilities
	assert.Greater(t, len(result.AttackReports), 0,
		"Fallback should produce attack reports")

	if len(result.AttackReports) > 0 {
		firstAttack := result.AttackReports[0]
		assert.Greater(t, len(firstAttack.Vulnerabilities), 0,
			"Fallback should detect vulnerabilities")
		assert.Greater(t, firstAttack.OverallRisk, 0.0,
			"Fallback should compute a risk score")
	}

	t.Logf("Fallback mode: %d rounds, %d attack reports",
		result.Rounds, len(result.AttackReports))
}

// TestAdversarial_CleanCodeTerminatesEarly verifies that the adversarial
// protocol terminates when no significant vulnerabilities are found.
func TestAdversarial_CleanCodeTerminatesEarly(t *testing.T) {

	// LLM that returns no vulnerabilities
	cleanLLM := &mockAdversarialLLM{
		attackResponses: []string{
			"VULNERABILITIES\n---\n" +
				"EDGE_CASES\n---\n" +
				"STRESS_SCENARIOS\n---\n" +
				"OVERALL_RISK: 0.05\n",
		},
	}

	config := agents.AdversarialConfig{
		MaxRounds:          5,
		MinVulnerabilities: 1,
		RiskThreshold:      0.2,
		Timeout:            15 * time.Second,
	}

	protocol := agents.NewAdversarialProtocol(config, cleanLLM)

	solution := `func safeQuery(db *sql.DB, id string) (*User, error) {
  if id == "" { return nil, errors.New("empty id") }
  row := db.QueryRow("SELECT * FROM users WHERE id = $1", id)
  var u User
  if err := row.Scan(&u.ID, &u.Name); err != nil { return nil, err }
  return &u, nil
}`

	ctx := context.Background()
	result, err := protocol.Execute(ctx, solution, "go")

	require.NoError(t, err, "Clean code should not cause errors")
	require.NotNil(t, result, "Should return a result")

	// Should terminate after 1 round since no significant vulns found
	assert.Equal(t, 1, result.Rounds,
		"Should terminate after 1 round for clean code")

	t.Logf("Clean code: terminated after %d rounds, remaining risks: %d",
		result.Rounds, len(result.RemainingRisks))
}

// failingAdversarialLLM always returns errors.
type failingAdversarialLLM struct{}

func (f *failingAdversarialLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "", fmt.Errorf("LLM service unavailable")
}
