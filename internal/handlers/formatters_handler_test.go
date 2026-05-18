package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/formatters"
)

func init() {
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Mock Formatter (implements formatters.Formatter)
// ============================================================================

type mockFormatter struct {
	name      string
	version   string
	languages []string
	meta      *formatters.FormatterMetadata

	formatFunc      func(ctx context.Context, req *formatters.FormatRequest) (*formatters.FormatResult, error)
	healthCheckFunc func(ctx context.Context) error
	validateCfgFunc func(config map[string]interface{}) error
}

func (m *mockFormatter) Name() string        { return m.name }
func (m *mockFormatter) Version() string     { return m.version }
func (m *mockFormatter) Languages() []string { return m.languages }

func (m *mockFormatter) SupportsStdin() bool   { return m.meta.SupportsStdin }
func (m *mockFormatter) SupportsInPlace() bool { return m.meta.SupportsInPlace }
func (m *mockFormatter) SupportsCheck() bool   { return m.meta.SupportsCheck }
func (m *mockFormatter) SupportsConfig() bool  { return m.meta.SupportsConfig }

func (m *mockFormatter) Format(ctx context.Context, req *formatters.FormatRequest) (*formatters.FormatResult, error) {
	if m.formatFunc != nil {
		return m.formatFunc(ctx, req)
	}
	return &formatters.FormatResult{
		Content:          req.Content,
		Changed:          false,
		FormatterName:    m.name,
		FormatterVersion: m.version,
		Duration:         10 * time.Millisecond,
		Success:          true,
	}, nil
}

func (m *mockFormatter) FormatBatch(ctx context.Context, reqs []*formatters.FormatRequest) ([]*formatters.FormatResult, error) {
	results := make([]*formatters.FormatResult, len(reqs))
	for i, req := range reqs {
		r, err := m.Format(ctx, req)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

func (m *mockFormatter) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil
}

func (m *mockFormatter) ValidateConfig(config map[string]interface{}) error {
	if m.validateCfgFunc != nil {
		return m.validateCfgFunc(config)
	}
	return nil
}

func (m *mockFormatter) DefaultConfig() map[string]interface{} {
	return make(map[string]interface{})
}

// ============================================================================
// Helpers
// ============================================================================

func newTestFormatterMetadata(name string, ftype formatters.FormatterType, languages []string) *formatters.FormatterMetadata {
	return &formatters.FormatterMetadata{
		Name:            name,
		Type:            ftype,
		Version:         "1.0.0",
		Languages:       languages,
		Performance:     "fast",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  true,
	}
}

func setupFormattersHandler() (*FormattersHandler, *gin.Engine, *formatters.FormatterRegistry) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	regConfig := &formatters.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
	}
	registry := formatters.NewFormatterRegistry(regConfig, logger)

	// Register mock formatters
	goMeta := newTestFormatterMetadata("gofmt", formatters.FormatterTypeNative, []string{"go"})
	goFmt := &mockFormatter{
		name: "gofmt", version: "1.21.0", languages: []string{"go"}, meta: goMeta,
		formatFunc: func(_ context.Context, req *formatters.FormatRequest) (*formatters.FormatResult, error) {
			return &formatters.FormatResult{
				Content:          req.Content + "\n",
				Changed:          true,
				FormatterName:    "gofmt",
				FormatterVersion: "1.21.0",
				Duration:         5 * time.Millisecond,
				Success:          true,
			}, nil
		},
	}
	_ = registry.Register(goFmt, goMeta)

	pyMeta := newTestFormatterMetadata("black", formatters.FormatterTypeNative, []string{"python"})
	pyFmt := &mockFormatter{
		name: "black", version: "24.3.0", languages: []string{"python"}, meta: pyMeta,
		formatFunc: func(_ context.Context, req *formatters.FormatRequest) (*formatters.FormatResult, error) {
			return &formatters.FormatResult{
				Content:          req.Content,
				Changed:          false,
				FormatterName:    "black",
				FormatterVersion: "24.3.0",
				Duration:         8 * time.Millisecond,
				Success:          true,
			}, nil
		},
	}
	_ = registry.Register(pyFmt, pyMeta)

	svcMeta := newTestFormatterMetadata("prettier", formatters.FormatterTypeService, []string{"javascript", "typescript"})
	svcMeta.ServiceURL = "http://localhost:9210"
	svcFmt := &mockFormatter{
		name: "prettier", version: "3.2.0", languages: []string{"javascript", "typescript"}, meta: svcMeta,
	}
	_ = registry.Register(svcFmt, svcMeta)

	execConfig := &formatters.ExecutorConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     0,
		EnableCache:    false,
	}
	executor := formatters.NewFormatterExecutor(registry, execConfig, logger)

	healthChecker := formatters.NewHealthChecker(registry, logger, 10*time.Second)

	h := NewFormattersHandler(registry, executor, healthChecker, logger)

	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)

	return h, r, registry
}

