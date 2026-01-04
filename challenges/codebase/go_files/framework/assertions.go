// Package framework provides the assertion engine implementation.
package framework

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Engine implements AssertionEngine.
type Engine struct {
	mu         sync.RWMutex
	evaluators map[string]AssertionEvaluator
}

// NewAssertionEngine creates a new assertion engine with default evaluators.
func NewAssertionEngine() *Engine {
	e := &Engine{
		evaluators: make(map[string]AssertionEvaluator),
	}

	// Register default evaluators
	e.registerDefaults()

	return e
}

// registerDefaults registers all default assertion evaluators.
func (e *Engine) registerDefaults() {
	e.evaluators["not_empty"] = evaluateNotEmpty
	e.evaluators["not_mock"] = evaluateNotMock
	e.evaluators["contains"] = evaluateContains
	e.evaluators["contains_any"] = evaluateContainsAny
	e.evaluators["min_length"] = evaluateMinLength
	e.evaluators["quality_score"] = evaluateQualityScore
	e.evaluators["reasoning_present"] = evaluateReasoningPresent
	e.evaluators["code_valid"] = evaluateCodeValid
	e.evaluators["min_count"] = evaluateMinCount
	e.evaluators["exact_count"] = evaluateExactCount
	e.evaluators["max_latency"] = evaluateMaxLatency
	e.evaluators["all_valid"] = evaluateAllValid
	e.evaluators["no_duplicates"] = evaluateNoDuplicates
	e.evaluators["all_pass"] = evaluateAllPass
	e.evaluators["no_mock_responses"] = evaluateNoMockResponses
	e.evaluators["min_score"] = evaluateMinScore
}

// Register adds a custom assertion evaluator.
func (e *Engine) Register(assertionType string, evaluator AssertionEvaluator) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.evaluators[assertionType]; exists {
		return fmt.Errorf("assertion type already registered: %s", assertionType)
	}

	e.evaluators[assertionType] = evaluator
	return nil
}

// Evaluate runs an assertion and returns the result.
func (e *Engine) Evaluate(assertion AssertionDefinition, value any) AssertionResult {
	e.mu.RLock()
	evaluator, exists := e.evaluators[assertion.Type]
	e.mu.RUnlock()

	if !exists {
		return AssertionResult{
			Type:    assertion.Type,
			Target:  assertion.Target,
			Passed:  false,
			Message: fmt.Sprintf("unknown assertion type: %s", assertion.Type),
		}
	}

	passed, message := evaluator(assertion, value)

	return AssertionResult{
		Type:     assertion.Type,
		Target:   assertion.Target,
		Expected: assertion.Value,
		Actual:   value,
		Passed:   passed,
		Message:  message,
	}
}

// EvaluateAll runs multiple assertions and returns all results.
func (e *Engine) EvaluateAll(assertions []AssertionDefinition, values map[string]any) []AssertionResult {
	results := make([]AssertionResult, 0, len(assertions))

	for _, assertion := range assertions {
		value, exists := values[assertion.Target]
		if !exists {
			results = append(results, AssertionResult{
				Type:    assertion.Type,
				Target:  assertion.Target,
				Passed:  false,
				Message: fmt.Sprintf("target not found: %s", assertion.Target),
			})
			continue
		}

		results = append(results, e.Evaluate(assertion, value))
	}

	return results
}

// Default assertion evaluators

func evaluateNotEmpty(assertion AssertionDefinition, value any) (bool, string) {
	if value == nil {
		return false, "value is nil"
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return false, "string is empty"
		}
	case []any:
		if len(v) == 0 {
			return false, "array is empty"
		}
	case map[string]any:
		if len(v) == 0 {
			return false, "map is empty"
		}
	}

	return true, "value is not empty"
}

func evaluateNotMock(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return true, "value is not a string"
	}

	mockPatterns := []string{
		"lorem ipsum",
		"placeholder",
		"mock response",
		"TODO",
		"not implemented",
		"[MOCK]",
		"test response",
		"dummy",
		"sample output",
	}

	lower := strings.ToLower(str)
	for _, pattern := range mockPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return false, fmt.Sprintf("response appears to be mocked (contains '%s')", pattern)
		}
	}

	return true, "response is not mocked"
}

