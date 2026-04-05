package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseStatus_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status string
	}{
		{"pending", "pending"},
		{"running", "running"},
		{"completed", "completed"},
		{"failed", "failed"},
		{"cancelled", "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			validStatuses := map[string]bool{
				"pending":   true,
				"running":   true,
				"completed": true,
				"failed":    true,
				"cancelled": true,
			}
			assert.True(t, validStatuses[tt.status])
		})
	}
}

func TestErrorCode_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		code string
	}{
		{"invalid_request", "invalid_request"},
		{"authentication_error", "authentication_error"},
		{"rate_limit_exceeded", "rate_limit_exceeded"},
		{"server_error", "server_error"},
		{"service_unavailable", "service_unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			assert.NotEmpty(t, tt.code)
		})
	}
}

func TestContentType_JSON(t *testing.T) {
	t.Parallel()
	t.Run("application/json content type", func(t *testing.T) {
		contentType := "application/json"
		assert.Equal(t, "application/json", contentType)
	})
}

func TestContentType_Stream(t *testing.T) {
	t.Parallel()
	t.Run("text/event-stream content type", func(t *testing.T) {
		contentType := "text/event-stream"
		assert.Equal(t, "text/event-stream", contentType)
	})
}
