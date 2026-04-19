// Package yolo provides ML-based auto-approval for tool execution
// Inspired by Claude Code's YOLO mode
// YOLO = "You Only Look Once" - fast classification for auto-approval
package yolo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// RiskLevel represents the risk level of a tool execution
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

// String returns string representation of risk level
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Classification represents the result of classifying a tool execution
type Classification struct {
	ToolName     string    `json:"tool_name"`
	Arguments    string    `json:"arguments"`
	RiskLevel    RiskLevel `json:"risk_level"`
	Confidence   float64   `json:"confidence"`
	ShouldAllow  bool      `json:"should_allow"`
	Reason       string    `json:"reason"`
	Alternatives []string  `json:"alternatives,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// ToolExecution represents a tool execution request
type ToolExecution struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Classifier interface for YOLO classification
type Classifier interface {
	Name() string
	Classify(ctx context.Context, exec ToolExecution) (*Classification, error)
	Train(ctx context.Context, examples []TrainingExample) error
}

// TrainingExample represents a training example
type TrainingExample struct {
	Execution   ToolExecution `json:"execution"`
	ShouldAllow bool          `json:"should_allow"`
	RiskLevel   RiskLevel     `json:"risk_level"`
}

// HeuristicClassifier uses rule-based heuristics for classification.
//
// Concurrent-safe by construction (CONST-029): history is a safe.Store.
// HistoryEntry field mutations route through Update; readers copy fields
// out under Update to avoid Pattern Beta races.
type HeuristicClassifier struct {
	rules      []Rule // set once at construction
	logger     *logrus.Logger
	history    *safe.Store[string, *HistoryEntry]
	maxHistory int
}

// Rule represents a classification rule
type Rule struct {
	Name        string
	Description string
	Match       func(ToolExecution) bool
	RiskLevel   RiskLevel
	Confidence  float64
}

// HistoryEntry tracks tool execution history
type HistoryEntry struct {
	ToolName    string    `json:"tool_name"`
	Count       int       `json:"count"`
	LastUsed    time.Time `json:"last_used"`
	SuccessRate float64   `json:"success_rate"`
}

// NewHeuristicClassifier creates a new heuristic classifier
func NewHeuristicClassifier(logger *logrus.Logger) *HeuristicClassifier {
	if logger == nil {
		logger = logrus.New()
	}

	hc := &HeuristicClassifier{
		logger:     logger,
		history:    safe.NewStore[string, *HistoryEntry](),
		maxHistory: 1000,
	}

	hc.registerDefaultRules()
	return hc
}

// Name returns classifier name
func (h *HeuristicClassifier) Name() string {
	return "heuristic"
}

// Classify classifies a tool execution
func (h *HeuristicClassifier) Classify(ctx context.Context, exec ToolExecution) (*Classification, error) {
	// Check rules
	for _, rule := range h.rules {
		if rule.Match(exec) {
			return &Classification{
				ToolName:    exec.ToolName,
				Arguments:   fmt.Sprintf("%v", exec.Arguments),
				RiskLevel:   rule.RiskLevel,
				Confidence:  rule.Confidence,
				ShouldAllow: rule.RiskLevel <= RiskMedium,
				Reason:      fmt.Sprintf("Matched rule: %s", rule.Name),
				Timestamp:   time.Now(),
			}, nil
		}
	}

	// Check history for similar executions
	historyRisk := h.assessHistoryRisk(exec)

	// Default classification based on tool name heuristics
	risk := h.assessToolRisk(exec.ToolName)

	// Combine with history
	if historyRisk > risk {
		risk = historyRisk
	}

	return &Classification{
		ToolName:    exec.ToolName,
		Arguments:   fmt.Sprintf("%v", exec.Arguments),
		RiskLevel:   risk,
		Confidence:  0.7,
		ShouldAllow: risk <= RiskMedium,
		Reason:      "Default heuristic assessment",
		Timestamp:   time.Now(),
	}, nil
}

// Train updates the classifier with training examples (no-op for heuristic)
func (h *HeuristicClassifier) Train(ctx context.Context, examples []TrainingExample) error {
	for _, ex := range examples {
		h.updateHistory(ex.Execution.ToolName, ex.ShouldAllow)
	}
	return nil
}

// registerDefaultRules registers default classification rules
func (h *HeuristicClassifier) registerDefaultRules() {
	h.rules = []Rule{
		{
			Name:        "read_only",
			Description: "Read-only operations are low risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "read") ||
					strings.Contains(name, "get") ||
					strings.Contains(name, "list") ||
					strings.Contains(name, "search")
			},
			RiskLevel:  RiskLow,
			Confidence: 0.9,
		},
		{
			Name:        "file_write",
			Description: "File write operations are medium risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return (strings.Contains(name, "write") ||
					strings.Contains(name, "edit") ||
					strings.Contains(name, "create")) &&
					strings.Contains(name, "file")
			},
			RiskLevel:  RiskMedium,
			Confidence: 0.8,
		},
		{
			Name:        "file_delete",
			Description: "File delete operations are high risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "delete") ||
					strings.Contains(name, "remove")
			},
			RiskLevel:  RiskHigh,
			Confidence: 0.85,
		},
		{
			Name:        "execute_command",
			Description: "Command execution is high risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "execute") ||
					strings.Contains(name, "run") ||
					strings.Contains(name, "exec") ||
					strings.Contains(name, "shell") ||
					strings.Contains(name, "terminal")
			},
			RiskLevel:  RiskHigh,
			Confidence: 0.9,
		},
		{
			Name:        "network_operation",
			Description: "Network operations are medium risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "http") ||
					strings.Contains(name, "fetch") ||
					strings.Contains(name, "url") ||
					strings.Contains(name, "network") ||
					strings.Contains(name, "api")
			},
			RiskLevel:  RiskMedium,
			Confidence: 0.75,
		},
		{
			Name:        "system_modification",
			Description: "System modifications are critical risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "system") ||
					strings.Contains(name, "config") ||
					strings.Contains(name, "install") ||
					strings.Contains(name, "uninstall")
			},
			RiskLevel:  RiskCritical,
			Confidence: 0.95,
		},
		{
			Name:        "git_operations",
			Description: "Git operations are medium risk",
			Match: func(exec ToolExecution) bool {
				name := strings.ToLower(exec.ToolName)
				return strings.Contains(name, "git") ||
					strings.Contains(name, "commit") ||
					strings.Contains(name, "push") ||
					strings.Contains(name, "pull")
			},
			RiskLevel:  RiskMedium,
			Confidence: 0.8,
		},
	}
}

// assessToolRisk assesses risk based on tool name
func (h *HeuristicClassifier) assessToolRisk(toolName string) RiskLevel {
	name := strings.ToLower(toolName)

	// Critical risk patterns
	criticalPatterns := []string{"system", "admin", "root", "sudo", "chmod", "chown"}
	for _, pattern := range criticalPatterns {
		if strings.Contains(name, pattern) {
			return RiskCritical
		}
	}

	// High risk patterns
	highPatterns := []string{"delete", "remove", "exec", "shell", "terminal", "dangerous"}
	for _, pattern := range highPatterns {
		if strings.Contains(name, pattern) {
			return RiskHigh
		}
	}

	// Medium risk patterns
	mediumPatterns := []string{"write", "edit", "modify", "update", "create", "post", "put"}
	for _, pattern := range mediumPatterns {
		if strings.Contains(name, pattern) {
			return RiskMedium
		}
	}

	return RiskLow
}

// assessHistoryRisk assesses risk based on historical usage
func (h *HeuristicClassifier) assessHistoryRisk(exec ToolExecution) RiskLevel {
	var (
		risk     = RiskMedium
		notFound bool
	)
	h.history.Update(exec.ToolName, func(entry *HistoryEntry, ok bool) (*HistoryEntry, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		if entry.SuccessRate < 0.5 {
			risk = RiskHigh
		} else if entry.Count > 10 && entry.SuccessRate > 0.9 {
			risk = RiskLow
		}
		return entry, true
	})
	if notFound {
		return RiskMedium
	}
	return risk
}

// updateHistory updates the execution history
func (h *HeuristicClassifier) updateHistory(toolName string, success bool) {
	h.history.Update(toolName, func(entry *HistoryEntry, _ bool) (*HistoryEntry, bool) {
		if entry == nil {
			entry = &HistoryEntry{ToolName: toolName}
		}
		entry.Count++
		entry.LastUsed = time.Now()
		if success {
			entry.SuccessRate = 0.9*entry.SuccessRate + 0.1
		} else {
			entry.SuccessRate = 0.9 * entry.SuccessRate
		}
		return entry, true
	})
}

// GetHistory returns execution history (a snapshot of value copies).
func (h *HeuristicClassifier) GetHistory() map[string]*HistoryEntry {
	history := make(map[string]*HistoryEntry)
	for _, k := range h.history.Keys() {
		h.history.Update(k, func(entry *HistoryEntry, ok bool) (*HistoryEntry, bool) {
			if !ok {
				return nil, false
			}
			copy := *entry
			history[k] = &copy
			return entry, true
		})
	}
	return history
}

// AutoApprover manages automatic approval based on classification
type AutoApprover struct {
	classifier    Classifier
	maxRiskLevel  RiskLevel
	minConfidence float64
	logger        *logrus.Logger
	stats         ApprovalStats
	statsMu       sync.RWMutex
}

// ApprovalStats tracks approval statistics
type ApprovalStats struct {
	TotalRequests int `json:"total_requests"`
	AutoApproved  int `json:"auto_approved"`
	AutoRejected  int `json:"auto_rejected"`
	Pending       int `json:"pending"`
}

// NewAutoApprover creates a new auto-approver
func NewAutoApprover(classifier Classifier, maxRisk RiskLevel, minConfidence float64, logger *logrus.Logger) *AutoApprover {
	if logger == nil {
		logger = logrus.New()
	}

	return &AutoApprover{
		classifier:    classifier,
		maxRiskLevel:  maxRisk,
		minConfidence: minConfidence,
		logger:        logger,
	}
}

// Evaluate evaluates a tool execution and returns approval decision
func (a *AutoApprover) Evaluate(ctx context.Context, exec ToolExecution) (*ApprovalDecision, error) {
	a.statsMu.Lock()
	a.stats.TotalRequests++
	a.statsMu.Unlock()

	classification, err := a.classifier.Classify(ctx, exec)
	if err != nil {
		return nil, err
	}

	decision := &ApprovalDecision{
		Classification: classification,
		Timestamp:      time.Now(),
	}

	// Determine approval based on classification
	if classification.Confidence < a.minConfidence {
		// Low confidence - requires manual review
		decision.Approved = false
		decision.Auto = false
		decision.Reason = "Requires manual review: confidence too low"
	} else if classification.RiskLevel > a.maxRiskLevel {
		// High risk - auto reject
		decision.Approved = false
		decision.Auto = true
		decision.Reason = "Auto-rejected: risk level too high"
	} else {
		// Within acceptable risk - auto approve/reject based on classification
		decision.Approved = classification.ShouldAllow
		decision.Auto = true
	}

	// Update stats
	a.statsMu.Lock()
	if decision.Auto {
		if decision.Approved {
			a.stats.AutoApproved++
		} else {
			a.stats.AutoRejected++
		}
	} else {
		a.stats.Pending++
	}
	a.statsMu.Unlock()

	a.logger.WithFields(logrus.Fields{
		"tool":       exec.ToolName,
		"risk":       classification.RiskLevel,
		"confidence": classification.Confidence,
		"approved":   decision.Approved,
		"auto":       decision.Auto,
	}).Debug("Tool execution evaluated")

	return decision, nil
}

// ApprovalDecision represents an approval decision
type ApprovalDecision struct {
	Classification *Classification `json:"classification"`
	Approved       bool            `json:"approved"`
	Auto           bool            `json:"auto"`
	Reason         string          `json:"reason,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// GetStats returns approval statistics
func (a *AutoApprover) GetStats() ApprovalStats {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()
	return a.stats
}

// ResetStats resets approval statistics
func (a *AutoApprover) ResetStats() {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()
	a.stats = ApprovalStats{}
}

// HashExecution creates a hash of tool execution for caching/lookup
func HashExecution(exec ToolExecution) string {
	data := fmt.Sprintf("%s:%v", exec.ToolName, exec.Arguments)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

// ConfidenceScore calculates a confidence score based on various factors
func ConfidenceScore(exec ToolExecution, history *HistoryEntry) float64 {
	score := 0.5 // Base confidence

	// Boost for known tools with good history
	if history != nil {
		if history.Count > 5 {
			score += 0.1
		}
		if history.SuccessRate > 0.8 {
			score += 0.2
		}
		if history.SuccessRate < 0.5 {
			score -= 0.3
		}
	}

	// Boost for simple read operations
	name := strings.ToLower(exec.ToolName)
	if strings.Contains(name, "read") || strings.Contains(name, "get") {
		score += 0.1
	}

	// Penalty for destructive operations
	if strings.Contains(name, "delete") || strings.Contains(name, "remove") {
		score -= 0.2
	}

	return math.Max(0.0, math.Min(1.0, score))
}
