// Package main implements the API Quality Testing challenge.
// This challenge tests the SuperAgent API via OpenAI-compatible endpoints.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ChatCompletionRequest represents an OpenAI-compatible request.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents an OpenAI-compatible response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TestPrompt defines a test case.
type TestPrompt struct {
	ID               string   `json:"id"`
	Category         string   `json:"category"`
	Prompt           string   `json:"prompt"`
	ExpectedElements []string `json:"expected_elements,omitempty"`
	ExpectedMentions []string `json:"expected_mentions,omitempty"`
	ExpectedAnswer   string   `json:"expected_answer,omitempty"`
	MinLength        int      `json:"min_response_length,omitempty"`
	QualityThreshold float64  `json:"quality_threshold,omitempty"`
	RequiresReasoning bool    `json:"requires_reasoning,omitempty"`
}

// TestResult holds the result of a single test.
type TestResult struct {
	TestID       string            `json:"test_id"`
	Category     string            `json:"category"`
	Prompt       string            `json:"prompt"`
	Response     string            `json:"response"`
	StatusCode   int               `json:"status_code"`
	ResponseTime time.Duration     `json:"response_time"`
	Assertions   []AssertionResult `json:"assertions"`
	Passed       bool              `json:"passed"`
	QualityScore float64           `json:"quality_score"`
	IsMocked     bool              `json:"is_mocked"`
	Error        string            `json:"error,omitempty"`
}

