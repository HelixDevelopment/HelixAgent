package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TestType represents the type of test
type TestType string

const (
	UnitTest        TestType = "unit"
	IntegrationTest TestType = "integration"
	E2ETest         TestType = "e2e"
	StressTest      TestType = "stress"
	SecurityTest    TestType = "security"
	StandaloneTest  TestType = "standalone"
)

// TestSuite represents a collection of tests
type TestSuite struct {
	Name   string
	Type   TestType
	Tests  []TestCase
	Config TestConfig
}

// TestCase represents an individual test
type TestCase struct {
	Name        string
	Description string
	Command     string
	Args        []string
	Timeout     time.Duration
	Expected    TestResult
}

// TestResult represents the result of a test
type TestResult struct {
	Passed   bool
	Output   string
	Error    string
	Duration time.Duration
	Coverage float64
}

// TestConfig represents configuration for test execution
type TestConfig struct {
	Parallel     bool
	Coverage     bool
	Verbose      bool
	Timeout      time.Duration
	CoverageFile string
}

// TestBankFramework manages all test execution
type TestBankFramework struct {
	suites  map[TestType]*TestSuite
	results map[TestType][]TestResult
	mu      sync.RWMutex
}

// NewTestBankFramework creates a new test framework
func NewTestBankFramework() *TestBankFramework {
	return &TestBankFramework{
		suites:  make(map[TestType]*TestSuite),
		results: make(map[TestType][]TestResult),
	}
}

// RegisterSuite registers a test suite
func (tbf *TestBankFramework) RegisterSuite(suite *TestSuite) {
	tbf.mu.Lock()
	defer tbf.mu.Unlock()
	tbf.suites[suite.Type] = suite
}

// RunAllSuites runs all registered test suites
func (tbf *TestBankFramework) RunAllSuites() (map[TestType][]TestResult, error) {
	tbf.mu.Lock()
	defer tbf.mu.Unlock()

	results := make(map[TestType][]TestResult)

	for testType, suite := range tbf.suites {
		suiteResults, err := tbf.runSuite(suite)
		if err != nil {
			return nil, fmt.Errorf("failed to run %s suite: %w", testType, err)
		}
		results[testType] = suiteResults
		tbf.results[testType] = suiteResults
	}

	return results, nil
}

// RunSuite runs a specific test suite
func (tbf *TestBankFramework) RunSuite(testType TestType) ([]TestResult, error) {
	tbf.mu.RLock()
	suite, exists := tbf.suites[testType]
	tbf.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("suite %s not found", testType)
	}

	return tbf.runSuite(suite)
}

// runSuite executes a test suite
func (tbf *TestBankFramework) runSuite(suite *TestSuite) ([]TestResult, error) {
	results := make([]TestResult, len(suite.Tests))

	if suite.Config.Parallel {
		var wg sync.WaitGroup
		for i := range suite.Tests {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index] = tbf.runTestCase(&suite.Tests[index], &suite.Config)
			}(i)
		}
		wg.Wait()
	} else {
		for i, testCase := range suite.Tests {
			results[i] = tbf.runTestCase(&testCase, &suite.Config)
		}
	}

	return results, nil
}

// runTestCase executes a single test case
func (tbf *TestBankFramework) runTestCase(testCase *TestCase, cfg *TestConfig) TestResult {
	start := time.Now()

	// Determine timeout
	timeout := cfg.Timeout
	if testCase.Timeout > 0 {
		timeout = testCase.Timeout
	}

	// Prepare command with context
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, testCase.Command, testCase.Args...)

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()
	duration := time.Since(start)

	result := TestResult{
		Passed:   err == nil,
		Duration: duration,
	}

	if err != nil {
		result.Error = err.Error()
		result.Output = stderr.String()
	} else {
		result.Output = stdout.String()
	}

	// Calculate coverage if enabled
	if cfg.Coverage && strings.Contains(testCase.Command, "go test") {
		result.Coverage = tbf.parseCoverage(result.Output)
	}

	return result
}

// parseCoverage parses coverage percentage from go test output
func (tbf *TestBankFramework) parseCoverage(output string) float64 {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "coverage:") {
			var coverage float64
			fmt.Sscanf(line, "coverage: %f%%", &coverage)
			return coverage
		}
	}
	return 0.0
}

