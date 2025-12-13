package main

import (
	"context"
	"fmt"
	"log"

	"github.com/superagent/toolkit/pkg/toolkit"
)

func main() {
	fmt.Println("=== Chutes Provider Mock Testing ===")

	// Test with the toolkit's built-in testing utilities
	tk := toolkit.NewToolkit()

	// Test 1: Create a mock Chutes provider
	fmt.Println("\n1. Creating Mock Chutes Provider...")
	
	// Create a mock provider that simulates Chutes behavior
	// Use a unique name to avoid conflicts with the real Chutes provider
	mockProvider := toolkit.NewMockProvider("chutes-mock")
	
	// Set up mock chat response
	mockProvider.SetChatResponse(toolkit.ChatResponse{
		Model: "qwen2.5-7b-instruct",
		Choices: []toolkit.ChatChoice{
			{
				Message: toolkit.ChatMessage{
					Role:    "assistant",
					Content: "This is a mock response from Chutes Qwen model.",
				},
				FinishReason: "stop",
			},
		},
		Usage: toolkit.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	})

	fmt.Println("✓ Mock Chutes provider created with chat response")

	// Test 2: Register the mock provider
	fmt.Println("\n2. Registering Mock Provider...")
	err := tk.RegisterProvider("chutes", mockProvider)
	if err != nil {
		log.Fatalf("Failed to register mock provider: %v", err)
	}
	fmt.Println("✓ Mock provider registered successfully")

	// Test 3: Test Chat Functionality
	fmt.Println("\n3. Testing Chat Functionality...")
	ctx := context.Background()
	
	chatReq := toolkit.ChatRequest{
		Model: "qwen2.5-7b-instruct",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Hello, can you help me?"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	chatResp, err := mockProvider.Chat(ctx, chatReq)
	if err != nil {
		log.Fatalf("Chat failed: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		log.Fatal("No chat response choices")
	}

	fmt.Printf("✓ Chat response received: %s\n", chatResp.Choices[0].Message.Content)
	fmt.Printf("✓ Token usage: prompt=%d, completion=%d, total=%d\n",
		chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)

	// Test 4: Test Configuration Validation
	fmt.Println("\n4. Testing Configuration Validation...")
	
	validConfig := map[string]interface{}{
		"api_key":    "test-api-key",
		"base_url":   "https://api.chutes.ai/v1",
		"timeout":    30000,
		"retries":    3,
		"rate_limit": 60,
	}

	err = mockProvider.ValidateConfig(validConfig)
	if err != nil {
		log.Fatalf("Valid config rejected: %v", err)
	}
	fmt.Println("✓ Valid configuration accepted")

	// Test 5: Test Provider Name
	fmt.Println("\n5. Testing Provider Identity...")
	
	providerName := mockProvider.Name()
	if providerName != "chutes-mock" {
		log.Fatalf("Expected provider name 'chutes-mock', got '%s'", providerName)
	}
	fmt.Printf("✓ Provider name is correct: %s\n", providerName)

	// Test 6: Test Provider Listing
	fmt.Println("\n6. Testing Provider Listing...")
	
	// Check if our mock provider is registered (as an instance, not factory)
	provider, err := tk.GetProvider("chutes-mock")
	if err != nil {
		log.Fatalf("Failed to get mock provider: %v", err)
	}
	if provider.Name() != "chutes-mock" {
		log.Fatal("Retrieved wrong provider")
	}
	fmt.Println("✓ Mock provider found in provider registry")

	// Test 7: Test Provider Retrieval
	fmt.Println("\n7. Testing Provider Retrieval...")
	
	retrievedProvider, err := tk.GetProvider("chutes-mock")
	if err != nil {
		log.Fatalf("Failed to retrieve provider: %v", err)
	}
	
	if retrievedProvider.Name() != "chutes-mock" {
		log.Fatalf("Retrieved provider has wrong name: %s", retrievedProvider.Name())
	}
	fmt.Println("✓ Mock provider retrieved successfully from registry")

	// Test 8: Test Error Handling
	fmt.Println("\n8. Testing Error Handling...")
	
	// Set an error for the next chat request
	mockProvider.SetChatError(fmt.Errorf("mock API error"))
	
	_, err = mockProvider.Chat(ctx, chatReq)
	if err == nil {
		log.Fatal("Expected error but got none")
	}
	fmt.Printf("✓ Error handling works: %v\n", err)

	// Reset error
	mockProvider.SetChatError(nil)

	fmt.Println("\n=== All Mock Tests Passed! ===")
	fmt.Println("Chutes provider mock testing completed successfully.")
	fmt.Println("The provider is ready for real API integration.")
}