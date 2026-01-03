package verifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func TestVerificationService_VerifyModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		modelID       string
		provider      string
		mockResponse  string
		expectVerified bool
		expectError   bool
	}{
		{
			name:          "successful verification with code visible",
			modelID:       "gpt-4",
			provider:      "openai",
			mockResponse:  "Yes, I can see your Python code. It defines a function called 'calculate_sum'.",
			expectVerified: true,
			expectError:   false,
		},
		{
			name:          "verification with code not visible",
			modelID:       "gpt-3.5-turbo",
			provider:      "openai",
			mockResponse:  "I don't see any code in our conversation.",
			expectVerified: true,
			expectError:   false,
		},
		{
			name:          "empty model ID",
			modelID:       "",
			provider:      "openai",
			expectVerified: false,
			expectError:   true,
		},
		{
			name:          "empty provider",
			modelID:       "gpt-4",
			provider:      "",
			expectVerified: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := verifier.DefaultConfig()
			service, err := verifier.NewVerificationService(nil, cfg)
			require.NoError(t, err)

			// Set up mock provider function
			service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
				return tt.mockResponse, nil
			})

			ctx := context.Background()
			result, err := service.VerifyModel(ctx, tt.modelID, tt.provider)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.modelID, result.ModelID)
			assert.Equal(t, tt.provider, result.Provider)
		})
	}
}

func TestVerificationService_BatchVerify(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewVerificationService(nil, cfg)
	require.NoError(t, err)

	service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		return "Yes, I can see your code.", nil
	})

	requests := []*verifier.BatchVerificationRequest{
		{ModelID: "gpt-4", Provider: "openai"},
		{ModelID: "claude-3", Provider: "anthropic"},
		{ModelID: "gemini-pro", Provider: "google"},
	}

	ctx := context.Background()
	results, err := service.BatchVerify(ctx, requests)

	require.NoError(t, err)
	assert.Len(t, results, 3)

	for _, result := range results {
		assert.NotEmpty(t, result.ModelID)
		assert.NotEmpty(t, result.Provider)
	}
}

func TestVerificationService_CodeVisibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		response       string
		expectVisible  bool
	}{
		{
			name:          "explicit yes",
			response:      "Yes, I can see your Python code that defines a function.",
			expectVisible: true,
		},
		{
			name:          "sees code mention",
			response:      "I see you have a function named calculate_sum in your code.",
			expectVisible: true,
		},
		{
			name:          "no code visible",
			response:      "I don't see any code in this conversation.",
			expectVisible: false,
		},
		{
			name:          "ambiguous response",
			response:      "I'm not sure what you're referring to.",
			expectVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := verifier.DefaultConfig()
			service, err := verifier.NewVerificationService(nil, cfg)
			require.NoError(t, err)

			service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
				return tt.response, nil
			})

			ctx := context.Background()
			result, err := service.TestCodeVisibility(ctx, "gpt-4", "openai", "python")

			require.NoError(t, err)
			assert.Equal(t, tt.expectVisible, result.CodeVisible)
		})
	}
}

func TestVerificationService_Stats(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewVerificationService(nil, cfg)
	require.NoError(t, err)

	stats := service.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalProviders, 0)
}

func TestVerificationService_InvalidateVerification(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewVerificationService(nil, cfg)
	require.NoError(t, err)

	// Should not panic even for non-existent model
	service.InvalidateVerification("non-existent-model")
}

func TestVerificationService_Concurrency(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewVerificationService(nil, cfg)
	require.NoError(t, err)

	service.SetProviderFunc(func(ctx context.Context, modelID, provider, prompt string) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "Yes, I see your code.", nil
	})

	ctx := context.Background()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := service.VerifyModel(ctx, "gpt-4", "openai")
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
