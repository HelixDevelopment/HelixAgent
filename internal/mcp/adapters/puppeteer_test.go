package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockBrowserClient implements BrowserClient for testing
type MockBrowserClient struct {
	currentURL   string
	currentTitle string
	content      string
	cookies      []Cookie
	shouldError  bool
}

func NewMockBrowserClient() *MockBrowserClient {
	return &MockBrowserClient{
		currentURL:   "https://example.com",
		currentTitle: "Example Domain",
		content:      "<html><head><title>Example Domain</title></head><body><h1>Example</h1></body></html>",
		cookies: []Cookie{
			{Name: "session", Value: "abc123", Domain: "example.com", Path: "/"},
		},
	}
}

func (m *MockBrowserClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockBrowserClient) Navigate(ctx context.Context, url string) error {
	if m.shouldError {
		return assert.AnError
	}
	m.currentURL = url
	return nil
}

func (m *MockBrowserClient) Screenshot(ctx context.Context, options ScreenshotOptions) ([]byte, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	// Return fake PNG data
	return []byte{0x89, 0x50, 0x4E, 0x47}, nil
}

func (m *MockBrowserClient) PDF(ctx context.Context, options PDFOptions) ([]byte, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	// Return fake PDF data
	return []byte{0x25, 0x50, 0x44, 0x46}, nil
}

func (m *MockBrowserClient) Click(ctx context.Context, selector string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Type(ctx context.Context, selector, text string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Select(ctx context.Context, selector string, values []string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Evaluate(ctx context.Context, script string) (interface{}, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return "script result", nil
}

func (m *MockBrowserClient) WaitForSelector(ctx context.Context, selector string, timeout time.Duration) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) WaitForNavigation(ctx context.Context, timeout time.Duration) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) GetContent(ctx context.Context) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return m.content, nil
}

func (m *MockBrowserClient) GetTitle(ctx context.Context) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return m.currentTitle, nil
}

func (m *MockBrowserClient) GetURL(ctx context.Context) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return m.currentURL, nil
}

func (m *MockBrowserClient) SetViewport(ctx context.Context, width, height int) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) SetCookie(ctx context.Context, cookie Cookie) error {
	if m.shouldError {
		return assert.AnError
	}
	m.cookies = append(m.cookies, cookie)
	return nil
}

func (m *MockBrowserClient) GetCookies(ctx context.Context) ([]Cookie, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.cookies, nil
}

func (m *MockBrowserClient) ClearCookies(ctx context.Context) error {
	if m.shouldError {
		return assert.AnError
	}
	m.cookies = []Cookie{}
	return nil
}

func (m *MockBrowserClient) ScrollTo(ctx context.Context, x, y int) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Hover(ctx context.Context, selector string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Focus(ctx context.Context, selector string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Press(ctx context.Context, key string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockBrowserClient) Close(ctx context.Context) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

// Tests

func TestDefaultPuppeteerConfig(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()

	assert.True(t, config.Headless)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, 1920, config.ViewportWidth)
	assert.Equal(t, 1080, config.ViewportHeight)
}

func TestNewPuppeteerAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "puppeteer", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestPuppeteerAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "puppeteer_navigate")
	assert.Contains(t, toolNames, "puppeteer_screenshot")
	assert.Contains(t, toolNames, "puppeteer_pdf")
	assert.Contains(t, toolNames, "puppeteer_click")
	assert.Contains(t, toolNames, "puppeteer_type")
}

