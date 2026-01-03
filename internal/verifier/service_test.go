package verifier

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewVerificationService(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Verification: VerificationConfig{
			VerificationTimeout: 10 * time.Second,
			RetryCount:          3,
		},
	}
	svc := NewVerificationService(cfg)
	if svc == nil {
		t.Fatal("NewVerificationService returned nil")
	}
	if svc.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestVerificationService_SetProviderFunc(t *testing.T) {
	svc := NewVerificationService(&Config{})

	called := false
	fn := func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		called = true
		return "response", nil
	}

	svc.SetProviderFunc(fn)

	if svc.providerFunc == nil {
		t.Error("providerFunc not set")
	}

	// Verify function is callable
	_, err := svc.providerFunc(context.Background(), "model", "provider", "prompt")
	if err != nil {
		t.Errorf("providerFunc returned error: %v", err)
	}
	if !called {
		t.Error("providerFunc was not called")
	}
}

func TestVerificationService_VerifyModel_NoProviderFunc(t *testing.T) {
	svc := NewVerificationService(&Config{})

	result, err := svc.VerifyModel(context.Background(), "test-model", "openai")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
	if result.ModelID != "test-model" {
		t.Errorf("expected model_id 'test-model', got '%s'", result.ModelID)
	}
}

func TestVerificationService_VerifyModel_Success(t *testing.T) {
	svc := NewVerificationService(&Config{})

	// Mock provider that confirms code visibility
	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	result, err := svc.VerifyModel(context.Background(), "gpt-4", "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ModelID != "gpt-4" {
		t.Errorf("expected model_id 'gpt-4', got '%s'", result.ModelID)
	}
	if result.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", result.Provider)
	}
	if !result.CodeVerified {
		t.Error("expected CodeVerified to be true")
	}
	if !result.CodeVisible {
		t.Error("expected CodeVisible to be true")
	}
	if len(result.Tests) == 0 {
		t.Error("expected tests to be populated")
	}
	if result.VerificationTimeMs < 0 {
		t.Error("expected non-negative verification time")
	}
	if len(result.TestsMap) == 0 {
		t.Error("expected TestsMap to be populated")
	}
}

func TestVerificationService_VerifyModel_ProviderError(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "", errors.New("provider error")
	})

	result, err := svc.VerifyModel(context.Background(), "test-model", "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
	if result.Verified {
		t.Error("expected Verified to be false")
	}
}

func TestVerificationService_VerifyModel_CodeNotVisible(t *testing.T) {
	svc := NewVerificationService(&Config{})

	// Mock provider that doesn't confirm code visibility
	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "I cannot see any code", nil
	})

	result, err := svc.VerifyModel(context.Background(), "test-model", "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CodeVerified {
		t.Error("expected CodeVerified to be false")
	}
	if result.Verified {
		t.Error("expected Verified to be false")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", result.Status)
	}
}

func TestVerificationService_isAffirmativeCodeResponse(t *testing.T) {
	svc := NewVerificationService(&Config{})

	tests := []struct {
		response string
		expected bool
	}{
		{"Yes, I can see your code", true},
		{"yes i can see the code", true},
		{"I see your code above", true},
		{"The code is visible to me", true},
		{"I can view the code", true},
		{"affirmative, I see it", true},
		{"No, I cannot see anything", false},
		{"What code?", false},
		{"I don't understand", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.response, func(t *testing.T) {
			result := svc.isAffirmativeCodeResponse(tt.response)
			if result != tt.expected {
				t.Errorf("isAffirmativeCodeResponse(%q) = %v, want %v", tt.response, result, tt.expected)
			}
		})
	}
}

func TestVerificationService_BatchVerify(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	requests := []*BatchVerificationRequest{
		{ModelID: "model1", Provider: "openai"},
		{ModelID: "model2", Provider: "anthropic"},
		{ModelID: "model3", Provider: "google"},
	}

	results, err := svc.BatchVerify(context.Background(), requests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if result.ModelID != requests[i].ModelID {
			t.Errorf("result[%d] model_id mismatch: expected %s, got %s", i, requests[i].ModelID, result.ModelID)
		}
	}
}

