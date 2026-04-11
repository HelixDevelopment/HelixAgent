package containers

import (
	"digital.vasic.containers/pkg/logging"
	"github.com/sirupsen/logrus"
)

// LogrusLogger adapts a logrus.Logger to the containers logging.Logger interface
type LogrusLogger struct {
	logger *logrus.Logger
}

// NewLogrusLogger creates a new LogrusLogger wrapper
func NewLogrusLogger(logger *logrus.Logger) logging.Logger {
	if logger == nil {
		logger = logrus.New()
	}
	return &LogrusLogger{logger: logger}
}

// Debug logs a debug-level message
func (l *LogrusLogger) Debug(msg string, args ...any) {
	l.logger.Debugf(msg, args...)
}

// Info logs an info-level message
func (l *LogrusLogger) Info(msg string, args ...any) {
	l.logger.Infof(msg, args...)
}

// Warn logs a warning-level message
func (l *LogrusLogger) Warn(msg string, args ...any) {
	l.logger.Warnf(msg, args...)
}

// Error logs an error-level message
func (l *LogrusLogger) Error(msg string, args ...any) {
	l.logger.Errorf(msg, args...)
}
