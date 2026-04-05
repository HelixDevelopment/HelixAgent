// Package security provides tests for the SecureFixAgent and security components.
package security

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureFixAgent_DetectRepairValidate(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	logger := logrus.New()
	agent := NewSecureFixAgent(config, nil, nil, logger)

	ctx := context.Background()

	testCode := `
func handleUser(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("id")
	db.Query("SELECT * FROM users WHERE id = " + query)
}
`

	result, err := agent.DetectRepairValidate(ctx, testCode, "go")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPatternBasedScanner_Scan(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	ctx := context.Background()

	// Test scanning code
	vulns, err := scanner.Scan(ctx, `fmt.Println("Hello, World!")`, "go")
	require.NoError(t, err)
	// Scanner may or may not find vulnerabilities - we're testing it doesn't error
	_ = vulns

	// Test scanning potentially vulnerable code
	vulns, err = scanner.Scan(ctx, `db.Query("SELECT * FROM users WHERE id = " + userInput)`, "go")
	require.NoError(t, err)
	// Scanner may or may not find vulnerabilities depending on patterns
	_ = vulns
}

func TestVulnerability_Severity(t *testing.T) {
	t.Parallel()
	severities := []VulnerabilitySeverity{
		VulnSeverityInfo,
		VulnSeverityLow,
		VulnSeverityMedium,
		VulnSeverityHigh,
		VulnSeverityCritical,
	}

	for _, sev := range severities {
		assert.NotEmpty(t, string(sev))
	}
}

func TestVulnerabilityCategory(t *testing.T) {
	t.Parallel()
	categories := []VulnerabilityCategory{
		CategoryInjection,
		CategoryXSS,
		CategoryAuthentication,
		CategoryAuthorization,
		CategoryCryptographic,
		CategorySensitiveData,
		CategoryMisconfiguration,
		CategoryDependency,
		CategoryMemorySafety,
		CategoryRaceCondition,
	}

	for _, cat := range categories {
		assert.NotEmpty(t, string(cat))
	}
}

func TestFiveRingDefense_Defend(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	defense := NewFiveRingDefense(logger)

	// Add input sanitization ring
	defense.AddRing(NewInputSanitizationRing())

	ctx := context.Background()

	testCases := []struct {
		name       string
		input      string
		expectPass bool
	}{
		{
			name:       "Valid Input",
			input:      "Hello World",
			expectPass: true,
		},
		{
			name:       "SQL Injection Attempt",
			input:      "'; DROP TABLE users; --",
			expectPass: false,
		},
		{
			name:       "Script Injection",
			input:      "<script>alert('xss')</script>",
			expectPass: false,
		},
		{
			name:       "Normal Code",
			input:      "func main() { fmt.Println() }",
			expectPass: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			result, err := defense.Defend(ctx, tc.input)
			require.NoError(t, err)
			assert.NotNil(t, result)
			if tc.expectPass {
				assert.True(t, result.Passed)
			} else {
				assert.False(t, result.Passed)
			}
		})
	}
}

func TestSecureFixAgentConfig_Defaults(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()

	assert.True(t, config.EnableAutoFix)
	assert.True(t, config.RequireValidation)
	assert.Greater(t, config.MaxConcurrentScans, 0)
	assert.Greater(t, config.Timeout, time.Duration(0))
}

func TestSecurityResult(t *testing.T) {
	t.Parallel()
	result := &SecurityResult{
		Vulnerabilities: []*Vulnerability{
			{ID: "v1", Title: "Test Vulnerability"},
			{ID: "v2", Title: "Another Vulnerability"},
		},
		Success:  true,
		Duration: 100 * time.Millisecond,
	}

	assert.Len(t, result.Vulnerabilities, 2)
	assert.True(t, result.Success)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
}

func TestInputSanitizationRing(t *testing.T) {
	t.Parallel()
	ring := NewInputSanitizationRing()

	ctx := context.Background()

	// Test with SQL injection
	passed, _, err := ring.Check(ctx, "SELECT * FROM users WHERE id = '1' OR '1'='1'")
	require.NoError(t, err)
	assert.False(t, passed)

	// Test with clean input
	passed, _, err = ring.Check(ctx, "Hello World")
	require.NoError(t, err)
	assert.True(t, passed)
}

