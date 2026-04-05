package servers

import (
	"context"
	"encoding/base64"
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
	tests := []struct {
		name   string
		config StableDiffusionConfig
		want   struct {
			baseURL string
			timeout time.Duration
		}
	}{
		{
			name: "with_custom_config",
			config: StableDiffusionConfig{
				BaseURL:  "http://192.168.1.100:7860",
				Username: "admin",
				Password: "password",
				Timeout:  600 * time.Second,
			},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "http://192.168.1.100:7860",
				timeout: 600 * time.Second,
			},
		},
		{
			name:   "with_default_values",
			config: StableDiffusionConfig{},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "http://127.0.0.1:7860",
				timeout: 300 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			adapter := NewStableDiffusionAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.want.baseURL, adapter.baseURL)
			assert.Equal(t, tt.want.timeout, adapter.client.Timeout)
		})
	}
}

func TestStableDiffusionAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_connection",
			statusCode: http.StatusOK,
			response: []map[string]interface{}{
				{"name": "Euler", "aliases": []string{"euler"}},
			},
			wantErr: false,
		},
		{
			name:        "authentication_failure",
			statusCode:  http.StatusUnauthorized,
			response:    map[string]string{"detail": "Unauthorized"},
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "server_error",
			statusCode:  http.StatusInternalServerError,
			response:    map[string]string{"detail": "Internal error"},
			wantErr:     true,
			errContains: "failed to connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/sdapi/v1/samplers", r.URL.Path)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			err := adapter.Connect(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, adapter.connected)
			}
		})
	}
}

func TestStableDiffusionAdapter_Health(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		connected  bool
		statusCode int
		wantErr    bool
	}{
		{
			name:       "healthy",
			connected:  true,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not_connected",
			connected:  false,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "unhealthy",
			connected:  true,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "Euler"}})
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})
			adapter.connected = tt.connected

			err := adapter.Health(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStableDiffusionAdapter_Txt2Img(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		request    *SDTxt2ImgRequest
		statusCode int
		response   interface{}
		wantImages int
		wantErr    bool
	}{
		{
			name: "successful_generation",
			request: &SDTxt2ImgRequest{
				Prompt: "a beautiful sunset",
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"images": []string{base64.StdEncoding.EncodeToString([]byte("fake_image_data"))},
				"parameters": map[string]interface{}{
					"prompt": "a beautiful sunset",
				},
				"info": "{}",
			},
			wantImages: 1,
			wantErr:    false,
		},
		{
			name: "generation_with_options",
			request: &SDTxt2ImgRequest{
				Prompt:         "a cat",
				NegativePrompt: "blurry",
				Width:          1024,
				Height:         1024,
				Steps:          30,
				CFGScale:       9.0,
				SamplerName:    "DPM++ 2M Karras",
				Seed:           12345,
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"images":     []string{"base64image"},
				"parameters": map[string]interface{}{},
				"info":       "{}",
			},
			wantImages: 1,
			wantErr:    false,
		},
		{
			name: "generation_failure",
			request: &SDTxt2ImgRequest{
				Prompt: "test",
			},
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"detail": "Generation failed"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/sdapi/v1/txt2img", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotEmpty(t, body["prompt"])

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			result, err := adapter.Txt2Img(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result.Images, tt.wantImages)
			}
		})
	}
}

func TestStableDiffusionAdapter_Img2Img(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		request    *SDImg2ImgRequest
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name: "successful_transformation",
			request: &SDImg2ImgRequest{
				InitImages: []string{base64.StdEncoding.EncodeToString([]byte("input_image"))},
				Prompt:     "make it more colorful",
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"images":     []string{"output_image"},
				"parameters": map[string]interface{}{},
				"info":       "{}",
			},
			wantErr: false,
		},
		{
			name: "inpainting",
			request: &SDImg2ImgRequest{
				InitImages:        []string{"image"},
				Prompt:            "a red car",
				Mask:              "mask_image",
				MaskBlur:          4,
				InpaintingFill:    1,
				DenoisingStrength: 0.9,
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"images":     []string{"inpainted_image"},
				"parameters": map[string]interface{}{},
				"info":       "{}",
			},
			wantErr: false,
		},
		{
			name: "transformation_failure",
			request: &SDImg2ImgRequest{
				InitImages: []string{"bad_image"},
				Prompt:     "test",
			},
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"detail": "Invalid image"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/sdapi/v1/img2img", r.URL.Path)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			result, err := adapter.Img2Img(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result.Images)
			}
		})
	}
}

func TestStableDiffusionAdapter_Upscale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		request    *SDUpscaleRequest
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name: "successful_upscale",
			request: &SDUpscaleRequest{
				Image:           base64.StdEncoding.EncodeToString([]byte("input_image")),
				UpscalingResize: 2.0,
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"image":     "upscaled_image",
				"html_info": "<p>Info</p>",
			},
			wantErr: false,
		},
		{
			name: "upscale_failure",
			request: &SDUpscaleRequest{
				Image: "bad_image",
			},
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"detail": "Upscale failed"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "extra")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			result, err := adapter.Upscale(context.Background(), tt.request)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result.Image)
			}
		})
	}
}

func TestStableDiffusionAdapter_GetProgress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		skipCurrentImage bool
		statusCode       int
		response         interface{}
		wantProgress     float64
		wantErr          bool
	}{
		{
			name:             "generation_in_progress",
			skipCurrentImage: false,
			statusCode:       http.StatusOK,
			response: map[string]interface{}{
				"progress":     0.5,
				"eta_relative": 10.0,
				"state": map[string]interface{}{
					"job": "text2img",
				},
			},
			wantProgress: 0.5,
			wantErr:      false,
		},
		{
			name:             "generation_complete",
			skipCurrentImage: true,
			statusCode:       http.StatusOK,
			response: map[string]interface{}{
				"progress":     1.0,
				"eta_relative": 0.0,
				"state":        map[string]interface{}{},
			},
			wantProgress: 1.0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/progress")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			progress, err := adapter.GetProgress(context.Background(), tt.skipCurrentImage)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantProgress, progress.Progress)
			}
		})
	}
}

