package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Helpers
// ============================================================================

func setupVisionHandler() (*VisionHandler, *gin.Engine) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	h := NewVisionHandler(nil, logger)
	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)
	return h, r
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewVisionHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewVisionHandler(nil, logger)

	assert.NotNil(t, h)
	assert.Nil(t, h.providerRegistry)
	assert.Equal(t, logger, h.logger)
	assert.NotNil(t, h.capabilities)
	assert.Len(t, h.capabilities, 6)
}

func TestNewVisionHandler_CapabilitiesInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewVisionHandler(nil, logger)

	expectedIDs := []string{"analyze", "ocr", "detect", "caption", "describe", "classify"}
	for _, id := range expectedIDs {
		cap, exists := h.capabilities[id]
		assert.True(t, exists, "capability %q should exist", id)
		assert.Equal(t, id, cap.ID)
		assert.Equal(t, "active", cap.Status)
		assert.NotEmpty(t, cap.Name)
		assert.NotEmpty(t, cap.Description)
		assert.NotEmpty(t, cap.Supported)
	}
}

// ============================================================================
// Type Tests
// ============================================================================

func TestVisionCapability_Fields(t *testing.T) {
	t.Parallel()
	cap := VisionCapability{
		ID:          "test-cap",
		Name:        "Test Capability",
		Description: "A test capability",
		Status:      "active",
		Supported:   []string{"png", "jpg"},
	}

	assert.Equal(t, "test-cap", cap.ID)
	assert.Equal(t, "Test Capability", cap.Name)
	assert.Equal(t, "A test capability", cap.Description)
	assert.Equal(t, "active", cap.Status)
	assert.Equal(t, []string{"png", "jpg"}, cap.Supported)
}

func TestVisionRequest_Fields(t *testing.T) {
	t.Parallel()
	req := VisionRequest{
		Capability: "analyze",
		Image:      "base64data",
		ImageURL:   "https://example.com/img.png",
		Prompt:     "describe this",
	}

	assert.Equal(t, "analyze", req.Capability)
	assert.Equal(t, "base64data", req.Image)
	assert.Equal(t, "https://example.com/img.png", req.ImageURL)
	assert.Equal(t, "describe this", req.Prompt)
}

func TestVisionResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := VisionResponse{
		Capability: "analyze",
		Status:     "completed",
		Result:     map[string]interface{}{"key": "value"},
		Text:       "result text",
		OCRText:    "ocr text",
		Detections: []Detection{
			{Label: "object", Confidence: 0.95, BoundingBox: []float64{10, 10, 90, 90}},
		},
		Metadata: map[string]interface{}{"source": "base64"},
		Duration: 42,
	}

	assert.Equal(t, "analyze", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.NotNil(t, resp.Result)
	assert.Equal(t, "result text", resp.Text)
	assert.Equal(t, "ocr text", resp.OCRText)
	assert.Len(t, resp.Detections, 1)
	assert.Equal(t, "object", resp.Detections[0].Label)
	assert.Equal(t, int64(42), resp.Duration)
}

func TestDetection_Fields(t *testing.T) {
	t.Parallel()
	d := Detection{
		Label:       "cat",
		Confidence:  0.99,
		BoundingBox: []float64{0, 0, 100, 100},
	}

	assert.Equal(t, "cat", d.Label)
	assert.Equal(t, 0.99, d.Confidence)
	assert.Equal(t, []float64{0, 0, 100, 100}, d.BoundingBox)
}

func TestVisionResponse_JSONSerialization(t *testing.T) {
	t.Parallel()
	resp := VisionResponse{
		Capability: "ocr",
		Status:     "completed",
		Text:       "Hello World",
		Duration:   100,
		Timestamp:  1700000000,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded VisionResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Capability, decoded.Capability)
	assert.Equal(t, resp.Status, decoded.Status)
	assert.Equal(t, resp.Text, decoded.Text)
	assert.Equal(t, resp.Duration, decoded.Duration)
	assert.Equal(t, resp.Timestamp, decoded.Timestamp)
}

// ============================================================================
// Health Endpoint Tests
// ============================================================================

func TestVisionHandler_Health(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/vision/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp["status"])
	assert.Equal(t, "vision", resp["service"])
	assert.Equal(t, "1.0.0", resp["version"])
	assert.Equal(t, float64(6), resp["capabilities"])
	assert.NotNil(t, resp["timestamp"])
	assert.NotNil(t, resp["supported_formats"])
}

// ============================================================================
// ListCapabilities Tests
// ============================================================================

func TestVisionHandler_ListCapabilities(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/vision/capabilities", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(6), resp["count"])
	caps, ok := resp["capabilities"].([]interface{})
	require.True(t, ok)
	assert.Len(t, caps, 6)
}

