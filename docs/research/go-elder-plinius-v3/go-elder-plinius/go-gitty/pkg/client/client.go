// Package client provides the Go client for the Gitty library.
// Go library for Gitty providing AI-powered Git assistance including commit message generation, code review, PR description writing, branch naming suggestions, and repository analysis.
//
// Basic usage:
//
//	import gitty "github.com/elder-plinius/go-gitty/pkg/client"
//
//	client, err := gitty.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-gitty/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the Gitty service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Gitty client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("gitty", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gitty",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gitty",
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

// GenerateCommitMessage Generate commit message from diff.
func (c *Client) GenerateCommitMessage(ctx context.Context, opts CommitOptions) (*CommitMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gitty", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "gitty",
		"GenerateCommitMessage requires backend service integration")
}

// ReviewCode Review code changes.
func (c *Client) ReviewCode(ctx context.Context, opts ReviewOptions) (*ReviewResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "gitty", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "gitty",
		"ReviewCode requires backend service integration")
}

// GeneratePRDescription Generate PR description.
func (c *Client) GeneratePRDescription(ctx context.Context, diff string, title string) (string, error) {
	return "", errors.New(errors.ErrCodeUnimplemented, "gitty",
		"GeneratePRDescription requires backend service integration")
}

// SuggestBranchName Suggest branch names.
func (c *Client) SuggestBranchName(ctx context.Context, description string) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gitty",
		"SuggestBranchName requires backend service integration")
}

// AnalyzeRepo Analyze repository.
func (c *Client) AnalyzeRepo(ctx context.Context, repoPath string) (*RepoStats, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "gitty",
		"AnalyzeRepo requires backend service integration")
}

// GenerateChangelog Generate changelog.
func (c *Client) GenerateChangelog(ctx context.Context, commits []string, style string) (string, error) {
	return "", errors.New(errors.ErrCodeUnimplemented, "gitty",
		"GenerateChangelog requires backend service integration")
}
