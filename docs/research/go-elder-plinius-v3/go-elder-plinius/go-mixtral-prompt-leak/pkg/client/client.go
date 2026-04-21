// Package client provides the Go client for the Mixtral System Prompt Leak library.
// Go library providing structured access to leaked and extracted system prompts from Mistral and Mixtral AI models, including system instructions, safety guidelines, tool use definitions, and model behavior directives.
//
// Basic usage:
//
//	import mixtral-prompt-leak "github.com/elder-plinius/go-mixtral-prompt-leak/pkg/client"
//
//	client, err := mixtral-prompt-leak.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-mixtral-prompt-leak/pkg/types"
)

// Client is the Go client for the Mixtral System Prompt Leak service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Mixtral System Prompt Leak client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("mixtral-prompt-leak", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "mixtral-prompt-leak",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "mixtral-prompt-leak",
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

// GetPrompt Get prompt by ID.
func (c *Client) GetPrompt(ctx context.Context, id string) (*PromptEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "mixtral-prompt-leak",
		"GetPrompt requires backend service integration")
}

// GetByModel Get prompts for model.
func (c *Client) GetByModel(ctx context.Context, model string) ([]PromptEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "mixtral-prompt-leak",
		"GetByModel requires backend service integration")
}

// Search Search prompt archive.
func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]PromptEntry, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, 0, errors.Wrap(errors.ErrCodeInvalidArgument, "mixtral-prompt-leak", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, 0, errors.New(errors.ErrCodeUnimplemented, "mixtral-prompt-leak",
		"Search requires backend service integration")
}

// GetModels List available models.
func (c *Client) GetModels(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "mixtral-prompt-leak",
		"GetModels requires backend service integration")
}

// Export Export archive.
func (c *Client) Export(ctx context.Context, format string) ([]byte, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "mixtral-prompt-leak",
		"Export requires backend service integration")
}

