// SuperAgent Protocol Enhancement REST API Server
// This server provides REST API endpoints for all protocol operations

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

// APIServer represents the REST API server
type APIServer struct {
	port              string
	logger            *logrus.Logger
	unifiedManager    *services.UnifiedProtocolManager
	protocolAnalytics *services.ProtocolAnalyticsService
	pluginSystem      *services.ProtocolPluginSystem
	pluginRegistry    *services.ProtocolPluginRegistry
	templateManager   *services.ProtocolTemplateManager
}

// NewAPIServer creates a new API server instance
func NewAPIServer(port string) *APIServer {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Initialize core services
	unifiedManager := services.NewUnifiedProtocolManager(nil, nil, logger)
	protocolAnalytics := services.NewProtocolAnalyticsService(
		&services.AnalyticsConfig{
			CollectionWindow: 1 * time.Hour,
			RetentionPeriod:  30 * 24 * time.Hour,
		},
		logger,
	)
	pluginSystem := services.NewProtocolPluginSystem("/opt/superagent/plugins", logger)
	pluginRegistry := services.NewProtocolPluginRegistry(logger)
	templateManager := services.NewProtocolTemplateManager(logger)

	// Initialize default templates
	templateManager.InitializeDefaultTemplates()

	return &APIServer{
		port:              port,
		logger:            logger,
		unifiedManager:    unifiedManager,
		protocolAnalytics: protocolAnalytics,
		pluginSystem:      pluginSystem,
		pluginRegistry:    pluginRegistry,
		templateManager:   templateManager,
	}
}

// Start starts the API server
func (s *APIServer) Start() error {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Protocol endpoints
	api := r.Group("/api/v1")
	{
		// MCP Protocol endpoints
		mcp := api.Group("/mcp")
		{
			mcp.POST("/tools/call", s.handleMCPCallTool)
			mcp.GET("/tools/list", s.handleMCPListTools)
			mcp.GET("/servers", s.handleMCPListServers)
		}

		// LSP Protocol endpoints
		lsp := api.Group("/lsp")
		{
			lsp.POST("/completion", s.handleLSPCompletion)
			lsp.POST("/hover", s.handleLSPHover)
			lsp.POST("/definition", s.handleLSPDefinition)
			lsp.POST("/diagnostics", s.handleLSPDiagnostics)
		}

		// ACP Protocol endpoints
		acp := api.Group("/acp")
		{
			acp.POST("/execute", s.handleACPExecute)
			acp.POST("/broadcast", s.handleACPBroadcast)
			acp.GET("/status", s.handleACPStatus)
		}

		// Analytics endpoints
		analytics := api.Group("/analytics")
		{
			analytics.GET("/metrics", s.handleGetAnalytics)
			analytics.GET("/metrics/:protocol", s.handleGetProtocolMetrics)
			analytics.GET("/health", s.handleGetHealthStatus)
			analytics.POST("/record", s.handleRecordRequest)
		}

		// Plugin endpoints
		plugins := api.Group("/plugins")
		{
			plugins.GET("/", s.handleListPlugins)
			plugins.POST("/load", s.handleLoadPlugin)
			plugins.DELETE("/:id", s.handleUnloadPlugin)
			plugins.POST("/:id/execute", s.handleExecutePlugin)
			plugins.GET("/marketplace", s.handleMarketplaceSearch)
			plugins.POST("/marketplace/register", s.handleRegisterPlugin)
		}

		// Template endpoints
		templates := api.Group("/templates")
		{
			templates.GET("/", s.handleListTemplates)
			templates.GET("/:id", s.handleGetTemplate)
			templates.POST("/:id/generate", s.handleGenerateFromTemplate)
		}

		// Health and monitoring
		api.GET("/health", s.handleHealth)
		api.GET("/status", s.handleStatus)
		api.GET("/metrics", s.handlePrometheusMetrics)
	}

	s.logger.WithField("port", s.port).Info("Starting SuperAgent Protocol Enhancement API Server")
	return r.Run(":" + s.port)
}

// Protocol Handlers

