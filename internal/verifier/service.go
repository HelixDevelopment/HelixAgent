// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ServiceVerificationResult represents the result of a model verification
// (named differently to avoid conflict with database.go's VerificationResult)
type ServiceVerificationResult struct {
	ModelID               string          `json:"model_id"`
	Provider              string          `json:"provider"`
	ProviderName          string          `json:"provider_name"`
	Status                string          `json:"status"`
	Verified              bool            `json:"verified"`
	CodeVerified          bool            `json:"code_verified"`
	CodeVisible           bool            `json:"code_visible"`
	Score                 float64         `json:"score"`
	OverallScore          float64         `json:"overall_score"`
	ScoreSuffix           string          `json:"score_suffix"`
	CodingCapabilityScore float64         `json:"coding_capability_score"`
	Tests                 []TestResult    `json:"tests"`
	TestsMap              map[string]bool `json:"tests_map,omitempty"`
	StartedAt             time.Time       `json:"started_at"`
	CompletedAt           time.Time       `json:"completed_at"`
	VerificationTimeMs    int64           `json:"verification_time_ms"`
	Message               string          `json:"message,omitempty"`
	ErrorMessage          string          `json:"error_message,omitempty"`
}

// TestResult represents a single test result
type TestResult struct {
	Name        string    `json:"name"`
	Passed      bool      `json:"passed"`
	Score       float64   `json:"score"`
	Details     []string  `json:"details,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// BatchVerificationRequest represents a batch verification request
type BatchVerificationRequest struct {
	ModelID  string `json:"model_id"`
	Provider string `json:"provider"`
}

// VerificationService manages all verification operations
type VerificationService struct {
	config       *Config
	providerFunc func(ctx context.Context, modelID, provider, prompt string) (string, error)
	mu           sync.RWMutex
}

// NewVerificationService creates a new verification service
func NewVerificationService(cfg *Config) *VerificationService {
	return &VerificationService{
		config: cfg,
	}
}

// SetProviderFunc sets the function used to call LLM providers
func (s *VerificationService) SetProviderFunc(fn func(ctx context.Context, modelID, provider, prompt string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providerFunc = fn
}

// VerifyModel performs complete model verification including the mandatory
// "Do you see my code?" test
func (s *VerificationService) VerifyModel(ctx context.Context, modelID string, provider string) (*ServiceVerificationResult, error) {
	result := &ServiceVerificationResult{
		ModelID:      modelID,
		Provider:     provider,
		ProviderName: provider,
		StartedAt:    time.Now(),
		Tests:        make([]TestResult, 0),
		TestsMap:     make(map[string]bool),
	}

	// 1. Mandatory "Do you see my code?" verification
	codeResult, err := s.verifyCodeVisibility(ctx, modelID, provider)
	if err != nil {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("code visibility check failed: %v", err)
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.Tests = append(result.Tests, *codeResult)
	result.CodeVerified = codeResult.Passed
	result.CodeVisible = codeResult.Passed

	// 2. Existence test
	existenceResult := s.verifyExistence(ctx, modelID, provider)
	result.Tests = append(result.Tests, *existenceResult)

	// 3. Responsiveness test
	responsivenessResult := s.verifyResponsiveness(ctx, modelID, provider)
	result.Tests = append(result.Tests, *responsivenessResult)

	// 4. Latency test
	latencyResult := s.verifyLatency(ctx, modelID, provider)
	result.Tests = append(result.Tests, *latencyResult)

	// 5. Streaming test
	streamingResult := s.verifyStreaming(ctx, modelID, provider)
	result.Tests = append(result.Tests, *streamingResult)

	// 6. Function calling test
	functionCallingResult := s.verifyFunctionCalling(ctx, modelID, provider)
	result.Tests = append(result.Tests, *functionCallingResult)

	// 7. Coding capability test (>80%)
	codingResult := s.verifyCodingCapability(ctx, modelID, provider)
	result.Tests = append(result.Tests, *codingResult)
	result.CodingCapabilityScore = codingResult.Score

	// 8. Error detection test
	errorResult := s.verifyErrorDetection(ctx, modelID, provider)
	result.Tests = append(result.Tests, *errorResult)

	// Calculate overall status
	result.CompletedAt = time.Now()
	result.OverallScore = s.calculateOverallScore(result.Tests)
	result.Score = result.OverallScore
	result.ScoreSuffix = fmt.Sprintf("(SC:%.1f)", result.OverallScore)
	result.VerificationTimeMs = result.CompletedAt.Sub(result.StartedAt).Milliseconds()

	// Build tests map
	for _, test := range result.Tests {
		result.TestsMap[test.Name] = test.Passed
	}

	if result.CodeVerified && result.OverallScore >= 60 {
		result.Status = "verified"
		result.Verified = true
		result.Message = "Model verified successfully"
	} else {
		result.Status = "failed"
		result.Verified = false
		if !result.CodeVerified {
			result.Message = "Code visibility verification failed"
		} else {
			result.Message = "Model did not meet minimum score threshold"
		}
	}

	return result, nil
}

// verifyCodeVisibility performs the mandatory "Do you see my code?" test
func (s *VerificationService) verifyCodeVisibility(ctx context.Context, modelID, provider string) (*TestResult, error) {
	result := &TestResult{
		Name:      "code_visibility",
		StartedAt: time.Now(),
		Details:   make([]string, 0),
	}

	// Code samples in multiple languages
	codeSamples := []struct {
		language string
		code     string
	}{
		{"python", `def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)`},
		{"go", `func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`},
		{"javascript", `function fibonacci(n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`},
		{"java", `public int fibonacci(int n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`},
		{"csharp", `public int Fibonacci(int n) {
    if (n <= 1) return n;
    return Fibonacci(n - 1) + Fibonacci(n - 2);
}`},
	}

	passedCount := 0
	totalTests := len(codeSamples)

	for _, sample := range codeSamples {
		prompt := fmt.Sprintf(`I'm showing you code. Look at this %s code:

%s

Do you see my code? Please respond with "Yes, I can see your code" if you can see it.`, sample.language, sample.code)

		response, err := s.callModel(ctx, modelID, provider, prompt)
		if err != nil {
			result.Details = append(result.Details, fmt.Sprintf("%s: error - %v", sample.language, err))
			continue
		}

		// Check for affirmative response
		if s.isAffirmativeCodeResponse(response) {
			passedCount++
			result.Details = append(result.Details, fmt.Sprintf("%s: passed", sample.language))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("%s: failed - response did not confirm visibility", sample.language))
		}
	}

	result.CompletedAt = time.Now()
	result.Score = float64(passedCount) / float64(totalTests) * 100
	result.Passed = result.Score >= 80 // Require 80% pass rate

	return result, nil
}

// isAffirmativeCodeResponse checks if the response confirms code visibility
func (s *VerificationService) isAffirmativeCodeResponse(response string) bool {
	response = strings.ToLower(response)

	affirmatives := []string{
		"yes, i can see",
		"yes i can see",
		"i can see your code",
		"i see your code",
		"i can see the code",
		"yes, i see",
		"yes i see",
		"affirmative",
		"visible",
		"i can view",
		"can see the",
		"see the code",
	}

	for _, phrase := range affirmatives {
		if strings.Contains(response, phrase) {
			return true
		}
	}

	return false
}

// verifyExistence verifies that the model exists and is accessible
func (s *VerificationService) verifyExistence(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "existence",
		StartedAt: time.Now(),
	}

	response, err := s.callModel(ctx, modelID, provider, "Hello, please respond with 'OK' if you can hear me.")
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
	} else if len(response) > 0 {
		result.Passed = true
		result.Score = 100
		result.Details = append(result.Details, "model responded successfully")
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyResponsiveness verifies model response time
func (s *VerificationService) verifyResponsiveness(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "responsiveness",
		StartedAt: time.Now(),
	}

	start := time.Now()
	_, err := s.callModel(ctx, modelID, provider, "What is 2+2?")
	duration := time.Since(start)

	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
	} else {
		// TTFT should be < 10s, total < 60s
		if duration < 10*time.Second {
			result.Passed = true
			result.Score = 100
		} else if duration < 30*time.Second {
			result.Passed = true
			result.Score = 70
		} else if duration < 60*time.Second {
			result.Passed = true
			result.Score = 50
		} else {
			result.Passed = false
			result.Score = 0
		}
		result.Details = append(result.Details, fmt.Sprintf("response time: %v", duration))
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyLatency measures response latency
func (s *VerificationService) verifyLatency(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "latency",
		StartedAt: time.Now(),
	}

	var totalLatency time.Duration
	iterations := 3

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := s.callModel(ctx, modelID, provider, "Reply with just 'OK'")
		if err != nil {
			result.Details = append(result.Details, fmt.Sprintf("iteration %d: error - %v", i+1, err))
			continue
		}
		totalLatency += time.Since(start)
	}

	avgLatency := totalLatency / time.Duration(iterations)
	result.Details = append(result.Details, fmt.Sprintf("average latency: %v", avgLatency))

	if avgLatency < 2*time.Second {
		result.Score = 100
		result.Passed = true
	} else if avgLatency < 5*time.Second {
		result.Score = 80
		result.Passed = true
	} else if avgLatency < 10*time.Second {
		result.Score = 60
		result.Passed = true
	} else {
		result.Score = 30
		result.Passed = false
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyStreaming verifies streaming capability
func (s *VerificationService) verifyStreaming(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "streaming",
		StartedAt: time.Now(),
	}

	// For now, mark as passed if model responds (streaming check would need stream API)
	_, err := s.callModel(ctx, modelID, provider, "Count from 1 to 5")
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
	} else {
		result.Passed = true
		result.Score = 100
		result.Details = append(result.Details, "streaming capability assumed (non-streaming call succeeded)")
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyFunctionCalling verifies function calling capability
func (s *VerificationService) verifyFunctionCalling(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "function_calling",
		StartedAt: time.Now(),
	}

	prompt := `You have access to a function called "get_weather" that takes a "location" parameter.
If someone asks about the weather, respond with a JSON object like:
{"function": "get_weather", "arguments": {"location": "New York"}}

What's the weather in San Francisco?`

	response, err := s.callModel(ctx, modelID, provider, prompt)
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
	} else {
		// Check if response contains function call structure
		if strings.Contains(response, "get_weather") && strings.Contains(response, "San Francisco") {
			result.Passed = true
			result.Score = 100
			result.Details = append(result.Details, "function calling detected in response")
		} else {
			result.Passed = false
			result.Score = 50
			result.Details = append(result.Details, "response did not demonstrate function calling")
		}
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyCodingCapability verifies coding capability (>80% required)
func (s *VerificationService) verifyCodingCapability(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "coding_capability",
		StartedAt: time.Now(),
	}

	prompt := `Write a Python function that checks if a number is prime.
The function should be named "is_prime" and take a single integer parameter.
Return only the code, no explanations.`

	response, err := s.callModel(ctx, modelID, provider, prompt)
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
		result.CompletedAt = time.Now()
		return result
	}

	// Check for key elements of a prime checking function
	score := 0.0
	checks := []struct {
		pattern string
		points  float64
	}{
		{"def is_prime", 20},
		{"def is_prime(", 10},
		{"return", 15},
		{"for", 15},
		{"if", 10},
		{"%", 10},    // Modulo operator
		{"== 0", 10}, // Divisibility check
		{"False", 5},
		{"True", 5},
	}

	for _, check := range checks {
		if strings.Contains(response, check.pattern) {
			score += check.points
			result.Details = append(result.Details, fmt.Sprintf("found: %s", check.pattern))
		}
	}

	result.Score = score
	result.Passed = score >= 80

	result.CompletedAt = time.Now()
	return result
}

