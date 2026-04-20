// Package client provides the Go client for the Misc-Prompt-Hacks library.
// Go library providing structured access to prompt hacking challenge solutions from games like Lakera's Gandalf, TensorTrust, and other prompt injection benchmarks.
//
// Basic usage:
//
//	import misc-prompthacks "github.com/elder-plinius/go-misc-prompthacks/pkg/client"
//
//	client, err := misc-prompthacks.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-misc-prompthacks/pkg/types"
)

// Client is the Go client for the Misc-Prompt-Hacks service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Misc-Prompt-Hacks client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("misc-prompthacks", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "misc-prompthacks",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "misc-prompthacks",
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

// SearchSolutions Search challenge solutions.
func (c *Client) SearchSolutions(ctx context.Context, opts SearchOptions) ([]ChallengeSolution, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "misc-prompthacks", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "misc-prompthacks",
		"SearchSolutions requires backend service integration")
}

// GetByPlatform Get challenges by platform.
func (c *Client) GetByPlatform(ctx context.Context, platform string) ([]ChallengeEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "misc-prompthacks",
		"GetByPlatform requires backend service integration")
}

// GetByDifficulty Get solutions by difficulty.
func (c *Client) GetByDifficulty(ctx context.Context, difficulty string) ([]ChallengeSolution, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "misc-prompthacks",
		"GetByDifficulty requires backend service integration")
}

// GetPlatforms List all platforms.
func (c *Client) GetPlatforms(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "misc-prompthacks",
		"GetPlatforms requires backend service integration")
}

// GetTags List all tags.
func (c *Client) GetTags(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "misc-prompthacks",
		"GetTags requires backend service integration")
}

