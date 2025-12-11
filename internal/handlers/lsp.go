package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// LSPHandler implements Language Server Protocol endpoints
type LSPHandler struct {
	providerRegistry *services.ProviderRegistry
	lspClient        *services.LSPClient
	config           *config.LSPConfig
}

// NewLSPHandler creates a new LSP handler
func NewLSPHandler(registry *services.ProviderRegistry, config *config.LSPConfig) *LSPHandler {
	return &LSPHandler{
		providerRegistry: registry,
		config:           config,
	}
}

// InitializeLSP initializes the LSP client for a specific language
func (h *LSPHandler) InitializeLSP(ctx context.Context, workspaceRoot, languageID string) error {
	h.lspClient = services.NewLSPClient(workspaceRoot, languageID)
	return h.lspClient.StartServer(ctx)
}

// GetLSPClient returns the LSP client
func (h *LSPHandler) GetLSPClient() *services.LSPClient {
	return h.lspClient
}

// LSPCapabilities returns LSP capabilities from all providers
func (h *LSPHandler) LSPCapabilities(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "LSP is not enabled",
		})
		return
	}

	// Return SuperAgent unified capabilities directly
	unifiedCaps := map[string]interface{}{
		"textDocumentSync": map[string]interface{}{
			"openClose": true,
			"change":    2, // Incremental
		},
		"completionProvider": map[string]interface{}{
			"resolveProvider":   true,
			"triggerCharacters": []string{".", "(", "["},
		},
		"hoverProvider":           true,
		"definitionProvider":      true,
		"referencesProvider":      true,
		"documentSymbolProvider":  true,
		"workspaceSymbolProvider": true,
		"codeActionProvider": map[string]interface{}{
			"resolveProvider": true,
		},
		"codeLensProvider": map[string]interface{}{
			"resolveProvider": true,
		},
		"documentFormattingProvider":      true,
		"documentRangeFormattingProvider": true,
		"renameProvider": map[string]interface{}{
			"prepareProvider": true,
		},
		"foldingRangeProvider":   true,
		"selectionRangeProvider": true,
		"callHierarchyProvider":  true,
		"semanticTokensProvider": map[string]interface{}{
			"legend": map[string]interface{}{
				"tokenTypes": []string{
					"namespace", "type", "class", "enum", "interface", "struct", "typeParameter",
					"parameter", "variable", "property", "enumMember", "event", "function",
					"method", "macro", "keyword", "modifier", "comment", "string", "number",
					"regexp", "operator", "decorator",
				},
				"tokenModifiers": []string{
					"declaration", "definition", "readonly", "static", "deprecated", "abstract",
					"async", "modification", "documentation", "defaultLibrary",
				},
			},
			"full": true,
		},
	}

	providers := h.providerRegistry.ListProviders()
	capabilities := map[string]interface{}{}

	for _, providerName := range providers {
		// Provide basic LSP capabilities for each provider
		capabilities[providerName] = map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    2,
			},
			"completionProvider": map[string]interface{}{
				"resolveProvider":   true,
				"triggerCharacters": []string{".", "(", "["},
			},
			"hoverProvider":      true,
			"definitionProvider": true,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"serverInfo": map[string]interface{}{
			"name":    "SuperAgent LSP Server",
			"version": "1.0.0",
		},
		"capabilities": unifiedCaps,
		"providers":    capabilities,
	})
}

