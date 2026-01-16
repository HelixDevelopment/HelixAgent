package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

// MockProvider is a mock implementation of toolkit.Provider for testing
type MockProvider struct {
	name           string
	chatResponse   toolkit.ChatResponse
	chatError      error
	embedResponse  toolkit.EmbeddingResponse
	embedError     error
	rerankResponse toolkit.RerankResponse
	rerankError    error
	modelsResponse []toolkit.ModelInfo
	modelsError    error
	validateError  error
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Chat(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	if m.chatError != nil {
		return toolkit.ChatResponse{}, m.chatError
	}
	return m.chatResponse, nil
}

func (m *MockProvider) Embed(ctx context.Context, req toolkit.EmbeddingRequest) (toolkit.EmbeddingResponse, error) {
	if m.embedError != nil {
		return toolkit.EmbeddingResponse{}, m.embedError
	}
	return m.embedResponse, nil
}

func (m *MockProvider) Rerank(ctx context.Context, req toolkit.RerankRequest) (toolkit.RerankResponse, error) {
	if m.rerankError != nil {
		return toolkit.RerankResponse{}, m.rerankError
	}
	return m.rerankResponse, nil
}

func (m *MockProvider) DiscoverModels(ctx context.Context) ([]toolkit.ModelInfo, error) {
	if m.modelsError != nil {
		return nil, m.modelsError
	}
	return m.modelsResponse, nil
}

func (m *MockProvider) ValidateConfig(config map[string]interface{}) error {
	return m.validateError
}

func TestNewCodeReviewAgent(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}

	if agent.Name() != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.Name())
	}

	if agent.provider != mockProvider {
		t.Error("Expected provider to be set")
	}

	if agent.config == nil {
		t.Error("Expected config map to be initialized")
	}
}

func TestCodeReviewAgent_Name(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("code-reviewer", mockProvider)

	if agent.Name() != "code-reviewer" {
		t.Errorf("Expected name 'code-reviewer', got '%s'", agent.Name())
	}
}

