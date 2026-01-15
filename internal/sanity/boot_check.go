// Package sanity provides boot-time sanity checks for HelixAgent
package sanity

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CheckResult represents the result of a sanity check
type CheckResult struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Status      CheckStatus   `json:"status"`
	Message     string        `json:"message,omitempty"`
	Details     string        `json:"details,omitempty"`
	Duration    time.Duration `json:"duration"`
	Critical    bool          `json:"critical"`
	Timestamp   time.Time     `json:"timestamp"`
}

// CheckStatus represents the status of a check
type CheckStatus string

const (
	StatusPassed  CheckStatus = "PASS"
	StatusFailed  CheckStatus = "FAIL"
	StatusWarning CheckStatus = "WARN"
	StatusSkipped CheckStatus = "SKIP"
)

// BootCheckReport represents the full sanity check report
type BootCheckReport struct {
	Timestamp       time.Time      `json:"timestamp"`
	Duration        time.Duration  `json:"duration"`
	TotalChecks     int            `json:"total_checks"`
	PassedChecks    int            `json:"passed_checks"`
	FailedChecks    int            `json:"failed_checks"`
	WarningChecks   int            `json:"warning_checks"`
	SkippedChecks   int            `json:"skipped_checks"`
	CriticalFailure bool           `json:"critical_failure"`
	Results         []CheckResult  `json:"results"`
	ReadyToStart    bool           `json:"ready_to_start"`
}

// BootChecker performs sanity checks at system boot
type BootChecker struct {
	config     *BootCheckConfig
	httpClient *http.Client
	results    []CheckResult
	mu         sync.Mutex
}

// BootCheckConfig contains configuration for boot checks
type BootCheckConfig struct {
	HelixAgentHost     string
	HelixAgentPort     int
	PostgresHost       string
	PostgresPort       int
	RedisHost          string
	RedisPort          int
	CogneeHost         string
	CogneePort         int
	Timeout            time.Duration
	SkipExternalChecks bool
}

// DefaultConfig returns default boot check configuration
func DefaultConfig() *BootCheckConfig {
	return &BootCheckConfig{
		HelixAgentHost: "localhost",
		HelixAgentPort: 7061,
		PostgresHost:   "localhost",
		PostgresPort:   5432,
		RedisHost:      "localhost",
		RedisPort:      6379,
		CogneeHost:     "localhost",
		CogneePort:     8000,
		Timeout:        10 * time.Second,
	}
}

// NewBootChecker creates a new boot checker
func NewBootChecker(config *BootCheckConfig) *BootChecker {
	if config == nil {
		config = DefaultConfig()
	}
	return &BootChecker{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		results: make([]CheckResult, 0),
	}
}

// RunAllChecks runs all sanity checks and returns a report
func (bc *BootChecker) RunAllChecks(ctx context.Context) *BootCheckReport {
	start := time.Now()
	logrus.Info("Starting boot sanity checks...")

	var wg sync.WaitGroup

	// Infrastructure checks (parallel)
	wg.Add(4)
	go func() { defer wg.Done(); bc.checkHelixAgentHealth(ctx) }()
	go func() { defer wg.Done(); bc.checkPostgresConnection(ctx) }()
	go func() { defer wg.Done(); bc.checkRedisConnection(ctx) }()
	go func() { defer wg.Done(); bc.checkCogneeHealth(ctx) }()
	wg.Wait()

	// Environment checks (sequential, fast)
	bc.checkEnvironmentVariables()
	bc.checkRequiredFiles()
	bc.checkPortAvailability()
	bc.checkDiskSpace()

	// External provider checks (parallel, optional)
	if !bc.config.SkipExternalChecks {
		wg.Add(3)
		go func() { defer wg.Done(); bc.checkDeepSeekProvider(ctx) }()
		go func() { defer wg.Done(); bc.checkGeminiProvider(ctx) }()
		go func() { defer wg.Done(); bc.checkQwenProvider(ctx) }()
		wg.Wait()
	}

	// Generate report
	report := bc.generateReport(start)
	bc.printReport(report)

	return report
}

