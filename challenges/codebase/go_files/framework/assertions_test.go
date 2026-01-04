package framework

import (
	"testing"
)

func TestAssertionEngine_NotEmpty(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"non-empty string", "hello", true},
		{"empty string", "", false},
		{"whitespace string", "   ", false},
		{"nil value", nil, false},
		{"non-empty array", []any{"a"}, true},
		{"empty array", []any{}, false},
		{"non-empty map", map[string]any{"key": "value"}, true},
		{"empty map", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{Type: "not_empty"}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("not_empty(%v) = %v, want %v", tt.value, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_NotMock(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"real response", "Here is a Go function to calculate factorial", true},
		{"lorem ipsum", "Lorem ipsum dolor sit amet", false},
		{"placeholder", "This is a placeholder response", false},
		{"mock response", "Mock response for testing", false},
		{"TODO", "TODO: implement this later", false},
		{"case insensitive", "LOREM IPSUM", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{Type: "not_mock"}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("not_mock(%q) = %v, want %v (%s)", tt.value, result.Passed, tt.expected, result.Message)
			}
		})
	}
}

func TestAssertionEngine_Contains(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    string
		contains string
		expected bool
	}{
		{"contains exact", "func main() {}", "func", true},
		{"contains case insensitive", "Hello World", "hello", true},
		{"does not contain", "Hello World", "goodbye", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "contains",
				Value: tt.contains,
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.value, tt.contains, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_ContainsAny(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    string
		contains string
		expected bool
	}{
		{"contains first", "The dog ran away", "dog,cat", true},
		{"contains second", "The cat ran away", "dog,cat", true},
		{"contains none", "The bird flew away", "dog,cat", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "contains_any",
				Value: tt.contains,
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("contains_any(%q, %q) = %v, want %v", tt.value, tt.contains, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_MinLength(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name      string
		value     string
		minLength int
		expected  bool
	}{
		{"meets minimum", "hello world", 5, true},
		{"exact minimum", "hello", 5, true},
		{"below minimum", "hi", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "min_length",
				Value: tt.minLength,
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("min_length(%q, %d) = %v, want %v", tt.value, tt.minLength, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_QualityScore(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    float64
		minScore float64
		expected bool
	}{
		{"meets threshold", 0.85, 0.8, true},
		{"exact threshold", 0.8, 0.8, true},
		{"below threshold", 0.7, 0.8, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "quality_score",
				Value: tt.minScore,
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("quality_score(%.2f, %.2f) = %v, want %v", tt.value, tt.minScore, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_ReasoningPresent(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"has because", "The answer is 42 because it's the meaning of life", true},
		{"has therefore", "Therefore, the result is correct", true},
		{"has step", "First step is to initialize", true},
		{"no reasoning", "The answer is 42", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{Type: "reasoning_present"}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("reasoning_present(%q) = %v, want %v", tt.value, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_CodeValid(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"go function", "func main() { fmt.Println() }", true},
		{"python function", "def hello():\n    print('hi')", true},
		{"class definition", "class MyClass { constructor() {} }", true},
		{"code block", "```go\nfunc main() {}\n```", true},
		{"plain text", "Hello, this is just text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{Type: "code_valid"}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("code_valid(%q) = %v, want %v (%s)", tt.value, result.Passed, tt.expected, result.Message)
			}
		})
	}
}

func TestAssertionEngine_MinCount(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    any
		minCount int
		expected bool
	}{
		{"array meets", []any{"a", "b", "c"}, 2, true},
		{"array below", []any{"a"}, 2, false},
		{"int value", 5, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "min_count",
				Value: tt.minCount,
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("min_count = %v, want %v", result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_NoDuplicates(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name     string
		value    []any
		expected bool
	}{
		{"no duplicates", []any{"a", "b", "c"}, true},
		{"has duplicates", []any{"a", "b", "a"}, false},
		{"empty", []any{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{Type: "no_duplicates"}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("no_duplicates = %v, want %v", result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_MaxLatency(t *testing.T) {
	engine := NewAssertionEngine()

	tests := []struct {
		name       string
		value      int64
		maxLatency int64
		expected   bool
	}{
		{"within limit", 1000, 5000, true},
		{"at limit", 5000, 5000, true},
		{"exceeds limit", 6000, 5000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Evaluate(AssertionDefinition{
				Type:  "max_latency",
				Value: float64(tt.maxLatency),
			}, tt.value)
			if result.Passed != tt.expected {
				t.Errorf("max_latency(%d, %d) = %v, want %v", tt.value, tt.maxLatency, result.Passed, tt.expected)
			}
		})
	}
}

func TestAssertionEngine_EvaluateAll(t *testing.T) {
	engine := NewAssertionEngine()

	assertions := []AssertionDefinition{
		{Type: "not_empty", Target: "response"},
		{Type: "contains", Target: "response", Value: "func"},
		{Type: "min_length", Target: "response", Value: 10},
	}

	values := map[string]any{
		"response": "func main() {}",
	}

	results := engine.EvaluateAll(assertions, values)

	if len(results) != 3 {
		t.Errorf("EvaluateAll() returned %d results, want 3", len(results))
	}

	// All should pass
	for _, r := range results {
		if !r.Passed {
			t.Errorf("Assertion %s failed: %s", r.Type, r.Message)
		}
	}
}

func TestAssertionEngine_CustomEvaluator(t *testing.T) {
	engine := NewAssertionEngine()

	// Register custom evaluator
	err := engine.Register("is_uppercase", func(assertion AssertionDefinition, value any) (bool, string) {
		str, ok := value.(string)
		if !ok {
			return false, "value is not a string"
		}
		if str == "" {
			return false, "string is empty"
		}
		for _, r := range str {
			if r >= 'a' && r <= 'z' {
				return false, "contains lowercase characters"
			}
		}
		return true, "string is uppercase"
	})

	if err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	// Test custom evaluator
	result := engine.Evaluate(AssertionDefinition{Type: "is_uppercase"}, "HELLO")
	if !result.Passed {
		t.Errorf("is_uppercase(HELLO) should pass")
	}

	result = engine.Evaluate(AssertionDefinition{Type: "is_uppercase"}, "Hello")
	if result.Passed {
		t.Errorf("is_uppercase(Hello) should fail")
	}

	// Duplicate registration should fail
	err = engine.Register("is_uppercase", func(AssertionDefinition, any) (bool, string) { return false, "" })
	if err == nil {
		t.Error("Register() should fail for duplicate")
	}
}

func TestParseAssertionString(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
		expectedVal  string
	}{
		{"not_empty", "not_empty", ""},
		{"contains:func", "contains", "func"},
		{"min_length:100", "min_length", "100"},
		{"contains_any:yes,true", "contains_any", "yes,true"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			aType, val := ParseAssertionString(tt.input)
			if aType != tt.expectedType {
				t.Errorf("type = %s, want %s", aType, tt.expectedType)
			}
			valStr, _ := val.(string)
			if tt.expectedVal != "" && valStr != tt.expectedVal {
				t.Errorf("value = %v, want %s", val, tt.expectedVal)
			}
		})
	}
}
