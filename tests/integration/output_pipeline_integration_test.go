//go:build integration
// +build integration

// Package integration provides output pipeline integration tests.
// These tests verify that the Pipeline processes content end-to-end
// through real parsers, formatters, and renderers.
package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/output"
)

// TestPipeline_ProcessCode verifies that Go source code is parsed,
// syntax-formatted, and rendered through the full pipeline.
func TestPipeline_ProcessCode(t *testing.T) {
	p := output.NewPipeline()

	input := &output.Input{
		Type: "code",
		Data: []byte("func main() { fmt.Println(\"hello\") }"),
		Metadata: map[string]interface{}{
			"language": "go",
		},
	}

	opts := output.DefaultOptions()
	opts.FormatOptions.Language = "go"
	opts.OutputType = "json"

	result, err := p.Process(context.Background(), input, opts)
	require.NoError(t, err, "pipeline should process Go code without error")
	assert.NotNil(t, result, "result must not be nil")
	assert.NotEmpty(t, result.Content, "rendered content should not be empty")
}

// TestPipeline_ProcessJSON verifies JSON content round-trips through
// parse, format, and render stages.
func TestPipeline_ProcessJSON(t *testing.T) {
	p := output.NewPipeline()

	raw := []byte(`{"key":"value","count":42}`)
	input := &output.Input{
		Type: "json",
		Data: raw,
	}

	opts := output.DefaultOptions()
	opts.OutputType = "json"

	result, err := p.Process(context.Background(), input, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// TestPipeline_ProcessMarkdown verifies markdown content flows through
// the pipeline and produces non-empty output.
func TestPipeline_ProcessMarkdown(t *testing.T) {
	p := output.NewPipeline()

	md := []byte("# Title\n\nSome **bold** text and `code`.\n")
	input := &output.Input{
		Type: "markdown",
		Data: md,
	}

	opts := output.DefaultOptions()
	opts.OutputType = "html"

	result, err := p.Process(context.Background(), input, opts)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// TestPipeline_ProcessText verifies plain text is handled as the fallback
// content type and renders cleanly.
func TestPipeline_ProcessText(t *testing.T) {
	p := output.NewPipeline()

	input := &output.Input{
		Type: "text",
		Data: []byte("plain text content for pipeline test"),
	}

	result, err := p.Process(context.Background(), input, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

// TestPipeline_NilOptionsUsesDefaults verifies that passing nil options
// falls back to DefaultOptions without panicking.
func TestPipeline_NilOptionsUsesDefaults(t *testing.T) {
	p := output.NewPipeline()

	input := &output.Input{
		Type: "code",
		Data: []byte("package main"),
	}

	result, err := p.Process(context.Background(), input, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}