// checkHelixAgentHealth checks if HelixAgent is healthy
func (bc *BootChecker) checkHelixAgentHealth(ctx context.Context) {
	start := time.Now()
	url := fmt.Sprintf("http://%s:%d/health", bc.config.HelixAgentHost, bc.config.HelixAgentPort)

	result := CheckResult{
		Name:      "HelixAgent Health",
		Category:  "Infrastructure",
		Critical:  true,
		Timestamp: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Status = StatusFailed
		result.Message = "Failed to create request"
		result.Details = err.Error()
	} else {
		resp, err := bc.httpClient.Do(req)
		if err != nil {
			result.Status = StatusFailed
			result.Message = "HelixAgent not responding"
			result.Details = err.Error()
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var healthResp map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&healthResp) == nil {
					status, _ := healthResp["status"].(string)
					result.Status = StatusPassed
					result.Message = fmt.Sprintf("HelixAgent is %s", status)
				} else {
					result.Status = StatusPassed
					result.Message = "HelixAgent is responding"
				}
			} else {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("Unexpected status: %d", resp.StatusCode)
			}
		}
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkPostgresConnection checks PostgreSQL connectivity
func (bc *BootChecker) checkPostgresConnection(ctx context.Context) {
	start := time.Now()
	addr := net.JoinHostPort(bc.config.PostgresHost, fmt.Sprintf("%d", bc.config.PostgresPort))

	result := CheckResult{
		Name:      "PostgreSQL Connection",
		Category:  "Database",
		Critical:  true,
		Timestamp: time.Now(),
	}

	conn, err := net.DialTimeout("tcp", addr, bc.config.Timeout)
	if err != nil {
		result.Status = StatusFailed
		result.Message = "PostgreSQL not reachable"
		result.Details = err.Error()
	} else {
		conn.Close()
		result.Status = StatusPassed
		result.Message = fmt.Sprintf("PostgreSQL is reachable at %s", addr)
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkRedisConnection checks Redis connectivity
func (bc *BootChecker) checkRedisConnection(ctx context.Context) {
	start := time.Now()
	addr := net.JoinHostPort(bc.config.RedisHost, fmt.Sprintf("%d", bc.config.RedisPort))

	result := CheckResult{
		Name:      "Redis Connection",
		Category:  "Cache",
		Critical:  false, // Redis is optional
		Timestamp: time.Now(),
	}

	conn, err := net.DialTimeout("tcp", addr, bc.config.Timeout)
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Redis not available (optional)"
		result.Details = err.Error()
	} else {
		conn.Close()
		result.Status = StatusPassed
		result.Message = fmt.Sprintf("Redis is reachable at %s", addr)
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkCogneeHealth checks Cognee service health
func (bc *BootChecker) checkCogneeHealth(ctx context.Context) {
	start := time.Now()
	url := fmt.Sprintf("http://%s:%d/api/v1/health", bc.config.CogneeHost, bc.config.CogneePort)

	result := CheckResult{
		Name:      "Cognee Service",
		Category:  "AI Services",
		Critical:  false, // Cognee is optional
		Timestamp: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Cognee not configured"
		result.Details = err.Error()
	} else {
		resp, err := bc.httpClient.Do(req)
		if err != nil {
			result.Status = StatusWarning
			result.Message = "Cognee not responding (optional)"
			result.Details = err.Error()
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				result.Status = StatusPassed
				result.Message = "Cognee is healthy"
			} else {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("Cognee returned: %d", resp.StatusCode)
			}
		}
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkEnvironmentVariables checks required environment variables
func (bc *BootChecker) checkEnvironmentVariables() {
	start := time.Now()

	requiredVars := map[string]bool{
		"DEEPSEEK_API_KEY": false,
		"GEMINI_API_KEY":   false,
		"QWEN_API_KEY":     false,
	}

	optionalVars := []string{
		"CLAUDE_API_KEY",
		"OPENAI_API_KEY",
		"HELIXAGENT_API_KEY",
		"GITHUB_TOKEN",
	}

	result := CheckResult{
		Name:      "Environment Variables",
		Category:  "Configuration",
		Critical:  true,
		Timestamp: time.Now(),
	}

	var missing []string
	var found []string
	var optionalFound []string

	for varName := range requiredVars {
		if os.Getenv(varName) != "" {
			found = append(found, varName)
		} else {
			missing = append(missing, varName)
		}
	}

	for _, varName := range optionalVars {
		if os.Getenv(varName) != "" {
			optionalFound = append(optionalFound, varName)
		}
	}

	// At least one API key should be present
	if len(found) == 0 && len(optionalFound) == 0 {
		result.Status = StatusFailed
		result.Message = "No API keys configured"
		result.Details = "Set at least one of: " + strings.Join(append(missing, optionalVars...), ", ")
	} else if len(found) < len(requiredVars)/2 {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("%d/%d recommended API keys found", len(found), len(requiredVars))
		result.Details = fmt.Sprintf("Found: %v, Missing: %v", found, missing)
	} else {
		result.Status = StatusPassed
		result.Message = fmt.Sprintf("%d API keys configured", len(found)+len(optionalFound))
		result.Details = fmt.Sprintf("Required: %v, Optional: %v", found, optionalFound)
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkRequiredFiles checks for required configuration files
func (bc *BootChecker) checkRequiredFiles() {
	start := time.Now()

	files := []struct {
		path     string
		required bool
	}{
		{"configs/production.yaml", false},
		{"configs/development.yaml", false},
		{".env", false},
	}

	result := CheckResult{
		Name:      "Configuration Files",
		Category:  "Configuration",
		Critical:  false,
		Timestamp: time.Now(),
	}

	var existing []string
	var missing []string

	for _, f := range files {
		if _, err := os.Stat(f.path); err == nil {
			existing = append(existing, f.path)
		} else if f.required {
			missing = append(missing, f.path)
		}
	}

	if len(missing) > 0 {
		result.Status = StatusFailed
		result.Message = "Missing required files"
		result.Details = strings.Join(missing, ", ")
	} else if len(existing) == 0 {
		result.Status = StatusWarning
		result.Message = "No configuration files found"
	} else {
		result.Status = StatusPassed
		result.Message = fmt.Sprintf("%d config files found", len(existing))
		result.Details = strings.Join(existing, ", ")
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkPortAvailability checks if the main port is available
func (bc *BootChecker) checkPortAvailability() {
	start := time.Now()
	addr := fmt.Sprintf(":%d", bc.config.HelixAgentPort)

	result := CheckResult{
		Name:      "Port Availability",
		Category:  "Network",
		Critical:  false, // Not critical if HelixAgent is already running
		Timestamp: time.Now(),
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Port is in use - might be HelixAgent already running
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("Port %d in use (HelixAgent may be running)", bc.config.HelixAgentPort)
	} else {
		listener.Close()
		result.Status = StatusPassed
		result.Message = fmt.Sprintf("Port %d is available", bc.config.HelixAgentPort)
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkDiskSpace checks available disk space
func (bc *BootChecker) checkDiskSpace() {
	start := time.Now()

	result := CheckResult{
		Name:      "Disk Space",
		Category:  "System",
		Critical:  false,
		Timestamp: time.Now(),
	}

	// Basic check - we just verify we can write to the current directory
	tempFile := ".sanity_check_temp"
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to write to disk"
		result.Details = err.Error()
	} else {
		os.Remove(tempFile)
		result.Status = StatusPassed
		result.Message = "Disk write verified"
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// checkDeepSeekProvider checks DeepSeek API availability
func (bc *BootChecker) checkDeepSeekProvider(ctx context.Context) {
	bc.checkExternalProvider(ctx, "DeepSeek", "https://api.deepseek.com/v1/models", "DEEPSEEK_API_KEY")
}

// checkGeminiProvider checks Gemini API availability
func (bc *BootChecker) checkGeminiProvider(ctx context.Context) {
	bc.checkExternalProvider(ctx, "Gemini", "https://generativelanguage.googleapis.com/v1/models", "GEMINI_API_KEY")
}

// checkQwenProvider checks Qwen API availability
func (bc *BootChecker) checkQwenProvider(ctx context.Context) {
	bc.checkExternalProvider(ctx, "Qwen", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation", "QWEN_API_KEY")
}

// checkExternalProvider checks an external LLM provider
func (bc *BootChecker) checkExternalProvider(ctx context.Context, name, url, envVar string) {
	start := time.Now()

	result := CheckResult{
		Name:      fmt.Sprintf("%s Provider", name),
		Category:  "LLM Providers",
		Critical:  false,
		Timestamp: time.Now(),
	}

	apiKey := os.Getenv(envVar)
	if apiKey == "" {
		result.Status = StatusSkipped
		result.Message = fmt.Sprintf("No API key (%s)", envVar)
		result.Duration = time.Since(start)
		bc.addResult(result)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Failed to create request"
		result.Details = err.Error()
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := bc.httpClient.Do(req)
		if err != nil {
			result.Status = StatusWarning
			result.Message = fmt.Sprintf("%s not reachable", name)
			result.Details = err.Error()
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				result.Status = StatusPassed
				result.Message = fmt.Sprintf("%s is available", name)
			} else {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("%s returned: %d", name, resp.StatusCode)
			}
		}
	}

	result.Duration = time.Since(start)
	bc.addResult(result)
}

// addResult safely adds a result to the list
func (bc *BootChecker) addResult(result CheckResult) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.results = append(bc.results, result)
}

// generateReport generates the final report
func (bc *BootChecker) generateReport(start time.Time) *BootCheckReport {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	report := &BootCheckReport{
		Timestamp:   start,
		Duration:    time.Since(start),
		TotalChecks: len(bc.results),
		Results:     bc.results,
	}

	for _, result := range bc.results {
		switch result.Status {
		case StatusPassed:
			report.PassedChecks++
		case StatusFailed:
			report.FailedChecks++
			if result.Critical {
				report.CriticalFailure = true
			}
		case StatusWarning:
			report.WarningChecks++
		case StatusSkipped:
			report.SkippedChecks++
		}
	}

	report.ReadyToStart = !report.CriticalFailure

	return report
}

// printReport prints the report to console
func (bc *BootChecker) printReport(report *BootCheckReport) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("                   HELIXAGENT BOOT SANITY CHECK REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nTimestamp: %s\n", report.Timestamp.Format(time.RFC3339))
	fmt.Printf("Duration: %v\n\n", report.Duration)

	// Group results by category
	categories := make(map[string][]CheckResult)
	for _, result := range report.Results {
		categories[result.Category] = append(categories[result.Category], result)
	}

	for category, results := range categories {
		fmt.Printf("\n[%s]\n", category)
		fmt.Println(strings.Repeat("-", 50))
		for _, result := range results {
			status := string(result.Status)
			switch result.Status {
			case StatusPassed:
				status = "PASS"
			case StatusFailed:
				status = "FAIL"
			case StatusWarning:
				status = "WARN"
			case StatusSkipped:
				status = "SKIP"
			}
			fmt.Printf("  [%s] %s: %s\n", status, result.Name, result.Message)
			if result.Details != "" && result.Status != StatusPassed {
				fmt.Printf("         Details: %s\n", result.Details)
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Printf("SUMMARY: %d/%d passed, %d warnings, %d failed, %d skipped\n",
		report.PassedChecks, report.TotalChecks,
		report.WarningChecks, report.FailedChecks, report.SkippedChecks)

	if report.ReadyToStart {
		fmt.Println("\n" + strings.Repeat("=", 70))
		fmt.Println("                        SYSTEM READY TO START")
		fmt.Println(strings.Repeat("=", 70))
	} else {
		fmt.Println("\n" + strings.Repeat("=", 70))
		fmt.Println("              CRITICAL FAILURE - SYSTEM NOT READY")
		fmt.Println(strings.Repeat("=", 70))
	}
	fmt.Println()
}

// RunSanityCheck is a convenience function to run all checks
func RunSanityCheck(config *BootCheckConfig) *BootCheckReport {
	checker := NewBootChecker(config)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return checker.RunAllChecks(ctx)
}
