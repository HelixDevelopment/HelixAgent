package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/services"
)

// VisionCapability represents a vision capability
type VisionCapability struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Supported   []string `json:"supported_formats"`
}

// VisionRequest represents a vision analysis request
type VisionRequest struct {
	Capability string `json:"capability,omitempty"`
	Image      string `json:"image,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
}

// VisionResponse represents a vision analysis response.
//
// CONST-035 §c (anti-bluff): the Verified field discriminates a real
// vision-provider-driven analysis (Verified=true, Status="completed")
// from a stub response that returns hard-coded labels/captions
// without ever calling a vision-capable LLM (Verified=false,
// Status="stub_only"). Until a real vision provider is wired into
// VisionHandler, every analysis endpoint returns Verified=false so
// CLI agents and SDK consumers can detect that the rich response
// fields (confidence scores, captions, categories) are fabricated
// stubs, not actual model output.
type VisionResponse struct {
	Capability string                 `json:"capability"`
	Status     string                 `json:"status"`
	Verified   bool                   `json:"verified"`
	Result     interface{}            `json:"result,omitempty"`
	Text       string                 `json:"text,omitempty"`
	OCRText    string                 `json:"ocr_text,omitempty"`
	Detections []Detection            `json:"detections,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Duration   int64                  `json:"duration_ms"`
	Timestamp  int64                  `json:"timestamp"`
}

// validateVisionInput rejects requests with neither image nor image_url.
// Without this guard, every vision endpoint silently returned 200 with
// a fabricated "successful analysis" of an empty input — a structural
// bluff per CONST-035 §c.
func validateVisionInput(req VisionRequest) error {
	if strings.TrimSpace(req.Image) == "" && strings.TrimSpace(req.ImageURL) == "" {
		return fmt.Errorf("vision request requires either 'image' (base64) or 'image_url' field")
	}
	return nil
}

// stubOnlyStatus is the canonical status value emitted by every vision
// endpoint until a real vision-capable provider is wired into
// VisionHandler. Callers checking `status` can detect that the
// response did not originate from a real model.
const stubOnlyStatus = "stub_only"

// Detection represents a detected object in an image
type Detection struct {
	Label       string    `json:"label"`
	Confidence  float64   `json:"confidence"`
	BoundingBox []float64 `json:"bounding_box,omitempty"`
}

// VisionHandler handles vision-related endpoints
type VisionHandler struct {
	providerRegistry *services.ProviderRegistry
	logger           *logrus.Logger
	capabilities     map[string]*VisionCapability
}

// NewVisionHandler creates a new vision handler
func NewVisionHandler(providerRegistry *services.ProviderRegistry, logger *logrus.Logger) *VisionHandler {
	h := &VisionHandler{
		providerRegistry: providerRegistry,
		logger:           logger,
		capabilities:     make(map[string]*VisionCapability),
	}

	// Initialize capabilities
	h.initializeCapabilities()

	return h
}

// initializeCapabilities sets up the available vision capabilities
func (h *VisionHandler) initializeCapabilities() {
	caps := []VisionCapability{
		{
			ID:          "analyze",
			Name:        "Image Analysis",
			Description: "General image analysis and understanding",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"},
		},
		{
			ID:          "ocr",
			Name:        "Optical Character Recognition",
			Description: "Extract text from images",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "tiff"},
		},
		{
			ID:          "detect",
			Name:        "Object Detection",
			Description: "Detect and locate objects in images",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp"},
		},
		{
			ID:          "caption",
			Name:        "Image Captioning",
			Description: "Generate captions for images",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp"},
		},
		{
			ID:          "describe",
			Name:        "Image Description",
			Description: "Generate detailed descriptions of images",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp"},
		},
		{
			ID:          "classify",
			Name:        "Image Classification",
			Description: "Classify images into categories",
			Status:      "active",
			Supported:   []string{"png", "jpg", "jpeg", "gif", "webp"},
		},
	}

	for i := range caps {
		h.capabilities[caps[i].ID] = &caps[i]
	}
}

// RegisterRoutes registers vision routes
func (h *VisionHandler) RegisterRoutes(router *gin.RouterGroup) {
	visionGroup := router.Group("/vision")
	{
		// Health endpoint
		visionGroup.GET("/health", h.Health)

		// Capabilities
		visionGroup.GET("/capabilities", h.ListCapabilities)

		// Capability status
		visionGroup.GET("/:capability/status", h.GetCapabilityStatus)

		// Analysis endpoints for each capability
		visionGroup.POST("/analyze", h.Analyze)
		visionGroup.POST("/ocr", h.OCR)
		visionGroup.POST("/detect", h.Detect)
		visionGroup.POST("/caption", h.Caption)
		visionGroup.POST("/describe", h.Describe)
		visionGroup.POST("/classify", h.Classify)

		// Generic endpoint that routes by capability field
		visionGroup.POST("/:capability", h.HandleCapability)
	}
}

// Health returns the health status of the vision service
func (h *VisionHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"service":           "vision",
		"version":           "1.0.0",
		"capabilities":      len(h.capabilities),
		"supported_formats": []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"},
		"timestamp":         time.Now().Unix(),
	})
}

// ListCapabilities returns all available vision capabilities
func (h *VisionHandler) ListCapabilities(c *gin.Context) {
	caps := make([]*VisionCapability, 0, len(h.capabilities))
	for _, cap := range h.capabilities {
		caps = append(caps, cap)
	}

	c.JSON(http.StatusOK, gin.H{
		"capabilities": caps,
		"count":        len(caps),
	})
}

