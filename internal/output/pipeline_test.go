package output

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewPipeline ---

func TestNewPipeline(t *testing.T) {
	p := NewPipeline()
	require.NotNil(t, p)

	// Verify default parsers are registered
	for _, name := range []string{"code", "diff", "json", "markdown", "text"} {
		assert.Contains(t, p.parsers, name, "parser %q should be registered", name)
	}

	// Verify default formatters are registered
	for _, name := range []string{"syntax", "diff", "table", "raw"} {
		assert.Contains(t, p.formatters, name, "formatter %q should be registered", name)
	}

	// Verify default renderers are registered
	for _, name := range []string{"terminal", "html", "json"} {
		assert.Contains(t, p.renderers, name, "renderer %q should be registered", name)
	}

	assert.NotNil(t, p.terminalUI)
}

// --- RegisterParser ---

func TestPipeline_RegisterParser_Custom(t *testing.T) {
	p := NewPipeline()

	custom := &TextParser{} // reuse TextParser as a custom parser
	p.RegisterParser("custom", custom)

	assert.Contains(t, p.parsers, "custom")
}

func TestPipeline_RegisterParser_Override(t *testing.T) {
	p := NewPipeline()

	override := &TextParser{}
	p.RegisterParser("code", override)

	// The code parser should now be our override instance
	assert.Equal(t, override, p.parsers["code"])
}

// --- RegisterFormatter ---

func TestPipeline_RegisterFormatter_Custom(t *testing.T) {
	p := NewPipeline()

	custom := &RawFormatter{}
	p.RegisterFormatter("custom-fmt", custom)

	assert.Contains(t, p.formatters, "custom-fmt")
}

// --- RegisterRenderer ---

func TestPipeline_RegisterRenderer_Custom(t *testing.T) {
	p := NewPipeline()

	custom := &JSONRenderer{}
	p.RegisterRenderer("custom-render", custom)

	assert.Contains(t, p.renderers, "custom-render")
}

// --- DefaultOptions ---

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	require.NotNil(t, opts)
	assert.Equal(t, "terminal", opts.OutputType)
	assert.True(t, opts.FormatOptions.LineNumbers)
	assert.Equal(t, 80, opts.FormatOptions.Width)
	assert.Equal(t, "dracula", opts.FormatOptions.ColorScheme)
}

// --- Parser unit tests (table-driven) ---

func TestParsers_Parse(t *testing.T) {
	tests := []struct {
		name       string
		parser     Parser
		input      []byte
		wantType   string
		wantErrNil bool
	}{
		{
			name:       "CodeParser_Go",
			parser:     &CodeParser{},
			input:      []byte("package main\n\nfunc main() {}"),
			wantType:   "code",
			wantErrNil: true,
		},
		{
			name:       "CodeParser_Python",
			parser:     &CodeParser{},
			input:      []byte("def hello():\n    pass"),
			wantType:   "code",
			wantErrNil: true,
		},
		{
			name:       "CodeParser_JavaScript",
			parser:     &CodeParser{},
			input:      []byte("const x = 42;"),
			wantType:   "code",
			wantErrNil: true,
		},
		{
			name:       "CodeParser_PlainText",
			parser:     &CodeParser{},
			input:      []byte("just some random text"),
			wantType:   "code",
			wantErrNil: true,
		},
		{
			name:       "DiffParser",
			parser:     &DiffParser{},
			input:      []byte("--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-old\n+new"),
			wantType:   "diff",
			wantErrNil: true,
		},
		{
			name:       "JSONParser_Valid",
			parser:     &JSONParser{},
			input:      []byte(`{"key":"value","num":42}`),
			wantType:   "json",
			wantErrNil: true,
		},
		{
			name:       "MarkdownParser",
			parser:     &MarkdownParser{},
			input:      []byte("# Title\n\nSome text **bold**"),
			wantType:   "markdown",
			wantErrNil: true,
		},
		{
			name:       "TextParser",
			parser:     &TextParser{},
			input:      []byte("Hello, World!"),
			wantType:   "text",
			wantErrNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			parsed, err := tt.parser.Parse(ctx, tt.input)
			if tt.wantErrNil {
				require.NoError(t, err)
				require.NotNil(t, parsed)
				assert.Equal(t, tt.wantType, parsed.Type)
				assert.NotNil(t, parsed.Content)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestJSONParser_Parse_InvalidJSON(t *testing.T) {
	p := &JSONParser{}
	ctx := context.Background()
	_, err := p.Parse(ctx, []byte("not json at all{{{"))
	assert.Error(t, err)
}

// --- detectLanguage ---

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Go", "package main\n\nfunc main() {}", "go"},
		{"Python_def", "def hello():\n    pass", "python"},
		{"Python_import", "import os\nimport sys", "python"},
		{"JavaScript_function", "function greet() { return 'hi'; }", "javascript"},
		{"JavaScript_const", "const x = 42;", "javascript"},
		{"PlainText", "just random text here", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguage([]byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Process (end-to-end through pipeline) ---

func TestPipeline_Process_TextToTerminal(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("Hello, World!"),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Content, "Hello, World!")
}

func TestPipeline_Process_CodeToTerminal(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "code",
		Data: []byte("package main\n\nfunc main() {}"),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	// Code output should contain something (syntax-highlighted content)
	assert.NotEmpty(t, out.Content)
}

func TestPipeline_Process_DiffToTerminal(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "diff",
		Data: []byte("--- a/file.go\n+++ b/file.go\n-old line\n+new line"),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.Content)
}

func TestPipeline_Process_MarkdownToTerminal(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "markdown",
		Data: []byte("# Heading\n\nSome **bold** text"),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NotEmpty(t, out.Content)
}

func TestPipeline_Process_JSONToTerminal_Panics(t *testing.T) {
	// The SyntaxFormatter default branch does content.Content.(string)
	// but JSONParser returns map[string]interface{}, causing a panic.
	// This documents the known limitation.
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "json",
		Data: []byte(`{"key":"value"}`),
	}

	assert.Panics(t, func() {
		_, _ = p.Process(ctx, input, nil)
	})
}

func TestPipeline_Process_JSONWithRawFormatter(t *testing.T) {
	// Demonstrate that JSON content works when the text parser is used
	// (unknown type falls back to text, which returns a string Content)
	p := NewPipeline()
	ctx := context.Background()

	// Use text type with JSON data — this works because TextParser
	// returns string Content
	input := &Input{
		Type: "text",
		Data: []byte(`{"key":"value"}`),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Content, `{"key":"value"}`)
}

func TestPipeline_Process_WithNilOptions(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("test content"),
	}

	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Content, "test content")
}

func TestPipeline_Process_WithCustomOptions(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("custom options test"),
	}

	opts := &Options{
		OutputType: "terminal",
		FormatOptions: FormatOptions{
			LineNumbers: false,
			Width:       120,
			ColorScheme: "monokai",
		},
	}

	out, err := p.Process(ctx, input, opts)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Content, "custom options test")
}

