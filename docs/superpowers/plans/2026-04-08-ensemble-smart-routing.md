# Ensemble Smart Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route tool-calling and simple requests to a direct provider, complex development requests to debate ensemble.

**Architecture:** Two-layer routing in `processWithEnsemble()`: Layer 1 checks for tools (deterministic), Layer 2 uses EnhancedIntentClassifier (LLM-based). Direct path uses HelixLLM with cloud fallback, or highest-scored provider when HelixLLM disabled.

**Tech Stack:** Go, existing LLMProvider interface, EnhancedIntentClassifier, LLMsVerifier scoring

---

### Task 1: Add smart routing in processWithEnsemble

**Files:**
- Modify: `internal/handlers/openai_compatible.go:2265-2326`

- [ ] **Step 1: Add routing logic at the top of processWithEnsemble**

Insert at the beginning of `processWithEnsemble()`, before the AgenticEnsemble check:

```go
// Smart routing: route tool-calling and simple requests to direct provider
// Layer 1 (deterministic): tools present → direct provider
if len(req.Tools) > 0 {
    logrus.WithField("tool_count", len(req.Tools)).Info("[Smart Routing] Tools detected — routing to direct provider")
    return h.processWithDirectProvider(ctx, req, openaiReq)
}

// Layer 2 (LLM-based): intent classification for non-tool requests
if h.debateService != nil && h.debateService.GetEnhancedIntentClassifier() != nil {
    classifier := h.debateService.GetEnhancedIntentClassifier()
    classifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    var lastUserMessage string
    for i := len(req.Messages) - 1; i >= 0; i-- {
        if req.Messages[i].Role == "user" {
            lastUserMessage = req.Messages[i].Content
            break
        }
    }
    
    if lastUserMessage != "" {
        result, err := classifier.ClassifyEnhancedIntent(classifyCtx, lastUserMessage, "", nil)
        if err == nil {
            if result.ActionType == services.ActionConversation || result.ActionType == services.ActionAnalysis || result.ActionType == services.ActionSingleOp {
                logrus.WithFields(logrus.Fields{
                    "action_type": result.ActionType,
                    "granularity": result.Granularity,
                }).Info("[Smart Routing] Simple intent — routing to direct provider")
                return h.processWithDirectProvider(ctx, req, openaiReq)
            }
        } else {
            logrus.WithError(err).Debug("[Smart Routing] Classification failed, defaulting to debate")
        }
    }
}
```

- [ ] **Step 2: Build and verify compilation**

Run: `go build ./cmd/helixagent/`

- [ ] **Step 3: Commit**

```bash
git add internal/handlers/openai_compatible.go
git commit -m "feat(routing): add Layer 1+2 smart routing in processWithEnsemble"
```

### Task 2: Implement processWithDirectProvider

**Files:**
- Modify: `internal/handlers/openai_compatible.go` (add new method)

- [ ] **Step 1: Add processWithDirectProvider method**

Add after `processWithEnsemble()`:

```go
// processWithDirectProvider routes a request to a single tool-capable provider
// instead of the multi-round debate ensemble. Used for tool-calling requests
// and simple conversational messages.
func (h *UnifiedHandler) processWithDirectProvider(ctx context.Context, req *models.LLMRequest, openaiReq *OpenAIChatRequest) (*services.EnsembleResult, error) {
    // Select provider: HelixLLM (local-first) or highest-scored cloud
    useHelixLLM := os.Getenv("USE_HELIX_LLM") == "true"
    
    if useHelixLLM && h.providerRegistry != nil {
        // Try HelixLLM first
        provider, err := h.providerRegistry.GetProvider("helixllm")
        if err == nil {
            response, err := provider.Complete(ctx, req)
            if err == nil {
                logrus.Info("[Direct Provider] HelixLLM responded successfully")
                return &services.EnsembleResult{
                    Responses:    []*models.LLMResponse{response},
                    Selected:     response,
                    VotingMethod: "direct_provider",
                    Scores:       map[string]float64{response.ID: 1.0},
                    Metadata: map[string]any{
                        "provider":    "helixllm",
                        "route":       "direct",
                        "tools_count": len(req.Tools),
                    },
                }, nil
            }
            logrus.WithError(err).Warn("[Direct Provider] HelixLLM failed, falling back to cloud")
        }
    }
    
    // Fallback: highest-scored verified provider
    if h.providerRegistry != nil {
        providers := h.providerRegistry.GetVerifiedProviders()
        for _, p := range providers {
            if p.Name == "helixllm" {
                continue // skip HelixLLM (already tried or disabled)
            }
            response, err := p.Provider.Complete(ctx, req)
            if err == nil {
                logrus.WithField("provider", p.Name).Info("[Direct Provider] Cloud provider responded")
                return &services.EnsembleResult{
                    Responses:    []*models.LLMResponse{response},
                    Selected:     response,
                    VotingMethod: "direct_provider",
                    Scores:       map[string]float64{response.ID: 1.0},
                    Metadata: map[string]any{
                        "provider":    p.Name,
                        "route":       "direct_fallback",
                        "tools_count": len(req.Tools),
                    },
                }, nil
            }
            logrus.WithError(err).WithField("provider", p.Name).Debug("[Direct Provider] Provider failed, trying next")
        }
    }
    
    return nil, fmt.Errorf("no direct provider available")
}
```

- [ ] **Step 2: Add os import if missing**

Check and add `"os"` to imports if not present.

- [ ] **Step 3: Build and verify**

Run: `go build ./cmd/helixagent/`

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/openai_compatible.go
git commit -m "feat(routing): implement processWithDirectProvider with HelixLLM fallback"
```

### Task 3: Add GetEnhancedIntentClassifier accessor

**Files:**
- Modify: `internal/services/debate_service.go`

- [ ] **Step 1: Add accessor method**

```go
// GetEnhancedIntentClassifier returns the intent classifier for external routing decisions.
func (ds *DebateService) GetEnhancedIntentClassifier() *EnhancedIntentClassifier {
    return ds.enhancedIntentClassifier
}
```

- [ ] **Step 2: Add GetVerifiedProviders to provider registry if missing**

Check if `GetVerifiedProviders()` exists on `ProviderRegistry`. If not, add a simple accessor that returns verified providers sorted by score.

- [ ] **Step 3: Build and verify**

Run: `go build ./cmd/helixagent/`

- [ ] **Step 4: Commit**

```bash
git add internal/services/debate_service.go internal/services/provider_registry.go
git commit -m "feat(routing): add accessor methods for smart routing"
```

### Task 4: Add streaming support for direct provider

**Files:**
- Modify: `internal/handlers/openai_compatible.go`

- [ ] **Step 1: Add routing in processWithEnsembleStream**

Same Layer 1 + Layer 2 logic but for streaming path. Route to direct provider's `CompleteStream` method.

- [ ] **Step 2: Build and verify**

- [ ] **Step 3: Commit**

### Task 5: Create challenge scripts

**Files:**
- Create: `challenges/scripts/cli_agent_ensemble_routing_challenge.sh`
- Create: `challenges/scripts/cli_agent_tool_execution_challenge.sh`

- [ ] **Step 1: Create routing challenge**
- [ ] **Step 2: Create tool execution challenge**
- [ ] **Step 3: Run both challenges**
- [ ] **Step 4: Commit**

### Task 6: Final integration test

- [ ] **Step 1: Restart HelixAgent with new binary**
- [ ] **Step 2: Run all challenges**
- [ ] **Step 3: Test with OpenCode manually**
- [ ] **Step 4: Commit and push all**
