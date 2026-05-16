package agents

import (
	"context"
	"testing"

	testutil "github.com/HelixDevelopment/helix_agent/Toolkit/Commons/testing"
	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

func TestNewGenericAgent(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	if agent == nil {
		t.Fatal("NewGenericAgent returned nil")
	}
	if agent.Name() != "TestAgent" {
		t.Errorf("Expected name 'TestAgent', got '%s'", agent.Name())
	}
	if agent.provider != mockProvider {
		t.Error("Provider not set correctly")
	}
}

func TestGenericAgent_Execute(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	mockProvider.SetChatResponse(toolkit.ChatResponse{
		Choices: []toolkit.Choice{
			{
				Message: toolkit.Message{
					Role:    "assistant",
					Content: "Test response",
				},
			},
		},
	})

	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	result, err := agent.Execute(context.Background(), "Test task", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", result)
	}
}

func TestGenericAgent_Execute_WithConfig(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	mockProvider.SetChatResponse(toolkit.ChatResponse{
		Choices: []toolkit.Choice{
			{
				Message: toolkit.Message{
					Role:    "assistant",
					Content: "Configured response",
				},
			},
		},
	})

	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	config := map[string]interface{}{
		"model":      "test-model",
		"max_tokens": 500,
	}

	result, err := agent.Execute(context.Background(), "Test task", config)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "Configured response" {
		t.Errorf("Expected 'Configured response', got '%s'", result)
	}
}

func TestGenericAgent_ValidateConfig(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	// Valid config
	config := map[string]interface{}{
		"model":      "test-model",
		"max_tokens": 500,
	}
	err := agent.ValidateConfig(config)
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}

	// Invalid config key
	config = map[string]interface{}{
		"invalid_key": "value",
	}
	err = agent.ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for invalid config key")
	}

	// Nil config
	err = agent.ValidateConfig(nil)
	if err != nil {
		t.Errorf("Expected no error for nil config, got: %v", err)
	}
}

func TestGenericAgent_Capabilities(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	caps := agent.Capabilities()
	expected := []string{"chat", "task_execution", "general_assistance"}

	if len(caps) != len(expected) {
		t.Fatalf("Expected %d capabilities, got %d", len(expected), len(caps))
	}

	for i, cap := range caps {
		if cap != expected[i] {
			t.Errorf("Expected capability '%s', got '%s'", expected[i], cap)
		}
	}
}

func TestGenericAgent_Config(t *testing.T) {
	mockProvider := testutil.NewMockProvider("mock")
	agent := NewGenericAgent("TestAgent", "A test agent", mockProvider)

	// Set config
	agent.SetConfig("test_key", "test_value")

	// Get config
	value := agent.GetConfig("test_key")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", value)
	}

	// Get non-existent config
	value = agent.GetConfig("non_existent")
	if value != nil {
		t.Errorf("Expected nil for non-existent key, got '%v'", value)
	}
}
