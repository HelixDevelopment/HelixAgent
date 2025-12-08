// Plugin interface definitions for LLM providers
package plugins

import (
	"context"

	"github.com/superagent/superagent/internal/models"
)

// LLMPlugin defines the interface for pluggable LLM providers
type LLMPlugin interface {
	// Core operations
	Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)

	// Metadata
	Name() string
	Version() string
	Capabilities() *models.ProviderCapabilities

	// Lifecycle
	Init(config map[string]interface{}) error
	Shutdown(ctx context.Context) error

	// Health
	HealthCheck(ctx context.Context) error
}

// PluginRegistry manages plugin loading and registration
type PluginRegistry interface {
	Register(plugin LLMPlugin) error
	Unregister(name string) error
	Get(name string) (LLMPlugin, bool)
	List() []string
}

// PluginLoader handles dynamic loading of plugins
type PluginLoader interface {
	Load(path string) (LLMPlugin, error)
	Unload(name string) error
}