// ============================================================================
// GetCapabilityStatus Tests
// ============================================================================

func TestVisionHandler_GetCapabilityStatus_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		capability   string
		expectedName string
	}{
		{"analyze", "analyze", "Image Analysis"},
		{"ocr", "ocr", "Optical Character Recognition"},
		{"detect", "detect", "Object Detection"},
		{"caption", "caption", "Image Captioning"},
		{"describe", "describe", "Image Description"},
		{"classify", "classify", "Image Classification"},
	}

	_, r := setupVisionHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(
				"GET",
				"/v1/vision/"+tt.capability+"/status",
				nil,
			)
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, "active", resp["status"])
			assert.True(t, resp["available"].(bool))
		})
	}
}

func TestVisionHandler_GetCapabilityStatus_NotFound(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/vision/nonexistent/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "capability not found", resp["error"])
	assert.Equal(t, "nonexistent", resp["capability"])
}

// ============================================================================
// Analyze Tests
// ============================================================================

func TestVisionHandler_Analyze_WithBase64Image(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	// Create a valid base64-encoded string (fake PNG header)
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	imageData := base64.StdEncoding.EncodeToString(pngHeader)

	body, _ := json.Marshal(VisionRequest{
		Image:  imageData,
		Prompt: "analyze this image",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "analyze", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, "Image analysis completed", resp.Text)
	assert.NotNil(t, resp.Result)
	assert.NotNil(t, resp.Metadata)
	assert.True(t, resp.Timestamp > 0)
}

func TestVisionHandler_Analyze_WithImageURL(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/image.png",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "analyze", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	metadata := resp.Metadata
	assert.Equal(t, "url", metadata["source"])
	assert.Equal(t, "image/png", metadata["content_type"])
}

func TestVisionHandler_Analyze_EmptyRequest(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "analyze", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, "unknown", resp.Metadata["source"])
}

