package handlers

import (
	"net/http"
	"strings"
	"time"

	"dev.helix.agent/internal/formatters"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// FormattersHandler handles formatter-related HTTP requests
type FormattersHandler struct {
	registry *formatters.FormatterRegistry
	executor *formatters.FormatterExecutor
	health   *formatters.HealthChecker
	logger   *logrus.Logger
}

// NewFormattersHandler creates a new formatters handler
func NewFormattersHandler(
	registry *formatters.FormatterRegistry,
	executor *formatters.FormatterExecutor,
	health *formatters.HealthChecker,
	logger *logrus.Logger,
) *FormattersHandler {
	return &FormattersHandler{
		registry: registry,
		executor: executor,
		health:   health,
		logger:   logger,
	}
}

// FormatCodeRequest is the request body for formatting code
type FormatCodeRequest struct {
	Content    string                 `json:"content" binding:"required"`
	Language   string                 `json:"language"`
	FilePath   string                 `json:"file_path"`
	Formatter  string                 `json:"formatter"`
	Config     map[string]interface{} `json:"config"`
	LineLength int                    `json:"line_length"`
	IndentSize int                    `json:"indent_size"`
	UseTabs    bool                   `json:"use_tabs"`
	CheckOnly  bool                   `json:"check_only"`
	AgentName  string                 `json:"agent_name"`
	SessionID  string                 `json:"session_id"`
}

// FormatCodeResponse is the response for formatting code
type FormatCodeResponse struct {
	Success          bool                    `json:"success"`
	Content          string                  `json:"content,omitempty"`
	Changed          bool                    `json:"changed"`
	FormatterName    string                  `json:"formatter_name"`
	FormatterVersion string                  `json:"formatter_version"`
	DurationMS       int64                   `json:"duration_ms"`
	Stats            *formatters.FormatStats `json:"stats,omitempty"`
	Error            string                  `json:"error,omitempty"`
	Warnings         []string                `json:"warnings,omitempty"`
}

// FormatCode handles POST /v1/format
func (h *FormattersHandler) FormatCode(c *gin.Context) {
	var req FormatCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create format request
	formatReq := &formatters.FormatRequest{
		Content:    req.Content,
		Language:   req.Language,
		FilePath:   req.FilePath,
		Config:     req.Config,
		LineLength: req.LineLength,
		IndentSize: req.IndentSize,
		UseTabs:    req.UseTabs,
		CheckOnly:  req.CheckOnly,
		AgentName:  req.AgentName,
		SessionID:  req.SessionID,
		Timeout:    30 * time.Second,
	}

	// Execute formatting
	result, err := h.executor.Execute(c.Request.Context(), formatReq)
	if err != nil {
		// CONST-035 §c: distinguish bad-input (400) from real server
		// faults (500). "no formatters available for language: X" is
		// caller-side: they specified an unknown language. Returning
		// 500 for that triggered SDK retry storms.
		errMsg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(errMsg, "no formatters available") ||
			strings.Contains(errMsg, "unsupported") ||
			strings.Contains(errMsg, "invalid") ||
			strings.Contains(errMsg, "not supported") {
			status = http.StatusBadRequest
		}
		h.logger.Errorf("Format execution failed: %v", err)
		c.JSON(status, gin.H{
			"success": false,
			"error":   errMsg,
		})
		return
	}

	// Build response
	response := FormatCodeResponse{
		Success:          result.Success,
		Content:          result.Content,
		Changed:          result.Changed,
		FormatterName:    result.FormatterName,
		FormatterVersion: result.FormatterVersion,
		DurationMS:       result.Duration.Milliseconds(),
		Stats:            result.Stats,
		Warnings:         result.Warnings,
	}

	if result.Error != nil {
		response.Error = result.Error.Error()
	}

	// CONST-035 §c wrapper-bluff fix: when result.Success is false the
	// formatter actually failed (e.g. gofmt returned non-zero stderr).
	// Returning HTTP 200 in that case is a wrapper bluff — caller's SDK
	// reads the status code and thinks the format succeeded. Map success
	// to status: true → 200, false → 422 Unprocessable Entity (request
	// was syntactically valid but the formatter couldn't process it).
	httpStatus := http.StatusOK
	if !result.Success {
		httpStatus = http.StatusUnprocessableEntity
	}
	c.JSON(httpStatus, response)
}

// FormatBatchRequest is the request for batch formatting
type FormatBatchRequest struct {
	Requests []FormatCodeRequest `json:"requests" binding:"required"`
}

// FormatBatchResponse is the response for batch formatting
type FormatBatchResponse struct {
	Results []FormatCodeResponse `json:"results"`
}

// FormatCodeBatch handles POST /v1/format/batch
func (h *FormattersHandler) FormatCodeBatch(c *gin.Context) {
	var req FormatBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to format requests
	formatReqs := make([]*formatters.FormatRequest, len(req.Requests))
	for i, r := range req.Requests {
		formatReqs[i] = &formatters.FormatRequest{
			Content:    r.Content,
			Language:   r.Language,
			FilePath:   r.FilePath,
			Config:     r.Config,
			LineLength: r.LineLength,
			IndentSize: r.IndentSize,
			UseTabs:    r.UseTabs,
			CheckOnly:  r.CheckOnly,
			AgentName:  r.AgentName,
			SessionID:  r.SessionID,
			Timeout:    30 * time.Second,
		}
	}

	// Execute batch formatting
	results, err := h.executor.ExecuteBatch(c.Request.Context(), formatReqs)
	if err != nil {
		h.logger.Errorf("Batch format execution failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Build response
	responses := make([]FormatCodeResponse, len(results))
	for i, result := range results {
		if result == nil {
			responses[i] = FormatCodeResponse{
				Success: false,
				Error:   "execution failed",
			}
			continue
		}

		responses[i] = FormatCodeResponse{
			Success:          result.Success,
			Content:          result.Content,
			Changed:          result.Changed,
			FormatterName:    result.FormatterName,
			FormatterVersion: result.FormatterVersion,
			DurationMS:       result.Duration.Milliseconds(),
			Stats:            result.Stats,
			Warnings:         result.Warnings,
		}

		if result.Error != nil {
			responses[i].Error = result.Error.Error()
		}
	}

	c.JSON(http.StatusOK, FormatBatchResponse{
		Results: responses,
	})
}

// CheckCodeRequest is the request for checking code formatting
type CheckCodeRequest struct {
	Content   string `json:"content" binding:"required"`
	Language  string `json:"language"`
	FilePath  string `json:"file_path"`
	Formatter string `json:"formatter"`
}

// CheckCodeResponse is the response for checking code formatting
type CheckCodeResponse struct {
	Formatted     bool   `json:"formatted"`
	FormatterName string `json:"formatter_name"`
	Error         string `json:"error,omitempty"`
}

// CheckCode handles POST /v1/format/check
func (h *FormattersHandler) CheckCode(c *gin.Context) {
	var req CheckCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create format request with check-only mode
	formatReq := &formatters.FormatRequest{
		Content:   req.Content,
		Language:  req.Language,
		FilePath:  req.FilePath,
		CheckOnly: true,
		Timeout:   10 * time.Second,
	}

	// Execute check
	result, err := h.executor.Execute(c.Request.Context(), formatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	response := CheckCodeResponse{
		Formatted:     !result.Changed,
		FormatterName: result.FormatterName,
	}

	if result.Error != nil {
		response.Error = result.Error.Error()
	}

	c.JSON(http.StatusOK, response)
}

// FormatterMetadataResponse is the response for formatter metadata
type FormatterMetadataResponse struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Version         string   `json:"version"`
	Languages       []string `json:"languages"`
	Performance     string   `json:"performance"`
	Supported       bool     `json:"supported"`
	Installed       bool     `json:"installed"`
	ServiceURL      string   `json:"service_url,omitempty"`
	SupportsStdin   bool     `json:"supports_stdin"`
	SupportsInPlace bool     `json:"supports_in_place"`
	SupportsCheck   bool     `json:"supports_check"`
	SupportsConfig  bool     `json:"supports_config"`
}

// ListFormatters handles GET /v1/formatters
func (h *FormattersHandler) ListFormatters(c *gin.Context) {
	language := c.Query("language")
	formatterType := c.Query("type")

	var names []string
	if formatterType != "" {
		names = h.registry.ListByType(formatters.FormatterType(formatterType))
	} else {
		names = h.registry.List()
	}

	// Filter by language if specified
	if language != "" {
		languageFormatters := h.registry.GetByLanguage(language)
		languageNames := make(map[string]bool)
		for _, f := range languageFormatters {
			languageNames[f.Name()] = true
		}

		filtered := make([]string, 0)
		for _, name := range names {
			if languageNames[name] {
				filtered = append(filtered, name)
			}
		}
		names = filtered
	}

	// Build response
	formatters := make([]FormatterMetadataResponse, 0, len(names))
	for _, name := range names {
		formatter, err := h.registry.Get(name)
		if err != nil {
			continue
		}

		metadata, err := h.registry.GetMetadata(name)
		if err != nil {
			continue
		}

		formatters = append(formatters, FormatterMetadataResponse{
			Name:            name,
			Type:            string(metadata.Type),
			Version:         formatter.Version(),
			Languages:       formatter.Languages(),
			Performance:     metadata.Performance,
			Supported:       true,
			Installed:       true,
			ServiceURL:      metadata.ServiceURL,
			SupportsStdin:   formatter.SupportsStdin(),
			SupportsInPlace: formatter.SupportsInPlace(),
			SupportsCheck:   formatter.SupportsCheck(),
			SupportsConfig:  formatter.SupportsConfig(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"count":      len(formatters),
		"formatters": formatters,
	})
}

// GetFormatter handles GET /v1/formatters/:name
func (h *FormattersHandler) GetFormatter(c *gin.Context) {
	name := c.Param("name")

	formatter, err := h.registry.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "formatter not found",
		})
		return
	}

	metadata, err := h.registry.GetMetadata(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get metadata",
		})
		return
	}

	response := FormatterMetadataResponse{
		Name:            name,
		Type:            string(metadata.Type),
		Version:         formatter.Version(),
		Languages:       formatter.Languages(),
		Performance:     metadata.Performance,
		Supported:       true,
		Installed:       true,
		ServiceURL:      metadata.ServiceURL,
		SupportsStdin:   formatter.SupportsStdin(),
		SupportsInPlace: formatter.SupportsInPlace(),
		SupportsCheck:   formatter.SupportsCheck(),
		SupportsConfig:  formatter.SupportsConfig(),
	}

	c.JSON(http.StatusOK, response)
}