func TestVerificationService_BatchVerify_Empty(t *testing.T) {
	svc := NewVerificationService(&Config{})

	results, err := svc.BatchVerify(context.Background(), []*BatchVerificationRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestVerificationService_GetVerifiedModels(t *testing.T) {
	svc := NewVerificationService(&Config{})

	models, err := svc.GetVerifiedModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if models == nil {
		t.Error("expected non-nil slice")
	}
}

func TestVerificationService_PerformCodeCheck(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	code := "def hello(): print('world')"
	result, err := svc.PerformCodeCheck(context.Background(), "gpt-4", "openai", code, "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "code_check" {
		t.Errorf("expected name 'code_check', got '%s'", result.Name)
	}
	if !result.Passed {
		t.Error("expected Passed to be true")
	}
	if result.Score != 100 {
		t.Errorf("expected score 100, got %f", result.Score)
	}
}

func TestVerificationService_PerformCodeCheck_Failed(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "I don't see any code", nil
	})

	result, err := svc.PerformCodeCheck(context.Background(), "test", "openai", "code", "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("expected Passed to be false")
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %f", result.Score)
	}
}

func TestVerificationService_PerformCodeCheck_Error(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "", errors.New("provider error")
	})

	result, err := svc.PerformCodeCheck(context.Background(), "test", "openai", "code", "python")
	if err == nil {
		t.Error("expected error")
	}
	if result.Passed {
		t.Error("expected Passed to be false on error")
	}
}

func TestVerificationService_GetVerificationStatus(t *testing.T) {
	svc := NewVerificationService(&Config{})

	status, err := svc.GetVerificationStatus(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.ModelID != "test-model" {
		t.Errorf("expected model_id 'test-model', got '%s'", status.ModelID)
	}
}

func TestVerificationService_TestCodeVisibility(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	result, err := svc.TestCodeVisibility(context.Background(), "gpt-4", "openai", "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ModelID != "gpt-4" {
		t.Errorf("expected model_id 'gpt-4', got '%s'", result.ModelID)
	}
	if !result.CodeVisible {
		t.Error("expected CodeVisible to be true")
	}
	if result.Language != "python" {
		t.Errorf("expected language 'python', got '%s'", result.Language)
	}
	if result.Prompt == "" {
		t.Error("expected Prompt to be set")
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestVerificationService_TestCodeVisibility_UnknownLanguage(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	result, err := svc.TestCodeVisibility(context.Background(), "gpt-4", "openai", "unknown-lang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to python
	if result.Language != "python" {
		t.Errorf("expected language 'python' (fallback), got '%s'", result.Language)
	}
}

func TestVerificationService_TestCodeVisibility_Error(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "", errors.New("provider error")
	})

	result, err := svc.TestCodeVisibility(context.Background(), "test", "openai", "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CodeVisible {
		t.Error("expected CodeVisible to be false on error")
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence 0, got %f", result.Confidence)
	}
}

func TestVerificationService_InvalidateVerification(t *testing.T) {
	svc := NewVerificationService(&Config{})

	// Should not panic
	svc.InvalidateVerification("test-model")
}

func TestVerificationService_GetStats(t *testing.T) {
	svc := NewVerificationService(&Config{})

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats == nil {
		t.Fatal("stats is nil")
	}
}

func TestVerificationService_calculateOverallScore(t *testing.T) {
	svc := NewVerificationService(&Config{})

	tests := []struct {
		name     string
		results  []TestResult
		expected float64
	}{
		{
			name:     "empty",
			results:  []TestResult{},
			expected: 0,
		},
		{
			name: "single 100",
			results: []TestResult{
				{Score: 100},
			},
			expected: 100,
		},
		{
			name: "mixed scores",
			results: []TestResult{
				{Score: 100},
				{Score: 50},
				{Score: 75},
			},
			expected: 75, // (100+50+75)/3 = 75
		},
		{
			name: "all zeros",
			results: []TestResult{
				{Score: 0},
				{Score: 0},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.calculateOverallScore(tt.results)
			if result != tt.expected {
				t.Errorf("calculateOverallScore() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestVerificationService_verifyExistence(t *testing.T) {
	svc := NewVerificationService(&Config{})

	t.Run("success", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "OK", nil
		})

		result := svc.verifyExistence(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score != 100 {
			t.Errorf("expected score 100, got %f", result.Score)
		}
	})

	t.Run("error", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "", errors.New("error")
		})

		result := svc.verifyExistence(context.Background(), "model", "provider")
		if result.Passed {
			t.Error("expected Passed to be false")
		}
		if result.Score != 0 {
			t.Errorf("expected score 0, got %f", result.Score)
		}
	})
}

func TestVerificationService_verifyResponsiveness(t *testing.T) {
	svc := NewVerificationService(&Config{})

	t.Run("fast response", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "4", nil
		})

		result := svc.verifyResponsiveness(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score != 100 {
			t.Errorf("expected score 100, got %f", result.Score)
		}
	})

	t.Run("error", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "", errors.New("error")
		})

		result := svc.verifyResponsiveness(context.Background(), "model", "provider")
		if result.Passed {
			t.Error("expected Passed to be false")
		}
	})
}

