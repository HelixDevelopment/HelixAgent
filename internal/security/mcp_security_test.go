package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MCPSecurityManager Tests
// =============================================================================

func TestNewMCPSecurityManager(t *testing.T) {
	t.Parallel()
	t.Run("with nil config and logger", func(t *testing.T) {
		manager := NewMCPSecurityManager(nil, nil)
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
		assert.NotNil(t, manager.logger)
		assert.NotNil(t, manager.trustedServers)
		assert.NotNil(t, manager.toolRegistry)
	})

	t.Run("with custom config", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers:         false,
			TrustedServers:        []string{"server1", "server2"},
			RequireToolSignatures: true,
			MaxCallDepth:          5,
		}
		logger := logrus.New()

		manager := NewMCPSecurityManager(config, logger)
		assert.NotNil(t, manager)
		assert.Equal(t, config, manager.config)
	})
}

func TestMCPSecurityManager_SetAuditLogger(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)
	auditLogger := &mockAuditLogger{}

	manager.SetAuditLogger(auditLogger)
	assert.NotNil(t, manager.auditLogger)
}

func TestMCPSecurityManager_RegisterTrustedServer(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	t.Run("register server with ID", func(t *testing.T) {
			t.Parallel()
		server := &TrustedServer{
			ID:           "server-001",
			Name:         "Test Server",
			URL:          "https://test.example.com",
			Capabilities: []string{"tool1", "tool2"},
		}

		err := manager.RegisterTrustedServer(server)
		require.NoError(t, err)
		assert.NotEmpty(t, server.Fingerprint)
		assert.True(t, server.Verified)
		assert.False(t, server.LastVerified.IsZero())
	})

	t.Run("register server without ID generates one", func(t *testing.T) {
			t.Parallel()
		server := &TrustedServer{
			Name: "Auto ID Server",
			URL:  "https://auto.example.com",
		}

		err := manager.RegisterTrustedServer(server)
		require.NoError(t, err)
		assert.NotEmpty(t, server.ID)
	})
}