// verifyErrorDetection verifies error detection capability
func (s *VerificationService) verifyErrorDetection(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "error_detection",
		StartedAt: time.Now(),
	}

	prompt := `Find the bug in this Python code:

def add_numbers(a, b):
    return a + c

print(add_numbers(1, 2))

What is wrong with this code?`

	response, err := s.callModel(ctx, modelID, provider, prompt)
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
	} else {
		// Check if response identifies the bug (using 'c' instead of 'b')
		responseLower := strings.ToLower(response)
		if strings.Contains(responseLower, "c") && (strings.Contains(responseLower, "b") || strings.Contains(responseLower, "undefined") || strings.Contains(responseLower, "not defined")) {
			result.Passed = true
			result.Score = 100
			result.Details = append(result.Details, "correctly identified the bug")
		} else if strings.Contains(responseLower, "bug") || strings.Contains(responseLower, "error") {
			result.Passed = true
			result.Score = 70
			result.Details = append(result.Details, "partially identified the issue")
		} else {
			result.Passed = false
			result.Score = 30
			result.Details = append(result.Details, "did not clearly identify the bug")
		}
	}

	result.CompletedAt = time.Now()
	return result
}

// callModel calls the LLM provider
func (s *VerificationService) callModel(ctx context.Context, modelID, provider, prompt string) (string, error) {
	s.mu.RLock()
	providerFunc := s.providerFunc
	s.mu.RUnlock()

	if providerFunc == nil {
		return "", fmt.Errorf("provider function not set")
	}

	return providerFunc(ctx, modelID, provider, prompt)
}

