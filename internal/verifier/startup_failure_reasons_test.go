// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFailureReason_CodeVisibilityFailed(t *testing.T) {
	t.Parallel()
	result := &ServiceVerificationResult{
		Verified:     false,
		CodeVisible:  false,
		OverallScore: 45.0,
		ErrorMessage: "",
		Tests: []TestResult{
			{Name: "basic_completion", Passed: true, Score: 80},
			{Name: "code_visibility", Passed: false, Score: 0},
			{Name: "streaming", Passed: true, Score: 70},
		},
	}

	reason := buildFailureReason(result)
	assert.Contains(t, reason, "code visibility test failed")
	assert.Contains(t, reason, "2/3 tests passed")
	assert.Contains(t, reason, "code_visibility")
}

func TestBuildFailureReason_ScoreBelowThreshold(t *testing.T) {
	t.Parallel()
	result := &ServiceVerificationResult{
		Verified:     false,
		CodeVisible:  true,
		OverallScore: 30.0,
		ErrorMessage: "score below minimum threshold",
		Tests: []TestResult{
			{Name: "basic_completion", Passed: true, Score: 30},
			{Name: "code_visibility", Passed: false, Score: 0},
		},
	}

	reason := buildFailureReason(result)
	assert.Contains(t, reason, "score below minimum threshold")
	assert.Contains(t, reason, "1/2 tests passed")
}

func TestBuildFailureReason_APIError(t *testing.T) {
	t.Parallel()
	result := &ServiceVerificationResult{
		Verified:     false,
		ErrorMessage: "connection refused: api.provider.com:443",
		Tests:        []TestResult{},
	}

	reason := buildFailureReason(result)
	assert.Contains(t, reason, "connection refused")
}

func TestBuildFailureReason_EmptyResponse(t *testing.T) {
	t.Parallel()
	result := &ServiceVerificationResult{
		Verified:     false,
		CodeVisible:  true,
		OverallScore: 0,
		LastResponse: "",
		Tests: []TestResult{
			{Name: "basic_completion", Passed: false, Score: 0},
		},
	}

	reason := buildFailureReason(result)
	assert.Contains(t, reason, "empty response")
}

func TestBuildFailureReason_NilResult(t *testing.T) {
	t.Parallel()
	reason := buildFailureReason(nil)
	assert.Equal(t, "verification returned nil result", reason)
}

func TestCategorizeFailure_AllCategories(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *ServiceVerificationResult
		expected string
	}{
		{
			name:     "nil result",
			result:   nil,
			expected: FailureCategoryAPIError,
		},
		{
			name: "empty response",
			result: &ServiceVerificationResult{
				LastResponse: "",
				Tests:        []TestResult{{Name: "test"}},
			},
			expected: FailureCategoryEmptyResponse,
		},
		{
			name: "canned response",
			result: &ServiceVerificationResult{
				LastResponse: "Unable to provide analysis at this time",
			},
			expected: FailureCategoryCannedResponse,
		},
		{
			name: "timeout error",
			result: &ServiceVerificationResult{
				ErrorMessage: "context deadline exceeded: timeout waiting for response",
				LastResponse: "partial",
			},
			expected: FailureCategoryTimeout,
		},
		{
			name: "auth error 401",
			result: &ServiceVerificationResult{
				ErrorMessage: "API returned 401 unauthorized",
				LastResponse: "error",
			},
			expected: FailureCategoryAuthError,
		},
		{
			name: "auth error forbidden",
			result: &ServiceVerificationResult{
				ErrorMessage: "403 Forbidden: invalid API key",
				LastResponse: "error",
			},
			expected: FailureCategoryAuthError,
		},
		{
			name: "code visibility failed",
			result: &ServiceVerificationResult{
				CodeVisible:  false,
				OverallScore: 60.0,
				LastResponse: "some valid response",
			},
			expected: FailureCategoryCodeVisibility,
		},
		{
			name: "score below threshold",
			result: &ServiceVerificationResult{
				CodeVisible:  true,
				OverallScore: 30.0,
				LastResponse: "some valid response",
			},
			expected: FailureCategoryScoreBelow,
		},
		{
			name: "generic api error",
			result: &ServiceVerificationResult{
				CodeVisible:  true,
				OverallScore: 0,
				ErrorMessage: "server returned 500",
				LastResponse: "Here is a normal but low-quality response with some content",
			},
			expected: FailureCategoryAPIError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			category := categorizeFailure(tc.result)
			assert.Equal(t, tc.expected, category)
		})
	}
}

