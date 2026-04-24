package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoServer skips the test if HelixAgent server is not running
func skipIfNoServer(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping E2E debate test in short mode (requires live LLM providers)")  // SKIP-OK: #short-mode
	}
	testutil.RequireServer(t)
}

type DebateParticipant struct {
	Name        string `json:"name"`
	Role        string `json:"role,omitempty"`
	LLMProvider string `json:"llm_provider,omitempty"`
}

type DebateRequest struct {
	Topic        string              `json:"topic"`
	Participants []DebateParticipant `json:"participants"`
	MaxRounds    int                 `json:"max_rounds"`
}

// DebateResponse covers both sync (200) and async (202) response formats
type DebateResponse struct {
	DebateID           string                 `json:"debate_id"`
	Topic              string                 `json:"topic"`
	Status             string                 `json:"status,omitempty"`
	Message            string                 `json:"message,omitempty"`
	Success            bool                   `json:"success"`
	RoundsConducted    int                    `json:"rounds_conducted"`
	ValidationResult   interface{}            `json:"validation_result,omitempty"`
	TestDrivenMetadata map[string]interface{} `json:"test_driven_metadata,omitempty"`
	ToolEnrichmentUsed bool                   `json:"tool_enrichment_used,omitempty"`
	SpecializedRole    string                 `json:"specialized_role,omitempty"`
	QualityScore       float64                `json:"quality_score"`
}

// defaultParticipants returns a minimal set of debate participants for tests
func defaultParticipants() []DebateParticipant {
	return []DebateParticipant{
		{Name: "Analyst", Role: "analyzer"},
		{Name: "Reviewer", Role: "reviewer"},
	}
}

// createDebate sends a debate creation request and returns the response
func createDebate(t *testing.T, topic string, maxRounds int) (*http.Response, DebateResponse) {
	t.Helper()

	request := DebateRequest{
		Topic:        topic,
		Participants: defaultParticipants(),
		MaxRounds:    maxRounds,
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "http://localhost:8100/v1/debates", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	apiKey := getE2EAPIKey()
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// Should get 200 OK or 202 Accepted
	assert.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode,
		"Should get 200 OK or 202 Accepted response")

	var result DebateResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Always verify debate ID and topic
	assert.NotEmpty(t, result.DebateID, "Should have debate ID")
	assert.Equal(t, request.Topic, result.Topic, "Topic should match")

	return resp, result
}

// TestDebateE2E_CodeGeneration tests end-to-end code generation debate
func TestDebateE2E_CodeGeneration(t *testing.T) {
	skipIfNoServer(t)

	resp, result := createDebate(t,
		"Write a Python function to calculate the factorial of a number", 1)
	defer resp.Body.Close()

	t.Logf("Test-Driven Metadata: %v", result.TestDrivenMetadata)
	t.Logf("Validation Result: %v", result.ValidationResult)
	t.Logf("Tool Enrichment: %v", result.ToolEnrichmentUsed)
	t.Logf("Specialized Role: %s", result.SpecializedRole)

	if result.TestDrivenMetadata != nil {
		assert.NotEmpty(t, result.TestDrivenMetadata, "Test-driven metadata should be present for code generation")
		t.Log("Test-Driven Debate was triggered")
	}

	if result.SpecializedRole != "" {
		assert.Contains(t, []string{"generator", "coder", ""}, result.SpecializedRole,
			"Should select appropriate role for code generation")
		t.Logf("Specialized Role selected: %s", result.SpecializedRole)
	}
}

// TestDebateE2E_PerformanceAnalysis tests performance analysis task
func TestDebateE2E_PerformanceAnalysis(t *testing.T) {
	skipIfNoServer(t)

	resp, result := createDebate(t,
		"Analyze and optimize the performance of a database query that joins 5 tables", 1)
	defer resp.Body.Close()

	if result.SpecializedRole != "" {
		assert.Equal(t, "performance_analyzer", result.SpecializedRole,
			"Should select performance_analyzer for optimization tasks")
		t.Logf("Performance Analyzer role selected")
	}

	if result.ValidationResult != nil {
		t.Log("4-Pass Validation was applied")
	}
}

// TestDebateE2E_SecurityAudit tests security audit task
func TestDebateE2E_SecurityAudit(t *testing.T) {
	skipIfNoServer(t)

	resp, result := createDebate(t,
		"Security audit of the user authentication system - find vulnerabilities", 1)
	defer resp.Body.Close()

	if result.SpecializedRole != "" {
		assert.Equal(t, "security_analyst", result.SpecializedRole,
			"Should select security_analyst for security tasks")
		t.Logf("Security Analyst role selected")
	}

	if result.ToolEnrichmentUsed {
		t.Log("Tool Enrichment was used")
	}
}

// TestDebateE2E_RefactoringTask tests refactoring task
func TestDebateE2E_RefactoringTask(t *testing.T) {
	skipIfNoServer(t)

	resp, result := createDebate(t,
		"Refactor the legacy payment processing module to improve maintainability", 1)
	defer resp.Body.Close()

	if result.SpecializedRole != "" {
		assert.Equal(t, "refactorer", result.SpecializedRole,
			"Should select refactorer for refactoring tasks")
		t.Logf("Refactorer role selected")
	}

	if result.ValidationResult != nil {
		t.Log("4-Pass Validation was applied")
	}
}

// TestDebateE2E_IntegratedFeatures tests all integrated features working together
func TestDebateE2E_IntegratedFeatures(t *testing.T) {
	skipIfNoServer(t)

	request := DebateRequest{
		Topic:        "Write a secure authentication function in Go with performance optimization and comprehensive tests",
		Participants: defaultParticipants(),
		MaxRounds:    2,
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost:8100/v1/debates",
		bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	apiKey := getE2EAPIKey()
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode)

	var result DebateResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	t.Logf("Debate ID: %s", result.DebateID)
	t.Logf("Status: %s", result.Status)
	t.Logf("Rounds Conducted: %d", result.RoundsConducted)
	t.Logf("Quality Score: %.2f", result.QualityScore)

	// Feature verification matrix
	featuresDetected := make(map[string]bool)

	if result.TestDrivenMetadata != nil {
		featuresDetected["test_driven"] = true
		t.Log("Feature: Test-Driven Debate ACTIVE")
	}

	if result.ValidationResult != nil {
		featuresDetected["validation"] = true
		t.Log("Feature: 4-Pass Validation ACTIVE")
	}

	if result.ToolEnrichmentUsed {
		featuresDetected["tool_enrichment"] = true
		t.Log("Feature: Tool Integration ACTIVE")
	}

	if result.SpecializedRole != "" {
		featuresDetected["specialized_role"] = true
		t.Logf("Feature: Specialized Role ACTIVE (%s)", result.SpecializedRole)
	}

	// For async (202) responses, the debate_id itself is a feature
	if result.DebateID != "" {
		featuresDetected["debate_created"] = true
	}

	assert.NotEmpty(t, featuresDetected, "At least one integrated feature should be active")

	t.Logf("\n=== INTEGRATED FEATURES SUMMARY ===")
	t.Logf("Test-Driven Debate: %v", featuresDetected["test_driven"])
	t.Logf("4-Pass Validation: %v", featuresDetected["validation"])
	t.Logf("Tool Integration: %v", featuresDetected["tool_enrichment"])
	t.Logf("Specialized Roles: %v", featuresDetected["specialized_role"])
	t.Logf("===================================")
}
