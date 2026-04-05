// Package adapters provides MCP server adapter tests.
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSVGMakerAdapter(t *testing.T) {
	t.Parallel()
	config := SVGMakerConfig{
		APIKey: "test-key",
	}

	adapter := NewSVGMakerAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "test-key", adapter.config.APIKey)
	assert.Equal(t, DefaultSVGMakerConfig().BaseURL, adapter.config.BaseURL)
}

func TestSVGMakerAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewSVGMakerAdapter(SVGMakerConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "svgmaker", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "SVG")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "generate_svg")
	assert.Contains(t, info.Capabilities, "edit_svg")
	assert.Contains(t, info.Capabilities, "optimize_svg")
}

func TestSVGMakerAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewSVGMakerAdapter(SVGMakerConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["svg_generate"])
	assert.True(t, toolNames["svg_edit"])
	assert.True(t, toolNames["svg_optimize"])
	assert.True(t, toolNames["svg_from_image"])
	assert.True(t, toolNames["svg_icon"])
	assert.True(t, toolNames["svg_to_png"])
	assert.True(t, toolNames["svg_analyze"])
	assert.True(t, toolNames["svg_combine"])
}

func TestSVGMakerAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewSVGMakerAdapter(SVGMakerConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestSVGMakerAdapter_GenerateSVG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/generate", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "a red circle", body["prompt"])

		response := SVGGenerateResponse{
			SVG:    `<svg><circle cx="50" cy="50" r="40" fill="red"/></svg>`,
			Width:  256,
			Height: 256,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_generate", map[string]interface{}{
		"prompt": "a red circle",
		"style":  "minimal",
		"width":  256,
		"height": 256,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Generated SVG")
}

func TestSVGMakerAdapter_EditSVG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/edit", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body["svg"])
		assert.Equal(t, "change color to blue", body["instructions"])

		response := SVGEditResponse{
			SVG:     `<svg><circle cx="50" cy="50" r="40" fill="blue"/></svg>`,
			Changes: []string{"Changed fill color from red to blue"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_edit", map[string]interface{}{
		"svg":          `<svg><circle fill="red"/></svg>`,
		"instructions": "change color to blue",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Edited SVG")
}

func TestSVGMakerAdapter_OptimizeSVG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/optimize", r.URL.Path)

		response := SVGOptimizeResponse{
			SVG:           `<svg><circle/></svg>`,
			OriginalSize:  1000,
			OptimizedSize: 500,
			Reduction:     50.0,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_optimize", map[string]interface{}{
		"svg":             `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40"/></svg>`,
		"precision":       2,
		"remove_metadata": true,
		"minify":          true,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Optimized")
	assert.Contains(t, result.Content[0].Text, "50.0%")
}

func TestSVGMakerAdapter_ImageToSVG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vectorize", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body["image"])
		assert.Equal(t, "color", body["mode"])

		response := SVGVectorizeResponse{
			SVG:       `<svg><path d="M0,0 L100,100"/></svg>`,
			PathCount: 25,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_from_image", map[string]interface{}{
		"image":        "base64encodedimage",
		"mode":         "color",
		"detail_level": "medium",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Vectorized")
	assert.Contains(t, result.Content[0].Text, "25")
}

func TestSVGMakerAdapter_GenerateIcon(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/icon", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "settings", body["concept"])

		response := SVGIconResponse{
			SVG:  `<svg><path d="M12,2 L14,22"/></svg>`,
			Name: "settings",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_icon", map[string]interface{}{
		"concept":      "settings",
		"style":        "outline",
		"size":         24,
		"stroke_width": 2.0,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "settings")
}

func TestSVGMakerAdapter_SVGToPNG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/convert/png", r.URL.Path)

		response := SVGConvertResponse{
			Image:  "base64pngimage",
			Width:  256,
			Height: 256,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_to_png", map[string]interface{}{
		"svg":        `<svg><circle/></svg>`,
		"width":      256,
		"height":     256,
		"background": "transparent",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "PNG")
}

func TestSVGMakerAdapter_AnalyzeSVG(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/analyze", r.URL.Path)

		response := SVGAnalyzeResponse{
			Width:    256,
			Height:   256,
			ViewBox:  "0 0 256 256",
			FileSize: 500,
			Elements: map[string]int{
				"circle": 3,
				"rect":   2,
				"path":   5,
			},
			Colors: []string{"#ff0000", "#00ff00", "#0000ff"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_analyze", map[string]interface{}{
		"svg": `<svg><circle/><rect/></svg>`,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "256x256")
	assert.Contains(t, result.Content[0].Text, "circle")
}

func TestSVGMakerAdapter_CombineSVGs(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/combine", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		svgs := body["svgs"].([]interface{})
		assert.Equal(t, 2, len(svgs))

		response := SVGCombineResponse{
			SVG:    `<svg><g/><g/></svg>`,
			Width:  512,
			Height: 256,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_combine", map[string]interface{}{
		"svgs":    []interface{}{"<svg>1</svg>", "<svg>2</svg>"},
		"layout":  "horizontal",
		"spacing": 10,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Combined 2 SVGs")
}

func TestSVGMakerAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	adapter := NewSVGMakerAdapter(SVGMakerConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "svg_generate", map[string]interface{}{
		"prompt": "test",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestDefaultSVGMakerConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerConfig()

	assert.Equal(t, "https://api.svgmaker.io/v1", config.BaseURL)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestGetBoolArg(t *testing.T) {
	t.Parallel()
	args := map[string]interface{}{
		"true_val":  true,
		"false_val": false,
		"string":    "true",
	}

	assert.True(t, getBoolArg(args, "true_val", false))
	assert.False(t, getBoolArg(args, "false_val", true))
	assert.True(t, getBoolArg(args, "missing", true))
	assert.False(t, getBoolArg(args, "missing", false))
	assert.False(t, getBoolArg(args, "string", false)) // string "true" is not bool
}
