package llm

import (
	"github.com/superagent/superagent/internal/models"
	"testing"
)

func TestProviderCapabilities_DefaultValues(t *testing.T) {
	cap := &models.ProviderCapabilities{}

	if cap.SupportedModels != nil {
		t.Errorf("SupportedModels should be nil, got %v", cap.SupportedModels)
	}
	if cap.SupportedFeatures != nil {
		t.Errorf("SupportedFeatures should be nil, got %v", cap.SupportedFeatures)
	}
	if cap.SupportedRequestTypes != nil {
		t.Errorf("SupportedRequestTypes should be nil, got %v", cap.SupportedRequestTypes)
	}
	if cap.SupportsStreaming != false {
		t.Errorf("SupportsStreaming should be false, got %v", cap.SupportsStreaming)
	}
	if cap.SupportsFunctionCalling != false {
		t.Errorf("SupportsFunctionCalling should be false, got %v", cap.SupportsFunctionCalling)
	}
	if cap.SupportsVision != false {
		t.Errorf("SupportsVision should be false, got %v", cap.SupportsVision)
	}
	if cap.SupportsTools != false {
		t.Errorf("SupportsTools should be false, got %v", cap.SupportsTools)
	}
	if cap.SupportsSearch != false {
		t.Errorf("SupportsSearch should be false, got %v", cap.SupportsSearch)
	}
	if cap.SupportsReasoning != false {
		t.Errorf("SupportsReasoning should be false, got %v", cap.SupportsReasoning)
	}
	if cap.SupportsCodeCompletion != false {
		t.Errorf("SupportsCodeCompletion should be false, got %v", cap.SupportsCodeCompletion)
	}
	if cap.SupportsCodeAnalysis != false {
		t.Errorf("SupportsCodeAnalysis should be false, got %v", cap.SupportsCodeAnalysis)
	}
	if cap.SupportsRefactoring != false {
		t.Errorf("SupportsRefactoring should be false, got %v", cap.SupportsRefactoring)
	}
	if cap.Limits.MaxTokens != 0 {
		t.Errorf("Limits.MaxTokens should be 0, got %v", cap.Limits.MaxTokens)
	}
	if cap.Limits.MaxInputLength != 0 {
		t.Errorf("Limits.MaxInputLength should be 0, got %v", cap.Limits.MaxInputLength)
	}
	if cap.Limits.MaxOutputLength != 0 {
		t.Errorf("Limits.MaxOutputLength should be 0, got %v", cap.Limits.MaxOutputLength)
	}
	if cap.Limits.MaxConcurrentRequests != 0 {
		t.Errorf("Limits.MaxConcurrentRequests should be 0, got %v", cap.Limits.MaxConcurrentRequests)
	}
	if cap.Metadata != nil {
		t.Errorf("Metadata should be nil, got %v", cap.Metadata)
	}
}

func TestModelLimits_DefaultValues(t *testing.T) {
	limits := &models.ModelLimits{}

	if limits.MaxTokens != 0 {
		t.Errorf("MaxTokens should be 0, got %v", limits.MaxTokens)
	}
	if limits.MaxInputLength != 0 {
		t.Errorf("MaxInputLength should be 0, got %v", limits.MaxInputLength)
	}
	if limits.MaxOutputLength != 0 {
		t.Errorf("MaxOutputLength should be 0, got %v", limits.MaxOutputLength)
	}
	if limits.MaxConcurrentRequests != 0 {
		t.Errorf("MaxConcurrentRequests should be 0, got %v", limits.MaxConcurrentRequests)
	}
}

func TestProviderCapabilities_Validation(t *testing.T) {
	tests := []struct {
		name string
		cap  *models.ProviderCapabilities
		want bool
	}{
		{
			name: "Valid capabilities",
			cap: &models.ProviderCapabilities{
				SupportedModels:         []string{"gpt-4"},
				SupportedFeatures:       []string{"chat"},
				SupportedRequestTypes:   []string{"text_completion"},
				SupportsStreaming:       true,
				SupportsFunctionCalling: true,
				SupportsVision:          false,
				SupportsTools:           true,
				SupportsSearch:          false,
				SupportsReasoning:       true,
				SupportsCodeCompletion:  true,
				SupportsCodeAnalysis:    true,
				SupportsRefactoring:     false,
				Limits: models.ModelLimits{
					MaxTokens:             4096,
					MaxInputLength:        8192,
					MaxOutputLength:       4096,
					MaxConcurrentRequests: 10,
				},
				Metadata: map[string]string{
					"provider": "test",
				},
			},
			want: true,
		},
		{
			name: "Empty capabilities",
			cap: &models.ProviderCapabilities{
				SupportedModels:       []string{},
				SupportedFeatures:     []string{},
				SupportedRequestTypes: []string{},
			},
			want: true, // Empty capabilities are technically valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For now, we just test that the struct can be created
			// In a real implementation, you might have validation logic
			if tt.cap.SupportedModels == nil && tt.want {
				t.Errorf("Expected supported models to be initialized")
			}
		})
	}
}
