package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/plugins"
)

// TestExamplePlugin_Name tests the Name method
func TestExamplePlugin_Name(t *testing.T) {
	plugin := &ExamplePlugin{}
	assert.Equal(t, "example", plugin.Name())
}

// TestExamplePlugin_Version tests the Version method
func TestExamplePlugin_Version(t *testing.T) {
	plugin := &ExamplePlugin{}
	assert.Equal(t, "1.0.0", plugin.Version())
}

// TestExamplePlugin_Capabilities tests the Capabilities method
func TestExamplePlugin_Capabilities(t *testing.T) {
	plugin := &ExamplePlugin{}
	caps := plugin.Capabilities()

	assert.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, "example-model")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedRequestTypes, "code_generation")
	assert.Contains(t, caps.SupportedRequestTypes, "reasoning")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, 2048, caps.Limits.MaxInputLength)
	assert.Equal(t, 2048, caps.Limits.MaxOutputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)
	assert.Equal(t, "SuperAgent Team", caps.Metadata["author"])
	assert.Equal(t, "MIT", caps.Metadata["license"])
}

// TestExamplePlugin_Init tests the Init method
func TestExamplePlugin_Init(t *testing.T) {
	plugin := &ExamplePlugin{}

	t.Run("initializes with config", func(t *testing.T) {
		config := map[string]interface{}{
			"setting1": "value1",
			"setting2": 42,
		}
		err := plugin.Init(config)
		assert.NoError(t, err)
		assert.Equal(t, config, plugin.config)
	})

	t.Run("initializes with nil config", func(t *testing.T) {
		plugin := &ExamplePlugin{}
		err := plugin.Init(nil)
		assert.NoError(t, err)
		assert.Nil(t, plugin.config)
	})

	t.Run("initializes with empty config", func(t *testing.T) {
		plugin := &ExamplePlugin{}
		config := map[string]interface{}{}
		err := plugin.Init(config)
		assert.NoError(t, err)
		assert.NotNil(t, plugin.config)
		assert.Empty(t, plugin.config)
	})
}

