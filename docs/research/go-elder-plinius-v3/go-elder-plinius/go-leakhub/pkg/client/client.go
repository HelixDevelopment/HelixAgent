// Package client provides the Go client for the LEAKHUB library.
// Go library for LEAKHUB providing prompt leak detection, extraction, and archival. Identifies potential system prompt leaks in AI model responses and maintains a searchable archive of known leaks.
//
// Basic usage:
//
//	import leakhub "github.com/elder-plinius/go-leakhub/pkg/client"
//
//	client, err := leakhub.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	. "github.com/elder-plinius/go-leakhub/pkg/types"
	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is the Go client for the LEAKHUB service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new LEAKHUB client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("leakhub", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub",
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

// DetectLeak Detect prompt leaks in response.
func (c *Client) DetectLeak(ctx context.Context, opts DetectionOptions) (*DetectionResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "leakhub", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"DetectLeak requires backend service integration")
}

// SearchArchive Search leak archive.
func (c *Client) SearchArchive(ctx context.Context, query string, limit int) ([]LeakEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"SearchArchive requires backend service integration")
}

// AddToArchive Add leak entry to archive.
func (c *Client) AddToArchive(ctx context.Context, entry LeakEntry) error {
	return errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"AddToArchive requires backend service integration")
}

// GetByModel Get leaks for specific model.
func (c *Client) GetByModel(ctx context.Context, model string) ([]LeakEntry, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"GetByModel requires backend service integration")
}

// GetStats Get archive statistics.
func (c *Client) GetStats(ctx context.Context) (*ArchiveStats, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"GetStats requires backend service integration")
}

// ExportArchive Export archive to format.
func (c *Client) ExportArchive(ctx context.Context, format string) ([]byte, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "leakhub",
		"ExportArchive requires backend service integration")
}
