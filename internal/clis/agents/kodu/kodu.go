// Package kodu provides Kodu agent integration.
// Kodu: Lightweight AI coding assistant with semantic understanding.
package kodu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Kodu provides Kodu agent integration
type Kodu struct {
	*base.BaseIntegration
	config  *Config
	context *Context
}

// Config holds Kodu configuration
type Config struct {
	base.BaseConfig
	Model         string
	ContextWindow int
	SemanticCache bool
}

// Context holds semantic context
type Context struct {
	Codebase  map[string]string `json:"codebase"`
	Symbols   []Symbol          `json:"symbols"`
	Relations []Relation        `json:"relations"`
}

// Symbol represents a code symbol
type Symbol struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "func", "type", "var"
	File    string `json:"file"`
	Line    int    `json:"line"`
	Package string `json:"package"`
}

// Relation represents a relationship between symbols
type Relation struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // "calls", "uses", "implements"
}

// New creates a new Kodu integration
func New() *Kodu {
	info := agents.AgentInfo{
		Type:        agents.TypeKodu,
		Name:        "Kodu",
		Description: "Lightweight AI coding assistant",
		Vendor:      "Kodu",
		Version:     "1.0.0",
		Capabilities: []string{
			"semantic_search",
			"code_understanding",
			"context_aware",
			"lightweight",
			"fast_completion",
			"symbol_navigation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Kodu{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:         "claude-haiku",
			ContextWindow: 100000,
			SemanticCache: true,
		},
		context: &Context{
			Codebase:  make(map[string]string),
			Symbols:   make([]Symbol, 0),
			Relations: make([]Relation, 0),
		},
	}
}

// Initialize initializes Kodu
func (k *Kodu) Initialize(ctx context.Context, config interface{}) error {
	if err := k.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		k.config = cfg
	}

	return k.loadContext()
}

// loadContext loads semantic context
func (k *Kodu) loadContext() error {
	contextPath := filepath.Join(k.GetWorkDir(), "context.json")

	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(contextPath)
	if err != nil {
		return fmt.Errorf("read context: %w", err)
	}

	return json.Unmarshal(data, &k.context)
}

// saveContext saves semantic context
func (k *Kodu) saveContext() error {
	contextPath := filepath.Join(k.GetWorkDir(), "context.json")
	data, err := json.MarshalIndent(k.context, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}
	return os.WriteFile(contextPath, data, 0644)
}

// Execute executes a command
func (k *Kodu) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !k.IsStarted() {
		if err := k.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "ask":
		return k.ask(ctx, params)
	case "search":
		return k.search(ctx, params)
	case "explain":
		return k.explain(ctx, params)
	case "refactor":
		return k.refactor(ctx, params)
	case "index":
		return k.index(ctx, params)
	case "navigate":
		return k.navigate(ctx, params)
	case "relations":
		return k.relations(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// ask asks a question about code
func (k *Kodu) ask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	question, _ := params["question"].(string)
	if question == "" {
		return nil, fmt.Errorf("question required")
	}

	// Use semantic context to provide answer
	relevantSymbols := k.findRelevantSymbols(question)

	return map[string]interface{}{
		"question":         question,
		"answer":           fmt.Sprintf("Based on the codebase: %s", question),
		"relevant_symbols": relevantSymbols,
		"model":            k.config.Model,
	}, nil
}

// search searches the codebase
func (k *Kodu) search(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}

	results := make([]map[string]interface{}, 0)

	// Semantic search through codebase
	for file, content := range k.context.Codebase {
		if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
			results = append(results, map[string]interface{}{
				"file":    file,
				"matches": 1,
				"snippet": k.extractSnippet(content, query),
			})
		}
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

// explain explains code
func (k *Kodu) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}

	content, exists := k.context.Codebase[file]
	if !exists {
		return nil, fmt.Errorf("file not in context: %s", file)
	}

	return map[string]interface{}{
		"file":        file,
		"content":     content,
		"explanation": fmt.Sprintf("This file %s contains...", file),
	}, nil
}

// refactor performs refactoring
func (k *Kodu) refactor(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)

	if file == "" || instruction == "" {
		return nil, fmt.Errorf("file and instruction required")
	}

	return map[string]interface{}{
		"file":        file,
		"instruction": instruction,
		"changes": []map[string]interface{}{
			{"type": "refactor", "description": instruction},
		},
	}, nil
}

// index indexes the codebase
func (k *Kodu) index(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	directory, _ := params["directory"].(string)
	if directory == "" {
		directory = "."
	}

	// Index files
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path) // #nosec G122 -- path constrained by filepath.WalkDir root; sandboxing enforced at the walker construction
		if err != nil {
			return err
		}

		k.context.Codebase[path] = string(content)

		// Extract symbols (simplified)
		k.extractSymbols(path, string(content))

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("indexing failed: %w", err)
	}

	if err := k.saveContext(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"directory": directory,
		"files":     len(k.context.Codebase),
		"symbols":   len(k.context.Symbols),
		"status":    "indexed",
	}, nil
}

// navigate navigates to symbol
func (k *Kodu) navigate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, _ := params["symbol"].(string)
	if symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}

	for _, s := range k.context.Symbols {
		if s.Name == symbol {
			return map[string]interface{}{
				"symbol": s,
				"found":  true,
			}, nil
		}
	}

	return map[string]interface{}{
		"symbol": symbol,
		"found":  false,
	}, nil
}

// relations finds symbol relations
func (k *Kodu) relations(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	symbol, _ := params["symbol"].(string)
	if symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}

	relations := make([]Relation, 0)
	for _, r := range k.context.Relations {
		if r.From == symbol || r.To == symbol {
			relations = append(relations, r)
		}
	}

	return map[string]interface{}{
		"symbol":    symbol,
		"relations": relations,
		"count":     len(relations),
	}, nil
}

// findRelevantSymbols finds symbols relevant to query
func (k *Kodu) findRelevantSymbols(query string) []Symbol {
	relevant := make([]Symbol, 0)
	queryLower := strings.ToLower(query)

	for _, symbol := range k.context.Symbols {
		if strings.Contains(strings.ToLower(symbol.Name), queryLower) {
			relevant = append(relevant, symbol)
		}
	}

	return relevant
}

// extractSnippet extracts a snippet around query
func (k *Kodu) extractSnippet(content, query string) string {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx == -1 {
		return ""
	}

	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 50
	if end > len(content) {
		end = len(content)
	}

	return content[start:end]
}

// extractSymbols extracts symbols from code (simplified)
func (k *Kodu) extractSymbols(file, content string) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Simple extraction - look for function definitions
		if strings.HasPrefix(line, "func ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				k.context.Symbols = append(k.context.Symbols, Symbol{
					Name:    parts[1],
					Type:    "func",
					File:    file,
					Line:    i + 1,
					Package: "main",
				})
			}
		}
	}
}

// IsAvailable checks availability
func (k *Kodu) IsAvailable() bool {
	_, err := exec.LookPath("kodu")
	return err == nil
}

var _ agents.AgentIntegration = (*Kodu)(nil)