func setupEmptyFormattersHandler() (*FormattersHandler, *gin.Engine) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	regConfig := &formatters.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
	}
	registry := formatters.NewFormatterRegistry(regConfig, logger)

	execConfig := &formatters.ExecutorConfig{
		DefaultTimeout: 30 * time.Second,
	}
	executor := formatters.NewFormatterExecutor(registry, execConfig, logger)
	healthChecker := formatters.NewHealthChecker(registry, logger, 10*time.Second)

	h := NewFormattersHandler(registry, executor, healthChecker, logger)

	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)

	return h, r
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewFormattersHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	regConfig := &formatters.RegistryConfig{}
	registry := formatters.NewFormatterRegistry(regConfig, logger)
	execConfig := &formatters.ExecutorConfig{}
	executor := formatters.NewFormatterExecutor(registry, execConfig, logger)
	healthChecker := formatters.NewHealthChecker(registry, logger, 10*time.Second)

	h := NewFormattersHandler(registry, executor, healthChecker, logger)

	assert.NotNil(t, h)
	assert.Equal(t, registry, h.registry)
	assert.Equal(t, executor, h.executor)
	assert.Equal(t, healthChecker, h.health)
	assert.Equal(t, logger, h.logger)
}

// ============================================================================
// Type Tests
// ============================================================================

func TestFormatCodeRequest_Fields(t *testing.T) {
	t.Parallel()
	req := FormatCodeRequest{
		Content:    "package main",
		Language:   "go",
		FilePath:   "main.go",
		Formatter:  "gofmt",
		Config:     map[string]interface{}{"indent": 4},
		LineLength: 100,
		IndentSize: 4,
		UseTabs:    true,
		CheckOnly:  false,
		AgentName:  "test-agent",
		SessionID:  "session-123",
	}

	assert.Equal(t, "package main", req.Content)
	assert.Equal(t, "go", req.Language)
	assert.Equal(t, "main.go", req.FilePath)
	assert.Equal(t, "gofmt", req.Formatter)
	assert.Equal(t, 100, req.LineLength)
	assert.Equal(t, 4, req.IndentSize)
	assert.True(t, req.UseTabs)
	assert.False(t, req.CheckOnly)
	assert.Equal(t, "test-agent", req.AgentName)
	assert.Equal(t, "session-123", req.SessionID)
}

func TestFormatCodeResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := FormatCodeResponse{
		Success:          true,
		Content:          "formatted code",
		Changed:          true,
		FormatterName:    "gofmt",
		FormatterVersion: "1.21.0",
		DurationMS:       42,
		Error:            "",
		Warnings:         []string{"trailing whitespace"},
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "formatted code", resp.Content)
	assert.True(t, resp.Changed)
	assert.Equal(t, "gofmt", resp.FormatterName)
	assert.Equal(t, "1.21.0", resp.FormatterVersion)
	assert.Equal(t, int64(42), resp.DurationMS)
	assert.Empty(t, resp.Error)
	assert.Len(t, resp.Warnings, 1)
}

func TestFormatCodeResponse_JSONSerialization(t *testing.T) {
	t.Parallel()
	resp := FormatCodeResponse{
		Success:          true,
		Content:          "package main\n",
		Changed:          true,
		FormatterName:    "gofmt",
		FormatterVersion: "1.21.0",
		DurationMS:       10,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded FormatCodeResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Success, decoded.Success)
	assert.Equal(t, resp.Content, decoded.Content)
	assert.Equal(t, resp.Changed, decoded.Changed)
	assert.Equal(t, resp.FormatterName, decoded.FormatterName)
}