// LSPCompletion provides code completion using LSP client or fallback to LLMs
func (h *LSPHandler) LSPCompletion(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "LSP is not enabled",
		})
		return
	}

	var request struct {
		TextDocument struct {
			URI   string   `json:"uri"`
			Lines []string `json:"lines"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
		Context struct {
			TriggerKind      int    `json:"triggerKind"`
			TriggerCharacter string `json:"triggerCharacter"`
		} `json:"context"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Try LSP client first if available
	if h.lspClient != nil {
		cursorPos := &models.Position{
			Line:      request.Position.Line,
			Character: request.Position.Character,
		}

		intelligence, err := h.lspClient.GetCodeIntelligence(c.Request.Context(), request.TextDocument.URI, cursorPos)
		if err == nil && len(intelligence.Completions) > 0 {
			// Convert LSP completions to response format
			completions := make([]interface{}, len(intelligence.Completions))
			for i, item := range intelligence.Completions {
				completions[i] = map[string]interface{}{
					"label":         item.Label,
					"kind":          item.Kind,
					"detail":        item.Detail,
					"documentation": item.Documentation,
					"insertText":    item.InsertText,
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"isIncomplete": false,
				"items":        completions,
			})
			return
		}
	}

	// Fallback to LLM-based completion
	prompt := h.buildCompletionPrompt(request.TextDocument.Lines, request.Position.Line, request.Position.Character)

	req := &models.LLMRequest{
		ID:     fmt.Sprintf("lsp-%d", time.Now().Unix()),
		Prompt: prompt,
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a code completion assistant. Provide concise, accurate code completions based on context.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ModelParams: models.ModelParameters{
			Model:     "superagent-ensemble",
			MaxTokens: 100,
		},
		Status: "pending",
	}

	// Get ensemble service
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ensemble service not available",
		})
		return
	}

	// Process with ensemble
	result, err := ensembleService.RunEnsemble(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Completion failed: %v", err),
		})
		return
	}

	// Convert LLM response to LSP completion items
	completions := h.convertToCompletionItems(result.Selected.Content, request.Position.Line, request.Position.Character)

	c.JSON(http.StatusOK, gin.H{
		"isIncomplete": false,
		"items":        completions,
	})
}

