// Package client provides the Go client for the Google Gemini System Prompt library.
// Go library providing structured access to leaked and extracted system prompts from Google Gemini models (formerly Bard), including security protocols, prime directives, system instructions, and behavior guidelines across multiple Gemini versions.
//
// Basic usage:
//
//	import gemini-prompt-leak "github.com/elder-plinius/go-gemini-prompt-leak/pkg/client"
//
//	client, err := gemini-prompt-leak.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-gemini-prompt-leak/pkg/types"
)

// Client is the Go client for the Google Gemini System Prompt service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Google Gemini System Prompt client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("gemini-prompt-leak", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gemini-prompt-leak",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gemini-prompt-leak",
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
	return nil, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"GetPrompt requires backend service integration")
}

// GetByModel Get prompts for model.
func (c *Client) GetByModel(ctx context.Context, model string) ([]PromptEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"GetByModel requires backend service integration")
}

// Search Search prompt archive.
func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]PromptEntry, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, 0, errors.Wrap(errors.ErrCodeInvalidArgument, "gemini-prompt-leak", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, 0, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"Search requires backend service integration")
}

// GetModels List available models.
func (c *Client) GetModels(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"GetModels requires backend service integration")
}

// GetVersions List model versions.
func (c *Client) GetVersions(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"GetVersions requires backend service integration")
}

// Export Export archive.
func (c *Client) Export(ctx context.Context, format string) ([]byte, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gemini-prompt-leak",
		"Export requires backend service integration")
}