func TestFormatBatchRequest_Fields(t *testing.T) {
	t.Parallel()
	req := FormatBatchRequest{
		Requests: []FormatCodeRequest{
			{Content: "code1", Language: "go"},
			{Content: "code2", Language: "python"},
		},
	}

	assert.Len(t, req.Requests, 2)
}

func TestFormatBatchResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := FormatBatchResponse{
		Results: []FormatCodeResponse{
			{Success: true, FormatterName: "gofmt"},
			{Success: true, FormatterName: "black"},
		},
	}

	assert.Len(t, resp.Results, 2)
}

func TestCheckCodeRequest_Fields(t *testing.T) {
	t.Parallel()
	req := CheckCodeRequest{
		Content:   "package main",
		Language:  "go",
		FilePath:  "main.go",
		Formatter: "gofmt",
	}

	assert.Equal(t, "package main", req.Content)
	assert.Equal(t, "go", req.Language)
	assert.Equal(t, "main.go", req.FilePath)
	assert.Equal(t, "gofmt", req.Formatter)
}

func TestCheckCodeResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := CheckCodeResponse{
		Formatted:     true,
		FormatterName: "gofmt",
		Error:         "",
	}

	assert.True(t, resp.Formatted)
	assert.Equal(t, "gofmt", resp.FormatterName)
	assert.Empty(t, resp.Error)
}

func TestFormatterMetadataResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := FormatterMetadataResponse{
		Name:            "gofmt",
		Type:            "native",
		Version:         "1.21.0",
		Languages:       []string{"go"},
		Performance:     "very_fast",
		Supported:       true,
		Installed:       true,
		ServiceURL:      "",
		SupportsStdin:   true,
		SupportsInPlace: true,
		SupportsCheck:   true,
		SupportsConfig:  false,
	}

	assert.Equal(t, "gofmt", resp.Name)
	assert.Equal(t, "native", resp.Type)
	assert.Equal(t, "1.21.0", resp.Version)
	assert.Equal(t, []string{"go"}, resp.Languages)
	assert.Equal(t, "very_fast", resp.Performance)
	assert.True(t, resp.Supported)
	assert.True(t, resp.Installed)
	assert.True(t, resp.SupportsStdin)
	assert.True(t, resp.SupportsInPlace)
	assert.True(t, resp.SupportsCheck)
	assert.False(t, resp.SupportsConfig)
}

func TestDetectFormatterRequest_Fields(t *testing.T) {
	t.Parallel()
	req := DetectFormatterRequest{
		FilePath: "main.go",
		Content:  "package main",
	}

	assert.Equal(t, "main.go", req.FilePath)
	assert.Equal(t, "package main", req.Content)
}

func TestDetectFormatterResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := DetectFormatterResponse{
		Language: "go",
		Formatters: []DetectedFormatterResponse{
			{Name: "gofmt", Type: "native", Priority: 1, Reason: "preferred"},
		},
	}

	assert.Equal(t, "go", resp.Language)
	assert.Len(t, resp.Formatters, 1)
	assert.Equal(t, "gofmt", resp.Formatters[0].Name)
	assert.Equal(t, 1, resp.Formatters[0].Priority)
}

func TestDetectedFormatterResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := DetectedFormatterResponse{
		Name:     "prettier",
		Type:     "service",
		Priority: 2,
		Reason:   "default formatter",
	}

	assert.Equal(t, "prettier", resp.Name)
	assert.Equal(t, "service", resp.Type)
	assert.Equal(t, 2, resp.Priority)
	assert.Equal(t, "default formatter", resp.Reason)
}

func TestValidateConfigRequest_Fields(t *testing.T) {
	t.Parallel()
	req := ValidateConfigRequest{
		Formatter: "gofmt",
		Config:    map[string]interface{}{"indent": 4},
	}

	assert.Equal(t, "gofmt", req.Formatter)
	assert.NotNil(t, req.Config)
}