// AssertionResult holds the outcome of an assertion.
type AssertionResult struct {
	Type    string `json:"type"`
	Target  string `json:"target,omitempty"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// ChallengeResult holds the complete challenge output.
type ChallengeResult struct {
	ChallengeID    string         `json:"challenge_id"`
	ChallengeName  string         `json:"challenge_name"`
	Timestamp      time.Time      `json:"timestamp"`
	Duration       time.Duration  `json:"duration"`
	Status         string         `json:"status"`
	TestResults    []TestResult   `json:"test_results"`
	Summary        TestSummary    `json:"summary"`
	APILogs        APILogSummary  `json:"api_logs"`
}

// TestSummary provides aggregated test statistics.
type TestSummary struct {
	TotalTests          int     `json:"total_tests"`
	PassedTests         int     `json:"passed_tests"`
	FailedTests         int     `json:"failed_tests"`
	AverageResponseTime float64 `json:"average_response_time_ms"`
	AverageQualityScore float64 `json:"average_quality_score"`
	MockDetections      int     `json:"mock_detections"`
	AssertionPassRate   float64 `json:"assertion_pass_rate"`
}

// APILogSummary provides API logging summary.
type APILogSummary struct {
	TotalRequests      int    `json:"total_requests"`
	TotalResponses     int    `json:"total_responses"`
	RequestLogFile     string `json:"request_log_file"`
	ResponseLogFile    string `json:"response_log_file"`
}

// APIClient handles HTTP requests to the API.
type APIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	RequestLog *os.File
	ResponseLog *os.File
}

// NewAPIClient creates a new API client.
func NewAPIClient(baseURL, apiKey string, requestLog, responseLog *os.File) *APIClient {
	return &APIClient{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		HTTPClient:  &http.Client{Timeout: 120 * time.Second},
		RequestLog:  requestLog,
		ResponseLog: responseLog,
	}
}

// SendRequest sends a chat completion request.
func (c *APIClient) SendRequest(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, int, time.Duration, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log request
	c.logRequest(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	start := time.Now()
	resp, err := c.HTTPClient.Do(httpReq)
	responseTime := time.Since(start)

	if err != nil {
		return nil, 0, responseTime, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, responseTime, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response
	c.logResponse(resp.StatusCode, respBody, responseTime)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, responseTime, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, resp.StatusCode, responseTime, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, resp.StatusCode, responseTime, nil
}

func (c *APIClient) logRequest(req ChatCompletionRequest) {
	if c.RequestLog == nil {
		return
	}
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"model":     req.Model,
		"messages":  req.Messages,
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintf(c.RequestLog, "%s\n", data)
}

func (c *APIClient) logResponse(statusCode int, body []byte, duration time.Duration) {
	if c.ResponseLog == nil {
		return
	}
	// Truncate body for logging
	bodyPreview := string(body)
	if len(bodyPreview) > 1000 {
		bodyPreview = bodyPreview[:1000] + "..."
	}
	entry := map[string]interface{}{
		"timestamp":       time.Now().Format(time.RFC3339),
		"status_code":     statusCode,
		"body_length":     len(body),
		"body_preview":    bodyPreview,
		"response_time_ms": duration.Milliseconds(),
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintf(c.ResponseLog, "%s\n", data)
}

// Test prompts
func getTestPrompts() []TestPrompt {
	return []TestPrompt{
		// Code generation tests
		{
			ID:               "go_factorial",
			Category:         "code_generation",
			Prompt:           "Write a Go function to calculate factorial of a number. Include error handling for negative numbers.",
			ExpectedElements: []string{"func", "if", "return"},
			MinLength:        100,
		},
		{
			ID:               "python_binary_search",
			Category:         "code_generation",
			Prompt:           "Implement binary search in Python. The function should take a sorted list and a target value, returning the index or -1 if not found.",
			ExpectedElements: []string{"def", "while", "return"},
			MinLength:        150,
		},
		{
			ID:               "typescript_class",
			Category:         "code_generation",
			Prompt:           "Create a TypeScript class for user management with properties for id, email, name, and methods for validation.",
			ExpectedElements: []string{"class", "constructor"},
			MinLength:        200,
		},

		// Code review tests
		{
			ID:               "division_bug",
			Category:         "code_review",
			Prompt:           "Review this Go code for bugs and explain the issues:\n```go\nfunc divide(a, b int) int {\n    return a / b\n}\n```",
			ExpectedMentions: []string{"zero", "division"},
			QualityThreshold: 0.7,
		},
		{
			ID:               "sql_injection",
			Category:         "code_review",
			Prompt:           "Find security issues in this code:\n```go\nfunc getUser(db *sql.DB, userInput string) (*User, error) {\n    query := \"SELECT * FROM users WHERE id = \" + userInput\n    return db.Query(query)\n}\n```",
			ExpectedMentions: []string{"injection", "sql"},
			QualityThreshold: 0.8,
		},

		// Reasoning tests
		{
			ID:                "sheep_problem",
			Category:          "reasoning",
			Prompt:            "A farmer has 17 sheep. All but 9 run away. How many sheep are left?",
			ExpectedAnswer:    "9",
			RequiresReasoning: true,
		},
		{
			ID:                "syllogism",
			Category:          "reasoning",
			Prompt:            "If all A are B, and all B are C, are all A also C? Explain your reasoning.",
			ExpectedMentions:  []string{"yes"},
			RequiresReasoning: true,
		},

		// Quality tests
		{
			ID:       "rest_practices",
			Category: "quality",
			Prompt:   "Explain REST API best practices for building scalable services. Cover naming conventions and HTTP methods.",
			MinLength: 200,
			ExpectedMentions: []string{"GET", "POST"},
		},
		{
			ID:             "capital_france",
			Category:       "quality",
			Prompt:         "What is the capital of France?",
			ExpectedAnswer: "Paris",
			MinLength:      5,
		},

		// Consensus test
		{
			ID:             "math_consensus",
			Category:       "consensus",
			Prompt:         "What is 2 + 2? Answer with just the number.",
			ExpectedAnswer: "4",
		},
	}
}

