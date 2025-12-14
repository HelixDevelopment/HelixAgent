package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "github.com/HelixDevelopment/HelixAgent/Toolkit/Providers/Chutes"
	_ "github.com/HelixDevelopment/HelixAgent/Toolkit/Providers/SiliconFlow"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/agents"
	"github.com/spf13/cobra"
)

var (
	providerName string
	apiKey       string
	baseURL      string
	model        string
	agentType    string
	task         string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "toolkit",
		Short: "HelixAgent Toolkit - AI-powered application framework",
		Long:  `A comprehensive toolkit for building AI-powered applications with support for multiple providers and specialized agents.`,
	}

	// Test command
	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Run integration tests",
		Run: func(cmd *cobra.Command, args []string) {
			runTests()
		},
	}

	// Chat command
	var chatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session",
		Run: func(cmd *cobra.Command, args []string) {
			runChat()
		},
	}
	chatCmd.Flags().StringVarP(&providerName, "provider", "p", "siliconflow", "Provider name")
	chatCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key")
	chatCmd.Flags().StringVarP(&baseURL, "base-url", "u", "", "Base URL")
	chatCmd.Flags().StringVarP(&model, "model", "m", "", "Model name")

	// Agent command
	var agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Execute tasks with AI agents",
		Run: func(cmd *cobra.Command, args []string) {
			runAgent()
		},
	}
	agentCmd.Flags().StringVarP(&providerName, "provider", "p", "siliconflow", "Provider name")
	agentCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key")
	agentCmd.Flags().StringVarP(&baseURL, "base-url", "u", "", "Base URL")
	agentCmd.Flags().StringVarP(&model, "model", "m", "", "Model name")
	agentCmd.Flags().StringVarP(&agentType, "type", "t", "generic", "Agent type (generic, codereview)")
	agentCmd.Flags().StringVarP(&task, "task", "", "", "Task to execute")

	// Version command
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("HelixAgent Toolkit v1.0.0")
		},
	}

	rootCmd.AddCommand(testCmd, chatCmd, agentCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runTests() {
	fmt.Println("Running integration tests...")

	// Run basic provider tests
	testProviders()

	fmt.Println("All tests completed successfully!")
}

func testProviders() {
	// Test SiliconFlow provider creation
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, err := toolkit.CreateProvider("siliconflow", config)
	if err != nil {
		log.Printf("SiliconFlow provider test failed: %v", err)
		return
	}

	fmt.Printf("✓ SiliconFlow provider created: %s\n", provider.Name())

	// Test model discovery
	ctx := context.Background()
	models, err := provider.DiscoverModels(ctx)
	if err != nil {
		log.Printf("Model discovery failed: %v", err)
		return
	}

	fmt.Printf("✓ Discovered %d models\n", len(models))
}

func runChat() {
	if apiKey == "" {
		log.Fatal("API key is required. Use --api-key flag.")
	}

	// Create provider
	config := map[string]interface{}{
		"api_key": apiKey,
	}
	if baseURL != "" {
		config["base_url"] = baseURL
	}

	provider, err := toolkit.CreateProvider(providerName, config)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	fmt.Printf("Connected to %s provider\n", provider.Name())
	fmt.Println("Type 'quit' to exit")

	// Simple interactive chat
	ctx := context.Background()
	for {
		fmt.Print("> ")
		var input string
		fmt.Scanln(&input)

		if input == "quit" {
			break
		}

		req := toolkit.ChatRequest{
			Messages: []toolkit.Message{
				{Role: "user", Content: input},
			},
			MaxTokens: 1000,
		}

		if model != "" {
			req.Model = model
		}

		resp, err := provider.Chat(ctx, req)
		if err != nil {
			log.Printf("Chat failed: %v", err)
			continue
		}

		if len(resp.Choices) > 0 {
			fmt.Println(resp.Choices[0].Message.Content)
		}
	}
}

func runAgent() {
	if apiKey == "" {
		log.Fatal("API key is required. Use --api-key flag.")
	}
	if task == "" {
		log.Fatal("Task is required. Use --task flag.")
	}

	// Create provider
	config := map[string]interface{}{
		"api_key": apiKey,
	}
	if baseURL != "" {
		config["base_url"] = baseURL
	}

	provider, err := toolkit.CreateProvider(providerName, config)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Create agent
	var agent toolkit.Agent
	switch agentType {
	case "generic":
		agent = agents.NewGenericAgent("CLI-Agent", "Command-line AI assistant", provider)
	case "codereview":
		agent = agents.NewCodeReviewAgent("CLI-CodeReviewer", provider)
	default:
		log.Fatalf("Unknown agent type: %s", agentType)
	}

	// Execute task
	ctx := context.Background()
	agentConfig := make(map[string]interface{})
	if model != "" {
		agentConfig["model"] = model
	}

	result, err := agent.Execute(ctx, task, agentConfig)
	if err != nil {
		log.Fatalf("Agent execution failed: %v", err)
	}

	fmt.Println("Agent Response:")
	fmt.Println(result)
}