func TestValidateConfigResponse_Fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		resp   ValidateConfigResponse
		valid  bool
		errors int
	}{
		{
			name:   "valid config",
			resp:   ValidateConfigResponse{Valid: true},
			valid:  true,
			errors: 0,
		},
		{
			name:   "invalid config",
			resp:   ValidateConfigResponse{Valid: false, Errors: []string{"bad indent"}},
			valid:  false,
			errors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.valid, tt.resp.Valid)
			assert.Len(t, tt.resp.Errors, tt.errors)
		})
	}
}

// ============================================================================
// FormatCode Tests
// ============================================================================

func TestFormattersHandler_FormatCode_Success(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(FormatCodeRequest{
		Content:  "package main",
		Language: "go",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp FormatCodeResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "gofmt", resp.FormatterName)
	assert.Equal(t, "1.21.0", resp.FormatterVersion)
	assert.True(t, resp.Changed)
	assert.Contains(t, resp.Content, "package main")
}

func TestFormattersHandler_FormatCode_Python(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(FormatCodeRequest{
		Content:  "def foo(): pass",
		Language: "python",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp FormatCodeResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "black", resp.FormatterName)
}

func TestFormattersHandler_FormatCode_BadRequest_MissingContent(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body := []byte(`{"language":"go"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFormattersHandler_FormatCode_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format", bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFormattersHandler_FormatCode_UnsupportedLanguage(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(FormatCodeRequest{
		Content:  "code",
		Language: "brainfuck",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Round-116 ISSUE-009: handler classifies "no formatters available for
	// language: X" as caller-side bad-input (400) per CONST-035 §c (see
	// formatters_handler.go:90-101) — returning 500 caused SDK retry storms.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// FormatCodeBatch Tests
// ============================================================================

func TestFormattersHandler_FormatCodeBatch_Success(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(FormatBatchRequest{
		Requests: []FormatCodeRequest{
			{Content: "package main", Language: "go"},
			{Content: "def foo(): pass", Language: "python"},
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp FormatBatchResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Results, 2)
	assert.True(t, resp.Results[0].Success)
	assert.True(t, resp.Results[1].Success)
}

func TestFormattersHandler_FormatCodeBatch_BadRequest(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/format/batch",
		bytes.NewBufferString(`{invalid`),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFormattersHandler_FormatCodeBatch_BadRequest_MissingRequests(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// CheckCode Tests
// ============================================================================

func TestFormattersHandler_CheckCode_Success(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(CheckCodeRequest{
		Content:  "package main",
		Language: "go",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format/check", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CheckCodeResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gofmt", resp.FormatterName)
}

func TestFormattersHandler_CheckCode_BadRequest(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/format/check",
		bytes.NewBufferString(`{invalid`),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFormattersHandler_CheckCode_BadRequest_MissingContent(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body := []byte(`{"language":"go"}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/format/check", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// ListFormatters Tests
// ============================================================================

func TestFormattersHandler_ListFormatters_All(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["count"])
	fmtrs, ok := resp["formatters"].([]interface{})
	require.True(t, ok)
	assert.Len(t, fmtrs, 3)
}

func TestFormattersHandler_ListFormatters_ByType(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters?type=native", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	count := resp["count"].(float64)
	assert.Equal(t, float64(2), count) // gofmt and black
}

func TestFormattersHandler_ListFormatters_ByLanguage(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters?language=go", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	count := resp["count"].(float64)
	assert.Equal(t, float64(1), count) // gofmt only
}

func TestFormattersHandler_ListFormatters_ByTypeAndLanguage(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters?type=service&language=javascript", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	count := resp["count"].(float64)
	assert.Equal(t, float64(1), count) // prettier
}

func TestFormattersHandler_ListFormatters_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupEmptyFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(0), resp["count"])
}

func TestFormattersHandler_ListFormatters_NonexistentLanguage(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters?language=cobol", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(0), resp["count"])
}

// ============================================================================
// GetFormatter Tests
// ============================================================================

func TestFormattersHandler_GetFormatter_Success(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/gofmt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp FormatterMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gofmt", resp.Name)
	assert.Equal(t, "native", resp.Type)
	assert.Equal(t, "1.21.0", resp.Version)
	assert.Contains(t, resp.Languages, "go")
	assert.True(t, resp.Supported)
	assert.True(t, resp.Installed)
	assert.True(t, resp.SupportsStdin)
	assert.True(t, resp.SupportsCheck)
}

func TestFormattersHandler_GetFormatter_NotFound(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "formatter not found", resp["error"])
}

func TestFormattersHandler_GetFormatter_ServiceFormatter(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/prettier", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp FormatterMetadataResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "prettier", resp.Name)
	assert.Equal(t, "service", resp.Type)
	assert.Equal(t, "http://localhost:9210", resp.ServiceURL)
	assert.Contains(t, resp.Languages, "javascript")
	assert.Contains(t, resp.Languages, "typescript")
}

// ============================================================================
// HealthCheckFormatter Tests
// ============================================================================

func TestFormattersHandler_HealthCheckFormatter_Healthy(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/gofmt/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "gofmt", resp["name"])
	assert.True(t, resp["healthy"].(bool))
}

func TestFormattersHandler_HealthCheckFormatter_NotFound(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/nonexistent/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================================================
// DetectFormatter Tests
// ============================================================================

func TestFormattersHandler_DetectFormatter_Go(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/detect?file_path=main.go", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DetectFormatterResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "go", resp.Language)
	assert.Len(t, resp.Formatters, 1)
	assert.Equal(t, "gofmt", resp.Formatters[0].Name)
	assert.Equal(t, 1, resp.Formatters[0].Priority)
	assert.Contains(t, resp.Formatters[0].Reason, "preferred")
}

func TestFormattersHandler_DetectFormatter_Python(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/detect?file_path=app.py", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DetectFormatterResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "python", resp.Language)
	assert.Len(t, resp.Formatters, 1)
	assert.Equal(t, "black", resp.Formatters[0].Name)
}

func TestFormattersHandler_DetectFormatter_JavaScript(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/detect?file_path=app.js", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp DetectFormatterResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "javascript", resp.Language)
	assert.True(t, len(resp.Formatters) >= 1)
}

func TestFormattersHandler_DetectFormatter_MissingFilePath(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/detect", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "file_path is required", resp["error"])
}

func TestFormattersHandler_DetectFormatter_UnknownExtension(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/detect?file_path=file.xyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// ValidateConfig Tests
// ============================================================================

func TestFormattersHandler_ValidateConfig_Valid(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(ValidateConfigRequest{
		Formatter: "gofmt",
		Config:    map[string]interface{}{"indent": 4},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/formatters/gofmt/validate-config",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ValidateConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Valid)
	assert.Empty(t, resp.Errors)
}

func TestFormattersHandler_ValidateConfig_Invalid(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	regConfig := &formatters.RegistryConfig{DefaultTimeout: 30 * time.Second}
	registry := formatters.NewFormatterRegistry(regConfig, logger)

	meta := newTestFormatterMetadata("strict-fmt", formatters.FormatterTypeNative, []string{"go"})
	strictFmt := &mockFormatter{
		name: "strict-fmt", version: "1.0.0", languages: []string{"go"}, meta: meta,
		validateCfgFunc: func(config map[string]interface{}) error {
			return fmt.Errorf("invalid indent value")
		},
	}
	_ = registry.Register(strictFmt, meta)

	execConfig := &formatters.ExecutorConfig{DefaultTimeout: 30 * time.Second}
	executor := formatters.NewFormatterExecutor(registry, execConfig, logger)
	healthChecker := formatters.NewHealthChecker(registry, logger, 10*time.Second)

	h := NewFormattersHandler(registry, executor, healthChecker, logger)
	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)

	body, _ := json.Marshal(ValidateConfigRequest{
		Formatter: "strict-fmt",
		Config:    map[string]interface{}{"indent": -1},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/formatters/strict-fmt/validate-config",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ValidateConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Valid)
	assert.Len(t, resp.Errors, 1)
	assert.Contains(t, resp.Errors[0], "invalid indent value")
}

func TestFormattersHandler_ValidateConfig_FormatterNotFound(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	body, _ := json.Marshal(ValidateConfigRequest{
		Formatter: "nonexistent",
		Config:    map[string]interface{}{"key": "value"},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/formatters/nonexistent/validate-config",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFormattersHandler_ValidateConfig_BadRequest(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/formatters/gofmt/validate-config",
		bytes.NewBufferString(`{bad`),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFormattersHandler_ValidateConfig_BadRequest_MissingFields(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	tests := []struct {
		name string
		body string
	}{
		{"missing formatter", `{"config":{"k":"v"}}`},
		{"missing config", `{"formatter":"gofmt"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(
				"POST",
				"/v1/formatters/gofmt/validate-config",
				bytes.NewBufferString(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// ============================================================================
// HealthCheckAll Tests
// ============================================================================

func TestFormattersHandler_HealthCheckAll_AllHealthy(t *testing.T) {
	t.Parallel()
	h, _, _ := setupFormattersHandler()

	// Use HealthCheckAll directly since it's not registered on a standard route
	r := gin.New()
	r.GET("/v1/formatters/health", h.HealthCheckAll)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["total_formatters"])
	assert.Equal(t, float64(3), resp["healthy_count"])
	assert.Equal(t, float64(0), resp["unhealthy_count"])
	assert.Equal(t, float64(100), resp["health_percentage"])
}

func TestFormattersHandler_HealthCheckAll_Empty(t *testing.T) {
	t.Parallel()
	h, _, _ := func() (*FormattersHandler, *gin.Engine, *formatters.FormatterRegistry) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		regConfig := &formatters.RegistryConfig{DefaultTimeout: 30 * time.Second}
		registry := formatters.NewFormatterRegistry(regConfig, logger)

		execConfig := &formatters.ExecutorConfig{DefaultTimeout: 30 * time.Second}
		executor := formatters.NewFormatterExecutor(registry, execConfig, logger)
		healthChecker := formatters.NewHealthChecker(registry, logger, 10*time.Second)

		h := NewFormattersHandler(registry, executor, healthChecker, logger)
		return h, nil, registry
	}()

	r := gin.New()
	r.GET("/v1/formatters/health", h.HealthCheckAll)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/formatters/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(0), resp["total_formatters"])
	assert.Equal(t, float64(100), resp["health_percentage"])
}

// ============================================================================
// RegisterRoutes Tests
// ============================================================================

func TestFormattersHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	h, _, _ := setupFormattersHandler()

	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)

	// POST routes
	postRoutes := []string{
		"/v1/format",
		"/v1/format/batch",
		"/v1/format/check",
	}

	for _, route := range postRoutes {
		w := httptest.NewRecorder()
		body := []byte(`{"content":"test","language":"go"}`)
		req, _ := http.NewRequest("POST", route, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.NotEqual(
			t, http.StatusNotFound, w.Code,
			"POST %s should be registered", route,
		)
	}

	// GET routes
	getRoutes := []string{
		"/v1/formatters",
		"/v1/formatters/gofmt",
		"/v1/formatters/gofmt/health",
	}

	for _, route := range getRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		r.ServeHTTP(w, req)
		assert.NotEqual(
			t, http.StatusNotFound, w.Code,
			"GET %s should be registered", route,
		)
	}
}

// ============================================================================
// Content-Type Tests
// ============================================================================

func TestFormattersHandler_ResponseContentType(t *testing.T) {
	t.Parallel()
	_, r, _ := setupFormattersHandler()

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/v1/formatters", ""},
		{"GET", "/v1/formatters/gofmt", ""},
		{"POST", "/v1/format", `{"content":"x","language":"go"}`},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			var req *http.Request
			if route.body != "" {
				req, _ = http.NewRequest(
					route.method,
					route.path,
					bytes.NewBufferString(route.body),
				)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(route.method, route.path, nil)
			}
			r.ServeHTTP(w, req)

			assert.Contains(
				t,
				w.Header().Get("Content-Type"),
				"application/json",
			)
		})
	}
}
