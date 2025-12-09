package fixtures

import (
	"time"

	"github.com/superagent/superagent/internal/models"
)

// MockProviders returns a list of mock LLM providers for testing
func MockProviders() []*models.LLMProvider {
	return []*models.LLMProvider{
		{
			ID:           "deepseek-provider-1",
			Name:         "DeepSeek",
			Type:         "api",
			APIKey:       "sk-test-deepseek-key",
			BaseURL:      "https://api.deepseek.com",
			Model:        "deepseek-coder",
			Weight:       1.0,
			Enabled:      true,
			Config:       map[string]any{},
			HealthStatus: "healthy",
			ResponseTime: 800,
			CreatedAt:    time.Now().Add(-24 * time.Hour),
			UpdatedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:           "claude-provider-1",
			Name:         "Claude",
			Type:         "api",
			APIKey:       "sk-ant-test-claude-key",
			BaseURL:      "https://api.anthropic.com",
			Model:        "claude-3-sonnet-20240229",
			Weight:       0.9,
			Enabled:      true,
			Config:       map[string]any{},
			HealthStatus: "healthy",
			ResponseTime: 600,
			CreatedAt:    time.Now().Add(-24 * time.Hour),
			UpdatedAt:    time.Now().Add(-30 * time.Minute),
		},
		{
			ID:           "gemini-provider-1",
			Name:         "Gemini",
			Type:         "api",
			APIKey:       "test-gemini-key",
			BaseURL:      "https://generativelanguage.googleapis.com",
			Model:        "gemini-pro",
			Weight:       0.8,
			Enabled:      false, // Disabled for testing
			Config:       map[string]any{},
			HealthStatus: "unhealthy",
			ResponseTime: 1200,
			CreatedAt:    time.Now().Add(-48 * time.Hour),
			UpdatedAt:    time.Now().Add(-2 * time.Hour),
		},
	}
}

// MockLLMRequests returns a list of mock LLM requests for testing
func MockLLMRequests() []*models.LLMRequest {
	return []*models.LLMRequest{
		{
			ID:          "req-code-generation-1",
			SessionID:   "session-123",
			UserID:      "user-456",
			Prompt:      "Write a Go function that validates email addresses",
			Messages:    []models.Message{},
			ModelParams: MockModelParameters("code_generation"),
			EnsembleConfig: &models.EnsembleConfig{
				Strategy:            "confidence_weighted",
				MinProviders:        2,
				ConfidenceThreshold: 0.8,
				FallbackToBest:      true,
				Timeout:             30,
				PreferredProviders:  []string{"deepseek", "claude"},
			},
			MemoryEnhanced: false,
			Memory:         map[string]string{},
			Status:         "completed",
			CreatedAt:      time.Now().Add(-10 * time.Minute),
			RequestType:    "code_generation",
		},
		{
			ID:          "req-reasoning-1",
			SessionID:   "session-123",
			UserID:      "user-456",
			Prompt:      "Explain the concept of recursion in programming",
			Messages:    []models.Message{},
			ModelParams: MockModelParameters("reasoning"),
			EnsembleConfig: &models.EnsembleConfig{
				Strategy:            "majority_vote",
				MinProviders:        3,
				ConfidenceThreshold: 0.7,
				FallbackToBest:      false,
				Timeout:             20,
				PreferredProviders:  []string{"claude", "gemini"},
			},
			MemoryEnhanced: true,
			Memory: map[string]string{
				"previous_context": "User is learning programming concepts",
			},
			Status:      "in_progress",
			CreatedAt:   time.Now().Add(-2 * time.Minute),
			RequestType: "reasoning",
		},
		{
			ID:          "req-tool-use-1",
			SessionID:   "session-789",
			UserID:      "user-789",
			Prompt:      "Create a REST API endpoint for user management",
			Messages:    []models.Message{},
			ModelParams: MockModelParameters("tool_use"),
			EnsembleConfig: &models.EnsembleConfig{
				Strategy:            "confidence_weighted",
				MinProviders:        2,
				ConfidenceThreshold: 0.9,
				FallbackToBest:      true,
				Timeout:             45,
				PreferredProviders:  []string{"deepseek"},
			},
			MemoryEnhanced: false,
			Memory:         map[string]string{},
			Status:         "pending",
			CreatedAt:      time.Now(),
			RequestType:    "tool_use",
		},
	}
}

// MockLLMResponses returns a list of mock LLM responses for testing
func MockLLMResponses() []*models.LLMResponse {
	return []*models.LLMResponse{
		{
			ID:           "resp-code-generation-1",
			RequestID:    "req-code-generation-1",
			ProviderID:   "deepseek-provider-1",
			ProviderName: "DeepSeek",
			Content: `func validateEmail(email string) bool {
    pattern := ` + "`" + `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$` + "`" + `
    matched, _ := regexp.MatchString(pattern, email)
    return matched
}`,
			Confidence:     0.95,
			TokensUsed:     120,
			ResponseTime:   800,
			FinishReason:   "stop",
			Metadata:       map[string]any{},
			Selected:       true,
			SelectionScore: 0.95,
			CreatedAt:      time.Now().Add(-8 * time.Minute),
		},
		{
			ID:             "resp-reasoning-1",
			RequestID:      "req-reasoning-1",
			ProviderID:     "claude-provider-1",
			ProviderName:   "Claude",
			Content:        "Recursion is a programming concept where a function calls itself to solve a problem by breaking it down into smaller, similar subproblems...",
			Confidence:     0.88,
			TokensUsed:     250,
			ResponseTime:   600,
			FinishReason:   "stop",
			Metadata:       map[string]any{},
			Selected:       true,
			SelectionScore: 0.88,
			CreatedAt:      time.Now().Add(-1 * time.Minute),
		},
		{
			ID:             "resp-tool-use-1",
			RequestID:      "req-tool-use-1",
			ProviderID:     "deepseek-provider-1",
			ProviderName:   "DeepSeek",
			Content:        "I'll help you create a REST API endpoint for user management with proper error handling and validation...",
			Confidence:     0.92,
			TokensUsed:     450,
			ResponseTime:   1200,
			FinishReason:   "stop",
			Metadata:       map[string]any{},
			Selected:       false,
			SelectionScore: 0.85,
			CreatedAt:      time.Now().Add(-30 * time.Second),
		},
	}
}

