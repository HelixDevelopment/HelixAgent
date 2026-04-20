// Package client provides the Go client for the I-LLM library.
// Go library for I-LLM providing interactive LLM conversation patterns, chain-of-thought templates, ReAct agent implementations, and structured reasoning frameworks.
//
// Basic usage:
//
//	import i-llm "github.com/elder-plinius/go-i-llm/pkg/client"
//
//	client, err := i-llm.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-i-llm/pkg/types"
)

// Client is the Go client for the I-LLM service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new I-LLM client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("i-llm", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "i-llm",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "i-llm",
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

// GetPattern Get conversation pattern by ID.
func (c *Client) GetPattern(ctx context.Context, id string) (*ConversationPattern, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"GetPattern requires backend service integration")
}

// ListPatterns List patterns by category.
func (c *Client) ListPatterns(ctx context.Context, category string) ([]ConversationPattern, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"ListPatterns requires backend service integration")
}

// RenderPattern Render pattern with variables.
func (c *Client) RenderPattern(ctx context.Context, pattern ConversationPattern, vars map[string]string) (string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"RenderPattern requires backend service integration")
}

// CreateAgent Create ReAct agent.
func (c *Client) CreateAgent(ctx context.Context, cfg AgentConfig) (*Agent, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "i-llm", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"CreateAgent requires backend service integration")
}

// RunChain Run prompt chain.
func (c *Client) RunChain(ctx context.Context, chain PromptChain, inputs map[string]string) (*ChainResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"RunChain requires backend service integration")
}

// ChainOfThought Generate chain-of-thought reasoning.
func (c *Client) ChainOfThought(ctx context.Context, problem string, model string) (*ChainResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"ChainOfThought requires backend service integration")
}

// TreeOfThought Generate tree-of-thought exploration.
func (c *Client) TreeOfThought(ctx context.Context, problem string, model string, breadth int) (*TreeResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"TreeOfThought requires backend service integration")
}

// GetCategories List pattern categories.
func (c *Client) GetCategories(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "i-llm",
		"GetCategories requires backend service integration")
}