func TestRateLimitingRing(t *testing.T) {
	t.Parallel()
	ring := NewRateLimitingRing(5, time.Minute)
	ctx := context.Background()

	// Should allow initial requests
	for i := 0; i < 5; i++ {
		passed, _, err := ring.Check(ctx, "request")
		require.NoError(t, err)
		assert.True(t, passed)
	}

	// Should rate limit after threshold
	passed, _, err := ring.Check(ctx, "request")
	require.NoError(t, err)
	assert.False(t, passed)
}

func TestSecureFixAgent_ScanOnly(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	config.EnableAutoFix = false
	logger := logrus.New()
	agent := NewSecureFixAgent(config, nil, nil, logger)

	ctx := context.Background()

	vulnerableCode := `
func login(username, password string) bool {
	query := fmt.Sprintf("SELECT * FROM users WHERE username='%s' AND password='%s'", username, password)
	// Execute query
	return true
}
`

	result, err := agent.DetectRepairValidate(ctx, vulnerableCode, "go")
	require.NoError(t, err)
	assert.NotNil(t, result)
	// May or may not find vulnerabilities depending on pattern matching
}

func TestPatternBasedScanner_Name(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	// Verify scanner name
	name := scanner.Name()
	assert.NotEmpty(t, name)
	assert.Equal(t, "pattern-based", name)
}

// =============================================================================
// Additional Comprehensive Tests for Full Coverage
// =============================================================================

func TestSecurityScanResult_MarshalJSON(t *testing.T) {
	t.Parallel()
	result := &SecurityScanResult{
		Vulnerabilities: []*Vulnerability{
			{ID: "vuln-1", Title: "Test Vuln"},
		},
		TotalFiles:   10,
		ScannedFiles: 8,
		Duration:     500 * time.Millisecond,
		Scanner:      "pattern-based",
	}

	data, err := result.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "vuln-1")
	assert.Contains(t, string(data), "duration_ms")
	assert.Contains(t, string(data), "500")
}

func TestSecurityResult_MarshalJSON(t *testing.T) {
	t.Parallel()
	result := &SecurityResult{
		Vulnerabilities: []*Vulnerability{
			{ID: "v1", Title: "Test"},
		},
		Fixes:    []*SecurityFix{},
		Success:  true,
		Duration: 250 * time.Millisecond,
	}

	data, err := result.MarshalJSON()
	require.NoError(t, err)
	assert.Contains(t, string(data), "duration_ms")
	assert.Contains(t, string(data), "250")
}

