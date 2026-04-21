// Package client provides the Go client for the Tempest library.
// Go library for Tempest providing AI-powered environmental context awareness, weather data integration for agent systems, and contextual prompt augmentation based on temporal and environmental conditions.
//
// Basic usage:
//
//	import tempest "github.com/elder-plinius/go-tempest/pkg/client"
//
//	client, err := tempest.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-tempest/pkg/types"
)

// Client is the Go client for the Tempest service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new Tempest client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("tempest", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "tempest",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "tempest",
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

// GetWeather Get weather data.
func (c *Client) GetWeather(ctx context.Context, location string) (*WeatherData, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "tempest",
		"GetWeather requires backend service integration")
}

// BuildContext Build environmental context.
func (c *Client) BuildContext(ctx context.Context, cfg ContextConfig) (*ContextResult, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "tempest",
		"BuildContext requires backend service integration")
}

// AugmentPrompt Augment prompt with context.
func (c *Client) AugmentPrompt(ctx context.Context, opts AugmentOptions) (string, error) {
	if err := opts.Validate(); err != nil {
		return "", errors.Wrap(errors.ErrCodeInvalidArgument, "tempest", "invalid parameters", err)
	}
	opts.Defaults()
	return "", errors.New(errors.ErrCodeUnimplemented, "tempest",
		"AugmentPrompt requires backend service integration")
}

// GetLocations List available locations.
func (c *Client) GetLocations(ctx context.Context) ([]string, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "tempest",
		"GetLocations requires backend service integration")
}

