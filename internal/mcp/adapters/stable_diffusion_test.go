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

func TestNewStableDiffusionAdapter(t *testing.T) {
	t.Parallel()
	config := StableDiffusionConfig{
		BaseURL: "http://localhost:7860",
	}

	adapter := NewStableDiffusionAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "http://localhost:7860", adapter.config.BaseURL)
}

func TestStableDiffusionAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})

	info := adapter.GetServerInfo()

	assert.Equal(t, "stable-diffusion", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Stable Diffusion")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "txt2img")
	assert.Contains(t, info.Capabilities, "img2img")
	assert.Contains(t, info.Capabilities, "controlnet")
}

func TestStableDiffusionAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["sd_txt2img"])
	assert.True(t, toolNames["sd_img2img"])
	assert.True(t, toolNames["sd_controlnet"])
	assert.True(t, toolNames["sd_upscale"])
	assert.True(t, toolNames["sd_list_models"])
	assert.True(t, toolNames["sd_list_samplers"])
	assert.True(t, toolNames["sd_list_loras"])
	assert.True(t, toolNames["sd_set_model"])
	assert.True(t, toolNames["sd_get_progress"])
}

func TestStableDiffusionAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestStableDiffusionAdapter_Txt2Img(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/txt2img", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "a beautiful sunset", body["prompt"])
		assert.Equal(t, float64(512), body["width"])
		assert.Equal(t, float64(512), body["height"])

		response := SDImageResponse{
			Images: []string{"base64encodedimage"},
			Info: SDInfo{
				Seed:   12345,
				Prompt: "a beautiful sunset",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_txt2img", map[string]interface{}{
		"prompt": "a beautiful sunset",
		"width":  512,
		"height": 512,
		"steps":  20,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "Generated")
}

func TestStableDiffusionAdapter_Img2Img(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/img2img", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "enhanced version", body["prompt"])
		assert.NotNil(t, body["init_images"])

		response := SDImageResponse{
			Images: []string{"base64encodedimage"},
			Info: SDInfo{
				Seed: 12345,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_img2img", map[string]interface{}{
		"prompt":             "enhanced version",
		"init_image":         "base64inputimage",
		"denoising_strength": 0.75,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "img2img")
}

func TestStableDiffusionAdapter_ListModels(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/sd-models", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		response := []SDModel{
			{Title: "v1-5-pruned", ModelName: "v1-5-pruned.safetensors"},
			{Title: "sd-xl-base", ModelName: "sd_xl_base_1.0.safetensors"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_list_models", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "v1-5-pruned")
}

func TestStableDiffusionAdapter_ListSamplers(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/samplers", r.URL.Path)

		response := []SDSampler{
			{Name: "Euler"},
			{Name: "DPM++ 2M"},
			{Name: "DDIM"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_list_samplers", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Euler")
}

func TestStableDiffusionAdapter_ListLoras(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/loras", r.URL.Path)

		response := []SDLora{
			{Name: "add_detail", Alias: "detail"},
			{Name: "anime_style", Alias: "anime"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_list_loras", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "add_detail")
}

func TestStableDiffusionAdapter_SetModel(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/options", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "sd_xl_base_1.0.safetensors", body["sd_model_checkpoint"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_set_model", map[string]interface{}{
		"model_name": "sd_xl_base_1.0.safetensors",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Model set to")
}

func TestStableDiffusionAdapter_GetProgress(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/progress", r.URL.Path)

		response := SDProgress{
			Progress:    0.5,
			ETARelative: 10.5,
			State: &SDState{
				SamplingStep:  10,
				SamplingSteps: 20,
				Job:           "txt2img",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_get_progress", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "50.0%")
}

func TestStableDiffusionAdapter_Upscale(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/extra-single-image", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		response := SDUpscaleResponse{
			Image: "base64upscaledimage",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_upscale", map[string]interface{}{
		"image":    "base64inputimage",
		"upscaler": "ESRGAN_4x",
		"scale":    2.0,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Upscaled")
}

func TestStableDiffusionAdapter_ControlNet(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sdapi/v1/txt2img", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["alwayson_scripts"])

		response := SDImageResponse{
			Images: []string{"base64encodedimage"},
			Info:   SDInfo{Seed: 12345},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	result, err := adapter.CallTool(context.Background(), "sd_controlnet", map[string]interface{}{
		"prompt":            "a person standing",
		"control_image":     "base64poseimage",
		"controlnet_model":  "openpose",
		"controlnet_weight": 1.0,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "ControlNet")
}

func TestStableDiffusionAdapter_WithAuth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.NotEmpty(t, authHeader)
		assert.Contains(t, authHeader, "Basic")

		response := []SDModel{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
		Auth: &SDAuth{
			Username: "user",
			Password: "pass",
		},
	})

	result, err := adapter.CallTool(context.Background(), "sd_list_models", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestDefaultStableDiffusionConfig(t *testing.T) {
	t.Parallel()
	config := DefaultStableDiffusionConfig()

	assert.Equal(t, "http://127.0.0.1:7860", config.BaseURL)
	assert.Equal(t, 300*time.Second, config.Timeout)
}
