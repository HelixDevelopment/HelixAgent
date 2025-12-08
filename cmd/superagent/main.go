package main

import (
	"github.com/gin-gonic/gin"
	llm "github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
	"net/http"
)

func main() {
	r := gin.Default()

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Providers endpoints (stub MVP)
	r.GET("/v1/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, []gin.H{
			{"id": "prov-default", "name": "DefaultProvider", "type": "builtin", "enabled": true},
		})
	})
	r.POST("/v1/providers", func(c *gin.Context) {
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

	_ = r.Run(":8080")
}
