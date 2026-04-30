package native

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"dev.helix.agent/internal/formatters"
	"github.com/sirupsen/logrus"
)

// NativeFormatter implements a formatter using a native binary
type NativeFormatter struct {
	*formatters.BaseFormatter
	binaryPath string
	args       []string
	stdinFlag  bool // whether to append `-` arg AND wire stdin (most CLIs)
	// stdinNoDash: when true, wire stdin but DO NOT append `-`. Required by
	// gofmt and similar tools that treat `-` as a filename rather than
	// "read stdin". CONST-035 §c bug fix from 2026-04-30: previously
	// gofmt was constructed with stdinFlag=true so the executor ran
	// `gofmt -` which lstat'd a file named `-` and exited 2. Result: every
	// /v1/format request for Go silently failed with `success:false`.
	stdinNoDash bool
	logger      *logrus.Logger
}

// NewNativeFormatter creates a new native binary formatter that wires
// stdin AND appends `-` to args when stdinFlag is true. This is the
// common convention (prettier --, stylua -, taplo fmt -).
func NewNativeFormatter(
	metadata *formatters.FormatterMetadata,
	binaryPath string,
	args []string,
	stdinFlag bool,
	logger *logrus.Logger,
) *NativeFormatter {
	return &NativeFormatter{
		BaseFormatter: formatters.NewBaseFormatter(metadata),
		binaryPath:    binaryPath,
		args:          args,
		stdinFlag:     stdinFlag,
		logger:        logger,
	}
}

// NewNativeFormatterStdinNoDash creates a native formatter that wires
// stdin WITHOUT appending `-`. Required for tools like gofmt that read
// stdin when given no filename arguments and treat `-` as a literal
// filename. Fixes CONST-035 §c "every /v1/format Go request silently
// fails" bug.
func NewNativeFormatterStdinNoDash(
	metadata *formatters.FormatterMetadata,
	binaryPath string,
	args []string,
	logger *logrus.Logger,
) *NativeFormatter {
	return &NativeFormatter{
		BaseFormatter: formatters.NewBaseFormatter(metadata),
		binaryPath:    binaryPath,
		args:          args,
		stdinFlag:     false,
		stdinNoDash:   true,
		logger:        logger,
	}
}

// Format formats code using the native binary
func (n *NativeFormatter) Format(ctx context.Context, req *formatters.FormatRequest) (*formatters.FormatResult, error) {
	start := time.Now()

	// Build command
	cmdArgs := n.buildArgs(req)
	cmd := exec.CommandContext(ctx, n.binaryPath, cmdArgs...)

	// Set stdin if supported (either stdinFlag with `-` OR stdinNoDash)
	if n.stdinFlag || n.stdinNoDash {
		cmd.Stdin = strings.NewReader(req.Content)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()
	duration := time.Since(start)

	// Handle errors
	if err != nil {
		return &formatters.FormatResult{
			Success:          false,
			Error:            fmt.Errorf("formatter execution failed: %w (stderr: %s)", err, stderr.String()),
			FormatterName:    n.Name(),
			FormatterVersion: n.Version(),
			Duration:         duration,
		}, nil // Return result with error, not an error
	}

	// Get formatted content
	formattedContent := stdout.String()

	// Check if content changed
	changed := formattedContent != req.Content

	// Calculate stats
	stats := &formatters.FormatStats{
		LinesTotal:   strings.Count(req.Content, "\n") + 1,
		LinesChanged: computeLineChanges(req.Content, formattedContent),
		BytesTotal:   len(req.Content),
		BytesChanged: len(formattedContent) - len(req.Content),
	}

	return &formatters.FormatResult{
		Content:          formattedContent,
		Changed:          changed,
		FormatterName:    n.Name(),
		FormatterVersion: n.Version(),
		Duration:         duration,
		Success:          true,
		Stats:            stats,
	}, nil
}

// FormatBatch formats multiple requests (default implementation)
func (n *NativeFormatter) FormatBatch(ctx context.Context, reqs []*formatters.FormatRequest) ([]*formatters.FormatResult, error) {
	results := make([]*formatters.FormatResult, len(reqs))

	for i, req := range reqs {
		result, err := n.Format(ctx, req)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return results, nil
}

// HealthCheck checks if the formatter binary is available
func (n *NativeFormatter) HealthCheck(ctx context.Context) error {
	// Check if binary exists
	cmd := exec.CommandContext(ctx, n.binaryPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("formatter binary not available: %w", err)
	}

	return nil
}

// buildArgs builds command arguments based on the request
func (n *NativeFormatter) buildArgs(req *formatters.FormatRequest) []string {
	args := make([]string, len(n.args))
	copy(args, n.args)

	// Add stdin flag if needed
	if n.stdinFlag {
		args = append(args, "-")
	}

	// Add check-only flag if supported and requested
	if req.CheckOnly && n.SupportsCheck() {
		args = append(args, "--check")
	}

	return args
}

// computeLineChanges calculates the number of lines changed between original and formatted content
func computeLineChanges(original, formatted string) int {
	if original == formatted {
		return 0
	}

	origLines := strings.Split(original, "\n")
	formattedLines := strings.Split(formatted, "\n")

	changed := 0
	maxLen := len(origLines)
	if len(formattedLines) > maxLen {
		maxLen = len(formattedLines)
	}

	for i := 0; i < maxLen; i++ {
		var origLine, formattedLine string
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(formattedLines) {
			formattedLine = formattedLines[i]
		}
		if origLine != formattedLine {
			changed++
		}
	}

	return changed
}
