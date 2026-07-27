package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// MCPSecurityManager provides security features for MCP protocol.
// Integrates with HelixAgent's existing MCP client and server infrastructure.
//
// Concurrent-safe by construction (CONST-029):
//   - trustedServers / toolRegistry / callStack are safe.* containers.
//   - mu (sync.RWMutex) survives as a Pattern Zeta scalar mutex: it
//     serialises CheckToolCall's compound operation (call-depth check
//   - push + rate-limit read-modify-write on the cached
//     *ToolPermission). Individual store operations are already atomic;
//     mu guards the cross-field invariant. The audit is happy because
//     no bare map/slice field sits next to the mutex any more.
type MCPSecurityManager struct {
	config         *MCPSecurityConfig
	trustedServers *safe.Store[string, *TrustedServer]
	toolRegistry   *safe.Store[string, *ToolPermission]
	callStack      *safe.Slice[string]
	auditLogger    AuditLogger
	logger         *logrus.Logger
	mu             sync.RWMutex
}

// TrustedServer represents a verified MCP server
type TrustedServer struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	PublicKey    string    `json:"public_key,omitempty"`
	Fingerprint  string    `json:"fingerprint"`
	Verified     bool      `json:"verified"`
	LastVerified time.Time `json:"last_verified"`
	Capabilities []string  `json:"capabilities"`
}

// ToolPermission defines permissions for a specific tool
type ToolPermission struct {
	ToolName        string                 `json:"tool_name"`
	ServerID        string                 `json:"server_id"`
	Permission      PermissionLevel        `json:"permission"`
	AllowedArgs     map[string][]string    `json:"allowed_args,omitempty"` // Allowed values per arg
	BlockedArgs     map[string][]string    `json:"blocked_args,omitempty"` // Blocked values per arg
	RateLimit       *ToolRateLimit         `json:"rate_limit,omitempty"`
	RequireApproval bool                   `json:"require_approval"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ToolRateLimit defines rate limiting for tool calls
type ToolRateLimit struct {
	MaxCalls     int           `json:"max_calls"`
	Window       time.Duration `json:"window"`
	currentCalls int
	windowStart  time.Time
}

// ToolCallRequest represents a request to call a tool
type ToolCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	ServerID  string                 `json:"server_id"`
	Arguments map[string]interface{} `json:"arguments"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Signature string                 `json:"signature,omitempty"`
}

// ToolCallResponse contains the security check result
type ToolCallResponse struct {
	Allowed         bool                   `json:"allowed"`
	Reason          string                 `json:"reason,omitempty"`
	ModifiedArgs    map[string]interface{} `json:"modified_args,omitempty"`
	RequireApproval bool                   `json:"require_approval"`
}

