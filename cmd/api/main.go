// ===========================================================================
// DEMO IMPLEMENTATION - NOT FOR PRODUCTION USE
// ===========================================================================
//
// HelixAgent Protocol Enhancement REST API Server (DEMO)
//
// This is a DEMONSTRATION server that showcases the protocol API structure.
// It returns HARDCODED/MOCK responses and does NOT connect to real backends.
//
// For production use, see:
//   - cmd/helixagent/main.go - Main production entry point
//   - internal/router/router.go - Production API router with real implementations
//
// This demo server is useful for:
//   - API structure exploration
//   - Client development and testing
//   - Documentation examples
//
// DO NOT deploy this server in production environments.
// ===========================================================================

package main

import (
	"fmt"
	"os"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// analyticsStore tracks protocol request metrics in-memory.
// Concurrent-safe by construction (CONST-029): requests is a safe.Slice.
type analyticsStore struct {
	requests *safe.Slice[analyticsRecord]
}

type analyticsRecord struct {
	Protocol  string  `json:"protocol"`
	Method    string  `json:"method"`
	Duration  float64 `json:"duration"`
	Success   bool    `json:"success"`
	ErrorType string  `json:"error_type"`
}

func newAnalyticsStore() *analyticsStore {
	return &analyticsStore{requests: safe.NewSlice[analyticsRecord]()}
}

func (a *analyticsStore) record(r analyticsRecord) {
	a.requests.Append(r)
}

func (a *analyticsStore) allMetrics() map[string]interface{} {
	snapshot := a.requests.Snapshot()
	total := len(snapshot)
	success := 0
	for _, r := range snapshot {
		if r.Success {
			success++
		}
	}
	return map[string]interface{}{
		"total_requests":    total,
		"successful":        success,
		"failed":            total - success,
		"protocols_tracked": protocolList(snapshot),
	}
}

// protocolList extracts the distinct non-empty protocols from a
// pre-taken snapshot. Taking a slice argument (vs. reading a.requests)
// keeps the single-snapshot consistency guarantee of allMetrics.
func protocolList(snapshot []analyticsRecord) []string {
	seen := map[string]bool{}
	var list []string
	for _, r := range snapshot {
		if r.Protocol != "" && !seen[r.Protocol] {
			seen[r.Protocol] = true
			list = append(list, r.Protocol)
		}
	}
	return list
}

func (a *analyticsStore) metricsForProtocol(
	protocol string,
) (map[string]interface{}, bool) {
	total := 0
	success := 0
	for _, r := range a.requests.Snapshot() {
		if r.Protocol == protocol {
			total++
			if r.Success {
				success++
			}
		}
	}
	if total == 0 {
		return nil, false
	}
	return map[string]interface{}{
		"protocol":       protocol,
		"total_requests": total,
		"successful":     success,
		"failed":         total - success,
	}, true
}

// integrationTemplate represents a reusable protocol integration template
type integrationTemplate struct {
	ID          string   `json:"ID"`
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol"`
	Description string   `json:"description"`
	Protocols   []string `json:"protocols"`
}

var defaultTemplates = []integrationTemplate{
	{
		ID:          "mcp-basic-integration",
		Name:        "MCP Basic Integration",
		Protocol:    "mcp",
		Description: "Basic MCP tool integration",
		Protocols:   []string{"mcp"},
	},
	{
		ID:          "lsp-code-navigation",
		Name:        "LSP Code Navigation",
		Protocol:    "lsp",
		Description: "LSP-based code navigation",
		Protocols:   []string{"lsp"},
	},
	{
		ID:          "acp-agent-communication",
		Name:        "ACP Agent Communication",
		Protocol:    "acp",
		Description: "ACP agent-to-agent communication",
		Protocols:   []string{"acp"},
	},
}

// APIServer represents the REST API server
type APIServer struct {
	port           string
	logger         *logrus.Logger
	unifiedManager *services.UnifiedProtocolManager
	analytics      *analyticsStore
}

// NewAPIServer creates a new API server instance
func NewAPIServer(port string) *APIServer {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Initialize core services
	unifiedManager := services.NewUnifiedProtocolManager(nil, nil, logger)

	return &APIServer{
		port:           port,
		logger:         logger,
		unifiedManager: unifiedManager,
		analytics:      newAnalyticsStore(),
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

	s.logger.WithField("port", s.port).Info("Starting HelixAgent Protocol Enhancement API Server")
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

	c.JSON(200, gin.H{
		"result": "MCP tool called successfully",
		"tool":   req.ToolName,
		"server": req.ServerID,
	})
}

func (s *APIServer) handleMCPListTools(c *gin.Context) {
	serverID := c.Query("server_id")

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

	c.JSON(200, gin.H{
		"definition": map[string]interface{}{
			"uri":   fmt.Sprintf("file://%s", req.FilePath),
			"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
		},
	})
}

func (s *APIServer) handleLSPDiagnostics(c *gin.Context) {
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
		"version":      version.Version,
		"capabilities": []string{"execute_action", "broadcast", "status"},
	})
}

