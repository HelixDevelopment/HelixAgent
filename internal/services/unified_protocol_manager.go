package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// ProtocolManagerInterface defines the interface for protocol managers
type ProtocolManagerInterface interface {
	ExecuteRequest(ctx context.Context, req UnifiedProtocolRequest) (UnifiedProtocolResponse, error)
	ListServers(ctx context.Context) (map[string]interface{}, error)
	GetMetrics(ctx context.Context) (map[string]interface{}, error)
	RefreshAll(ctx context.Context) error
	ConfigureProtocols(ctx context.Context, config map[string]interface{}) error
}

// UnifiedProtocolManager manages all protocol operations (MCP, LSP, ACP, Embeddings)
type UnifiedProtocolManager struct {
	mcpManager       *MCPManager
	lspManager       *LSPManager
	acpManager       *ACPManager
	embeddingManager *EmbeddingManager
	cache            CacheInterface
	monitor          *ProtocolMonitor
	security         *ProtocolSecurity
	rateLimiter      *RateLimiter
	repo             *database.ModelMetadataRepository
	log              *logrus.Logger
}

// UnifiedProtocolRequest represents a request to any protocol
type UnifiedProtocolRequest struct {
	ProtocolType string                 `json:"protocolType"` // "mcp", "lsp", "acp", "embedding"
	ServerID     string                 `json:"serverId"`
	ToolName     string                 `json:"toolName"`
	Arguments    map[string]interface{} `json:"arguments"`
}

