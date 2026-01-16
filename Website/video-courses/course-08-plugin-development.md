# Video Course 08: Plugin Development Deep Dive

## Course Overview

**Duration:** 4.5 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 04 (Custom Integration), Go programming experience

Master HelixAgent plugin development, from basic plugins to advanced hot-reloadable systems with dependency resolution.

---

## Module 1: Plugin Architecture

### Video 1.1: Plugin System Overview (25 min)

**Topics:**
- Plugin architecture design
- Plugin interface contracts
- Hot-reload mechanism
- Dependency resolution

**Architecture:**
```
┌─────────────────────────────────────────────────────────────┐
│                     Plugin Registry                          │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐  │
│  │                    Plugin Loader                      │  │
│  │  - Load() error                                      │  │
│  │  - Unload() error                                    │  │
│  │  - Reload() error                                    │  │
│  │  - Watch() <-chan PluginEvent                        │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│  ┌────────┬────────┬────────┬────────┬────────┐           │
│  │Plugin A│Plugin B│Plugin C│Plugin D│Plugin E│           │
│  └────────┴────────┴────────┴────────┴────────┘           │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Dependency Graph Manager                 │  │
│  │  - ResolveDependencies(plugin)                       │  │
│  │  - GetLoadOrder() []Plugin                           │  │
│  │  - DetectCycles() error                              │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Video 1.2: Plugin Interface (20 min)

**Topics:**
- Plugin interface definition
- Lifecycle methods
- Configuration handling
- Error management

**Core Interface:**
```go
// internal/plugins/plugin.go
type Plugin interface {
    // Metadata
    Name() string
    Version() string
    Description() string

    // Lifecycle
    Init(ctx context.Context, config map[string]interface{}) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error

    // Dependencies
    Dependencies() []string

    // Health
    HealthCheck(ctx context.Context) error
}
```

**Extended Interface:**
```go
type ProviderPlugin interface {
    Plugin
    GetProvider() LLMProvider
}

type MiddlewarePlugin interface {
    Plugin
    GetMiddleware() gin.HandlerFunc
}

type ToolPlugin interface {
    Plugin
    GetTools() []Tool
}
```

### Video 1.3: Plugin Configuration (20 min)

**Topics:**
- YAML configuration
- Environment variables
- Runtime configuration
- Validation

**Configuration Example:**
```yaml
# configs/plugins.yaml
plugins:
  custom-provider:
    enabled: true
    priority: 10
    config:
      api_url: "https://api.custom.ai"
      timeout: 30s
      retry_count: 3

  rate-limiter:
    enabled: true
    config:
      requests_per_minute: 100
      burst_size: 20

  custom-logger:
    enabled: true
    depends_on:
      - rate-limiter
    config:
      log_level: debug
      format: json
```

---

## Module 2: Building Your First Plugin

### Video 2.1: Plugin Project Structure (15 min)

**Topics:**
- Directory layout
- File organization
- Build configuration
- Testing setup

**Project Structure:**
```
plugins/
├── my-plugin/
│   ├── plugin.go          # Main plugin implementation
│   ├── config.go          # Configuration handling
│   ├── handlers.go        # HTTP handlers (if any)
│   ├── models.go          # Data models
│   ├── plugin_test.go     # Tests
│   └── README.md          # Documentation
├── another-plugin/
│   └── ...
└── registry.go            # Plugin registration
```

### Video 2.2: Implementing Plugin Interface (30 min)

**Topics:**
- Basic plugin implementation
- Configuration parsing
- Lifecycle management
- Error handling

**Complete Example:**
```go
// plugins/custom-logger/plugin.go
package customlogger

import (
    "context"
    "fmt"
    "log"
)

type CustomLoggerPlugin struct {
    name    string
    version string
    config  *Config
    logger  *log.Logger
}

type Config struct {
    LogLevel string `yaml:"log_level"`
    Format   string `yaml:"format"`
    Output   string `yaml:"output"`
}

func New() *CustomLoggerPlugin {
    return &CustomLoggerPlugin{
        name:    "custom-logger",
        version: "1.0.0",
    }
}

func (p *CustomLoggerPlugin) Name() string {
    return p.name
}

func (p *CustomLoggerPlugin) Version() string {
    return p.version
}

func (p *CustomLoggerPlugin) Description() string {
    return "Custom logging plugin with structured output"
}

func (p *CustomLoggerPlugin) Dependencies() []string {
    return []string{} // No dependencies
}

func (p *CustomLoggerPlugin) Init(ctx context.Context, rawConfig map[string]interface{}) error {
    // Parse configuration
    config, err := parseConfig(rawConfig)
    if err != nil {
        return fmt.Errorf("failed to parse config: %w", err)
    }
    p.config = config

    // Initialize logger based on config
    p.logger = log.New(os.Stdout, "[PLUGIN] ", log.LstdFlags)

    return nil
}

