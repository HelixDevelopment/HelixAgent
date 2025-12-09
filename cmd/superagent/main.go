package main

import (
	"time"

	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	llm "github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/middleware"
	"github.com/superagent/superagent/internal/models"
)

func main() {
	cfg := config.Load()
	r := gin.Default()

	// Create auth middleware
	authConfig := middleware.AuthConfig{
		SecretKey:   cfg.Server.JWTSecret,
		TokenExpiry: 24 * time.Hour,
		Issuer:      "superagent",
		SkipPaths:   []string{"/health", "/v1/health"},
		Required:    true,
	}
	auth := middleware.NewAuthMiddleware(authConfig)

	// Attach auth middleware to protected routes
	protected := r.Group("", auth.Middleware([]string{"/health", "/v1/health"}))

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Providers endpoints (stub MVP) - protected for mutation
	protected.GET("/v1/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{
			{"id": "prov-default", "name": "DefaultProvider", "type": "builtin", "enabled": true},
		})
	})
	protected.POST("/v1/providers", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"success": true, "message": "provider added (stub)"})
	})

	// Completions endpoint (ensemble-based)
	r.POST("/v1/completions", func(c *gin.Context) {
		var req models.LLMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		responses, selected, err := llm.RunEnsemble(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"responses": responses, "selected": selected})
	})

	// gRPC-like bridge endpoint (paralleled path to show gRPC integration readiness)
	r.POST("/grpc/llm/complete", func(c *gin.Context) {
		var req models.LLMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		responses, selected, err := llm.RunEnsemble(&req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"responses": responses, "selected": selected})
	})

	// Extra health/checks route for richer observability
	r.GET("/v1/health", func(c *gin.Context) {
		detailed := c.Query("detailed")
		_ = detailed
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "components": []string{"database", "providers"}})
	})

	// Metrics endpoint placeholder
	r.GET("/v1/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"llm_requests_total": 42, "provider_health": "healthy"})
	})

	_ = os.Setenv("SUPERAGENT_API_KEY", "dev-key-123")
	_ = r.Run(":8080")
}