// HealthCheckFormatter handles GET /v1/formatters/:name/health
func (h *FormattersHandler) HealthCheckFormatter(c *gin.Context) {
	name := c.Param("name")

	result, err := h.health.Check(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	status := http.StatusOK
	if !result.Healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"name":        result.Name,
		"healthy":     result.Healthy,
		"error":       result.Error,
		"duration_ms": result.Duration.Milliseconds(),
	})
}

// DetectFormatterRequest is the request for detecting a formatter
type DetectFormatterRequest struct {
	FilePath string `json:"file_path" form:"file_path"`
	Content  string `json:"content" form:"content"`
}

// DetectFormatterResponse is the response for detecting a formatter
type DetectFormatterResponse struct {
	Language   string                      `json:"language"`
	Formatters []DetectedFormatterResponse `json:"formatters"`
}

// DetectedFormatterResponse represents a detected formatter
type DetectedFormatterResponse struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Priority int    `json:"priority"`
	Reason   string `json:"reason"`
}

// DetectFormatter handles GET /v1/formatters/detect
func (h *FormattersHandler) DetectFormatter(c *gin.Context) {
	filePath := c.Query("file_path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file_path is required",
		})
		return
	}

	language, err := h.registry.DetectLanguage(filePath, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	formatters := h.registry.GetByLanguage(language)
	detected := make([]DetectedFormatterResponse, 0, len(formatters))

	for i, formatter := range formatters {
		metadata, _ := h.registry.GetMetadata(formatter.Name()) //nolint:errcheck

		reason := "default formatter"
		if i == 0 {
			reason = "preferred formatter for " + language
		}

		detected = append(detected, DetectedFormatterResponse{
			Name:     formatter.Name(),
			Type:     string(metadata.Type),
			Priority: i + 1,
			Reason:   reason,
		})
	}

	c.JSON(http.StatusOK, DetectFormatterResponse{
		Language:   language,
		Formatters: detected,
	})
}

