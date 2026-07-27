//go:build ignore

// Package main demonstrates sub-agent usage
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"dev.helix.agent/internal/agents/subagent"
	"dev.helix.agent/internal/llm"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fmt.Println("=== HelixAgent Sub-Agent Example ===")
	fmt.Println()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY or ANTHROPIC_API_KEY")
		return
	}

	providerType := "openai"
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		providerType = "anthropic"
	}

	// Create sub-agent manager
	manager := subagent.NewManager(&subagent.Config{
		ProviderType: providerType,
		APIKey:       apiKey,
		Logger:       logger,
	})

	ctx := context.Background()

	// Example 1: Explore agent for discovery
	fmt.Println("--- Explore Agent ---")
	task := "Research the best practices for error handling in Go"

	exploreAgent, err := manager.CreateAgent(ctx, "explore", subagent.ProfileConfig{
		Name:        "code-explorer",
		Model:       "gpt-4o-mini",
		MaxTokens:   2000,
		Temperature: 0.7,
		Tools:       []string{"file_read", "web_search"},
	})
	if err != nil {
		fmt.Printf("Error creating explore agent: %v\n", err)
		return
	}

	fmt.Printf("Task: %s\n", task)
	start := time.Now()

	exploreResult, err := exploreAgent.Execute(ctx, subagent.Task{
		Description: task,
		MaxSteps:    10,
	})
	if err != nil {
		fmt.Printf("Explore error: %v\n", err)
		return
	}

	fmt.Printf("Discoveries (%d):\n", len(exploreResult.Discoveries))
	for i, d := range exploreResult.Discoveries {
		fmt.Printf("  %d. %s\n", i+1, d)
	}
	fmt.Printf("Files examined: %d\n", len(exploreResult.FilesExamined))
	fmt.Printf("Explore latency: %v\n\n", time.Since(start))

	// Example 2: Plan agent for solution design
	fmt.Println("--- Plan Agent ---")

	planAgent, err := manager.CreateAgent(ctx, "plan", subagent.ProfileConfig{
		Name:        "solution-planner",
		Model:       "gpt-4o",
		MaxTokens:   3000,
		Temperature: 0.5,
	})
	if err != nil {
		fmt.Printf("Error creating plan agent: %v\n", err)
		return
	}

	planInput := subagent.PlanInput{
		Objective:   "Implement a robust error handling system for a Go web service",
		Discoveries: exploreResult.Discoveries,
		Constraints: []string{
			"Must use structured logging",
			"Must be compatible with standard library errors",
			"Must support error wrapping",
		},
	}

	start = time.Now()
	planResult, err := planAgent.CreatePlan(ctx, planInput)
	if err != nil {
		fmt.Printf("Plan error: %v\n", err)
		return
	}

	fmt.Printf("Plan created with %d steps:\n", len(planResult.Steps))
	for i, step := range planResult.Steps {
		fmt.Printf("  %d. %s (%s)\n", i+1, step.Description, step.Priority)
	}
	fmt.Printf("Files to create: %d\n", len(planResult.FilesToCreate))
	fmt.Printf("Files to modify: %d\n", len(planResult.FilesToModify))
	fmt.Printf("Plan latency: %v\n\n", time.Since(start))

	// Example 3: General agent for implementation
	fmt.Println("--- General Agent ---")

	generalAgent, err := manager.CreateAgent(ctx, "general", subagent.ProfileConfig{
		Name:        "code-impl",
		Model:       "gpt-4o",
		MaxTokens:   4000,
		Temperature: 0.3,
		Tools:       []string{"file_write", "file_read", "shell"},
	})
	if err != nil {
		fmt.Printf("Error creating general agent: %v\n", err)
		return
	}

	// Execute the plan
	start = time.Now()
	implResult, err := generalAgent.ExecutePlan(ctx, planResult)
	if err != nil {
		fmt.Printf("Implementation error: %v\n", err)
		return
	}

	fmt.Printf("Implementation complete!\n")
	fmt.Printf("Files written: %d\n", len(implResult.FilesWritten))
	fmt.Printf("Commands executed: %d\n", len(implResult.CommandsExecuted))
	if implResult.Error != "" {
		fmt.Printf("Error encountered: %s\n", implResult.Error)
	}
	fmt.Printf("Implementation latency: %v\n\n", time.Since(start))

	// Example 4: Parallel sub-agents for concurrent tasks
	fmt.Println("--- Parallel Sub-Agents ---")

	tasks := []string{
		"Find examples of error wrapping in Go",
		"Research structured logging libraries",
		"Look for panic recovery patterns",
	}

	start = time.Now()
	results := make(chan subagent.TaskResult, len(tasks))

	for i, task := range tasks {
		go func(idx int, t string) {
			agent, _ := manager.CreateAgent(ctx, "explore", subagent.ProfileConfig{
				Name:        fmt.Sprintf("explorer-%d", idx),
				Model:       "gpt-4o-mini",
				MaxTokens:   1000,
				Temperature: 0.7,
			})

			result, err := agent.Execute(ctx, subagent.Task{
				Description: t,
				MaxSteps:    5,
			})
			results <- subagent.TaskResult{Result: result, Error: err}
		}(i, task)
	}

	for i := 0; i < len(tasks); i++ {
		result := <-results
		if result.Error != nil {
			fmt.Printf("  Task %d failed: %v\n", i+1, result.Error)
		} else {
			fmt.Printf("  Task %d complete: %d discoveries\n", i+1, len(result.Result.Discoveries))
		}
	}

	fmt.Printf("Parallel execution latency: %v\n", time.Since(start))

	// Cleanup
	manager.Shutdown(ctx)
	fmt.Println("\nAll agents shutdown successfully")
}
