package containers

import (
	"bytes"
	"testing"

	"digital.vasic.containers/pkg/logging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewLogrusLogger ---

func TestNewLogrusLogger(t *testing.T) {
	logger := logrus.New()
	adapter := NewLogrusLogger(logger)
	require.NotNil(t, adapter)

	// Should implement logging.Logger interface
	var _ logging.Logger = adapter
}

func TestNewLogrusLogger_NilLogger(t *testing.T) {
	// Passing nil should create a default logrus logger internally
	adapter := NewLogrusLogger(nil)
	require.NotNil(t, adapter)

	var _ logging.Logger = adapter
}

// --- Logging methods ---

func TestLogrusLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Debug("debug message: %s", "test")

	assert.Contains(t, buf.String(), "debug message: test")
}

func TestLogrusLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.InfoLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Info("info message: %d", 42)

	assert.Contains(t, buf.String(), "info message: 42")
}

func TestLogrusLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.WarnLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Warn("warn message: %v", true)

	assert.Contains(t, buf.String(), "warn message: true")
}

func TestLogrusLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.ErrorLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Error("error message: %s %d", "code", 500)

	assert.Contains(t, buf.String(), "error message: code 500")
}

// --- Level filtering ---

func TestLogrusLogger_DebugFiltered_AtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.InfoLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Debug("should not appear")

	assert.NotContains(t, buf.String(), "should not appear")
}

func TestLogrusLogger_InfoFiltered_AtWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.WarnLevel)

	adapter := NewLogrusLogger(logger)
	adapter.Info("should not appear")

	assert.NotContains(t, buf.String(), "should not appear")
}

// --- Format string variations ---

func TestLogrusLogger_FormatStrings(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
		args   []any
		expect string
	}{
		{"no_args", "info", "simple message", nil, "simple message"},
		{"string_arg", "info", "hello %s", []any{"world"}, "hello world"},
		{"int_arg", "info", "count: %d", []any{99}, "count: 99"},
		{"multi_args", "info", "%s has %d items", []any{"list", 5}, "list has 5 items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&buf)
			logger.SetLevel(logrus.DebugLevel)

			adapter := NewLogrusLogger(logger)
			adapter.Info(tt.format, tt.args...)

			assert.Contains(t, buf.String(), tt.expect)
		})
	}
}