// MockModelParameters returns model parameters optimized for the given request type
func MockModelParameters(requestType string) models.ModelParameters {
	switch requestType {
	case "code_generation":
		return models.ModelParameters{
			Model:            "deepseek-coder",
			Temperature:      0.1, // Lower temperature for code
			MaxTokens:        2000,
			TopP:             0.95,
			StopSequences:    []string{"```", "END"},
			ProviderSpecific: map[string]any{},
		}
	case "reasoning":
		return models.ModelParameters{
			Model:            "claude-3-sonnet-20240229",
			Temperature:      0.7, // Higher temperature for reasoning
			MaxTokens:        1500,
			TopP:             1.0,
			StopSequences:    []string{},
			ProviderSpecific: map[string]any{},
		}
	case "tool_use":
		return models.ModelParameters{
			Model:            "gemini-pro",
			Temperature:      0.3,
			MaxTokens:        3000,
			TopP:             0.9,
			StopSequences:    []string{},
			ProviderSpecific: map[string]any{},
		}
	default:
		return models.ModelParameters{
			Model:            "default-model",
			Temperature:      0.7,
			MaxTokens:        1000,
			TopP:             1.0,
			StopSequences:    []string{},
			ProviderSpecific: map[string]any{},
		}
	}
}

// MockUserSessions returns a list of mock user sessions for testing
func MockUserSessions() []*models.UserSession {
	return []*models.UserSession{
		{
			ID:           "session-123",
			UserID:       "user-456",
			SessionToken: "token-abc123def456",
			Context: map[string]any{
				"preferred_language": "go",
				"experience_level":   "intermediate",
				"project_type":       "web-api",
			},
			MemoryID:     stringPtr("memory-789"),
			Status:       "active",
			RequestCount: 15,
			LastActivity: time.Now().Add(-5 * time.Minute),
			ExpiresAt:    time.Now().Add(23 * time.Hour),
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:           "session-789",
			UserID:       "user-789",
			SessionToken: "token-xyz789abc123",
			Context: map[string]any{
				"preferred_language": "python",
				"experience_level":   "beginner",
				"project_type":       "data-analysis",
			},
			MemoryID:     nil,
			Status:       "active",
			RequestCount: 3,
			LastActivity: time.Now().Add(-30 * time.Second),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now().Add(-10 * time.Minute),
		},
		{
			ID:           "session-expired",
			UserID:       "user-111",
			SessionToken: "token-expired123",
			Context:      map[string]any{},
			MemoryID:     nil,
			Status:       "expired",
			RequestCount: 50,
			LastActivity: time.Now().Add(-25 * time.Hour),
			ExpiresAt:    time.Now().Add(-1 * time.Hour),
			CreatedAt:    time.Now().Add(-48 * time.Hour),
		},
	}
}

// MockProviderCapabilities returns mock provider capabilities for testing
func MockProviderCapabilities() map[string]*models.ProviderCapabilities {
	return map[string]*models.ProviderCapabilities{
		"deepseek": {
			SupportedModels:         []string{"deepseek-coder", "deepseek-chat"},
			SupportedFeatures:       []string{"streaming", "function_calling"},
			SupportedRequestTypes:   []string{"code_generation", "reasoning", "tool_use"},
			SupportsStreaming:       true,
			SupportsFunctionCalling: true,
			SupportsVision:          false,
			Limits: models.ModelLimits{
				MaxTokens:             4096,
				MaxInputLength:        3200,
				MaxOutputLength:       2048,
				MaxConcurrentRequests: 20,
			},
			Metadata: map[string]string{
				"provider":    "DeepSeek",
				"specialty":   "Code Generation",
				"cost_per_1k": "0.002",
			},
		},
		"claude": {
			SupportedModels:         []string{"claude-3-sonnet-20240229", "claude-3-opus-20240229"},
			SupportedFeatures:       []string{"streaming", "function_calling", "vision"},
			SupportedRequestTypes:   []string{"reasoning", "tool_use", "analysis"},
			SupportsStreaming:       true,
			SupportsFunctionCalling: true,
			SupportsVision:          true,
			Limits: models.ModelLimits{
				MaxTokens:             200000,
				MaxInputLength:        100000,
				MaxOutputLength:       4096,
				MaxConcurrentRequests: 10,
			},
			Metadata: map[string]string{
				"provider":    "Anthropic",
				"specialty":   "Reasoning & Analysis",
				"cost_per_1k": "0.015",
			},
		},
		"gemini": {
			SupportedModels:         []string{"gemini-pro", "gemini-pro-vision"},
			SupportedFeatures:       []string{"streaming", "function_calling", "vision"},
			SupportedRequestTypes:   []string{"reasoning", "tool_use", "multimodal"},
			SupportsStreaming:       true,
			SupportsFunctionCalling: true,
			SupportsVision:          true,
			Limits: models.ModelLimits{
				MaxTokens:             32768,
				MaxInputLength:        16384,
				MaxOutputLength:       8192,
				MaxConcurrentRequests: 15,
			},
			Metadata: map[string]string{
				"provider":    "Google",
				"specialty":   "Multimodal Capabilities",
				"cost_per_1k": "0.0005",
			},
		},
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
