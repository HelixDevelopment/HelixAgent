// Package client provides the Go client for the Bing Prompt Leak library.
// Go library documenting and providing structured access to prompt leak techniques for Microsoft Bing Chat (Copilot), including leetspeak-based extraction, encoding bypasses, and discovered system prompt content.
//
// Basic usage:
//
//	import bing-prompt-leak "github.com/elder-plinius/go-bing-prompt-leak/pkg/client"
//
//	client, err := bing-prompt-leak.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-bing-prompt-leak/pkg/types"
)

// Client is the Go client for the Bing Prompt Leak service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Bing Prompt Leak client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("bing-prompt-leak", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "bing-prompt-leak",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "bing-prompt-leak",
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

// GetLeak Get leak entry by ID.
func (c *Client) GetLeak(ctx context.Context, id string) (*LeakEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "bing-prompt-leak",
		"GetLeak requires backend service integration")
}

// GetTechnique Get technique details.
func (c *Client) GetTechnique(ctx context.Context, name string) (*TechniqueEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "bing-prompt-leak",
		"GetTechnique requires backend service integration")
}

// Search Search leak archive.
func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]LeakEntry, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "bing-prompt-leak", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "bing-prompt-leak",
		"Search requires backend service integration")
}

// GetTechniques List all techniques.
func (c *Client) GetTechniques(ctx context.Context) ([]TechniqueEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "bing-prompt-leak",
		"GetTechniques requires backend service integration")
}

// Export Export archive.
func (c *Client) Export(ctx context.Context, format string) ([]byte, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "bing-prompt-leak",
		"Export requires backend service integration")
}