// Run a single test
func runTest(ctx context.Context, client *APIClient, prompt TestPrompt, model string) TestResult {
	result := TestResult{
		TestID:   prompt.ID,
		Category: prompt.Category,
		Prompt:   prompt.Prompt,
	}

	req := ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt.Prompt},
		},
	}

	resp, statusCode, responseTime, err := client.SendRequest(ctx, req)
	result.StatusCode = statusCode
	result.ResponseTime = responseTime

	if err != nil {
		result.Error = err.Error()
		result.Passed = false
		return result
	}

	if len(resp.Choices) == 0 {
		result.Error = "No choices in response"
		result.Passed = false
		return result
	}

	result.Response = resp.Choices[0].Message.Content

	// Run assertions
	result.Assertions = evaluateAssertions(prompt, result.Response)

	// Check for mock responses
	result.IsMocked = isMockedResponse(result.Response)
	if result.IsMocked {
		result.Assertions = append(result.Assertions, AssertionResult{
			Type:    "not_mock",
			Passed:  false,
			Message: "Response appears to be mocked",
		})
	}

	// Calculate quality score
	result.QualityScore = calculateQualityScore(result.Response, prompt)

	// Determine overall pass/fail
	result.Passed = true
	for _, a := range result.Assertions {
		if !a.Passed {
			result.Passed = false
			break
		}
	}

	return result
}

// Evaluate assertions for a response
func evaluateAssertions(prompt TestPrompt, response string) []AssertionResult {
	var results []AssertionResult
	responseLower := strings.ToLower(response)

	// not_empty assertion
	if strings.TrimSpace(response) == "" {
		results = append(results, AssertionResult{
			Type:    "not_empty",
			Passed:  false,
			Message: "Response is empty",
		})
	} else {
		results = append(results, AssertionResult{
			Type:    "not_empty",
			Passed:  true,
			Message: "Response is not empty",
		})
	}

	// min_length assertion
	if prompt.MinLength > 0 {
		if len(response) >= prompt.MinLength {
			results = append(results, AssertionResult{
				Type:    "min_length",
				Target:  fmt.Sprintf("%d", prompt.MinLength),
				Passed:  true,
				Message: fmt.Sprintf("Length %d >= %d", len(response), prompt.MinLength),
			})
		} else {
			results = append(results, AssertionResult{
				Type:    "min_length",
				Target:  fmt.Sprintf("%d", prompt.MinLength),
				Passed:  false,
				Message: fmt.Sprintf("Length %d < %d", len(response), prompt.MinLength),
			})
		}
	}

	// expected_elements assertion (for code)
	for _, elem := range prompt.ExpectedElements {
		if strings.Contains(responseLower, strings.ToLower(elem)) {
			results = append(results, AssertionResult{
				Type:    "contains",
				Target:  elem,
				Passed:  true,
				Message: fmt.Sprintf("Contains '%s'", elem),
			})
		} else {
			results = append(results, AssertionResult{
				Type:    "contains",
				Target:  elem,
				Passed:  false,
				Message: fmt.Sprintf("Does not contain '%s'", elem),
			})
		}
	}

	// expected_mentions assertion
	if len(prompt.ExpectedMentions) > 0 {
		found := false
		for _, mention := range prompt.ExpectedMentions {
			if strings.Contains(responseLower, strings.ToLower(mention)) {
				found = true
				break
			}
		}
		if found {
			results = append(results, AssertionResult{
				Type:    "contains_any",
				Target:  strings.Join(prompt.ExpectedMentions, ","),
				Passed:  true,
				Message: "Contains expected mention",
			})
		} else {
			results = append(results, AssertionResult{
				Type:    "contains_any",
				Target:  strings.Join(prompt.ExpectedMentions, ","),
				Passed:  false,
				Message: fmt.Sprintf("Does not contain any of: %v", prompt.ExpectedMentions),
			})
		}
	}

	// expected_answer assertion
	if prompt.ExpectedAnswer != "" {
		if strings.Contains(responseLower, strings.ToLower(prompt.ExpectedAnswer)) {
			results = append(results, AssertionResult{
				Type:    "contains",
				Target:  prompt.ExpectedAnswer,
				Passed:  true,
				Message: fmt.Sprintf("Contains expected answer '%s'", prompt.ExpectedAnswer),
			})
		} else {
			results = append(results, AssertionResult{
				Type:    "contains",
				Target:  prompt.ExpectedAnswer,
				Passed:  false,
				Message: fmt.Sprintf("Does not contain expected answer '%s'", prompt.ExpectedAnswer),
			})
		}
	}

	// reasoning_present assertion
	if prompt.RequiresReasoning {
		reasoningIndicators := []string{"because", "therefore", "since", "thus", "step", "first", "then", "reason"}
		hasReasoning := false
		for _, indicator := range reasoningIndicators {
			if strings.Contains(responseLower, indicator) {
				hasReasoning = true
				break
			}
		}
		if hasReasoning {
			results = append(results, AssertionResult{
				Type:    "reasoning_present",
				Passed:  true,
				Message: "Response contains reasoning",
			})
		} else {
			results = append(results, AssertionResult{
				Type:    "reasoning_present",
				Passed:  false,
				Message: "Response lacks reasoning indicators",
			})
		}
	}

	return results
}