func TestStableDiffusionAdapter_Interrupt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful_interrupt",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "interrupt_failure",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/sdapi/v1/interrupt", r.URL.Path)

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			err := adapter.Interrupt(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStableDiffusionAdapter_Skip(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/sdapi/v1/skip", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	err := adapter.Skip(context.Background())
	assert.NoError(t, err)
}

func TestStableDiffusionAdapter_GetModels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "successful_get",
			statusCode: http.StatusOK,
			response: []map[string]interface{}{
				{"title": "v1-5-pruned.safetensors", "model_name": "v1-5-pruned"},
				{"title": "sdxl.safetensors", "model_name": "sdxl"},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:       "get_failure",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"detail": "Error"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/sdapi/v1/sd-models", r.URL.Path)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			models, err := adapter.GetModels(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, models, tt.wantCount)
			}
		})
	}
}

func TestStableDiffusionAdapter_GetSamplers(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "Euler", "aliases": []string{"euler"}},
			{"name": "DPM++ 2M Karras", "aliases": []string{"dpmpp_2m_karras"}},
		})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	samplers, err := adapter.GetSamplers(context.Background())
	require.NoError(t, err)
	assert.Len(t, samplers, 2)
	assert.Equal(t, "Euler", samplers[0].Name)
}

func TestStableDiffusionAdapter_GetUpscalers(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "R-ESRGAN 4x+"},
			{"name": "LDSR"},
		})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	upscalers, err := adapter.GetUpscalers(context.Background())
	require.NoError(t, err)
	assert.Len(t, upscalers, 2)
}

func TestStableDiffusionAdapter_GetLoras(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "lora1", "path": "/models/lora/lora1.safetensors"},
		})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	loras, err := adapter.GetLoras(context.Background())
	require.NoError(t, err)
	assert.Len(t, loras, 1)
}

func TestStableDiffusionAdapter_GetOptions(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"sd_model_checkpoint":      "v1-5-pruned.safetensors",
			"CLIP_stop_at_last_layers": 1,
		})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	options, err := adapter.GetOptions(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, options.SDModelCheckpoint)
}

func TestStableDiffusionAdapter_SetOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		options    map[string]interface{}
		statusCode int
		wantErr    bool
	}{
		{
			name: "successful_set",
			options: map[string]interface{}{
				"sd_model_checkpoint": "sdxl.safetensors",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "set_failure",
			options:    map[string]interface{}{"invalid": "option"},
			statusCode: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/sdapi/v1/options", r.URL.Path)

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			err := adapter.SetOptions(context.Background(), tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStableDiffusionAdapter_RefreshModels(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/sdapi/v1/refresh-checkpoints", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	err := adapter.RefreshModels(context.Background())
	assert.NoError(t, err)
}

func TestStableDiffusionAdapter_PNGInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_extraction",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"info": "prompt: a cat, steps: 20",
			},
			wantErr: false,
		},
		{
			name:       "extraction_failure",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"detail": "Invalid image"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/sdapi/v1/png-info", r.URL.Path)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
				BaseURL: server.URL,
			})

			info, err := adapter.PNGInfo(context.Background(), "base64image")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, info)
			}
		})
	}
}

func TestStableDiffusionAdapter_DecodeImages(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})

	imageData := []byte("fake_image_data")
	encoded := base64.StdEncoding.EncodeToString(imageData)

	response := &SDGenerationResponse{
		Images: []string{encoded, encoded},
	}

	images, err := adapter.DecodeImages(response)
	require.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, imageData, images[0])
}

func TestStableDiffusionAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.connected)
}

func TestStableDiffusionAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{})

	tools := adapter.GetMCPTools()
	assert.NotEmpty(t, tools)
	assert.Equal(t, 11, len(tools))

	// Verify key tools
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "sd_txt2img")
	assert.Contains(t, toolNames, "sd_img2img")
	assert.Contains(t, toolNames, "sd_upscale")
	assert.Contains(t, toolNames, "sd_get_models")
	assert.Contains(t, toolNames, "sd_get_progress")
	assert.Contains(t, toolNames, "sd_interrupt")
}

func TestStableDiffusionAdapter_BasicAuth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", username)
		assert.Equal(t, "password123", password)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "Euler"}})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: "password123",
	})

	err := adapter.Connect(context.Background())
	assert.NoError(t, err)
}

func TestStableDiffusionAdapter_Txt2ImgDefaults(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Verify defaults are set
		assert.Equal(t, float64(20), body["steps"])
		assert.Equal(t, float64(7.0), body["cfg_scale"])
		assert.Equal(t, float64(512), body["width"])
		assert.Equal(t, float64(512), body["height"])
		assert.Equal(t, "Euler", body["sampler_name"])
		assert.Equal(t, float64(-1), body["seed"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"images":     []string{"image"},
			"parameters": map[string]interface{}{},
			"info":       "{}",
		})
	}))
	defer server.Close()

	adapter := NewStableDiffusionAdapter(StableDiffusionConfig{
		BaseURL: server.URL,
	})

	_, err := adapter.Txt2Img(context.Background(), &SDTxt2ImgRequest{
		Prompt: "test",
	})
	assert.NoError(t, err)
}
