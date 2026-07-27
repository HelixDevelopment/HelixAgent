// Package framework provides comprehensive logging for the challenges system.
package framework

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents logging severity levels.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of a log level.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// JSONLogger implements Logger with JSON Lines output.
type JSONLogger struct {
	mu             sync.Mutex
	output         io.Writer
	apiRequestLog  io.Writer
	apiResponseLog io.Writer
	level          LogLevel
	fields         map[string]any
	verbose        bool
	closed         bool
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// LoggerConfig configures the logger.
type LoggerConfig struct {
	OutputPath     string
	APIRequestLog  string
	APIResponseLog string
	Level          LogLevel
	Verbose        bool
	Fields         map[string]any
}

// NewJSONLogger creates a new JSON logger.
func NewJSONLogger(config LoggerConfig) (*JSONLogger, error) {
	logger := &JSONLogger{
		level:   config.Level,
		verbose: config.Verbose,
		fields:  config.Fields,
	}

	if logger.fields == nil {
		logger.fields = make(map[string]any)
	}

	// Open main output file
	if config.OutputPath != "" {
		if err := os.MkdirAll(filepath.Dir(config.OutputPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		file, err := os.OpenFile(config.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.output = file
	} else {
		logger.output = os.Stdout
	}

	// Open API request log
	if config.APIRequestLog != "" {
		file, err := os.OpenFile(config.APIRequestLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open API request log: %w", err)
		}
		logger.apiRequestLog = file
	}

	// Open API response log
	if config.APIResponseLog != "" {
		file, err := os.OpenFile(config.APIResponseLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open API response log: %w", err)
		}
		logger.apiResponseLog = file
	}

	return logger, nil
}

// log writes a log entry.
func (l *JSONLogger) log(level LogLevel, msg string, fields ...Field) {
	if l.closed {
		return
	}

	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Fields:    make(map[string]any),
	}

	// Add default fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Add provided fields
	for _, f := range fields {
		entry.Fields[f.Key] = f.Value
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	fmt.Fprintln(l.output, string(data))
}

// Info logs an informational message.
func (l *JSONLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *JSONLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *JSONLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// Debug logs a debug message (only if verbose).
func (l *JSONLogger) Debug(msg string, fields ...Field) {
	if l.verbose {
		l.log(LevelDebug, msg, fields...)
	}
}

// WithFields returns a logger with additional default fields.
func (l *JSONLogger) WithFields(fields ...Field) Logger {
	newFields := make(map[string]any)
	for k, v := range l.fields {
		newFields[k] = v
	}
	for _, f := range fields {
		newFields[f.Key] = f.Value
	}

	return &JSONLogger{
		output:         l.output,
		apiRequestLog:  l.apiRequestLog,
		apiResponseLog: l.apiResponseLog,
		level:          l.level,
		verbose:        l.verbose,
		fields:         newFields,
	}
}

// LogAPIRequest logs an API request.
func (l *JSONLogger) LogAPIRequest(request APIRequestLog) {
	if l.apiRequestLog == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(request)
	if err != nil {
		return
	}

	fmt.Fprintln(l.apiRequestLog, string(data))
}

// LogAPIResponse logs an API response.
func (l *JSONLogger) LogAPIResponse(response APIResponseLog) {
	if l.apiResponseLog == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	fmt.Fprintln(l.apiResponseLog, string(data))
}

// Close closes the logger and flushes buffers.
func (l *JSONLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true

	// Close file handles
	var errs []error

	if closer, ok := l.output.(io.Closer); ok && l.output != os.Stdout {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if closer, ok := l.apiRequestLog.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if closer, ok := l.apiResponseLog.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// MultiLogger writes to multiple loggers.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple destinations.
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Info logs to all loggers.
func (m *MultiLogger) Info(msg string, fields ...Field) {
	for _, l := range m.loggers {
		l.Info(msg, fields...)
	}
}

// Warn logs to all loggers.
func (m *MultiLogger) Warn(msg string, fields ...Field) {
	for _, l := range m.loggers {
		l.Warn(msg, fields...)
	}
}

// Error logs to all loggers.
func (m *MultiLogger) Error(msg string, fields ...Field) {
	for _, l := range m.loggers {
		l.Error(msg, fields...)
	}
}

// Debug logs to all loggers.
func (m *MultiLogger) Debug(msg string, fields ...Field) {
	for _, l := range m.loggers {
		l.Debug(msg, fields...)
	}
}

// WithFields returns a logger with additional fields.
func (m *MultiLogger) WithFields(fields ...Field) Logger {
	var newLoggers []Logger
	for _, l := range m.loggers {
		newLoggers = append(newLoggers, l.WithFields(fields...))
	}
	return &MultiLogger{loggers: newLoggers}
}

// LogAPIRequest logs to all loggers.
func (m *MultiLogger) LogAPIRequest(request APIRequestLog) {
	for _, l := range m.loggers {
		l.LogAPIRequest(request)
	}
}

// LogAPIResponse logs to all loggers.
func (m *MultiLogger) LogAPIResponse(response APIResponseLog) {
	for _, l := range m.loggers {
		l.LogAPIResponse(response)
	}
}

// Close closes all loggers.
func (m *MultiLogger) Close() error {
	var lastErr error
	for _, l := range m.loggers {
		if err := l.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ConsoleLogger provides colored console output.
type ConsoleLogger struct {
	mu      sync.Mutex
	output  io.Writer
	verbose bool
	fields  map[string]any
}

// NewConsoleLogger creates a console logger.
func NewConsoleLogger(verbose bool) *ConsoleLogger {
	return &ConsoleLogger{
		output:  os.Stdout,
		verbose: verbose,
		fields:  make(map[string]any),
	}
}

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

func (c *ConsoleLogger) log(level LogLevel, color, msg string, fields ...Field) {
	c.mu.Lock()
	defer c.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	levelStr := level.String()

	var fieldStr string
	if len(fields) > 0 {
		parts := make([]string, 0, len(fields))
		for _, f := range fields {
			parts = append(parts, fmt.Sprintf("%s=%v", f.Key, f.Value))
		}
		fieldStr = " " + colorGray + fmt.Sprintf("{%s}", joinStrings(parts, ", ")) + colorReset
	}

	fmt.Fprintf(c.output, "%s%s%s [%s%-5s%s] %s%s\n",
		colorGray, timestamp, colorReset,
		color, levelStr, colorReset,
		msg, fieldStr)
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// Info logs an info message.
func (c *ConsoleLogger) Info(msg string, fields ...Field) {
	c.log(LevelInfo, colorBlue, msg, fields...)
}

// Warn logs a warning message.
func (c *ConsoleLogger) Warn(msg string, fields ...Field) {
	c.log(LevelWarn, colorYellow, msg, fields...)
}

// Error logs an error message.
func (c *ConsoleLogger) Error(msg string, fields ...Field) {
	c.log(LevelError, colorRed, msg, fields...)
}

// Debug logs a debug message.
func (c *ConsoleLogger) Debug(msg string, fields ...Field) {
	if c.verbose {
		c.log(LevelDebug, colorGray, msg, fields...)
	}
}

// WithFields returns a logger with additional fields.
func (c *ConsoleLogger) WithFields(fields ...Field) Logger {
	newFields := make(map[string]any)
	for k, v := range c.fields {
		newFields[k] = v
	}
	for _, f := range fields {
		newFields[f.Key] = f.Value
	}
	return &ConsoleLogger{
		output:  c.output,
		verbose: c.verbose,
		fields:  newFields,
	}
}

// LogAPIRequest logs an API request (simplified for console).
func (c *ConsoleLogger) LogAPIRequest(request APIRequestLog) {
	c.Info("API Request",
		Field{Key: "request_id", Value: request.RequestID},
		Field{Key: "method", Value: request.Method},
		Field{Key: "url", Value: request.URL},
	)
}

// LogAPIResponse logs an API response (simplified for console).
func (c *ConsoleLogger) LogAPIResponse(response APIResponseLog) {
	c.Info("API Response",
		Field{Key: "request_id", Value: response.RequestID},
		Field{Key: "status", Value: response.StatusCode},
		Field{Key: "time_ms", Value: response.ResponseTimeMs},
	)
}

// Close is a no-op for console logger.
func (c *ConsoleLogger) Close() error {
	return nil
}

// NullLogger discards all log output.
type NullLogger struct{}

func (NullLogger) Info(msg string, fields ...Field)       {}
func (NullLogger) Warn(msg string, fields ...Field)       {}
func (NullLogger) Error(msg string, fields ...Field)      {}
func (NullLogger) Debug(msg string, fields ...Field)      {}
func (NullLogger) WithFields(fields ...Field) Logger      { return NullLogger{} }
func (NullLogger) LogAPIRequest(request APIRequestLog)    {}
func (NullLogger) LogAPIResponse(response APIResponseLog) {}
func (NullLogger) Close() error                           { return nil }

// SetupLogging creates a logging configuration for a challenge.
func SetupLogging(logsDir string, verbose bool) (*JSONLogger, error) {
	config := LoggerConfig{
		OutputPath:     filepath.Join(logsDir, "challenge.log"),
		APIRequestLog:  filepath.Join(logsDir, "api_requests.log"),
		APIResponseLog: filepath.Join(logsDir, "api_responses.log"),
		Level:          LevelInfo,
		Verbose:        verbose,
	}

	if verbose {
		config.Level = LevelDebug
	}

	return NewJSONLogger(config)
}

// Helper function to create common fields.
func LogField(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Redacting logger wrapper that automatically redacts sensitive data.
type RedactingLogger struct {
	inner   Logger
	secrets []string
}

// NewRedactingLogger creates a logger that redacts sensitive strings.
func NewRedactingLogger(inner Logger, secrets ...string) *RedactingLogger {
	return &RedactingLogger{
		inner:   inner,
		secrets: secrets,
	}
}

func (r *RedactingLogger) redact(msg string) string {
	result := msg
	for _, secret := range r.secrets {
		if secret != "" && len(secret) > 4 {
			result = replaceAll(result, secret, RedactAPIKey(secret))
		}
	}
	return result
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	result := s
	for {
		i := indexOf(result, old)
		if i == -1 {
			break
		}
		result = result[:i] + new + result[i+len(old):]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (r *RedactingLogger) Info(msg string, fields ...Field) {
	r.inner.Info(r.redact(msg), r.redactFields(fields)...)
}

func (r *RedactingLogger) Warn(msg string, fields ...Field) {
	r.inner.Warn(r.redact(msg), r.redactFields(fields)...)
}

func (r *RedactingLogger) Error(msg string, fields ...Field) {
	r.inner.Error(r.redact(msg), r.redactFields(fields)...)
}

func (r *RedactingLogger) Debug(msg string, fields ...Field) {
	r.inner.Debug(r.redact(msg), r.redactFields(fields)...)
}

func (r *RedactingLogger) WithFields(fields ...Field) Logger {
	return &RedactingLogger{
		inner:   r.inner.WithFields(r.redactFields(fields)...),
		secrets: r.secrets,
	}
}

func (r *RedactingLogger) redactFields(fields []Field) []Field {
	result := make([]Field, len(fields))
	for i, f := range fields {
		if str, ok := f.Value.(string); ok {
			result[i] = Field{Key: f.Key, Value: r.redact(str)}
		} else {
			result[i] = f
		}
	}
	return result
}

func (r *RedactingLogger) LogAPIRequest(request APIRequestLog) {
	// Redact headers
	request.Headers = RedactHeaders(request.Headers)
	r.inner.LogAPIRequest(request)
}

func (r *RedactingLogger) LogAPIResponse(response APIResponseLog) {
	response.Headers = RedactHeaders(response.Headers)
	r.inner.LogAPIResponse(response)
}

func (r *RedactingLogger) Close() error {
	return r.inner.Close()
}
