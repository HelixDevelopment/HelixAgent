# Implementation Specification: Browser Automation & Computer Use

**Document ID:** IMPL-003  
**Feature:** Browser Automation  
**Priority:** HIGH  
**Phase:** 1  
**Estimated Effort:** 3 weeks  
**Source:** Cline

---

## Overview

Implement browser automation capabilities using Playwright to enable agents to interact with web pages, capture screenshots, fill forms, and extract information.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Browser Automation System                         │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │   Browser    │  │    Page      │  │   Security   │              │
│  │   Manager    │  │   Actions    │  │   Sandbox    │              │
│  │              │  │              │  │              │              │
│  │ - Pool mgmt  │  │ - Click      │  │ - URL allow  │              │
│  │ - Context    │  │ - Type       │  │ - CSP        │              │
│  │ - Cleanup    │  │ - Navigate   │  │ - Isolation  │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         │                 │                 │                       │
│         └─────────────────┴─────────────────┘                       │
│                           │                                         │
│                           ▼                                         │
│                ┌──────────────────────┐                             │
│                │    Playwright        │                             │
│                │    (Chromium/Firefox/│                             │
│                │     WebKit)          │                             │
│                └──────────────────────┘                             │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Browser Manager (`internal/browser/manager.go`)

```go
package browser

// Manager handles browser lifecycle
type Manager struct {
    pool       *BrowserPool
    config     BrowserConfig
    sandbox    *Sandbox
}

type BrowserConfig struct {
    MaxBrowsers        int
    MaxPagesPerBrowser int
    Headless           bool
    Timeout            time.Duration
    
    // Security
    AllowedDomains     []string
    BlockedDomains     []string
    AllowScreenshots   bool
    AllowDownloads     bool
    
    // Browser options
    UserAgent          string
    Viewport           Viewport
    Locale             string
    Timezone           string
}

type Viewport struct {
    Width  int
    Height int
}

func (m *Manager) Acquire(ctx context.Context) (*Browser, error)
func (m *Manager) Release(browser *Browser)
func (m *Manager) Execute(ctx context.Context, actions []Action) (*ActionResult, error)
```

### 2. Browser Actions (`internal/browser/actions.go`)

```go
package browser

// Action represents a browser action
type Action interface {
    Execute(ctx context.Context, page playwright.Page) error
    String() string
}

// NavigationAction navigates to URL
type NavigationAction struct {
    URL     string
    WaitFor string // selector to wait for
    Timeout time.Duration
}

// ClickAction clicks an element
type ClickAction struct {
    Selector    string
    Button      string // left, right, middle
    ClickCount  int
    Timeout     time.Duration
}

// TypeAction types text into input
type TypeAction struct {
    Selector string
    Text     string
    Delay    time.Duration // typing delay between keystrokes
    Clear    bool          // clear before typing
}

// ScreenshotAction captures screenshot
type ScreenshotAction struct {
    Selector  string // empty for full page
    FullPage  bool
    Format    string // png, jpeg
    Quality   int    // for jpeg
}

// ScrollAction scrolls the page
type ScrollAction struct {
    Direction string // up, down, left, right
    Amount    int    // pixels
    Selector  string // scroll within element
}

// ExtractAction extracts content
type ExtractAction struct {
    Selector string
    Type     string // text, html, attribute, innerText
    Attribute string // for attribute extraction
}

// EvaluateAction executes JavaScript
type EvaluateAction struct {
    Script   string
    Selector string // element to evaluate on
}

// WaitAction waits for condition
type WaitAction struct {
    Type      string // selector, timeout, navigation, load
    Selector  string
    Timeout   time.Duration
}
```

### 3. Action Result Types

```go
type ActionResult struct {
    Success     bool
    Screenshot  *ScreenshotResult
    Extracted   *ExtractResult
    Evaluated   *EvaluateResult
    URL         string
    Title       string
    Error       error
    Duration    time.Duration
}

type ScreenshotResult struct {
    Data     []byte
    Format   string
    Width    int
    Height   int
}

type ExtractResult struct {
    Selector string
    Content  string
    Count    int // number of elements found
}

type EvaluateResult struct {
    Result interface{}
    Type   string
}
```

## MCP Tools