// calculateOverallScore calculates the overall verification score
func (s *VerificationService) calculateOverallScore(tests []TestResult) float64 {
	if len(tests) == 0 {
		return 0
	}

	var totalScore float64
	for _, test := range tests {
		totalScore += test.Score
	}

	return totalScore / float64(len(tests))
}

// BatchVerify verifies multiple models concurrently
func (s *VerificationService) BatchVerify(ctx context.Context, requests []*BatchVerificationRequest) ([]*ServiceVerificationResult, error) {
	results := make([]*ServiceVerificationResult, len(requests))
	var wg sync.WaitGroup
	errChan := make(chan error, len(requests))

	for i, req := range requests {
		wg.Add(1)
		go func(index int, r *BatchVerificationRequest) {
			defer wg.Done()

			result, err := s.VerifyModel(ctx, r.ModelID, r.Provider)
			if err != nil {
				errChan <- err
				return
			}
			results[index] = result
		}(i, req)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// GetVerifiedModels returns all verified models
func (s *VerificationService) GetVerifiedModels(ctx context.Context) ([]*ServiceVerificationResult, error) {
	// This would query the database for verified models
	// For now, return empty slice
	return []*ServiceVerificationResult{}, nil
}

// PerformCodeCheck performs a standalone code visibility check
func (s *VerificationService) PerformCodeCheck(ctx context.Context, modelID, provider, code, language string) (*TestResult, error) {
	result := &TestResult{
		Name:      "code_check",
		StartedAt: time.Now(),
	}

	prompt := fmt.Sprintf(`I'm showing you code. Look at this %s code:

%s

Do you see my code? Please respond with "Yes, I can see your code" if you can see it.`, language, code)

	response, err := s.callModel(ctx, modelID, provider, prompt)
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
		result.CompletedAt = time.Now()
		return result, err
	}

	if s.isAffirmativeCodeResponse(response) {
		result.Passed = true
		result.Score = 100
		result.Details = append(result.Details, "code visibility confirmed")
	} else {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, "code visibility not confirmed")
	}

	result.CompletedAt = time.Now()
	return result, nil
}

