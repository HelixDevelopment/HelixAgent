package yolo

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRiskLevel_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "low", RiskLow.String())
	assert.Equal(t, "medium", RiskMedium.String())
	assert.Equal(t, "high", RiskHigh.String())
	assert.Equal(t, "critical", RiskCritical.String())
	assert.Equal(t, "unknown", RiskLevel(99).String())
}

func TestNewHeuristicClassifier(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	hc := NewHeuristicClassifier(logger)

	require.NotNil(t, hc)
	assert.NotNil(t, hc.rules)
	assert.NotNil(t, hc.history)
	assert.NotNil(t, hc.logger)
	assert.Greater(t, len(hc.rules), 0)
}

func TestHeuristicClassifier_Name(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	assert.Equal(t, "heuristic", hc.Name())
}

func TestHeuristicClassifier_Classify_ReadOperation(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "read_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}

	classification, err := hc.Classify(ctx, exec)

	require.NoError(t, err)
	assert.Equal(t, "read_file", classification.ToolName)
	assert.Equal(t, RiskLow, classification.RiskLevel)
	assert.True(t, classification.ShouldAllow)
	assert.Greater(t, classification.Confidence, 0.0)
}

func TestHeuristicClassifier_Classify_WriteOperation(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "write_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt", "content": "data"},
	}

	classification, err := hc.Classify(ctx, exec)

	require.NoError(t, err)
	assert.Equal(t, RiskMedium, classification.RiskLevel)
}

func TestHeuristicClassifier_Classify_DeleteOperation(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "delete_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}

	classification, err := hc.Classify(ctx, exec)

	require.NoError(t, err)
	assert.Equal(t, RiskHigh, classification.RiskLevel)
	assert.False(t, classification.ShouldAllow)
}

func TestHeuristicClassifier_Classify_ExecuteCommand(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "execute_terminal",
		Arguments: map[string]interface{}{"command": "ls"},
	}

	classification, err := hc.Classify(ctx, exec)

	require.NoError(t, err)
	assert.Equal(t, RiskHigh, classification.RiskLevel)
}

func TestHeuristicClassifier_Classify_SystemOperation(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "system_config",
		Arguments: map[string]interface{}{"key": "value"},
	}

	classification, err := hc.Classify(ctx, exec)

	require.NoError(t, err)
	assert.Equal(t, RiskCritical, classification.RiskLevel)
	assert.False(t, classification.ShouldAllow)
}

func TestHeuristicClassifier_Train(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)
	ctx := context.Background()

	examples := []TrainingExample{
		{
			Execution:   ToolExecution{ToolName: "safe_tool"},
			ShouldAllow: true,
			RiskLevel:   RiskLow,
		},
		{
			Execution:   ToolExecution{ToolName: "unsafe_tool"},
			ShouldAllow: false,
			RiskLevel:   RiskHigh,
		},
	}

	err := hc.Train(ctx, examples)
	require.NoError(t, err)

	// Check history was updated
	history := hc.GetHistory()
	assert.Contains(t, history, "safe_tool")
	assert.Contains(t, history, "unsafe_tool")
}

func TestHeuristicClassifier_assessToolRisk(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)

	tests := []struct {
		toolName string
		expected RiskLevel
	}{
		{"read_file", RiskLow},
		{"write_file", RiskMedium},
		{"delete_file", RiskHigh},
		{"system_config", RiskCritical},
		{"admin_panel", RiskCritical},
		{"safe_read", RiskLow},
	}

	for _, tt := range tests {
		risk := hc.assessToolRisk(tt.toolName)
		assert.Equal(t, tt.expected, risk, "Tool: %s", tt.toolName)
	}
}

func TestHeuristicClassifier_GetHistory(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)

	// Train with some examples
	ctx := context.Background()
	hc.Train(ctx, []TrainingExample{
		{Execution: ToolExecution{ToolName: "tool1"}, ShouldAllow: true},
	})

	history := hc.GetHistory()

	assert.NotNil(t, history)
	assert.Contains(t, history, "tool1")
}

func TestNewAutoApprover(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	logger := logrus.New()

	approver := NewAutoApprover(classifier, RiskMedium, 0.7, logger)

	require.NotNil(t, approver)
	assert.NotNil(t, approver.classifier)
	assert.Equal(t, RiskMedium, approver.maxRiskLevel)
	assert.Equal(t, 0.7, approver.minConfidence)
	assert.NotNil(t, approver.logger)
}

func TestAutoApprover_Evaluate_AutoApprove(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	approver := NewAutoApprover(classifier, RiskMedium, 0.5, nil)
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "read_file",
		Arguments: map[string]interface{}{},
	}

	decision, err := approver.Evaluate(ctx, exec)

	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.True(t, decision.Auto)
	assert.True(t, decision.Approved)
	assert.Equal(t, RiskLow, decision.Classification.RiskLevel)
}

func TestAutoApprover_Evaluate_AutoReject(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	approver := NewAutoApprover(classifier, RiskLow, 0.5, nil) // Only allow low risk
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "delete_file", // Critical operation
		Arguments: map[string]interface{}{},
	}

	decision, err := approver.Evaluate(ctx, exec)

	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.True(t, decision.Auto)
	assert.False(t, decision.Approved)
}

func TestAutoApprover_Evaluate_ManualReview(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	approver := NewAutoApprover(classifier, RiskLow, 0.95, nil) // High confidence threshold
	ctx := context.Background()

	exec := ToolExecution{
		ToolName:  "unknown_tool",
		Arguments: map[string]interface{}{},
	}

	decision, err := approver.Evaluate(ctx, exec)

	require.NoError(t, err)
	require.NotNil(t, decision)
	assert.False(t, decision.Auto)
	assert.False(t, decision.Approved)
}

