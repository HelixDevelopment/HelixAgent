package sanity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "localhost", config.HelixAgentHost)
	assert.Equal(t, 7061, config.HelixAgentPort)
	assert.Equal(t, "localhost", config.PostgresHost)
	assert.Equal(t, 5432, config.PostgresPort)
	assert.Equal(t, "localhost", config.RedisHost)
	assert.Equal(t, 6379, config.RedisPort)
	assert.Equal(t, "localhost", config.CogneeHost)
	assert.Equal(t, 8000, config.CogneePort)
	assert.Equal(t, 10*time.Second, config.Timeout)
}

func TestNewBootChecker(t *testing.T) {
	checker := NewBootChecker(nil)

	assert.NotNil(t, checker)
	assert.NotNil(t, checker.config)
	assert.NotNil(t, checker.httpClient)
	assert.NotNil(t, checker.results)
	assert.Equal(t, 0, checker.results.Len())
}

func TestNewBootChecker_WithConfig(t *testing.T) {
	config := &BootCheckConfig{
		HelixAgentHost: "custom-host",
		HelixAgentPort: 9000,
		Timeout:        30 * time.Second,
	}

	checker := NewBootChecker(config)

	assert.Equal(t, "custom-host", checker.config.HelixAgentHost)
	assert.Equal(t, 9000, checker.config.HelixAgentPort)
	assert.Equal(t, 30*time.Second, checker.config.Timeout)
}

func TestCheckStatus_Constants(t *testing.T) {
	assert.Equal(t, CheckStatus("PASS"), StatusPassed)
	assert.Equal(t, CheckStatus("FAIL"), StatusFailed)
	assert.Equal(t, CheckStatus("WARN"), StatusWarning)
	assert.Equal(t, CheckStatus("SKIP"), StatusSkipped)
}

func TestCheckResult_Fields(t *testing.T) {
	result := CheckResult{
		Name:      "Test Check",
		Category:  "Test",
		Status:    StatusPassed,
		Message:   "Test passed",
		Details:   "All good",
		Duration:  100 * time.Millisecond,
		Critical:  true,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "Test Check", result.Name)
	assert.Equal(t, "Test", result.Category)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "Test passed", result.Message)
	assert.Equal(t, "All good", result.Details)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
	assert.True(t, result.Critical)
	assert.NotZero(t, result.Timestamp)
}

func TestBootCheckReport_Fields(t *testing.T) {
	report := &BootCheckReport{
		Timestamp:       time.Now(),
		Duration:        5 * time.Second,
		TotalChecks:     10,
		PassedChecks:    7,
		FailedChecks:    2,
		WarningChecks:   1,
		SkippedChecks:   0,
		CriticalFailure: false,
		ReadyToStart:    true,
		Results:         []CheckResult{},
	}

	assert.Equal(t, 10, report.TotalChecks)
	assert.Equal(t, 7, report.PassedChecks)
	assert.Equal(t, 2, report.FailedChecks)
	assert.Equal(t, 1, report.WarningChecks)
	assert.True(t, report.ReadyToStart)
}

func TestBootChecker_AddResult(t *testing.T) {
	checker := NewBootChecker(nil)

	result := CheckResult{
		Name:     "Test",
		Status:   StatusPassed,
		Category: "Test",
	}

	checker.addResult(result)

	assert.Equal(t, 1, checker.results.Len())
	first, _ := checker.results.At(0)
	assert.Equal(t, "Test", first.Name)
}

func TestBootChecker_AddResult_Concurrent(t *testing.T) {
	checker := NewBootChecker(nil)

	const numResults = 100
	done := make(chan struct{})

	for i := 0; i < numResults; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			result := CheckResult{
				Name:     "Test",
				Status:   StatusPassed,
				Category: "Test",
			}
			checker.addResult(result)
		}(i)
	}

	for i := 0; i < numResults; i++ {
		<-done
	}

	assert.Equal(t, numResults, checker.results.Len())
}

func TestBootChecker_GenerateReport(t *testing.T) {
	checker := NewBootChecker(nil)

	// Add some results
	checker.addResult(CheckResult{Name: "Pass1", Status: StatusPassed, Critical: false})
	checker.addResult(CheckResult{Name: "Pass2", Status: StatusPassed, Critical: false})
	checker.addResult(CheckResult{Name: "Fail1", Status: StatusFailed, Critical: true})
	checker.addResult(CheckResult{Name: "Warn1", Status: StatusWarning, Critical: false})
	checker.addResult(CheckResult{Name: "Skip1", Status: StatusSkipped, Critical: false})

	start := time.Now()
	report := checker.generateReport(start)

	assert.Equal(t, 5, report.TotalChecks)
	assert.Equal(t, 2, report.PassedChecks)
	assert.Equal(t, 1, report.FailedChecks)
	assert.Equal(t, 1, report.WarningChecks)
	assert.Equal(t, 1, report.SkippedChecks)
	assert.True(t, report.CriticalFailure)
	assert.False(t, report.ReadyToStart)
}

func TestBootChecker_GenerateReport_NoFailures(t *testing.T) {
	checker := NewBootChecker(nil)

	// Add only passing results
	checker.addResult(CheckResult{Name: "Pass1", Status: StatusPassed, Critical: true})
	checker.addResult(CheckResult{Name: "Pass2", Status: StatusPassed, Critical: false})

	start := time.Now()
	report := checker.generateReport(start)

	assert.Equal(t, 2, report.TotalChecks)
	assert.Equal(t, 2, report.PassedChecks)
	assert.Equal(t, 0, report.FailedChecks)
	assert.False(t, report.CriticalFailure)
	assert.True(t, report.ReadyToStart)
}