// --- Process error paths ---

func TestPipeline_Process_UnknownParserFallsBackToText(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "unknown-type",
		Data: []byte("fallback content"),
	}

	// Unknown parser type should fall back to text parser
	out, err := p.Process(ctx, input, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Content, "fallback content")
}

func TestPipeline_Process_UnknownRendererReturnsError(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("test"),
	}

	opts := &Options{
		OutputType: "nonexistent-renderer",
	}

	_, err := p.Process(ctx, input, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render")
	assert.Contains(t, err.Error(), "unknown output type")
}

func TestPipeline_Process_NoFormatterReturnsError(t *testing.T) {
	// Create a pipeline with no syntax formatter
	p := NewPipeline()
	delete(p.formatters, "syntax")
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("test"),
	}

	_, err := p.Process(ctx, input, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
	assert.Contains(t, err.Error(), "no formatter available")
}

// --- Renderers (table-driven) ---

func TestHTMLRenderer_Render_WithHTMLContent(t *testing.T) {
	r := &HTMLRenderer{}
	ctx := context.Background()

	content := &FormattedContent{
		Type:    "text",
		Format:  "html",
		Content: "plain text",
		HTML:    "<b>Bold</b>",
	}

	var buf strings.Builder
	err := r.Render(ctx, content, &buf)
	require.NoError(t, err)
	assert.Equal(t, "<b>Bold</b>", buf.String())
}

func TestHTMLRenderer_Render_WithoutHTMLContent(t *testing.T) {
	r := &HTMLRenderer{}
	ctx := context.Background()

	content := &FormattedContent{
		Type:    "text",
		Format:  "terminal",
		Content: "<script>alert('xss')</script>",
	}

	var buf strings.Builder
	err := r.Render(ctx, content, &buf)
	require.NoError(t, err)
	// Content should be escaped inside <pre>
	assert.Contains(t, buf.String(), "<pre>")
	assert.Contains(t, buf.String(), "&lt;script&gt;")
	assert.NotContains(t, buf.String(), "<script>")
}

func TestJSONRenderer_Render(t *testing.T) {
	r := &JSONRenderer{}
	ctx := context.Background()

	content := &FormattedContent{
		Type:    "code",
		Format:  "terminal",
		Content: "hello world",
	}

	var buf strings.Builder
	err := r.Render(ctx, content, &buf)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(buf.String()), &result)
	require.NoError(t, err)
	assert.Equal(t, "code", result["type"])
	assert.Equal(t, "terminal", result["format"])
	assert.Equal(t, "hello world", result["content"])
}