func (p *CustomLoggerPlugin) Start(ctx context.Context) error {
    p.logger.Println("Custom logger plugin started")
    return nil
}

func (p *CustomLoggerPlugin) Stop(ctx context.Context) error {
    p.logger.Println("Custom logger plugin stopped")
    return nil
}

func (p *CustomLoggerPlugin) HealthCheck(ctx context.Context) error {
    if p.logger == nil {
        return fmt.Errorf("logger not initialized")
    }
    return nil
}
```

### Video 2.3: Registering and Loading Plugins (20 min)

**Topics:**
- Plugin registration
- Manual loading
- Auto-discovery
- Load ordering

**Registration:**
```go
// plugins/registry.go
package plugins

import (
    "dev.helix.agent/plugins/customlogger"
    "dev.helix.agent/plugins/customprovider"
)

func RegisterAll(registry *PluginRegistry) {
    // Register built-in plugins
    registry.Register(customlogger.New())
    registry.Register(customprovider.New())

    // Auto-discover plugins in directory
    registry.AutoDiscover("./plugins")
}
```

**Loading:**
```go
// internal/plugins/loader.go
func (l *PluginLoader) LoadAll(ctx context.Context) error {
    // Get load order based on dependencies
    order := l.registry.GetLoadOrder()

    for _, plugin := range order {
        if err := l.loadPlugin(ctx, plugin); err != nil {
            return fmt.Errorf("failed to load %s: %w", plugin.Name(), err)
        }
    }

    return nil
}

func (l *PluginLoader) loadPlugin(ctx context.Context, plugin Plugin) error {
    // Initialize
    if err := plugin.Init(ctx, l.getConfig(plugin.Name())); err != nil {
        return err
    }

    // Start
    if err := plugin.Start(ctx); err != nil {
        return err
    }

    l.loaded[plugin.Name()] = plugin
    return nil
}
```

---

## Module 3: Provider Plugins

### Video 3.1: LLM Provider Plugin (35 min)

**Topics:**
- Provider interface implementation
- API integration
- Response transformation
- Tool support

**Complete Provider Plugin:**
```go
// plugins/custom-provider/plugin.go
package customprovider

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "dev.helix.agent/internal/models"
)

type CustomProviderPlugin struct {
    name     string
    version  string
    config   *Config
    client   *http.Client
    provider *CustomProvider
}

type Config struct {
    APIURL  string `yaml:"api_url"`
    APIKey  string `yaml:"api_key"`
    Timeout int    `yaml:"timeout"`
}

type CustomProvider struct {
    config *Config
    client *http.Client
}

func (p *CustomProviderPlugin) GetProvider() models.LLMProvider {
    return p.provider
}

// Provider implementation
func (cp *CustomProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
    // Build API request
    apiReq := &customAPIRequest{
        Model:    req.Model,
        Messages: transformMessages(req.Messages),
    }

    body, _ := json.Marshal(apiReq)

    httpReq, _ := http.NewRequestWithContext(ctx, "POST",
        cp.config.APIURL+"/chat/completions",
        bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+cp.config.APIKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := cp.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("API request failed: %w", err)
    }
    defer resp.Body.Close()

    var apiResp customAPIResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, err
    }

    return &models.LLMResponse{
        Content:    apiResp.Choices[0].Message.Content,
        Model:      apiResp.Model,
        Confidence: calculateConfidence(apiResp),
        TokensUsed: apiResp.Usage.TotalTokens,
    }, nil
}

func (cp *CustomProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.StreamChunk, error) {
    chunks := make(chan *models.StreamChunk)

    go func() {
        defer close(chunks)
        // Implement SSE streaming
        // ...
    }()

    return chunks, nil
}

func (cp *CustomProvider) HealthCheck(ctx context.Context) error {
    resp, err := cp.client.Get(cp.config.APIURL + "/health")
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: %d", resp.StatusCode)
    }
    return nil
}

func (cp *CustomProvider) GetCapabilities() *models.ProviderCapabilities {
    return &models.ProviderCapabilities{
        SupportsStreaming: true,
        SupportsTools:     true,
        MaxTokens:         8192,
        Models: []string{
            "custom-model-v1",
            "custom-model-v2",
        },
    }
}
```

### Video 3.2: Tool Support in Providers (25 min)

**Topics:**
- Tool schema definition
- Function calling
- Tool response handling
- Multi-turn tool usage

**Tool Implementation:**
```go
type CustomTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
}

type CustomToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"`
    } `json:"function"`
}