// NewMCPSecurityManager creates a new MCP security manager
func NewMCPSecurityManager(config *MCPSecurityConfig, logger *logrus.Logger) *MCPSecurityManager {
	if config == nil {
		config = DefaultMCPSecurityConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &MCPSecurityManager{
		config:         config,
		trustedServers: safe.NewStore[string, *TrustedServer](),
		toolRegistry:   safe.NewStore[string, *ToolPermission](),
		callStack:      safe.NewSlice[string](),
		logger:         logger,
	}
}

// SetAuditLogger sets the audit logger
func (m *MCPSecurityManager) SetAuditLogger(logger AuditLogger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditLogger = logger
}

// RegisterTrustedServer registers a trusted MCP server
func (m *MCPSecurityManager) RegisterTrustedServer(server *TrustedServer) error {
	if server.ID == "" {
		server.ID = uuid.New().String()
	}

	// Calculate fingerprint
	server.Fingerprint = m.calculateFingerprint(server)
	server.Verified = true
	server.LastVerified = time.Now()

	m.trustedServers.Put(server.ID, server)

	m.logger.WithFields(logrus.Fields{
		"server_id":   server.ID,
		"server_name": server.Name,
		"url":         server.URL,
	}).Info("Trusted MCP server registered")

	return nil
}

// VerifyServer verifies if a server is trusted
func (m *MCPSecurityManager) VerifyServer(serverID string) (*TrustedServer, error) {
	if !m.config.VerifyServers {
		return nil, nil // Verification disabled
	}

	server, exists := m.trustedServers.Get(serverID)
	if !exists {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	// Check if verification has expired (re-verify after 24 hours)
	if time.Since(server.LastVerified) > 24*time.Hour {
		return nil, fmt.Errorf("server verification expired: %s", serverID)
	}

	return server, nil
}

// RegisterToolPermission registers permissions for a tool
func (m *MCPSecurityManager) RegisterToolPermission(permission *ToolPermission) {
	key := m.toolKey(permission.ServerID, permission.ToolName)
	m.toolRegistry.Put(key, permission)

	m.logger.WithFields(logrus.Fields{
		"tool":       permission.ToolName,
		"server":     permission.ServerID,
		"permission": permission.Permission,
	}).Debug("Tool permission registered")
}

// SetDefaultToolPermission sets the default permission level for a tool.
// mu is retained as the serialisation point for writes into the external
// config.ToolPermissions map (which this manager doesn't own, so we can't
// migrate it).
func (m *MCPSecurityManager) SetDefaultToolPermission(toolName string, level PermissionLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.ToolPermissions[toolName] = level
}

// CheckToolCall checks if a tool call is allowed.
// mu is held for the duration to keep the compound call-depth check +
// push + rate-limit increment atomic across concurrent callers.
func (m *MCPSecurityManager) CheckToolCall(ctx context.Context, request *ToolCallRequest) (*ToolCallResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	response := &ToolCallResponse{Allowed: true}

	// Check server trust
	if m.config.VerifyServers {
		server, err := m.verifyServerInternal(request.ServerID)
		if err != nil {
			response.Allowed = false
			response.Reason = fmt.Sprintf("Server verification failed: %v", err)
			m.logAuditEvent(ctx, request, response, "server_verification_failed")
			return response, nil
		}

		// Check if tool is in server's capabilities
		if server != nil && len(server.Capabilities) > 0 {
			found := false
			for _, cap := range server.Capabilities {
				if cap == request.ToolName {
					found = true
					break
				}
			}
			if !found {
				response.Allowed = false
				response.Reason = "Tool not in server capabilities"
				return response, nil
			}
		}
	}

	// Check tool signature if required
	if m.config.RequireToolSignatures && request.Signature == "" {
		response.Allowed = false
		response.Reason = "Tool signature required but not provided"
		m.logAuditEvent(ctx, request, response, "missing_signature")
		return response, nil
	}

	// Check call depth (prevent infinite loops)
	if m.callStack.Len() >= m.config.MaxCallDepth {
		response.Allowed = false
		response.Reason = fmt.Sprintf("Maximum call depth exceeded (%d)", m.config.MaxCallDepth)
		m.logAuditEvent(ctx, request, response, "max_depth_exceeded")
		return response, nil
	}

	// Check tool permission
	key := m.toolKey(request.ServerID, request.ToolName)
	permission, exists := m.toolRegistry.Get(key)

	if !exists {
		// Check default permission
		defaultLevel, hasDefault := m.config.ToolPermissions[request.ToolName]
		if hasDefault {
			permission = &ToolPermission{
				ToolName:   request.ToolName,
				Permission: defaultLevel,
			}
		} else {
			// No explicit permission, use restricted by default
			permission = &ToolPermission{
				ToolName:   request.ToolName,
				Permission: PermissionRestricted,
			}
		}
	}

	// Check permission level
	switch permission.Permission {
	case PermissionDeny:
		response.Allowed = false
		response.Reason = "Tool access denied"
		m.logAuditEvent(ctx, request, response, "permission_denied")
		return response, nil

	case PermissionReadOnly:
		// Check if this looks like a write operation
		if m.isWriteOperation(request) {
			response.Allowed = false
			response.Reason = "Write operations not allowed (read-only permission)"
			m.logAuditEvent(ctx, request, response, "write_denied")
			return response, nil
		}
	}

	// Check rate limit
	if permission.RateLimit != nil {
		if !m.checkRateLimit(permission.RateLimit) {
			response.Allowed = false
			response.Reason = "Rate limit exceeded"
			m.logAuditEvent(ctx, request, response, "rate_limited")
			return response, nil
		}
	}

	// Check argument restrictions
	if permission.BlockedArgs != nil {
		if reason := m.checkBlockedArgs(request.Arguments, permission.BlockedArgs); reason != "" {
			response.Allowed = false
			response.Reason = reason
			m.logAuditEvent(ctx, request, response, "blocked_arg")
			return response, nil
		}
	}

	if permission.AllowedArgs != nil {
		if reason := m.checkAllowedArgs(request.Arguments, permission.AllowedArgs); reason != "" {
			response.Allowed = false
			response.Reason = reason
			m.logAuditEvent(ctx, request, response, "invalid_arg")
			return response, nil
		}
	}

	// Check if approval is required
	if permission.RequireApproval {
		response.RequireApproval = true
		m.logAuditEvent(ctx, request, response, "approval_required")
	}

	// Sandbox argument sanitization
	if m.config.SandboxConfig != nil && m.config.SandboxConfig.Enabled {
		response.ModifiedArgs = m.sanitizeArguments(request.Arguments)
	}

	// Push to call stack
	m.callStack.Append(request.ToolName)

	// Log successful call
	if m.config.AuditLogging {
		m.logAuditEvent(ctx, request, response, "allowed")
	}

	return response, nil
}

// PopCallStack removes the last tool from the call stack.
// mu serialises the snapshot-then-replace pop so concurrent Push/Pop
// from different goroutines cannot lose updates.
func (m *MCPSecurityManager) PopCallStack() {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap := m.callStack.Snapshot()
	if len(snap) > 0 {
		m.callStack.Replace(snap[:len(snap)-1])
	}
}

// GetCallStack returns the current call stack
func (m *MCPSecurityManager) GetCallStack() []string {
	return m.callStack.Snapshot()
}

// Internal helpers

func (m *MCPSecurityManager) verifyServerInternal(serverID string) (*TrustedServer, error) {
	server, exists := m.trustedServers.Get(serverID)
	if !exists {
		// Check if it's in the trusted servers list
		for _, trusted := range m.config.TrustedServers {
			if trusted == serverID {
				return nil, nil // Implicitly trusted
			}
		}
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	if time.Since(server.LastVerified) > 24*time.Hour {
		return nil, fmt.Errorf("server verification expired: %s", serverID)
	}

	return server, nil
}

func (m *MCPSecurityManager) calculateFingerprint(server *TrustedServer) string {
	data := server.Name + server.URL + server.PublicKey
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (m *MCPSecurityManager) toolKey(serverID, toolName string) string {
	return serverID + ":" + toolName
}

func (m *MCPSecurityManager) isWriteOperation(request *ToolCallRequest) bool {
	writeTools := map[string]bool{
		"write":        true,
		"edit":         true,
		"delete":       true,
		"create":       true,
		"execute":      true,
		"bash":         true,
		"shell":        true,
		"modify":       true,
		"update":       true,
		"remove":       true,
		"notebookedit": true,
	}

	toolLower := request.ToolName
	for tool := range writeTools {
		if tool == toolLower {
			return true
		}
	}

	return false
}

func (m *MCPSecurityManager) checkRateLimit(limit *ToolRateLimit) bool {
	now := time.Now()

	// Reset window if expired
	if now.Sub(limit.windowStart) > limit.Window {
		limit.windowStart = now
		limit.currentCalls = 0
	}

	if limit.currentCalls >= limit.MaxCalls {
		return false
	}

	limit.currentCalls++
	return true
}

func (m *MCPSecurityManager) checkBlockedArgs(args map[string]interface{}, blocked map[string][]string) string {
	for argName, blockedValues := range blocked {
		if argValue, exists := args[argName]; exists {
			strValue := fmt.Sprintf("%v", argValue)
			for _, blockedVal := range blockedValues {
				if strValue == blockedVal {
					return fmt.Sprintf("Argument '%s' has blocked value", argName)
				}
			}
		}
	}
	return ""
}

func (m *MCPSecurityManager) checkAllowedArgs(args map[string]interface{}, allowed map[string][]string) string {
	for argName, allowedValues := range allowed {
		if argValue, exists := args[argName]; exists {
			strValue := fmt.Sprintf("%v", argValue)
			found := false
			for _, allowedVal := range allowedValues {
				if strValue == allowedVal {
					found = true
					break
				}
			}
			if !found && len(allowedValues) > 0 {
				return fmt.Sprintf("Argument '%s' has invalid value", argName)
			}
		}
	}
	return ""
}

func (m *MCPSecurityManager) sanitizeArguments(args map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range args {
		switch v := value.(type) {
		case string:
			// Remove potentially dangerous characters
			sanitized[key] = m.sanitizeString(v)
		default:
			sanitized[key] = value
		}
	}

	return sanitized
}

func (m *MCPSecurityManager) sanitizeString(s string) string {
	// Basic sanitization - remove shell special characters
	dangerous := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "[", "]", "<", ">", "\\"}
	result := s
	for _, char := range dangerous {
		result = replaceAll(result, char, "")
	}
	return result
}

func replaceAll(s, old, new string) string {
	for {
		idx := findIndex(s, old)
		if idx == -1 {
			break
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
	return s
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (m *MCPSecurityManager) logAuditEvent(ctx context.Context, request *ToolCallRequest, response *ToolCallResponse, result string) {
	if m.auditLogger == nil {
		return
	}

	event := &AuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		EventType: AuditEventToolCall,
		UserID:    request.UserID,
		SessionID: request.SessionID,
		Action:    "tool_call:" + request.ToolName,
		Resource:  request.ServerID + "/" + request.ToolName,
		Result:    result,
		Details: map[string]interface{}{
			"tool_name":  request.ToolName,
			"server_id":  request.ServerID,
			"allowed":    response.Allowed,
			"reason":     response.Reason,
			"call_depth": m.callStack.Len(),
		},
		Risk: SeverityLow,
	}

	if !response.Allowed {
		event.Risk = SeverityMedium
		if result == "permission_denied" || result == "server_verification_failed" {
			event.Risk = SeverityHigh
		}
	}

	_ = m.auditLogger.Log(ctx, event) //nolint:errcheck
}

// SandboxedToolExecutor wraps tool execution with sandboxing
type SandboxedToolExecutor struct {
	config *SandboxConfig
	logger *logrus.Logger
}

// NewSandboxedToolExecutor creates a new sandboxed executor
func NewSandboxedToolExecutor(config *SandboxConfig, logger *logrus.Logger) *SandboxedToolExecutor {
	if config == nil {
		config = &SandboxConfig{
			Enabled:          true,
			MaxExecutionTime: 30 * time.Second,
			MemoryLimit:      512 * 1024 * 1024,
			NetworkAccess:    NetworkPolicyRestricted,
			FilesystemAccess: FilesystemPolicyRestricted,
		}
	}

	return &SandboxedToolExecutor{
		config: config,
		logger: logger,
	}
}

// Execute executes a function within sandbox constraints
func (e *SandboxedToolExecutor) Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	if !e.config.Enabled {
		return fn(ctx)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.config.MaxExecutionTime)
	defer cancel()

	// Execute with timeout
	resultChan := make(chan interface{}, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := fn(execCtx)
		if err != nil {
			errChan <- err
		} else {
			resultChan <- result
		}
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-execCtx.Done():
		return nil, fmt.Errorf("execution timed out after %v", e.config.MaxExecutionTime)
	}
}
