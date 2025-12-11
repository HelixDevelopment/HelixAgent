package services

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// SecuritySandbox provides sandboxed execution for tools and plugins
type SecuritySandbox struct {
	allowedCommands        map[string]bool
	timeout                time.Duration
	maxOutputSize          int
	enableContainerization bool
	containerImage         string
	resourceLimits         ResourceLimits
}

// ResourceLimits defines resource constraints
type ResourceLimits struct {
	MaxCPU    string // e.g., "500m"
	MaxMemory string // e.g., "256Mi"
	MaxDisk   string // e.g., "100Mi"
}

// NewSecuritySandbox creates a new security sandbox
func NewSecuritySandbox() *SecuritySandbox {
	return &SecuritySandbox{
		allowedCommands: map[string]bool{
			"grep":    true,
			"find":    true,
			"ls":      true,
			"cat":     true,
			"head":    true,
			"tail":    true,
			"wc":      true,
			"sort":    true,
			"uniq":    true,
			"python3": true,
			"node":    true,
			"go":      true,
			"rustc":   true,
			"gcc":     true,
			"javac":   true,
		},
		timeout:       30 * time.Second,
		maxOutputSize: 1024 * 1024, // 1MB
	}
}

// ExecuteSandboxed executes a command in a sandboxed environment
func (s *SecuritySandbox) ExecuteSandboxed(ctx context.Context, command string, args []string) (*SandboxedResult, error) {
	// Validate command
	if !s.allowedCommands[command] {
		return nil, fmt.Errorf("command %s is not allowed", command)
	}

	// Sanitize arguments
	for _, arg := range args {
		if strings.Contains(arg, "..") || strings.Contains(arg, "/etc") || strings.Contains(arg, "/proc") {
			return nil, fmt.Errorf("potentially dangerous argument: %s", arg)
		}
	}

	if s.enableContainerization {
		return s.executeInContainer(ctx, command, args)
	}

	return s.executeDirectly(ctx, command, args)
}

// executeInContainer executes command in a Docker container
func (s *SecuritySandbox) executeInContainer(ctx context.Context, command string, args []string) (*SandboxedResult, error) {
	// Build docker run command
	dockerArgs := []string{
		"run", "--rm",
		"--cpus", s.resourceLimits.MaxCPU,
		"--memory", s.resourceLimits.MaxMemory,
		"--read-only",
		"--tmpfs", "/tmp",
		"--network", "none",
		s.containerImage,
		command,
	}
	dockerArgs = append(dockerArgs, args...)

	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		log.Printf("Docker not available, falling back to direct execution")
		return s.executeDirectly(ctx, command, args)
	}

	return s.executeDirectly(ctx, "docker", dockerArgs)
}

// executeDirectly executes command directly with resource monitoring
func (s *SecuritySandbox) executeDirectly(ctx context.Context, command string, args []string) (*SandboxedResult, error) {
	// Prepare execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, command, args...)

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	result := &SandboxedResult{
		Command: command,
		Args:    args,
		Success: err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		result.Output = stderr.String()
	} else {
		// Limit output size
		output := stdout.String()
		if len(output) > s.maxOutputSize {
			output = output[:s.maxOutputSize]
			result.Truncated = true
		}
		result.Output = output
	}

	return result, nil
}

// ValidateToolExecution validates tool execution parameters
func (s *SecuritySandbox) ValidateToolExecution(toolName string, params map[string]interface{}) error {
	// Check for dangerous parameters
	for key, value := range params {
		if str, ok := value.(string); ok {
			if strings.Contains(str, ";") || strings.Contains(str, "|") || strings.Contains(str, "`") {
				return fmt.Errorf("potentially dangerous parameter %s: %s", key, str)
			}
		}
	}

	return nil
}

// SandboxedResult represents the result of a sandboxed execution
type SandboxedResult struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Output    string   `json:"output,omitempty"`
	Error     string   `json:"error,omitempty"`
	Success   bool     `json:"success"`
	Truncated bool     `json:"truncated,omitempty"`
}

// PerformanceMonitor monitors system performance
type PerformanceMonitor struct {
	metrics map[string]*PerformanceMetric
}

// PerformanceMetric represents a performance metric
type PerformanceMetric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics: make(map[string]*PerformanceMetric),
	}
}

// RecordMetric records a performance metric
func (pm *PerformanceMonitor) RecordMetric(name string, value float64, labels map[string]string) {
	pm.metrics[name] = &PerformanceMetric{
		Name:      name,
		Value:     value,
		Timestamp: time.Now(),
		Labels:    labels,
	}
}

// GetMetric retrieves a performance metric
func (pm *PerformanceMonitor) GetMetric(name string) (*PerformanceMetric, bool) {
	metric, exists := pm.metrics[name]
	return metric, exists
}

// GetAllMetrics returns all performance metrics
func (pm *PerformanceMonitor) GetAllMetrics() map[string]*PerformanceMetric {
	result := make(map[string]*PerformanceMetric)
	for k, v := range pm.metrics {
		result[k] = v
	}
	return result
}

// MonitorExecution monitors the execution of a function
func (pm *PerformanceMonitor) MonitorExecution(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start).Seconds()

	pm.RecordMetric(name+"_duration", duration, map[string]string{"operation": name})

	if err != nil {
		pm.RecordMetric(name+"_errors", 1, map[string]string{"operation": name, "error": err.Error()})
	} else {
		pm.RecordMetric(name+"_success", 1, map[string]string{"operation": name})
	}

	return err
}

// TestingFramework provides comprehensive testing utilities
type TestingFramework struct {
	sandbox   *SecuritySandbox
	monitor   *PerformanceMonitor
	testCases map[string]*TestCase
}

// TestCase represents a test case
type TestCase struct {
	Name        string
	Description string
	Function    func() error
	Expected    interface{}
	Actual      interface{}
	Passed      bool
	Duration    time.Duration
	Error       error
}

// NewTestingFramework creates a new testing framework
func NewTestingFramework() *TestingFramework {
	return &TestingFramework{
		sandbox:   NewSecuritySandbox(),
		monitor:   NewPerformanceMonitor(),
		testCases: make(map[string]*TestCase),
	}
}

// AddTestCase adds a test case
func (tf *TestingFramework) AddTestCase(name, description string, fn func() error) {
	tf.testCases[name] = &TestCase{
		Name:        name,
		Description: description,
		Function:    fn,
	}
}

// RunTests runs all test cases
func (tf *TestingFramework) RunTests(ctx context.Context) *TestResults {
	results := &TestResults{
		Total:     len(tf.testCases),
		Passed:    0,
		Failed:    0,
		StartTime: time.Now(),
	}

	for name, testCase := range tf.testCases {
		start := time.Now()

		err := tf.monitor.MonitorExecution("test_"+name, testCase.Function)

		testCase.Duration = time.Since(start)
		testCase.Error = err

		if err != nil {
			testCase.Passed = false
			results.Failed++
			results.Errors = append(results.Errors, TestError{
				TestName: name,
				Error:    err.Error(),
			})
		} else {
			testCase.Passed = true
			results.Passed++
		}
	}

	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)

	return results
}

// TestResults represents the results of running tests
type TestResults struct {
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Errors    []TestError   `json:"errors,omitempty"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// TestError represents a test error
type TestError struct {
	TestName string `json:"test_name"`
	Error    string `json:"error"`
}