func (cp *CustomProvider) CompleteWithTools(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
    apiReq := &customAPIRequest{
        Model:    req.Model,
        Messages: transformMessages(req.Messages),
        Tools:    transformTools(req.Tools),
    }

    // ... API call

    response := &models.LLMResponse{
        Content: apiResp.Choices[0].Message.Content,
    }

    // Handle tool calls
    if len(apiResp.Choices[0].Message.ToolCalls) > 0 {
        response.ToolCalls = transformToolCalls(apiResp.Choices[0].Message.ToolCalls)
    }

    return response, nil
}
```

---

## Module 4: Middleware Plugins

### Video 4.1: HTTP Middleware Plugin (25 min)

**Topics:**
- Gin middleware integration
- Request/response modification
- Context enhancement
- Chain ordering

**Middleware Plugin:**
```go
// plugins/request-logger/plugin.go
package requestlogger

type RequestLoggerPlugin struct {
    name    string
    config  *Config
}

type Config struct {
    LogBody    bool     `yaml:"log_body"`
    LogHeaders []string `yaml:"log_headers"`
    SkipPaths  []string `yaml:"skip_paths"`
}

func (p *RequestLoggerPlugin) GetMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        // Skip configured paths
        for _, skip := range p.config.SkipPaths {
            if strings.HasPrefix(path, skip) {
                c.Next()
                return
            }
        }

        // Log request
        log.Printf("[REQUEST] %s %s", c.Request.Method, path)

        // Capture response
        blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
        c.Writer = blw

        c.Next()

        // Log response
        latency := time.Since(start)
        log.Printf("[RESPONSE] %s %s %d %v",
            c.Request.Method, path, c.Writer.Status(), latency)
    }
}
```

### Video 4.2: Authentication Plugin (30 min)

**Topics:**
- Custom auth schemes
- Token validation
- Role-based access
- Session management

**Auth Plugin:**
```go
// plugins/custom-auth/plugin.go
package customauth

type CustomAuthPlugin struct {
    name      string
    config    *Config
    validator TokenValidator
}

type Config struct {
    Issuer       string   `yaml:"issuer"`
    Audience     string   `yaml:"audience"`
    PublicKeyURL string   `yaml:"public_key_url"`
    AllowedRoles []string `yaml:"allowed_roles"`
}

func (p *CustomAuthPlugin) GetMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token
        token := extractBearerToken(c.GetHeader("Authorization"))
        if token == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "missing authorization token",
            })
            return
        }

        // Validate token
        claims, err := p.validator.Validate(token)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid token",
            })
            return
        }

        // Check roles
        if !p.hasAllowedRole(claims.Roles) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "insufficient permissions",
            })
            return
        }

        // Add claims to context
        c.Set("user_claims", claims)
        c.Next()
    }
}
```

---

## Module 5: Hot Reload System

### Video 5.1: File Watching and Detection (25 min)

**Topics:**
- File system watching
- Change detection
- Debouncing
- Event handling

**File Watcher:**
```go
// internal/plugins/watcher.go
type PluginWatcher struct {
    watcher  *fsnotify.Watcher
    registry *PluginRegistry
    loader   *PluginLoader
    debounce time.Duration
}