func TestPuppeteerAdapter_Navigate(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestPuppeteerAdapter_Screenshot(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_screenshot", map[string]interface{}{
		"full_page": true,
		"type":      "png",
		"quality":   90,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_PDF(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_pdf", map[string]interface{}{
		"format":           "A4",
		"landscape":        false,
		"print_background": true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Click(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_click", map[string]interface{}{
		"selector": "#submit-button",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Type(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_type", map[string]interface{}{
		"selector": "#username",
		"text":     "testuser",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Select(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_select", map[string]interface{}{
		"selector": "#country",
		"values":   []interface{}{"us", "uk"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Evaluate(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_evaluate", map[string]interface{}{
		"script": "document.title",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_WaitForSelector(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_wait_for_selector", map[string]interface{}{
		"selector": ".loading-complete",
		"timeout":  5000,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_GetContent(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_get_content", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_GetTitle(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_get_title", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Scroll(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_scroll", map[string]interface{}{
		"x": 0,
		"y": 500,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_Hover(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_hover", map[string]interface{}{
		"selector": "#menu-item",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_SetCookie(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_set_cookie", map[string]interface{}{
		"name":   "auth_token",
		"value":  "xyz789",
		"domain": "example.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_ClearCookies(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_clear_cookies", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPuppeteerAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestPuppeteerAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	browser.SetError(true)
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestScreenshotOptionsTypes(t *testing.T) {
	t.Parallel()
	options := ScreenshotOptions{
		FullPage: true,
		Type:     "jpeg",
		Quality:  85,
		Clip: &Clip{
			X:      100,
			Y:      200,
			Width:  800,
			Height: 600,
		},
	}

	assert.True(t, options.FullPage)
	assert.Equal(t, "jpeg", options.Type)
	assert.Equal(t, 85, options.Quality)
	assert.NotNil(t, options.Clip)
	assert.Equal(t, 100, options.Clip.X)
}

func TestPDFOptionsTypes(t *testing.T) {
	t.Parallel()
	options := PDFOptions{
		Format:          "Letter",
		PrintBackground: true,
		Landscape:       true,
		Scale:           1.5,
		MarginTop:       "1in",
		MarginBottom:    "1in",
		MarginLeft:      "0.5in",
		MarginRight:     "0.5in",
	}

	assert.Equal(t, "Letter", options.Format)
	assert.True(t, options.PrintBackground)
	assert.True(t, options.Landscape)
	assert.Equal(t, 1.5, options.Scale)
}

func TestClipTypes(t *testing.T) {
	t.Parallel()
	clip := Clip{
		X:      50,
		Y:      100,
		Width:  400,
		Height: 300,
	}

	assert.Equal(t, 50, clip.X)
	assert.Equal(t, 100, clip.Y)
	assert.Equal(t, 400, clip.Width)
	assert.Equal(t, 300, clip.Height)
}

func TestCookieTypes(t *testing.T) {
	t.Parallel()
	cookie := Cookie{
		Name:     "session_id",
		Value:    "abc123xyz",
		Domain:   "example.com",
		Path:     "/app",
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
	}

	assert.Equal(t, "session_id", cookie.Name)
	assert.Equal(t, "abc123xyz", cookie.Value)
	assert.Equal(t, "example.com", cookie.Domain)
	assert.True(t, cookie.HTTPOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, "Strict", cookie.SameSite)
}

func TestPuppeteerConfigTypes(t *testing.T) {
	t.Parallel()
	config := PuppeteerConfig{
		Headless:       false,
		DefaultTimeout: 60 * time.Second,
		ViewportWidth:  1280,
		ViewportHeight: 720,
		UserAgent:      "Mozilla/5.0 Custom",
		ProxyServer:    "http://proxy.example.com:8080",
	}

	assert.False(t, config.Headless)
	assert.Equal(t, 60*time.Second, config.DefaultTimeout)
	assert.Equal(t, 1280, config.ViewportWidth)
	assert.Equal(t, 720, config.ViewportHeight)
	assert.NotEmpty(t, config.UserAgent)
	assert.NotEmpty(t, config.ProxyServer)
}

func TestFormatCookies(t *testing.T) {
	t.Parallel()
	cookies := []Cookie{
		{Name: "session", Value: "abc123", Domain: "example.com"},
		{Name: "prefs", Value: "dark_mode", Domain: "example.com"},
	}

	result := FormatCookies(cookies)
	assert.Contains(t, result, "Found 2 cookies")
	assert.Contains(t, result, "session=abc123")
	assert.Contains(t, result, "prefs=dark_mode")
}

func TestPuppeteerAdapter_GetServerInfoCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	info := adapter.GetServerInfo()
	assert.Contains(t, info.Capabilities, "navigation")
	assert.Contains(t, info.Capabilities, "screenshots")
	assert.Contains(t, info.Capabilities, "pdf_generation")
	assert.Contains(t, info.Capabilities, "form_interaction")
	assert.Contains(t, info.Capabilities, "javascript_execution")
}

func TestPuppeteerAdapter_ScreenshotJPEG(t *testing.T) {
	t.Parallel()
	config := DefaultPuppeteerConfig()
	browser := NewMockBrowserClient()
	adapter := NewPuppeteerAdapter(config, browser)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "puppeteer_screenshot", map[string]interface{}{
		"type":    "jpeg",
		"quality": 80,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}
