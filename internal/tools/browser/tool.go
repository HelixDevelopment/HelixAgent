// Package browser provides browser automation as a tool
package browser

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// Tool provides browser automation as a tool
type Tool struct {
	browser *Browser
	logger  *logrus.Logger
}

// ToolResult represents the result of tool execution
type ToolResult struct {
	Success    bool                   `json:"success"`
	URL        string                 `json:"url,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Screenshot string                 `json:"screenshot,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewTool creates a new browser tool
func NewTool(logger *logrus.Logger) *Tool {
	if logger == nil {
		logger = logrus.New()
	}

	config := DefaultConfig()
	browser := NewBrowser(config, logger)

	return &Tool{
		browser: browser,
		logger:  logger,
	}
}

// Name returns the tool name
func (t *Tool) Name() string {
	return "Browser"
}

// Description returns the tool description
func (t *Tool) Description() string {
	return "Automate browser actions: navigate to URLs, extract content, take screenshots"
}

// Schema returns the tool schema
func (t *Tool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"navigate", "fetch", "extract", "screenshot"},
				"description": "The browser action to perform",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to navigate to",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector for content extraction (for extract action)",
			},
			"wait_for": map[string]interface{}{
				"type":        "string",
				"description": "Element to wait for before extracting (optional)",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds",
				"default":     30,
			},
		},
		"required": []string{"action", "url"},
	}
}

// Execute runs the browser tool
func (t *Tool) Execute(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	// Extract parameters
	actionType, ok := input["action"].(string)
	if !ok || actionType == "" {
		return &ToolResult{
			Success: false,
			Error:   "action is required",
		}, nil
	}

	urlStr, ok := input["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "url is required",
		}, nil
	}

	// Get optional parameters
	selector := ""
	if sel, ok := input["selector"].(string); ok {
		selector = sel
	}

	timeout := 30
	if to, ok := input["timeout"].(float64); ok {
		timeout = int(to)
	}

	// Apply the per-call timeout via context rather than mutating the
	// shared t.browser.timeout field — the previous approach was a
	// data race when Tool.Execute was called from multiple goroutines
	// (TestTool_ConcurrentExecution detects this under -race).
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Create action
	action := Action{
		Type:     actionType,
		URL:      urlStr,
		Selector: selector,
	}

	// Execute
	result, err := t.browser.Execute(callCtx, action)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success:    result.Success,
		URL:        result.URL,
		Title:      result.Title,
		Content:    result.Content,
		Screenshot: result.Screenshot,
		Error:      result.Error,
		Metadata:   result.Metadata,
	}, nil
}

// Navigate navigates to a URL and returns basic info
func (t *Tool) Navigate(ctx context.Context, url string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"action": "navigate",
		"url":    url,
	})
}

// Fetch fetches content from a URL
func (t *Tool) Fetch(ctx context.Context, url string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"action": "fetch",
		"url":    url,
	})
}

// Extract extracts specific content from a URL
func (t *Tool) Extract(ctx context.Context, url, selector string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"action":   "extract",
		"url":      url,
		"selector": selector,
	})
}

// Screenshot captures a screenshot of a URL
func (t *Tool) Screenshot(ctx context.Context, url string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"action": "screenshot",
		"url":    url,
	})
}
