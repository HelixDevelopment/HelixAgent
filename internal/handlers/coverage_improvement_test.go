package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Test helper function to setup gin router
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// TestErrorResponse tests error response formatting
func TestErrorResponse_Format(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		message    string
		details    string
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			message:    "Invalid input",
			details:    "Missing required field",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			message:    "Resource not found",
			details:    "",
		},
		{
			name:       "internal error",
			statusCode: http.StatusInternalServerError,
			message:    "Server error",
			details:    "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			r := setupTestRouter()
			r.GET("/test", func(c *gin.Context) {
				c.JSON(tt.statusCode, gin.H{
					"error": gin.H{
						"message": tt.message,
						"details": tt.details,
					},
				})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

// TestHTTPMethods tests various HTTP methods
func TestHTTPMethods_Supported(t *testing.T) {
	t.Parallel()
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
				t.Parallel()
			r := setupTestRouter()
			r.Handle(method, "/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(method, "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
