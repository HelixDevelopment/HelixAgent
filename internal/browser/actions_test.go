package browser

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// NavigationAction Tests

func TestNavigationAction_Struct(t *testing.T) {
	action := NavigationAction{
		URL:     "https://example.com",
		WaitFor: "#content",
		Timeout: 30 * time.Second,
	}

	assert.Equal(t, "https://example.com", action.URL)
	assert.Equal(t, "#content", action.WaitFor)
	assert.Equal(t, 30*time.Second, action.Timeout)
}

func TestNavigationAction_DefaultTimeout(t *testing.T) {
	action := NavigationAction{
		URL:     "https://example.com",
		Timeout: 0, // Should use default in Execute
	}

	assert.Equal(t, time.Duration(0), action.Timeout)
}

// ClickAction Tests

func TestClickAction_Struct(t *testing.T) {
	action := ClickAction{
		Selector:   "#submit",
		Button:     "left",
		ClickCount: 1,
		Timeout:    5 * time.Second,
	}

	assert.Equal(t, "#submit", action.Selector)
	assert.Equal(t, "left", action.Button)
	assert.Equal(t, 1, action.ClickCount)
	assert.Equal(t, 5*time.Second, action.Timeout)
}

func TestClickAction_DefaultValues(t *testing.T) {
	action := ClickAction{
		Selector:   "#button",
		Button:     "", // Should default to left
		ClickCount: 0,  // Should default to 1
		Timeout:    0,  // Will use default in Execute
	}

	assert.Empty(t, action.Button)
	assert.Equal(t, 0, action.ClickCount)
	assert.Equal(t, time.Duration(0), action.Timeout)
}

func TestClickAction_DifferentButtons(t *testing.T) {
	tests := []struct {
		name   string
		button string
	}{
		{"left button", "left"},
		{"right button", "right"},
		{"middle button", "middle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := ClickAction{
				Selector: "#button",
				Button:   tt.button,
			}
			assert.Equal(t, tt.button, action.Button)
		})
	}
}

// TypeAction Tests

func TestTypeAction_Struct(t *testing.T) {
	action := TypeAction{
		Selector: "#search",
		Text:     "search query",
		Delay:    50 * time.Millisecond,
		Clear:    true,
	}

	assert.Equal(t, "#search", action.Selector)
	assert.Equal(t, "search query", action.Text)
	assert.Equal(t, 50*time.Millisecond, action.Delay)
	assert.True(t, action.Clear)
}

func TestTypeAction_NoClear(t *testing.T) {
	action := TypeAction{
		Selector: "#input",
		Text:     "append text",
		Clear:    false,
	}

	assert.False(t, action.Clear)
}

func TestTypeAction_NoDelay(t *testing.T) {
	action := TypeAction{
		Selector: "#input",
		Text:     "fast type",
		Delay:    0,
	}

	assert.Equal(t, time.Duration(0), action.Delay)
}

// ScreenshotAction Tests

func TestScreenshotAction_Struct(t *testing.T) {
	action := ScreenshotAction{
		Selector: "#content",
		FullPage: true,
		Format:   "png",
		Quality:  90,
	}

	assert.Equal(t, "#content", action.Selector)
	assert.True(t, action.FullPage)
	assert.Equal(t, "png", action.Format)
	assert.Equal(t, 90, action.Quality)
}

func TestScreenshotAction_Defaults(t *testing.T) {
	action := ScreenshotAction{
		// No fields set - using defaults
	}

	assert.Empty(t, action.Selector)
	assert.False(t, action.FullPage)
	assert.Empty(t, action.Format)
	assert.Equal(t, 0, action.Quality)
}

func TestScreenshotAction_Formats(t *testing.T) {
	formats := []string{"png", "jpeg", "webp"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			action := ScreenshotAction{
				Format: format,
			}
			assert.Equal(t, format, action.Format)
		})
	}
}

// ScrollAction Tests

func TestScrollAction_Struct(t *testing.T) {
	action := ScrollAction{
		Direction: "down",
		Amount:    500,
		Selector:  "#container",
	}

	assert.Equal(t, "down", action.Direction)
	assert.Equal(t, 500, action.Amount)
	assert.Equal(t, "#container", action.Selector)
}

func TestScrollAction_Directions(t *testing.T) {
	directions := []string{"up", "down", "left", "right"}

	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			action := ScrollAction{
				Direction: dir,
				Amount:    100,
			}
			assert.Equal(t, dir, action.Direction)
		})
	}
}

func TestScrollAction_DefaultAmount(t *testing.T) {
	action := ScrollAction{
		Direction: "down",
		Amount:    0, // Should use default (500) in Execute
	}

	assert.Equal(t, 0, action.Amount)
}

func TestScrollAction_PageScroll(t *testing.T) {
	action := ScrollAction{
		Direction: "down",
		Amount:    300,
		Selector:  "", // Empty selector = page scroll
	}

	assert.Empty(t, action.Selector)
}

// ExtractAction Tests

