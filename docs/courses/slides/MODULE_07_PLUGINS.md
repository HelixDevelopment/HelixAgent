# Module 7: Plugin Development

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 7: Plugin Development
- Duration: 75 minutes
- Extending HelixAgent Functionality

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand the plugin architecture
- Develop custom plugins
- Implement hot-reloadable extensions
- Deploy and manage plugins

---

## Slide 3: Plugin System Overview

**Why Plugins?**

- Extend functionality without modifying core
- Hot-reload without server restart
- Modular, maintainable architecture
- Community contributions
- Custom business logic integration

---

## Slide 4: Plugin Architecture

**System Components:**

```
+------------------+
|  Plugin Manager  |
+--------+---------+
         |
   +-----+-----+
   |           |
+--v--+     +--v--+
|Loader|     |Registry|
+--+--+     +---+---+
   |           |
   +-----+-----+
         |
   +-----v-----+
   |  Plugins  |
   +-----------+
   | Plugin A  |
   | Plugin B  |
   | Plugin C  |
   +-----------+
```

---

## Slide 5: Plugin Interfaces

**Core Interfaces:**

```go
// Plugin interface
type Plugin interface {
    Name() string
    Version() string
    Initialize(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    HealthCheck(ctx context.Context) error
}

// PluginRegistry interface
type PluginRegistry interface {
    Register(plugin Plugin) error
    Unregister(name string) error
    Get(name string) (Plugin, error)
    List() []Plugin
}
```

---

## Slide 6: Plugin Loader

**Loading Mechanism:**

```go
// PluginLoader interface
type PluginLoader interface {
    // Load plugin from path
    Load(path string) (Plugin, error)

    // Unload plugin
    Unload(name string) error

    // Reload plugin (hot-reload)
    Reload(name string) error

    // Watch for changes
    Watch(dir string) error
}
```

---

## Slide 7: Plugin Lifecycle

**Plugin States:**

```
+----------+     +------------+     +----------+
| Loaded   | --> | Initialized| --> | Running  |
+----------+     +------------+     +----------+
     |                                    |
     v                                    v
+----------+     +------------+     +----------+
| Error    | <-- | Stopping   | <-- | Stopped  |
+----------+     +------------+     +----------+
```

---

## Slide 8: Creating a Plugin

**Basic Plugin Structure:**

```go
package myplugin

import "context"

type MyPlugin struct {
    name    string
    version string
    config  *Config
}

func New(config *Config) *MyPlugin {
    return &MyPlugin{
        name:    "my-plugin",
        version: "1.0.0",
        config:  config,
    }
}
```

---

## Slide 9: Implementing Plugin Interface

**Required Methods:**

```go
func (p *MyPlugin) Name() string {
    return p.name
}

func (p *MyPlugin) Version() string {
    return p.version
}

func (p *MyPlugin) Initialize(ctx context.Context) error {
    // Setup resources, connections, etc.
    return nil
}

func (p *MyPlugin) Start(ctx context.Context) error {
    // Start background tasks
    return nil
}
```

---

## Slide 10: Stop and Health Check

**Cleanup and Monitoring:**

```go
func (p *MyPlugin) Stop(ctx context.Context) error {
    // Cleanup resources
    // Close connections
    return nil
}

func (p *MyPlugin) HealthCheck(ctx context.Context) error {
    // Verify plugin is healthy
    if !p.isHealthy() {
        return errors.New("plugin unhealthy")
    }
    return nil
}
```

---

## Slide 11: Plugin Configuration

**Configuration Structure:**

```go
type Config struct {
    // Plugin-specific settings
    APIEndpoint  string        `yaml:"api_endpoint"`
    Timeout      time.Duration `yaml:"timeout"`
    MaxRetries   int           `yaml:"max_retries"`
    Enabled      bool          `yaml:"enabled"`
}

// Load configuration
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    return &cfg, yaml.Unmarshal(data, &cfg)
}
```

---

## Slide 12: Plugin Directory Structure

**Recommended Layout:**

```
plugins/
  +-- my-plugin/
  |     +-- plugin.go       # Main plugin code
  |     +-- config.go       # Configuration
  |     +-- handlers.go     # HTTP handlers
  |     +-- service.go      # Business logic
  |     +-- plugin_test.go  # Tests
  |     +-- config.yaml     # Default config
  |     +-- README.md       # Documentation
```

---

## Slide 13: Adding HTTP Handlers

**Extending API:**

```go
type MyPlugin struct {
    // ... fields
    router *gin.RouterGroup
}

func (p *MyPlugin) RegisterRoutes(r *gin.RouterGroup) {
    p.router = r.Group("/my-plugin")
    p.router.GET("/status", p.handleStatus)
    p.router.POST("/action", p.handleAction)
}

func (p *MyPlugin) handleStatus(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "ok",
        "version": p.version,
    })
}
```

---

## Slide 14: Hot Reloading

**Enabling Hot Reload:**

```yaml
plugins:
  hot_reload:
    enabled: true
    watch_interval: 5s
    directories:
      - ./plugins
```

**How it works:**
1. File system watcher detects changes
2. Plugin gracefully stopped
3. New version loaded
4. Plugin restarted
5. Zero downtime

---

## Slide 15: Dependency Resolution

**Managing Dependencies:**

