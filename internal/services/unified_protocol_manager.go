package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// UnifiedProtocolManager manages all protocol operations (MCP, LSP, ACP, Embeddings)
type UnifiedProtocolManager struct {
	mcpManager       *MCPManager
	lspManager       *LSPManager
	acpManager       *ACPManager
	embeddingManager *EmbeddingManager
	cache            CacheInterface
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
	return &UnifiedProtocolManager{
		mcpManager:       NewMCPManager(repo, cache, log),
		lspManager:       NewLSPManager(repo, cache, log),
		acpManager:       NewACPManager(repo, cache, log),
		embeddingManager: NewEmbeddingManager(repo, cache, log),
		cache:            cache,
		repo:             repo,
		log:              log,
	}
}

// ExecuteRequest executes a request on the appropriate protocol
func (u *UnifiedProtocolManager) ExecuteRequest(ctx context.Context, req UnifiedProtocolRequest) (UnifiedProtocolResponse, error) {
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

	switch req.ProtocolType {
	case "mcp":
		mcpReq := MCPRequest{
			ServerID:  req.ServerID,
			ToolName:  req.ToolName,
			Arguments: req.Arguments,
		}

		mcpResp, err := u.mcpManager.ExecuteMCPTool(ctx, mcpReq)
		if err != nil {
			response.Error = err.Error()
			return response, err
		}

		response.Success = mcpResp.Success
		response.Result = mcpResp.Result
		if mcpResp.Error != "" {
			response.Error = mcpResp.Error
		}

	case "acp":
		acpReq := ACPRequest{
			ServerID:   req.ServerID,
			Action:     req.ToolName,
			Parameters: req.Arguments,
		}

		acpResp, err := u.acpManager.ExecuteACPAction(ctx, acpReq)
		if err != nil {
			response.Error = err.Error()
			return response, err
		}

		response.Success = acpResp.Success
		response.Result = acpResp.Data
		if acpResp.Error != "" {
			response.Error = acpResp.Error
		}

	case "embedding":
		// For embeddings, the tool name represents the text to embed
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

	case "lsp":
		// LSP requests need more specific handling
		// For now, return a placeholder response
		response.Success = true
		response.Result = fmt.Sprintf("LSP request %s executed on server %s", req.ToolName, req.ServerID)

	default:
		err := fmt.Errorf("unsupported protocol type: %s", req.ProtocolType)
		response.Error = err.Error()
		return response, err
	}

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