func TestTerminalRenderer_Render(t *testing.T) {
	p := NewPipeline()
	r := p.renderers["terminal"]
	require.NotNil(t, r)
	ctx := context.Background()

	content := &FormattedContent{
		Type:    "text",
		Format:  "terminal",
		Content: "terminal output test",
	}

	var buf strings.Builder
	err := r.Render(ctx, content, &buf)
	require.NoError(t, err)
	assert.Equal(t, "terminal output test", buf.String())
}

// --- Process to HTML renderer ---

func TestPipeline_Process_ToHTMLRenderer(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("html render test"),
	}

	opts := &Options{
		OutputType: "html",
	}

	out, err := p.Process(ctx, input, opts)
	require.NoError(t, err)
	require.NotNil(t, out)
	// The content goes through SyntaxFormatter (default case) then
	// HTMLRenderer which wraps in <pre> tags
	assert.Contains(t, out.Content, "html render test")
}

// --- Process to JSON renderer ---

func TestPipeline_Process_ToJSONRenderer(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	input := &Input{
		Type: "text",
		Data: []byte("json render test"),
	}

	opts := &Options{
		OutputType: "json",
	}

	out, err := p.Process(ctx, input, opts)
	require.NoError(t, err)
	require.NotNil(t, out)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(out.Content), &result)
	require.NoError(t, err)
	assert.Equal(t, "json render test", result["content"])
}

// --- Formatter unit tests ---

func TestSyntaxFormatter_Format_Code(t *testing.T) {
	p := NewPipeline()
	f := p.formatters["syntax"]
	require.NotNil(t, f)
	ctx := context.Background()

	content := &ParsedContent{
		Type:     "code",
		Language: "go",
		Content:  "package main",
	}

	result, err := f.Format(ctx, content, FormatOptions{LineNumbers: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "code", result.Type)
	assert.Equal(t, "terminal", result.Format)
}

func TestSyntaxFormatter_Format_Diff(t *testing.T) {
	p := NewPipeline()
	f := p.formatters["syntax"]
	require.NotNil(t, f)
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "diff",
		Content: "-removed\n+added",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "diff", result.Type)
}

func TestSyntaxFormatter_Format_Markdown(t *testing.T) {
	p := NewPipeline()
	f := p.formatters["syntax"]
	require.NotNil(t, f)
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "markdown",
		Content: "# Title\nParagraph",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "markdown", result.Type)
}

func TestSyntaxFormatter_Format_Default(t *testing.T) {
	p := NewPipeline()
	f := p.formatters["syntax"]
	require.NotNil(t, f)
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "text",
		Content: "plain text",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "text", result.Type)
	assert.Equal(t, "terminal", result.Format)
	assert.Equal(t, "plain text", result.Content)
}

func TestRawFormatter_Format(t *testing.T) {
	f := &RawFormatter{}
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "text",
		Content: "raw content",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "text", result.Type)
	assert.Equal(t, "raw", result.Format)
	assert.Equal(t, "raw content", result.Content)
}

func TestTableFormatter_Format(t *testing.T) {
	f := &TableFormatter{}
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "table",
		Content: "col1\tcol2\nval1\tval2",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "table", result.Type)
	assert.Equal(t, "terminal", result.Format)
}

func TestDiffFormatter_Format(t *testing.T) {
	p := NewPipeline()
	f := p.formatters["diff"]
	require.NotNil(t, f)
	ctx := context.Background()

	content := &ParsedContent{
		Type:    "diff",
		Content: "-old\n+new",
	}

	result, err := f.Format(ctx, content, FormatOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "diff", result.Type)
}

// --- ProcessStream ---

func TestPipeline_ProcessStream_BasicFlow(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	inputCh := make(chan *Input, 3)
	outputCh := make(chan *Output, 3)

	inputCh <- &Input{Type: "text", Data: []byte("first")}
	inputCh <- &Input{Type: "text", Data: []byte("second")}
	inputCh <- &Input{Type: "text", Data: []byte("third")}
	close(inputCh)

	err := p.ProcessStream(ctx, inputCh, nil, outputCh)
	require.NoError(t, err)

	var outputs []*Output
	for out := range outputCh {
		outputs = append(outputs, out)
	}

	assert.Len(t, outputs, 3)
	assert.Contains(t, outputs[0].Content, "first")
	assert.Contains(t, outputs[1].Content, "second")
	assert.Contains(t, outputs[2].Content, "third")
}

