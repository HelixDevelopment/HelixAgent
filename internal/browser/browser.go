// Package browser provides browser automation capabilities
package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/playwright-community/playwright-go"
)

// Manager manages browser instances
type Manager struct {
	pool   *Pool
	config Config
	mu     sync.Mutex
}

// Config holds browser configuration
type Config struct {
	MaxInstances     int
	Headless         bool
	Timeout          int
	AllowedDomains   []string
	BlockedDomains   []string
	AllowScreenshots bool
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		MaxInstances:     3,
		Headless:         true,
		Timeout:          30000,
		AllowScreenshots: true,
	}
}

// NewManager creates a new browser manager
func NewManager(config Config) (*Manager, error) {
	pool, err := NewPool(config.MaxInstances, config.Headless)
	if err != nil {
		return nil, err
	}

	return &Manager{
		pool:   pool,
		config: config,
	}, nil
}

// Execute executes browser actions
func (m *Manager) Execute(ctx context.Context, actions []Action) (*ActionResult, error) {
	instance, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire browser: %w", err)
	}
	defer m.pool.Release(instance)

	result := &ActionResult{}

	for _, action := range actions {
		if err := action.Execute(ctx, instance.Page); err != nil {
			result.Error = err
			return result, nil
		}

		// Update result based on action type
		switch a := action.(type) {
		case *ScreenshotAction:
			data, err := instance.Page.Screenshot(playwright.PageScreenshotOptions{
				FullPage: playwright.Bool(a.FullPage),
			})
			if err == nil {
				result.Screenshot = &ScreenshotResult{
					Data: data,
				}
			}
		case *ExtractAction:
			content, err := instance.Page.InnerText(a.Selector)
			if err == nil {
				result.Extracted = &ExtractResult{
					Selector: a.Selector,
					Content:  content,
				}
			}
		}
	}

	result.URL = instance.Page.URL()
	result.Title, _ = instance.Page.Title()
	result.Success = true

	return result, nil
}

// Close closes the browser manager
func (m *Manager) Close() error {
	return m.pool.Close()
}

// Instance represents a browser instance
type Instance struct {
	Browser playwright.Browser
	Page    playwright.Page
	Context playwright.BrowserContext
}

// Pool manages a pool of browser instances
type Pool struct {
	maxSize   int
	headless  bool
	instances chan *Instance
	pw        *playwright.Playwright
	mu        sync.Mutex
}

// NewPool creates a new browser pool
func NewPool(maxSize int, headless bool) (*Pool, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start Playwright: %w", err)
	}

	return &Pool{
		maxSize:   maxSize,
		headless:  headless,
		instances: make(chan *Instance, maxSize),
		pw:        pw,
	}, nil
}

// Acquire gets a browser instance from the pool
func (p *Pool) Acquire(ctx context.Context) (*Instance, error) {
	select {
	case instance := <-p.instances:
		return instance, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Create new instance
		return p.createInstance()
	}
}

// Release returns a browser instance to the pool
func (p *Pool) Release(instance *Instance) {
	select {
	case p.instances <- instance:
	default:
		// Pool is full, close instance
		instance.Close()
	}
}

// createInstance creates a new browser instance
func (p *Pool) createInstance() (*Instance, error) {
	browser, err := p.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(p.headless),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	context, err := browser.NewContext()
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	page, err := context.NewPage()
	if err != nil {
		context.Close()
		browser.Close()
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	return &Instance{
		Browser: browser,
		Page:    page,
		Context: context,
	}, nil
}

// Close closes the pool
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.instances)
	for instance := range p.instances {
		instance.Close()
	}

	return p.pw.Stop()
}

// Close closes a browser instance
func (i *Instance) Close() {
	i.Page.Close()
	i.Context.Close()
	i.Browser.Close()
}
