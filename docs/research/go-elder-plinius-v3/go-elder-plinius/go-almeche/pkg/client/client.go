// Package client provides the Go client for the AlmechE library.
// Go client for AlmechE -- the Idea-to-Object speech-to-CAD generation service. Transforms spoken descriptions into physical 3D models ready for 3D printing.
//
// Basic usage:
//
//	import almeche "github.com/elder-plinius/go-almeche/pkg/client"
//
//	client, err := almeche.New()
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
package client

import (
	"context"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
	. "github.com/elder-plinius/go-almeche/pkg/types"
)

// Client is the Go client for the AlmechE service.
type Client struct {
	cfg    *config.Config
	closed bool
}

// New creates a new AlmechE client.
func New(opts ...config.Option) (*Client, error) {
	cfg := config.New("almeche", opts...)
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche",
			"invalid configuration", err)
	}
	return &Client{cfg: cfg}, nil
}

// NewFromConfig creates a client from a config object.
func NewFromConfig(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche",
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

// ProcessSpeech Speech-to-CAD pipeline.
func (c *Client) ProcessSpeech(ctx context.Context, opts ProcessSpeechOptions) (*GenerateCADResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"ProcessSpeech requires backend service integration")
}

// GenerateCAD Generate CAD from text.
func (c *Client) GenerateCAD(ctx context.Context, opts GenerateCADOptions) (*GenerateCADResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"GenerateCAD requires backend service integration")
}

// ExportModel Export model to format.
func (c *Client) ExportModel(ctx context.Context, opts ExportOptions) (*ExportResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"ExportModel requires backend service integration")
}

// TextToSpeech Convert text to speech.
func (c *Client) TextToSpeech(ctx context.Context, opts TextToSpeechOptions) (*TextToSpeechResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"TextToSpeech requires backend service integration")
}

// GetAvailableMaterials Get 3D printing materials.
func (c *Client) GetAvailableMaterials(ctx context.Context) ([]Material, error) {
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"GetAvailableMaterials requires backend service integration")
}

// EstimateCost Estimate print cost.
func (c *Client) EstimateCost(ctx context.Context, opts EstimateOptions) (*EstimateResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrCodeInvalidArgument, "almeche", "invalid parameters", err)
	}
	opts.Defaults()
	return nil, errors.New(errors.ErrCodeUnimplemented, "almeche",
		"EstimateCost requires backend service integration")
}

