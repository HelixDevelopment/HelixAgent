package testing

import (
	"testing"
	"time"
)

func TestNewTestBankFramework(t *testing.T) {
	framework := NewTestBankFramework()
	if framework == nil {
		t.Fatal("Expected framework to be created")
	}
}

func TestRegisterSuite(t *testing.T) {
	framework := NewTestBankFramework()
	suite := &TestSuite{
		Name: "Test Suite",
		Type: UnitTest,
		Tests: []TestCase{
			{
				Name:    "Test 1",
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}

	framework.RegisterSuite(suite)

	// Try to run the suite
	results, err := framework.RunSuite(UnitTest)
	if err != nil {
		t.Fatalf("Failed to run suite: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 test result, got %d", len(results))
	}

	if !results[0].Passed {
		t.Fatal("Expected test to pass")
	}
}

func TestRunTestCase(t *testing.T) {
	framework := NewTestBankFramework()
	testCase := &TestCase{
		Name:    "Echo Test",
		Command: "echo",
		Args:    []string{"test"},
		Timeout: 5 * time.Second,
	}

	cfg := &TestConfig{
		Parallel: false,
		Coverage: false,
		Verbose:  false,
		Timeout:  10 * time.Second,
	}

	result := framework.runTestCase(testCase, cfg)

	if !result.Passed {
		t.Fatalf("Expected test to pass, got error: %v", result.Error)
	}

	if result.Duration == 0 {
		t.Fatal("Expected duration to be recorded")
	}
}

func TestParseCoverage(t *testing.T) {
	framework := NewTestBankFramework()

	// Test with coverage output
	output := `PASS
coverage: 76.7% of statements
ok  	github.com/superagent/superagent/internal/utils	0.875s`

	coverage := framework.parseCoverage(output)
	if coverage != 76.7 {
		t.Fatalf("Expected coverage 76.7, got %f", coverage)
	}

	// Test without coverage
	output2 := `PASS
ok  	github.com/superagent/superagent/internal/utils	0.875s`

	coverage2 := framework.parseCoverage(output2)
	if coverage2 != 0.0 {
		t.Fatalf("Expected coverage 0.0, got %f", coverage2)
	}
}

func TestGenerateReport(t *testing.T) {
	framework := NewTestBankFramework()

	// Add some test results
	framework.results[UnitTest] = []TestResult{
		{
			Passed:   true,
			Output:   "Test passed",
			Duration: 1 * time.Second,
			Coverage: 76.7,
		},
		{
			Passed:   false,
			Error:    "Test failed",
			Output:   "Test output",
			Duration: 2 * time.Second,
			Coverage: 0.0,
		},
	}

	// Test JSON report
	jsonReport, err := framework.GenerateReport("json")
	if err != nil {
		t.Fatalf("Failed to generate JSON report: %v", err)
	}
	if jsonReport == "" {
		t.Fatal("Expected JSON report to be non-empty")
	}

	// Test HTML report
	htmlReport, err := framework.GenerateReport("html")
	if err != nil {
		t.Fatalf("Failed to generate HTML report: %v", err)
	}
	if htmlReport == "" {
		t.Fatal("Expected HTML report to be non-empty")
	}

	// Test text report
	textReport, err := framework.GenerateReport("text")
	if err != nil {
		t.Fatalf("Failed to generate text report: %v", err)
	}
	if textReport == "" {
		t.Fatal("Expected text report to be non-empty")
	}
}

func TestTestTypeConstants(t *testing.T) {
	if UnitTest != "unit" {
		t.Fatalf("Expected UnitTest to be 'unit', got %s", UnitTest)
	}
	if IntegrationTest != "integration" {
		t.Fatalf("Expected IntegrationTest to be 'integration', got %s", IntegrationTest)
	}
	if E2ETest != "e2e" {
		t.Fatalf("Expected E2ETest to be 'e2e', got %s", E2ETest)
	}
	if StressTest != "stress" {
		t.Fatalf("Expected StressTest to be 'stress', got %s", StressTest)
	}
	if SecurityTest != "security" {
		t.Fatalf("Expected SecurityTest to be 'security', got %s", SecurityTest)
	}
	if StandaloneTest != "standalone" {
		t.Fatalf("Expected StandaloneTest to be 'standalone', got %s", StandaloneTest)
	}
}

func TestTestResultFields(t *testing.T) {
	result := TestResult{
		Passed:   true,
		Output:   "output",
		Error:    "",
		Duration: 1 * time.Second,
		Coverage: 50.0,
	}

	if !result.Passed {
		t.Fatal("Expected Passed to be true")
	}
	if result.Output != "output" {
		t.Fatalf("Expected Output to be 'output', got %s", result.Output)
	}
	if result.Error != "" {
		t.Fatalf("Expected Error to be empty, got %s", result.Error)
	}
	if result.Duration != 1*time.Second {
		t.Fatalf("Expected Duration to be 1s, got %v", result.Duration)
	}
	if result.Coverage != 50.0 {
		t.Fatalf("Expected Coverage to be 50.0, got %f", result.Coverage)
	}
}

func TestTestConfigFields(t *testing.T) {
	cfg := TestConfig{
		Parallel:     true,
		Coverage:     true,
		Verbose:      true,
		Timeout:      5 * time.Minute,
		CoverageFile: "coverage.out",
	}

	if !cfg.Parallel {
		t.Fatal("Expected Parallel to be true")
	}
	if !cfg.Coverage {
		t.Fatal("Expected Coverage to be true")
	}
	if !cfg.Verbose {
		t.Fatal("Expected Verbose to be true")
	}
	if cfg.Timeout != 5*time.Minute {
		t.Fatalf("Expected Timeout to be 5m, got %v", cfg.Timeout)
	}
	if cfg.CoverageFile != "coverage.out" {
		t.Fatalf("Expected CoverageFile to be 'coverage.out', got %s", cfg.CoverageFile)
	}
}

func TestRunAllSuites(t *testing.T) {
	framework := NewTestBankFramework()

	// Register multiple suites
	unitSuite := &TestSuite{
		Name: "Unit Tests",
		Type: UnitTest,
		Tests: []TestCase{
			{
				Name:    "Echo Unit Test",
				Command: "echo",
				Args:    []string{"unit"},
			},
		},
	}

	integrationSuite := &TestSuite{
		Name: "Integration Tests",
		Type: IntegrationTest,
		Tests: []TestCase{
			{
				Name:    "Echo Integration Test",
				Command: "echo",
				Args:    []string{"integration"},
			},
		},
	}

	framework.RegisterSuite(unitSuite)
	framework.RegisterSuite(integrationSuite)

	// Run all suites
	results, err := framework.RunAllSuites()
	if err != nil {
		t.Fatalf("Failed to run all suites: %v", err)
	}

	// Verify results for both suites
	if len(results) != 2 {
		t.Fatalf("Expected 2 suite results, got %d", len(results))
	}

	if unitResults, ok := results[UnitTest]; !ok {
		t.Fatal("Expected unit test results")
	} else if len(unitResults) != 1 {
		t.Fatalf("Expected 1 unit test result, got %d", len(unitResults))
	} else if !unitResults[0].Passed {
		t.Fatal("Expected unit test to pass")
	}

	if integrationResults, ok := results[IntegrationTest]; !ok {
		t.Fatal("Expected integration test results")
	} else if len(integrationResults) != 1 {
		t.Fatalf("Expected 1 integration test result, got %d", len(integrationResults))
	} else if !integrationResults[0].Passed {
		t.Fatal("Expected integration test to pass")
	}
}

func TestRunAllSuites_Empty(t *testing.T) {
	framework := NewTestBankFramework()

	// Run without any suites registered
	results, err := framework.RunAllSuites()
	if err != nil {
		t.Fatalf("Failed to run empty suites: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("Expected 0 results from empty suites, got %d", len(results))
	}
}

func TestRunSuite_NotFound(t *testing.T) {
	framework := NewTestBankFramework()

	_, err := framework.RunSuite("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent suite")
	}
}