func TestAutoApprover_GetStats(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	approver := NewAutoApprover(classifier, RiskMedium, 0.5, nil)
	ctx := context.Background()

	// Make some evaluations
	approver.Evaluate(ctx, ToolExecution{ToolName: "read_file"})
	approver.Evaluate(ctx, ToolExecution{ToolName: "delete_file"})
	approver.Evaluate(ctx, ToolExecution{ToolName: "system_config"})

	stats := approver.GetStats()

	assert.Equal(t, 3, stats.TotalRequests)
	assert.Greater(t, stats.AutoApproved, 0)
}

func TestAutoApprover_ResetStats(t *testing.T) {
	t.Parallel()
	classifier := NewHeuristicClassifier(nil)
	approver := NewAutoApprover(classifier, RiskMedium, 0.5, nil)
	ctx := context.Background()

	approver.Evaluate(ctx, ToolExecution{ToolName: "read_file"})
	approver.ResetStats()

	stats := approver.GetStats()
	assert.Equal(t, 0, stats.TotalRequests)
}

func TestHashExecution(t *testing.T) {
	t.Parallel()
	exec1 := ToolExecution{
		ToolName:  "read_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}

	exec2 := ToolExecution{
		ToolName:  "read_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}

	exec3 := ToolExecution{
		ToolName:  "write_file",
		Arguments: map[string]interface{}{"path": "/tmp/test.txt"},
	}

	hash1 := HashExecution(exec1)
	hash2 := HashExecution(exec2)
	hash3 := HashExecution(exec3)

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, hash1, 16)
}

func TestConfidenceScore(t *testing.T) {
	t.Parallel()
	exec := ToolExecution{ToolName: "read_file"}

	// With good history
	goodHistory := &HistoryEntry{
		Count:       10,
		SuccessRate: 0.95,
	}
	score := ConfidenceScore(exec, goodHistory)
	assert.Greater(t, score, 0.5)

	// With bad history
	badHistory := &HistoryEntry{
		Count:       10,
		SuccessRate: 0.3,
	}
	score = ConfidenceScore(exec, badHistory)
	assert.Less(t, score, 0.5)

	// No history - base score with read bonus
	score = ConfidenceScore(exec, nil)
	assert.GreaterOrEqual(t, score, 0.5)
}

func TestConfidenceScore_ReadOperations(t *testing.T) {
	t.Parallel()
	readExec := ToolExecution{ToolName: "read_file"}
	writeExec := ToolExecution{ToolName: "write_file"}
	deleteExec := ToolExecution{ToolName: "delete_file"}

	readScore := ConfidenceScore(readExec, nil)
	writeScore := ConfidenceScore(writeExec, nil)
	deleteScore := ConfidenceScore(deleteExec, nil)

	assert.Greater(t, readScore, writeScore)
	assert.Greater(t, writeScore, deleteScore)
}

func TestClassification(t *testing.T) {
	t.Parallel()
	classification := &Classification{
		ToolName:    "test_tool",
		RiskLevel:   RiskMedium,
		Confidence:  0.8,
		ShouldAllow: true,
		Reason:      "Test reason",
	}

	assert.Equal(t, "test_tool", classification.ToolName)
	assert.Equal(t, RiskMedium, classification.RiskLevel)
	assert.Equal(t, 0.8, classification.Confidence)
	assert.True(t, classification.ShouldAllow)
}

func TestApprovalDecision(t *testing.T) {
	t.Parallel()
	classification := &Classification{
		ToolName:   "test",
		RiskLevel:  RiskLow,
		Confidence: 0.9,
	}

	decision := &ApprovalDecision{
		Classification: classification,
		Approved:       true,
		Auto:           true,
	}

	assert.True(t, decision.Approved)
	assert.True(t, decision.Auto)
	assert.NotNil(t, decision.Classification)
}

func TestHistoryEntry(t *testing.T) {
	t.Parallel()
	entry := &HistoryEntry{
		ToolName:    "test_tool",
		Count:       5,
		SuccessRate: 0.8,
	}

	assert.Equal(t, "test_tool", entry.ToolName)
	assert.Equal(t, 5, entry.Count)
	assert.Equal(t, 0.8, entry.SuccessRate)
}

func TestRule_Match(t *testing.T) {
	t.Parallel()
	rule := Rule{
		Name: "test_rule",
		Match: func(exec ToolExecution) bool {
			return exec.ToolName == "test_tool"
		},
		RiskLevel:  RiskLow,
		Confidence: 0.9,
	}

	matchingExec := ToolExecution{ToolName: "test_tool"}
	nonMatchingExec := ToolExecution{ToolName: "other_tool"}

	assert.True(t, rule.Match(matchingExec))
	assert.False(t, rule.Match(nonMatchingExec))
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	hc := NewHeuristicClassifier(nil)

	done := make(chan bool, 3)

	go func() {
		ctx := context.Background()
		hc.Classify(ctx, ToolExecution{ToolName: "tool1"})
		done <- true
	}()

	go func() {
		hc.GetHistory()
		done <- true
	}()

	go func() {
		ctx := context.Background()
		hc.Train(ctx, []TrainingExample{
			{Execution: ToolExecution{ToolName: "tool2"}, ShouldAllow: true},
		})
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not panic
}