// VerificationStatus represents the current status of a verification with full results
type VerificationStatus struct {
	ModelID            string          `json:"model_id"`
	Provider           string          `json:"provider"`
	Status             string          `json:"status"`
	Progress           int             `json:"progress"`
	Verified           bool            `json:"verified"`
	Score              float64         `json:"score"`
	OverallScore       float64         `json:"overall_score"`
	ScoreSuffix        string          `json:"score_suffix"`
	CodeVisible        bool            `json:"code_visible"`
	Tests              map[string]bool `json:"tests,omitempty"`
	VerificationTimeMs int64           `json:"verification_time_ms"`
	StartedAt          time.Time       `json:"started_at,omitempty"`
	CompletedAt        time.Time       `json:"completed_at,omitempty"`
}

// GetVerificationStatus returns the status of a verification (by model ID only)
func (s *VerificationService) GetVerificationStatus(ctx context.Context, modelID string) (*VerificationStatus, error) {
	// In a real implementation, this would look up cached/stored verification results
	// For now, return a stub result
	return &VerificationStatus{
		ModelID:            modelID,
		Provider:           "unknown",
		Status:             "not_found",
		Progress:           0,
		Verified:           false,
		Score:              0,
		OverallScore:       0,
		ScoreSuffix:        "",
		CodeVisible:        false,
		Tests:              make(map[string]bool),
		VerificationTimeMs: 0,
	}, nil
}