// GenerateReport generates a test report
func (tbf *TestBankFramework) GenerateReport(format string) (string, error) {
	tbf.mu.RLock()
	defer tbf.mu.RUnlock()

	report := TestReport{
		Timestamp: time.Now(),
		Suites:    make(map[string]SuiteReport),
	}

	totalTests := 0
	totalPassed := 0

	for testType, results := range tbf.results {
		suiteReport := SuiteReport{
			Type:    string(testType),
			Tests:   len(results),
			Passed:  0,
			Failed:  0,
			Results: results,
		}

		for _, result := range results {
			if result.Passed {
				suiteReport.Passed++
			} else {
				suiteReport.Failed++
			}
		}

		report.Suites[string(testType)] = suiteReport
		totalTests += suiteReport.Tests
		totalPassed += suiteReport.Passed
	}

	report.TotalTests = totalTests
	report.TotalPassed = totalPassed
	report.TotalFailed = totalTests - totalPassed

	switch format {
	case "json":
		return tbf.generateJSONReport(report)
	case "html":
		return tbf.generateHTMLReport(report)
	default:
		return tbf.generateTextReport(report)
	}
}

// TestReport represents a complete test report
type TestReport struct {
	Timestamp   time.Time              `json:"timestamp"`
	TotalTests  int                    `json:"total_tests"`
	TotalPassed int                    `json:"total_passed"`
	TotalFailed int                    `json:"total_failed"`
	Suites      map[string]SuiteReport `json:"suites"`
}

