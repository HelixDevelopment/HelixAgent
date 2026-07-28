// Package client provides the Go client for the ST3GG library.
// Go implementation of ST3GG providing 100+ steganography techniques for hiding data in images (LSB, DCT, DWT), audio (echo hiding, phase coding, spread spectrum), documents (whitespace, metadata), and network packets (TCP/IP headers, DNS tunnels).
//
// Basic usage:
//
//	import st3gg "github.com/elder-plinius/go-st3gg/pkg/client"
//
//	client, err := st3gg.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-st3gg/pkg/types"
)

// Client is the Go client for the ST3GG service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new ST3GG client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("st3gg", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "st3gg",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "st3gg",
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

// ListMethods List all steganography methods.
func (c *Client) ListMethods(ctx context.Context) ([]StegoMethod, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"ListMethods requires backend service integration")
}

// Embed Embed secret data in carrier.
func (c *Client) Embed(ctx context.Context, opts EmbedOptions) (*EmbedResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "st3gg", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"Embed requires backend service integration")
}

// Extract Extract secret from carrier.
func (c *Client) Extract(ctx context.Context, opts ExtractOptions) (*ExtractResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "st3gg", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"Extract requires backend service integration")
}

// Analyze Analyze carrier for hidden data.
func (c *Client) Analyze(ctx context.Context, carrier []byte) (*AnalyzeResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"Analyze requires backend service integration")
}

// GetCapacity Get embedding capacity.
func (c *Client) GetCapacity(ctx context.Context, carrier []byte, method string) (int64, error) {
	return 0, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"GetCapacity requires backend service integration")
}

// BatchEmbed Embed across multiple carriers.
func (c *Client) BatchEmbed(ctx context.Context, carriers [][]byte, secret []byte, method string) ([]EmbedResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"BatchEmbed requires backend service integration")
}

// CompareMethods Compare methods for given carrier.
func (c *Client) CompareMethods(ctx context.Context, carrier []byte, secret []byte) ([]MethodComparison, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "st3gg",
		"CompareMethods requires backend service integration")
}
