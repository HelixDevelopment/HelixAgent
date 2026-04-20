package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrCodeUnavailable, "autotemp", "service is down")
	assert.Equal(t, ErrCodeUnavailable, err.Code)
	assert.Equal(t, "autotemp", err.Service)
	assert.Equal(t, "service is down", err.Message)
	assert.True(t, err.Retryable)
	assert.Equal(t, "[autotemp:UNAVAILABLE] service is down", err.Error())
}

func TestNewWithCause(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := New(ErrCodeConnection, "autotemp", "failed to connect").WithCause(cause)
	assert.Equal(t, "[autotemp:CONNECTION_ERROR] failed to connect: connection refused", err.Error())
	assert.Equal(t, cause, err.Unwrap())
}

func TestNewf(t *testing.T) {
	err := Newf(ErrCodeInvalidArgument, "autotemp", "invalid temperature: %f", 1.5)
	assert.Equal(t, "invalid temperature: 1.500000", err.Message)
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("network error")
	err := Wrap(ErrCodeConnection, "autotemp", "request failed", cause)
	assert.Equal(t, cause, err.Unwrap())
}

func TestWithDetail(t *testing.T) {
	err := New(ErrCodeInvalidArgument, "autotemp", "bad request").
		WithDetail("field", "temperature").
		WithDetail("value", 2.5)
	assert.Equal(t, "temperature", err.Details["field"])
	assert.Equal(t, 2.5, err.Details["value"])
}

func TestIs(t *testing.T) {
	err := New(ErrCodeUnavailable, "autotemp", "down")
	assert.True(t, Is(err, ErrCodeUnavailable))
	assert.False(t, Is(err, ErrCodeInternal))

	// Test with wrapped error
	wrapped := fmt.Errorf("outer: %w", err)
	assert.True(t, Is(wrapped, ErrCodeUnavailable))
}

func TestIsRetryableError(t *testing.T) {
	assert.True(t, IsRetryableError(New(ErrCodeUnavailable, "test", "down")))
	assert.True(t, IsRetryableError(New(ErrCodeTimeout, "test", "timeout")))
	assert.False(t, IsRetryableError(New(ErrCodeInvalidArgument, "test", "bad")))
	assert.False(t, IsRetryableError(New(ErrCodeNotFound, "test", "missing")))

	// Sentinel errors
	assert.True(t, IsRetryableError(fmt.Errorf("some error: %w", ErrConnection)))
	assert.True(t, IsRetryableError(fmt.Errorf("some error: %w", ErrTimeout)))
}

func TestMustBePliniusError(t *testing.T) {
	pe := New(ErrCodeInternal, "autotemp", "oops")
	result := MustBePliniusError(pe)
	assert.NotNil(t, result)
	assert.Equal(t, ErrCodeInternal, result.Code)

	// Standard error
	result2 := MustBePliniusError(fmt.Errorf("plain error"))
	assert.Nil(t, result2)
}

func TestErrorCodesRetryableDefaults(t *testing.T) {
	tests := []struct {
		code      ErrorCode
		retryable bool
	}{
		{ErrCodeUnavailable, true},
		{ErrCodeResourceExhausted, true},
		{ErrCodeAborted, true},
		{ErrCodeTimeout, true},
		{ErrCodeConnection, true},
		{ErrCodeInternal, true},
		{ErrCodeInvalidArgument, false},
		{ErrCodeNotFound, false},
		{ErrCodeAlreadyExists, false},
		{ErrCodePermissionDenied, false},
		{ErrCodeUnauthenticated, false},
		{ErrCodeUnimplemented, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := New(tt.code, "test", "test")
			assert.Equal(t, tt.retryable, err.Retryable)
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	rootCause := errors.New("root cause")
	err := New(ErrCodeInternal, "test", "failure").WithCause(rootCause)

	unwrapped := errors.Unwrap(err)
	assert.Equal(t, rootCause, unwrapped)

	// errors.Is should work through the chain
	assert.True(t, errors.Is(err, rootCause))
}