// ValidateConfigRequest is the request for validating config
type ValidateConfigRequest struct {
	Formatter string                 `json:"formatter" binding:"required"`
	Config    map[string]interface{} `json:"config" binding:"required"`
}

// ValidateConfigResponse is the response for validating config
type ValidateConfigResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ValidateConfig handles POST /v1/formatters/:name/validate-config
func (h *FormattersHandler) ValidateConfig(c *gin.Context) {
	name := c.Param("name")

	var req ValidateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	formatter, err := h.registry.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "formatter not found",
		})
		return
	}

	err = formatter.ValidateConfig(req.Config)

	response := ValidateConfigResponse{
		Valid: err == nil,
	}

	if err != nil {
		response.Errors = []string{err.Error()}
	}

	c.JSON(http.StatusOK, response)
}

// RegisterRoutes registers all formatter routes
func (h *FormattersHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/format", h.FormatCode)
	router.POST("/format/batch", h.FormatCodeBatch)
	router.POST("/format/check", h.CheckCode)
	router.GET("/formatters", h.ListFormatters)
	router.GET("/formatters/detect", h.DetectFormatter)
	router.GET("/formatters/:name", h.GetFormatter)
	router.GET("/formatters/:name/health", h.HealthCheckFormatter)
	router.POST("/formatters/:name/validate-config", h.ValidateConfig)
}

// HealthCheckAll handles GET /v1/formatters/health
func (h *FormattersHandler) HealthCheckAll(c *gin.Context) {
	report := h.health.CheckAll(c.Request.Context())

	status := http.StatusOK
	if !report.IsHealthy() {
		status = http.StatusServiceUnavailable
	}

	results := make([]gin.H, len(report.FormatterResults))
	for i, result := range report.FormatterResults {
		results[i] = gin.H{
			"name":        result.Name,
			"healthy":     result.Healthy,
			"error":       result.Error,
			"duration_ms": result.Duration.Milliseconds(),
		}
	}

	c.JSON(status, gin.H{
		"total_formatters":  report.TotalFormatters,
		"healthy_count":     report.HealthyCount,
		"unhealthy_count":   report.UnhealthyCount,
		"health_percentage": report.HealthPercentage(),
		"formatters":        results,
		"timestamp":         report.Timestamp,
	})
}
