// Package client provides the Go client for the OBLITERATUS library.
// Go client for OBLITERATUS -- an advanced toolkit for understanding and removing refusal behaviors from large language models through abliteration (surgically removing refusal representations).
//
// Basic usage:
//
//	import obliteratus "github.com/elder-plinius/go-obliteratus/pkg/client"
//
//	client, err := obliteratus.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-obliteratus/pkg/types"
)

// Client is the Go client for the OBLITERATUS service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new OBLITERATUS client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("obliteratus", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "obliteratus",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "obliteratus",
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

// Obliterate Run model abliteration.
func (c *Client) Obliterate(ctx context.Context, opts ObliterateOptions) (error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "obliteratus", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "obliteratus",
		"Obliterate requires backend service integration")
}

// GetAvailableMethods Get all abliteration methods.
func (c *Client) GetAvailableMethods(ctx context.Context) ([]MethodInfo, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "obliteratus",
		"GetAvailableMethods requires backend service integration")
}

// GetMethodDetails Get method details.
func (c *Client) GetMethodDetails(ctx context.Context, method string) (*MethodInfo, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "obliteratus",
		"GetMethodDetails requires backend service integration")
}

// AnalyzeModel Analyze model without modifying.
func (c *Client) AnalyzeModel(ctx context.Context, opts ObliterateOptions) (*AnalysisResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "obliteratus", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "obliteratus",
		"AnalyzeModel requires backend service integration")
}

// CancelOperation Cancel running operation.
func (c *Client) CancelOperation(ctx context.Context, operationID string) (*CancelResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "obliteratus",
		"CancelOperation requires backend service integration")
}

