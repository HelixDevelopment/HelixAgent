package features

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultMiddlewareConfig(t *testing.T) {
	config := DefaultMiddlewareConfig()
	require.NotNil(t, config)

	assert.NotNil(t, config.Config)
	assert.NotNil(t, config.Logger)
	assert.True(t, config.EnableAgentDetection)
	assert.False(t, config.StrictMode)
	assert.True(t, config.TrackUsage)
}

func TestMiddlewareBasic(t *testing.T) {
	router := gin.New()
	router.Use(Middleware(nil))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql_enabled": fc.IsEnabled(FeatureGraphQL),
			"sse_enabled":     fc.IsEnabled(FeatureSSE),
		})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should have X-Features-Enabled header
	assert.NotEmpty(t, w.Header().Get("X-Features-Enabled"))
}

func TestMiddlewareAgentDetection(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.EnableAgentDetection = true

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"agent": fc.AgentName,
		})
	})

	tests := []struct {
		userAgent     string
		expectedAgent string
	}{
		{"HelixCode/1.0", "helixcode"},
		{"OpenCode CLI", "opencode"},
		{"claude-code/1.0", "claudecode"},
		{"Aider/1.0", "aider"},
		{"Goose AI Assistant", "codenamegoose"},
		{"Unknown Agent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("User-Agent", tt.userAgent)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tt.expectedAgent != "" {
				assert.Equal(t, tt.expectedAgent, w.Header().Get("X-Agent-Detected"))
			}
		})
	}
}

func TestMiddlewareHeaderOverrides(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.Config.AllowFeatureHeaders = true

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
			"toon":    fc.IsEnabled(FeatureTOON),
		})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Feature-GraphQL", "true")
	req.Header.Set("X-Feature-TOON", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("X-Features-Enabled"), "graphql")
}

func TestMiddlewareCompactFeatureHeader(t *testing.T) {
	config := DefaultMiddlewareConfig()

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
			"toon":    fc.IsEnabled(FeatureTOON),
			"sse":     fc.IsEnabled(FeatureSSE),
		})
	})

	tests := []struct {
		name           string
		header         string
		graphqlEnabled bool
		sseEnabled     bool
	}{
		{
			name:           "enable_graphql",
			header:         "graphql=true,toon=true",
			graphqlEnabled: true,
		},
		{
			name:           "disable_sse",
			header:         "!sse",
			sseEnabled:     false,
		},
		{
			name:           "mixed",
			header:         "graphql,-sse",
			graphqlEnabled: true,
			sseEnabled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Features", tt.header)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMiddlewareQueryParamOverrides(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.Config.AllowFeatureQueryParams = true

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
		})
	})

	req, _ := http.NewRequest("GET", "/test?graphql=true&toon=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("X-Features-Enabled"), "graphql")
}

func TestMiddlewareCompactQueryParam(t *testing.T) {
	config := DefaultMiddlewareConfig()

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
		})
	})

	req, _ := http.NewRequest("GET", "/test?features=graphql,toon,-sse", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddlewareStrictMode(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.StrictMode = true

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Invalid combination: HTTP2 and HTTP3 together
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Features", "http2=true,http3=true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid feature combination")
}

func TestMiddlewareResponseHeaders(t *testing.T) {
	router := gin.New()
	router.Use(Middleware(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Features-Enabled"))
	assert.NotEmpty(t, w.Header().Get("X-Transport-Protocol"))
	assert.NotEmpty(t, w.Header().Get("X-Streaming-Method"))
}

func TestDetectAgent(t *testing.T) {
	tests := []struct {
		userAgent string
		expected  string
	}{
		{"HelixCode/1.0", "helixcode"},
		{"helix-code CLI", "helixcode"},
		{"OpenCode CLI 1.0", "opencode"},
		{"Crush Terminal AI", "crush"},
		{"Kiro Agent", "kiro"},
		{"Aider 0.50.1", "aider"},
		{"claude-code/1.0", "claudecode"},
		{"Claude Code CLI", "claudecode"},
		{"anthropic-cli/1.0", "claudecode"},
		{"Cline Extension", "cline"},
		{"goose CLI", "codenamegoose"},
		{"Codename-Goose", "codenamegoose"},
		{"DeepSeek CLI", "deepseekcli"},
		{"deepseek-cli/1.0", "deepseekcli"},
		{"Forge Agent", "forge"},
		{"Gemini CLI", "geminicli"},
		{"gpt-engineer/1.0", "gptengineer"},
		{"Kilo Code", "kilocode"},
		{"kilo-code/1.0", "kilocode"},
		{"Mistral Code", "mistralcode"},
		{"mistral-code CLI", "mistralcode"},
		{"Ollama Code", "ollamacode"},
		{"ollama-code/1.0", "ollamacode"},
		{"Plandex CLI", "plandex"},
		{"Qwen Code", "qwencode"},
		{"qwen-code/1.0", "qwencode"},
		{"amazon-q CLI", "amazonq"},
		{"AmazonQ Developer", "amazonq"},
		{"aws-q/1.0", "amazonq"},
		{"Unknown Browser", ""},
		{"curl/7.68.0", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			result := detectAgent(tt.userAgent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseFeatureHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected map[Feature]bool
	}{
		{
			name: "individual_headers",
			headers: http.Header{
				"X-Feature-GraphQL": []string{"true"},
				"X-Feature-TOON":    []string{"false"},
			},
			expected: map[Feature]bool{
				FeatureGraphQL: true,
				FeatureTOON:    false,
			},
		},
		{
			name: "compact_header",
			headers: http.Header{
				"X-Features": []string{"graphql,toon=true,-sse"},
			},
			expected: map[Feature]bool{
				FeatureGraphQL: true,
				FeatureTOON:    true,
				FeatureSSE:     false,
			},
		},
		{
			name: "mixed_headers",
			headers: http.Header{
				"X-Feature-GraphQL": []string{"true"},
				"X-Features":        []string{"toon"},
			},
			expected: map[Feature]bool{
				FeatureGraphQL: true,
				FeatureTOON:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFeatureHeaders(tt.headers)
			for feature, expected := range tt.expected {
				assert.Equal(t, expected, result[feature], "feature %s mismatch", feature)
			}
		})
	}
}

func TestParseFeatureQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		query    map[string][]string
		expected map[Feature]bool
	}{
		{
			name: "individual_params",
			query: map[string][]string{
				"graphql": {"true"},
				"toon":    {"false"},
			},
			expected: map[Feature]bool{
				FeatureGraphQL: true,
				FeatureTOON:    false,
			},
		},
		{
			name: "compact_param",
			query: map[string][]string{
				"features": {"graphql,toon=true,-sse"},
			},
			expected: map[Feature]bool{
				FeatureGraphQL: true,
				FeatureTOON:    true,
				FeatureSSE:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFeatureQueryParams(tt.query)
			for feature, expected := range tt.expected {
				assert.Equal(t, expected, result[feature], "feature %s mismatch", feature)
			}
		})
	}
}

func TestParseBoolValue(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"enabled", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"disabled", false},
		{"", true},         // Empty defaults to true
		{"invalid", true},  // Unknown defaults to true
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseBoolValue(tt.input))
		})
	}
}