func (pw *PluginWatcher) Watch(ctx context.Context) error {
    for {
        select {
        case event, ok := <-pw.watcher.Events:
            if !ok {
                return nil
            }

            if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
                pw.handleChange(ctx, event.Name)
            }

        case err, ok := <-pw.watcher.Errors:
            if !ok {
                return nil
            }
            log.Printf("Watcher error: %v", err)

        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (pw *PluginWatcher) handleChange(ctx context.Context, path string) {
    // Debounce
    time.Sleep(pw.debounce)

    pluginName := extractPluginName(path)
    if err := pw.loader.Reload(ctx, pluginName); err != nil {
        log.Printf("Failed to reload plugin %s: %v", pluginName, err)
    }
}
```

### Video 5.2: Safe Reload Mechanism (30 min)

**Topics:**
- Graceful shutdown
- State preservation
- Connection draining
- Rollback on failure

**Safe Reload:**
```go
// internal/plugins/loader.go
func (l *PluginLoader) Reload(ctx context.Context, name string) error {
    l.mu.Lock()
    defer l.mu.Unlock()

    current, exists := l.loaded[name]
    if !exists {
        return fmt.Errorf("plugin %s not loaded", name)
    }

    // Create new instance
    newPlugin := l.registry.Get(name).Clone()

    // Initialize new instance
    if err := newPlugin.Init(ctx, l.getConfig(name)); err != nil {
        return fmt.Errorf("init failed: %w", err)
    }

    // Start new instance
    if err := newPlugin.Start(ctx); err != nil {
        return fmt.Errorf("start failed: %w", err)
    }

    // Stop old instance (with timeout)
    stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := current.Stop(stopCtx); err != nil {
        // Rollback: stop new, keep old
        newPlugin.Stop(ctx)
        return fmt.Errorf("stop old failed: %w", err)
    }

    // Swap
    l.loaded[name] = newPlugin

    log.Printf("Plugin %s reloaded successfully", name)
    return nil
}
```

### Video 5.3: Dependency-Aware Reload (25 min)

**Topics:**
- Dependency graph
- Cascade reload
- Cycle detection
- Ordered shutdown

**Dependency Resolution:**
```go
// internal/plugins/dependencies.go
type DependencyGraph struct {
    nodes map[string]*PluginNode
}

type PluginNode struct {
    plugin     Plugin
    dependsOn  []*PluginNode
    dependents []*PluginNode
}

func (dg *DependencyGraph) GetReloadOrder(name string) ([]string, error) {
    node := dg.nodes[name]
    if node == nil {
        return nil, fmt.Errorf("plugin %s not found", name)
    }

    // Get all dependents that need to be reloaded
    var toReload []string
    visited := make(map[string]bool)

    var traverse func(n *PluginNode)
    traverse = func(n *PluginNode) {
        if visited[n.plugin.Name()] {
            return
        }
        visited[n.plugin.Name()] = true

        for _, dep := range n.dependents {
            traverse(dep)
        }
        toReload = append(toReload, n.plugin.Name())
    }

    traverse(node)

    // Reverse for correct order (dependents first)
    slices.Reverse(toReload)
    return toReload, nil
}
```

---

## Module 6: Testing Plugins

### Video 6.1: Unit Testing Plugins (25 min)

**Topics:**
- Mock dependencies
- Lifecycle testing
- Configuration testing
- Error scenarios

**Unit Tests:**
```go
// plugins/custom-logger/plugin_test.go
func TestCustomLoggerPlugin_Init(t *testing.T) {
    tests := []struct {
        name    string
        config  map[string]interface{}
        wantErr bool
    }{
        {
            name: "valid config",
            config: map[string]interface{}{
                "log_level": "debug",
                "format":    "json",
            },
            wantErr: false,
        },
        {
            name: "invalid log level",
            config: map[string]interface{}{
                "log_level": "invalid",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := New()
            err := p.Init(context.Background(), tt.config)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestCustomLoggerPlugin_Lifecycle(t *testing.T) {
    p := New()
    ctx := context.Background()

    // Init
    err := p.Init(ctx, defaultConfig())
    require.NoError(t, err)

    // Start
    err = p.Start(ctx)
    require.NoError(t, err)

    // Health check
    err = p.HealthCheck(ctx)
    require.NoError(t, err)

    // Stop
    err = p.Stop(ctx)
    require.NoError(t, err)
}
```

### Video 6.2: Integration Testing (25 min)

**Topics:**
- Plugin registry testing
- Loader testing
- Hot reload testing
- Dependency testing

**Integration Tests:**
```go
func TestPluginRegistry_Integration(t *testing.T) {
    registry := NewPluginRegistry()
    loader := NewPluginLoader(registry, testConfig)

    // Register plugins
    registry.Register(NewPluginA())
    registry.Register(NewPluginB()) // depends on A

    // Load all
    err := loader.LoadAll(context.Background())
    require.NoError(t, err)

    // Verify load order
    assert.True(t, loader.IsLoaded("plugin-a"))
    assert.True(t, loader.IsLoaded("plugin-b"))

    // Test reload
    err = loader.Reload(context.Background(), "plugin-a")
    require.NoError(t, err)

    // Verify plugin-b was also reloaded (dependency)
    assert.True(t, loader.WasReloaded("plugin-b"))
}
```

---

## Hands-on Labs

### Lab 1: Basic Plugin
Create a simple logging plugin with configuration.

### Lab 2: Provider Plugin
Implement a custom LLM provider plugin.

### Lab 3: Middleware Plugin
Build a rate-limiting middleware plugin.

### Lab 4: Hot Reload
Test hot reload functionality with dependencies.

---

## Resources

- [Plugin Interface Reference](/docs/api/plugins.md)
- [Plugin Examples](https://github.com/helix-agent/plugin-examples)
- [Plugin Development Guide](/docs/guides/plugin-development.md)
- [HelixAgent GitHub](https://dev.helix.agent)

---

## Course Completion

Congratulations! You've completed the Plugin Development Deep Dive course. You should now be able to:

- Design and implement plugins for HelixAgent
- Create provider and middleware plugins
- Implement hot-reload capable plugins
- Test plugins effectively
- Handle plugin dependencies correctly