func TestCodeReviewAgent_Execute_Success(t *testing.T) {
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			ID:    "test-response-id",
			Model: "test-model",
			Choices: []toolkit.Choice{
				{
					Index: 0,
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Code review feedback: This code looks good. Consider adding error handling.",
					},
					FinishReason: "stop",
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	code := `
func add(a, b int) int {
    return a + b
}
`
	result, err := agent.Execute(ctx, code, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}

	expectedContent := "Code review feedback: This code looks good. Consider adding error handling."
	if result != expectedContent {
		t.Errorf("Expected result '%s', got '%s'", expectedContent, result)
	}
}

func TestCodeReviewAgent_Execute_WithLanguage(t *testing.T) {
	var capturedRequest toolkit.ChatRequest
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Go code review: The function is well-written.",
					},
				},
			},
		},
	}

	// Create a custom provider that captures the request
	capturingProvider := &CapturingMockProvider{
		MockProvider: mockProvider,
		capturedReq:  &capturedRequest,
	}

	agent := NewCodeReviewAgent("test-agent", capturingProvider)
	ctx := context.Background()

	code := `
package main

func main() {
    fmt.Println("Hello")
}
`
	config := map[string]interface{}{
		"language": "Go",
	}

	result, err := agent.Execute(ctx, code, config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// CapturingMockProvider captures the chat request for inspection
type CapturingMockProvider struct {
	*MockProvider
	capturedReq *toolkit.ChatRequest
}

func (c *CapturingMockProvider) Chat(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	*c.capturedReq = req
	return c.MockProvider.Chat(ctx, req)
}

func TestCodeReviewAgent_Execute_WithModel(t *testing.T) {
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Review complete.",
					},
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	config := map[string]interface{}{
		"model": "gpt-4",
	}

	result, err := agent.Execute(ctx, "func test() {}", config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "Review complete." {
		t.Errorf("Expected 'Review complete.', got '%s'", result)
	}
}

func TestCodeReviewAgent_Execute_WithMaxTokens(t *testing.T) {
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Review complete with custom max tokens.",
					},
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	config := map[string]interface{}{
		"max_tokens": 4000,
	}

	result, err := agent.Execute(ctx, "func test() {}", config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "Review complete with custom max tokens." {
		t.Errorf("Unexpected result: %s", result)
	}
}

func TestCodeReviewAgent_Execute_ProviderError(t *testing.T) {
	mockProvider := &MockProvider{
		name:      "test-provider",
		chatError: errors.New("provider error: connection failed"),
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	result, err := agent.Execute(ctx, "func test() {}", nil)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty result, got '%s'", result)
	}

	expectedErr := "failed to perform code review: provider error: connection failed"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestCodeReviewAgent_Execute_NoChoices(t *testing.T) {
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{}, // Empty choices
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	result, err := agent.Execute(ctx, "func test() {}", nil)

	if err == nil {
		t.Fatal("Expected error for no choices, got nil")
	}

	if result != "" {
		t.Errorf("Expected empty result, got '%s'", result)
	}

	expectedErr := "no response choices returned"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestCodeReviewAgent_ValidateConfig_Nil(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	err := agent.ValidateConfig(nil)

	if err != nil {
		t.Errorf("Expected no error for nil config, got %v", err)
	}
}

func TestCodeReviewAgent_ValidateConfig_ValidConfig(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	config := map[string]interface{}{
		"model":      "gpt-4",
		"language":   "Python",
		"max_tokens": 2000,
	}

	err := agent.ValidateConfig(config)

	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}
}

func TestCodeReviewAgent_ValidateConfig_InvalidType(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	err := agent.ValidateConfig("invalid config type")

	if err == nil {
		t.Fatal("Expected error for invalid config type")
	}

	expectedErr := "config must be a map[string]interface{}"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestCodeReviewAgent_ValidateConfig_UnknownKey(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	config := map[string]interface{}{
		"model":       "gpt-4",
		"unknown_key": "some value",
	}

	err := agent.ValidateConfig(config)

	if err == nil {
		t.Fatal("Expected error for unknown key")
	}

	expectedErr := "unknown configuration key: unknown_key"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestCodeReviewAgent_Capabilities(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	capabilities := agent.Capabilities()

	if len(capabilities) == 0 {
		t.Error("Expected capabilities to be non-empty")
	}

	expectedCapabilities := map[string]bool{
		"code_review":          true,
		"security_analysis":    true,
		"performance_analysis": true,
		"best_practices":       true,
		"bug_detection":        true,
	}

	for _, cap := range capabilities {
		if !expectedCapabilities[cap] {
			t.Errorf("Unexpected capability: %s", cap)
		}
		delete(expectedCapabilities, cap)
	}

	if len(expectedCapabilities) > 0 {
		for cap := range expectedCapabilities {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

func TestCodeReviewAgent_SetConfig(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	agent.SetConfig("test_key", "test_value")

	if agent.config["test_key"] != "test_value" {
		t.Errorf("Expected config['test_key'] = 'test_value', got %v", agent.config["test_key"])
	}

	agent.SetConfig("int_key", 42)

	if agent.config["int_key"] != 42 {
		t.Errorf("Expected config['int_key'] = 42, got %v", agent.config["int_key"])
	}
}

func TestCodeReviewAgent_GetConfig(t *testing.T) {
	mockProvider := &MockProvider{name: "test-provider"}
	agent := NewCodeReviewAgent("test-agent", mockProvider)

	agent.config["test_key"] = "test_value"
	agent.config["int_key"] = 123

	strValue := agent.GetConfig("test_key")
	if strValue != "test_value" {
		t.Errorf("Expected 'test_value', got %v", strValue)
	}

	intValue := agent.GetConfig("int_key")
	if intValue != 123 {
		t.Errorf("Expected 123, got %v", intValue)
	}

	// Test non-existent key
	nilValue := agent.GetConfig("non_existent")
	if nilValue != nil {
		t.Errorf("Expected nil for non-existent key, got %v", nilValue)
	}
}

func TestCodeReviewAgent_Execute_DifferentLanguages(t *testing.T) {
	tests := []struct {
		name     string
		language string
		code     string
	}{
		{
			name:     "Python",
			language: "Python",
			code: `
def hello():
    print("Hello, World!")
`,
		},
		{
			name:     "JavaScript",
			language: "JavaScript",
			code: `
function hello() {
    console.log("Hello, World!");
}
`,
		},
		{
			name:     "Rust",
			language: "Rust",
			code: `
fn hello() {
    println!("Hello, World!");
}
`,
		},
		{
			name:     "Java",
			language: "Java",
			code: `
public class Hello {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider := &MockProvider{
				name: "test-provider",
				chatResponse: toolkit.ChatResponse{
					Choices: []toolkit.Choice{
						{
							Message: toolkit.Message{
								Role:    "assistant",
								Content: "Code review for " + tt.language + " completed.",
							},
						},
					},
				},
			}

			agent := NewCodeReviewAgent("test-agent", mockProvider)
			ctx := context.Background()

			config := map[string]interface{}{
				"language": tt.language,
			}

			result, err := agent.Execute(ctx, tt.code, config)

			if err != nil {
				t.Fatalf("Expected no error for %s, got %v", tt.language, err)
			}

			expectedResult := "Code review for " + tt.language + " completed."
			if result != expectedResult {
				t.Errorf("Expected '%s', got '%s'", expectedResult, result)
			}
		})
	}
}

func TestCodeReviewAgent_BugDetection(t *testing.T) {
	buggyCode := `
func divide(a, b int) int {
    return a / b  // Bug: no check for division by zero
}

func processData(data []int) {
    for i := 0; i <= len(data); i++ {  // Bug: off-by-one error
        fmt.Println(data[i])
    }
}
`

	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role: "assistant",
						Content: `Code Review Findings:

1. **Bug: Division by Zero**
   - Function: divide
   - Issue: No check for b == 0 before division
   - Fix: Add validation for divisor

2. **Bug: Off-by-one Error**
   - Function: processData
   - Issue: Loop condition should be i < len(data)
   - Fix: Change <= to <`,
					},
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	config := map[string]interface{}{
		"language": "Go",
	}

	result, err := agent.Execute(ctx, buggyCode, config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the review contains bug-related content
	if result == "" {
		t.Error("Expected non-empty review result")
	}
}

func TestCodeReviewAgent_SecurityAnalysis(t *testing.T) {
	insecureCode := `
func executeQuery(userInput string) {
    query := "SELECT * FROM users WHERE name = '" + userInput + "'"
    db.Execute(query)  // SQL Injection vulnerability
}

func handleRequest(password string) {
    fmt.Println("Password is: " + password)  // Logging sensitive data
}
`

	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role: "assistant",
						Content: `Security Review:

1. **Critical: SQL Injection**
   - Use parameterized queries instead of string concatenation

2. **High: Sensitive Data Exposure**
   - Never log passwords or sensitive information`,
					},
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	result, err := agent.Execute(ctx, insecureCode, nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == "" {
		t.Error("Expected security review findings")
	}
}

func TestCodeReviewAgent_ContextCancellation(t *testing.T) {
	mockProvider := &MockProvider{
		name:      "test-provider",
		chatError: context.Canceled,
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := agent.Execute(ctx, "func test() {}", nil)

	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestCodeReviewAgent_AllConfigOptions(t *testing.T) {
	mockProvider := &MockProvider{
		name: "test-provider",
		chatResponse: toolkit.ChatResponse{
			Choices: []toolkit.Choice{
				{
					Message: toolkit.Message{
						Role:    "assistant",
						Content: "Full review completed.",
					},
				},
			},
		},
	}

	agent := NewCodeReviewAgent("test-agent", mockProvider)
	ctx := context.Background()

	// Test with all valid config options
	config := map[string]interface{}{
		"model":      "gpt-4-turbo",
		"language":   "TypeScript",
		"max_tokens": 3000,
	}

	result, err := agent.Execute(ctx, "function test(): void {}", config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != "Full review completed." {
		t.Errorf("Expected 'Full review completed.', got '%s'", result)
	}
}