// Check if response is mocked
func isMockedResponse(response string) bool {
	mockPatterns := []string{
		"lorem ipsum",
		"placeholder",
		"mock response",
		"TODO",
		"not implemented",
		"[MOCK]",
		"sample output",
		"test response",
	}

	lower := strings.ToLower(response)
	for _, pattern := range mockPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// Calculate quality score (0-1)
func calculateQualityScore(response string, prompt TestPrompt) float64 {
	score := 0.0
	factors := 0

	// Length factor
	if prompt.MinLength > 0 {
		lengthRatio := float64(len(response)) / float64(prompt.MinLength)
		if lengthRatio > 2.0 {
			lengthRatio = 1.0 // Cap at 100% for very long responses
		} else if lengthRatio > 1.0 {
			lengthRatio = 1.0
		}
		score += lengthRatio
		factors++
	}

	// Content relevance (simple heuristic)
	responseLower := strings.ToLower(response)

	// Check expected elements
	if len(prompt.ExpectedElements) > 0 {
		found := 0
		for _, elem := range prompt.ExpectedElements {
			if strings.Contains(responseLower, strings.ToLower(elem)) {
				found++
			}
		}
		score += float64(found) / float64(len(prompt.ExpectedElements))
		factors++
	}

	// Check mentions
	if len(prompt.ExpectedMentions) > 0 {
		found := 0
		for _, mention := range prompt.ExpectedMentions {
			if strings.Contains(responseLower, strings.ToLower(mention)) {
				found++
			}
		}
		score += float64(found) / float64(len(prompt.ExpectedMentions))
		factors++
	}

	// Check for code patterns if code category
	if strings.Contains(prompt.Category, "code") {
		codePatterns := []string{`func\s+\w+`, `def\s+\w+`, `class\s+\w+`, `return\s+`}
		codeFound := 0
		for _, pattern := range codePatterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(response) {
				codeFound++
			}
		}
		if codeFound > 0 {
			score += float64(codeFound) / float64(len(codePatterns))
			factors++
		}
	}

	// Not mocked bonus
	if !isMockedResponse(response) {
		score += 1.0
		factors++
	}

	if factors == 0 {
		return 0.5 // Default score if no factors apply
	}

	return score / float64(factors)
}

