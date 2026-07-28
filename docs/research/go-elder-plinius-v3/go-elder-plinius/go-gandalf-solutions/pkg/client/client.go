// Package client provides the Go client for the Gandalf Solutions library.
// Go library providing structured access to solutions for Lakera's Gandalf prompt hacking game (levels 1-8 + adventures), including system prompt leak techniques, emoji encoding solutions, reverse Gandalf strategies, and extracted system prompts from Gandalf the White.
//
// Basic usage:
//
//	import gandalf-solutions "github.com/elder-plinius/go-gandalf-solutions/pkg/client"
//
//	client, err := gandalf-solutions.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-gandalf-solutions/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the Gandalf Solutions service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Gandalf Solutions client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("gandalf-solutions", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gandalf-solutions",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gandalf-solutions",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// Close gracefully closes the client.
func (c *Client) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config { return c.cfg }

// GetLevel Get solution for a specific level.
func (c *Client) GetLevel(ctx context.Context, level int) (*LevelSolution, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetLevel requires backend service integration")
}

// GetAdventure Get adventure solution.
func (c *Client) GetAdventure(ctx context.Context, name string) (*AdventureSolution, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetAdventure requires backend service integration")
}

// SearchSolutions Search solutions.
func (c *Client) SearchSolutions(ctx context.Context, opts SearchOptions) ([]LevelSolution, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gandalf-solutions", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"SearchSolutions requires backend service integration")
}

// GetPromptLeaks Get prompt leaks by source.
func (c *Client) GetPromptLeaks(ctx context.Context, source string) ([]PromptLeak, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetPromptLeaks requires backend service integration")
}

// GetTechniques List available techniques.
func (c *Client) GetTechniques(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetTechniques requires backend service integration")
}

// GetCategories List categories.
func (c *Client) GetCategories(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetCategories requires backend service integration")
}

// GetArchiveStats Get archive statistics.
func (c *Client) GetArchiveStats(ctx context.Context) (*ArchiveStats, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"GetArchiveStats requires backend service integration")
}

// ExportLevel Export level solutions.
func (c *Client) ExportLevel(ctx context.Context, level int, format string) ([]byte, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gandalf-solutions",
		"ExportLevel requires backend service integration")
}