// System Handlers

func (s *APIServer) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   version.Version,
	})
}

func (s *APIServer) handleStatus(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":              "operational",
		"timestamp":           time.Now().Unix(),
		"protocols_active":    []string{"mcp", "lsp", "acp"},
		"plugins_loaded":      0,
		"templates_available": len(defaultTemplates),
	})
}

func (s *APIServer) handlePrometheusMetrics(c *gin.Context) {
	totalRequests := s.analytics.requests.Len()

	metrics := fmt.Sprintf(`# HELP helixagent_up Server is up
# TYPE helixagent_up gauge
helixagent_up 1
# HELP helixagent_protocols_active Number of active protocols
# TYPE helixagent_protocols_active gauge
helixagent_protocols_active 3
# HELP helixagent_plugins_loaded Number of loaded plugins
# TYPE helixagent_plugins_loaded gauge
helixagent_plugins_loaded 0
# HELP helixagent_requests_total Total number of requests
# TYPE helixagent_requests_total counter
helixagent_requests_total %d
`, totalRequests)
	c.Header("Content-Type", "text/plain")
	c.String(200, metrics)
}

// Analytics Handlers

func (s *APIServer) handleGetAnalytics(c *gin.Context) {
	c.JSON(200, s.analytics.allMetrics())
}

func (s *APIServer) handleGetProtocolMetrics(c *gin.Context) {
	protocol := c.Param("protocol")
	metrics, found := s.analytics.metricsForProtocol(protocol)
	if !found {
		c.JSON(404, gin.H{"error": "no metrics for protocol: " + protocol})
		return
	}
	c.JSON(200, metrics)
}

func (s *APIServer) handleGetHealthStatus(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"analytics": "operational",
	})
}

func (s *APIServer) handleRecordRequest(c *gin.Context) {
	var req analyticsRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	s.analytics.record(req)
	c.JSON(200, gin.H{"status": "recorded"})
}

// Plugin Handlers

func (s *APIServer) handleListPlugins(c *gin.Context) {
	c.JSON(200, gin.H{
		"plugins": []interface{}{},
	})
}

func (s *APIServer) handleLoadPlugin(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Demo: attempt to load plugin (will fail because file doesn't exist)
	c.JSON(500, gin.H{
		"error": fmt.Sprintf("failed to load plugin from %s: file not found", req.Path),
	})
}

func (s *APIServer) handleUnloadPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	c.JSON(500, gin.H{
		"error": fmt.Sprintf("plugin %s not found", pluginID),
	})
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
	c.JSON(500, gin.H{
		"error": fmt.Sprintf("plugin %s not found", pluginID),
	})
}

func (s *APIServer) handleMarketplaceSearch(c *gin.Context) {
	c.JSON(200, gin.H{
		"plugins": []interface{}{},
		"query":   c.Query("q"),
	})
}

func (s *APIServer) handleRegisterPlugin(c *gin.Context) {
	var req struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Author      string   `json:"author"`
		Protocols   []string `json:"protocols"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"status":    "registered",
		"plugin_id": req.ID,
	})
}

// Template Handlers

func (s *APIServer) handleListTemplates(c *gin.Context) {
	protocol := c.Query("protocol")
	var result []integrationTemplate
	for _, t := range defaultTemplates {
		if protocol == "" || t.Protocol == protocol {
			result = append(result, t)
		}
	}
	if result == nil {
		result = []integrationTemplate{}
	}
	c.JSON(200, gin.H{
		"templates": result,
	})
}

func (s *APIServer) handleGetTemplate(c *gin.Context) {
	id := c.Param("id")
	for _, t := range defaultTemplates {
		if t.ID == id {
			c.JSON(200, gin.H{
				"ID":          t.ID,
				"name":        t.Name,
				"protocol":    t.Protocol,
				"description": t.Description,
				"protocols":   t.Protocols,
			})
			return
		}
	}
	c.JSON(404, gin.H{"error": "template not found: " + id})
}

func (s *APIServer) handleGenerateFromTemplate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Find template
	var found *integrationTemplate
	for i := range defaultTemplates {
		if defaultTemplates[i].ID == id {
			found = &defaultTemplates[i]
			break
		}
	}
	if found == nil {
		c.JSON(500, gin.H{
			"error": "template not found: " + id,
		})
		return
	}
	c.JSON(200, gin.H{
		"generated":   true,
		"template_id": found.ID,
		"config":      req.Config,
	})
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