func TestMapTestDetails(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []TestResult{
		{
			Name:        "basic_completion",
			Passed:      true,
			Score:       85.0,
			Details:     []string{"Response received", "Content valid"},
			StartedAt:   now,
			CompletedAt: now.Add(150 * time.Millisecond),
		},
		{
			Name:        "code_visibility",
			Passed:      false,
			Score:       0,
			Details:     []string{"Code block not found in response"},
			StartedAt:   now.Add(200 * time.Millisecond),
			CompletedAt: now.Add(350 * time.Millisecond),
		},
	}

	details := mapTestDetails(tests)
	require.Len(t, details, 2)

	assert.Equal(t, "basic_completion", details[0].Name)
	assert.True(t, details[0].Passed)
	assert.Equal(t, 85.0, details[0].Score)
	assert.Equal(t, []string{"Response received", "Content valid"}, details[0].Details)
	assert.Equal(t, int64(150), details[0].DurationMs)

	assert.Equal(t, "code_visibility", details[1].Name)
	assert.False(t, details[1].Passed)
	assert.Equal(t, 0.0, details[1].Score)
	assert.Equal(t, int64(150), details[1].DurationMs)
}

func TestMapTestDetails_Empty(t *testing.T) {
	t.Parallel()
	details := mapTestDetails(nil)
	assert.Nil(t, details)

	details = mapTestDetails([]TestResult{})
	assert.Nil(t, details)
}

func TestProviderTestDetail_Serialization(t *testing.T) {
	t.Parallel()
	detail := ProviderTestDetail{
		Name:       "code_visibility",
		Passed:     false,
		Score:      42.5,
		Details:    []string{"Code block missing", "Expected Go code"},
		DurationMs: 250,
	}

	data, err := json.Marshal(detail)
	require.NoError(t, err)

	var decoded ProviderTestDetail
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, detail.Name, decoded.Name)
	assert.Equal(t, detail.Passed, decoded.Passed)
	assert.Equal(t, detail.Score, decoded.Score)
	assert.Equal(t, detail.Details, decoded.Details)
	assert.Equal(t, detail.DurationMs, decoded.DurationMs)
}

func TestUnifiedProvider_FailureFields_JSON(t *testing.T) {
	t.Parallel()
	provider := UnifiedProvider{
		ID:              "test-provider",
		Name:            "test",
		Verified:        false,
		FailureReason:   "code visibility test failed. 2/3 tests passed (score: 45.0)",
		FailureCategory: FailureCategoryCodeVisibility,
		TestDetails: []ProviderTestDetail{
			{Name: "basic", Passed: true, Score: 80, DurationMs: 100},
			{Name: "code_vis", Passed: false, Score: 0, DurationMs: 200},
		},
		VerificationMsg:   "Verification complete with warnings",
		LastModelResponse: "Some truncated response...",
	}

	data, err := json.Marshal(provider)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "code visibility test failed. 2/3 tests passed (score: 45.0)", decoded["failure_reason"])
	assert.Equal(t, FailureCategoryCodeVisibility, decoded["failure_category"])
	assert.Equal(t, "Verification complete with warnings", decoded["verification_message"])
	assert.Equal(t, "Some truncated response...", decoded["last_model_response"])

	testDetails, ok := decoded["test_details"].([]interface{})
	require.True(t, ok)
	assert.Len(t, testDetails, 2)
}

