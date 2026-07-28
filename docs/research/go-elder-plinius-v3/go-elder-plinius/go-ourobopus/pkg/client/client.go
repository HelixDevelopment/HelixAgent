// Package client provides the Go client for the ourobopus library.
// Go library for ourobopus implementing self-referential AI patterns including recursive self-improvement, metacognitive reasoning, self-evaluation loops, and feedback-driven prompt refinement.
//
// Basic usage:
//
//	import ourobopus "github.com/elder-plinius/go-ourobopus/pkg/client"
//
//	client, err := ourobopus.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-ourobopus/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the ourobopus service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new ourobopus client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("ourobopus", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "ourobopus",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "ourobopus",
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

// SelfReflect Generate self-reflection.
func (c *Client) SelfReflect(ctx context.Context, prompt string, model string) (*SelfReflection, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "ourobopus",
		"SelfReflect requires backend service integration")
}

// Refine Iteratively refine prompt.
func (c *Client) Refine(ctx context.Context, cfg RefinementConfig) (*RefinementResult, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "ourobopus", "invalid parameters", err)
	}
	cfg.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "ourobopus",
		"Refine requires backend service integration")
}

// MetaEvaluate Meta-evaluate prompt-output pair.
func (c *Client) MetaEvaluate(ctx context.Context, prompt string, output string, criteria []string) (*MetaEvaluation, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "ourobopus",
		"MetaEvaluate requires backend service integration")
}

// SelfImprove Self-improving prompt loop.
func (c *Client) SelfImprove(ctx context.Context, prompt string, model string, iterations int) (*RefinementResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "ourobopus",
		"SelfImprove requires backend service integration")
}

// GetMetaPatterns Get available meta-patterns.
func (c *Client) GetMetaPatterns(ctx context.Context) ([]MetaPrompt, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "ourobopus",
		"GetMetaPatterns requires backend service integration")
}
