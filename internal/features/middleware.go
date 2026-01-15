// Package features provides HTTP middleware for feature flag detection.
// This middleware detects agent capabilities, parses feature headers/query params,
// and applies the appropriate feature configuration to each request.
package features

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MiddlewareConfig configures the feature middleware
type MiddlewareConfig struct {
	// Config is the base feature configuration
	Config *FeatureConfig

	// Logger for logging feature decisions
	Logger *logrus.Logger

	// EnableAgentDetection enables automatic agent detection from User-Agent
	EnableAgentDetection bool

	// StrictMode rejects requests with invalid feature combinations
	StrictMode bool

	// TrackUsage enables feature usage tracking
	TrackUsage bool
}

// DefaultMiddlewareConfig returns the default middleware configuration
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		Config:               DefaultFeatureConfig(),
		Logger:               logrus.New(),
		EnableAgentDetection: true,
		StrictMode:           false,
		TrackUsage:           true,
	}
}

// Middleware creates a Gin middleware for feature flag management
func Middleware(cfg *MiddlewareConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultMiddlewareConfig()
	}
	if cfg.Config == nil {
		cfg.Config = DefaultFeatureConfig()
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	return func(c *gin.Context) {
		// Create feature context from configuration
		fc := NewFeatureContextFromConfig(cfg.Config, c.Request.URL.Path)

		// Detect agent from User-Agent header
		if cfg.EnableAgentDetection {
			agentName := detectAgent(c.GetHeader("User-Agent"))
			if agentName != "" {
				fc.ApplyAgentCapabilities(agentName)
				cfg.Logger.WithFields(logrus.Fields{
					"agent":    agentName,
					"endpoint": c.Request.URL.Path,
				}).Debug("Detected agent, applied capabilities")
			}
		}

		// Parse and apply header-based feature overrides
		if cfg.Config.AllowFeatureHeaders {
			headerOverrides := parseFeatureHeaders(c.Request.Header)
			if len(headerOverrides) > 0 {
				fc.ApplyOverrides(headerOverrides, SourceHeaderOverride)
				cfg.Logger.WithFields(logrus.Fields{
					"overrides": len(headerOverrides),
					"endpoint":  c.Request.URL.Path,
				}).Debug("Applied header-based feature overrides")
			}
		}

		// Parse and apply query parameter overrides
		if cfg.Config.AllowFeatureQueryParams {
			queryOverrides := parseFeatureQueryParams(c.Request.URL.Query())
			if len(queryOverrides) > 0 {
				fc.ApplyOverrides(queryOverrides, SourceQueryOverride)
				cfg.Logger.WithFields(logrus.Fields{
					"overrides": len(queryOverrides),
					"endpoint":  c.Request.URL.Path,
				}).Debug("Applied query param feature overrides")
			}
		}

		// Validate feature combination if strict mode
		if cfg.StrictMode {
			if err := fc.Validate(); err != nil {
				cfg.Logger.WithError(err).Warn("Invalid feature combination")
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Invalid feature combination: " + err.Error(),
				})
				c.Abort()
				return
			}
		}

		// Track usage if enabled
		if cfg.TrackUsage {
			GetUsageTracker().RecordUsage(fc)
		}

		// Store feature context in Gin context
		c.Set("feature_context", fc)

		// Also store in request context for handlers that use context.Context
		ctx := WithFeatureContext(c.Request.Context(), fc)
		c.Request = c.Request.WithContext(ctx)

		// Set response headers indicating enabled features
		setFeatureResponseHeaders(c, fc)

		// Log feature decision
		if cfg.Logger.Level >= logrus.DebugLevel {
			cfg.Logger.WithFields(logrus.Fields{
				"enabled_features": fc.GetEnabledFeatures(),
				"source":           fc.Source,
				"agent":            fc.AgentName,
				"endpoint":         c.Request.URL.Path,
			}).Debug("Feature context established")
		}

		c.Next()
	}
}

// detectAgent attempts to identify the CLI agent from the User-Agent header
func detectAgent(userAgent string) string {
	if userAgent == "" {
		return ""
	}

	ua := strings.ToLower(userAgent)

	// Agent detection patterns
	agentPatterns := map[string][]string{
		"helixcode":     {"helixcode", "helix-code", "helix_code"},
		"opencode":      {"opencode", "open-code"},
		"crush":         {"crush"},
		"kiro":          {"kiro"},
		"aider":         {"aider"},
		"claudecode":    {"claude-code", "claudecode", "claude_code", "claude code", "anthropic-cli"},
		"cline":         {"cline"},
		"codenamegoose": {"goose", "codename-goose", "codenamegoose"},
		"deepseekcli":   {"deepseek", "deepseek-cli"},
		"forge":         {"forge"},
		"geminicli":     {"gemini", "gemini-cli"},
		"gptengineer":   {"gpt-engineer", "gptengineer"},
		"kilocode":      {"kilo", "kilo-code", "kilocode"},
		"mistralcode":   {"mistral", "mistral-code"},
		"ollamacode":    {"ollama", "ollama-code"},
		"plandex":       {"plandex"},
		"qwencode":      {"qwen", "qwen-code"},
		"amazonq":       {"amazon-q", "amazonq", "aws-q"},
	}

	for agent, patterns := range agentPatterns {
		for _, pattern := range patterns {
			if strings.Contains(ua, pattern) {
				return agent
			}
		}
	}

	return ""
}

