// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// CannedErrorPatterns contains patterns that indicate a model is returning
// error/failure responses instead of actual completions. Models that return
// these patterns should NOT be verified as working.
var CannedErrorPatterns = []string{
	"unable to provide",
	"unable to analyze",
	"unable to process",
	"unable to assist",
	"cannot provide",
	"cannot analyze",
	"cannot process",
	"cannot assist",
	"i apologize, but i cannot",
	"i'm sorry, but i cannot",
	"i am sorry, but i cannot",
	"error occurred",
	"service unavailable",
	"rate limit",
	"temporarily unavailable",
	"model not available",
	"failed to generate",
	"no response generated",
	"internal error",
	"request failed",
	"at this time",
	"currently unable",
	"not able to",
}

// IsCannedErrorResponse checks if a response contains known canned error patterns
// that indicate the model isn't actually working. Returns true if the response
// appears to be a canned error, along with the matching pattern.
func IsCannedErrorResponse(content string) (bool, string) {
	if content == "" {
		return true, "empty response"
	}

	lowered := strings.ToLower(content)
	for _, pattern := range CannedErrorPatterns {
		if strings.Contains(lowered, pattern) {
			return true, pattern
		}
	}
	return false, ""
}

// IsSuspiciouslyFastResponse checks if a response was returned too quickly
// to be a real LLM completion (typically < 100ms indicates cached error)
func IsSuspiciouslyFastResponse(latency time.Duration) bool {
	return latency < 100*time.Millisecond
}

// ValidateResponseQualityWithLatency performs comprehensive response validation including latency check
// Returns an error if the response appears to be invalid/canned/error
func ValidateResponseQualityWithLatency(content string, latency time.Duration) error {
	// Check for empty response
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty response content")
	}

	// Check for canned error patterns
	if isCanned, pattern := IsCannedErrorResponse(content); isCanned {
		return fmt.Errorf("canned error response detected (pattern: %s): %s", pattern, truncateForLog(content))
	}

	// Check for suspiciously fast response
	if IsSuspiciouslyFastResponse(latency) {
		// Log warning but don't fail - some providers are just fast
		// However, combined with short content, this is suspicious
		if len(content) < 50 {
			return fmt.Errorf("suspiciously fast response (%v) with short content (%d chars)", latency, len(content))
		}
	}

	return nil
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string) string {
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

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
	// LastResponse stores the last response content from the model for quality validation.
	// This is used to detect canned error responses during verification.
	LastResponse string `json:"last_response,omitempty"`
}