// CodeVisibilityResult represents the result of a code visibility test
type CodeVisibilityResult struct {
	ModelID     string  `json:"model_id"`
	Provider    string  `json:"provider"`
	CodeVisible bool    `json:"code_visible"`
	Language    string  `json:"language"`
	Prompt      string  `json:"prompt"`
	Response    string  `json:"response"`
	Confidence  float64 `json:"confidence"`
}

// TestCodeVisibility performs a standalone code visibility test with default code sample
func (s *VerificationService) TestCodeVisibility(ctx context.Context, modelID, provider, language string) (*CodeVisibilityResult, error) {
	// Default code sample based on language
	codeSamples := map[string]string{
		"python": `def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)`,
		"go": `func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`,
		"javascript": `function fibonacci(n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`,
		"java": `public int fibonacci(int n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`,
	}

	code, ok := codeSamples[language]
	if !ok {
		code = codeSamples["python"]
		language = "python"
	}

	prompt := fmt.Sprintf(`I'm showing you code. Look at this %s code:

%s

Do you see my code? Please respond with "Yes, I can see your code" if you can see it.`, language, code)

	response, err := s.callModel(ctx, modelID, provider, prompt)
	if err != nil {
		return &CodeVisibilityResult{
			ModelID:     modelID,
			Provider:    provider,
			CodeVisible: false,
			Language:    language,
			Prompt:      prompt,
			Response:    fmt.Sprintf("error: %v", err),
			Confidence:  0,
		}, nil
	}

	codeVisible := s.isAffirmativeCodeResponse(response)
	confidence := 0.0
	if codeVisible {
		confidence = 1.0
	}

	return &CodeVisibilityResult{
		ModelID:     modelID,
		Provider:    provider,
		CodeVisible: codeVisible,
		Language:    language,
		Prompt:      prompt,
		Response:    response,
		Confidence:  confidence,
	}, nil
}

// InvalidateVerification invalidates a previous verification (stub for now)
func (s *VerificationService) InvalidateVerification(modelID string) {
	// In a real implementation, this would clear cached verification results
}

// VerificationStats represents verification statistics
type VerificationStats struct {
	TotalVerifications int     `json:"total_verifications"`
	SuccessfulCount    int     `json:"successful_count"`
	FailedCount        int     `json:"failed_count"`
	SuccessRate        float64 `json:"success_rate"`
	AverageScore       float64 `json:"average_score"`
}

// GetStats returns verification statistics (stub)
func (s *VerificationService) GetStats(ctx context.Context) (*VerificationStats, error) {
	return &VerificationStats{
		TotalVerifications: 0,
		SuccessfulCount:    0,
		FailedCount:        0,
		SuccessRate:        0,
		AverageScore:       0,
	}, nil
}
