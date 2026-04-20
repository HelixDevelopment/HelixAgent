// Package client provides the Go client for the L1B3RT4S library.
// Go library for the L1B3RT4S collection of jailbreak and prompt injection techniques. Provides structured access to prompt patterns targeting various AI models with safety testing capabilities.
//
// Basic usage:
//
//	import l1b3rt4s "github.com/elder-plinius/go-l1b3rt4s/pkg/client"
//
//	client, err := l1b3rt4s.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-l1b3rt4s/pkg/types"
)

// Client is the Go client for the L1B3RT4S service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new L1B3RT4S client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("l1b3rt4s", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "l1b3rt4s",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "l1b3rt4s",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// Close gracefully closes the client.
func (c *Client) Close() error {
	if c.closed { return nil }
	c.closed = true
	return nil
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config { return c.cfg }

// SearchPrompts Search jailbreak prompts.
func (c *Client) SearchPrompts(ctx context.Context, opts SearchOptions) ([]JailbreakPrompt, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "l1b3rt4s", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"SearchPrompts requires backend service integration")
}

// GetPromptByID Get prompt by ID.
func (c *Client) GetPromptByID(ctx context.Context, id string) (*JailbreakPrompt, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"GetPromptByID requires backend service integration")
}

// GetByCategory Get prompts by category.
func (c *Client) GetByCategory(ctx context.Context, category string) ([]JailbreakPrompt, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"GetByCategory requires backend service integration")
}

// GetByTargetModel Get prompts for specific model.
func (c *Client) GetByTargetModel(ctx context.Context, model string) ([]JailbreakPrompt, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"GetByTargetModel requires backend service integration")
}

// RenderTemplate Render a prompt template with variables.
func (c *Client) RenderTemplate(ctx context.Context, templateID string, variables map[string]string) (string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"RenderTemplate requires backend service integration")
}

// TestPrompt Test a prompt against a model.
func (c *Client) TestPrompt(ctx context.Context, opts SafetyCheckOptions) (*TestResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "l1b3rt4s", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"TestPrompt requires backend service integration")
}

// GetCategories Get all categories.
func (c *Client) GetCategories(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"GetCategories requires backend service integration")
}

// GetTargetModels Get all target models.
func (c *Client) GetTargetModels(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"GetTargetModels requires backend service integration")
}

// CompareEffectiveness Compare prompt effectiveness.
func (c *Client) CompareEffectiveness(ctx context.Context, ids []string) (*EffectivenessComparison, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "l1b3rt4s",
		"CompareEffectiveness requires backend service integration")
}

