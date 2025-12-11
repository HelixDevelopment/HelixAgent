package plugins

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/superagent/superagent/internal/utils"
)

// SecurityValidator provides sandboxing and security validation for plugins
type SecurityValidator struct {
	allowedPaths     []string
	blockedFunctions []string
}

// PluginSecurityContext provides security constraints for plugins
type PluginSecurityContext struct {
	AllowedPaths     []string
	BlockedFunctions []string
	ResourceLimits   ResourceLimits
}

// ResourceLimits defines resource constraints for plugins
type ResourceLimits struct {
	MaxMemoryMB        int
	MaxCPUPercent      int
	MaxFileDescriptors int
	NetworkAccess      bool
}

func NewSecurityValidator(allowedPaths []string) *SecurityValidator {
	return &SecurityValidator{
		allowedPaths: allowedPaths,
		blockedFunctions: []string{
			"exec",
			"syscall",
			"os/exec",
			"reflect",
			"unsafe",
		},
	}
}

func (s *SecurityValidator) ValidatePluginPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid plugin path: %w", err)
	}

	// Check if path is in allowed directories
	allowed := false
	for _, allowedPath := range s.allowedPaths {
		if strings.HasPrefix(absPath, allowedPath) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("plugin path %s not in allowed directories", absPath)
	}

	// Check file extension
	if !strings.HasSuffix(path, ".so") {
		return fmt.Errorf("plugin must be a .so file")
	}

	return nil
}

func (s *SecurityValidator) ValidatePluginCapabilities(plugin LLMPlugin) error {
	caps := plugin.Capabilities()

	// Validate supported request types
	validTypes := []string{"code_generation", "reasoning", "tool_use"}
	for _, reqType := range caps.SupportedRequestTypes {
		valid := false
		for _, validType := range validTypes {
			if reqType == validType {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unsupported request type: %s", reqType)
		}
	}

	// Validate limits
	if caps.Limits.MaxTokens <= 0 || caps.Limits.MaxTokens > 128000 {
		return fmt.Errorf("invalid max tokens limit: %d", caps.Limits.MaxTokens)
	}

	utils.GetLogger().Infof("Validated plugin %s security constraints", plugin.Name())
	return nil
}

func (s *SecurityValidator) SandboxPlugin(plugin LLMPlugin) error {
	utils.GetLogger().Infof("Setting up sandbox for plugin %s", plugin.Name())

	// Create plugin-specific security context
	securityContext := &PluginSecurityContext{
		AllowedPaths:     s.allowedPaths,
		BlockedFunctions: s.blockedFunctions,
		ResourceLimits: ResourceLimits{
			MaxMemoryMB:        256,
			MaxCPUPercent:      50,
			MaxFileDescriptors: 100,
			NetworkAccess:      false,
		},
	}

	// Register security context with plugin
	if err := plugin.SetSecurityContext(securityContext); err != nil {
		return fmt.Errorf("failed to set security context: %w", err)
	}

	utils.GetLogger().Infof("Successfully sandboxed plugin %s", plugin.Name())
	return nil
}
