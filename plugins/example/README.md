# Example LLM Provider Plugin

This is a template for creating LLM provider plugins for SuperAgent.

## Building the Plugin

```bash
go build -buildmode=plugin -o example.so plugin.go
```

## Plugin Interface

The plugin must implement the `plugins.LLMPlugin` interface:

- `Name() string` - Returns the plugin name
- `Version() string` - Returns the plugin version
- `Capabilities() *models.ProviderCapabilities` - Returns plugin capabilities
- `Init(config map[string]interface{}) error` - Initialize with configuration
- `Shutdown(ctx context.Context) error` - Cleanup resources
- `HealthCheck(ctx context.Context) error` - Health check
- `Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)` - Synchronous completion
- `CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)` - Streaming completion

## Configuration

Plugins receive configuration through the `Init` method. Configuration is loaded from JSON files in the plugin config directory.

Example config (`example.json`):
```json
{
  "api_key": "your-api-key",
  "base_url": "https://api.example.com",
  "timeout": "30s",
  "max_retries": 3
}
```

## Loading the Plugin

Plugins are automatically discovered and loaded from configured plugin directories. The plugin file must have a `.so` extension and export a `Plugin` variable of type `plugins.LLMPlugin`.

## Security

Plugins run in the same process as the main application. Ensure proper validation of inputs and outputs to prevent security issues.