func TestVerificationService_verifyFunctionCalling(t *testing.T) {
	svc := NewVerificationService(&Config{})

	t.Run("success", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return `{"function": "get_weather", "arguments": {"location": "San Francisco"}}`, nil
		})

		result := svc.verifyFunctionCalling(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score != 100 {
			t.Errorf("expected score 100, got %f", result.Score)
		}
	})

	t.Run("no function call", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "The weather is nice today", nil
		})

		result := svc.verifyFunctionCalling(context.Background(), "model", "provider")
		if result.Passed {
			t.Error("expected Passed to be false")
		}
	})
}

func TestVerificationService_verifyCodingCapability(t *testing.T) {
	svc := NewVerificationService(&Config{})

	t.Run("good code", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return `def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True`, nil
		})

		result := svc.verifyCodingCapability(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score < 80 {
			t.Errorf("expected score >= 80, got %f", result.Score)
		}
	})

	t.Run("bad code", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "I don't know how to write code", nil
		})

		result := svc.verifyCodingCapability(context.Background(), "model", "provider")
		if result.Passed {
			t.Error("expected Passed to be false")
		}
	})
}

func TestVerificationService_verifyErrorDetection(t *testing.T) {
	svc := NewVerificationService(&Config{})

	t.Run("finds bug", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "The bug is that you're using variable 'c' which is undefined. You should use 'b' instead.", nil
		})

		result := svc.verifyErrorDetection(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score != 100 {
			t.Errorf("expected score 100, got %f", result.Score)
		}
	})

	t.Run("partial detection", func(t *testing.T) {
		// Response mentions "error" but doesn't identify the specific bug (using 'c' instead of 'b')
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "There appears to be an error in the return statement", nil
		})

		result := svc.verifyErrorDetection(context.Background(), "model", "provider")
		if !result.Passed {
			t.Error("expected Passed to be true")
		}
		if result.Score != 70 {
			t.Errorf("expected score 70, got %f", result.Score)
		}
	})

	t.Run("no detection", func(t *testing.T) {
		svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
			return "This code looks fine to me", nil
		})

		result := svc.verifyErrorDetection(context.Background(), "model", "provider")
		if result.Passed {
			t.Error("expected Passed to be false")
		}
	})
}

func TestServiceVerificationResult_Fields(t *testing.T) {
	result := &ServiceVerificationResult{
		ModelID:               "test-model",
		Provider:              "openai",
		ProviderName:          "OpenAI",
		Status:                "verified",
		Verified:              true,
		CodeVerified:          true,
		CodeVisible:           true,
		Score:                 95.5,
		OverallScore:          95.5,
		ScoreSuffix:           "(SC:95.5)",
		CodingCapabilityScore: 90.0,
		Tests:                 []TestResult{{Name: "test1", Passed: true, Score: 100}},
		TestsMap:              map[string]bool{"test1": true},
		StartedAt:             time.Now(),
		CompletedAt:           time.Now().Add(time.Second),
		VerificationTimeMs:    1000,
		Message:               "success",
	}

	if result.ModelID != "test-model" {
		t.Error("ModelID mismatch")
	}
	if result.Provider != "openai" {
		t.Error("Provider mismatch")
	}
	if !result.Verified {
		t.Error("Verified should be true")
	}
	if result.OverallScore != 95.5 {
		t.Error("OverallScore mismatch")
	}
	if len(result.Tests) != 1 {
		t.Error("Tests length mismatch")
	}
	if !result.TestsMap["test1"] {
		t.Error("TestsMap value mismatch")
	}
}