// parseFeatureHeaders extracts feature toggles from HTTP headers
func parseFeatureHeaders(headers http.Header) map[Feature]bool {
	overrides := make(map[Feature]bool)
	registry := GetRegistry()

	for key, values := range headers {
		// Check if this is a feature header
		if feature, ok := registry.GetFeatureByHeader(key); ok {
			if len(values) > 0 {
				overrides[feature] = parseBoolValue(values[0])
			}
		}
	}

	// Also check for the compact X-Features header
	if features := headers.Get("X-Features"); features != "" {
		for _, part := range strings.Split(features, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// Parse feature=value or just feature (implies true)
			enabled := true
			name := part

			if strings.Contains(part, "=") {
				parts := strings.SplitN(part, "=", 2)
				name = strings.TrimSpace(parts[0])
				enabled = parseBoolValue(parts[1])
			} else if strings.HasPrefix(part, "!") || strings.HasPrefix(part, "-") {
				name = strings.TrimPrefix(strings.TrimPrefix(part, "!"), "-")
				enabled = false
			}

			feature := ParseFeature(name)
			if registry.IsValidFeature(feature) {
				overrides[feature] = enabled
			}
		}
	}

	return overrides
}

// parseFeatureQueryParams extracts feature toggles from query parameters
func parseFeatureQueryParams(query map[string][]string) map[Feature]bool {
	overrides := make(map[Feature]bool)
	registry := GetRegistry()

	for key, values := range query {
		// Check if this is a feature query param
		if feature, ok := registry.GetFeatureByQueryParam(key); ok {
			if len(values) > 0 {
				overrides[feature] = parseBoolValue(values[0])
			}
		}
	}

	// Also check for the compact features param
	if features, ok := query["features"]; ok && len(features) > 0 {
		for _, part := range strings.Split(features[0], ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			enabled := true
			name := part

			if strings.Contains(part, "=") {
				parts := strings.SplitN(part, "=", 2)
				name = strings.TrimSpace(parts[0])
				enabled = parseBoolValue(parts[1])
			} else if strings.HasPrefix(part, "!") || strings.HasPrefix(part, "-") {
				name = strings.TrimPrefix(strings.TrimPrefix(part, "!"), "-")
				enabled = false
			}

			feature := ParseFeature(name)
			if registry.IsValidFeature(feature) {
				overrides[feature] = enabled
			}
		}
	}

	return overrides
}

// parseBoolValue parses a string as a boolean
func parseBoolValue(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return true // Default to true if present but unclear
	}
}

// setFeatureResponseHeaders adds feature-related headers to the response
func setFeatureResponseHeaders(c *gin.Context, fc *FeatureContext) {
	// Add enabled features summary header
	enabled := fc.GetEnabledFeatures()
	if len(enabled) > 0 {
		var names []string
		for _, f := range enabled {
			names = append(names, string(f))
		}
		c.Header("X-Features-Enabled", strings.Join(names, ","))
	}

	// Add transport information
	c.Header("X-Transport-Protocol", fc.GetTransportProtocol())

	// Add compression information if enabled
	if compression := fc.GetCompressionMethod(); compression != "" {
		c.Header("X-Compression-Available", compression)
	}

	// Add streaming information
	c.Header("X-Streaming-Method", fc.GetStreamingMethod())

	// Add agent information if detected
	if fc.AgentName != "" {
		c.Header("X-Agent-Detected", fc.AgentName)
	}
}

// GetFeatureContextFromGin retrieves the FeatureContext from a Gin context
func GetFeatureContextFromGin(c *gin.Context) *FeatureContext {
	if fc, exists := c.Get("feature_context"); exists {
		if featureCtx, ok := fc.(*FeatureContext); ok {
			return featureCtx
		}
	}
	return NewFeatureContext()
}

// IsFeatureEnabled is a convenience function to check if a feature is enabled
func IsFeatureEnabled(c *gin.Context, feature Feature) bool {
	return GetFeatureContextFromGin(c).IsEnabled(feature)
}

// RequireFeature creates middleware that requires a specific feature
func RequireFeature(feature Feature) gin.HandlerFunc {
	return func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		if !fc.IsEnabled(feature) {
			c.JSON(http.StatusNotImplemented, gin.H{
				"error":   "Feature not enabled",
				"feature": string(feature),
				"message": "This endpoint requires the " + string(feature) + " feature to be enabled",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAnyFeature creates middleware that requires at least one of the specified features
func RequireAnyFeature(features ...Feature) gin.HandlerFunc {
	return func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		for _, feature := range features {
			if fc.IsEnabled(feature) {
				c.Next()
				return
			}
		}

		featureNames := make([]string, len(features))
		for i, f := range features {
			featureNames[i] = string(f)
		}

		c.JSON(http.StatusNotImplemented, gin.H{
			"error":    "Feature not enabled",
			"features": featureNames,
			"message":  "This endpoint requires at least one of the following features: " + strings.Join(featureNames, ", "),
		})
		c.Abort()
	}
}

// ConditionalMiddleware creates middleware that only runs if a feature is enabled
func ConditionalMiddleware(feature Feature, middleware gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		fc := GetFeatureContextFromGin(c)
		if fc.IsEnabled(feature) {
			middleware(c)
		} else {
			c.Next()
		}
	}
}
