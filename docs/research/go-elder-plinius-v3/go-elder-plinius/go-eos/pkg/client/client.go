// Package client provides the Go client for the Eos library.
// Go client for Eos -- a Discord bot that orchestrates open-source developers across multiple servers, facilitating recruitment, skill matching, project discovery, and notifications.
//
// Basic usage:
//
//	import eos "github.com/elder-plinius/go-eos/pkg/client"
//
//	client, err := eos.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-eos/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the Eos service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Eos client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("eos", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos",
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

// Authenticate Authenticate user via Discord.
func (c *Client) Authenticate(ctx context.Context, opts AuthenticateOptions) (*AuthenticateResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "eos",
		"Authenticate requires backend service integration")
}

// MatchSkills Match user skills to projects.
func (c *Client) MatchSkills(ctx context.Context, opts MatchSkillsOptions) (*MatchSkillsResponse, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "eos",
		"MatchSkills requires backend service integration")
}

// DiscoverProjects Discover projects by criteria.
func (c *Client) DiscoverProjects(ctx context.Context, opts DiscoverOptions) ([]*Project, int, error) {
	if err := opts.Validate(); err != nil {
		return nil, 0, errors.Wrap(errors.ErrCodeInvalidArgument, "eos", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, 0, errors.New(errors.ErrCodeUnimplemented, "eos",
		"DiscoverProjects requires backend service integration")
}

// JoinProject Join a project.
func (c *Client) JoinProject(ctx context.Context, opts JoinProjectOptions) (*JoinResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "eos",
		"JoinProject requires backend service integration")
}

// GetProject Get project details.
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "eos",
		"GetProject requires backend service integration")
}

// OnboardUser Complete user onboarding.
func (c *Client) OnboardUser(ctx context.Context, opts OnboardOptions) (*OnboardResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "eos", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "eos",
		"OnboardUser requires backend service integration")
}
