// Package output provides output formatting and rendering for HelixAgent.
package output

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"

	"dev.helix.agent/internal/clis/claude_code"
)

// Pipeline manages output formatting and rendering.
type Pipeline struct {
	// Parsers for different content types
	parsers map[string]Parser

	// Formatters for different formats
	formatters map[string]Formatter

	// Renderers for different output types
	renderers map[string]Renderer

	// Default terminal UI
	terminalUI *claude_code.TerminalUI
}

// Parser parses raw content into structured form.
type Parser interface {
	Parse(ctx context.Context, data []byte) (*ParsedContent, error)
}

// Formatter formats parsed content.
type Formatter interface {
	Format(ctx context.Context, content *ParsedContent, opts FormatOptions) (*FormattedContent, error)
}

// Renderer renders formatted content to output.
type Renderer interface {
	Render(ctx context.Context, content *FormattedContent, w io.Writer) error
}

// ParsedContent represents parsed content.
type ParsedContent struct {
	Type     string
	Language string
	Content  interface{}
	Metadata map[string]interface{}
}

// FormattedContent represents formatted content.
type FormattedContent struct {
	Type    string
	Format  string
	Content string
	HTML    string
	JSON    map[string]interface{}
}

// FormatOptions contains formatting options.
type FormatOptions struct {
	Language    string
	LineNumbers bool
	Width       int
	ColorScheme string
	ShowDiff    bool
	Highlight   []string
}

// NewPipeline creates a new output pipeline.
func NewPipeline() *Pipeline {
	p := &Pipeline{
		parsers:    make(map[string]Parser),
		formatters: make(map[string]Formatter),
		renderers:  make(map[string]Renderer),
		terminalUI: claude_code.NewTerminalUI(),
	}

	// Register default parsers
	p.RegisterParser("code", &CodeParser{})
	p.RegisterParser("diff", &DiffParser{})
	p.RegisterParser("json", &JSONParser{})
	p.RegisterParser("markdown", &MarkdownParser{})
	p.RegisterParser("text", &TextParser{})

	// Register default formatters
	p.RegisterFormatter("syntax", &SyntaxFormatter{ui: p.terminalUI})
	p.RegisterFormatter("diff", &DiffFormatter{ui: p.terminalUI})
	p.RegisterFormatter("table", &TableFormatter{})
	p.RegisterFormatter("raw", &RawFormatter{})

	// Register default renderers
	p.RegisterRenderer("terminal", &TerminalRenderer{ui: p.terminalUI})
	p.RegisterRenderer("html", &HTMLRenderer{})
	p.RegisterRenderer("json", &JSONRenderer{})

	return p
}

// RegisterParser registers a parser.
func (p *Pipeline) RegisterParser(contentType string, parser Parser) {
	p.parsers[contentType] = parser
}

// RegisterFormatter registers a formatter.
func (p *Pipeline) RegisterFormatter(format string, formatter Formatter) {
	p.formatters[format] = formatter
}

// RegisterRenderer registers a renderer.
func (p *Pipeline) RegisterRenderer(outputType string, renderer Renderer) {
	p.renderers[outputType] = renderer
}

// Process processes content through the pipeline.
func (p *Pipeline) Process(
	ctx context.Context,
	input *Input,
	opts *Options,
) (*Output, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// 1. Parse input
	parsed, err := p.parse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// 2. Format content
	formatted, err := p.format(ctx, parsed, opts.FormatOptions)
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}

	// 3. Render output
	var buf strings.Builder
	if err := p.render(ctx, formatted, opts.OutputType, &buf); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	return &Output{
		Content:  buf.String(),
		HTML:     formatted.HTML,
		JSON:     formatted.JSON,
		Metadata: parsed.Metadata,
	}, nil
}