func TestGetFeatureContextFromGin(t *testing.T) {
	router := gin.New()
	router.Use(Middleware(nil))

	var capturedFC *FeatureContext
	router.GET("/test", func(c *gin.Context) {
		capturedFC = GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotNil(t, capturedFC)
	assert.NotNil(t, capturedFC.Features)
}

func TestGetFeatureContextFromGinWithoutMiddleware(t *testing.T) {
	router := gin.New()

	var capturedFC *FeatureContext
	router.GET("/test", func(c *gin.Context) {
		capturedFC = GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return default context
	assert.NotNil(t, capturedFC)
	assert.NotNil(t, capturedFC.Features)
}

func TestIsFeatureEnabled(t *testing.T) {
	router := gin.New()
	router.Use(Middleware(nil))

	var graphqlEnabled, sseEnabled bool
	router.GET("/test", func(c *gin.Context) {
		graphqlEnabled = IsFeatureEnabled(c, FeatureGraphQL)
		sseEnabled = IsFeatureEnabled(c, FeatureSSE)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Feature-GraphQL", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, graphqlEnabled)
	assert.True(t, sseEnabled) // Default enabled
}

func TestRequireFeature(t *testing.T) {
	config := DefaultMiddlewareConfig()

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/graphql", RequireFeature(FeatureGraphQL), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Without feature enabled
	req1, _ := http.NewRequest("GET", "/graphql", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusNotImplemented, w1.Code)

	// With feature enabled
	req2, _ := http.NewRequest("GET", "/graphql", nil)
	req2.Header.Set("X-Feature-GraphQL", "true")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRequireAnyFeature(t *testing.T) {
	router := gin.New()
	router.Use(Middleware(nil))
	router.GET("/stream", RequireAnyFeature(FeatureWebSocket, FeatureSSE), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// SSE is enabled by default
	req, _ := http.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Disable all streaming
	req2, _ := http.NewRequest("GET", "/stream?websocket=false&sse=false", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotImplemented, w2.Code)
}

func TestConditionalMiddleware(t *testing.T) {
	calledCount := 0
	countingMiddleware := func(c *gin.Context) {
		calledCount++
		c.Next()
	}

	router := gin.New()
	router.Use(Middleware(nil))
	router.Use(ConditionalMiddleware(FeatureGraphQL, countingMiddleware))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Without GraphQL
	calledCount = 0
	req1, _ := http.NewRequest("GET", "/test", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, 0, calledCount)

	// With GraphQL
	calledCount = 0
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Feature-GraphQL", "true")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 1, calledCount)
}

func TestMiddlewareWithCustomLogger(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	config := &MiddlewareConfig{
		Config:               DefaultFeatureConfig(),
		Logger:               logger,
		EnableAgentDetection: true,
		TrackUsage:           true,
	}

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "HelixCode/1.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddlewareDisabledFeatureHeaders(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.Config.AllowFeatureHeaders = false

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
		})
	})

	// Header should be ignored
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Feature-GraphQL", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// GraphQL should still be disabled (header was ignored)
	assert.NotContains(t, w.Header().Get("X-Features-Enabled"), "graphql")
}

func TestMiddlewareDisabledQueryParams(t *testing.T) {
	config := DefaultMiddlewareConfig()
	config.Config.AllowFeatureQueryParams = false

	router := gin.New()
	router.Use(Middleware(config))
	router.GET("/test", func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		c.JSON(http.StatusOK, gin.H{
			"graphql": fc.IsEnabled(FeatureGraphQL),
		})
	})

	// Query param should be ignored
	req, _ := http.NewRequest("GET", "/test?graphql=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// GraphQL should still be disabled (query param was ignored)
	assert.NotContains(t, w.Header().Get("X-Features-Enabled"), "graphql")
}
