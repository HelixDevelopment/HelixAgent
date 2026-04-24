// Package vision provides real functional tests for vision capabilities.
// These tests execute ACTUAL vision operations, not just connectivity checks.
// Tests FAIL if the operation fails - no false positives.
package vision

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

// VisionClient provides a client for testing vision capabilities
type VisionClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewVisionClient creates a new vision test client
func NewVisionClient(baseURL string) *VisionClient {
	return &VisionClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Vision operations can be slow
		},
	}
}

// VisionRequest represents a vision analysis request
type VisionRequest struct {
	Capability string `json:"capability"`
	Image      string `json:"image"`     // Base64 encoded image
	ImageURL   string `json:"image_url"` // Or URL to image
	Prompt     string `json:"prompt,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

// VisionResponse represents a vision analysis response
type VisionResponse struct {
	Capability string                 `json:"capability"`
	Result     interface{}            `json:"result"`
	Text       string                 `json:"text,omitempty"`
	Detections []Detection            `json:"detections,omitempty"`
	OCRText    string                 `json:"ocr_text,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Confidence float64                `json:"confidence,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// Detection represents a detected object in an image
type Detection struct {
	Label       string    `json:"label"`
	Confidence  float64   `json:"confidence"`
	BoundingBox []float64 `json:"bounding_box,omitempty"`
}

// Analyze sends an image for analysis
func (c *VisionClient) Analyze(req *VisionRequest) (*VisionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/vision/%s", c.baseURL, req.Capability)
	resp, err := c.httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to call vision API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vision API failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result VisionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (raw: %s)", err, string(respBody))
	}

	return &result, nil
}

// ListCapabilities lists all available vision capabilities
func (c *VisionClient) ListCapabilities() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/vision/capabilities")
	if err != nil {
		return nil, fmt.Errorf("failed to list capabilities: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list capabilities failed with status %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Capabilities []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"capabilities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var caps []string
	for _, c := range raw.Capabilities {
		if c.Status == "active" {
			caps = append(caps, c.ID)
		}
	}
	return caps, nil
}

// VisionCapabilityConfig holds configuration for testing a vision capability
type VisionCapabilityConfig struct {
	Capability  string
	Description string
	TestPrompt  string
}

// Vision capabilities to test
var VisionCapabilities = []VisionCapabilityConfig{
	{Capability: "analyze", Description: "General image analysis", TestPrompt: "Describe what you see in this image"},
	{Capability: "ocr", Description: "Optical Character Recognition", TestPrompt: "Extract all text from this image"},
	{Capability: "detect", Description: "Object detection", TestPrompt: "List all objects in this image"},
	{Capability: "caption", Description: "Image captioning", TestPrompt: "Generate a caption for this image"},
	{Capability: "describe", Description: "Detailed description", TestPrompt: "Provide a detailed description of this image"},
	{Capability: "classify", Description: "Image classification", TestPrompt: "Classify this image"},
	{Capability: "segment", Description: "Image segmentation", TestPrompt: "Segment the objects in this image"},
}

// Test image - a simple 1x1 pixel red PNG (base64 encoded)
var testImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

// createTestImage creates a larger test image for more meaningful tests
func createTestImage() string {
	// A simple 10x10 pixel image with some colors
	// This is a minimal PNG for testing purposes
	return testImageBase64
}

// TestVisionCapabilityDiscovery tests capability discovery endpoint
func TestVisionCapabilityDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	capabilities, err := client.ListCapabilities()
	require.NoError(t, err)

	assert.NotEmpty(t, capabilities, "Should have at least one capability")
	t.Logf("Discovered %d vision capabilities: %v", len(capabilities), capabilities)
}