// GetCapabilityStatus returns the status of a specific capability
func (h *VisionHandler) GetCapabilityStatus(c *gin.Context) {
	capabilityID := c.Param("capability")

	cap, exists := h.capabilities[capabilityID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "capability not found",
			"capability": capabilityID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"capability": cap,
		"status":     cap.Status,
		"available":  cap.Status == "active",
	})
}

// HandleCapability handles generic capability requests
func (h *VisionHandler) HandleCapability(c *gin.Context) {
	capability := c.Param("capability")

	// Check if capability exists
	if _, exists := h.capabilities[capability]; !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "capability not found",
			"capability": capability,
		})
		return
	}

	// Route to appropriate handler
	switch capability {
	case "analyze":
		h.Analyze(c)
	case "ocr":
		h.OCR(c)
	case "detect":
		h.Detect(c)
	case "caption":
		h.Caption(c)
	case "describe":
		h.Describe(c)
	case "classify":
		h.Classify(c)
	default:
		h.genericAnalyze(c, capability)
	}
}

// Analyze performs general image analysis.
//
// CONST-035 §c: this is a stub implementation. Until a real
// vision-capable provider is wired in (OpenAI Vision / Gemini Vision
// / Claude Vision), the rich analysis fields (dominant colors,
// quality score, detected objects) are NOT computed from the input
// — they are fabricated constants. The Verified=false discriminator
// and Status="stub_only" tell callers exactly that.
func (h *VisionHandler) Analyze(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":         "stub-only response: no vision-capable provider is wired into VisionHandler yet",
		"content_type": imageInfo["content_type"],
		"source":       imageInfo["source"],
	}

	response := VisionResponse{
		Capability: "analyze",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// OCR extracts text from images. Stub-only — see Analyze for details.
func (h *VisionHandler) OCR(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":           "stub-only response: no vision-capable OCR provider is wired in",
		"extracted_text": "",
		"content_type":   imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: "ocr",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// Detect performs object detection. Stub-only — see Analyze for details.
func (h *VisionHandler) Detect(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":          "stub-only response: no object-detection provider is wired in",
		"detections":    []Detection{},
		"total_objects": 0,
		"content_type":  imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: "detect",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// Caption generates a caption for the image. Stub-only — see Analyze.
func (h *VisionHandler) Caption(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":         "stub-only response: no caption-generating vision provider is wired in",
		"caption":      "",
		"content_type": imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: "caption",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// Describe generates a detailed description. Stub-only — see Analyze.
func (h *VisionHandler) Describe(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":         "stub-only response: no description-generating vision provider is wired in",
		"description":  "",
		"content_type": imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: "describe",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// Classify classifies the image into categories. Stub-only — see Analyze.
func (h *VisionHandler) Classify(c *gin.Context) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"note":            "stub-only response: no classification provider is wired in",
		"classifications": []map[string]interface{}{},
		"content_type":    imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: "classify",
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// genericAnalyze handles generic analysis requests. Stub-only — see Analyze.
func (h *VisionHandler) genericAnalyze(c *gin.Context, capability string) {
	var req VisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateVisionInput(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startTime := time.Now()
	imageInfo := h.processImage(req.Image, req.ImageURL)

	result := map[string]interface{}{
		"capability":   capability,
		"note":         fmt.Sprintf("stub-only response: capability '%s' is not backed by a real vision provider yet", capability),
		"content_type": imageInfo["content_type"],
	}

	response := VisionResponse{
		Capability: capability,
		Status:     stubOnlyStatus,
		Verified:   false,
		Result:     result,
		Metadata:   imageInfo,
		Duration:   time.Since(startTime).Milliseconds(),
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

// processImage extracts information from the image.
//
// CONST-035 §c: previously this function fabricated width=100,
// height=100 for every input regardless of what was actually
// provided. Real dimensions require an image decode (we don't do
// one here yet — that's a vision-provider job), so we no longer
// emit dimension fields at all rather than emit lying ones.
// content_type IS real — derived from base64 magic bytes or URL
// extension.
func (h *VisionHandler) processImage(imageBase64, imageURL string) map[string]interface{} {
	info := map[string]interface{}{
		"source":       "unknown",
		"content_type": "application/octet-stream",
	}

	if imageBase64 != "" {
		// Decode base64 to get image info
		decoded, err := base64.StdEncoding.DecodeString(imageBase64)
		if err == nil {
			info["source"] = "base64"
			info["size_bytes"] = len(decoded)

			// Detect image type from magic bytes
			if len(decoded) >= 8 {
				if decoded[0] == 0x89 && decoded[1] == 'P' && decoded[2] == 'N' && decoded[3] == 'G' {
					info["content_type"] = "image/png"
				} else if decoded[0] == 0xFF && decoded[1] == 0xD8 {
					info["content_type"] = "image/jpeg"
				} else if string(decoded[0:4]) == "GIF8" {
					info["content_type"] = "image/gif"
				} else if string(decoded[0:4]) == "RIFF" && len(decoded) >= 12 && string(decoded[8:12]) == "WEBP" {
					info["content_type"] = "image/webp"
				}
			}
		}
	} else if imageURL != "" {
		info["source"] = "url"
		info["url"] = imageURL

		// Detect content type from URL
		urlLower := strings.ToLower(imageURL)
		switch {
		case strings.Contains(urlLower, ".png"):
			info["content_type"] = "image/png"
		case strings.Contains(urlLower, ".jpg") || strings.Contains(urlLower, ".jpeg"):
			info["content_type"] = "image/jpeg"
		case strings.Contains(urlLower, ".gif"):
			info["content_type"] = "image/gif"
		case strings.Contains(urlLower, ".webp"):
			info["content_type"] = "image/webp"
		}
	}

	return info
}