func TestExtractAction_Struct(t *testing.T) {
	action := ExtractAction{
		Selector:  "h1",
		Type:      "text",
		Attribute: "data-id",
	}

	assert.Equal(t, "h1", action.Selector)
	assert.Equal(t, "text", action.Type)
	assert.Equal(t, "data-id", action.Attribute)
}

func TestExtractAction_Types(t *testing.T) {
	types := []string{"text", "html", "value", "attribute"}

	for _, extractType := range types {
		t.Run(extractType, func(t *testing.T) {
			action := ExtractAction{
				Type: extractType,
			}
			assert.Equal(t, extractType, action.Type)
		})
	}
}

// EvaluateAction Tests

func TestEvaluateAction_Struct(t *testing.T) {
	action := EvaluateAction{
		Script:   "document.title = 'Test'",
		Selector: "body",
	}

	assert.Equal(t, "document.title = 'Test'", action.Script)
	assert.Equal(t, "body", action.Selector)
}

func TestEvaluateAction_ComplexScript(t *testing.T) {
	script := `
		const elements = document.querySelectorAll('.item');
		return Array.from(elements).map(e => e.textContent);
	`

	action := EvaluateAction{
		Script: script,
	}

	assert.Contains(t, action.Script, "querySelectorAll")
}

// WaitAction Tests

func TestWaitAction_Struct(t *testing.T) {
	action := WaitAction{
		Type:     "selector",
		Selector: "#content",
		Timeout:  10 * time.Second,
	}

	assert.Equal(t, "selector", action.Type)
	assert.Equal(t, "#content", action.Selector)
	assert.Equal(t, 10*time.Second, action.Timeout)
}

func TestWaitAction_Types(t *testing.T) {
	tests := []struct {
		name     string
		waitType string
		selector string
	}{
		{"selector", "selector", "#element"},
		{"timeout", "timeout", ""},
		{"navigation", "navigation", ""},
		{"load", "load", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := WaitAction{
				Type:     tt.waitType,
				Selector: tt.selector,
			}
			assert.Equal(t, tt.waitType, action.Type)
			assert.Equal(t, tt.selector, action.Selector)
		})
	}
}

func TestWaitAction_DefaultTimeout(t *testing.T) {
	action := WaitAction{
		Type:     "selector",
		Selector: "#content",
		// Timeout is 0, should default to 5 seconds in Execute
	}

	assert.Equal(t, time.Duration(0), action.Timeout)
}

// ActionResult Tests

func TestActionResult_Struct(t *testing.T) {
	result := ActionResult{
		Success: true,
		Screenshot: &ScreenshotResult{
			Data:   []byte{0x89, 0x50, 0x4E, 0x47},
			Format: "png",
			Width:  1920,
			Height: 1080,
		},
		Extracted: &ExtractResult{
			Selector: "h1",
			Content:  "Title",
			Count:    1,
		},
		Evaluated: &EvaluateResult{
			Result: "test",
			Type:   "string",
		},
		URL:   "https://example.com",
		Title: "Example",
		Error: nil,
	}

	assert.True(t, result.Success)
	assert.NotNil(t, result.Screenshot)
	assert.NotNil(t, result.Extracted)
	assert.NotNil(t, result.Evaluated)
	assert.Equal(t, "https://example.com", result.URL)
	assert.Equal(t, "Example", result.Title)
	assert.NoError(t, result.Error)
}

func TestActionResult_Error(t *testing.T) {
	result := ActionResult{
		Success:    false,
		Error:      assert.AnError,
		Screenshot: nil,
		Extracted:  nil,
	}

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}

func TestActionResult_Empty(t *testing.T) {
	result := ActionResult{}

	assert.False(t, result.Success)
	assert.Nil(t, result.Screenshot)
	assert.Nil(t, result.Extracted)
	assert.Nil(t, result.Evaluated)
	assert.Empty(t, result.URL)
	assert.Empty(t, result.Title)
}

// ScreenshotResult Tests

func TestScreenshotResult_Struct(t *testing.T) {
	result := ScreenshotResult{
		Data:   []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
		Format: "png",
		Width:  1920,
		Height: 1080,
	}

	assert.NotNil(t, result.Data)
	assert.Equal(t, "png", result.Format)
	assert.Equal(t, 1920, result.Width)
	assert.Equal(t, 1080, result.Height)
}

func TestScreenshotResult_Empty(t *testing.T) {
	result := ScreenshotResult{}

	assert.Nil(t, result.Data)
	assert.Empty(t, result.Format)
	assert.Equal(t, 0, result.Width)
	assert.Equal(t, 0, result.Height)
}

func TestScreenshotResult_DifferentSizes(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{1920, 1080},
		{1366, 768},
		{3840, 2160},
		{800, 600},
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			result := ScreenshotResult{
				Width:  size.width,
				Height: size.height,
			}
			assert.Equal(t, size.width, result.Width)
			assert.Equal(t, size.height, result.Height)
		})
	}
}

// ExtractResult Tests