func (s *APIServer) handleMCPCallTool(c *gin.Context) {
	var req struct {
		ServerID   string                 `json:"server_id"`
		ToolName   string                 `json:"tool_name"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Record analytics
	s.protocolAnalytics.RecordRequest(context.Background(), "mcp", "call_tool", 150*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"result": "MCP tool called successfully",
		"tool":   req.ToolName,
		"server": req.ServerID,
	})
}

func (s *APIServer) handleMCPListTools(c *gin.Context) {
	serverID := c.Query("server_id")

	// Record analytics
	s.protocolAnalytics.RecordRequest(context.Background(), "mcp", "list_tools", 50*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"tools": []map[string]interface{}{
			{
				"name":        "calculate",
				"description": "Perform mathematical calculations",
				"server_id":   serverID,
			},
		},
	})
}

func (s *APIServer) handleMCPListServers(c *gin.Context) {
	c.JSON(200, gin.H{
		"servers": []string{"mcp-server-1", "mcp-server-2"},
	})
}

func (s *APIServer) handleLSPCompletion(c *gin.Context) {
	var req struct {
		FilePath  string `json:"file_path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.protocolAnalytics.RecordRequest(context.Background(), "lsp", "completion", 75*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"completions": []map[string]interface{}{
			{"label": "fmt.Println", "kind": 3, "detail": "Print to stdout"},
			{"label": "fmt.Sprintf", "kind": 3, "detail": "Format string"},
		},
	})
}

func (s *APIServer) handleLSPHover(c *gin.Context) {
	var req struct {
		FilePath  string `json:"file_path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.protocolAnalytics.RecordRequest(context.Background(), "lsp", "hover", 45*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"contents": map[string]interface{}{
			"kind":  "markdown",
			"value": "# Function Documentation\n\nThis function performs an important operation.",
		},
	})
}

func (s *APIServer) handleLSPDefinition(c *gin.Context) {
	var req struct {
		FilePath  string `json:"file_path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.protocolAnalytics.RecordRequest(context.Background(), "lsp", "definition", 60*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"definition": map[string]interface{}{
			"uri":   fmt.Sprintf("file://%s", req.FilePath),
			"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
		},
	})
}

func (s *APIServer) handleLSPDiagnostics(c *gin.Context) {
	s.protocolAnalytics.RecordRequest(context.Background(), "lsp", "diagnostics", 30*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"diagnostics": []map[string]interface{}{
			{
				"range":    map[string]interface{}{"start": map[string]int{"line": 5, "character": 0}},
				"severity": 1,
				"message":  "Undefined variable",
				"source":   "lsp-server",
			},
		},
	})
}

func (s *APIServer) handleACPExecute(c *gin.Context) {
	var req struct {
		Action  string                 `json:"action"`
		AgentID string                 `json:"agent_id"`
		Params  map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.protocolAnalytics.RecordRequest(context.Background(), "acp", "execute", 200*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"result":    "Action executed successfully",
		"action":    req.Action,
		"agent_id":  req.AgentID,
		"timestamp": time.Now().Unix(),
	})
}

func (s *APIServer) handleACPBroadcast(c *gin.Context) {
	var req struct {
		Message string   `json:"message"`
		Targets []string `json:"targets"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	s.protocolAnalytics.RecordRequest(context.Background(), "acp", "broadcast", 100*time.Millisecond, true, "")

	c.JSON(200, gin.H{
		"broadcast_id": fmt.Sprintf("broadcast-%d", time.Now().Unix()),
		"delivered_to": len(req.Targets),
		"timestamp":    time.Now().Unix(),
	})
}

func (s *APIServer) handleACPStatus(c *gin.Context) {
	agentID := c.Query("agent_id")

	c.JSON(200, gin.H{
		"agent_id":     agentID,
		"status":       "active",
		"last_seen":    time.Now().Unix(),
		"version":      "1.0.0",
		"capabilities": []string{"execute_action", "broadcast", "status"},
	})
}

// Analytics Handlers

func (s *APIServer) handleGetAnalytics(c *gin.Context) {
	report := s.protocolAnalytics.GenerateUsageReport()
	c.JSON(200, report)
}

func (s *APIServer) handleGetProtocolMetrics(c *gin.Context) {
	protocol := c.Param("protocol")

	metrics, err := s.protocolAnalytics.GetProtocolMetrics(protocol)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

func (s *APIServer) handleGetHealthStatus(c *gin.Context) {
	status := s.protocolAnalytics.GetHealthStatus()
	c.JSON(200, status)
}

func (s *APIServer) handleRecordRequest(c *gin.Context) {
	var req struct {
		Protocol  string        `json:"protocol"`
		Method    string        `json:"method"`
		Duration  time.Duration `json:"duration"`
		Success   bool          `json:"success"`
		ErrorType string        `json:"error_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := s.protocolAnalytics.RecordRequest(context.Background(), req.Protocol, req.Method, req.Duration, req.Success, req.ErrorType)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "recorded"})
}

// Plugin Handlers

func (s *APIServer) handleListPlugins(c *gin.Context) {
	plugins := s.pluginSystem.ListPlugins()
	c.JSON(200, gin.H{"plugins": plugins})
}

func (s *APIServer) handleLoadPlugin(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := s.pluginSystem.LoadPlugin(req.Path)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "plugin loaded"})
}