func TestPipeline_ProcessStream_ContextCancellation(t *testing.T) {
	p := NewPipeline()
	ctx, cancel := context.WithCancel(context.Background())

	inputCh := make(chan *Input)
	outputCh := make(chan *Output, 10)

	// Cancel context before sending input
	cancel()

	err := p.ProcessStream(ctx, inputCh, nil, outputCh)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestPipeline_ProcessStream_EmptyChannel(t *testing.T) {
	p := NewPipeline()
	ctx := context.Background()

	inputCh := make(chan *Input)
	outputCh := make(chan *Output, 10)

	close(inputCh)

	err := p.ProcessStream(ctx, inputCh, nil, outputCh)
	require.NoError(t, err)

	// Output channel should be closed with no items
	var count int
	for range outputCh {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestPipeline_ProcessStream_ErrorInProcessing(t *testing.T) {
	p := NewPipeline()
	// Remove the syntax formatter to trigger error
	delete(p.formatters, "syntax")
	ctx := context.Background()

	inputCh := make(chan *Input, 1)
	outputCh := make(chan *Output, 1)

	inputCh <- &Input{Type: "text", Data: []byte("will fail")}
	close(inputCh)

	err := p.ProcessStream(ctx, inputCh, nil, outputCh)
	require.NoError(t, err) // ProcessStream itself doesn't error on processing failures

	var outputs []*Output
	for out := range outputCh {
		outputs = append(outputs, out)
	}

	require.Len(t, outputs, 1)
	assert.NotEmpty(t, outputs[0].Error)
	assert.Contains(t, outputs[0].Error, "no formatter available")
}

func TestPipeline_ProcessStream_OutputContextCancellation(t *testing.T) {
	p := NewPipeline()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	inputCh := make(chan *Input, 1)
	// Use an unbuffered output channel so sending blocks
	outputCh := make(chan *Output)

	inputCh <- &Input{Type: "text", Data: []byte("blocks on output")}
	// Don't close inputCh — let context timeout handle it

	err := p.ProcessStream(ctx, inputCh, nil, outputCh)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// --- Input / Output structs ---

func TestInput_Struct(t *testing.T) {
	input := &Input{
		Type: "code",
		Data: []byte("package main"),
		Metadata: map[string]interface{}{
			"filename": "main.go",
		},
	}

	assert.Equal(t, "code", input.Type)
	assert.Equal(t, []byte("package main"), input.Data)
	assert.Equal(t, "main.go", input.Metadata["filename"])
}

func TestOutput_Struct(t *testing.T) {
	out := &Output{
		Content:  "rendered content",
		HTML:     "<p>rendered</p>",
		JSON:     map[string]interface{}{"key": "value"},
		Metadata: map[string]interface{}{"source": "test"},
		Error:    "",
	}

	assert.Equal(t, "rendered content", out.Content)
	assert.Equal(t, "<p>rendered</p>", out.HTML)
	assert.Equal(t, "value", out.JSON["key"])
	assert.Equal(t, "test", out.Metadata["source"])
	assert.Empty(t, out.Error)
}

// --- ParsedContent / FormattedContent structs ---

func TestParsedContent_Struct(t *testing.T) {
	pc := &ParsedContent{
		Type:     "code",
		Language: "go",
		Content:  "package main",
		Metadata: map[string]interface{}{"lines": 1},
	}

	assert.Equal(t, "code", pc.Type)
	assert.Equal(t, "go", pc.Language)
	assert.Equal(t, "package main", pc.Content)
	assert.Equal(t, 1, pc.Metadata["lines"])
}

func TestFormattedContent_Struct(t *testing.T) {
	fc := &FormattedContent{
		Type:    "code",
		Format:  "terminal",
		Content: "highlighted code",
		HTML:    "<pre>highlighted code</pre>",
		JSON:    map[string]interface{}{"type": "code"},
	}

	assert.Equal(t, "code", fc.Type)
	assert.Equal(t, "terminal", fc.Format)
	assert.Equal(t, "highlighted code", fc.Content)
	assert.Equal(t, "<pre>highlighted code</pre>", fc.HTML)
	assert.Equal(t, "code", fc.JSON["type"])
}

// --- FormatOptions ---

func TestFormatOptions_Struct(t *testing.T) {
	opts := FormatOptions{
		Language:    "go",
		LineNumbers: true,
		Width:       120,
		ColorScheme: "monokai",
		ShowDiff:    true,
		Highlight:   []string{"line1", "line2"},
	}

	assert.Equal(t, "go", opts.Language)
	assert.True(t, opts.LineNumbers)
	assert.Equal(t, 120, opts.Width)
	assert.Equal(t, "monokai", opts.ColorScheme)
	assert.True(t, opts.ShowDiff)
	assert.Len(t, opts.Highlight, 2)
}