func TestMCPSecurityManager_VerifyServer(t *testing.T) {
	t.Parallel()
	config := &MCPSecurityConfig{
		VerifyServers: true,
	}
	manager := NewMCPSecurityManager(config, nil)

	t.Run("verify registered server", func(t *testing.T) {
			t.Parallel()
		server := &TrustedServer{
			ID:   "verified-server",
			Name: "Verified",
			URL:  "https://verified.example.com",
		}
		err := manager.RegisterTrustedServer(server)
		require.NoError(t, err)

		result, err := manager.VerifyServer("verified-server")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Verified)
	})

	t.Run("verify non-existent server fails", func(t *testing.T) {
			t.Parallel()
		_, err := manager.VerifyServer("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server not found")
	})

	t.Run("verify with verification disabled returns nil", func(t *testing.T) {
			t.Parallel()
		disabledConfig := &MCPSecurityConfig{
			VerifyServers: false,
		}
		disabledManager := NewMCPSecurityManager(disabledConfig, nil)

		result, err := disabledManager.VerifyServer("any-server")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestMCPSecurityManager_RegisterToolPermission(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	permission := &ToolPermission{
		ToolName:   "read_file",
		ServerID:   "server-001",
		Permission: PermissionReadOnly,
		RateLimit: &ToolRateLimit{
			MaxCalls: 100,
			Window:   time.Minute,
		},
	}

	manager.RegisterToolPermission(permission)

	// Verify by checking tool call with this permission
	// The permission should be found when checking tool calls
	key := manager.toolKey("server-001", "read_file")
	assert.Contains(t, manager.toolRegistry, key)
}

func TestMCPSecurityManager_SetDefaultToolPermission(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	manager.SetDefaultToolPermission("bash", PermissionDeny)
	manager.SetDefaultToolPermission("read", PermissionReadOnly)

	assert.Equal(t, PermissionDeny, manager.config.ToolPermissions["bash"])
	assert.Equal(t, PermissionReadOnly, manager.config.ToolPermissions["read"])
}

func TestMCPSecurityManager_CheckToolCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("allow tool call without verification", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers:         false,
			RequireToolSignatures: false,
			MaxCallDepth:          10,
			AuditLogging:          false,
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName: "read_file",
			ServerID: "any-server",
			Arguments: map[string]interface{}{
				"path": "/test/file.txt",
			},
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response.Allowed)
	})

	t.Run("deny unverified server", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: true,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName: "read_file",
			ServerID: "untrusted-server",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "verification failed")
	})

	t.Run("deny missing signature when required", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers:         false,
			RequireToolSignatures: true,
			MaxCallDepth:          10,
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName:  "tool",
			Signature: "", // Missing signature
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "signature required")
	})

	t.Run("allow with valid signature", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers:         false,
			RequireToolSignatures: true,
			MaxCallDepth:          10,
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName:  "tool",
			Signature: "valid-signature",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response.Allowed)
	})

	t.Run("deny exceeding max call depth", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  2,
		}
		manager := NewMCPSecurityManager(config, nil)

		// Fill up call stack
		manager.callStack = []string{"tool1", "tool2"}

		request := &ToolCallRequest{
			ToolName: "tool3",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "depth exceeded")
	})

	t.Run("deny tool with PermissionDeny", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:   "dangerous_tool",
			ServerID:   "server1",
			Permission: PermissionDeny,
		})

		request := &ToolCallRequest{
			ToolName: "dangerous_tool",
			ServerID: "server1",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "denied")
	})

	t.Run("deny write operation with read-only permission", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:   "write",
			ServerID:   "server1",
			Permission: PermissionReadOnly,
		})

		request := &ToolCallRequest{
			ToolName: "write",
			ServerID: "server1",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "Write operations not allowed")
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:   "rate_limited_tool",
			ServerID:   "server1",
			Permission: PermissionFull,
			RateLimit: &ToolRateLimit{
				MaxCalls: 1,
				Window:   time.Hour,
			},
		})

		request := &ToolCallRequest{
			ToolName: "rate_limited_tool",
			ServerID: "server1",
		}

		// First call should succeed
		response1, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response1.Allowed)

		// Pop the call stack for the next test
		manager.PopCallStack()

		// Second call should be rate limited
		response2, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response2.Allowed)
		assert.Contains(t, response2.Reason, "Rate limit")
	})

	t.Run("blocked argument value", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:   "tool",
			ServerID:   "server1",
			Permission: PermissionFull,
			BlockedArgs: map[string][]string{
				"path": {"/etc/passwd", "/etc/shadow"},
			},
		})

		request := &ToolCallRequest{
			ToolName: "tool",
			ServerID: "server1",
			Arguments: map[string]interface{}{
				"path": "/etc/passwd",
			},
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "blocked value")
	})

	t.Run("invalid argument value (not in allowed list)", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:   "tool",
			ServerID:   "server1",
			Permission: PermissionFull,
			AllowedArgs: map[string][]string{
				"operation": {"read", "list"},
			},
		})

		request := &ToolCallRequest{
			ToolName: "tool",
			ServerID: "server1",
			Arguments: map[string]interface{}{
				"operation": "delete",
			},
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "invalid value")
	})

	t.Run("require approval flag", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		manager.RegisterToolPermission(&ToolPermission{
			ToolName:        "sensitive_tool",
			ServerID:        "server1",
			Permission:      PermissionFull,
			RequireApproval: true,
		})

		request := &ToolCallRequest{
			ToolName: "sensitive_tool",
			ServerID: "server1",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response.Allowed)
		assert.True(t, response.RequireApproval)
	})

	t.Run("sandbox sanitizes arguments", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
			SandboxConfig: &SandboxConfig{
				Enabled: true,
			},
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName: "tool",
			Arguments: map[string]interface{}{
				"command": "ls; rm -rf /",
			},
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response.Allowed)

		// Sanitized args should not contain dangerous characters
		if response.ModifiedArgs != nil {
			cmd, ok := response.ModifiedArgs["command"].(string)
			if ok {
				assert.NotContains(t, cmd, ";")
			}
		}
	})

	t.Run("audit logging on allowed call", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
			AuditLogging:  true,
		}
		manager := NewMCPSecurityManager(config, nil)
		auditLogger := &mockAuditLogger{}
		manager.SetAuditLogger(auditLogger)

		request := &ToolCallRequest{
			ToolName:  "tool",
			UserID:    "user-123",
			SessionID: "session-456",
		}

		_, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)

		// Verify audit event was logged
		assert.NotEmpty(t, auditLogger.events)
		event := auditLogger.events[len(auditLogger.events)-1]
		assert.Equal(t, AuditEventToolCall, event.EventType)
		assert.Equal(t, "user-123", event.UserID)
	})

	t.Run("trusted server from config list", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers:  true,
			TrustedServers: []string{"implicitly-trusted"},
			MaxCallDepth:   10,
		}
		manager := NewMCPSecurityManager(config, nil)

		request := &ToolCallRequest{
			ToolName: "tool",
			ServerID: "implicitly-trusted",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.True(t, response.Allowed)
	})

	t.Run("check tool not in server capabilities", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: true,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		// Register server with specific capabilities
		server := &TrustedServer{
			ID:           "limited-server",
			Name:         "Limited",
			URL:          "https://limited.example.com",
			Capabilities: []string{"read", "list"},
		}
		_ = manager.RegisterTrustedServer(server)

		request := &ToolCallRequest{
			ToolName: "write", // Not in capabilities
			ServerID: "limited-server",
		}

		response, err := manager.CheckToolCall(ctx, request)
		require.NoError(t, err)
		assert.False(t, response.Allowed)
		assert.Contains(t, response.Reason, "not in server capabilities")
	})
}

