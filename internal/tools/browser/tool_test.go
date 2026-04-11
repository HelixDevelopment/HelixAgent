package browser

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTool(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	tool := NewTool(logger)

	require.NotNil(t, tool)
	require.NotNil(t, tool.browser)
	assert.NotNil(t, tool.logger)
}

func TestTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	assert.Equal(t, "Browser", tool.Name())
}

func TestTool_Description(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	desc := tool.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "browser")
}

func TestTool_Schema(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	schema := tool.Schema()

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])

	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)

	// Check required fields exist
	assert.Contains(t, properties, "action")
	assert.Contains(t, properties, "url")
	assert.Contains(t, properties, "selector")
	assert.Contains(t, properties, "timeout")

	// Check required array
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "action")
	assert.Contains(t, required, "url")
}

func TestTool_Execute_MissingAction(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"url": "https://example.com",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "action is required")
}

func TestTool_Execute_MissingURL(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "navigate",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "url is required")
}

func TestTool_Execute_ValidNavigate(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "navigate",
		"url":     "https://example.com",
		"timeout": float64(10),
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// May succeed or fail depending on connectivity
	if result.Success {
		assert.NotEmpty(t, result.URL)
	} else {
		// Error is acceptable if offline
		assert.NotEmpty(t, result.Error)
	}
}

func TestTool_Execute_WithSelector(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":   "extract",
		"url":      "https://example.com",
		"selector": "title",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// May succeed or fail depending on connectivity
	if result.Success {
		assert.NotEmpty(t, result.URL)
	}
}

func TestTool_Navigate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewTool(logger)
	ctx := context.Background()

	result, err := tool.Navigate(ctx, "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, result)

	// May succeed or fail depending on connectivity
}

func TestTool_Fetch(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewTool(logger)
	ctx := context.Background()

	result, err := tool.Fetch(ctx, "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, result)

	// May succeed or fail depending on connectivity
}

func TestTool_Extract(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewTool(logger)
	ctx := context.Background()

	result, err := tool.Extract(ctx, "https://example.com", "title")

	require.NoError(t, err)
	require.NotNil(t, result)

	// May succeed or fail depending on connectivity
}

func TestTool_Screenshot(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	result, err := tool.Screenshot(ctx, "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Playwright")
}

func TestTool_Execute_InvalidTimeoutType(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())
	ctx := context.Background()

	// Pass timeout as string instead of number
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "navigate",
		"url":     "https://example.com",
		"timeout": "invalid",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should use default timeout, so this may still work
}

func TestTool_Execute_ContextTimeout(t *testing.T) {
	t.Parallel()
	tool := NewTool(logrus.New())

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "navigate",
		"url":     "https://example.com",
		"timeout": float64(30),
	})

	// Context timeout may cause an error or just a failed result
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	} else {
		require.NotNil(t, result)
	}
}

func TestTool_ConcurrentExecution(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tool := NewTool(logger)

	// Run multiple executions concurrently
	urls := []string{
		"https://example.com",
		"https://example.org",
		"https://example.net",
	}

	results := make(chan *ToolResult, len(urls))

	for _, url := range urls {
		go func(u string) {
			ctx := context.Background()
			result, _ := tool.Navigate(ctx, u)
			results <- result
		}(url)
	}

	// Collect results
	for i := 0; i < len(urls); i++ {
		result := <-results
		require.NotNil(t, result)
		// Results may vary based on connectivity
	}
}

func BenchmarkTool_Execute(b *testing.B) {
	tool := NewTool(logrus.New())
	ctx := context.Background()
	input := map[string]interface{}{
		"action": "navigate",
		"url":    "https://example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, input)
	}
}
