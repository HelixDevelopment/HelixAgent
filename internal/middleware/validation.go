package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidationConfig defines validation parameters
type ValidationConfig struct {
	MaxBodySize     int64   // Maximum request body size in bytes
	MaxPromptLength int     // Maximum prompt length in characters
	MaxTokensLimit  int     // Maximum tokens limit that can be requested
	MinTemperature  float64 // Minimum temperature value
	MaxTemperature  float64 // Maximum temperature value
	MinTopP         float64 // Minimum top_p value
	MaxTopP         float64 // Maximum top_p value
	MaxStopSequences int    // Maximum number of stop sequences
	MaxMessagesCount int    // Maximum number of messages in a request
}

// DefaultValidationConfig returns sensible defaults for validation
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxBodySize:      10 * 1024 * 1024, // 10MB
		MaxPromptLength:  100000,           // 100k characters
		MaxTokensLimit:   32000,            // Most models support this
		MinTemperature:   0.0,
		MaxTemperature:   2.0,
		MinTopP:          0.0,
		MaxTopP:          1.0,
		MaxStopSequences: 10,
		MaxMessagesCount: 100,
	}
}

// ValidationError represents a validation error with field information
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// ValidationErrors holds multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(msgs, "; ")
}

// Add adds a validation error
func (e *ValidationErrors) Add(field, message string, value any) {
	e.Errors = append(e.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Validator provides request validation middleware
type Validator struct {
	config ValidationConfig
}

// NewValidator creates a new validator with the given config
func NewValidator(config ValidationConfig) *Validator {
	return &Validator{config: config}
}

// NewDefaultValidator creates a validator with default configuration
func NewDefaultValidator() *Validator {
	return NewValidator(DefaultValidationConfig())
}

// BodySizeMiddleware validates request body size
func (v *Validator) BodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > v.config.MaxBodySize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("request body too large: %d bytes exceeds maximum %d bytes",
						c.Request.ContentLength, v.config.MaxBodySize),
					"type": "invalid_request_error",
					"code": "body_too_large",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CompletionRequest represents the request structure for validation
type CompletionValidationRequest struct {
	Prompt      string                   `json:"prompt"`
	Messages    []MessageValidation      `json:"messages"`
	Model       string                   `json:"model"`
	Temperature *float64                 `json:"temperature"`
	MaxTokens   *int                     `json:"max_tokens"`
	TopP        *float64                 `json:"top_p"`
	Stop        []string                 `json:"stop"`
	Stream      bool                     `json:"stream"`
}

// MessageValidation represents a message for validation
type MessageValidation struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ValidateCompletionMiddleware validates completion request parameters
func (v *Validator) ValidateCompletionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "failed to read request body",
					"type":    "invalid_request_error",
					"code":    "read_error",
				},
			})
			c.Abort()
			return
		}

		// Restore body for subsequent handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Parse JSON
		var req CompletionValidationRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			var syntaxErr *json.SyntaxError
			var unmarshalErr *json.UnmarshalTypeError

			if errors.As(err, &syntaxErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("JSON syntax error at position %d", syntaxErr.Offset),
						"type":    "invalid_request_error",
						"code":    "json_parse_error",
					},
				})
			} else if errors.As(err, &unmarshalErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("invalid type for field '%s': expected %s",
							unmarshalErr.Field, unmarshalErr.Type.String()),
						"type":    "invalid_request_error",
						"code":    "type_error",
					},
				})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "invalid JSON format",
						"type":    "invalid_request_error",
						"code":    "json_parse_error",
					},
				})
			}
			c.Abort()
			return
		}

		// Validate fields
		validationErrors := v.validateCompletionRequest(&req)
		if validationErrors.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": validationErrors.Error(),
					"type":    "invalid_request_error",
					"code":    "validation_error",
					"details": validationErrors.Errors,
				},
			})
			c.Abort()
			return
		}

		// Restore body again for handler
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		c.Next()
	}
}