// ProcessStream processes content as a stream.
func (p *Pipeline) ProcessStream(
	ctx context.Context,
	input <-chan *Input,
	opts *Options,
	output chan<- *Output,
) error {
	defer close(output)

	for {
		select {
		case in, ok := <-input:
			if !ok {
				return nil
			}

			out, err := p.Process(ctx, in, opts)
			if err != nil {
				// Send error as special output
				output <- &Output{
					Error: err.Error(),
				}
				continue
			}

			select {
			case output <- out:
			case <-ctx.Done():
				return ctx.Err()
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// parse parses input content.
func (p *Pipeline) parse(ctx context.Context, input *Input) (*ParsedContent, error) {
	parser, ok := p.parsers[input.Type]
	if !ok {
		// Fallback to text parser
		parser = p.parsers["text"]
	}

	return parser.Parse(ctx, input.Data)
}

// format formats parsed content.
func (p *Pipeline) format(
	ctx context.Context,
	parsed *ParsedContent,
	opts FormatOptions,
) (*FormattedContent, error) {
	formatter, ok := p.formatters["syntax"]
	if !ok {
		return nil, fmt.Errorf("no formatter available")
	}

	return formatter.Format(ctx, parsed, opts)
}

// render renders formatted content.
func (p *Pipeline) render(
	ctx context.Context,
	formatted *FormattedContent,
	outputType string,
	w io.Writer,
) error {
	renderer, ok := p.renderers[outputType]
	if !ok {
		return fmt.Errorf("unknown output type: %s", outputType)
	}

	return renderer.Render(ctx, formatted, w)
}

// Input represents pipeline input.
type Input struct {
	Type     string
	Data     []byte
	Metadata map[string]interface{}
}

// Options represents pipeline options.
type Options struct {
	OutputType    string
	FormatOptions FormatOptions
}

// DefaultOptions returns default options.
func DefaultOptions() *Options {
	return &Options{
		OutputType: "terminal",
		FormatOptions: FormatOptions{
			LineNumbers: true,
			Width:       80,
			ColorScheme: "dracula",
		},
	}
}

// Output represents pipeline output.
type Output struct {
	Content  string
	HTML     string
	JSON     map[string]interface{}
	Metadata map[string]interface{}
	Error    string
}

// Parser implementations

// CodeParser parses code content.
type CodeParser struct{}

// Parse parses code.
func (p *CodeParser) Parse(ctx context.Context, data []byte) (*ParsedContent, error) {
	// Detect language from content
	lang := detectLanguage(data)

	return &ParsedContent{
		Type:     "code",
		Language: lang,
		Content:  string(data),
	}, nil
}

// DiffParser parses diff content.
type DiffParser struct{}

// Parse parses diff.
func (p *DiffParser) Parse(ctx context.Context, data []byte) (*ParsedContent, error) {
	return &ParsedContent{
		Type:    "diff",
		Content: string(data),
	}, nil
}

// JSONParser parses JSON content.
type JSONParser struct{}

// Parse parses JSON.
func (p *JSONParser) Parse(ctx context.Context, data []byte) (*ParsedContent, error) {
	var content interface{}
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, err
	}

	return &ParsedContent{
		Type:    "json",
		Content: content,
	}, nil
}

// MarkdownParser parses markdown content.
type MarkdownParser struct{}

// Parse parses markdown.
func (p *MarkdownParser) Parse(ctx context.Context, data []byte) (*ParsedContent, error) {
	return &ParsedContent{
		Type:    "markdown",
		Content: string(data),
	}, nil
}

// TextParser parses plain text.
type TextParser struct{}

// Parse parses text.
func (p *TextParser) Parse(ctx context.Context, data []byte) (*ParsedContent, error) {
	return &ParsedContent{
		Type:    "text",
		Content: string(data),
	}, nil
}

// Formatter implementations

// SyntaxFormatter formats with syntax highlighting.
type SyntaxFormatter struct {
	ui *claude_code.TerminalUI
}

// Format formats with syntax highlighting.
func (f *SyntaxFormatter) Format(
	ctx context.Context,
	content *ParsedContent,
	opts FormatOptions,
) (*FormattedContent, error) {
	switch content.Type {
	case "code":
		return &FormattedContent{
			Type:    "code",
			Format:  "terminal",
			Content: f.ui.RenderCodeBlock(content.Content.(string), content.Language, opts.LineNumbers),
		}, nil

	case "diff":
		return &FormattedContent{
			Type:    "diff",
			Format:  "terminal",
			Content: f.ui.RenderDiff("", content.Content.(string)),
		}, nil

	case "markdown":
		return &FormattedContent{
			Type:    "markdown",
			Format:  "terminal",
			Content: f.ui.RenderMarkdown(content.Content.(string)),
		}, nil

	default:
		return &FormattedContent{
			Type:    content.Type,
			Format:  "terminal",
			Content: content.Content.(string),
		}, nil
	}
}

// DiffFormatter formats diffs.
type DiffFormatter struct {
	ui *claude_code.TerminalUI
}

// Format formats a diff.
func (f *DiffFormatter) Format(
	ctx context.Context,
	content *ParsedContent,
	opts FormatOptions,
) (*FormattedContent, error) {
	return &FormattedContent{
		Type:    "diff",
		Format:  "terminal",
		Content: f.ui.RenderDiff("", content.Content.(string)),
	}, nil
}

// TableFormatter formats tables.
type TableFormatter struct{}

// Format formats a table.
func (f *TableFormatter) Format(
	ctx context.Context,
	content *ParsedContent,
	opts FormatOptions,
) (*FormattedContent, error) {
	// Implementation would format table data
	return &FormattedContent{
		Type:    "table",
		Format:  "terminal",
		Content: content.Content.(string),
	}, nil
}

// RawFormatter passes content through unchanged.
type RawFormatter struct{}

// Format formats raw content.
func (f *RawFormatter) Format(
	ctx context.Context,
	content *ParsedContent,
	opts FormatOptions,
) (*FormattedContent, error) {
	return &FormattedContent{
		Type:    content.Type,
		Format:  "raw",
		Content: content.Content.(string),
	}, nil
}

// Renderer implementations

// TerminalRenderer renders to terminal.
type TerminalRenderer struct {
	ui *claude_code.TerminalUI
}

// Render renders to terminal.
func (r *TerminalRenderer) Render(
	ctx context.Context,
	content *FormattedContent,
	w io.Writer,
) error {
	_, err := fmt.Fprint(w, content.Content)
	return err
}

// HTMLRenderer renders to HTML.
type HTMLRenderer struct{}

// Render renders to HTML.
func (r *HTMLRenderer) Render(
	ctx context.Context,
	content *FormattedContent,
	w io.Writer,
) error {
	if content.HTML != "" {
		_, err := fmt.Fprint(w, content.HTML)
		return err
	}

	// Convert content to HTML
	html := fmt.Sprintf("<pre>%s</pre>", html.EscapeString(content.Content))
	_, err := fmt.Fprint(w, html)
	return err
}

// JSONRenderer renders to JSON.
type JSONRenderer struct{}

// Render renders to JSON.
func (r *JSONRenderer) Render(
	ctx context.Context,
	content *FormattedContent,
	w io.Writer,
) error {
	data := map[string]interface{}{
		"type":    content.Type,
		"format":  content.Format,
		"content": content.Content,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Helper functions

func detectLanguage(data []byte) string {
	// Simple language detection based on file content patterns
	content := string(data)

	// Check for Go
	if strings.Contains(content, "package ") && strings.Contains(content, "func ") {
		return "go"
	}

	// Check for Python
	if strings.Contains(content, "def ") || strings.Contains(content, "import ") {
		return "python"
	}

	// Check for JavaScript/TypeScript
	if strings.Contains(content, "function ") || strings.Contains(content, "const ") {
		return "javascript"
	}

	return "text"
}