func TestVisionHandler_Analyze_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/analyze",
		bytes.NewBuffer([]byte(`{invalid json}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// OCR Tests
// ============================================================================

func TestVisionHandler_OCR_Success(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/text.png",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/ocr", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "ocr", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.NotNil(t, resp.Result)
}

func TestVisionHandler_OCR_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/ocr",
		bytes.NewBuffer([]byte(`not json`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Detect Tests
// ============================================================================

func TestVisionHandler_Detect_Success(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/objects.jpg",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/detect", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "detect", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Len(t, resp.Detections, 1)
	assert.Equal(t, "object", resp.Detections[0].Label)
	assert.Equal(t, 0.95, resp.Detections[0].Confidence)
	assert.Len(t, resp.Detections[0].BoundingBox, 4)
}

func TestVisionHandler_Detect_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/detect",
		bytes.NewBuffer([]byte(`{bad`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Caption Tests
// ============================================================================

func TestVisionHandler_Caption_Success(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/photo.webp",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/caption", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "caption", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Contains(t, resp.Text, "image")
}

func TestVisionHandler_Caption_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/caption",
		bytes.NewBuffer([]byte(`???`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Describe Tests
// ============================================================================

func TestVisionHandler_Describe_Success(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/scene.gif",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/describe", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "describe", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Contains(t, resp.Text, "visual content")
}

func TestVisionHandler_Describe_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/describe",
		bytes.NewBuffer([]byte(`[`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// Classify Tests
// ============================================================================

func TestVisionHandler_Classify_Success(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/item.jpg",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/classify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "classify", resp.Capability)
	assert.Equal(t, "completed", resp.Status)
	assert.Contains(t, resp.Text, "general")
}

func TestVisionHandler_Classify_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/classify",
		bytes.NewBuffer([]byte(`{{`)),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// HandleCapability Tests (generic routing)
// ============================================================================

func TestVisionHandler_HandleCapability_RoutesCorrectly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		{"routes to analyze", "analyze", "analyze"},
		{"routes to ocr", "ocr", "ocr"},
		{"routes to detect", "detect", "detect"},
		{"routes to caption", "caption", "caption"},
		{"routes to describe", "describe", "describe"},
		{"routes to classify", "classify", "classify"},
	}

	_, r := setupVisionHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			body, _ := json.Marshal(VisionRequest{
				ImageURL: "https://example.com/test.png",
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(
				"POST",
				"/v1/vision/"+tt.capability,
				bytes.NewBuffer(body),
			)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp VisionResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, resp.Capability)
			assert.Equal(t, "completed", resp.Status)
		})
	}
}

func TestVisionHandler_HandleCapability_NotFound(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/test.png",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/vision/nonexistent_cap",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "capability not found", resp["error"])
}

// ============================================================================
// processImage Tests (via handler responses)
// ============================================================================

func TestVisionHandler_ProcessImage_Base64PNG(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	imageData := base64.StdEncoding.EncodeToString(pngHeader)

	body, _ := json.Marshal(VisionRequest{Image: imageData})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "base64", resp.Metadata["source"])
	assert.Equal(t, "image/png", resp.Metadata["content_type"])
}

func TestVisionHandler_ProcessImage_Base64JPEG(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	imageData := base64.StdEncoding.EncodeToString(jpegHeader)

	body, _ := json.Marshal(VisionRequest{Image: imageData})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "base64", resp.Metadata["source"])
	assert.Equal(t, "image/jpeg", resp.Metadata["content_type"])
}

func TestVisionHandler_ProcessImage_Base64GIF(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	gifHeader := []byte("GIF89a\x00\x00\x00\x00")
	imageData := base64.StdEncoding.EncodeToString(gifHeader)

	body, _ := json.Marshal(VisionRequest{Image: imageData})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "base64", resp.Metadata["source"])
	assert.Equal(t, "image/gif", resp.Metadata["content_type"])
}

func TestVisionHandler_ProcessImage_Base64WEBP(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	webpHeader := []byte("RIFF\x00\x00\x00\x00WEBP\x00\x00")
	imageData := base64.StdEncoding.EncodeToString(webpHeader)

	body, _ := json.Marshal(VisionRequest{Image: imageData})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "base64", resp.Metadata["source"])
	assert.Equal(t, "image/webp", resp.Metadata["content_type"])
}

func TestVisionHandler_ProcessImage_URLContentTypeDetection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		url         string
		contentType string
	}{
		{"png url", "https://example.com/img.png", "image/png"},
		{"jpg url", "https://example.com/img.jpg", "image/jpeg"},
		{"jpeg url", "https://example.com/img.JPEG", "image/jpeg"},
		{"gif url", "https://example.com/img.gif", "image/gif"},
		{"webp url", "https://example.com/img.webp", "image/webp"},
	}

	_, r := setupVisionHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			body, _ := json.Marshal(VisionRequest{ImageURL: tt.url})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp VisionResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, "url", resp.Metadata["source"])
			assert.Equal(t, tt.contentType, resp.Metadata["content_type"])
		})
	}
}

func TestVisionHandler_ProcessImage_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		Image: "not-valid-base64!!!",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Handler should still succeed; invalid base64 falls through to default info
	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Source stays "unknown" because base64 decode failed
	assert.Equal(t, "unknown", resp.Metadata["source"])
}

func TestVisionHandler_ProcessImage_NoImageProvided(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "unknown", resp.Metadata["source"])
}

// ============================================================================
// Route Registration Tests
// ============================================================================

func TestVisionHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	h := NewVisionHandler(nil, logger)
	r := gin.New()
	api := r.Group("/v1")
	h.RegisterRoutes(api)

	// Test GET routes respond
	getRoutes := []string{
		"/v1/vision/health",
		"/v1/vision/capabilities",
		"/v1/vision/analyze/status",
	}

	for _, route := range getRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		r.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusNotFound, w.Code, "Route %s should be registered", route)
	}
}

// ============================================================================
// Content-Type Tests
// ============================================================================

func TestVisionHandler_ResponseContentType(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/v1/vision/health", ""},
		{"GET", "/v1/vision/capabilities", ""},
		{"POST", "/v1/vision/analyze", `{"image_url":"https://example.com/img.png"}`},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			var req *http.Request
			if ep.body != "" {
				req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(ep.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(ep.method, ep.path, nil)
			}
			r.ServeHTTP(w, req)

			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		})
	}
}

// ============================================================================
// Duration and Timestamp Tests
// ============================================================================

func TestVisionHandler_Analyze_HasDurationAndTimestamp(t *testing.T) {
	t.Parallel()
	_, r := setupVisionHandler()

	body, _ := json.Marshal(VisionRequest{
		ImageURL: "https://example.com/test.png",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/vision/analyze", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp VisionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Duration >= 0)
	assert.True(t, resp.Timestamp > 0)
}

// ============================================================================
// All Capability Endpoint BadRequest Tests (table-driven)
// ============================================================================

func TestVisionHandler_AllEndpoints_BadRequest(t *testing.T) {
	t.Parallel()
	endpoints := []string{
		"/v1/vision/analyze",
		"/v1/vision/ocr",
		"/v1/vision/detect",
		"/v1/vision/caption",
		"/v1/vision/describe",
		"/v1/vision/classify",
	}

	_, r := setupVisionHandler()

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(
				"POST",
				ep,
				bytes.NewBufferString(`invalid`),
			)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}