func evaluateContains(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return false, "value is not a string"
	}

	expected, ok := assertion.Value.(string)
	if !ok {
		return false, "expected value is not a string"
	}

	if strings.Contains(strings.ToLower(str), strings.ToLower(expected)) {
		return true, fmt.Sprintf("contains '%s'", expected)
	}

	return false, fmt.Sprintf("does not contain '%s'", expected)
}

func evaluateContainsAny(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return false, "value is not a string"
	}

	lower := strings.ToLower(str)

	var values []string
	switch v := assertion.Value.(type) {
	case string:
		values = strings.Split(v, ",")
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
	case []string:
		values = v
	default:
		if assertion.Values != nil {
			for _, item := range assertion.Values {
				if s, ok := item.(string); ok {
					values = append(values, s)
				}
			}
		}
	}

	for _, expected := range values {
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(expected))) {
			return true, fmt.Sprintf("contains '%s'", expected)
		}
	}

	return false, fmt.Sprintf("does not contain any of: %v", values)
}

func evaluateMinLength(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return false, "value is not a string"
	}

	var minLength int
	switch v := assertion.Value.(type) {
	case int:
		minLength = v
	case float64:
		minLength = int(v)
	default:
		return false, "expected value is not a number"
	}

	actual := len(str)
	if actual >= minLength {
		return true, fmt.Sprintf("length %d >= %d", actual, minLength)
	}

	return false, fmt.Sprintf("length %d < %d", actual, minLength)
}

func evaluateQualityScore(assertion AssertionDefinition, value any) (bool, string) {
	score, ok := value.(float64)
	if !ok {
		return false, "value is not a number"
	}

	var minScore float64
	switch v := assertion.Value.(type) {
	case float64:
		minScore = v
	case int:
		minScore = float64(v)
	default:
		return false, "expected value is not a number"
	}

	if score >= minScore {
		return true, fmt.Sprintf("quality score %.2f >= %.2f", score, minScore)
	}

	return false, fmt.Sprintf("quality score %.2f < %.2f", score, minScore)
}

func evaluateReasoningPresent(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return false, "value is not a string"
	}

	indicators := []string{
		"because", "therefore", "since", "thus",
		"step", "first", "then", "next",
		"reason", "explanation", "conclude",
		"let me", "let's",
	}

	lower := strings.ToLower(str)
	for _, indicator := range indicators {
		if strings.Contains(lower, indicator) {
			return true, fmt.Sprintf("reasoning present (found '%s')", indicator)
		}
	}

	return false, "no reasoning indicators found"
}

func evaluateCodeValid(assertion AssertionDefinition, value any) (bool, string) {
	str, ok := value.(string)
	if !ok {
		return false, "value is not a string"
	}

	// Check for code block markers
	hasCodeBlock := strings.Contains(str, "```") || strings.Contains(str, "    ")

	// Check for common code patterns
	codePatterns := []string{
		`func\s+\w+`, // Go function
		`def\s+\w+`,  // Python function
		`class\s+\w+`, // Class definition
		`function\s+\w+`, // JavaScript function
		`=>\s*{`, // Arrow function
		`public\s+\w+`, // Java/C# access modifier
		`import\s+`, // Import statement
		`return\s+`, // Return statement
	}

	for _, pattern := range codePatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(str) {
			return true, "valid code detected"
		}
	}

	if hasCodeBlock {
		return true, "code block present"
	}

	return false, "no valid code detected"
}

func evaluateMinCount(assertion AssertionDefinition, value any) (bool, string) {
	var count int
	switch v := value.(type) {
	case int:
		count = v
	case float64:
		count = int(v)
	case []any:
		count = len(v)
	case map[string]any:
		count = len(v)
	default:
		return false, "value is not countable"
	}

	var minCount int
	switch v := assertion.Value.(type) {
	case int:
		minCount = v
	case float64:
		minCount = int(v)
	default:
		return false, "expected value is not a number"
	}

	if count >= minCount {
		return true, fmt.Sprintf("count %d >= %d", count, minCount)
	}

	return false, fmt.Sprintf("count %d < %d", count, minCount)
}

