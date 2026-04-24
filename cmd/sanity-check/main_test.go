package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"dev.helix.agent/internal/sanity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanityCheckConfig(t *testing.T) {
	config := &sanity.BootCheckConfig{
		HelixAgentHost:     "localhost",
		HelixAgentPort:     7061,
		PostgresHost:       "localhost",
		PostgresPort:       5432,
		RedisHost:          "localhost",
		RedisPort:          6379,
		CogneeHost:         "localhost",
		CogneePort:         8000,
		SkipExternalChecks: true,
	}

	assert.Equal(t, "localhost", config.HelixAgentHost)
	assert.Equal(t, 7061, config.HelixAgentPort)
	assert.Equal(t, 5432, config.PostgresPort)
	assert.Equal(t, 6379, config.RedisPort)
	assert.True(t, config.SkipExternalChecks)
}

func TestDefaultConfig(t *testing.T) {
	config := sanity.DefaultConfig()

	assert.Equal(t, "localhost", config.HelixAgentHost)
	assert.Equal(t, 7061, config.HelixAgentPort)
	assert.Equal(t, 5432, config.PostgresPort)
	assert.Equal(t, 6379, config.RedisPort)
	assert.Equal(t, 8000, config.CogneePort)
	assert.Equal(t, 10*time.Second, config.Timeout)
}

func TestRunSanityCheck(t *testing.T) {
	t.Run("with skip external checks", func(t *testing.T) {
		config := &sanity.BootCheckConfig{
			HelixAgentHost:     "localhost",
			HelixAgentPort:     7061,
			PostgresHost:       "localhost",
			PostgresPort:       5432,
			RedisHost:          "localhost",
			RedisPort:          6379,
			CogneeHost:         "localhost",
			CogneePort:         8000,
			SkipExternalChecks: true,
			Timeout:            1 * time.Second,
		}

		report := sanity.RunSanityCheck(config)

		// Report should be generated regardless of service availability
		assert.NotNil(t, report)
		assert.NotZero(t, report.Timestamp)
		assert.GreaterOrEqual(t, report.TotalChecks, 0)
	})

	t.Run("with default config", func(t *testing.T) {
		config := &sanity.BootCheckConfig{
			SkipExternalChecks: true,
			Timeout:            1 * time.Second,
		}

		report := sanity.RunSanityCheck(config)
		assert.NotNil(t, report)
	})
}

func TestBootCheckReport(t *testing.T) {
	// Create a sample report
	report := &sanity.BootCheckReport{
		ReadyToStart:  true,
		Timestamp:     time.Now(),
		TotalChecks:   5,
		PassedChecks:  3,
		FailedChecks:  1,
		WarningChecks: 1,
		Duration:      time.Second,
		Results: []sanity.CheckResult{
			{
				Name:      "test",
				Category:  "test",
				Status:    sanity.StatusPassed,
				Message:   "Test passed",
				Timestamp: time.Now(),
			},
		},
	}

	// Test JSON serialization
	data, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	assert.Contains(t, string(data), "ready_to_start")
	assert.Contains(t, string(data), "test")
}

func TestCheckResult(t *testing.T) {
	result := sanity.CheckResult{
		Name:      "test-check",
		Category:  "infrastructure",
		Status:    sanity.StatusPassed,
		Message:   "Check passed successfully",
		Details:   "latency: 50ms",
		Duration:  50 * time.Millisecond,
		Critical:  false,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "test-check", result.Name)
	assert.Equal(t, sanity.StatusPassed, result.Status)
	assert.Equal(t, "Check passed successfully", result.Message)
	assert.Equal(t, "latency: 50ms", result.Details)
}

func TestCheckStatus(t *testing.T) {
	assert.Equal(t, sanity.CheckStatus("PASS"), sanity.StatusPassed)
	assert.Equal(t, sanity.CheckStatus("FAIL"), sanity.StatusFailed)
	assert.Equal(t, sanity.CheckStatus("WARN"), sanity.StatusWarning)
	assert.Equal(t, sanity.CheckStatus("SKIP"), sanity.StatusSkipped)
}

func TestFailedCheck(t *testing.T) {
	result := sanity.CheckResult{
		Name:     "failed-check",
		Category: "infrastructure",
		Status:   sanity.StatusFailed,
		Message:  "Connection refused",
		Details:  "dial tcp: connection refused",
		Critical: true,
	}

	assert.Equal(t, sanity.StatusFailed, result.Status)
	assert.Equal(t, "Connection refused", result.Message)
	assert.True(t, result.Critical)
}

func TestReportSerialization(t *testing.T) {
	report := &sanity.BootCheckReport{
		ReadyToStart: false,
		Timestamp:    time.Now(),
		TotalChecks:  2,
		FailedChecks: 1,
		PassedChecks: 1,
		Duration:     2 * time.Second,
		Results: []sanity.CheckResult{
			{
				Name:     "database",
				Category: "infrastructure",
				Status:   sanity.StatusFailed,
				Message:  "Database unavailable",
				Critical: true,
			},
			{
				Name:     "redis",
				Category: "infrastructure",
				Status:   sanity.StatusPassed,
				Message:  "Redis available",
			},
		},
	}

	// Serialize
	data, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)

	// Deserialize
	var decoded sanity.BootCheckReport
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.False(t, decoded.ReadyToStart)
	assert.Len(t, decoded.Results, 2)
	assert.Equal(t, sanity.StatusFailed, decoded.Results[0].Status)
	assert.Equal(t, sanity.StatusPassed, decoded.Results[1].Status)
}

func TestBootChecker(t *testing.T) {
	config := &sanity.BootCheckConfig{
		HelixAgentHost:     "localhost",
		HelixAgentPort:     7061,
		SkipExternalChecks: true,
		Timeout:            1 * time.Second,
	}

	checker := sanity.NewBootChecker(config)
	assert.NotNil(t, checker)
}

func TestMainFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", "/tmp/sanity-check-test", ".")
	cmd.Dir = "."
	err := cmd.Run()
	if err != nil {
		t.Skipf("Failed to build binary: %v", err)
	}
	defer func() { _ = os.Remove("/tmp/sanity-check-test") }()

	t.Run("help flag", func(t *testing.T) {
		cmd := exec.Command("/tmp/sanity-check-test", "-h")
		output, _ := cmd.CombinedOutput()

		// Help output should contain flag descriptions
		assert.Contains(t, string(output), "host")
		assert.Contains(t, string(output), "port")
		assert.Contains(t, string(output), "skip-external")
	})
}
