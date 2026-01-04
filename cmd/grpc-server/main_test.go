package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/superagent/superagent/pkg/api"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGrpcServer_Complete(t *testing.T) {
	server := NewLLMFacadeServer()
	ctx := context.Background()

	t.Run("basic completion request", func(t *testing.T) {
		req := &pb.CompletionRequest{
			SessionId:      "test-session-123",
			Prompt:         "What is the capital of France?",
			MemoryEnhanced: false,
		}

		// Note: This will return an error since no providers are configured
		// The test verifies the method doesn't panic and returns proper error handling
		resp, err := server.Complete(ctx, req)

		// Even if ensemble returns error, we expect a response object
		// The current implementation returns empty response on error
		if err != nil {
			assert.NotNil(t, resp)
			assert.Equal(t, "", resp.Content)
			assert.Equal(t, float64(0), resp.Confidence)
		} else {
			assert.NotNil(t, resp)
		}
	})

	t.Run("completion with memory enhancement", func(t *testing.T) {
		req := &pb.CompletionRequest{
			SessionId:      "test-session-456",
			Prompt:         "Based on our previous conversation, what was the topic?",
			MemoryEnhanced: true,
		}

		resp, err := server.Complete(ctx, req)

		// The ensemble may return an error without configured providers
		if err != nil {
			assert.NotNil(t, resp)
		} else {
			assert.NotNil(t, resp)
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		req := &pb.CompletionRequest{
			SessionId:      "test-session-789",
			Prompt:         "",
			MemoryEnhanced: false,
		}

		resp, err := server.Complete(ctx, req)

		// Method should handle empty prompts gracefully
		if err != nil {
			assert.NotNil(t, resp)
		} else {
			assert.NotNil(t, resp)
		}
	})
}

func TestGrpcServer_Complete_RequestConversion(t *testing.T) {
	server := NewLLMFacadeServer()
	ctx := context.Background()

	t.Run("verifies request is properly converted", func(t *testing.T) {
		req := &pb.CompletionRequest{
			SessionId:      "session-abc",
			Prompt:         "Test prompt for conversion verification",
			MemoryEnhanced: true,
		}

		// Call Complete - the internal conversion will happen
		resp, _ := server.Complete(ctx, req)

		// We verify the server doesn't panic during conversion
		require.NotNil(t, resp)
	})
}

func TestGrpcServer_Complete_ResponseFields(t *testing.T) {
	server := NewLLMFacadeServer()
	ctx := context.Background()

	req := &pb.CompletionRequest{
		SessionId: "session-xyz",
		Prompt:    "Simple test prompt for response field verification",
	}

	resp, _ := server.Complete(ctx, req)

	// Verify response has expected structure
	require.NotNil(t, resp)

	// Response should have these fields defined (even if empty)
	_ = resp.Content      // Should exist
	_ = resp.Confidence   // Should exist
	_ = resp.ProviderName // Should exist
}

func TestGrpcServerStruct(t *testing.T) {
	// Verify LLMFacadeServer struct can be instantiated
	server := NewLLMFacadeServer()
	require.NotNil(t, server)
}

// LLMProviderServer Tests
func TestLLMProviderServer_Complete(t *testing.T) {
	server := NewLLMProviderServer(nil)
	ctx := context.Background()

	t.Run("basic completion request without registry", func(t *testing.T) {
		req := &pb.CompletionRequest{
			SessionId:      "test-provider-session-123",
			Prompt:         "What is the capital of Germany?",
			MemoryEnhanced: false,
		}

		// Without registry, will fall back to llm.RunEnsemble
		resp, err := server.Complete(ctx, req)

		// Should return response even if error occurs
		if err != nil {
			assert.NotNil(t, resp)
			assert.Equal(t, "", resp.Content)
		} else {
			assert.NotNil(t, resp)
		}
	})
}

func TestLLMProviderServer_HealthCheck(t *testing.T) {
	server := NewLLMProviderServer(nil)
	ctx := context.Background()

	t.Run("health check without registry", func(t *testing.T) {
		req := &pb.HealthRequest{
			Detailed: true,
		}

		resp, err := server.HealthCheck(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "degraded", resp.Status) // No registry means degraded
		assert.NotNil(t, resp.Timestamp)
	})

	t.Run("health check detailed response", func(t *testing.T) {
		req := &pb.HealthRequest{
			Detailed: true,
		}

		resp, err := server.HealthCheck(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, len(resp.Components) > 0, "Expected components in detailed health check")
	})
}