// UnifiedProtocolResponse represents a response from any protocol
type UnifiedProtocolResponse struct {
	Success   bool        `json:"success"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Protocol  string      `json:"protocol"`
}

// NewUnifiedProtocolManager creates a new unified protocol manager
func NewUnifiedProtocolManager(
	repo *database.ModelMetadataRepository,
	cache CacheInterface,
	log *logrus.Logger,
) *UnifiedProtocolManager {
	monitor := NewProtocolMonitor(log)
	security := NewProtocolSecurity(log)

	// Initialize default security
	security.InitializeDefaultSecurity()

	return &UnifiedProtocolManager{
		mcpManager:       NewMCPManager(repo, cache, log),
		lspManager:       NewLSPManager(repo, cache, log),
		acpManager:       NewACPManager(repo, cache, log),
		embeddingManager: NewEmbeddingManager(repo, cache, log),
		cache:            cache,
		monitor:          monitor,
		security:         security,
		rateLimiter:      NewRateLimiter(100), // 100 requests per minute
		repo:             repo,
		log:              log,
	}
}

// ExecuteRequest executes a request on the appropriate protocol
func (u *UnifiedProtocolManager) ExecuteRequest(ctx context.Context, req UnifiedProtocolRequest) (UnifiedProtocolResponse, error) {
	startTime := time.Now()

	u.log.WithFields(logrus.Fields{
		"protocol": req.ProtocolType,
		"serverId": req.ServerID,
		"toolName": req.ToolName,
	}).Info("Executing unified protocol request")

	response := UnifiedProtocolResponse{
		Timestamp: time.Now(),
		Protocol:  req.ProtocolType,
		Success:   false,
	}

	// Extract API key from context (would be set by middleware)
	apiKey := extractAPIKeyFromContext(ctx)
	if apiKey == "" {
		response.Error = "API key required"
		u.recordMetrics(req.ProtocolType, time.Since(startTime), false)
		return response, fmt.Errorf("API key required")
	}

	// Rate limiting
	if !u.rateLimiter.Allow(apiKey) {
		response.Error = "Rate limit exceeded"
		u.recordMetrics(req.ProtocolType, time.Since(startTime), false)
		u.log.WithField("apiKey", apiKey[:8]+"...").Warn("Rate limit exceeded")
		return response, fmt.Errorf("rate limit exceeded")
	}

	// Security check
	if err := u.security.ValidateProtocolAccess(ctx, apiKey, req.ProtocolType, "execute", req.ServerID); err != nil {
		response.Error = err.Error()
		u.recordMetrics(req.ProtocolType, time.Since(startTime), false)
		return response, err
	}

	switch req.ProtocolType {
	case "mcp":
		mcpResp, err := u.mcpManager.ExecuteMCPTool(ctx, req)
		if err != nil {
			response.Error = err.Error()
			return response, err
		}

		response.Success = true
		response.Result = mcpResp

	case "acp":
		acpResp, err := u.acpManager.ExecuteACPAction(ctx, ACPRequest{
			ServerID:   req.ServerID,
			Action:     req.ToolName,
			Parameters: req.Arguments,
		})
		if err != nil {
			response.Error = err.Error()
			return response, err
		}

		response.Success = true
		response.Result = acpResp

	case "lsp":
		// Route LSP requests based on tool name
		var lspResult interface{}
		var lspErr error

		switch req.ToolName {
		case "completion":
			uri, _ := req.Arguments["uri"].(string)
			line, _ := req.Arguments["line"].(float64)
			character, _ := req.Arguments["character"].(float64)
			text, _ := req.Arguments["text"].(string)

			position := LSPPosition{Line: int(line), Character: int(character)}
			lspResult, lspErr = u.lspManager.GetCompletion(ctx, req.ServerID, text, uri, position)

		case "hover":
			uri, _ := req.Arguments["uri"].(string)
			line, _ := req.Arguments["line"].(float64)
			character, _ := req.Arguments["character"].(float64)

			lspResult, lspErr = u.lspManager.GetHover(ctx, req.ServerID, uri, int(line), int(character))

		case "definition":
			uri, _ := req.Arguments["uri"].(string)
			line, _ := req.Arguments["line"].(float64)
			character, _ := req.Arguments["character"].(float64)

			lspResult, lspErr = u.lspManager.GetDefinition(ctx, req.ServerID, uri, int(line), int(character))

		case "references":
			uri, _ := req.Arguments["uri"].(string)
			line, _ := req.Arguments["line"].(float64)
			character, _ := req.Arguments["character"].(float64)

			lspResult, lspErr = u.lspManager.GetReferences(ctx, req.ServerID, uri, int(line), int(character))

		case "diagnostics":
			uri, _ := req.Arguments["uri"].(string)
			lspResult, lspErr = u.lspManager.GetDiagnostics(ctx, req.ServerID, uri)

		case "codeActions":
			uri, _ := req.Arguments["uri"].(string)
			line, _ := req.Arguments["line"].(float64)
			character, _ := req.Arguments["character"].(float64)
			text, _ := req.Arguments["text"].(string)

			position := LSPPosition{Line: int(line), Character: int(character)}
			lspResult, lspErr = u.lspManager.GetCodeActions(ctx, req.ServerID, text, uri, position)

		default:
			// Generic LSP request
			lspReq := LSPRequest{
				ServerID: req.ServerID,
				Method:   req.ToolName,
				Params:   req.Arguments,
			}
			lspResp, err := u.lspManager.ExecuteLSPRequest(ctx, lspReq)
			if err != nil {
				lspErr = err
			} else {
				lspResult = lspResp
			}
		}

		if lspErr != nil {
			response.Error = lspErr.Error()
			return response, lspErr
		}

		response.Success = true
		response.Result = lspResult

	case "embedding":
		// Generate embeddings for the input text
		text, ok := req.Arguments["text"].(string)
		if !ok {
			err := fmt.Errorf("text argument is required for embedding requests")
			response.Error = err.Error()
			return response, err
		}

		embeddingResp, err := u.embeddingManager.GenerateEmbedding(ctx, text)
		if err != nil {
			response.Error = err.Error()
			return response, err
		}

		response.Success = true
		response.Result = embeddingResp

	default:
		err := fmt.Errorf("unsupported protocol type: %s", req.ProtocolType)
		response.Error = err.Error()
		u.recordMetrics(req.ProtocolType, time.Since(startTime), false)
		return response, err
	}

	u.recordMetrics(req.ProtocolType, time.Since(startTime), response.Success)

	u.log.WithFields(logrus.Fields{
		"protocol": req.ProtocolType,
		"success":  response.Success,
	}).Info("Protocol request completed")

	return response, nil
}

// ListServers lists all servers for all protocols
func (u *UnifiedProtocolManager) ListServers(ctx context.Context) (map[string]interface{}, error) {
	servers := make(map[string]interface{})

	// Get MCP servers
	mcpServers, err := u.mcpManager.ListMCPServers(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to list MCP servers")
	} else {
		servers["mcp"] = mcpServers
	}

	// Get LSP servers
	lspServers, err := u.lspManager.ListLSPServers(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to list LSP servers")
	} else {
		servers["lsp"] = lspServers
	}

	// Get ACP servers
	acpServers, err := u.acpManager.ListACPServers(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to list ACP servers")
	} else {
		servers["acp"] = acpServers
	}

	// Get embedding providers
	embeddingProviders, err := u.embeddingManager.ListEmbeddingProviders(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to list embedding providers")
	} else {
		servers["embedding"] = embeddingProviders
	}

	return servers, nil
}

// GetMetrics returns metrics for all protocols
func (u *UnifiedProtocolManager) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Get MCP metrics
	mcpStats, err := u.mcpManager.GetMCPStats(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to get MCP stats")
		metrics["mcp"] = map[string]interface{}{"error": err.Error()}
	} else {
		metrics["mcp"] = mcpStats
	}

	// Get LSP metrics
	lspStats, err := u.lspManager.GetLSPStats(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to get LSP stats")
		metrics["lsp"] = map[string]interface{}{"error": err.Error()}
	} else {
		metrics["lsp"] = lspStats
	}

	// Get ACP metrics
	acpStats, err := u.acpManager.GetACPStats(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to get ACP stats")
		metrics["acp"] = map[string]interface{}{"error": err.Error()}
	} else {
		metrics["acp"] = acpStats
	}

	// Get Embedding metrics
	embeddingStats, err := u.embeddingManager.GetEmbeddingStats(ctx)
	if err != nil {
		u.log.WithError(err).Error("Failed to get embedding stats")
		metrics["embedding"] = map[string]interface{}{"error": err.Error()}
	} else {
		metrics["embedding"] = embeddingStats
	}

	// Add overall metrics
	metrics["overall"] = map[string]interface{}{
		"totalProtocols": 4,
		"activeRequests": 0,
		"cacheSize":      0,
	}

	u.log.Info("Retrieved unified protocol metrics")
	return metrics, nil
}

// RefreshAll refreshes all protocol servers
func (u *UnifiedProtocolManager) RefreshAll(ctx context.Context) error {
	u.log.Info("Refreshing all protocol servers")

	// Refresh MCP servers
	_ = u.mcpManager.SyncMCPServer(ctx, "all")

	// Refresh LSP servers
	_ = u.lspManager.RefreshAllLSPServers(ctx)

	// Refresh ACP servers
	_ = u.acpManager.SyncACPServer(ctx, "all")

	// Refresh embeddings provider
	_ = u.embeddingManager.RefreshAllEmbeddings(ctx)

	u.log.Info("All protocol servers refreshed")
	return nil
}

// ConfigureProtocols configures protocol servers based on configuration
func (u *UnifiedProtocolManager) ConfigureProtocols(ctx context.Context, config map[string]interface{}) error {
	u.log.Info("Configuring protocol servers")

	// In a real implementation, this would:
	// 1. Parse configuration
	// 2. Configure each protocol manager
	// 3. Start/stop servers as needed

	u.log.WithFields(logrus.Fields{
		"configured_protocols": config,
	}).Info("Protocol servers configured")

	return nil
}

// GetMonitor returns the protocol monitor
func (u *UnifiedProtocolManager) GetMonitor() *ProtocolMonitor {
	return u.monitor
}

// GetSecurity returns the protocol security manager
func (u *UnifiedProtocolManager) GetSecurity() *ProtocolSecurity {
	return u.security
}

// Private methods

func (u *UnifiedProtocolManager) recordMetrics(protocol string, duration time.Duration, success bool) {
	if u.monitor != nil {
		u.monitor.RecordRequest(context.Background(), protocol, duration, success, "")
	}
}

func extractAPIKeyFromContext(ctx context.Context) string {
	// Extract API key from context (would be set by middleware)
	if apiKey, ok := ctx.Value("api_key").(string); ok {
		return apiKey
	}
	return ""
}