func evaluateExactCount(assertion AssertionDefinition, value any) (bool, string) {
	var count int
	switch v := value.(type) {
	case int:
		count = v
	case float64:
		count = int(v)
	case []any:
		count = len(v)
	case map[string]any:
		count = len(v)
	default:
		return false, "value is not countable"
	}

	var expectedCount int
	switch v := assertion.Value.(type) {
	case int:
		expectedCount = v
	case float64:
		expectedCount = int(v)
	default:
		return false, "expected value is not a number"
	}

	if count == expectedCount {
		return true, fmt.Sprintf("count %d == %d", count, expectedCount)
	}

	return false, fmt.Sprintf("count %d != %d", count, expectedCount)
}

func evaluateMaxLatency(assertion AssertionDefinition, value any) (bool, string) {
	var latency int64
	switch v := value.(type) {
	case int:
		latency = int64(v)
	case int64:
		latency = v
	case float64:
		latency = int64(v)
	default:
		return false, "value is not a number"
	}

	var maxLatency int64
	switch v := assertion.Value.(type) {
	case int:
		maxLatency = int64(v)
	case int64:
		maxLatency = v
	case float64:
		maxLatency = int64(v)
	default:
		return false, "expected value is not a number"
	}

	if latency <= maxLatency {
		return true, fmt.Sprintf("latency %dms <= %dms", latency, maxLatency)
	}

	return false, fmt.Sprintf("latency %dms > %dms", latency, maxLatency)
}

func evaluateAllValid(assertion AssertionDefinition, value any) (bool, string) {
	items, ok := value.([]any)
	if !ok {
		return false, "value is not an array"
	}

	for i, item := range items {
		if item == nil {
			return false, fmt.Sprintf("item %d is nil", i)
		}
		if str, ok := item.(string); ok && str == "" {
			return false, fmt.Sprintf("item %d is empty", i)
		}
	}

	return true, "all items are valid"
}

func evaluateNoDuplicates(assertion AssertionDefinition, value any) (bool, string) {
	items, ok := value.([]any)
	if !ok {
		return false, "value is not an array"
	}

	seen := make(map[string]bool)
	for _, item := range items {
		key := fmt.Sprintf("%v", item)
		if seen[key] {
			return false, fmt.Sprintf("duplicate found: %s", key)
		}
		seen[key] = true
	}

	return true, "no duplicates found"
}

func evaluateAllPass(assertion AssertionDefinition, value any) (bool, string) {
	results, ok := value.([]AssertionResult)
	if !ok {
		// Try to handle []any
		items, ok := value.([]any)
		if !ok {
			return false, "value is not an array of results"
		}
		for i, item := range items {
			if result, ok := item.(map[string]any); ok {
				if passed, exists := result["passed"]; exists {
					if p, ok := passed.(bool); ok && !p {
						return false, fmt.Sprintf("item %d failed", i)
					}
				}
			}
		}
		return true, "all items passed"
	}

	for _, result := range results {
		if !result.Passed {
			return false, fmt.Sprintf("assertion '%s' failed: %s", result.Type, result.Message)
		}
	}

	return true, "all assertions passed"
}

func evaluateNoMockResponses(assertion AssertionDefinition, value any) (bool, string) {
	responses, ok := value.([]any)
	if !ok {
		// Single response
		return evaluateNotMock(assertion, value)
	}

	for i, resp := range responses {
		if passed, msg := evaluateNotMock(assertion, resp); !passed {
			return false, fmt.Sprintf("response %d: %s", i, msg)
		}
	}

	return true, "no mock responses detected"
}

func evaluateMinScore(assertion AssertionDefinition, value any) (bool, string) {
	return evaluateQualityScore(assertion, value)
}

// ParseAssertionString parses an assertion string like "contains:func" into components.
func ParseAssertionString(s string) (assertionType string, value any) {
	parts := strings.SplitN(s, ":", 2)
	assertionType = parts[0]

	if len(parts) > 1 {
		value = parts[1]
	}

	return
}