```go
type PluginMetadata struct {
    Name         string   `yaml:"name"`
    Version      string   `yaml:"version"`
    Dependencies []string `yaml:"dependencies"`
    Provides     []string `yaml:"provides"`
}

// Dependencies are loaded in order
// Circular dependencies are detected
```

---

## Slide 16: Plugin Communication

**Inter-Plugin Messaging:**

```go
// Publish event
p.eventBus.Publish("my-plugin.event", eventData)

// Subscribe to events
p.eventBus.Subscribe("other-plugin.event",
    func(data interface{}) {
        // Handle event
    },
)
```

---

## Slide 17: Error Handling

**Graceful Error Handling:**

```go
func (p *MyPlugin) Initialize(ctx context.Context) error {
    if err := p.validateConfig(); err != nil {
        return fmt.Errorf("config validation: %w", err)
    }

    if err := p.connectToService(); err != nil {
        // Retry with backoff
        return p.retryConnect(ctx, err)
    }

    return nil
}

func (p *MyPlugin) retryConnect(ctx context.Context, err error) error {
    backoff := time.Second
    for i := 0; i < p.config.MaxRetries; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
            if err := p.connectToService(); err == nil {
                return nil
            }
            backoff *= 2
        }
    }
    return fmt.Errorf("failed after %d retries: %w",
        p.config.MaxRetries, err)
}
```

---

## Slide 18: Plugin Metrics

**Adding Metrics:**

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "myplugin_requests_total",
            Help: "Total requests processed",
        },
        []string{"method", "status"},
    )
)

func init() {
    prometheus.MustRegister(requestCounter)
}

func (p *MyPlugin) handleAction(c *gin.Context) {
    requestCounter.WithLabelValues("POST", "success").Inc()
    // Handle request
}
```

---

## Slide 19: Testing Plugins

**Unit Testing:**

```go
func TestPlugin_Initialize(t *testing.T) {
    cfg := &Config{
        APIEndpoint: "http://localhost:7061",
        Timeout:     30 * time.Second,
        Enabled:     true,
    }

    plugin := New(cfg)
    ctx := context.Background()

    err := plugin.Initialize(ctx)
    assert.NoError(t, err)

    err = plugin.HealthCheck(ctx)
    assert.NoError(t, err)

    err = plugin.Stop(ctx)
    assert.NoError(t, err)
}
```

---

## Slide 20: Integration Testing

**Testing with HelixAgent:**

```go
func TestPlugin_Integration(t *testing.T) {
    // Start HelixAgent with plugin
    app := testutil.NewTestApp(t)
    app.LoadPlugin(t, "./plugins/my-plugin")

    // Test plugin endpoint
    resp := app.GET("/api/v1/my-plugin/status")
    assert.Equal(t, 200, resp.Code)

    var result map[string]interface{}
    json.Unmarshal(resp.Body.Bytes(), &result)
    assert.Equal(t, "ok", result["status"])
}
```

---

## Slide 21: Plugin Deployment

**Deployment Options:**

| Method | Use Case |
|--------|----------|
| Bundled | Core plugins, always needed |
| Directory | Dynamic loading, hot reload |
| Remote | Distributed deployment |
| Container | Isolated execution |

---

## Slide 22: Plugin Security

**Security Considerations:**

1. **Input Validation**: Validate all inputs
2. **Access Control**: Limit plugin permissions
3. **Sandboxing**: Isolate plugin execution
4. **Audit Logging**: Log plugin activities
5. **Code Review**: Review plugin code

```go
// Example: validate input
func (p *MyPlugin) handleAction(c *gin.Context) {
    var req ActionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }
    // Sanitize and validate req
}
```

---

## Slide 23: Example: Response Enhancer

**Complete Plugin Example:**

```go
package enhancer

type EnhancerPlugin struct {
    name    string
    version string
    client  *http.Client
}

func (p *EnhancerPlugin) EnhanceResponse(
    resp *llm.Response,
) (*llm.Response, error) {
    // Add metadata
    resp.Metadata["enhanced"] = true
    resp.Metadata["enhancer_version"] = p.version

    // Post-process content
    resp.Content = p.postProcess(resp.Content)

    return resp, nil
}
```

---

## Slide 24: Plugin Best Practices

**Development Guidelines:**

| Practice | Description |
|----------|-------------|
| Idempotent | Safe to restart |
| Stateless | No local state dependencies |
| Configurable | Externalize configuration |
| Observable | Metrics and logging |
| Testable | Comprehensive tests |

---

## Slide 25: Hands-On Lab

**Lab Exercise 7.1: Plugin Development**

Tasks:
1. Create a simple plugin skeleton
2. Implement Plugin interface
3. Add HTTP handlers
4. Write unit tests
5. Deploy and test hot reload

Time: 35 minutes

---

## Slide 26: Module Summary

**Key Takeaways:**

- Plugins extend HelixAgent without core changes
- Implement Plugin interface for custom functionality
- Hot-reload enables zero-downtime updates
- Dependency resolution for plugin ordering
- Comprehensive testing essential
- Security considerations critical

**Next: Module 8 - MCP/LSP Integration**

---

## Speaker Notes

### Slide 5 Notes
Explain each interface and its purpose. PluginRegistry manages known plugins, PluginLoader handles loading/unloading.

### Slide 14 Notes
Demonstrate hot reload live if possible. Show a plugin being modified and automatically reloaded.

### Slide 22 Notes
Security is critical for plugins. Untrusted plugins can compromise the entire system. Discuss the principle of least privilege.
