// Package browser provides browser actions
package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Action represents a browser action
type Action interface {
	Execute(ctx context.Context, page playwright.Page) error
}

// NavigationAction navigates to a URL
type NavigationAction struct {
	URL     string
	WaitFor string
	Timeout time.Duration
}

// Execute implements Action
func (a *NavigationAction) Execute(ctx context.Context, page playwright.Page) error {
	_, err := page.Goto(a.URL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(a.Timeout.Milliseconds())),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	if a.WaitFor != "" {
		if _, err := page.WaitForSelector(a.WaitFor); err != nil {
			return fmt.Errorf("failed to wait for selector: %w", err)
		}
	}

	return nil
}

// ClickAction clicks an element
type ClickAction struct {
	Selector   string
	Button     string
	ClickCount int
	Timeout    time.Duration
}

// Execute implements Action
func (a *ClickAction) Execute(ctx context.Context, page playwright.Page) error {
	button := playwright.MouseButtonLeft
	if a.Button == "right" {
		button = playwright.MouseButtonRight
	} else if a.Button == "middle" {
		button = playwright.MouseButtonMiddle
	}

	count := a.ClickCount
	if count == 0 {
		count = 1
	}

	err := page.Click(a.Selector, playwright.PageClickOptions{
		Button:     button,
		ClickCount: &count,
		Timeout:    playwright.Float(float64(a.Timeout.Milliseconds())),
	})
	if err != nil {
		return fmt.Errorf("failed to click: %w", err)
	}

	return nil
}

// TypeAction types text into an input
type TypeAction struct {
	Selector string
	Text     string
	Delay    time.Duration
	Clear    bool
}

// Execute implements Action
func (a *TypeAction) Execute(ctx context.Context, page playwright.Page) error {
	if a.Clear {
		if err := page.Fill(a.Selector, ""); err != nil {
			return fmt.Errorf("failed to clear: %w", err)
		}
	}

	options := playwright.PageTypeOptions{}
	if a.Delay > 0 {
		options.Delay = playwright.Float(float64(a.Delay.Milliseconds()))
	}

	if err := page.Type(a.Selector, a.Text, options); err != nil {
		return fmt.Errorf("failed to type: %w", err)
	}

	return nil
}

// ScreenshotAction captures a screenshot
type ScreenshotAction struct {
	Selector string
	FullPage bool
	Format   string
	Quality  int
}

// Execute implements Action
func (a *ScreenshotAction) Execute(ctx context.Context, page playwright.Page) error {
	// Screenshot is handled by the manager
	return nil
}

// ScrollAction scrolls the page
type ScrollAction struct {
	Direction string
	Amount    int
	Selector  string
}

// Execute implements Action
func (a *ScrollAction) Execute(ctx context.Context, page playwright.Page) error {
	amount := a.Amount
	if amount == 0 {
		amount = 500
	}

	var script string
	if a.Selector != "" {
		// Scroll element
		switch a.Direction {
		case "up":
			script = fmt.Sprintf(`document.querySelector("%s").scrollBy(0, -%d)`, a.Selector, amount)
		case "left":
			script = fmt.Sprintf(`document.querySelector("%s").scrollBy(-%d, 0)`, a.Selector, amount)
		case "right":
			script = fmt.Sprintf(`document.querySelector("%s").scrollBy(%d, 0)`, a.Selector, amount)
		default: // down
			script = fmt.Sprintf(`document.querySelector("%s").scrollBy(0, %d)`, a.Selector, amount)
		}
	} else {
		// Scroll page
		switch a.Direction {
		case "up":
			script = fmt.Sprintf(`window.scrollBy(0, -%d)`, amount)
		case "left":
			script = fmt.Sprintf(`window.scrollBy(-%d, 0)`, amount)
		case "right":
			script = fmt.Sprintf(`window.scrollBy(%d, 0)`, amount)
		default: // down
			script = fmt.Sprintf(`window.scrollBy(0, %d)`, amount)
		}
	}

	_, err := page.Evaluate(script)
	if err != nil {
		return fmt.Errorf("failed to scroll: %w", err)
	}

	return nil
}

// ExtractAction extracts content from the page
type ExtractAction struct {
	Selector  string
	Type      string
	Attribute string
}

// Execute implements Action
func (a *ExtractAction) Execute(ctx context.Context, page playwright.Page) error {
	// Extraction is handled by the manager
	return nil
}

// EvaluateAction executes JavaScript
type EvaluateAction struct {
	Script   string
	Selector string
}

// Execute implements Action
func (a *EvaluateAction) Execute(ctx context.Context, page playwright.Page) error {
	_, err := page.Evaluate(a.Script)
	if err != nil {
		return fmt.Errorf("failed to evaluate: %w", err)
	}
	return nil
}

// WaitAction waits for a condition
type WaitAction struct {
	Type     string
	Selector string
	Timeout  time.Duration
}

// Execute implements Action
func (a *WaitAction) Execute(ctx context.Context, page playwright.Page) error {
	timeout := a.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	switch a.Type {
	case "selector":
		if _, err := page.WaitForSelector(a.Selector, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(float64(timeout.Milliseconds())),
		}); err != nil {
			return fmt.Errorf("failed to wait for selector: %w", err)
		}
	case "timeout":
		time.Sleep(timeout)
	case "navigation":
		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			Timeout: playwright.Float(float64(timeout.Milliseconds())),
		}); err != nil {
			return fmt.Errorf("failed to wait for navigation: %w", err)
		}
	case "load":
		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateLoad,
			Timeout: playwright.Float(float64(timeout.Milliseconds())),
		}); err != nil {
			return fmt.Errorf("failed to wait for load: %w", err)
		}
	}

	return nil
}

// ActionResult contains action results
type ActionResult struct {
	Success    bool
	Screenshot *ScreenshotResult
	Extracted  *ExtractResult
	Evaluated  *EvaluateResult
	URL        string
	Title      string
	Error      error
}

// ScreenshotResult contains screenshot data
type ScreenshotResult struct {
	Data   []byte
	Format string
	Width  int
	Height int
}

// ExtractResult contains extracted data
type ExtractResult struct {
	Selector string
	Content  string
	Count    int
}

// EvaluateResult contains evaluation result
type EvaluateResult struct {
	Result interface{}
	Type   string
}