func TestMCPSecurityManager_CallStack(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	t.Run("get empty call stack", func(t *testing.T) {
			t.Parallel()
		stack := manager.GetCallStack()
		assert.Empty(t, stack)
	})

	t.Run("pop call stack", func(t *testing.T) {
			t.Parallel()
		config := &MCPSecurityConfig{
			VerifyServers: false,
			MaxCallDepth:  10,
		}
		manager := NewMCPSecurityManager(config, nil)

		ctx := context.Background()

		// Make a tool call to add to stack
		request := &ToolCallRequest{
			ToolName: "tool1",
		}
		_, _ = manager.CheckToolCall(ctx, request)

		stack := manager.GetCallStack()
		assert.Len(t, stack, 1)
		assert.Equal(t, "tool1", stack[0])

		manager.PopCallStack()

		stack = manager.GetCallStack()
		assert.Empty(t, stack)
	})

	t.Run("pop empty call stack", func(t *testing.T) {
			t.Parallel()
		manager := NewMCPSecurityManager(nil, nil)

		// Should not panic
		manager.PopCallStack()

		stack := manager.GetCallStack()
		assert.Empty(t, stack)
	})
}

func TestMCPSecurityManager_IsWriteOperation(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	writeTools := []string{"write", "edit", "delete", "create", "execute", "bash", "shell", "modify", "update", "remove", "notebookedit"}
	readTools := []string{"read", "list", "search", "query", "get"}

	for _, tool := range writeTools {
		t.Run(tool+"_is_write", func(t *testing.T) {
				t.Parallel()
			request := &ToolCallRequest{ToolName: tool}
			assert.True(t, manager.isWriteOperation(request))
		})
	}

	for _, tool := range readTools {
		t.Run(tool+"_is_not_write", func(t *testing.T) {
				t.Parallel()
			request := &ToolCallRequest{ToolName: tool}
			assert.False(t, manager.isWriteOperation(request))
		})
	}
}

func TestMCPSecurityManager_CheckRateLimit(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	t.Run("within rate limit", func(t *testing.T) {
			t.Parallel()
		limit := &ToolRateLimit{
			MaxCalls:    5,
			Window:      time.Minute,
			windowStart: time.Now(),
		}

		for i := 0; i < 5; i++ {
			assert.True(t, manager.checkRateLimit(limit))
		}
	})

	t.Run("exceeds rate limit", func(t *testing.T) {
			t.Parallel()
		limit := &ToolRateLimit{
			MaxCalls:     2,
			Window:       time.Minute,
			windowStart:  time.Now(),
			currentCalls: 2,
		}

		assert.False(t, manager.checkRateLimit(limit))
	})

	t.Run("window expired resets count", func(t *testing.T) {
			t.Parallel()
		limit := &ToolRateLimit{
			MaxCalls:     2,
			Window:       time.Millisecond,
			windowStart:  time.Now().Add(-time.Second), // Expired
			currentCalls: 2,
		}

		assert.True(t, manager.checkRateLimit(limit))
		assert.Equal(t, 1, limit.currentCalls)
	})
}