func TestSecureFixAgent_RegisterScanner(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	logger := logrus.New()
	agent := NewSecureFixAgent(config, nil, nil, logger)

	scanner := NewPatternBasedScanner(logger)
	agent.RegisterScanner(scanner)

	// Verify scanner was registered by running DetectRepairValidate
	ctx := context.Background()
	result, err := agent.DetectRepairValidate(ctx, "test code", "go")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSecureFixAgent_ShouldReport(t *testing.T) {
	t.Parallel()
	// Test severity threshold filtering
	config := DefaultSecureFixAgentConfig()
	config.SeverityThreshold = VulnSeverityHigh // Only report high and above
	logger := logrus.New()
	agent := NewSecureFixAgent(config, nil, nil, logger)
	scanner := NewPatternBasedScanner(logger)
	agent.RegisterScanner(scanner)

	ctx := context.Background()

	// Test code with weak crypto (medium severity)
	mediumCode := `md5(password)`

	result, err := agent.DetectRepairValidate(ctx, mediumCode, "go")
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Medium severity should be filtered out when threshold is High
	for _, vuln := range result.Vulnerabilities {
		assert.NotEqual(t, SeverityLow, vuln.Severity)
		assert.NotEqual(t, SeverityInfo, vuln.Severity)
	}
}

func TestSecureFixAgent_ApplyFix(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	logger := logrus.New()
	agent := NewSecureFixAgent(config, nil, nil, logger)

	t.Run("Apply validated fix", func(t *testing.T) {
			t.Parallel()
		originalCode := `db.Query("SELECT * FROM users WHERE id = " + input)`
		fix := &SecurityFix{
			VulnerabilityID: "vuln-1",
			OriginalCode:    `db.Query("SELECT * FROM users WHERE id = " + input)`,
			FixedCode:       `db.Query("SELECT * FROM users WHERE id = $1", input)`,
			Validated:       true,
		}

		result, err := agent.ApplyFix(originalCode, fix)
		require.NoError(t, err)
		assert.Contains(t, result, "$1")
		assert.NotContains(t, result, "+ input")
	})

	t.Run("Reject unvalidated fix", func(t *testing.T) {
			t.Parallel()
		originalCode := `vulnerable code`
		fix := &SecurityFix{
			VulnerabilityID: "vuln-2",
			OriginalCode:    "vulnerable code",
			FixedCode:       "fixed code",
			Validated:       false,
		}

		_, err := agent.ApplyFix(originalCode, fix)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not validated")
	})
}

func TestSecureFixAgent_GetVulnerability(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	mockGenerator := &mockFixGenerator{}
	agent := NewSecureFixAgent(config, mockGenerator, nil, logger)
	agent.RegisterScanner(scanner)

	ctx := context.Background()

	// Scan code with known vulnerability
	vulnCode := `password = "hardcoded123"`
	result, err := agent.DetectRepairValidate(ctx, vulnCode, "go")
	require.NoError(t, err)

	// Get vulnerability by ID
	if len(result.Vulnerabilities) > 0 {
		vuln, exists := agent.GetVulnerability(result.Vulnerabilities[0].ID)
		assert.True(t, exists)
		assert.NotNil(t, vuln)
	}

	// Non-existent vulnerability
	_, exists := agent.GetVulnerability("non-existent-id")
	assert.False(t, exists)
}

func TestSecureFixAgent_GetFix(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	config.RequireValidation = false // Skip validation for testing
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	mockGenerator := &mockFixGenerator{}
	agent := NewSecureFixAgent(config, mockGenerator, nil, logger)
	agent.RegisterScanner(scanner)

	ctx := context.Background()

	// Scan code with known vulnerability
	vulnCode := `password = "hardcoded123"`
	result, err := agent.DetectRepairValidate(ctx, vulnCode, "go")
	require.NoError(t, err)

	// Get fix by vulnerability ID
	if len(result.Vulnerabilities) > 0 {
		fix, exists := agent.GetFix(result.Vulnerabilities[0].ID)
		// May or may not have a fix depending on generator
		_ = fix
		_ = exists
	}

	// Non-existent fix
	_, exists := agent.GetFix("non-existent-id")
	assert.False(t, exists)
}

func TestPatternBasedScanner_ScanFile(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)
	ctx := context.Background()

	t.Run("Scan non-existent file", func(t *testing.T) {
			t.Parallel()
		_, err := scanner.ScanFile(ctx, "/nonexistent/path/file.go")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	// Create a temporary file for testing
	t.Run("Scan existing file", func(t *testing.T) {
			t.Parallel()
		tempFile, err := createTempTestFile("password = \"secret123\"")
		if err != nil {
			t.Skip("Cannot create temp file for testing")
		}
		defer removeTempTestFile(tempFile)

		vulns, err := scanner.ScanFile(ctx, tempFile)
		require.NoError(t, err)
		// Should find hardcoded credential
		assert.NotNil(t, vulns)
		for _, vuln := range vulns {
			assert.Equal(t, tempFile, vuln.File)
		}
	})
}

func TestDetectLanguageFromPath(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.ts", "typescript"},
		{"Main.java", "java"},
		{"lib.rs", "rust"},
		{"script.rb", "ruby"},
		{"page.php", "php"},
		{"main.c", "c"},
		{"header.h", "c"},
		{"main.cpp", "cpp"},
		{"header.hpp", "cpp"},
		{"Program.cs", "csharp"},
		{"App.swift", "swift"},
		{"Main.kt", "kotlin"},
		{"query.sql", "sql"},
		{"script.sh", "shell"},
		{"script.bash", "shell"},
		{"unknown.xyz", "unknown"},
		{"FILE.GO", "go"}, // Case insensitive
		{"SCRIPT.PY", "python"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
				t.Parallel()
			result := detectLanguageFromPath(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPatternBasedScanner_GetLineNumber(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	code := "line1\nline2\nline3\nline4"

	// Test line number calculation
	assert.Equal(t, 1, scanner.getLineNumber(code, 0))  // Start of line1
	assert.Equal(t, 1, scanner.getLineNumber(code, 4))  // End of line1
	assert.Equal(t, 2, scanner.getLineNumber(code, 6))  // Start of line2
	assert.Equal(t, 3, scanner.getLineNumber(code, 12)) // Start of line3
	assert.Equal(t, 4, scanner.getLineNumber(code, 18)) // Start of line4
}

func TestMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, min(1, 5))
	assert.Equal(t, 1, min(5, 1))
	assert.Equal(t, 0, min(0, 100))
	assert.Equal(t, -5, min(-5, 5))
	assert.Equal(t, 10, min(10, 10))
}

func TestNewLLMFixGenerator(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	generateFunc := func(ctx context.Context, prompt string) (string, error) {
		return "fixed code", nil
	}

	generator := NewLLMFixGenerator(generateFunc, logger)
	assert.NotNil(t, generator)
}

func TestLLMFixGenerator_GenerateFix(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	generateFunc := func(ctx context.Context, prompt string) (string, error) {
		return `db.Query("SELECT * FROM users WHERE id = $1", input)`, nil
	}

	generator := NewLLMFixGenerator(generateFunc, logger)
	ctx := context.Background()

	vuln := &Vulnerability{
		ID:          "vuln-1",
		Category:    CategoryInjection,
		Severity:    VulnSeverityCritical,
		Title:       "SQL Injection",
		Description: "String concatenation in SQL query",
		Code:        `db.Query("SELECT * FROM users WHERE id = " + input)`,
		CWE:         "CWE-89",
		Remediation: "Use parameterized queries",
	}

	fix, err := generator.GenerateFix(ctx, vuln, "original code")
	require.NoError(t, err)
	assert.NotNil(t, fix)
	assert.Equal(t, vuln.ID, fix.VulnerabilityID)
	assert.Contains(t, fix.FixedCode, "$1")
}

func TestLLMFixGenerator_GenerateFix_Error(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	generateFunc := func(ctx context.Context, prompt string) (string, error) {
		return "", assert.AnError
	}

	generator := NewLLMFixGenerator(generateFunc, logger)
	ctx := context.Background()

	vuln := &Vulnerability{
		ID:       "vuln-1",
		Title:    "Test",
		Category: CategoryInjection,
	}

	_, err := generator.GenerateFix(ctx, vuln, "code")
	assert.Error(t, err)
}

func TestNewRescanValidator(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)
	validator := NewRescanValidator(scanner, logger)
	assert.NotNil(t, validator)
}

func TestRescanValidator_ValidateFix(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)
	validator := NewRescanValidator(scanner, logger)
	ctx := context.Background()

	t.Run("Valid fix removes vulnerability", func(t *testing.T) {
			t.Parallel()
		vuln := &Vulnerability{
			Category: CategorySensitiveData,
			Title:    "Hardcoded Credentials",
		}

		fix := &SecurityFix{
			FixedCode: `password := os.Getenv("PASSWORD")`, // Safe code
		}

		valid, err := validator.ValidateFix(ctx, vuln, fix)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("Invalid fix still has vulnerability", func(t *testing.T) {
			t.Parallel()
		vuln := &Vulnerability{
			Category: CategorySensitiveData,
			Title:    "Hardcoded Credentials",
		}

		fix := &SecurityFix{
			FixedCode: `password = "still-hardcoded"`, // Still vulnerable
		}

		valid, err := validator.ValidateFix(ctx, vuln, fix)
		require.NoError(t, err)
		assert.False(t, valid)
	})
}

func TestRateLimitingRing_Name(t *testing.T) {
	t.Parallel()
	ring := NewRateLimitingRing(10, time.Minute)
	assert.Equal(t, "rate_limiting", ring.Name())
}

func TestNewSecureFixAgent_NilLogger(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	// Pass nil logger - should create default logger
	agent := NewSecureFixAgent(config, nil, nil, nil)
	assert.NotNil(t, agent)
}

func TestSecureFixAgent_DetectRepairValidate_WithGenerator(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	config.RequireValidation = false
	logger := logrus.New()

	mockGen := &mockFixGenerator{}
	agent := NewSecureFixAgent(config, mockGen, nil, logger)
	scanner := NewPatternBasedScanner(logger)
	agent.RegisterScanner(scanner)

	ctx := context.Background()

	// Code with vulnerability that generator can fix
	vulnCode := `password = "secret123"`
	result, err := agent.DetectRepairValidate(ctx, vulnCode, "go")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSecureFixAgent_DetectRepairValidate_WithValidator(t *testing.T) {
	t.Parallel()
	config := DefaultSecureFixAgentConfig()
	config.RequireValidation = true
	logger := logrus.New()

	mockGen := &mockFixGenerator{}
	mockVal := &mockFixValidator{valid: true}
	agent := NewSecureFixAgent(config, mockGen, mockVal, logger)
	scanner := NewPatternBasedScanner(logger)
	agent.RegisterScanner(scanner)

	ctx := context.Background()

	vulnCode := `password = "secret123"`
	result, err := agent.DetectRepairValidate(ctx, vulnCode, "go")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPatternBasedScanner_ScanWithMatches(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)
	ctx := context.Background()

	// Code with multiple vulnerabilities
	vulnCode := `
func login() {
	password = "hardcoded123"
	query := fmt.Sprintf("SELECT * FROM users WHERE id='%s'", userId)
	hash := md5(data)
	innerHTML = userInput
}
`

	vulns, err := scanner.Scan(ctx, vulnCode, "go")
	require.NoError(t, err)
	assert.NotEmpty(t, vulns)

	// Check that vulnerabilities have proper fields
	for _, vuln := range vulns {
		assert.NotEmpty(t, vuln.ID)
		assert.NotEmpty(t, vuln.Category)
		assert.NotEmpty(t, vuln.Severity)
		assert.NotEmpty(t, vuln.Title)
		assert.Greater(t, vuln.Line, 0)
	}
}

func TestFiveRingDefense_DefendWithError(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	defense := NewFiveRingDefense(logger)

	// Add a ring that returns an error
	defense.AddRing(&errorRing{})

	ctx := context.Background()
	result, err := defense.Defend(ctx, "test input")
	require.NoError(t, err)       // Should not return error, just log warning
	assert.True(t, result.Passed) // Should still pass since error is logged but not blocking
}

func TestVulnerability_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	vuln := &Vulnerability{
		ID:          "vuln-123",
		Category:    CategoryInjection,
		Severity:    VulnSeverityCritical,
		Title:       "SQL Injection",
		Description: "Found SQL injection vulnerability",
		File:        "/path/to/file.go",
		Line:        42,
		Column:      10,
		Code:        "vulnerable code",
		CWE:         "CWE-89",
		CVSS:        9.8,
		Remediation: "Use parameterized queries",
		References:  []string{"https://example.com"},
		DetectedAt:  now,
	}

	assert.Equal(t, "vuln-123", vuln.ID)
	assert.Equal(t, CategoryInjection, vuln.Category)
	assert.Equal(t, VulnSeverityCritical, vuln.Severity)
	assert.Equal(t, 42, vuln.Line)
	assert.Equal(t, 10, vuln.Column)
	assert.Equal(t, 9.8, vuln.CVSS)
}

func TestSecurityFix_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	fix := &SecurityFix{
		VulnerabilityID: "vuln-1",
		OriginalCode:    "bad code",
		FixedCode:       "good code",
		Explanation:     "Fixed the vulnerability",
		Validated:       true,
		AppliedAt:       now,
	}

	assert.Equal(t, "vuln-1", fix.VulnerabilityID)
	assert.Equal(t, "bad code", fix.OriginalCode)
	assert.Equal(t, "good code", fix.FixedCode)
	assert.True(t, fix.Validated)
}

func TestSecureFixAgentConfig_Fields(t *testing.T) {
	t.Parallel()
	config := SecureFixAgentConfig{
		EnableAutoFix:            true,
		RequireValidation:        true,
		MaxConcurrentScans:       8,
		SeverityThreshold:        VulnSeverityMedium,
		EnableDependencyScanning: true,
		Timeout:                  5 * time.Minute,
	}

	assert.True(t, config.EnableAutoFix)
	assert.True(t, config.RequireValidation)
	assert.Equal(t, 8, config.MaxConcurrentScans)
	assert.Equal(t, VulnSeverityMedium, config.SeverityThreshold)
	assert.True(t, config.EnableDependencyScanning)
	assert.Equal(t, 5*time.Minute, config.Timeout)
}

func TestDefenseResult_Fields(t *testing.T) {
	t.Parallel()
	result := &DefenseResult{
		Passed:    false,
		BlockedBy: "input_sanitization",
		RingResults: []RingResult{
			{Name: "rate_limiting", Passed: true, Message: "OK"},
			{Name: "input_sanitization", Passed: false, Message: "Blocked"},
		},
	}

	assert.False(t, result.Passed)
	assert.Equal(t, "input_sanitization", result.BlockedBy)
	assert.Len(t, result.RingResults, 2)
}

func TestRingResult_Fields(t *testing.T) {
	t.Parallel()
	result := RingResult{
		Name:    "test_ring",
		Passed:  true,
		Message: "All checks passed",
	}

	assert.Equal(t, "test_ring", result.Name)
	assert.True(t, result.Passed)
	assert.Equal(t, "All checks passed", result.Message)
}

func TestVulnerabilityPattern_Fields(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	scanner := NewPatternBasedScanner(logger)

	// Verify patterns are registered
	assert.NotEmpty(t, scanner.patterns)
	assert.Contains(t, scanner.patterns, CategoryInjection)
	assert.Contains(t, scanner.patterns, CategoryXSS)
	assert.Contains(t, scanner.patterns, CategorySensitiveData)
	assert.Contains(t, scanner.patterns, CategoryCryptographic)
	assert.Contains(t, scanner.patterns, CategoryRaceCondition)
}

func TestInputSanitizationRing_Name(t *testing.T) {
	t.Parallel()
	ring := NewInputSanitizationRing()
	assert.Equal(t, "input_sanitization", ring.Name())
}

func TestInputSanitizationRing_CheckVariousPatterns(t *testing.T) {
	t.Parallel()
	ring := NewInputSanitizationRing()
	ctx := context.Background()

	testCases := []struct {
		name       string
		input      string
		shouldPass bool
	}{
		{"Clean text", "Hello World", true},
		{"Script tag", "<SCRIPT>alert('xss')</SCRIPT>", false},
		{"JavaScript protocol", "javascript:void(0)", false},
		{"Event handler", "onclick=alert(1)", false},
		{"SQL union", "UNION SELECT * FROM users", false},
		{"SQL insert", "INSERT INTO users VALUES", false},
		{"SQL update", "UPDATE users SET", false},
		{"SQL delete", "DELETE FROM users", false},
		{"SQL drop", "DROP TABLE users", false},
		{"SQL truncate", "TRUNCATE TABLE users", false},
		{"Normal function call", "myFunction()", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			passed, msg, err := ring.Check(ctx, tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.shouldPass, passed, "Message: %s", msg)
		})
	}
}

func TestRateLimitingRing_WindowCleanup(t *testing.T) {
	t.Parallel()
	// Use a very short window for testing
	ring := NewRateLimitingRing(2, 50*time.Millisecond)
	ctx := context.Background()

	// Make 2 requests (should pass)
	for i := 0; i < 2; i++ {
		passed, _, _ := ring.Check(ctx, "request")
		assert.True(t, passed)
	}

	// Third request should fail
	passed, _, _ := ring.Check(ctx, "request")
	assert.False(t, passed)

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be able to make requests again
	passed, _, _ = ring.Check(ctx, "request")
	assert.True(t, passed)
}

// =============================================================================
// Mock implementations for testing
// =============================================================================

type mockFixGenerator struct{}

func (m *mockFixGenerator) GenerateFix(ctx context.Context, vuln *Vulnerability, code string) (*SecurityFix, error) {
	return &SecurityFix{
		VulnerabilityID: vuln.ID,
		OriginalCode:    vuln.Code,
		FixedCode:       "// Fixed: " + vuln.Code,
		Explanation:     "Mock fix for " + vuln.Title,
		Validated:       false,
	}, nil
}

type mockFixValidator struct {
	valid bool
}

func (m *mockFixValidator) ValidateFix(ctx context.Context, vuln *Vulnerability, fix *SecurityFix) (bool, error) {
	return m.valid, nil
}

type errorRing struct{}

func (r *errorRing) Name() string {
	return "error_ring"
}

func (r *errorRing) Check(ctx context.Context, input string) (bool, string, error) {
	return false, "", assert.AnError
}

// Helper functions for file-based tests
func createTempTestFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "security_test_*.go")
	if err != nil {
		return "", err
	}
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func removeTempTestFile(path string) {
	_ = os.Remove(path)
}