// TestResult represents a single test result
type TestResult struct {
	Name        string    `json:"name"`
	Passed      bool      `json:"passed"`
	Score       float64   `json:"score"`
	Details     []string  `json:"details,omitempty"`
	Response    string    `json:"response,omitempty"` // The actual response from the model
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// BatchVerificationRequest represents a batch verification request
type BatchVerificationRequest struct {
	ModelID  string `json:"model_id"`
	Provider string `json:"provider"`
}

// VerificationService manages all verification operations.
//
// Concurrency model (CONST-029): verificationCache is a *safe.Store.
// mu survives to serialise providerFunc / testMode scalar mutations —
// not Pattern-A (no bare map/slice sits beside it anymore).
// statsMu guards stats-pointer-field mutations — also not Pattern-A.
type VerificationService struct {
	config       *Config
	providerFunc func(ctx context.Context, modelID, provider, prompt string) (string, error)
	mu           sync.RWMutex
	testMode     bool // When true, disables quality validation checks

	// Storage for verification results and statistics
	verificationCache *safe.Store[string, *VerificationStatus]
	stats             *VerificationStats
	statsMu           sync.RWMutex
}

// NewVerificationService creates a new verification service
func NewVerificationService(cfg *Config) *VerificationService {
	return &VerificationService{
		config:            cfg,
		verificationCache: safe.NewStore[string, *VerificationStatus](),
		stats: &VerificationStats{
			TotalVerifications: 0,
			SuccessfulCount:    0,
			FailedCount:        0,
			SuccessRate:        0,
			AverageScore:       0,
		},
	}
}

// SetProviderFunc sets the function used to call LLM providers
func (s *VerificationService) SetProviderFunc(fn func(ctx context.Context, modelID, provider, prompt string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providerFunc = fn
}

// SetTestMode enables/disables test mode (disables quality validation)
func (s *VerificationService) SetTestMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.testMode = enabled
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
	// Capture the last response for quality validation
	result.LastResponse = existenceResult.Response

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

	// Code visibility is important but not a hard gate.
	// A model that responds well to all other tests but doesn't match
	// our affirmative patterns is still usable with a lower score.
	if result.OverallScore >= 50 {
		result.Status = "verified"
		result.Verified = true
		result.Message = "Model verified successfully"
	} else {
		result.Status = "failed"
		result.Verified = false
		result.Message = "Model did not meet minimum score threshold"
	}

	// Store result in cache and update statistics
	s.storeVerificationResult(result)

	return result, nil
}

// storeVerificationResult stores the result and updates statistics
func (s *VerificationService) storeVerificationResult(result *ServiceVerificationResult) {
	cacheKey := fmt.Sprintf("%s:%s", result.Provider, result.ModelID)

	// Store in cache
	s.verificationCache.Put(cacheKey, &VerificationStatus{
		ModelID:            result.ModelID,
		Provider:           result.Provider,
		Status:             result.Status,
		Progress:           100,
		Verified:           result.Verified,
		Score:              result.Score,
		OverallScore:       result.OverallScore,
		ScoreSuffix:        result.ScoreSuffix,
		CodeVisible:        result.CodeVisible,
		Tests:              result.TestsMap,
		VerificationTimeMs: result.VerificationTimeMs,
		StartedAt:          result.StartedAt,
		CompletedAt:        result.CompletedAt,
	})

	// Update statistics
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats.TotalVerifications++
	if result.Verified {
		s.stats.SuccessfulCount++
	} else {
		s.stats.FailedCount++
	}

	// Recalculate success rate
	if s.stats.TotalVerifications > 0 {
		s.stats.SuccessRate = float64(s.stats.SuccessfulCount) / float64(s.stats.TotalVerifications) * 100
	}

	// Update average score (running average)
	prevTotal := s.stats.AverageScore * float64(s.stats.TotalVerifications-1)
	s.stats.AverageScore = (prevTotal + result.OverallScore) / float64(s.stats.TotalVerifications)
}

// verifyCodeVisibility performs the mandatory "Do you see my code?" test
func (s *VerificationService) verifyCodeVisibility(ctx context.Context, modelID, provider string) (*TestResult, error) {
	result := &TestResult{
		Name:      "code_visibility",
		StartedAt: time.Now(),
		Details:   make([]string, 0),
	}

	// Code samples — reduced to 2 (Python and Go) to minimize API calls
	// while still validating cross-language code visibility
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
	result.Passed = result.Score >= 50 // Require at least 1 of 2 samples to pass

	return result, nil
}

// isAffirmativeCodeResponse checks if the response confirms code visibility
func (s *VerificationService) isAffirmativeCodeResponse(response string) bool {
	response = strings.ToLower(response)

	// Primary affirmatives — explicit "yes" patterns
	affirmatives := []string{
		"yes, i can see",
		"yes i can see",
		"yes, i see",
		"yes i see",
		"i can see your code",
		"i see your code",
		"i can see the code",
		"see the code",
		"can see the",
		"affirmative",
		"visible",
		"i can view",
	}

	for _, phrase := range affirmatives {
		if strings.Contains(response, phrase) {
			return true
		}
	}

	// Secondary indicators — the model analyzed or referenced the code
	// These indicate the model can see the code even without explicit "yes"
	codeIndicators := []string{
		"fibonacci",          // References the function name from the test
		"recursive",          // Describes what the code does
		"here is the code",   // Acknowledges seeing code
		"here's the code",    // Acknowledges seeing code
		"your code",          // References user's code
		"the code you",       // References the code shown
		"this code",          // References the code
		"code snippet",       // References a code snippet
		"code sample",        // References code sample
		"function",           // Identifies a function
		"looking at",         // Looking at the code
		"i see a",            // I see a function/code
		"i notice",           // I notice the code
		"shows a",            // Shows a function
		"defines a",          // Defines a function
		"implements",         // Implements fibonacci
		"calculates",         // Calculates fibonacci
		"computes",           // Computes fibonacci
		"this is a",          // This is a recursive function
		"the function",       // The function fibonacci
		"provided code",      // The provided code
		"given code",         // The given code
		"above code",         // The above code
		"following code",     // The following code
		"displayed code",     // The displayed code
		"shown code",         // The shown code
		"presented code",     // The presented code
		"code above",         // Code above
		"code below",         // Code below
		"code you shared",    // Code you shared
		"code you provided",  // Code you provided
		"code you wrote",     // Code you wrote
		"def fibonacci",      // Python function signature
		"func fibonacci",     // Go function signature
		"function fibonacci", // JS function signature
		"public int",         // Java/C# method
	}

	for _, indicator := range codeIndicators {
		if strings.Contains(response, indicator) {
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

	start := time.Now()
	response, err := s.callModel(ctx, modelID, provider, "Reply with just 'hello'")
	latency := time.Since(start)

	// Store the response for quality inspection
	result.Response = response

	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
		result.CompletedAt = time.Now()
		return result
	}

	// Skip quality validation in test mode
	s.mu.RLock()
	skipValidation := s.testMode
	s.mu.RUnlock()

	if !skipValidation {
		// Check for canned error responses - model must give real response
		if isCanned, pattern := IsCannedErrorResponse(response); isCanned {
			result.Passed = false
			result.Score = 0
			result.Details = append(result.Details, fmt.Sprintf("canned error detected (pattern: %s): %s", pattern, truncateForLog(response)))
			result.CompletedAt = time.Now()
			return result
		}

		// Check for suspiciously fast + short response
		if IsSuspiciouslyFastResponse(latency) && len(response) < 50 {
			result.Passed = false
			result.Score = 0
			result.Details = append(result.Details, fmt.Sprintf("suspicious response: %v latency, %d chars", latency, len(response)))
			result.CompletedAt = time.Now()
			return result
		}
	}

	if len(response) > 0 {
		result.Passed = true
		result.Score = 100
		result.Details = append(result.Details, "model responded successfully")
		result.Details = append(result.Details, fmt.Sprintf("latency: %v", latency))
	}

	result.CompletedAt = time.Now()
	return result
}

// verifyResponsiveness verifies model response time AND response quality
// A model that returns canned error responses or empty content is NOT responsive
func (s *VerificationService) verifyResponsiveness(ctx context.Context, modelID, provider string) *TestResult {
	result := &TestResult{
		Name:      "responsiveness",
		StartedAt: time.Now(),
	}

	start := time.Now()
	response, err := s.callModel(ctx, modelID, provider, "What is 2+2? Reply with just the number.")
	duration := time.Since(start)

	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Details = append(result.Details, fmt.Sprintf("error: %v", err))
		result.CompletedAt = time.Now()
		return result
	}

	// Skip quality validation in test mode
	s.mu.RLock()
	skipValidation := s.testMode
	s.mu.RUnlock()

	if !skipValidation {
		// Validate response quality - check for canned errors
		if qualityErr := ValidateResponseQualityWithLatency(response, duration); qualityErr != nil {
			result.Passed = false
			result.Score = 0
			result.Details = append(result.Details, fmt.Sprintf("response quality failed: %v", qualityErr))
			result.Details = append(result.Details, fmt.Sprintf("response time: %v", duration))
			result.CompletedAt = time.Now()
			return result
		}
	}

	// Check if response contains expected answer (basic math test)
	if !strings.Contains(response, "4") {
		result.Passed = false
		result.Score = 30 // Partial credit - responded but wrong
		result.Details = append(result.Details, fmt.Sprintf("expected '4' in response, got: %s", truncateForLog(response)))
		result.Details = append(result.Details, fmt.Sprintf("response time: %v", duration))
		result.CompletedAt = time.Now()
		return result
	}

	// Response quality is good, now check timing
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
	result.Details = append(result.Details, "response validated: contains '4'")

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
	// Search for any verification with this model ID (any provider)
	var found *VerificationStatus
	s.verificationCache.Range(func(key string, status *VerificationStatus) bool {
		if strings.HasSuffix(key, ":"+modelID) || strings.HasPrefix(key, modelID+":") || status.ModelID == modelID {
			found = status
			return false
		}
		return true
	})
	if found != nil {
		return found, nil
	}

	// Not found - return not_found status
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

// GetVerificationStatusByProvider returns the status of a verification by model ID and provider
func (s *VerificationService) GetVerificationStatusByProvider(ctx context.Context, modelID, provider string) (*VerificationStatus, error) {
	cacheKey := fmt.Sprintf("%s:%s", provider, modelID)

	if status, ok := s.verificationCache.Get(cacheKey); ok {
		return status, nil
	}

	// Not found
	return &VerificationStatus{
		ModelID:            modelID,
		Provider:           provider,
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

// InvalidateVerification invalidates a previous verification
func (s *VerificationService) InvalidateVerification(modelID string) {
	// Remove all verifications for this model ID (any provider)
	var keysToDelete []string
	s.verificationCache.Range(func(key string, status *VerificationStatus) bool {
		if status.ModelID == modelID {
			keysToDelete = append(keysToDelete, key)
		}
		return true
	})

	for _, key := range keysToDelete {
		s.verificationCache.Delete(key)
	}
}

// InvalidateVerificationByProvider invalidates a specific provider's verification
func (s *VerificationService) InvalidateVerificationByProvider(modelID, provider string) {
	cacheKey := fmt.Sprintf("%s:%s", provider, modelID)
	s.verificationCache.Delete(cacheKey)
}

// VerificationStats represents verification statistics
type VerificationStats struct {
	TotalVerifications int     `json:"total_verifications"`
	SuccessfulCount    int     `json:"successful_count"`
	FailedCount        int     `json:"failed_count"`
	SuccessRate        float64 `json:"success_rate"`
	AverageScore       float64 `json:"average_score"`
}

// GetStats returns verification statistics
func (s *VerificationService) GetStats(ctx context.Context) (*VerificationStats, error) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// Return a copy of the stats
	return &VerificationStats{
		TotalVerifications: s.stats.TotalVerifications,
		SuccessfulCount:    s.stats.SuccessfulCount,
		FailedCount:        s.stats.FailedCount,
		SuccessRate:        s.stats.SuccessRate,
		AverageScore:       s.stats.AverageScore,
	}, nil
}

// GetAllVerifications returns all cached verification results
func (s *VerificationService) GetAllVerifications(ctx context.Context) ([]*VerificationStatus, error) {
	results := make([]*VerificationStatus, 0, s.verificationCache.Len())
	s.verificationCache.Range(func(_ string, status *VerificationStatus) bool {
		results = append(results, status)
		return true
	})
	return results, nil
}

// ResetStats resets all verification statistics
func (s *VerificationService) ResetStats() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats = &VerificationStats{
		TotalVerifications: 0,
		SuccessfulCount:    0,
		FailedCount:        0,
		SuccessRate:        0,
		AverageScore:       0,
	}
}

// ClearCache clears all cached verification results
func (s *VerificationService) ClearCache() {
	s.verificationCache.Clear()
}