func TestTestResult_Fields(t *testing.T) {
	now := time.Now()
	result := TestResult{
		Name:        "code_visibility",
		Passed:      true,
		Score:       100,
		Details:     []string{"detail1", "detail2"},
		StartedAt:   now,
		CompletedAt: now.Add(time.Second),
	}

	if result.Name != "code_visibility" {
		t.Error("Name mismatch")
	}
	if !result.Passed {
		t.Error("Passed should be true")
	}
	if result.Score != 100 {
		t.Error("Score mismatch")
	}
	if len(result.Details) != 2 {
		t.Error("Details length mismatch")
	}
}

func TestBatchVerificationRequest_Fields(t *testing.T) {
	req := BatchVerificationRequest{
		ModelID:  "test-model",
		Provider: "openai",
	}

	if req.ModelID != "test-model" {
		t.Error("ModelID mismatch")
	}
	if req.Provider != "openai" {
		t.Error("Provider mismatch")
	}
}

func TestVerificationStatus_Fields(t *testing.T) {
	now := time.Now()
	status := VerificationStatus{
		ModelID:            "test-model",
		Provider:           "openai",
		Status:             "completed",
		Progress:           100,
		Verified:           true,
		Score:              95.0,
		OverallScore:       95.0,
		ScoreSuffix:        "(SC:95.0)",
		CodeVisible:        true,
		Tests:              map[string]bool{"test1": true},
		VerificationTimeMs: 1000,
		StartedAt:          now,
		CompletedAt:        now.Add(time.Second),
	}

	if status.ModelID != "test-model" {
		t.Error("ModelID mismatch")
	}
	if status.Progress != 100 {
		t.Error("Progress mismatch")
	}
	if !status.Verified {
		t.Error("Verified should be true")
	}
}

func TestCodeVisibilityResult_Fields(t *testing.T) {
	result := CodeVisibilityResult{
		ModelID:     "test-model",
		Provider:    "openai",
		CodeVisible: true,
		Language:    "python",
		Prompt:      "test prompt",
		Response:    "test response",
		Confidence:  0.95,
	}

	if result.ModelID != "test-model" {
		t.Error("ModelID mismatch")
	}
	if !result.CodeVisible {
		t.Error("CodeVisible should be true")
	}
	if result.Language != "python" {
		t.Error("Language mismatch")
	}
	if result.Confidence != 0.95 {
		t.Error("Confidence mismatch")
	}
}

func TestVerificationStats_Fields(t *testing.T) {
	stats := VerificationStats{
		TotalVerifications: 100,
		SuccessfulCount:    80,
		FailedCount:        20,
		SuccessRate:        0.8,
		AverageScore:       85.5,
	}

	if stats.TotalVerifications != 100 {
		t.Error("TotalVerifications mismatch")
	}
	if stats.SuccessfulCount != 80 {
		t.Error("SuccessfulCount mismatch")
	}
	if stats.FailedCount != 20 {
		t.Error("FailedCount mismatch")
	}
	if stats.SuccessRate != 0.8 {
		t.Error("SuccessRate mismatch")
	}
	if stats.AverageScore != 85.5 {
		t.Error("AverageScore mismatch")
	}
}

func TestVerificationService_ContextCancellation(t *testing.T) {
	svc := NewVerificationService(&Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			return "response", nil
		}
	})

	// Should handle cancelled context gracefully
	result, _ := svc.VerifyModel(ctx, "test-model", "openai")
	if result == nil {
		t.Fatal("result should not be nil even with cancelled context")
	}
}

func TestVerificationService_ConcurrentAccess(t *testing.T) {
	svc := NewVerificationService(&Config{})

	svc.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code", nil
	})

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			_, _ = svc.VerifyModel(context.Background(), "model", "provider")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
