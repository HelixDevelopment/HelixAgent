package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/models"
)

func TestNewSecurityValidator(t *testing.T) {
	allowedPaths := []string{"/tmp/plugins", "/opt/plugins"}
	sv := NewSecurityValidator(allowedPaths)

	require.NotNil(t, sv)
	assert.Equal(t, allowedPaths, sv.allowedPaths)
	assert.Contains(t, sv.blockedFunctions, "exec")
	assert.Contains(t, sv.blockedFunctions, "syscall")
	assert.Contains(t, sv.blockedFunctions, "os/exec")
	assert.Contains(t, sv.blockedFunctions, "reflect")
	assert.Contains(t, sv.blockedFunctions, "unsafe")
}

func TestSecurityValidator_ValidatePluginPath(t *testing.T) {
	tmpDir := t.TempDir()
	sv := NewSecurityValidator([]string{tmpDir})

	t.Run("valid path in allowed directory", func(t *testing.T) {
		pluginPath := filepath.Join(tmpDir, "test-plugin.so")
		err := sv.ValidatePluginPath(pluginPath)
		assert.NoError(t, err)
	})

	t.Run("path not in allowed directories", func(t *testing.T) {
		err := sv.ValidatePluginPath("/other/path/plugin.so")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed directories")
	})

	t.Run("invalid file extension", func(t *testing.T) {
		pluginPath := filepath.Join(tmpDir, "test-plugin.dll")
		err := sv.ValidatePluginPath(pluginPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a .so file")
	})

	t.Run("subdirectory in allowed path", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "subdir")
		os.MkdirAll(subDir, 0755)
		pluginPath := filepath.Join(subDir, "nested-plugin.so")
		err := sv.ValidatePluginPath(pluginPath)
		assert.NoError(t, err)
	})

	t.Run("multiple allowed paths", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		sv2 := NewSecurityValidator([]string{tmpDir, tmpDir2})

		err := sv2.ValidatePluginPath(filepath.Join(tmpDir, "plugin.so"))
		assert.NoError(t, err)

		err = sv2.ValidatePluginPath(filepath.Join(tmpDir2, "plugin.so"))
		assert.NoError(t, err)
	})
}

func TestSecurityValidator_ValidatePluginCapabilities(t *testing.T) {
	sv := NewSecurityValidator([]string{"/tmp"})

	t.Run("valid capabilities", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("test-plugin")
		plugin.On("Capabilities").Return(&models.ProviderCapabilities{
			SupportedRequestTypes: []string{"code_generation", "reasoning"},
			Limits: models.ModelLimits{
				MaxTokens: 4096,
			},
		})

		err := sv.ValidatePluginCapabilities(plugin)
		assert.NoError(t, err)
	})

	t.Run("unsupported request type", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("bad-plugin")
		plugin.On("Capabilities").Return(&models.ProviderCapabilities{
			SupportedRequestTypes: []string{"invalid_type"},
			Limits: models.ModelLimits{
				MaxTokens: 4096,
			},
		})

		err := sv.ValidatePluginCapabilities(plugin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported request type: invalid_type")
	})

	t.Run("max tokens zero", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("zero-tokens-plugin")
		plugin.On("Capabilities").Return(&models.ProviderCapabilities{
			SupportedRequestTypes: []string{"reasoning"},
			Limits: models.ModelLimits{
				MaxTokens: 0,
			},
		})

		err := sv.ValidatePluginCapabilities(plugin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max tokens limit: 0")
	})

	t.Run("max tokens too high", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("high-tokens-plugin")
		plugin.On("Capabilities").Return(&models.ProviderCapabilities{
			SupportedRequestTypes: []string{"reasoning"},
			Limits: models.ModelLimits{
				MaxTokens: 200000,
			},
		})

		err := sv.ValidatePluginCapabilities(plugin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max tokens limit: 200000")
	})

	t.Run("all valid request types", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("full-plugin")
		plugin.On("Capabilities").Return(&models.ProviderCapabilities{
			SupportedRequestTypes: []string{"code_generation", "reasoning", "tool_use"},
			Limits: models.ModelLimits{
				MaxTokens: 128000,
			},
		})

		err := sv.ValidatePluginCapabilities(plugin)
		assert.NoError(t, err)
	})
}

func TestSecurityValidator_SandboxPlugin(t *testing.T) {
	sv := NewSecurityValidator([]string{"/tmp/plugins"})

	t.Run("successful sandbox", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("sandbox-plugin")
		plugin.On("SetSecurityContext", mock.MatchedBy(func(ctx *PluginSecurityContext) bool {
			return ctx.ResourceLimits.MaxMemoryMB == 256 &&
				ctx.ResourceLimits.MaxCPUPercent == 50 &&
				ctx.ResourceLimits.MaxFileDescriptors == 100 &&
				ctx.ResourceLimits.NetworkAccess == false
		})).Return(nil)

		err := sv.SandboxPlugin(plugin)
		assert.NoError(t, err)
		plugin.AssertExpectations(t)
	})

	t.Run("sandbox error", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("error-plugin")
		plugin.On("SetSecurityContext", mock.Anything).Return(assert.AnError)

		err := sv.SandboxPlugin(plugin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set security context")
	})
}

func TestPluginSecurityContext_Structure(t *testing.T) {
	ctx := &PluginSecurityContext{
		AllowedPaths:     []string{"/tmp", "/opt"},
		BlockedFunctions: []string{"exec", "syscall"},
		ResourceLimits: ResourceLimits{
			MaxMemoryMB:        512,
			MaxCPUPercent:      75,
			MaxFileDescriptors: 200,
			NetworkAccess:      true,
		},
	}

	assert.Len(t, ctx.AllowedPaths, 2)
	assert.Len(t, ctx.BlockedFunctions, 2)
	assert.Equal(t, 512, ctx.ResourceLimits.MaxMemoryMB)
	assert.Equal(t, 75, ctx.ResourceLimits.MaxCPUPercent)
	assert.Equal(t, 200, ctx.ResourceLimits.MaxFileDescriptors)
	assert.True(t, ctx.ResourceLimits.NetworkAccess)
}

func TestResourceLimits_Structure(t *testing.T) {
	limits := ResourceLimits{
		MaxMemoryMB:        1024,
		MaxCPUPercent:      100,
		MaxFileDescriptors: 500,
		NetworkAccess:      false,
	}

	assert.Equal(t, 1024, limits.MaxMemoryMB)
	assert.Equal(t, 100, limits.MaxCPUPercent)
	assert.Equal(t, 500, limits.MaxFileDescriptors)
	assert.False(t, limits.NetworkAccess)
}