// LSPHover provides hover information
func (h *LSPHandler) LSPHover(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "LSP is not enabled",
		})
		return
	}

	var request struct {
		TextDocument struct {
			URI   string   `json:"uri"`
			Lines []string `json:"lines"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Build context for hover
	prompt := h.buildHoverPrompt(request.TextDocument.Lines, request.Position.Line, request.Position.Character)

	req := &models.LLMRequest{
		ID:     fmt.Sprintf("lsp-hover-%d", time.Now().Unix()),
		Prompt: prompt,
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a programming documentation assistant. Provide clear, concise explanations of code elements.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ModelParams: models.ModelParameters{
			Model:     "superagent-ensemble",
			MaxTokens: 150,
		},
		Status: "pending",
	}

	// Get ensemble service
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ensemble service not available",
		})
		return
	}

	result, err := ensembleService.RunEnsemble(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Hover failed: %v", err),
		})
		return
	}

	hoverResponse := map[string]interface{}{
		"contents": map[string]interface{}{
			"kind":  "markdown",
			"value": result.Selected.Content,
		},
		"range": map[string]interface{}{
			"start": map[string]interface{}{
				"line":      request.Position.Line,
				"character": request.Position.Character,
			},
			"end": map[string]interface{}{
				"line":      request.Position.Line,
				"character": request.Position.Character + 5, // Approximate
			},
		},
	}

	c.JSON(http.StatusOK, hoverResponse)
}

// LSPCodeActions provides code actions using LLMs
func (h *LSPHandler) LSPCodeActions(c *gin.Context) {
	if !h.config.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "LSP is not enabled",
		})
		return
	}

	var request struct {
		TextDocument struct {
			URI   string   `json:"uri"`
			Lines []string `json:"lines"`
		} `json:"textDocument"`
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
			End struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"end"`
		} `json:"range"`
		Context struct {
			Diagnostics []interface{} `json:"diagnostics"`
		} `json:"context"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Build context for code actions
	prompt := h.buildCodeActionPrompt(request.TextDocument.Lines, request.Range)

	req := &models.LLMRequest{
		ID:     fmt.Sprintf("lsp-action-%d", time.Now().Unix()),
		Prompt: prompt,
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a code refactoring assistant. Suggest specific code improvements and refactoring actions.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ModelParams: models.ModelParameters{
			Model:     "superagent-ensemble",
			MaxTokens: 200,
		},
		Status: "pending",
	}

	// Get ensemble service
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ensemble service not available",
		})
		return
	}

	result, err := ensembleService.RunEnsemble(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Code actions failed: %v", err),
		})
		return
	}

	actions := h.convertToCodeActions(result.Selected.Content, request.Range)

	c.JSON(http.StatusOK, actions)
}

// Helper functions
func (h *LSPHandler) convertToLSPCapabilities(providerName string, caps *models.ProviderCapabilities) map[string]interface{} {
	lspCaps := map[string]interface{}{
		"textDocumentSync": map[string]interface{}{
			"openClose": true,
			"change":    2, // Incremental
		},
	}

	if caps.SupportsCodeCompletion {
		lspCaps["completionProvider"] = map[string]interface{}{
			"resolveProvider":   true,
			"triggerCharacters": []string{".", "(", "["},
		}
	}

	if caps.SupportsCodeAnalysis {
		lspCaps["hoverProvider"] = true
		lspCaps["definitionProvider"] = true
		lspCaps["referencesProvider"] = true
		lspCaps["documentSymbolProvider"] = true
	}

	if caps.SupportsRefactoring {
		lspCaps["codeActionProvider"] = map[string]interface{}{
			"resolveProvider": true,
		}
		lspCaps["renameProvider"] = map[string]interface{}{
			"prepareProvider": true,
		}
	}

	return lspCaps
}

func (h *LSPHandler) buildCompletionPrompt(lines []string, line, char int) string {
	context := ""
	if line >= 0 && line < len(lines) {
		// Include current line and a few lines before/after for context
		start := line - 2
		if start < 0 {
			start = 0
		}
		end := line + 2
		if end >= len(lines) {
			end = len(lines) - 1
		}

		for i := start; i <= end; i++ {
			context += fmt.Sprintf("%d: %s\n", i, lines[i])
		}
	}

	return fmt.Sprintf(`Complete the code at position line %d, character %d:

%s

Provide 3-5 concise completion options. Format as JSON array:
[
  {"label": "completion1", "insertText": "completion1"},
  {"label": "completion2", "insertText": "completion2"},
  ...
]`, line, char, context)
}

func (h *LSPHandler) buildHoverPrompt(lines []string, line, char int) string {
	if line >= 0 && line < len(lines) {
		return fmt.Sprintf(`Explain the code element at line %d, character %d:

%s

Provide a brief, clear explanation of what this code element does or represents.`, line, char, lines[line])
	}
	return "Explain the code element at the specified position."
}

func (h *LSPHandler) buildCodeActionPrompt(lines []string, range_ struct {
	Start struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"start"`
	End struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"end"`
}) string {
	codeRange := ""
	if range_.Start.Line >= 0 && range_.Start.Line < len(lines) {
		for i := range_.Start.Line; i <= range_.End.Line && i < len(lines); i++ {
			codeRange += lines[i] + "\n"
		}
	}

	return fmt.Sprintf(`Suggest code improvements for this range (lines %d-%d):

%s

Provide 3-5 specific code actions. Format as JSON array:
[
  {"title": "action1", "kind": "refactor", "edit": {"newText": "improved code"}},
  {"title": "action2", "kind": "quickfix", "edit": {"newText": "fixed code"}},
  ...
]`, range_.Start.Line, range_.End.Line, codeRange)
}

func (h *LSPHandler) convertToCompletionItems(content string, line, char int) []interface{} {
	// Parse the LLM response and convert to LSP completion items
	// For now, return a basic structure
	return []interface{}{
		map[string]interface{}{
			"label":      "Example completion",
			"kind":       1, // Text
			"insertText": content,
			"detail":     "Suggested by SuperAgent ensemble",
			"documentation": map[string]interface{}{
				"kind":  "markdown",
				"value": "Code completion generated using multiple LLM providers",
			},
		},
	}
}

func (h *LSPHandler) convertToCodeActions(content string, range_ interface{}) interface{} {
	// Parse the LLM response and convert to LSP code actions
	// For now, return a basic structure
	return []interface{}{
		map[string]interface{}{
			"title": "Suggested refactoring",
			"kind":  "refactor",
			"edit": map[string]interface{}{
				"range":   range_,
				"newText": content,
			},
		},
	}
}
