// Package browser provides browser automation capabilities
// Inspired by Cline and OpenHands browser automation
package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Browser provides browser automation capabilities
type Browser struct {
	logger    *logrus.Logger
	userAgent string
	timeout   time.Duration
	headless  bool
	viewport  Viewport
}

// Viewport defines browser window size
type Viewport struct {
	Width  int
	Height int
}

// DefaultViewport returns default viewport size
func DefaultViewport() Viewport {
	return Viewport{Width: 1280, Height: 720}
}

// Config configures the browser
type Config struct {
	Headless  bool
	Timeout   time.Duration
	Viewport  Viewport
	UserAgent string
}

// DefaultConfig returns default browser configuration
func DefaultConfig() Config {
	return Config{
		Headless:  true,
		Timeout:   30 * time.Second,
		Viewport:  DefaultViewport(),
		UserAgent: "HelixAgent/1.0",
	}
}

// NewBrowser creates a new browser instance
func NewBrowser(config Config, logger *logrus.Logger) *Browser {
	if logger == nil {
		logger = logrus.New()
	}

	return &Browser{
		logger:    logger,
		userAgent: config.UserAgent,
		timeout:   config.Timeout,
		headless:  config.Headless,
		viewport:  config.Viewport,
	}
}

// Action represents a browser action
type Action struct {
	Type     string                 `json:"type"` // navigate, click, type, screenshot, scroll, extract
	URL      string                 `json:"url,omitempty"`
	Selector string                 `json:"selector,omitempty"`
	Text     string                 `json:"text,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// Result represents the result of a browser action
type Result struct {
	Success    bool                   `json:"success"`
	URL        string                 `json:"url,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Screenshot string                 `json:"screenshot,omitempty"` // base64 encoded
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Execute performs a browser action
func (b *Browser) Execute(ctx context.Context, action Action) (*Result, error) {
	// Apply timeout
	if b.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.timeout)
		defer cancel()
	}

	switch action.Type {
	case "navigate":
		return b.navigate(ctx, action.URL)
	case "fetch":
		return b.fetch(ctx, action.URL)
	case "extract":
		return b.extract(ctx, action.URL, action.Selector)
	case "screenshot":
		return b.screenshot(ctx, action.URL)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action type: %s", action.Type),
		}, nil
	}
}

// navigate navigates to a URL and returns basic info
func (b *Browser) navigate(ctx context.Context, urlStr string) (*Result, error) {
	if urlStr == "" {
		return &Result{Success: false, Error: "URL is required"}, nil
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("invalid URL: %v", err)}, nil
	}

	// For now, use HTTP client to fetch basic info
	// In production, this would use Playwright or Selenium
	client := &http.Client{
		Timeout: b.timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	req.Header.Set("User-Agent", b.userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	// Read content
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Limit to 1MB
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Extract title
	title := extractTitle(string(body))

	return &Result{
		Success: true,
		URL:     resp.Request.URL.String(),
		Title:   title,
		Content: truncate(string(body), 5000),
		Metadata: map[string]interface{}{
			"status_code":  resp.StatusCode,
			"content_type": resp.Header.Get("Content-Type"),
		},
	}, nil
}

// fetch fetches content from a URL
func (b *Browser) fetch(ctx context.Context, urlStr string) (*Result, error) {
	return b.navigate(ctx, urlStr)
}

// extract extracts specific content from a URL
func (b *Browser) extract(ctx context.Context, urlStr, selector string) (*Result, error) {
	result, err := b.navigate(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return result, nil
	}

	// Simple extraction based on selector
	// In production, this would use a proper HTML parser
	if selector != "" {
		content := extractBySelector(result.Content, selector)
		result.Content = content
	}

	return result, nil
}

// screenshot captures a screenshot of a URL
func (b *Browser) screenshot(ctx context.Context, urlStr string) (*Result, error) {
	// Placeholder implementation
	// In production, this would use Playwright or similar
	return &Result{
		Success: false,
		Error:   "screenshot requires Playwright integration (not yet implemented)",
		Metadata: map[string]interface{}{
			"note": "Install Playwright for full browser automation",
		},
	}, nil
}

// extractTitle extracts title from HTML
func extractTitle(html string) string {
	// Simple regex-based extraction
	// In production, use a proper HTML parser
	start := strings.Index(html, "<title>")
	end := strings.Index(html, "</title>")

	if start != -1 && end != -1 && end > start {
		title := html[start+7 : end]
		return strings.TrimSpace(title)
	}

	// Try h1
	start = strings.Index(html, "<h1")
	if start != -1 {
		endTag := strings.Index(html[start:], ">")
		closeTag := strings.Index(html[start:], "</h1>")
		if endTag != -1 && closeTag != -1 {
			return strings.TrimSpace(stripTags(html[start+endTag+1 : start+closeTag]))
		}
	}

	return ""
}

// extractBySelector extracts content by CSS selector (simplified)
func extractBySelector(html, selector string) string {
	// Very simplified extraction
	// In production, use goquery or similar

	// Handle common selectors
	switch selector {
	case "title":
		return extractTitle(html)
	case "body":
		start := strings.Index(html, "<body")
		end := strings.LastIndex(html, "</body>")
		if start != -1 && end != -1 {
			return stripTags(html[start:end])
		}
	case "text":
		return stripTags(html)
	default:
		// Try to find by tag name
		start := strings.Index(html, "<"+selector)
		if start != -1 {
			endTag := strings.Index(html[start:], ">")
			closeTag := strings.Index(html[start:], "</"+selector+">")
			if endTag != -1 && closeTag != -1 {
				return strings.TrimSpace(stripTags(html[start+endTag+1 : start+closeTag]))
			}
		}
	}

	return ""
}

// stripTags removes HTML tags
func stripTags(html string) string {
	var result strings.Builder
	inTag := false

	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ScreenshotToBase64 converts bytes to base64 (placeholder for actual implementation)
func ScreenshotToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
