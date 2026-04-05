package formatters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecordRequestStart(t *testing.T) {
	t.Parallel()
	// Should return a completion function without panicking
	completeFn := RecordRequestStart("black", "python")
	assert.NotNil(t, completeFn)

	// Calling the completion function should not panic
	completeFn(true, 100*time.Millisecond, 1024)
}

func TestRecordRequestStart_Failure(t *testing.T) {
	t.Parallel()
	completeFn := RecordRequestStart("gofmt", "go")
	// Record a failed request
	completeFn(false, 50*time.Millisecond, 512)
}

func TestRecordCacheHit(t *testing.T) {
	t.Parallel()
	// Should not panic
	RecordCacheHit("black", "python")
	RecordCacheHit("gofmt", "go")
}

func TestRecordCacheMiss(t *testing.T) {
	t.Parallel()
	// Should not panic
	RecordCacheMiss("black", "python")
	RecordCacheMiss("gofmt", "go")
}

func TestSuccessToString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{"success", true, "true"},
		{"failure", false, "false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected, successToString(tc.input))
		})
	}
}
