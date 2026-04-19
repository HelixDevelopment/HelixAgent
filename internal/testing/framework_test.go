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
ok  	dev.helix.agent/internal/utils	0.875s`

	coverage := framework.parseCoverage(output)
	if coverage != 76.7 {
		t.Fatalf("Expected coverage 76.7, got %f", coverage)
	}

	// Test without coverage
	output2 := `PASS
ok  	dev.helix.agent/internal/utils	0.875s`

	coverage2 := framework.parseCoverage(output2)
	if coverage2 != 0.0 {
		t.Fatalf("Expected coverage 0.0, got %f", coverage2)
	}
}

func TestGenerateReport(t *testing.T) {
	framework := NewTestBankFramework()

	// Add some test results
	framework.results.Put(UnitTest, []TestResult{
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
	})

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

func TestTestCaseFields(t *testing.T) {
	testCase := TestCase{
		Name:        "Test Case",
		Description: "Test case description",
		Command:     "echo",
		Args:        []string{"hello", "world"},
		Timeout:     30 * time.Second,
		Expected: TestResult{
			Passed: true,
		},
	}

	if testCase.Name != "Test Case" {
		t.Fatalf("Expected Name to be 'Test Case', got %s", testCase.Name)
	}
	if testCase.Description != "Test case description" {
		t.Fatalf("Expected Description to be set")
	}
	if testCase.Command != "echo" {
		t.Fatalf("Expected Command to be 'echo', got %s", testCase.Command)
	}
	if len(testCase.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(testCase.Args))
	}
	if testCase.Timeout != 30*time.Second {
		t.Fatalf("Expected Timeout to be 30s, got %v", testCase.Timeout)
	}
	if !testCase.Expected.Passed {
		t.Fatal("Expected Expected.Passed to be true")
	}
}

func TestTestSuiteFields(t *testing.T) {
	suite := TestSuite{
		Name: "My Suite",
		Type: E2ETest,
		Tests: []TestCase{
			{Name: "Test 1"},
			{Name: "Test 2"},
		},
		Config: TestConfig{
			Parallel: true,
			Coverage: true,
		},
	}

	if suite.Name != "My Suite" {
		t.Fatalf("Expected Name to be 'My Suite', got %s", suite.Name)
	}
	if suite.Type != E2ETest {
		t.Fatalf("Expected Type to be E2ETest, got %s", suite.Type)
	}
	if len(suite.Tests) != 2 {
		t.Fatalf("Expected 2 tests, got %d", len(suite.Tests))
	}
	if !suite.Config.Parallel {
		t.Fatal("Expected Config.Parallel to be true")
	}
	if !suite.Config.Coverage {
		t.Fatal("Expected Config.Coverage to be true")
	}
}

func TestRunSuiteParallel(t *testing.T) {
	framework := NewTestBankFramework()

	// Create a suite with multiple tests that run in parallel
	suite := &TestSuite{
		Name: "Parallel Tests",
		Type: StressTest,
		Tests: []TestCase{
			{
				Name:    "Parallel Test 1",
				Command: "echo",
				Args:    []string{"parallel1"},
			},
			{
				Name:    "Parallel Test 2",
				Command: "echo",
				Args:    []string{"parallel2"},
			},
			{
				Name:    "Parallel Test 3",
				Command: "echo",
				Args:    []string{"parallel3"},
			},
		},
		Config: TestConfig{
			Parallel: true,
			Timeout:  5 * time.Second,
		},
	}

	framework.RegisterSuite(suite)

	results, err := framework.RunSuite(StressTest)
	if err != nil {
		t.Fatalf("Failed to run parallel suite: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Passed {
			t.Fatalf("Expected test %d to pass", i+1)
		}
	}
}

func TestRunTestCaseWithTimeout(t *testing.T) {
	framework := NewTestBankFramework()

	// Test case with specific timeout
	testCase := &TestCase{
		Name:    "Sleep Test",
		Command: "sleep",
		Args:    []string{"0.1"},
		Timeout: 5 * time.Second,
	}

	cfg := &TestConfig{
		Timeout: 10 * time.Second,
	}

	result := framework.runTestCase(testCase, cfg)

	if !result.Passed {
		t.Fatalf("Expected test to pass, got error: %s", result.Error)
	}
}

func TestRunTestCaseFailure(t *testing.T) {
	framework := NewTestBankFramework()

	// Test case that fails
	testCase := &TestCase{
		Name:    "Failing Test",
		Command: "false",
		Args:    []string{},
		Timeout: 5 * time.Second,
	}

	cfg := &TestConfig{
		Timeout: 10 * time.Second,
	}

	result := framework.runTestCase(testCase, cfg)

	if result.Passed {
		t.Fatal("Expected test to fail")
	}
	if result.Error == "" {
		t.Fatal("Expected error to be set")
	}
}

func TestTestReportFields(t *testing.T) {
	report := TestReport{
		Timestamp:   time.Now(),
		TotalTests:  10,
		TotalPassed: 8,
		TotalFailed: 2,
		Suites: map[string]SuiteReport{
			"unit": {
				Type:   "unit",
				Tests:  5,
				Passed: 4,
				Failed: 1,
			},
		},
	}

	if report.TotalTests != 10 {
		t.Fatalf("Expected TotalTests to be 10, got %d", report.TotalTests)
	}
	if report.TotalPassed != 8 {
		t.Fatalf("Expected TotalPassed to be 8, got %d", report.TotalPassed)
	}
	if report.TotalFailed != 2 {
		t.Fatalf("Expected TotalFailed to be 2, got %d", report.TotalFailed)
	}
	if len(report.Suites) != 1 {
		t.Fatalf("Expected 1 suite, got %d", len(report.Suites))
	}
}

func TestSuiteReportFields(t *testing.T) {
	report := SuiteReport{
		Type:   "integration",
		Tests:  10,
		Passed: 9,
		Failed: 1,
		Results: []TestResult{
			{Passed: true, Duration: 1 * time.Second},
			{Passed: false, Error: "failed", Duration: 2 * time.Second},
		},
	}

	if report.Type != "integration" {
		t.Fatalf("Expected Type to be 'integration', got %s", report.Type)
	}
	if report.Tests != 10 {
		t.Fatalf("Expected Tests to be 10, got %d", report.Tests)
	}
	if report.Passed != 9 {
		t.Fatalf("Expected Passed to be 9, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Fatalf("Expected Failed to be 1, got %d", report.Failed)
	}
	if len(report.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(report.Results))
	}
}

func TestGenerateReportFormats(t *testing.T) {
	framework := NewTestBankFramework()

	framework.results.Put(SecurityTest, []TestResult{
		{Passed: true, Output: "Security test passed", Duration: 500 * time.Millisecond},
	})

	t.Run("json format", func(t *testing.T) {
		report, err := framework.GenerateReport("json")
		if err != nil {
			t.Fatalf("Failed to generate JSON report: %v", err)
		}
		if report == "" {
			t.Fatal("Expected non-empty JSON report")
		}
		// Check it's valid JSON structure
		if report[0] != '{' {
			t.Fatal("Expected JSON report to start with '{'")
		}
	})

	t.Run("html format", func(t *testing.T) {
		report, err := framework.GenerateReport("html")
		if err != nil {
			t.Fatalf("Failed to generate HTML report: %v", err)
		}
		if report == "" {
			t.Fatal("Expected non-empty HTML report")
		}
		// Check it contains HTML tags
		if len(report) < 10 || report[:5] != "<html" {
			t.Fatal("Expected HTML report to start with '<html'")
		}
	})

	t.Run("text format", func(t *testing.T) {
		report, err := framework.GenerateReport("text")
		if err != nil {
			t.Fatalf("Failed to generate text report: %v", err)
		}
		if report == "" {
			t.Fatal("Expected non-empty text report")
		}
		// Check it contains expected text
		if len(report) < 10 {
			t.Fatal("Expected text report to have content")
		}
	})

	t.Run("default format", func(t *testing.T) {
		report, err := framework.GenerateReport("unknown")
		if err != nil {
			t.Fatalf("Failed to generate default report: %v", err)
		}
		// Should fall back to text format
		if report == "" {
			t.Fatal("Expected non-empty default report")
		}
	})
}

func TestDiscoverGoTests(t *testing.T) {
	framework := NewTestBankFramework()

	// Test with non-existent path - should return fallback
	tests := framework.discoverGoTests("/nonexistent/path", "-run=Test")

	if len(tests) != 1 {
		t.Fatalf("Expected 1 fallback test, got %d", len(tests))
	}
	if tests[0].Name != "Basic Go Test" {
		t.Fatalf("Expected fallback test name, got %s", tests[0].Name)
	}
}

func TestDiscoverStandaloneTests(t *testing.T) {
	framework := NewTestBankFramework()

	// Test with non-existent path - should return fallback
	tests := framework.discoverStandaloneTests("/nonexistent/path")

	if len(tests) != 1 {
		t.Fatalf("Expected 1 fallback test, got %d", len(tests))
	}
	if tests[0].Name != "Standalone Test" {
		t.Fatalf("Expected fallback test name, got %s", tests[0].Name)
	}
}

func TestLoadSuitesFromConfig(t *testing.T) {
	framework := NewTestBankFramework()

	err := framework.LoadSuitesFromConfig()
	if err != nil {
		t.Fatalf("Failed to load suites from config: %v", err)
	}

	// Check that suites were registered
	suites := []TestType{UnitTest, IntegrationTest, E2ETest, StressTest, SecurityTest, StandaloneTest}
	for _, suiteType := range suites {
		if !framework.suites.Has(suiteType) {
			t.Fatalf("Expected %s suite to be registered", suiteType)
		}
	}
}

func TestConcurrentFrameworkAccess(t *testing.T) {
	framework := NewTestBankFramework()

	// Register a suite
	suite := &TestSuite{
		Name: "Concurrent Suite",
		Type: UnitTest,
		Tests: []TestCase{
			{
				Name:    "Concurrent Test",
				Command: "echo",
				Args:    []string{"concurrent"},
			},
		},
	}
	framework.RegisterSuite(suite)

	// Access framework concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = framework.RunSuite(UnitTest)
			_, _ = framework.GenerateReport("text")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