// validateCompletionRequest validates the completion request fields
func (v *Validator) validateCompletionRequest(req *CompletionValidationRequest) *ValidationErrors {
	errs := &ValidationErrors{}

	// Validate prompt or messages are present
	if req.Prompt == "" && len(req.Messages) == 0 {
		errs.Add("prompt", "either 'prompt' or 'messages' is required", nil)
	}

	// Validate prompt length
	if len(req.Prompt) > v.config.MaxPromptLength {
		errs.Add("prompt", fmt.Sprintf("prompt exceeds maximum length of %d characters", v.config.MaxPromptLength), len(req.Prompt))
	}

	// Validate temperature
	if req.Temperature != nil {
		if *req.Temperature < v.config.MinTemperature || *req.Temperature > v.config.MaxTemperature {
			errs.Add("temperature", fmt.Sprintf("must be between %.1f and %.1f", v.config.MinTemperature, v.config.MaxTemperature), *req.Temperature)
		}
	}

	// Validate max_tokens
	if req.MaxTokens != nil {
		if *req.MaxTokens <= 0 {
			errs.Add("max_tokens", "must be a positive integer", *req.MaxTokens)
		}
		if *req.MaxTokens > v.config.MaxTokensLimit {
			errs.Add("max_tokens", fmt.Sprintf("exceeds maximum of %d", v.config.MaxTokensLimit), *req.MaxTokens)
		}
	}

	// Validate top_p
	if req.TopP != nil {
		if *req.TopP < v.config.MinTopP || *req.TopP > v.config.MaxTopP {
			errs.Add("top_p", fmt.Sprintf("must be between %.1f and %.1f", v.config.MinTopP, v.config.MaxTopP), *req.TopP)
		}
	}

	// Validate stop sequences
	if len(req.Stop) > v.config.MaxStopSequences {
		errs.Add("stop", fmt.Sprintf("exceeds maximum of %d stop sequences", v.config.MaxStopSequences), len(req.Stop))
	}

	// Validate messages
	if len(req.Messages) > v.config.MaxMessagesCount {
		errs.Add("messages", fmt.Sprintf("exceeds maximum of %d messages", v.config.MaxMessagesCount), len(req.Messages))
	}

	// Validate message roles
	validRoles := map[string]bool{"system": true, "user": true, "assistant": true, "function": true, "tool": true}
	for i, msg := range req.Messages {
		if msg.Role == "" {
			errs.Add(fmt.Sprintf("messages[%d].role", i), "role is required", nil)
		} else if !validRoles[msg.Role] {
			errs.Add(fmt.Sprintf("messages[%d].role", i), fmt.Sprintf("invalid role '%s', must be one of: system, user, assistant, function, tool", msg.Role), msg.Role)
		}
		if msg.Content == "" && msg.Role != "assistant" {
			errs.Add(fmt.Sprintf("messages[%d].content", i), "content is required for non-assistant messages", nil)
		}
	}

	return errs
}

// SanitizeInputMiddleware sanitizes potentially dangerous input
func (v *Validator) SanitizeInputMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}

		// Sanitize the body
		sanitized := v.sanitizeJSON(bodyBytes)

		// Restore sanitized body
		c.Request.Body = io.NopCloser(bytes.NewBuffer(sanitized))
		c.Next()
	}
}

// sanitizeJSON removes potentially dangerous content from JSON
func (v *Validator) sanitizeJSON(data []byte) []byte {
	// Remove null bytes
	data = bytes.ReplaceAll(data, []byte{0x00}, []byte{})

	// Convert to string for regex processing
	content := string(data)

	// Remove control characters except for \n, \r, \t
	controlCharRegex := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	content = controlCharRegex.ReplaceAllString(content, "")

	return []byte(content)
}

// RequireContentType requires specific content types
func RequireContentType(contentTypes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.ContentType()

		// Allow empty content type for GET requests
		if ct == "" && c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		for _, allowed := range contentTypes {
			if strings.HasPrefix(ct, allowed) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("unsupported content type '%s', expected one of: %s", ct, strings.Join(contentTypes, ", ")),
				"type":    "invalid_request_error",
				"code":    "unsupported_media_type",
			},
		})
		c.Abort()
	}
}

// RequireJSON requires JSON content type for POST/PUT/PATCH requests
func RequireJSON() gin.HandlerFunc {
	return RequireContentType("application/json")
}

// GetConfig returns the current validation config
func (v *Validator) GetConfig() ValidationConfig {
	return v.config
}
