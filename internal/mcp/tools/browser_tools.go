// Package tools provides MCP tools for browser automation
package tools

// BrowserTools returns browser-related MCP tools
func BrowserTools() []ToolDefinition {
	return []ToolDefinition{
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
						"type":    "string",
						"enum":    []string{"left", "right", "middle"},
						"default": "left",
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
						"type":    "boolean",
						"default": true,
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
						"type":    "boolean",
						"default": false,
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
						"type":    "string",
						"enum":    []string{"text", "html", "innerText"},
						"default": "text",
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
						"type": "string",
						"enum": []string{"up", "down", "left", "right"},
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
						"type": "string",
						"enum": []string{"selector", "timeout", "navigation", "load"},
					},
					"selector": map[string]interface{}{
						"type": "string",
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
}