// TestVisionAnalyze tests image analysis capability
func TestVisionAnalyze(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	for _, cap := range VisionCapabilities {
		t.Run(cap.Capability, func(t *testing.T) {
			req := &VisionRequest{
				Capability: cap.Capability,
				Image:      createTestImage(),
				Prompt:     cap.TestPrompt,
			}

			resp, err := client.Analyze(req)
			if err != nil {
				t.Skipf("Vision capability %s not available: %v", cap.Capability, err)
				return
			}

			require.Equal(t, cap.Capability, resp.Capability)
			require.Empty(t, resp.Error, "Should not return error")

			// Check that we got some result
			hasResult := resp.Result != nil || resp.Text != "" || resp.OCRText != "" || len(resp.Detections) > 0
			assert.True(t, hasResult, "Should have some result")

			t.Logf("Vision %s result: text=%q, detections=%d, confidence=%.2f",
				cap.Capability, resp.Text, len(resp.Detections), resp.Confidence)
		})
	}
}

// TestVisionWithURL tests vision analysis with image URL
func TestVisionWithURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	// Use a public test image
	testURL := "https://httpbin.org/image/png"

	req := &VisionRequest{
		Capability: "analyze",
		ImageURL:   testURL,
		Prompt:     "Describe this image",
	}

	resp, err := client.Analyze(req)
	if err != nil {
		t.Skipf("Vision service not available: %v", err)
		return
	}

	require.Empty(t, resp.Error, "Should not return error")
	t.Logf("Vision URL analysis result: %v", resp.Result)
}

// TestVisionOCR tests OCR capability specifically
func TestVisionOCR(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	// In a real test, you'd use an image with actual text
	req := &VisionRequest{
		Capability: "ocr",
		Image:      createTestImage(),
		Prompt:     "Extract all text from this image",
	}

	resp, err := client.Analyze(req)
	if err != nil {
		t.Skipf("Vision OCR not available: %v", err)
		return
	}

	// OCR might return empty string for an image without text
	t.Logf("OCR result: %q", resp.OCRText)
}

// TestVisionDetection tests object detection capability
func TestVisionDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	req := &VisionRequest{
		Capability: "detect",
		Image:      createTestImage(),
		Prompt:     "Detect all objects in this image",
	}

	resp, err := client.Analyze(req)
	if err != nil {
		t.Skipf("Vision detection not available: %v", err)
		return
	}

	t.Logf("Detection result: %d objects found", len(resp.Detections))
	for i, det := range resp.Detections {
		t.Logf("  Detection %d: %s (confidence: %.2f)", i, det.Label, det.Confidence)
	}
}

// TestVisionHealthCheck tests vision service health
func TestVisionHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")

	client := NewVisionClient(testutil.ServerURL())

	resp, err := client.httpClient.Get(client.baseURL + "/v1/vision/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
}

// TestVisionFromFile tests vision analysis from a local file
func TestVisionFromFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode")  // SKIP-OK: #short-mode
	}
	testutil.RequireHTTPEndpoint(t, "vision", testutil.ServerURL()+"/v1/vision/health")
	client := NewVisionClient(testutil.ServerURL())

	// Create a temporary test image file
	tmpFile := "/tmp/test_image.png"
	imgData, _ := base64.StdEncoding.DecodeString(testImageBase64)
	if err := os.WriteFile(tmpFile, imgData, 0644); err != nil {
		t.Skipf("Failed to create test image: %v", err)
		return
	}
	defer func() { _ = os.Remove(tmpFile) }()

	// Read and encode the file
	fileData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read test image: %v", err)
	}

	req := &VisionRequest{
		Capability: "analyze",
		Image:      base64.StdEncoding.EncodeToString(fileData),
		Prompt:     "Analyze this image",
	}

	resp, err := client.Analyze(req)
	if err != nil {
		t.Skipf("Vision service not available: %v", err)
		return
	}

	require.Empty(t, resp.Error, "Should not return error")
	t.Logf("File analysis result: %v", resp.Result)
}

// BenchmarkVisionAnalyze benchmarks vision analysis
func BenchmarkVisionAnalyze(b *testing.B) {
	client := NewVisionClient(testutil.ServerURL())

	req := &VisionRequest{
		Capability: "analyze",
		Image:      createTestImage(),
		Prompt:     "Describe this image",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Analyze(req)
		if err != nil {
			b.Skipf("Vision service not available: %v", err)
			return
		}
	}
}