// SuiteReport represents a suite report
type SuiteReport struct {
	Type    string       `json:"type"`
	Tests   int          `json:"tests"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Results []TestResult `json:"results"`
}

// generateJSONReport generates JSON report
func (tbf *TestBankFramework) generateJSONReport(report TestReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// generateHTMLReport generates HTML report
func (tbf *TestBankFramework) generateHTMLReport(report TestReport) (string, error) {
	html := `<html>
<head>
<title>Test Report</title>
<style>
body { font-family: Arial, sans-serif; }
.suite { margin: 20px; padding: 10px; border: 1px solid #ccc; }
.passed { color: green; }
.failed { color: red; }
</style>
</head>
<body>
<h1>Test Report</h1>
<p>Total Tests: ` + fmt.Sprintf("%d", report.TotalTests) + `</p>
<p>Passed: <span class="passed">` + fmt.Sprintf("%d", report.TotalPassed) + `</span></p>
<p>Failed: <span class="failed">` + fmt.Sprintf("%d", report.TotalFailed) + `</span></p>
`

	for suiteName, suite := range report.Suites {
		html += `<div class="suite">
<h2>` + suiteName + `</h2>
<p>Tests: ` + fmt.Sprintf("%d", suite.Tests) + `, Passed: <span class="passed">` + fmt.Sprintf("%d", suite.Passed) + `</span>, Failed: <span class="failed">` + fmt.Sprintf("%d", suite.Failed) + `</span></p>
</div>`
	}

	html += `</body></html>`
	return html, nil
}

// generateTextReport generates text report
func (tbf *TestBankFramework) generateTextReport(report TestReport) (string, error) {
	text := fmt.Sprintf("Test Report - %s\n", report.Timestamp.Format(time.RFC3339))
	text += fmt.Sprintf("Total Tests: %d\n", report.TotalTests)
	text += fmt.Sprintf("Passed: %d\n", report.TotalPassed)
	text += fmt.Sprintf("Failed: %d\n\n", report.TotalFailed)

	for suiteName, suite := range report.Suites {
		text += fmt.Sprintf("%s Suite:\n", suiteName)
		text += fmt.Sprintf("  Tests: %d, Passed: %d, Failed: %d\n", suite.Tests, suite.Passed, suite.Failed)
		for i, result := range suite.Results {
			status := "PASS"
			if !result.Passed {
				status = "FAIL"
			}
			text += fmt.Sprintf("    %d. %s - %s (%.2fs)\n", i+1, suiteName, status, result.Duration.Seconds())
		}
		text += "\n"
	}

	return text, nil
}

// LoadSuitesFromConfig loads test suites from configuration
func (tbf *TestBankFramework) LoadSuitesFromConfig() error {
	// Load unit tests
	unitSuite := &TestSuite{
		Name: "Unit Tests",
		Type: UnitTest,
		Config: TestConfig{
			Parallel:     true,
			Coverage:     true,
			Verbose:      true,
			Timeout:      5 * time.Minute,
			CoverageFile: "coverage_unit.out",
		},
	}
	// Add test cases for unit tests
	unitSuite.Tests = tbf.discoverGoTests("./...", "-run=Test", "-short")

	tbf.RegisterSuite(unitSuite)

	// Load integration tests
	integrationSuite := &TestSuite{
		Name: "Integration Tests",
		Type: IntegrationTest,
		Config: TestConfig{
			Parallel: false,
			Coverage: true,
			Verbose:  true,
			Timeout:  10 * time.Minute,
		},
	}
	integrationSuite.Tests = tbf.discoverGoTests("tests/integration", "-run=TestIntegration")

	tbf.RegisterSuite(integrationSuite)

	// Load E2E tests
	e2eSuite := &TestSuite{
		Name: "E2E Tests",
		Type: E2ETest,
		Config: TestConfig{
			Parallel: false,
			Coverage: false,
			Verbose:  true,
			Timeout:  15 * time.Minute,
		},
	}
	e2eSuite.Tests = tbf.discoverGoTests("tests/e2e", "-run=TestE2E")

	tbf.RegisterSuite(e2eSuite)

	// Load stress tests
	stressSuite := &TestSuite{
		Name: "Stress Tests",
		Type: StressTest,
		Config: TestConfig{
			Parallel: true,
			Coverage: false,
			Verbose:  true,
			Timeout:  20 * time.Minute,
		},
	}
	stressSuite.Tests = tbf.discoverGoTests("tests/stress", "-run=TestStress")

	tbf.RegisterSuite(stressSuite)

	// Load security tests
	securitySuite := &TestSuite{
		Name: "Security Tests",
		Type: SecurityTest,
		Config: TestConfig{
			Parallel: false,
			Coverage: false,
			Verbose:  true,
			Timeout:  10 * time.Minute,
		},
	}
	securitySuite.Tests = tbf.discoverGoTests("tests/security", "-run=TestSecurity")

	tbf.RegisterSuite(securitySuite)

	// Load standalone tests
	standaloneSuite := &TestSuite{
		Name: "Standalone Tests",
		Type: StandaloneTest,
		Config: TestConfig{
			Parallel: true,
			Coverage: false,
			Verbose:  true,
			Timeout:  5 * time.Minute,
		},
	}
	standaloneSuite.Tests = tbf.discoverStandaloneTests("tests/standalone")

	tbf.RegisterSuite(standaloneSuite)

	return nil
}

// discoverGoTests discovers Go test files
func (tbf *TestBankFramework) discoverGoTests(path, runFlag string, extraArgs ...string) []TestCase {
	var tests []TestCase

	// Find test files
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(filePath, "_test.go") {
			testCase := TestCase{
				Name:    strings.TrimSuffix(filepath.Base(filePath), "_test.go"),
				Command: "go",
				Args:    []string{"test", "-v", runFlag, filePath},
				Timeout: 5 * time.Minute,
			}
			testCase.Args = append(testCase.Args, extraArgs...)
			tests = append(tests, testCase)
		}
		return nil
	})

	if err != nil {
		// Fallback to basic test
		tests = append(tests, TestCase{
			Name:    "Basic Go Test",
			Command: "go",
			Args:    []string{"test", "-v", path},
			Timeout: 5 * time.Minute,
		})
	}

	return tests
}

// discoverStandaloneTests discovers standalone test programs
func (tbf *TestBankFramework) discoverStandaloneTests(path string) []TestCase {
	var tests []TestCase

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(filepath.Base(filePath), "test_") {
			testCase := TestCase{
				Name:    strings.TrimPrefix(filepath.Base(filePath), "test_"),
				Command: filePath,
				Args:    []string{},
				Timeout: 5 * time.Minute,
			}
			tests = append(tests, testCase)
		}
		return nil
	})

	if err != nil {
		// Fallback
		tests = append(tests, TestCase{
			Name:    "Standalone Test",
			Command: "echo",
			Args:    []string{"No standalone tests found"},
			Timeout: 1 * time.Minute,
		})
	}

	return tests
}