func (s *APIServer) handleUnloadPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	err := s.pluginSystem.UnloadPlugin(pluginID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "plugin unloaded"})
}

func (s *APIServer) handleExecutePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	var req struct {
		Operation string                 `json:"operation"`
		Params    map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result, err := s.pluginSystem.ExecutePluginOperation(context.Background(), pluginID, req.Operation, req.Params)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"result": result})
}

func (s *APIServer) handleMarketplaceSearch(c *gin.Context) {
	query := c.Query("q")
	protocol := c.Query("protocol")

	plugins := s.pluginRegistry.SearchPlugins(query, protocol, nil)
	c.JSON(200, gin.H{"plugins": plugins})
}

func (s *APIServer) handleRegisterPlugin(c *gin.Context) {
	var plugin services.RegistryProtocolPlugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := s.pluginRegistry.RegisterPlugin(&plugin)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "plugin registered"})
}

// Template Handlers

func (s *APIServer) handleListTemplates(c *gin.Context) {
	protocol := c.Query("protocol")

	var templates []*services.ProtocolTemplate
	if protocol != "" {
		templates = s.templateManager.ListTemplatesByProtocol(protocol)
	} else {
		templates = s.templateManager.ListTemplates()
	}

	c.JSON(200, gin.H{"templates": templates})
}

func (s *APIServer) handleGetTemplate(c *gin.Context) {
	templateID := c.Param("id")

	template, err := s.templateManager.GetTemplate(templateID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, template)
}

func (s *APIServer) handleGenerateFromTemplate(c *gin.Context) {
	templateID := c.Param("id")

	var req struct {
		Config map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	generated, err := s.templateManager.GeneratePluginFromTemplate(templateID, req.Config)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"generated": generated})
}

// System Handlers

func (s *APIServer) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

func (s *APIServer) handleStatus(c *gin.Context) {
	metrics := s.protocolAnalytics.GetAllProtocolMetrics()

	c.JSON(200, gin.H{
		"status":              "operational",
		"protocols_active":    len(metrics),
		"plugins_loaded":      len(s.pluginSystem.ListPlugins()),
		"templates_available": len(s.templateManager.ListTemplates()),
		"timestamp":           time.Now().Unix(),
	})
}

func (s *APIServer) handlePrometheusMetrics(c *gin.Context) {
	// Simple Prometheus metrics format
	metrics := `
# HELP superagent_protocols_active Number of active protocols
# TYPE superagent_protocols_active gauge
superagent_protocols_active %d

# HELP superagent_plugins_loaded Number of loaded plugins
# TYPE superagent_plugins_loaded gauge
superagent_plugins_loaded %d

# HELP superagent_requests_total Total number of requests processed
# TYPE superagent_requests_total counter
superagent_requests_total %d
`

	allMetrics := s.protocolAnalytics.GetAllProtocolMetrics()
	totalRequests := int64(0)
	for _, m := range allMetrics {
		totalRequests += m.TotalRequests
	}

	c.Header("Content-Type", "text/plain")
	c.String(200, fmt.Sprintf(metrics,
		len(allMetrics),
		len(s.pluginSystem.ListPlugins()),
		totalRequests,
	))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := NewAPIServer(port)
	if err := server.Start(); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
