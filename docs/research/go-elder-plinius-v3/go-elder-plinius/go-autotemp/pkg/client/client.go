// Package client provides the Go client for the AutoTemp library.
// Go client for the AutoTemp service -- intelligent temperature optimization for LLM interactions. Runs prompts at multiple temperatures and selects the best output using multi-judge structured scoring.
//
// Basic usage:
//
//	import autotemp "github.com/elder-plinius/go-autotemp/pkg/client"
//
//	client, err := autotemp.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-autotemp/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the AutoTemp service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new AutoTemp client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("autotemp", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp",
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

// Run Run temperature optimization.
func (c *Client) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "autotemp",
		"Run requires backend service integration")
}

// RunAdvanced Run UCB bandit optimization.
func (c *Client) RunAdvanced(ctx context.Context, opts AdvancedOptions) (*RunResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "autotemp",
		"RunAdvanced requires backend service integration")
}

// Evaluate Evaluate single output.
func (c *Client) Evaluate(ctx context.Context, opts EvaluateOptions) (*EvaluateResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "autotemp",
		"Evaluate requires backend service integration")
}

// Benchmark Batch evaluation.
func (c *Client) Benchmark(ctx context.Context, opts BenchmarkOptions) (*BenchmarkResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "autotemp", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "autotemp",
		"Benchmark requires backend service integration")
}
