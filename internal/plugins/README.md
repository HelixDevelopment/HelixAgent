# Plugins Package

The `plugins` package provides a hot-reloadable plugin system for extending HelixAgent functionality with dependency resolution, versioning, and security sandboxing.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Plugin System                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Discovery  │  │    Loader    │  │      Registry        │  │
│  │              │  │              │  │                      │  │
│  │  Scan dirs   │  │  Load .so    │  │  Plugin metadata     │  │
│  │  Find plugins│  │  Init plugin │  │  Version tracking    │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Dependencies │  │  Hot Reload  │  │      Security        │  │
│  │              │  │              │  │                      │  │
│  │  Resolution  │  │  File watch  │  │  Sandboxing          │  │
│  │  Load order  │  │  Auto reload │  │  Permission check    │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  Lifecycle   │  │  Versioning  │  │      Metrics         │  │
│  │              │  │              │  │                      │  │
│  │  Init/Start  │  │  Semver      │  │  Load times          │  │
│  │  Stop/Unload │  │  Compat check│  │  Call counts         │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Plugin Interface

All plugins implement:

```go
type Plugin interface {
    // Info returns plugin metadata
    Info() PluginInfo

    // Init initializes the plugin
    Init(ctx context.Context, config map[string]interface{}) error

    // Start starts the plugin
    Start(ctx context.Context) error

    // Stop stops the plugin
    Stop(ctx context.Context) error

    // HealthCheck returns plugin health
    HealthCheck(ctx context.Context) error
}

type PluginInfo struct {
    Name         string   `json:"name"`
    Version      string   `json:"version"`
    Description  string   `json:"description"`
    Author       string   `json:"author"`
    Dependencies []string `json:"dependencies"`
    Capabilities []string `json:"capabilities"`
}
```

## Components

### Discovery (`discovery.go`)

Find available plugins:

```go
discovery := plugins.NewDiscovery(config)

// Scan plugin directories
found, err := discovery.Scan(ctx, pluginDirs)

// Get plugin metadata without loading
info, err := discovery.GetInfo(pluginPath)
```

### Loader (`loader.go`)

Load and initialize plugins:

```go
loader := plugins.NewLoader(config, logger)

// Load a plugin
plugin, err := loader.Load(ctx, pluginPath)

// Unload a plugin
err = loader.Unload(ctx, pluginName)
```

### Registry (`registry.go`)

Track loaded plugins:

```go
registry := plugins.NewRegistry()

// Register plugin
registry.Register(plugin)

// Get plugin
plugin, exists := registry.Get("analyzer")

// List all plugins
all := registry.List()
```

### Dependencies (`dependencies.go`)

Resolve plugin dependencies:

```go
resolver := plugins.NewDependencyResolver(registry)

// Get load order
order, err := resolver.ResolveOrder(pluginNames)

// Check if dependencies satisfied
satisfied, missing := resolver.CheckDependencies(plugin)
```

### Hot Reload (`hot_reload.go`)

Automatic plugin reloading:

```go
reloader := plugins.NewHotReloader(loader, config, logger)

// Start watching for changes
err := reloader.Start(ctx)

// Add directory to watch
reloader.Watch(pluginDir)

// Stop watching
reloader.Stop()
```

### Watcher (`watcher.go`)

File system monitoring:

```go
watcher := plugins.NewWatcher(config)

// Watch directory
err := watcher.Add(directory)

// Get events channel
events := watcher.Events()
```

### Security (`security.go`)

Plugin sandboxing:

```go
sandbox := plugins.NewSandbox(config)

// Check plugin permissions
allowed := sandbox.CheckPermission(plugin, "network")

// Run in sandbox
result, err := sandbox.Execute(ctx, plugin, func() error {
    return plugin.Start(ctx)
})
```

### Versioning (`versioning.go`)

Semantic versioning support:

```go
versioner := plugins.NewVersioner()

// Check compatibility
compatible := versioner.IsCompatible("1.2.3", ">=1.0.0")

// Compare versions
result := versioner.Compare("2.0.0", "1.5.0") // returns 1

// Get latest version
latest := versioner.Latest(versions)
```

### Lifecycle (`lifecycle.go`)

Plugin lifecycle management:

```go
lifecycle := plugins.NewLifecycleManager(loader, registry, logger)

// Start all plugins
err := lifecycle.StartAll(ctx)

// Stop all plugins
err := lifecycle.StopAll(ctx)

// Restart plugin
err := lifecycle.Restart(ctx, "analyzer")
```

### Health (`health.go`)

Plugin health monitoring:

```go
health := plugins.NewHealthChecker(registry, config)

// Check all plugins
statuses := health.CheckAll(ctx)

// Check specific plugin
status, err := health.Check(ctx, "analyzer")
```

### Metrics (`metrics.go`)

Plugin performance metrics:

```go
metrics := plugins.NewMetrics()

// Record plugin load
metrics.RecordLoad(pluginName, duration)

// Record plugin call
metrics.RecordCall(pluginName, method, duration)

// Get statistics
stats := metrics.GetStats(pluginName)
```

### Config (`config.go`)

Plugin configuration:

```go
config := plugins.Config{
    PluginDirs:     []string{"./plugins", "/etc/helix/plugins"},
    HotReload:      true,
    WatchInterval:  time.Second,
    LoadTimeout:    30 * time.Second,
    SandboxEnabled: true,
}
```

## Files

| File | Description |
|------|-------------|
| `plugin.go` | Plugin interface definition |
| `registry.go` | Plugin registry |
| `loader.go` | Plugin loading |
| `discovery.go` | Plugin discovery |
| `dependencies.go` | Dependency resolution |
| `hot_reload.go` | Hot reloading |
| `watcher.go` | File watching |
| `reload.go` | Reload logic |
| `security.go` | Sandboxing |
| `versioning.go` | Version management |
| `lifecycle.go` | Lifecycle management |
| `health.go` | Health checking |
| `metrics.go` | Performance metrics |
| `config.go` | Configuration |

## Usage

### Loading Plugins

```go
// Create plugin system
system := plugins.NewPluginSystem(config, logger)

// Discover and load all plugins
err := system.LoadAll(ctx)

// Get a specific plugin
analyzer, _ := system.Get("analyzer")

// Execute plugin functionality
result, err := analyzer.Execute(ctx, params)
```

### Creating a Plugin

```go
type MyPlugin struct {
    config map[string]interface{}
}

func (p *MyPlugin) Info() plugins.PluginInfo {
    return plugins.PluginInfo{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "My custom plugin",
    }
}

func (p *MyPlugin) Init(ctx context.Context, config map[string]interface{}) error {
    p.config = config
    return nil
}

func (p *MyPlugin) Start(ctx context.Context) error {
    // Start plugin operations
    return nil
}

func (p *MyPlugin) Stop(ctx context.Context) error {
    // Cleanup resources
    return nil
}

func (p *MyPlugin) HealthCheck(ctx context.Context) error {
    return nil
}
```

## Testing

```bash
go test -v ./internal/plugins/...
```

Tests cover:
- Plugin loading and unloading
- Dependency resolution
- Hot reload functionality
- Version compatibility
- Security sandboxing
- Lifecycle management
- Health checking
- Concurrent access