```go
// internal/mcp/tools/browser_tools.go

var BrowserTools = []ToolDefinition{
    {
        Name:        "browser_navigate",
        Description: "Navigate to a URL",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "url": map[string]interface{}{
                    "type":        "string",
                    "description": "URL to navigate to",
                },
                "wait_for": map[string]interface{}{
                    "type":        "string",
                    "description": "CSS selector to wait for",
                },
            },
            "required": []string{"url"},
        },
    },
    {
        Name:        "browser_click",
        Description: "Click an element",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "selector": map[string]interface{}{
                    "type":        "string",
                    "description": "CSS selector",
                },
                "button": map[string]interface{}{
                    "type":        "string",
                    "enum":        []string{"left", "right", "middle"},
                    "default":     "left",
                },
            },
            "required": []string{"selector"},
        },
    },
    {
        Name:        "browser_type",
        Description: "Type text into input",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "selector": map[string]interface{}{
                    "type":        "string",
                    "description": "CSS selector",
                },
                "text": map[string]interface{}{
                    "type":        "string",
                    "description": "Text to type",
                },
                "clear": map[string]interface{}{
                    "type":        "boolean",
                    "default":     true,
                },
            },
            "required": []string{"selector", "text"},
        },
    },
    {
        Name:        "browser_screenshot",
        Description: "Capture screenshot",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "selector": map[string]interface{}{
                    "type":        "string",
                    "description": "CSS selector (empty for full page)",
                },
                "full_page": map[string]interface{}{
                    "type":        "boolean",
                    "default":     false,
                },
            },
        },
    },
    {
        Name:        "browser_extract",
        Description: "Extract content from page",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "selector": map[string]interface{}{
                    "type":        "string",
                    "description": "CSS selector",
                },
                "type": map[string]interface{}{
                    "type":        "string",
                    "enum":        []string{"text", "html", "innerText"},
                    "default":     "text",
                },
            },
            "required": []string{"selector"},
        },
    },
    {
        Name:        "browser_scroll",
        Description: "Scroll the page",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "direction": map[string]interface{}{
                    "type":        "string",
                    "enum":        []string{"up", "down", "left", "right"},
                },
                "amount": map[string]interface{}{
                    "type":        "integer",
                    "description": "Pixels to scroll",
                    "default":     500,
                },
            },
        },
    },
    {
        Name:        "browser_evaluate",
        Description: "Execute JavaScript",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "script": map[string]interface{}{
                    "type":        "string",
                    "description": "JavaScript code",
                },
            },
            "required": []string{"script"},
        },
    },
    {
        Name:        "browser_wait",
        Description: "Wait for condition",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "type": map[string]interface{}{
                    "type":        "string",
                    "enum":        []string{"selector", "timeout", "navigation", "load"},
                },
                "selector": map[string]interface{}{
                    "type":        "string",
                },
                "timeout": map[string]interface{}{
                    "type":        "integer",
                    "description": "Milliseconds to wait",
                    "default":     5000,
                },
            },
        },
    },
}
```

## Security Model

```go
// internal/browser/sandbox.go

type Sandbox struct {
    allowedHosts []string
    blockedHosts []string
}

func (s *Sandbox) ValidateURL(url string) error {
    // Check against allowed/blocked lists
    // Block file:// protocol
    // Block localhost in production
    // Validate scheme (http/https only)
}

func (s *Sandbox) CreateIsolatedContext() (playwright.BrowserContext, error) {
    // Disable JavaScript if needed
    // Set strict CSP
    // Block popups
    // Disable downloads (unless allowed)
    // Clear cookies/localStorage on start
}
```

## Configuration

```yaml
# configs/browser.yaml
browser:
  enabled: true
  
  pool:
    max_browsers: 5
    max_pages_per_browser: 3
    idle_timeout: 300
    
  browser_options:
    headless: true
    timeout: 30000
    viewport:
      width: 1280
      height: 720
    user_agent: "helix_agent/1.0"
    
  security:
    allowed_domains: []
    blocked_domains:
      - "*.internal.company.com"
      - "localhost"
      - "127.0.0.1"
    allow_screenshots: true
    allow_downloads: false
    allow_javascript: true
    allow_cookies: false
    
  screenshots:
    format: "png"
    quality: 90
    max_size: 10485760  # 10MB
    storage: "s3://helixagent-screenshots"
```

## API Endpoints

```go
// internal/handlers/browser_handler.go

func (h *BrowserHandler) Navigate(c *gin.Context) {
    var req NavigateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, ErrorResponse{Error: err.Error()})
        return
    }
    
    result, err := h.browser.Execute(c.Request.Context(), []Action{
        &NavigationAction{URL: req.URL, WaitFor: req.WaitFor},
    })
    
    if err != nil {
        c.JSON(500, ErrorResponse{Error: err.Error()})
        return
    }
    
    c.JSON(200, result)
}

func (h *BrowserHandler) ExecuteActions(c *gin.Context) {
    var req ExecuteRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, ErrorResponse{Error: err.Error()})
        return
    }
    
    // Parse and validate actions
    actions, err := ParseActions(req.Actions)
    if err != nil {
        c.JSON(400, ErrorResponse{Error: err.Error()})
        return
    }
    
    result, err := h.browser.Execute(c.Request.Context(), actions)
    if err != nil {
        c.JSON(500, ErrorResponse{Error: err.Error()})
        return
    }
    
    c.JSON(200, result)
}
```

## Usage Example

```yaml
# Example workflow
steps:
  - action: browser_navigate
    url: "https://example.com/login"
    
  - action: browser_type
    selector: "#username"
    text: "user@example.com"
    
  - action: browser_type
    selector: "#password"
    text: "secretpassword"
    
  - action: browser_click
    selector: "#submit"
    
  - action: browser_wait
    type: "selector"
    selector: ".dashboard"
    
  - action: browser_screenshot
    full_page: true
    
  - action: browser_extract
    selector: ".dashboard .stats"
    type: "text"
```

## Implementation Timeline

**Week 1: Core Browser Infrastructure**
- [ ] Setup Playwright integration
- [ ] Implement BrowserManager
- [ ] Create browser pool
- [ ] Basic navigation

**Week 2: Actions & Security**
- [ ] Implement all action types
- [ ] Security sandbox
- [ ] URL validation
- [ ] Screenshot handling

**Week 3: Integration & Testing**
- [ ] MCP tool integration
- [ ] API endpoints
- [ ] Security testing
- [ ] Performance optimization

## Dependencies

```go
// go.mod
go get github.com/playwright-community/playwright-go
```

## Testing

```go
func TestBrowserManager_Navigate(t *testing.T) {}
func TestSandbox_ValidateURL(t *testing.T) {}
func TestActionExecution(t *testing.T) {}
```
