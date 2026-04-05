package browser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBrowser(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	
	browser := NewBrowser(config, logger)
	
	require.NotNil(t, browser)
	assert.Equal(t, config.UserAgent, browser.userAgent)
	assert.Equal(t, config.Timeout, browser.timeout)
	assert.Equal(t, config.Headless, browser.headless)
	assert.Equal(t, config.Viewport, browser.viewport)
}

func TestBrowser_Execute_Navigate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	config := DefaultConfig()
	config.Timeout = 10 * time.Second
	browser := NewBrowser(config, logger)
	
	ctx := context.Background()
	action := Action{
		Type: "navigate",
		URL:  "https://example.com",
	}
	
	result, err := browser.Execute(ctx, action)
	
	// May fail if offline, but should not panic
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Either success or error should be set
	if result.Success {
		assert.NotEmpty(t, result.URL)
	} else {
		assert.NotEmpty(t, result.Error)
	}
}

func TestBrowser_Execute_Fetch(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type: "fetch",
		URL:  "https://example.com",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Either success or error should be set
	if result.Success {
		assert.NotEmpty(t, result.URL)
	}
}

func TestBrowser_Execute_Extract(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type:     "extract",
		URL:      "https://example.com",
		Selector: "title",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Should work since it's same as navigate
	// Result content depends on connectivity
}

func TestBrowser_Execute_Screenshot(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type: "screenshot",
		URL:  "https://example.com",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Playwright")
}

func TestBrowser_Execute_InvalidAction(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type: "invalid_action",
		URL:  "https://example.com",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown action")
}

func TestBrowser_Execute_EmptyURL(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type: "navigate",
		URL:  "",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "URL is required")
}

func TestBrowser_Execute_InvalidURL(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	browser := NewBrowser(DefaultConfig(), logger)
	
	ctx := context.Background()
	action := Action{
		Type: "navigate",
		URL:  "://invalid-url",
	}
	
	result, err := browser.Execute(ctx, action)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestExtractTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "title tag",
			html:     "<html><head><title>Test Title</title></head></html>",
			expected: "Test Title",
		},
		{
			name:     "title with whitespace",
			html:     "<title>  Title With Spaces  </title>",
			expected: "Title With Spaces",
		},
		{
			name:     "no title",
			html:     "<html><body><h1>Heading</h1></body></html>",
			expected: "Heading",
		},
		{
			name:     "empty html",
			html:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := extractTitle(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple tags",
			input:    "<p>Hello</p>",
			expected: "Hello",
		},
		{
			name:     "nested tags",
			input:    "<div><p>Hello <b>World</b></p></div>",
			expected: "Hello World",
		},
		{
			name:     "with attributes",
			input:    `<div class="test">Content</div>`,
			expected: "Content",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := stripTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		maxLen  int
		expected string
	}{
		{
			name:    "short string",
			input:   "hello",
			maxLen:  10,
			expected: "hello",
		},
		{
			name:    "exact length",
			input:   "hello",
			maxLen:  5,
			expected: "hello",
		},
		{
			name:    "needs truncation",
			input:   "hello world this is long",
			maxLen:  10,
			expected: "hello worl...",
		},
		{
			name:    "empty string",
			input:   "",
			maxLen:  10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	
	assert.True(t, config.Headless)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, "HelixAgent/1.0", config.UserAgent)
	assert.Equal(t, DefaultViewport(), config.Viewport)
}

func TestDefaultViewport(t *testing.T) {
	t.Parallel()
	viewport := DefaultViewport()
	
	assert.Equal(t, 1280, viewport.Width)
	assert.Equal(t, 720, viewport.Height)
}

func TestScreenshotToBase64(t *testing.T) {
	t.Parallel()
	data := []byte("test screenshot data")
	encoded := ScreenshotToBase64(data)
	
	assert.NotEmpty(t, encoded)
	assert.True(t, strings.HasPrefix(encoded, "dGVzd")) // base64 for "test"
}

func TestBrowser_WithTimeout(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	browser := NewBrowser(DefaultConfig(), logger)
	
	// Set a very short timeout
	browser.timeout = 1 * time.Millisecond
	
	ctx := context.Background()
	action := Action{
		Type: "navigate",
		URL:  "https://example.com",
	}
	
	result, err := browser.Execute(ctx, action)
	
	// Should either succeed quickly or timeout
	require.NoError(t, err)
	require.NotNil(t, result)
}