func TestMCPSecurityManager_SanitizeString(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	testCases := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"ls; rm -rf /", "ls rm -rf /"},
		{"$(cat /etc/passwd)", "cat /etc/passwd"},
		{"`whoami`", "whoami"},
		{"test|pipe", "testpipe"},
		{"test&background", "testbackground"},
		{"hello (world)", "hello world"},
		{"{object}", "object"},
		{"test<>redirect", "testredirect"},
		{"path\\escape", "pathescape"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
				t.Parallel()
			result := manager.sanitizeString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMCPSecurityManager_SanitizeArguments(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	args := map[string]interface{}{
		"command":  "ls; rm -rf /",
		"count":    42,
		"enabled":  true,
		"nested":   map[string]interface{}{"key": "value"},
		"safe_str": "normal text",
	}

	sanitized := manager.sanitizeArguments(args)

	// String values should be sanitized
	assert.Equal(t, "ls rm -rf /", sanitized["command"])
	// Non-string values should be preserved
	assert.Equal(t, 42, sanitized["count"])
	assert.Equal(t, true, sanitized["enabled"])
}

func TestMCPSecurityManager_CalculateFingerprint(t *testing.T) {
	t.Parallel()
	manager := NewMCPSecurityManager(nil, nil)

	server := &TrustedServer{
		Name:      "Test Server",
		URL:       "https://test.example.com",
		PublicKey: "test-public-key",
	}

	fingerprint := manager.calculateFingerprint(server)
	assert.NotEmpty(t, fingerprint)
	assert.Len(t, fingerprint, 64) // SHA256 hex length

	// Same input should produce same fingerprint
	fingerprint2 := manager.calculateFingerprint(server)
	assert.Equal(t, fingerprint, fingerprint2)
}

// =============================================================================
// SandboxedToolExecutor Tests
// =============================================================================

func TestNewSandboxedToolExecutor(t *testing.T) {
	t.Parallel()
	t.Run("with nil config", func(t *testing.T) {
		executor := NewSandboxedToolExecutor(nil, nil)
		assert.NotNil(t, executor)
		assert.NotNil(t, executor.config)
		assert.True(t, executor.config.Enabled)
	})

	t.Run("with custom config", func(t *testing.T) {
			t.Parallel()
		config := &SandboxConfig{
			Enabled:          true,
			MaxExecutionTime: time.Minute,
			MemoryLimit:      1024 * 1024 * 1024,
			NetworkAccess:    NetworkPolicyNone,
			FilesystemAccess: FilesystemPolicyReadOnly,
		}
		logger := logrus.New()

		executor := NewSandboxedToolExecutor(config, logger)
		assert.NotNil(t, executor)
		assert.Equal(t, config, executor.config)
	})
}

func TestSandboxedToolExecutor_Execute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("execute with sandbox disabled", func(t *testing.T) {
			t.Parallel()
		executor := NewSandboxedToolExecutor(&SandboxConfig{
			Enabled: false,
		}, nil)

		result, err := executor.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return "result", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "result", result)
	})

	t.Run("execute with sandbox enabled - success", func(t *testing.T) {
			t.Parallel()
		executor := NewSandboxedToolExecutor(&SandboxConfig{
			Enabled:          true,
			MaxExecutionTime: 5 * time.Second,
		}, nil)

		result, err := executor.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return "sandboxed result", nil
		})

		require.NoError(t, err)
		assert.Equal(t, "sandboxed result", result)
	})

	t.Run("execute with sandbox enabled - error", func(t *testing.T) {
			t.Parallel()
		executor := NewSandboxedToolExecutor(&SandboxConfig{
			Enabled:          true,
			MaxExecutionTime: 5 * time.Second,
		}, nil)

		_, err := executor.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("execution error")
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "execution error")
	})

	t.Run("execute with sandbox enabled - timeout", func(t *testing.T) {
			t.Parallel()
		executor := NewSandboxedToolExecutor(&SandboxConfig{
			Enabled:          true,
			MaxExecutionTime: 50 * time.Millisecond,
		}, nil)

		_, err := executor.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			// Simulate slow operation
			time.Sleep(200 * time.Millisecond)
			return "result", nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
	})
}

// =============================================================================
// MCPSecurityConfig Tests
// =============================================================================

func TestDefaultMCPSecurityConfig(t *testing.T) {
	t.Parallel()
	config := DefaultMCPSecurityConfig()

	assert.True(t, config.VerifyServers)
	assert.Empty(t, config.TrustedServers)
	assert.False(t, config.RequireToolSignatures)
	assert.NotNil(t, config.ToolPermissions)
	assert.True(t, config.AuditLogging)
	assert.Equal(t, 10, config.MaxCallDepth)
	assert.NotNil(t, config.SandboxConfig)
	assert.True(t, config.SandboxConfig.Enabled)
}

func TestPermissionLevel(t *testing.T) {
	t.Parallel()
	levels := []PermissionLevel{
		PermissionDeny,
		PermissionReadOnly,
		PermissionRestricted,
		PermissionFull,
	}

	for _, level := range levels {
		assert.NotEmpty(t, string(level))
	}
}

func TestNetworkPolicy(t *testing.T) {
	t.Parallel()
	policies := []NetworkPolicy{
		NetworkPolicyNone,
		NetworkPolicyLocal,
		NetworkPolicyRestricted,
		NetworkPolicyFull,
	}

	for _, policy := range policies {
		assert.NotEmpty(t, string(policy))
	}
}

