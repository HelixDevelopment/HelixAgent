package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDefaultValidationConfig(t *testing.T) {
	t.Parallel()
	config := DefaultValidationConfig()

	if config.MaxBodySize != 10*1024*1024 {
		t.Errorf("Expected MaxBodySize 10MB, got %d", config.MaxBodySize)
	}

	if config.MaxPromptLength != 100000 {
		t.Errorf("Expected MaxPromptLength 100000, got %d", config.MaxPromptLength)
	}

	if config.MaxTokensLimit != 32000 {
		t.Errorf("Expected MaxTokensLimit 32000, got %d", config.MaxTokensLimit)
	}

	if config.MaxTemperature != 2.0 {
		t.Errorf("Expected MaxTemperature 2.0, got %f", config.MaxTemperature)
	}
}

func TestValidationErrors_Add(t *testing.T) {
	t.Parallel()
	errs := &ValidationErrors{}

	errs.Add("field1", "error1", nil)
	errs.Add("field2", "error2", "value")

	if len(errs.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errs.Errors))
	}

	if !errs.HasErrors() {
		t.Error("Expected HasErrors() to return true")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()
	errs := &ValidationErrors{}
	errs.Add("temperature", "must be between 0 and 2", 3.0)
	errs.Add("max_tokens", "must be positive", -1)

	errMsg := errs.Error()
	if !strings.Contains(errMsg, "temperature") {
		t.Error("Error message should contain 'temperature'")
	}
	if !strings.Contains(errMsg, "max_tokens") {
		t.Error("Error message should contain 'max_tokens'")
	}
}