func TestExtractResult_Struct(t *testing.T) {
	result := ExtractResult{
		Selector: "#content",
		Content:  "Extracted text",
		Count:    5,
	}

	assert.Equal(t, "#content", result.Selector)
	assert.Equal(t, "Extracted text", result.Content)
	assert.Equal(t, 5, result.Count)
}

func TestExtractResult_Empty(t *testing.T) {
	result := ExtractResult{}

	assert.Empty(t, result.Selector)
	assert.Empty(t, result.Content)
	assert.Equal(t, 0, result.Count)
}

func TestExtractResult_SingleElement(t *testing.T) {
	result := ExtractResult{
		Selector: "h1",
		Content:  "Page Title",
		Count:    1,
	}

	assert.Equal(t, 1, result.Count)
}

func TestExtractResult_MultipleElements(t *testing.T) {
	result := ExtractResult{
		Selector: ".item",
		Content:  "Item 1, Item 2, Item 3",
		Count:    3,
	}

	assert.Equal(t, 3, result.Count)
}

// EvaluateResult Tests

func TestEvaluateResult_Struct(t *testing.T) {
	result := EvaluateResult{
		Result: map[string]interface{}{
			"key": "value",
			"num": 42,
		},
		Type: "object",
	}

	assert.NotNil(t, result.Result)
	assert.Equal(t, "object", result.Type)
}

func TestEvaluateResult_DifferentTypes(t *testing.T) {
	tests := []struct {
		name   string
		result interface{}
		typ    string
	}{
		{"string", "hello", "string"},
		{"number", 42, "number"},
		{"boolean", true, "boolean"},
		{"array", []int{1, 2, 3}, "array"},
		{"object", map[string]string{"a": "b"}, "object"},
		{"null", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EvaluateResult{
				Result: tt.result,
				Type:   tt.typ,
			}
			assert.Equal(t, tt.result, result.Result)
			assert.Equal(t, tt.typ, result.Type)
		})
	}
}

func TestEvaluateResult_Empty(t *testing.T) {
	result := EvaluateResult{}

	assert.Nil(t, result.Result)
	assert.Empty(t, result.Type)
}

// Action Interface Test

func TestAction_Interface(t *testing.T) {
	// Ensure all action types implement the Action interface by checking they compile
	// We can't actually call Execute without a playwright.Page mock
	var _ Action = (*NavigationAction)(nil)
	var _ Action = (*ClickAction)(nil)
	var _ Action = (*TypeAction)(nil)
	var _ Action = (*ScreenshotAction)(nil)
	var _ Action = (*ScrollAction)(nil)
	var _ Action = (*ExtractAction)(nil)
	var _ Action = (*EvaluateAction)(nil)
	var _ Action = (*WaitAction)(nil)
}

// Edge case tests

func TestScrollAction_AllDirections(t *testing.T) {
	directions := map[string]struct {
		direction string
		amount    int
	}{
		"up":    {"up", 500},
		"down":  {"down", 500},
		"left":  {"left", 300},
		"right": {"right", 300},
	}

	for name, tc := range directions {
		t.Run(name, func(t *testing.T) {
			action := ScrollAction{
				Direction: tc.direction,
				Amount:    tc.amount,
			}
			assert.Equal(t, tc.direction, action.Direction)
			assert.Equal(t, tc.amount, action.Amount)
		})
	}
}

func TestWaitAction_AllTypes(t *testing.T) {
	waitTypes := []string{"selector", "timeout", "navigation", "load"}

	for _, wt := range waitTypes {
		t.Run(wt, func(t *testing.T) {
			action := WaitAction{
				Type:    wt,
				Timeout: 5 * time.Second,
			}
			assert.Equal(t, wt, action.Type)
		})
	}
}

func TestScreenshotAction_QualityRange(t *testing.T) {
	qualities := []int{0, 50, 75, 90, 100}

	for _, q := range qualities {
		t.Run(fmt.Sprintf("quality_%d", q), func(t *testing.T) {
			action := ScreenshotAction{
				Format:  "jpeg",
				Quality: q,
			}
			assert.Equal(t, q, action.Quality)
		})
	}
}

func TestNavigationAction_URLVariations(t *testing.T) {
	urls := []string{
		"https://example.com",
		"http://localhost:8080",
		"file:///path/to/file",
		"about:blank",
		"data:text/html,<html></html>",
	}

	for _, url := range urls {
		t.Run(url[:10], func(t *testing.T) {
			action := NavigationAction{
				URL: url,
			}
			assert.Equal(t, url, action.URL)
		})
	}
}

// Test action creation helpers (if any exist in actual implementation)

func TestAction_FieldMutability(t *testing.T) {
	// Test that action fields can be modified
	action := NavigationAction{}

	action.URL = "https://example.com"
	action.WaitFor = "#content"
	action.Timeout = 30 * time.Second

	assert.Equal(t, "https://example.com", action.URL)
	assert.Equal(t, "#content", action.WaitFor)
	assert.Equal(t, 30*time.Second, action.Timeout)
}