func TestUnifiedProvider_FailureFields_OmitEmpty(t *testing.T) {
	t.Parallel()
	// Verified provider should omit failure fields
	provider := UnifiedProvider{
		ID:       "verified-provider",
		Name:     "verified",
		Verified: true,
	}

	data, err := json.Marshal(provider)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	_, hasFailureReason := decoded["failure_reason"]
	_, hasFailureCategory := decoded["failure_category"]
	_, hasTestDetails := decoded["test_details"]
	_, hasVerificationMsg := decoded["verification_message"]
	_, hasLastModelResponse := decoded["last_model_response"]

	assert.False(t, hasFailureReason, "failure_reason should be omitted for verified provider")
	assert.False(t, hasFailureCategory, "failure_category should be omitted for verified provider")
	assert.False(t, hasTestDetails, "test_details should be omitted for verified provider")
	assert.False(t, hasVerificationMsg, "verification_message should be omitted for verified provider")
	assert.False(t, hasLastModelResponse, "last_model_response should be omitted for verified provider")
}

func TestTruncateString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string that should be truncated", 20, "this is a longer ..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 10, ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
				t.Parallel()
			result := truncateString(tc.input, tc.maxLen)
			assert.Equal(t, tc.expected, result)
			assert.LessOrEqual(t, len(result), tc.maxLen)
		})
	}
}

func TestPopulateFailureDetails_VerifiedProvider(t *testing.T) {
	t.Parallel()
	provider := &UnifiedProvider{Verified: true}
	result := &ServiceVerificationResult{
		Message:      "All tests passed",
		LastResponse: "valid code response",
		Tests: []TestResult{
			{Name: "test1", Passed: true, Score: 90},
		},
	}

	populateFailureDetails(provider, result)

	// Should populate test details and message even for verified providers
	assert.Len(t, provider.TestDetails, 1)
	assert.Equal(t, "All tests passed", provider.VerificationMsg)
	assert.Equal(t, "valid code response", provider.LastModelResponse)
	// But should NOT populate failure reason/category
	assert.Empty(t, provider.FailureReason)
	assert.Empty(t, provider.FailureCategory)
}

func TestPopulateFailureDetails_FailedProvider(t *testing.T) {
	t.Parallel()
	provider := &UnifiedProvider{Verified: false}
	result := &ServiceVerificationResult{
		Verified:     false,
		CodeVisible:  false,
		OverallScore: 45.0,
		Message:      "Verification failed",
		ErrorMessage: "",
		LastResponse: "Unable to provide analysis at this time",
		Tests: []TestResult{
			{Name: "basic", Passed: true, Score: 80},
			{Name: "code_vis", Passed: false, Score: 0},
		},
	}

	populateFailureDetails(provider, result)

	assert.NotEmpty(t, provider.FailureReason)
	assert.NotEmpty(t, provider.FailureCategory)
	assert.Len(t, provider.TestDetails, 2)
	assert.Contains(t, provider.FailureReason, "code visibility test failed")
}

func BenchmarkBuildFailureReason(b *testing.B) {
	result := &ServiceVerificationResult{
		Verified:     false,
		CodeVisible:  false,
		OverallScore: 45.0,
		ErrorMessage: "code visibility check failed",
		LastResponse: "Unable to provide analysis at this time. Please try again later.",
		Tests: []TestResult{
			{Name: "basic_completion", Passed: true, Score: 80},
			{Name: "code_visibility", Passed: false, Score: 0, Details: []string{"No code block found"}},
			{Name: "streaming", Passed: true, Score: 70},
			{Name: "tool_use", Passed: false, Score: 0, Details: []string{"Tools not supported"}},
			{Name: "context_window", Passed: true, Score: 90},
			{Name: "response_quality", Passed: true, Score: 75},
			{Name: "latency", Passed: true, Score: 85},
			{Name: "consistency", Passed: false, Score: 20},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildFailureReason(result)
	}
}