func TestFilesystemPolicy(t *testing.T) {
	t.Parallel()
	policies := []FilesystemPolicy{
		FilesystemPolicyNone,
		FilesystemPolicyReadOnly,
		FilesystemPolicyRestricted,
		FilesystemPolicyFull,
	}

	for _, policy := range policies {
		assert.NotEmpty(t, string(policy))
	}
}

func TestTrustedServerFields(t *testing.T) {
	t.Parallel()
	server := &TrustedServer{
		ID:           "server-001",
		Name:         "Test Server",
		URL:          "https://test.example.com",
		PublicKey:    "public-key-123",
		Fingerprint:  "abc123",
		Verified:     true,
		LastVerified: time.Now(),
		Capabilities: []string{"tool1", "tool2"},
	}

	assert.Equal(t, "server-001", server.ID)
	assert.Equal(t, "Test Server", server.Name)
	assert.True(t, server.Verified)
	assert.Len(t, server.Capabilities, 2)
}

func TestToolPermissionFields(t *testing.T) {
	t.Parallel()
	permission := &ToolPermission{
		ToolName:   "read_file",
		ServerID:   "server-001",
		Permission: PermissionReadOnly,
		AllowedArgs: map[string][]string{
			"path": {"/safe/*"},
		},
		BlockedArgs: map[string][]string{
			"path": {"/etc/passwd"},
		},
		RateLimit: &ToolRateLimit{
			MaxCalls: 100,
			Window:   time.Minute,
		},
		RequireApproval: true,
		Metadata: map[string]interface{}{
			"description": "Read file tool",
		},
	}

	assert.Equal(t, "read_file", permission.ToolName)
	assert.Equal(t, PermissionReadOnly, permission.Permission)
	assert.NotNil(t, permission.AllowedArgs)
	assert.NotNil(t, permission.BlockedArgs)
	assert.NotNil(t, permission.RateLimit)
	assert.True(t, permission.RequireApproval)
}

func TestToolCallRequestFields(t *testing.T) {
	t.Parallel()
	request := &ToolCallRequest{
		ToolName:  "read_file",
		ServerID:  "server-001",
		Arguments: map[string]interface{}{"path": "/test.txt"},
		UserID:    "user-123",
		SessionID: "session-456",
		Signature: "sig-789",
	}

	assert.Equal(t, "read_file", request.ToolName)
	assert.Equal(t, "server-001", request.ServerID)
	assert.Equal(t, "user-123", request.UserID)
	assert.NotEmpty(t, request.Signature)
}

func TestToolCallResponseFields(t *testing.T) {
	t.Parallel()
	response := &ToolCallResponse{
		Allowed:         true,
		Reason:          "Access granted",
		ModifiedArgs:    map[string]interface{}{"safe": true},
		RequireApproval: false,
	}

	assert.True(t, response.Allowed)
	assert.NotEmpty(t, response.Reason)
	assert.NotNil(t, response.ModifiedArgs)
	assert.False(t, response.RequireApproval)
}

func TestSandboxConfigFields(t *testing.T) {
	t.Parallel()
	config := &SandboxConfig{
		Enabled:          true,
		MaxExecutionTime: 30 * time.Second,
		MemoryLimit:      512 * 1024 * 1024,
		NetworkAccess:    NetworkPolicyRestricted,
		FilesystemAccess: FilesystemPolicyReadOnly,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 30*time.Second, config.MaxExecutionTime)
	assert.Equal(t, int64(512*1024*1024), config.MemoryLimit)
}

func TestToolRateLimitFields(t *testing.T) {
	t.Parallel()
	limit := &ToolRateLimit{
		MaxCalls: 100,
		Window:   5 * time.Minute,
	}

	assert.Equal(t, 100, limit.MaxCalls)
	assert.Equal(t, 5*time.Minute, limit.Window)
}

// =============================================================================
// Helper function tests
// =============================================================================

func TestReplaceAll(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input    string
		old      string
		new      string
		expected string
	}{
		{"hello world", "world", "universe", "hello universe"},
		{"aaa", "a", "b", "bbb"},
		{"no match", "x", "y", "no match"},
		{"", "a", "b", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
				t.Parallel()
			result := replaceAll(tc.input, tc.old, tc.new)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFindIndex(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		s        string
		substr   string
		expected int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"", "a", -1},
		{"abc", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.s+"_"+tc.substr, func(t *testing.T) {
				t.Parallel()
			result := findIndex(tc.s, tc.substr)
			assert.Equal(t, tc.expected, result)
		})
	}
}
