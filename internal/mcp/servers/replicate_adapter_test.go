package servers

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

func TestNewReplicateAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config ReplicateConfig
		want   struct {
			baseURL string
			timeout time.Duration
		}
	}{
		{
			name: "with_custom_config",
			config: ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  "https://custom.replicate.com/v1",
				Timeout:  120 * time.Second,
			},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "https://custom.replicate.com/v1",
				timeout: 120 * time.Second,
			},
		},
		{
			name: "with_default_values",
			config: ReplicateConfig{
				APIToken: "test-token",
			},
			want: struct {
				baseURL string
				timeout time.Duration
			}{
				baseURL: "https://api.replicate.com/v1",
				timeout: 60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			adapter := NewReplicateAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.want.baseURL, adapter.baseURL)
			assert.Equal(t, tt.want.timeout, adapter.client.Timeout)
		})
	}
}

func TestReplicateAdapter_Connect(t *testing.T) {
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
			response: map[string]interface{}{
				"type":     "user",
				"username": "testuser",
			},
			wantErr: false,
		},
		{
			name:        "authentication_failure",
			statusCode:  http.StatusUnauthorized,
			response:    map[string]string{"detail": "Invalid token"},
			wantErr:     true,
			errContains: "authentication failed",
		},
		{
			name:        "server_error",
			statusCode:  http.StatusInternalServerError,
			response:    map[string]string{"detail": "Internal error"},
			wantErr:     true,
			errContains: "failed to authenticate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/account", r.URL.Path)
				assert.Equal(t, "Token test-token", r.Header.Get("Authorization"))

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
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

func TestReplicateAdapter_Health(t *testing.T) {
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
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
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

func TestReplicateAdapter_GetModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		owner       string
		modelName   string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_get",
			owner:      "stability-ai",
			modelName:  "sdxl",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"url":         "https://replicate.com/stability-ai/sdxl",
				"owner":       "stability-ai",
				"name":        "sdxl",
				"description": "Stable Diffusion XL",
				"run_count":   1000000,
			},
			wantErr: false,
		},
		{
			name:        "model_not_found",
			owner:       "unknown",
			modelName:   "model",
			statusCode:  http.StatusNotFound,
			response:    map[string]string{"detail": "Not found"},
			wantErr:     true,
			errContains: "model not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/models/"+tt.owner+"/"+tt.modelName)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			model, err := adapter.GetModel(context.Background(), tt.owner, tt.modelName)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.owner, model.Owner)
				assert.Equal(t, tt.modelName, model.Name)
			}
		})
	}
}

func TestReplicateAdapter_GetModelVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		owner      string
		modelName  string
		versionID  string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful_get",
			owner:      "stability-ai",
			modelName:  "sdxl",
			versionID:  "version123",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":          "version123",
				"cog_version": "0.8.0",
			},
			wantErr: false,
		},
		{
			name:       "version_not_found",
			owner:      "stability-ai",
			modelName:  "sdxl",
			versionID:  "nonexistent",
			statusCode: http.StatusNotFound,
			response:   map[string]string{"detail": "Not found"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			version, err := adapter.GetModelVersion(context.Background(), tt.owner, tt.modelName, tt.versionID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.versionID, version.ID)
			}
		})
	}
}

func TestReplicateAdapter_ListModels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		cursor     string
		statusCode int
		response   interface{}
		wantCount  int
		wantNext   string
		wantErr    bool
	}{
		{
			name:       "successful_list",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"results": []map[string]interface{}{
					{"owner": "stability-ai", "name": "sdxl"},
					{"owner": "meta", "name": "llama"},
				},
				"next": "cursor123",
			},
			wantCount: 2,
			wantNext:  "cursor123",
			wantErr:   false,
		},
		{
			name:       "with_cursor",
			cursor:     "prev_cursor",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"results": []map[string]interface{}{
					{"owner": "openai", "name": "whisper"},
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:       "list_failure",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"detail": "Forbidden"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/models")

				if tt.cursor != "" {
					assert.Equal(t, tt.cursor, r.URL.Query().Get("cursor"))
				}

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			models, next, err := adapter.ListModels(context.Background(), tt.cursor)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, models, tt.wantCount)
				assert.Equal(t, tt.wantNext, next)
			}
		})
	}
}

func TestReplicateAdapter_CreatePrediction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		version    string
		input      map[string]interface{}
		webhook    *ReplicateWebhook
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:    "successful_create",
			version: "version123",
			input: map[string]interface{}{
				"prompt": "a beautiful sunset",
			},
			statusCode: http.StatusCreated,
			response: map[string]interface{}{
				"id":      "prediction123",
				"version": "version123",
				"status":  "starting",
			},
			wantErr: false,
		},
		{
			name:    "create_with_webhook",
			version: "version123",
			input: map[string]interface{}{
				"prompt": "a cat",
			},
			webhook: &ReplicateWebhook{
				URL:    "https://example.com/webhook",
				Events: []string{"completed"},
			},
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":     "prediction456",
				"status": "starting",
			},
			wantErr: false,
		},
		{
			name:       "create_failure",
			version:    "invalid",
			input:      map[string]interface{}{},
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"detail": "Invalid version"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/predictions", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, tt.version, body["version"])

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			prediction, err := adapter.CreatePrediction(context.Background(), tt.version, tt.input, tt.webhook)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, prediction.ID)
			}
		})
	}
}