// Generate report
func generateReport(result ChallengeResult) string {
	var sb strings.Builder

	sb.WriteString("# API Quality Test Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", result.Timestamp.Format(time.RFC3339)))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", strings.ToUpper(result.Status)))
	sb.WriteString(fmt.Sprintf("| Total Tests | %d |\n", result.Summary.TotalTests))
	sb.WriteString(fmt.Sprintf("| Passed | %d |\n", result.Summary.PassedTests))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", result.Summary.FailedTests))
	sb.WriteString(fmt.Sprintf("| Mock Detections | %d |\n", result.Summary.MockDetections))
	sb.WriteString(fmt.Sprintf("| Average Response Time | %.0fms |\n", result.Summary.AverageResponseTime))
	sb.WriteString(fmt.Sprintf("| Average Quality Score | %.2f |\n", result.Summary.AverageQualityScore))
	sb.WriteString(fmt.Sprintf("| Assertion Pass Rate | %.0f%% |\n", result.Summary.AssertionPassRate*100))
	sb.WriteString(fmt.Sprintf("| Duration | %v |\n", result.Duration))

	sb.WriteString("\n## Test Results\n\n")
	sb.WriteString("| Test ID | Category | Passed | Quality | Response Time | Error |\n")
	sb.WriteString("|---------|----------|--------|---------|---------------|-------|\n")
	for _, t := range result.TestResults {
		passedStr := "No"
		if t.Passed {
			passedStr = "Yes"
		}
		errorStr := "-"
		if t.Error != "" {
			errorStr = truncate(t.Error, 30)
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.2f | %v | %s |\n",
			t.TestID, t.Category, passedStr, t.QualityScore, t.ResponseTime, errorStr))
	}

	// Detailed test results
	sb.WriteString("\n## Detailed Results\n\n")
	for _, t := range result.TestResults {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", t.TestID, t.Category))
		sb.WriteString(fmt.Sprintf("**Prompt:** %s\n\n", truncate(t.Prompt, 100)))
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", passedOrFailed(t.Passed)))
		sb.WriteString(fmt.Sprintf("**Quality Score:** %.2f\n\n", t.QualityScore))
		sb.WriteString(fmt.Sprintf("**Response Time:** %v\n\n", t.ResponseTime))

		if t.Error != "" {
			sb.WriteString(fmt.Sprintf("**Error:** %s\n\n", t.Error))
		}

		if len(t.Assertions) > 0 {
			sb.WriteString("**Assertions:**\n\n")
			for _, a := range t.Assertions {
				status := "PASS"
				if !a.Passed {
					status = "FAIL"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", status, a.Type, a.Message))
			}
			sb.WriteString("\n")
		}

		if t.Response != "" {
			sb.WriteString("**Response Preview:**\n\n")
			sb.WriteString("```\n")
			sb.WriteString(truncate(t.Response, 500))
			sb.WriteString("\n```\n\n")
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by SuperAgent Challenges*\n")

	return sb.String()
}

func passedOrFailed(passed bool) string {
	if passed {
		return "PASSED"
	}
	return "FAILED"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func main() {
	resultsDir := flag.String("results-dir", "", "Directory to store results")
	apiURL := flag.String("api-url", "", "SuperAgent API URL")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *resultsDir == "" {
		log.Fatal("--results-dir is required")
	}

	start := time.Now()
	ctx := context.Background()

	// Create directories
	resultsPath := filepath.Join(*resultsDir, "results")
	logsPath := filepath.Join(*resultsDir, "logs")
	if err := os.MkdirAll(resultsPath, 0755); err != nil {
		log.Fatalf("Failed to create results directory: %v", err)
	}
	if err := os.MkdirAll(logsPath, 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	// Determine API URL
	baseURL := *apiURL
	if baseURL == "" {
		baseURL = os.Getenv("SUPERAGENT_API_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	apiKey := os.Getenv("SUPERAGENT_API_KEY")

	// Open log files
	requestLogFile := filepath.Join(logsPath, "api_requests.log")
	responseLogFile := filepath.Join(logsPath, "api_responses.log")

	reqLog, err := os.Create(requestLogFile)
	if err != nil {
		log.Printf("Warning: Could not create request log: %v", err)
	}
	defer reqLog.Close()

	respLog, err := os.Create(responseLogFile)
	if err != nil {
		log.Printf("Warning: Could not create response log: %v", err)
	}
	defer respLog.Close()

	// Create API client
	client := NewAPIClient(baseURL, apiKey, reqLog, respLog)

	if *verbose {
		log.Printf("Testing API at: %s", baseURL)
	}

	// Get test prompts
	prompts := getTestPrompts()
	model := "superagent-debate" // Virtual debate group model

	// Run tests
	var testResults []TestResult
	for _, prompt := range prompts {
		if *verbose {
			log.Printf("Running test: %s", prompt.ID)
		}

		result := runTest(ctx, client, prompt, model)
		testResults = append(testResults, result)

		if *verbose {
			status := "PASS"
			if !result.Passed {
				status = "FAIL"
			}
			log.Printf("  %s: %s (%.2f, %v)", prompt.ID, status, result.QualityScore, result.ResponseTime)
		}
	}

	// Calculate summary
	var totalResponseTime time.Duration
	var totalQualityScore float64
	passedCount := 0
	failedCount := 0
	mockCount := 0
	totalAssertions := 0
	passedAssertions := 0

	for _, r := range testResults {
		totalResponseTime += r.ResponseTime
		totalQualityScore += r.QualityScore

		if r.Passed {
			passedCount++
		} else {
			failedCount++
		}

		if r.IsMocked {
			mockCount++
		}

		for _, a := range r.Assertions {
			totalAssertions++
			if a.Passed {
				passedAssertions++
			}
		}
	}

	avgResponseTime := float64(0)
	avgQualityScore := float64(0)
	assertionPassRate := float64(0)

	if len(testResults) > 0 {
		avgResponseTime = float64(totalResponseTime.Milliseconds()) / float64(len(testResults))
		avgQualityScore = totalQualityScore / float64(len(testResults))
	}
	if totalAssertions > 0 {
		assertionPassRate = float64(passedAssertions) / float64(totalAssertions)
	}

	// Determine status
	status := "passed"
	if failedCount > 0 || mockCount > 0 || avgQualityScore < 0.8 {
		status = "failed"
	}

	// Build result
	result := ChallengeResult{
		ChallengeID:   "api_quality_test",
		ChallengeName: "API Quality Testing",
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Status:        status,
		TestResults:   testResults,
		Summary: TestSummary{
			TotalTests:          len(testResults),
			PassedTests:         passedCount,
			FailedTests:         failedCount,
			AverageResponseTime: avgResponseTime,
			AverageQualityScore: avgQualityScore,
			MockDetections:      mockCount,
			AssertionPassRate:   assertionPassRate,
		},
		APILogs: APILogSummary{
			TotalRequests:   len(testResults),
			TotalResponses:  len(testResults),
			RequestLogFile:  requestLogFile,
			ResponseLogFile: responseLogFile,
		},
	}

	// Write outputs
	resultsFile := filepath.Join(resultsPath, "test_results.json")
	resultsData, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(resultsFile, resultsData, 0644); err != nil {
		log.Printf("Warning: Failed to write results: %v", err)
	}

	assertionsFile := filepath.Join(resultsPath, "assertion_report.json")
	var allAssertions []AssertionResult
	for _, t := range testResults {
		allAssertions = append(allAssertions, t.Assertions...)
	}
	assertionsData, _ := json.MarshalIndent(allAssertions, "", "  ")
	if err := os.WriteFile(assertionsFile, assertionsData, 0644); err != nil {
		log.Printf("Warning: Failed to write assertions: %v", err)
	}

	reportFile := filepath.Join(resultsPath, "api_quality_report.md")
	report := generateReport(result)
	if err := os.WriteFile(reportFile, []byte(report), 0644); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== API Quality Test Complete ===\n")
	fmt.Printf("Status: %s\n", strings.ToUpper(result.Status))
	fmt.Printf("Tests: %d passed, %d failed\n", passedCount, failedCount)
	fmt.Printf("Mock Detections: %d\n", mockCount)
	fmt.Printf("Average Quality Score: %.2f\n", avgQualityScore)
	fmt.Printf("Average Response Time: %.0fms\n", avgResponseTime)
	fmt.Printf("Assertion Pass Rate: %.0f%%\n", assertionPassRate*100)
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Results: %s\n", resultsPath)

	if result.Status == "failed" {
		os.Exit(1)
	}
}
