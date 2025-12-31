package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSecurityTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewProtocolSecurity(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	require.NotNil(t, security)
	assert.NotNil(t, security.apiKeys)
	assert.NotNil(t, security.permissions)
	assert.NotNil(t, security.logger)
}

func TestProtocolSecurity_CreateAPIKey(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	t.Run("create API key with permissions", func(t *testing.T) {
		permissions := []string{"mcp:read", "mcp:execute", "lsp:read"}
		apiKey, err := security.CreateAPIKey("test-key", "test-owner", permissions)

		require.NoError(t, err)
		assert.NotNil(t, apiKey)
		assert.NotEmpty(t, apiKey.Key)
		assert.Equal(t, "test-key", apiKey.Name)
		assert.Equal(t, "test-owner", apiKey.Owner)
		assert.Equal(t, permissions, apiKey.Permissions)
		assert.True(t, apiKey.Active)
		assert.False(t, apiKey.CreatedAt.IsZero())
	})

	t.Run("create API key with wildcard permission", func(t *testing.T) {
		apiKey, err := security.CreateAPIKey("admin-key", "admin", []string{"*"})

		require.NoError(t, err)
		assert.NotNil(t, apiKey)
		assert.Contains(t, apiKey.Permissions, "*")
	})
}

func TestProtocolSecurity_ValidateAccess(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	// Create API key with specific permissions
	apiKey, _ := security.CreateAPIKey("test-key", "test-owner", []string{
		"mcp:read",
		"mcp:execute",
		"lsp:*",
	})

	t.Run("valid access with exact permission", func(t *testing.T) {
		req := ProtocolAccessRequest{
			APIKey:   apiKey.Key,
			Protocol: "mcp",
			Action:   "read",
			Resource: "test-resource",
		}

		err := security.ValidateAccess(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("valid access with wildcard permission", func(t *testing.T) {
		req := ProtocolAccessRequest{
			APIKey:   apiKey.Key,
			Protocol: "lsp",
			Action:   "anything",
			Resource: "test-resource",
		}

		err := security.ValidateAccess(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("invalid API key", func(t *testing.T) {
		req := ProtocolAccessRequest{
			APIKey:   "invalid-key",
			Protocol: "mcp",
			Action:   "read",
			Resource: "test-resource",
		}

		err := security.ValidateAccess(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		req := ProtocolAccessRequest{
			APIKey:   apiKey.Key,
			Protocol: "acp",
			Action:   "write",
			Resource: "test-resource",
		}

		err := security.ValidateAccess(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient permissions")
	})
}

func TestProtocolSecurity_ValidateAccess_WithFullAccess(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	// Create API key with full access
	apiKey, _ := security.CreateAPIKey("admin-key", "admin", []string{"*"})

	t.Run("full access grants any permission", func(t *testing.T) {
		req := ProtocolAccessRequest{
			APIKey:   apiKey.Key,
			Protocol: "any-protocol",
			Action:   "any-action",
			Resource: "any-resource",
		}

		err := security.ValidateAccess(context.Background(), req)
		assert.NoError(t, err)
	})
}

func TestProtocolSecurity_RevokeAPIKey(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	apiKey, _ := security.CreateAPIKey("revoke-key", "owner", []string{"mcp:read"})

	t.Run("revoke existing key", func(t *testing.T) {
		err := security.RevokeAPIKey(apiKey.Key)
		require.NoError(t, err)

		// Try to use revoked key
		req := ProtocolAccessRequest{
			APIKey:   apiKey.Key,
			Protocol: "mcp",
			Action:   "read",
			Resource: "test",
		}

		err = security.ValidateAccess(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("revoke non-existent key", func(t *testing.T) {
		err := security.RevokeAPIKey("non-existent-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key not found")
	})
}

func TestProtocolSecurity_ListAPIKeys(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	t.Run("list empty keys", func(t *testing.T) {
		keys := security.ListAPIKeys()
		assert.Empty(t, keys)
	})

	t.Run("list multiple keys", func(t *testing.T) {
		security.CreateAPIKey("key1", "owner1", []string{"mcp:read"})
		security.CreateAPIKey("key2", "owner2", []string{"lsp:read"})
		security.CreateAPIKey("key3", "owner3", []string{"acp:read"})

		keys := security.ListAPIKeys()
		assert.Len(t, keys, 3)
	})
}

func TestProtocolSecurity_InitializeDefaultSecurity(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	err := security.InitializeDefaultSecurity()
	require.NoError(t, err)

	keys := security.ListAPIKeys()
	assert.Len(t, keys, 2) // admin and user keys
}

func TestProtocolSecurity_ValidateProtocolAccess(t *testing.T) {
	log := newSecurityTestLogger()
	security := NewProtocolSecurity(log)

	apiKey, _ := security.CreateAPIKey("test-key", "owner", []string{"mcp:read"})

	t.Run("valid access", func(t *testing.T) {
		err := security.ValidateProtocolAccess(context.Background(), apiKey.Key, "mcp", "read", "resource")
		assert.NoError(t, err)
	})

	t.Run("invalid access", func(t *testing.T) {
		err := security.ValidateProtocolAccess(context.Background(), apiKey.Key, "mcp", "write", "resource")
		assert.Error(t, err)
	})
}

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(100)

	require.NotNil(t, limiter)
	assert.NotNil(t, limiter.requests)
	assert.Equal(t, 100, limiter.maxPerMin)
}

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("first request allowed", func(t *testing.T) {
		limiter := NewRateLimiter(10)
		assert.True(t, limiter.Allow("test-key"))
	})

	t.Run("requests under limit allowed", func(t *testing.T) {
		limiter := NewRateLimiter(10)

		for i := 0; i < 10; i++ {
			assert.True(t, limiter.Allow("test-key"), "Request %d should be allowed", i)
		}
	})

	t.Run("requests over limit denied", func(t *testing.T) {
		limiter := NewRateLimiter(5)

		// First 5 should be allowed
		for i := 0; i < 5; i++ {
			assert.True(t, limiter.Allow("test-key"), "Request %d should be allowed", i)
		}

		// 6th should be denied
		assert.False(t, limiter.Allow("test-key"), "Request over limit should be denied")
	})

	t.Run("different keys are tracked separately", func(t *testing.T) {
		limiter := NewRateLimiter(2)

		assert.True(t, limiter.Allow("key1"))
		assert.True(t, limiter.Allow("key1"))
		assert.False(t, limiter.Allow("key1")) // Over limit

		assert.True(t, limiter.Allow("key2"))  // Different key, should be allowed
		assert.True(t, limiter.Allow("key2"))
	})
}

func TestExtractAPIKeyFromHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"bearer token", "Bearer sk-test-key", "sk-test-key"},
		{"raw key", "sk-raw-key", "sk-raw-key"},
		{"empty header", "", ""},
		{"bearer without key", "Bearer ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAPIKeyFromHeader(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIKey_Structure(t *testing.T) {
	now := time.Now()
	apiKey := &APIKey{
		Key:         "sk-test-key",
		Name:        "Test Key",
		Owner:       "test-owner",
		Permissions: []string{"mcp:read", "lsp:execute"},
		CreatedAt:   now,
		LastUsed:    now,
		Active:      true,
	}

	assert.Equal(t, "sk-test-key", apiKey.Key)
	assert.Equal(t, "Test Key", apiKey.Name)
	assert.Equal(t, "test-owner", apiKey.Owner)
	assert.Len(t, apiKey.Permissions, 2)
	assert.True(t, apiKey.Active)
}

func TestProtocolAccessRequest_Structure(t *testing.T) {
	req := ProtocolAccessRequest{
		APIKey:   "sk-test-key",
		Protocol: "mcp",
		Action:   "execute",
		Resource: "/path/to/resource",
	}

	assert.Equal(t, "sk-test-key", req.APIKey)
	assert.Equal(t, "mcp", req.Protocol)
	assert.Equal(t, "execute", req.Action)
	assert.Equal(t, "/path/to/resource", req.Resource)
}

func TestGlobalRateLimiter(t *testing.T) {
	require.NotNil(t, GlobalRateLimiter)
	assert.Equal(t, 100, GlobalRateLimiter.maxPerMin)
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000000) // High limit to avoid hitting it

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("bench-key")
	}
}

func BenchmarkProtocolSecurity_ValidateAccess(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	security := NewProtocolSecurity(log)

	apiKey, _ := security.CreateAPIKey("bench-key", "owner", []string{"mcp:*"})

	req := ProtocolAccessRequest{
		APIKey:   apiKey.Key,
		Protocol: "mcp",
		Action:   "read",
		Resource: "test",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = security.ValidateAccess(ctx, req)
	}
}