func TestValidator_BodySizeMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := ValidationConfig{
		MaxBodySize: 100, // Very small for testing
	}
	validator := NewValidator(config)

	router := gin.New()
	router.Use(validator.BodySizeMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		bodySize       int
		expectedStatus int
	}{
		{"small body allowed", 50, http.StatusOK},
		{"large body rejected", 200, http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Repeat("x", tt.bodySize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.ContentLength = int64(tt.bodySize)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestValidator_ValidateCompletionMiddleware_ValidRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	temp := 0.7
	maxTokens := 100
	body := CompletionValidationRequest{
		Prompt:      "Hello, world!",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestValidator_ValidateCompletionMiddleware_InvalidTemperature(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	temp := 5.0 // Invalid - too high
	body := CompletionValidationRequest{
		Prompt:      "Hello",
		Temperature: &temp,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Error("Expected error response")
	}
}

func TestValidator_ValidateCompletionMiddleware_InvalidMaxTokens(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	maxTokens := -10 // Invalid - negative
	body := CompletionValidationRequest{
		Prompt:    "Hello",
		MaxTokens: &maxTokens,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_InvalidRole(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "invalid_role", Content: "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_InvalidJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_MissingPromptAndMessages(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		// Neither prompt nor messages
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_ValidMessages(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello!"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestValidator_ValidateCompletionMiddleware_TooManyStopSequences(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := DefaultValidationConfig()
	config.MaxStopSequences = 2
	validator := NewValidator(config)

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Prompt: "Hello",
		Stop:   []string{"stop1", "stop2", "stop3", "stop4"}, // Too many
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRequireJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireJSON())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		contentType    string
		expectedStatus int
	}{
		{"valid JSON content type", "application/json", http.StatusOK},
		{"JSON with charset", "application/json; charset=utf-8", http.StatusOK},
		{"invalid content type", "text/plain", http.StatusUnsupportedMediaType},
		{"XML content type", "application/xml", http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test": true}`))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestRequireJSON_GetRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireJSON())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// GET requests with no content type should be allowed
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestValidator_SanitizeInputMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.SanitizeInputMiddleware())
	router.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.JSON(http.StatusOK, gin.H{"body": string(body)})
	})

	// Input with null bytes and control characters
	input := "Hello\x00World\x01Test\x7F"
	req := httptest.NewRequest("POST", "/test", strings.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if strings.Contains(resp["body"], "\x00") {
		t.Error("Response should not contain null bytes")
	}
}

func TestNewDefaultValidator(t *testing.T) {
	t.Parallel()
	validator := NewDefaultValidator()
	if validator == nil {
		t.Fatal("Expected validator instance")
	}

	config := validator.GetConfig()
	if config.MaxBodySize != 10*1024*1024 {
		t.Errorf("Expected default MaxBodySize")
	}
}

func TestValidator_GetConfig(t *testing.T) {
	t.Parallel()
	config := ValidationConfig{
		MaxBodySize:     1000,
		MaxPromptLength: 500,
	}
	validator := NewValidator(config)

	retrieved := validator.GetConfig()
	if retrieved.MaxBodySize != 1000 {
		t.Errorf("Expected MaxBodySize 1000, got %d", retrieved.MaxBodySize)
	}
	if retrieved.MaxPromptLength != 500 {
		t.Errorf("Expected MaxPromptLength 500, got %d", retrieved.MaxPromptLength)
	}
}

func TestValidator_ValidateCompletionMiddleware_TopP(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		topP           float64
		expectedStatus int
	}{
		{"valid top_p 0.5", 0.5, http.StatusOK},
		{"valid top_p 0", 0.0, http.StatusOK},
		{"valid top_p 1", 1.0, http.StatusOK},
		{"invalid top_p negative", -0.5, http.StatusBadRequest},
		{"invalid top_p too high", 1.5, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := CompletionValidationRequest{
				Prompt: "Hello",
				TopP:   &tt.topP,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for top_p=%f", tt.expectedStatus, w.Code, tt.topP)
			}
		})
	}
}

func TestValidator_ValidateCompletionMiddleware_MaxTokensExceedsLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := DefaultValidationConfig()
	config.MaxTokensLimit = 1000
	validator := NewValidator(config)

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	maxTokens := 5000 // Exceeds limit
	body := CompletionValidationRequest{
		Prompt:    "Hello",
		MaxTokens: &maxTokens,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_PromptExceedsMaxLength(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := DefaultValidationConfig()
	config.MaxPromptLength = 50
	validator := NewValidator(config)

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Prompt: strings.Repeat("a", 100), // Exceeds 50 char limit
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_TooManyMessages(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	config := DefaultValidationConfig()
	config.MaxMessagesCount = 3
	validator := NewValidator(config)

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "Good"},
			{Role: "user", Content: "Great"}, // 5th message exceeds limit of 3
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_EmptyRole(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "", Content: "Hello"}, // Empty role
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_MissingContent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "user", Content: ""}, // Empty content for user
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidator_ValidateCompletionMiddleware_AssistantEmptyContentAllowed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: ""}, // Empty content allowed for assistant
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestValidator_ValidateCompletionMiddleware_JSONTypeError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Send temperature as string instead of float
	jsonBody := []byte(`{"prompt": "Hello", "temperature": "hot"}`)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Error("Expected error response")
	}
}

func TestValidator_ValidateCompletionMiddleware_JSONSyntaxError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Malformed JSON with syntax error
	jsonBody := []byte(`{"prompt": "Hello", temperature: 0.5}`) // Missing quotes around temperature

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestValidationErrors_Empty(t *testing.T) {
	t.Parallel()
	errs := &ValidationErrors{}

	if errs.HasErrors() {
		t.Error("Expected HasErrors() to return false for empty errors")
	}

	errMsg := errs.Error()
	if errMsg != "validation failed" {
		t.Errorf("Expected 'validation failed', got %s", errMsg)
	}
}

func TestRequireContentType(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireContentType("application/json", "application/xml"))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		contentType    string
		expectedStatus int
	}{
		{"JSON allowed", "application/json", http.StatusOK},
		{"XML allowed", "application/xml", http.StatusOK},
		{"text not allowed", "text/plain", http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("POST", "/test", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestValidator_SanitizeInputMiddleware_PreservesValidContent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.SanitizeInputMiddleware())
	router.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.JSON(http.StatusOK, gin.H{"body": string(body)})
	})

	// Valid JSON with escaped newlines and tabs
	input := `{"message": "Hello\nWorld\tTab"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// The input should be preserved (the escaped \n and \t should still be in the body)
	if !strings.Contains(resp["body"], `\n`) {
		t.Error("Escaped newlines should be preserved")
	}
	if !strings.Contains(resp["body"], `\t`) {
		t.Error("Escaped tabs should be preserved")
	}
}

func TestValidator_ValidateCompletionMiddleware_FunctionRole(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "user", Content: "Call the weather function"},
			{Role: "function", Content: `{"temperature": 72}`},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestValidator_ValidateCompletionMiddleware_ToolRole(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	validator := NewDefaultValidator()

	router := gin.New()
	router.Use(validator.ValidateCompletionMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	body := CompletionValidationRequest{
		Messages: []MessageValidation{
			{Role: "user", Content: "Use the calculator tool"},
			{Role: "tool", Content: `{"result": 42}`},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// --- Benchmarks ---

func BenchmarkValidator_ValidateCompletionRequest(b *testing.B) {
	v := NewDefaultValidator()
	temp := 0.7
	maxTokens := 2048
	topP := 0.9
	req := &CompletionValidationRequest{
		Model:       "gpt-4",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
		Messages: []MessageValidation{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Stop: []string{"\n"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.validateCompletionRequest(req)
	}
}

func BenchmarkValidationErrors_AddAndError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errs := &ValidationErrors{}
		errs.Add("field1", "message1", "value1")
		errs.Add("field2", "message2", 42)
		errs.Add("field3", "message3", nil)
		_ = errs.Error()
		_ = errs.HasErrors()
	}
}

func BenchmarkValidator_SanitizeJSON(b *testing.B) {
	v := NewDefaultValidator()
	// Input with control characters and null bytes
	input := []byte(`{"prompt": "Hello world, how are you?", "model": "gpt-4"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.sanitizeJSON(input)
	}
}