func TestReplicateAdapter_GetPrediction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		predictionID string
		statusCode   int
		response     interface{}
		wantErr      bool
		errContains  string
	}{
		{
			name:         "successful_get",
			predictionID: "prediction123",
			statusCode:   http.StatusOK,
			response: map[string]interface{}{
				"id":     "prediction123",
				"status": "succeeded",
				"output": []string{"https://example.com/image.png"},
			},
			wantErr: false,
		},
		{
			name:         "prediction_not_found",
			predictionID: "nonexistent",
			statusCode:   http.StatusNotFound,
			response:     map[string]string{"detail": "Not found"},
			wantErr:      true,
			errContains:  "prediction not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/predictions/"+tt.predictionID)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			prediction, err := adapter.GetPrediction(context.Background(), tt.predictionID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.predictionID, prediction.ID)
			}
		})
	}
}

func TestReplicateAdapter_CancelPrediction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		predictionID string
		statusCode   int
		response     interface{}
		wantErr      bool
	}{
		{
			name:         "successful_cancel",
			predictionID: "prediction123",
			statusCode:   http.StatusOK,
			response: map[string]interface{}{
				"id":     "prediction123",
				"status": "canceled",
			},
			wantErr: false,
		},
		{
			name:         "cancel_failure",
			predictionID: "completed_prediction",
			statusCode:   http.StatusBadRequest,
			response:     map[string]string{"detail": "Cannot cancel completed prediction"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/cancel")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			prediction, err := adapter.CancelPrediction(context.Background(), tt.predictionID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "canceled", prediction.Status)
			}
		})
	}
}

func TestReplicateAdapter_ListPredictions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		cursor     string
		statusCode int
		response   interface{}
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "successful_list",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": "pred1", "status": "succeeded"},
					{"id": "pred2", "status": "processing"},
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:       "list_failure",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"detail": "Server error"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			predictions, _, err := adapter.ListPredictions(context.Background(), tt.cursor)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, predictions, tt.wantCount)
			}
		})
	}
}

func TestReplicateAdapter_WaitForPrediction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		predictionID string
		statuses     []string
		wantStatus   string
		wantErr      bool
	}{
		{
			name:         "immediate_success",
			predictionID: "pred1",
			statuses:     []string{"succeeded"},
			wantStatus:   "succeeded",
			wantErr:      false,
		},
		{
			name:         "poll_until_success",
			predictionID: "pred2",
			statuses:     []string{"starting", "processing", "succeeded"},
			wantStatus:   "succeeded",
			wantErr:      false,
		},
		{
			name:         "prediction_failed",
			predictionID: "pred3",
			statuses:     []string{"starting", "failed"},
			wantStatus:   "failed",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := tt.statuses[callCount]
				if callCount < len(tt.statuses)-1 {
					callCount++
				}

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id":     tt.predictionID,
					"status": status,
				})
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			prediction, err := adapter.WaitForPrediction(ctx, tt.predictionID, 10*time.Millisecond)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStatus, prediction.Status)
			}
		})
	}
}

func TestReplicateAdapter_GetCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		slug        string
		statusCode  int
		response    interface{}
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful_get",
			slug:       "text-to-image",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"name":        "Text to Image",
				"slug":        "text-to-image",
				"description": "Generate images from text",
			},
			wantErr: false,
		},
		{
			name:        "collection_not_found",
			slug:        "nonexistent",
			statusCode:  http.StatusNotFound,
			response:    map[string]string{"detail": "Not found"},
			wantErr:     true,
			errContains: "collection not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/collections/"+tt.slug)

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			collection, err := adapter.GetCollection(context.Background(), tt.slug)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.slug, collection.Slug)
			}
		})
	}
}

func TestReplicateAdapter_ListCollections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "successful_list",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"results": []map[string]interface{}{
					{"slug": "text-to-image", "name": "Text to Image"},
					{"slug": "image-to-image", "name": "Image to Image"},
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter := NewReplicateAdapter(ReplicateConfig{
				APIToken: "test-token",
				BaseURL:  server.URL,
			})

			collections, _, err := adapter.ListCollections(context.Background(), "")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, collections, tt.wantCount)
			}
		})
	}
}

func TestReplicateAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewReplicateAdapter(ReplicateConfig{
		APIToken: "test-token",
	})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.connected)
}

func TestReplicateAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewReplicateAdapter(ReplicateConfig{
		APIToken: "test-token",
	})

	tools := adapter.GetMCPTools()
	assert.NotEmpty(t, tools)
	assert.Equal(t, 10, len(tools))

	// Verify tool names
	expectedTools := []string{
		"replicate_get_model",
		"replicate_list_models",
		"replicate_create_prediction",
		"replicate_get_prediction",
		"replicate_cancel_prediction",
		"replicate_list_predictions",
		"replicate_run_model",
		"replicate_generate_image",
		"replicate_get_collection",
		"replicate_list_collections",
	}

	for i, expected := range expectedTools {
		assert.Equal(t, expected, tools[i].Name)
	}
}

func TestReplicateAdapter_AuthHeader(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Token auth
		assert.Equal(t, "Token test-api-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"type": "user"})
	}))
	defer server.Close()

	adapter := NewReplicateAdapter(ReplicateConfig{
		APIToken: "test-api-token",
		BaseURL:  server.URL,
	})

	err := adapter.Connect(context.Background())
	assert.NoError(t, err)
}