func TestBootChecker_GenerateReport_NonCriticalFailure(t *testing.T) {
	checker := NewBootChecker(nil)

	// Add non-critical failure
	checker.addResult(CheckResult{Name: "Pass1", Status: StatusPassed, Critical: true})
	checker.addResult(CheckResult{Name: "Fail1", Status: StatusFailed, Critical: false})

	start := time.Now()
	report := checker.generateReport(start)

	assert.Equal(t, 1, report.FailedChecks)
	assert.False(t, report.CriticalFailure)
	assert.True(t, report.ReadyToStart)
}

func TestBootChecker_CheckEnvironmentVariables(t *testing.T) {
	checker := NewBootChecker(nil)

	// Run the check
	checker.checkEnvironmentVariables()

	// Should have one result
	assert.Equal(t, 1, checker.results.Len())

	result, _ := checker.results.At(0)
	assert.Equal(t, "Environment Variables", result.Name)
	assert.Equal(t, "Configuration", result.Category)
	assert.True(t, result.Critical)
}

func TestBootChecker_CheckRequiredFiles(t *testing.T) {
	checker := NewBootChecker(nil)

	// Run the check
	checker.checkRequiredFiles()

	// Should have one result
	assert.Equal(t, 1, checker.results.Len())

	result, _ := checker.results.At(0)
	assert.Equal(t, "Configuration Files", result.Name)
	assert.Equal(t, "Configuration", result.Category)
	assert.False(t, result.Critical)
}

func TestBootChecker_CheckDiskSpace(t *testing.T) {
	checker := NewBootChecker(nil)

	// Run the check
	checker.checkDiskSpace()

	// Should have one result
	require.Equal(t, 1, checker.results.Len())

	result, _ := checker.results.At(0)
	assert.Equal(t, "Disk Space", result.Name)
	assert.Equal(t, "System", result.Category)
	assert.Equal(t, StatusPassed, result.Status)
}

func TestBootChecker_CheckPortAvailability(t *testing.T) {
	checker := NewBootChecker(&BootCheckConfig{
		HelixAgentPort: 19999, // Use unlikely port for testing
	})

	// Run the check
	checker.checkPortAvailability()

	// Should have one result
	require.Equal(t, 1, checker.results.Len())

	result, _ := checker.results.At(0)
	assert.Equal(t, "Port Availability", result.Name)
	assert.Equal(t, "Network", result.Category)
}

func TestBootChecker_RunAllChecks_SkipExternal(t *testing.T) {
	checker := NewBootChecker(&BootCheckConfig{
		HelixAgentHost:     "localhost",
		HelixAgentPort:     1, // Invalid port
		PostgresHost:       "localhost",
		PostgresPort:       1,
		RedisHost:          "localhost",
		RedisPort:          1,
		CogneeHost:         "localhost",
		CogneePort:         1,
		Timeout:            100 * time.Millisecond,
		SkipExternalChecks: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report := checker.RunAllChecks(ctx)

	assert.NotNil(t, report)
	assert.Greater(t, report.TotalChecks, 0)
	// Should have at least the environment and file checks
	assert.GreaterOrEqual(t, report.TotalChecks, 4)
}

func TestRunSanityCheck(t *testing.T) {
	config := &BootCheckConfig{
		HelixAgentHost:     "localhost",
		HelixAgentPort:     1,
		PostgresHost:       "localhost",
		PostgresPort:       1,
		RedisHost:          "localhost",
		RedisPort:          1,
		CogneeHost:         "localhost",
		CogneePort:         1,
		Timeout:            100 * time.Millisecond,
		SkipExternalChecks: true,
	}

	report := RunSanityCheck(config)

	assert.NotNil(t, report)
	assert.Greater(t, report.TotalChecks, 0)
}

func TestBootCheckConfig_Default(t *testing.T) {
	config := &BootCheckConfig{}

	assert.Equal(t, "", config.HelixAgentHost)
	assert.Equal(t, 0, config.HelixAgentPort)
	assert.False(t, config.SkipExternalChecks)
}

func TestBootChecker_CheckExternalProvider_NoAPIKey(t *testing.T) {
	checker := NewBootChecker(nil)

	ctx := context.Background()
	// This should skip because no API key is set
	checker.checkExternalProvider(ctx, "TestProvider", "https://example.com", "NONEXISTENT_API_KEY_123")

	require.Equal(t, 1, checker.results.Len())
	first, _ := checker.results.At(0)
	assert.Equal(t, StatusSkipped, first.Status)
}

func TestBootCheckReport_JSON(t *testing.T) {
	report := &BootCheckReport{
		Timestamp:       time.Now(),
		Duration:        5 * time.Second,
		TotalChecks:     3,
		PassedChecks:    2,
		FailedChecks:    1,
		CriticalFailure: false,
		ReadyToStart:    true,
		Results: []CheckResult{
			{Name: "Test1", Status: StatusPassed, Category: "Test"},
			{Name: "Test2", Status: StatusPassed, Category: "Test"},
			{Name: "Test3", Status: StatusFailed, Category: "Test"},
		},
	}

	assert.Equal(t, 3, len(report.Results))
	assert.True(t, report.ReadyToStart)
}