// TestExamplePlugin_Shutdown tests the Shutdown method
func TestExamplePlugin_Shutdown(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{"key": "value"})

	t.Run("shuts down successfully", func(t *testing.T) {
		ctx := context.Background()
		err := plugin.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("shuts down with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()
		err := plugin.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("handles canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		err := plugin.Shutdown(ctx)
		// Should still succeed as Shutdown doesn't check context
		assert.NoError(t, err)
	})
}

// TestExamplePlugin_HealthCheck tests the HealthCheck method
func TestExamplePlugin_HealthCheck(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{})

	t.Run("health check succeeds", func(t *testing.T) {
		ctx := context.Background()
		err := plugin.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("health check with timeout", func(t *testing.T) {
		// Health check should complete within timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := plugin.HealthCheck(ctx)
		assert.NoError(t, err)
	})
}

// TestExamplePlugin_SetSecurityContext tests the SetSecurityContext method
func TestExamplePlugin_SetSecurityContext(t *testing.T) {
	plugin := &ExamplePlugin{}

	t.Run("sets security context", func(t *testing.T) {
		ctx := &plugins.PluginSecurityContext{
			AllowedPaths:     []string{"/tmp", "/var/log"},
			BlockedFunctions: []string{"exec", "syscall"},
			ResourceLimits: plugins.ResourceLimits{
				MaxMemoryMB:        512,
				MaxCPUPercent:      50,
				MaxFileDescriptors: 100,
				NetworkAccess:      true,
			},
		}
		err := plugin.SetSecurityContext(ctx)
		assert.NoError(t, err)
	})

	t.Run("sets nil security context", func(t *testing.T) {
		err := plugin.SetSecurityContext(nil)
		// Should handle nil gracefully
		assert.NoError(t, err)
	})
}

// TestExamplePlugin_Complete tests the Complete method
func TestExamplePlugin_Complete(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{})

	t.Run("returns response for valid request", func(t *testing.T) {
		ctx := context.Background()
		req := &models.LLMRequest{
			ID:        "test-request-1",
			SessionID: "test-session",
			UserID:    "test-user",
			Prompt:    "Hello, world!",
			ModelParams: models.ModelParameters{
				Model:       "example-model",
				Temperature: 0.7,
				MaxTokens:   100,
			},
		}

		resp, err := plugin.Complete(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Equal(t, req.ID, resp.RequestID)
		assert.Equal(t, "example", resp.ProviderID)
		assert.Equal(t, "example", resp.ProviderName)
		assert.Contains(t, resp.Content, "Example response to:")
		assert.Contains(t, resp.Content, req.Prompt)
		assert.Equal(t, 0.8, resp.Confidence)
		assert.Equal(t, 150, resp.TokensUsed)
		assert.Equal(t, int64(500), resp.ResponseTime)
		assert.Equal(t, "stop", resp.FinishReason)
		assert.False(t, resp.Selected)
		assert.Equal(t, 0.0, resp.SelectionScore)
		assert.False(t, resp.CreatedAt.IsZero())
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req := &models.LLMRequest{
			ID:     "test-request-2",
			Prompt: "Test prompt",
		}

		// The plugin doesn't check context cancellation during Complete
		// So it should still return a response
		resp, err := plugin.Complete(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("handles empty prompt", func(t *testing.T) {
		ctx := context.Background()
		req := &models.LLMRequest{
			ID:     "test-request-3",
			Prompt: "",
		}

		resp, err := plugin.Complete(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Contains(t, resp.Content, "Example response to:")
	})
}

// TestExamplePlugin_CompleteStream tests the CompleteStream method
func TestExamplePlugin_CompleteStream(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{})

	t.Run("returns stream channel for valid request", func(t *testing.T) {
		ctx := context.Background()
		req := &models.LLMRequest{
			ID:        "stream-request-1",
			SessionID: "test-session",
			UserID:    "test-user",
			Prompt:    "Tell me a story",
			ModelParams: models.ModelParameters{
				Model:       "example-model",
				Temperature: 0.7,
				MaxTokens:   100,
			},
		}

		ch, err := plugin.CompleteStream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Collect all responses
		var responses []*models.LLMResponse
		for resp := range ch {
			responses = append(responses, resp)
		}

		// Should have multiple responses (streaming chunks + final)
		assert.Greater(t, len(responses), 1)

		// Check first response (streaming chunk)
		first := responses[0]
		assert.Equal(t, req.ID, first.RequestID)
		assert.Equal(t, "example", first.ProviderID)
		assert.Equal(t, "example", first.ProviderName)
		assert.NotEmpty(t, first.Content)
		assert.Equal(t, "", first.FinishReason)

		// Check last response (final)
		last := responses[len(responses)-1]
		assert.Equal(t, "stop", last.FinishReason)
		assert.Equal(t, "", last.Content) // Final chunk has empty content
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		req := &models.LLMRequest{
			ID:     "stream-request-2",
			Prompt: "Test prompt",
		}

		ch, err := plugin.CompleteStream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, ch)

		// Cancel after receiving first response
		<-ch
		cancel()

		// Channel should be closed after context cancellation
		// Drain remaining responses
		var count int
		for range ch {
			count++
		}
		// May have received some responses before cancellation
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("streaming response contains expected words", func(t *testing.T) {
		ctx := context.Background()
		req := &models.LLMRequest{
			ID:     "stream-request-3",
			Prompt: "Test",
		}

		ch, err := plugin.CompleteStream(ctx, req)
		require.NoError(t, err)

		var fullContent string
		for resp := range ch {
			fullContent += resp.Content
		}

		// The streaming response should contain these words
		expectedWords := []string{"This", "is", "an", "example", "streaming", "response"}
		for _, word := range expectedWords {
			assert.Contains(t, fullContent, word)
		}
	})

	t.Run("streaming preserves request ID", func(t *testing.T) {
		ctx := context.Background()
		req := &models.LLMRequest{
			ID:     "unique-stream-request-id",
			Prompt: "Test",
		}

		ch, err := plugin.CompleteStream(ctx, req)
		require.NoError(t, err)

		for resp := range ch {
			assert.Equal(t, req.ID, resp.RequestID)
		}
	})
}

// TestExamplePlugin_Implements_Interface tests that ExamplePlugin implements LLMPlugin
func TestExamplePlugin_Implements_Interface(t *testing.T) {
	var plugin plugins.LLMPlugin = &ExamplePlugin{}
	assert.NotNil(t, plugin)

	// Verify all interface methods exist
	assert.Equal(t, "example", plugin.Name())
	assert.Equal(t, "1.0.0", plugin.Version())
	assert.NotNil(t, plugin.Capabilities())

	err := plugin.Init(nil)
	assert.NoError(t, err)

	ctx := context.Background()
	err = plugin.HealthCheck(ctx)
	assert.NoError(t, err)

	err = plugin.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestPlugin_GlobalVariable tests the exported Plugin variable
func TestPlugin_GlobalVariable(t *testing.T) {
	// The Plugin variable should be exported and be an LLMPlugin
	assert.NotNil(t, Plugin)

	// Should be an ExamplePlugin
	_, ok := Plugin.(*ExamplePlugin)
	assert.True(t, ok)

	// Should implement LLMPlugin interface
	var _ plugins.LLMPlugin = Plugin
}

// TestExamplePlugin_Concurrent_Complete tests concurrent Complete calls
func TestExamplePlugin_Concurrent_Complete(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{})

	ctx := context.Background()
	const numGoroutines = 10

	results := make(chan *models.LLMResponse, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			req := &models.LLMRequest{
				ID:     "concurrent-request-" + string(rune('0'+id)),
				Prompt: "Test prompt",
			}
			resp, err := plugin.Complete(ctx, req)
			if err != nil {
				errors <- err
				return
			}
			results <- resp
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case resp := <-results:
			assert.NotNil(t, resp)
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(time.Second * 10):
			t.Fatal("Test timed out")
		}
	}
}

// TestExamplePlugin_Concurrent_Stream tests concurrent CompleteStream calls
func TestExamplePlugin_Concurrent_Stream(t *testing.T) {
	plugin := &ExamplePlugin{}
	plugin.Init(map[string]interface{}{})

	ctx := context.Background()
	const numGoroutines = 5

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			req := &models.LLMRequest{
				ID:     "stream-concurrent-" + string(rune('0'+id)),
				Prompt: "Test prompt",
			}
			ch, err := plugin.CompleteStream(ctx, req)
			if err != nil {
				errors <- err
				return
			}
			// Drain the channel
			for range ch {
			}
			done <- true
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(time.Second * 10):
			t.Fatal("Test timed out")
		}
	}
}