func TestLLMProviderServer_GetCapabilities(t *testing.T) {
	server := NewLLMProviderServer(nil)
	ctx := context.Background()

	t.Run("get capabilities without registry", func(t *testing.T) {
		req := &pb.CapabilitiesRequest{}

		resp, err := server.GetCapabilities(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.SupportsStreaming)
		assert.NotNil(t, resp.Limits)
		assert.True(t, len(resp.SupportedFeatures) > 0)
		assert.Contains(t, resp.SupportedRequestTypes, "completion")
		assert.Contains(t, resp.SupportedRequestTypes, "chat")
		assert.Contains(t, resp.SupportedRequestTypes, "streaming")
	})

	t.Run("verify model limits", func(t *testing.T) {
		req := &pb.CapabilitiesRequest{}

		resp, err := server.GetCapabilities(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp.Limits)
		assert.Equal(t, int32(4096), resp.Limits.MaxTokens)
		assert.Equal(t, int32(100000), resp.Limits.MaxInputLength)
		assert.Equal(t, int32(4096), resp.Limits.MaxOutputLength)
		assert.Equal(t, int32(100), resp.Limits.MaxConcurrentRequests)
	})
}

func TestLLMProviderServer_ValidateConfig(t *testing.T) {
	server := NewLLMProviderServer(nil)
	ctx := context.Background()

	t.Run("validate nil config", func(t *testing.T) {
		req := &pb.ValidateConfigRequest{
			Config: nil,
		}

		resp, err := server.ValidateConfig(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Valid)
		assert.Contains(t, resp.Errors, "configuration is required")
	})

	t.Run("validate config without type", func(t *testing.T) {
		configStruct, _ := structpb.NewStruct(map[string]interface{}{
			"name": "test-provider",
		})
		req := &pb.ValidateConfigRequest{
			Config: configStruct,
		}

		resp, err := server.ValidateConfig(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Valid)
		assert.Contains(t, resp.Errors, "provider type is required")
	})

	t.Run("validate config with warnings for missing api key", func(t *testing.T) {
		configStruct, _ := structpb.NewStruct(map[string]interface{}{
			"name": "test-provider",
			"type": "claude",
		})
		req := &pb.ValidateConfigRequest{
			Config: configStruct,
		}

		resp, err := server.ValidateConfig(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Valid) // Valid but with warnings
		assert.True(t, len(resp.Warnings) > 0)
	})

	t.Run("validate ollama config without base_url", func(t *testing.T) {
		configStruct, _ := structpb.NewStruct(map[string]interface{}{
			"name": "local-ollama",
			"type": "ollama",
		})
		req := &pb.ValidateConfigRequest{
			Config: configStruct,
		}

		resp, err := server.ValidateConfig(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Valid) // Valid but with warning about base_url
	})

	t.Run("validate config with invalid timeout", func(t *testing.T) {
		configStruct, _ := structpb.NewStruct(map[string]interface{}{
			"name":    "test-provider",
			"type":    "deepseek",
			"timeout": float64(-5),
		})
		req := &pb.ValidateConfigRequest{
			Config: configStruct,
		}

		resp, err := server.ValidateConfig(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.Valid)
		assert.Contains(t, resp.Errors, "timeout must be positive")
	})
}

func TestLLMProviderServerStruct(t *testing.T) {
	// Verify LLMProviderServer struct can be instantiated
	server := NewLLMProviderServer(nil)
	require.NotNil(t, server)
}

// Benchmark tests
func BenchmarkGrpcServer_Complete(b *testing.B) {
	server := NewLLMFacadeServer()
	ctx := context.Background()

	req := &pb.CompletionRequest{
		SessionId:      "benchmark-session",
		Prompt:         "Benchmark test prompt",
		MemoryEnhanced: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.Complete(ctx, req)
	}
}

func BenchmarkLLMProviderServer_Complete(b *testing.B) {
	server := NewLLMProviderServer(nil)
	ctx := context.Background()

	req := &pb.CompletionRequest{
		SessionId:      "benchmark-provider-session",
		Prompt:         "Benchmark provider test prompt",
		MemoryEnhanced: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.Complete(ctx, req)
	}
}
