package testutils

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestContext(t *testing.T) {
	tc := NewTestContext(5 * time.Second)
	require.NotNil(t, tc)
	require.NotNil(t, tc.Ctx)
	require.NotNil(t, tc.Cancel)
	defer tc.Cleanup()

	assert.NoError(t, tc.Ctx.Err())
}

func TestTestContextCleanup(t *testing.T) {
	tc := NewTestContext(100 * time.Millisecond)
	tc.Cleanup()

	assert.Error(t, tc.Ctx.Err())
	assert.Equal(t, context.Canceled, tc.Ctx.Err())
}

func TestTestContextTimeout(t *testing.T) {
	tc := NewTestContext(50 * time.Millisecond)
	defer tc.Cleanup()

	time.Sleep(100 * time.Millisecond)

	assert.Error(t, tc.Ctx.Err())
	assert.Equal(t, context.DeadlineExceeded, tc.Ctx.Err())
}

func TestNewTestRouter(t *testing.T) {
	tr := NewTestRouter(t)
	require.NotNil(t, tr)
	require.NotNil(t, tr.Engine)
	require.Equal(t, t, tr.T)
}

func TestTestRouterAddRoute(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("GET", "/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	rec := tr.GET("/test", nil)
	assert.Equal(t, 200, rec.Code)
}

func TestTestRouterPOST(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("POST", "/test", func(c *gin.Context) {
		c.JSON(201, gin.H{"created": true})
	})

	rec := tr.POST("/test", map[string]string{"key": "value"}, nil)
	assert.Equal(t, 201, rec.Code)
}

func TestTestRouterPUT(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("PUT", "/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"updated": true})
	})

	rec := tr.PUT("/test", map[string]string{"key": "value"}, nil)
	assert.Equal(t, 200, rec.Code)
}

func TestTestRouterDELETE(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("DELETE", "/test", func(c *gin.Context) {
		c.JSON(204, nil)
	})

	rec := tr.DELETE("/test", nil)
	assert.Equal(t, 204, rec.Code)
}

func TestTestRouterAssertStatus(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("GET", "/ok", func(c *gin.Context) {
		c.Status(200)
	})

	rec := tr.GET("/ok", nil)
	tr.AssertStatus(rec, 200)
}

func TestTestRouterAssertJSON(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("GET", "/json", func(c *gin.Context) {
		c.JSON(200, gin.H{"key": "value"})
	})

	rec := tr.GET("/json", nil)
	result := tr.AssertJSON(rec)

	assert.Equal(t, "value", result["key"])
}

func TestTestRouterAssertJSONArray(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("GET", "/array", func(c *gin.Context) {
		c.JSON(200, []interface{}{1, 2, 3})
	})

	rec := tr.GET("/array", nil)
	result := tr.AssertJSONArray(rec)

	assert.Len(t, result, 3)
}

func TestNewTestConfig(t *testing.T) {
	cfg := NewTestConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "8100", cfg.Server.Port)
	assert.NotEmpty(t, cfg.Server.JWTSecret)
}

func TestNewTestConfigWithJWTSecret(t *testing.T) {
	cfg := NewTestConfigWithJWTSecret("my-secret")
	require.NotNil(t, cfg)
	assert.Equal(t, "my-secret", cfg.Server.JWTSecret)
}

func TestNewTestConfigWithProviders(t *testing.T) {
	providers := map[string]config.ProviderConfig{
		"test": {Enabled: true, Model: "test-model"},
	}

	cfg := NewTestConfigWithProviders(providers)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.LLM.Providers, 1)
}

func TestCreateTestLLMRequest(t *testing.T) {
	req := CreateTestLLMRequest("test prompt")
	require.NotNil(t, req)
	assert.Contains(t, req.ID, "test-request-")
	assert.Equal(t, "test prompt", req.Prompt)
	assert.Equal(t, "test-model", req.ModelParams.Model)
}

func TestCreateTestLLMResponse(t *testing.T) {
	resp := CreateTestLLMResponse("req-123", "test content")
	require.NotNil(t, resp)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "test content", resp.Content)
}

func TestCreateTestChatRequest(t *testing.T) {
	messages := []models.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
	}

	req := CreateTestChatRequest(messages)
	require.NotNil(t, req)
	assert.Len(t, req.Messages, 2)
	assert.Equal(t, "chat", req.RequestType)
}

func TestCreateTestMessages(t *testing.T) {
	messages := CreateTestMessages("user message")
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "user message", messages[1].Content)
}

func TestAssertErrorContains(t *testing.T) {
	err := assert.AnError
	AssertErrorContains(t, err, "assert")
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestWaitForCondition_Immediate(t *testing.T) {
	result := WaitForCondition(t, func() bool { return true }, 1*time.Second, 10*time.Millisecond)
	assert.True(t, result)
}

func TestWaitForCondition_Delayed(t *testing.T) {
	count := 0
	result := WaitForCondition(t, func() bool {
		count++
		return count >= 3
	}, 1*time.Second, 10*time.Millisecond)
	assert.True(t, result)
	assert.GreaterOrEqual(t, count, 3)
}

func TestWaitForCondition_Timeout(t *testing.T) {
	result := WaitForCondition(t, func() bool { return false }, 50*time.Millisecond, 10*time.Millisecond)
	assert.False(t, result)
}

func TestCreateAuthHeaders(t *testing.T) {
	headers := CreateAuthHeaders("test-token")
	require.NotNil(t, headers)
	assert.Equal(t, "Bearer test-token", headers["Authorization"])
}

func TestCreateAPIKeyHeaders(t *testing.T) {
	headers := CreateAPIKeyHeaders("test-api-key")
	require.NotNil(t, headers)
	assert.Equal(t, "test-api-key", headers["X-API-Key"])
}

func TestSkipIfShort(t *testing.T) {
	if testing.Short() {
		SkipIfShort(t)
	}
}

func TestJSONMarshal(t *testing.T) {
	data := map[string]string{"key": "value"}
	result := JSONMarshal(t, data)
	assert.Contains(t, string(result), `"key"`)
	assert.Contains(t, string(result), `"value"`)
}

func TestJSONUnmarshal(t *testing.T) {
	data := []byte(`{"key": "value"}`)
	var result map[string]string
	JSONUnmarshal(t, data, &result)
	assert.Equal(t, "value", result["key"])
}

func TestTestRouterWithHeaders(t *testing.T) {
	tr := NewTestRouter(t)

	tr.AddRoute("GET", "/auth", func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		c.JSON(200, gin.H{"auth": auth})
	})

	headers := map[string]string{"Authorization": "Bearer test-token"}
	rec := tr.GET("/auth", headers)

	result := tr.AssertJSON(rec)
	assert.Equal(t, "Bearer test-token", result["auth"])
}

func BenchmarkNewTestRouter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewTestRouter(nil)
	}
}

func BenchmarkCreateTestLLMRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CreateTestLLMRequest("test")
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	data := map[string]string{"key": "value"}
	for i := 0; i < b.N; i++ {
		_ = JSONMarshal(nil, data)
	}
}
