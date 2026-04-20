// Package errors defines custom error types and error handling utilities
// for all Plinius Go service clients. It provides structured errors with
// error codes, retry hints, and causal chains.
//
// All errors from Plinius services should be wrapped using these types
// to provide consistent error handling across services.
package errors

import (
	"errors"
	"fmt"
)

// ErrorCode is a typed string representing an error category.
type ErrorCode string

const (
	// ErrCodeUnavailable indicates the service is temporarily unavailable.
	ErrCodeUnavailable ErrorCode = "UNAVAILABLE"

	// ErrCodeInvalidArgument indicates the request was malformed or invalid.
	ErrCodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"

	// ErrCodeNotFound indicates the requested resource was not found.
	ErrCodeNotFound ErrorCode = "NOT_FOUND"

	// ErrCodeAlreadyExists indicates the resource already exists.
	ErrCodeAlreadyExists ErrorCode = "ALREADY_EXISTS"

	// ErrCodePermissionDenied indicates insufficient permissions.
	ErrCodePermissionDenied ErrorCode = "PERMISSION_DENIED"

	// ErrCodeUnauthenticated indicates missing or invalid credentials.
	ErrCodeUnauthenticated ErrorCode = "UNAUTHENTICATED"

	// ErrCodeResourceExhausted indicates rate limiting or quota exceeded.
	ErrCodeResourceExhausted ErrorCode = "RESOURCE_EXHAUSTED"

	// ErrCodeFailedPrecondition indicates a precondition was not met.
	ErrCodeFailedPrecondition ErrorCode = "FAILED_PRECONDITION"

	// ErrCodeAborted indicates the operation was aborted.
	ErrCodeAborted ErrorCode = "ABORTED"

	// ErrCodeOutOfRange indicates a value is outside the valid range.
	ErrCodeOutOfRange ErrorCode = "OUT_OF_RANGE"

	// ErrCodeUnimplemented indicates the operation is not implemented.
	ErrCodeUnimplemented ErrorCode = "UNIMPLEMENTED"

	// ErrCodeInternal indicates an internal server error.
	ErrCodeInternal ErrorCode = "INTERNAL"

	// ErrCodeUnknown indicates an unknown error occurred.
	ErrCodeUnknown ErrorCode = "UNKNOWN"

	// ErrCodeTimeout indicates the operation timed out.
	ErrCodeTimeout ErrorCode = "TIMEOUT"

	// ErrCodeCancelled indicates the operation was cancelled by the caller.
	ErrCodeCancelled ErrorCode = "CANCELLED"

	// ErrCodeConnection indicates a connection error occurred.
	ErrCodeConnection ErrorCode = "CONNECTION_ERROR"
)

// PliniusError is the standard error type for all Plinius service errors.
// It provides structured error information including an error code,
// human-readable message, retry hint, and optional cause.
type PliniusError struct {
	// Code is the error code categorizing the error.
	Code ErrorCode `json:"code"`

	// Message is a human-readable description of the error.
	Message string `json:"message"`

	// Service is the name of the service that generated the error.
	Service string `json:"service"`

	// Retryable indicates whether the operation can be retried.
	Retryable bool `json:"retryable"`

	// RetryAfter suggests a minimum duration to wait before retrying.
	RetryAfterSeconds int `json:"retry_after_seconds,omitempty"`

	// Details contains additional structured error information.
	Details map[string]interface{} `json:"details,omitempty"`

	// cause is the underlying error that caused this error.
	cause error
}

// Error implements the error interface.
func (e *PliniusError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Service, e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Service, e.Code, e.Message)
}

// Unwrap returns the underlying cause of the error.
func (e *PliniusError) Unwrap() error {
	return e.cause
}

// WithCause adds a cause to the error.
func (e *PliniusError) WithCause(cause error) *PliniusError {
	e.cause = cause
	return e
}

// WithDetail adds a detail key-value pair to the error.
func (e *PliniusError) WithDetail(key string, value interface{}) *PliniusError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// IsRetryable returns true if the error indicates the operation can be retried.
func (e *PliniusError) IsRetryable() bool {
	return e.Retryable
}

// New creates a new PliniusError.
func New(code ErrorCode, service, message string) *PliniusError {
	return &PliniusError{
		Code:      code,
		Service:   service,
		Message:   message,
		Retryable: isDefaultRetryable(code),
	}
}

// Wrap wraps an existing error as a PliniusError.
func Wrap(code ErrorCode, service, message string, cause error) *PliniusError {
	return New(code, service, message).WithCause(cause)
}

// Newf creates a new PliniusError with a formatted message.
func Newf(code ErrorCode, service, format string, args ...interface{}) *PliniusError {
	return New(code, service, fmt.Sprintf(format, args...))
}

// Is checks if an error (or any error in its chain) is a PliniusError
// with the given error code.
func Is(err error, code ErrorCode) bool {
	var pe *PliniusError
	if errors.As(err, &pe) {
		return pe.Code == code
	}
	return false
}

// IsRetryableError checks if an error indicates the operation can be retried.
func IsRetryableError(err error) bool {
	var pe *PliniusError
	if errors.As(err, &pe) {
		return pe.IsRetryable()
	}
	// Default: connection errors and timeouts are retryable
	if errors.Is(err, ErrConnection) || errors.Is(err, ErrTimeout) {
		return true
	}
	return false
}

// Predefined sentinel errors for common cases.
var (
	ErrConnection = fmt.Errorf("connection error")
	ErrTimeout    = fmt.Errorf("timeout")
	ErrCancelled  = fmt.Errorf("cancelled")
)

// isDefaultRetryable determines the default retryability for an error code.
func isDefaultRetryable(code ErrorCode) bool {
	switch code {
	case ErrCodeUnavailable, ErrCodeResourceExhausted, ErrCodeAborted,
		ErrCodeTimeout, ErrCodeConnection:
		return true
	case ErrCodeInvalidArgument, ErrCodeNotFound, ErrCodeAlreadyExists,
		ErrCodePermissionDenied, ErrCodeUnauthenticated, ErrCodeUnimplemented,
		ErrCodeOutOfRange:
		return false
	case ErrCodeInternal, ErrCodeFailedPrecondition, ErrCodeUnknown:
		return true // Conservative: these might be transient
	default:
		return false
	}
}

// MustBePliniusError extracts a PliniusError from the error chain.
// Returns nil if no PliniusError is found.
func MustBePliniusError(err error) *PliniusError {
	var pe *PliniusError
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}
