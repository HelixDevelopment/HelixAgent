//go:build ignore

// Package main demonstrates function/tool calling
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"dev.helix.agent/internal/llm"
	"go.uber.org/zap"
)

// Calculator functions
func add(a, b float64) float64 { return a + b }
func sub(a, b float64) float64 { return a - b }
func mul(a, b float64) float64 { return a * b }
func div(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("cannot divide by zero")
	}
	return a / b, nil
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fmt.Println("=== HelixAgent Tool Calling Example ===")
	fmt.Println()

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY")
		return
	}

	client := llm.NewClient("openai", apiKey, logger)
	ctx := context.Background()

	// Define available tools
	tools := []llm.ToolDefinition{
		{
			Name:        "add",
			Description: "Add two numbers",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]string{"type": "number", "description": "First number"},
					"b": map[string]string{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
		},
		{
			Name:        "subtract",
			Description: "Subtract second number from first",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]string{"type": "number", "description": "First number"},
					"b": map[string]string{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
		},
		{
			Name:        "multiply",
			Description: "Multiply two numbers",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]string{"type": "number", "description": "First number"},
					"b": map[string]string{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
		},
		{
			Name:        "divide",
			Description: "Divide first number by second",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]string{"type": "number", "description": "First number"},
					"b": map[string]string{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
		},
	}

	queries := []string{
		"What is 42 + 58?",
		"Calculate 100 divided by 4",
		"Multiply 7 by 8",
		"What is 25 - 17?",
	}

	for _, query := range queries {
		fmt.Printf("Query: %s\n", query)

		start := time.Now()
		response, err := client.Chat(ctx, llm.ChatRequest{
			Model:     "gpt-4o-mini",
			Messages:  []llm.Message{{Role: "user", Content: query}},
			Tools:     tools,
			MaxTokens: 200,
		})
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		latency := time.Since(start)

		if len(response.ToolCalls) > 0 {
			for _, call := range response.ToolCalls {
				fmt.Printf("  Tool called: %s\n", call.Name)
				fmt.Printf("  Arguments: %s\n", call.Arguments)

				// Execute the tool
				var args struct {
					A float64 `json:"a"`
					B float64 `json:"b"`
				}
				if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
					fmt.Printf("  Error parsing args: %v\n", err)
					continue
				}

				var result float64
				var err error
				switch call.Name {
				case "add":
					result = add(args.A, args.B)
				case "subtract":
					result = sub(args.A, args.B)
				case "multiply":
					result = mul(args.A, args.B)
				case "divide":
					result, err = div(args.A, args.B)
				}

				if err != nil {
					fmt.Printf("  Error: %v\n", err)
				} else {
					fmt.Printf("  Result: %.2f\n", result)
				}
			}
		} else {
			fmt.Printf("  Direct response: %s\n", response.Content)
		}
		fmt.Printf("  Latency: %v\n\n", latency)
	}

	// Example with multiple tools in sequence
	fmt.Println("--- Multi-step calculation ---")
	complexQuery := "If I have 100 apples and give away 30, then buy 25 more, how many do I have?"
	fmt.Printf("Query: %s\n", complexQuery)

	messages := []llm.Message{{Role: "user", Content: complexQuery}}

	for step := 0; step < 3; step++ {
		response, err := client.Chat(ctx, llm.ChatRequest{
			Model:     "gpt-4o",
			Messages:  messages,
			Tools:     tools,
			MaxTokens: 200,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}

		if len(response.ToolCalls) == 0 {
			fmt.Printf("Final answer: %s\n", response.Content)
			break
		}

		// Execute tool and continue conversation
		for _, call := range response.ToolCalls {
			fmt.Printf("Step %d: Using %s(%s)\n", step+1, call.Name, call.Arguments)

			var args struct {
				A float64 `json:"a"`
				B float64 `json:"b"`
			}
			json.Unmarshal([]byte(call.Arguments), &args)

			var result float64
			switch call.Name {
			case "add":
				result = add(args.A, args.B)
			case "subtract":
				result = sub(args.A, args.B)
			case "multiply":
				result = mul(args.A, args.B)
			case "divide":
				result, _ = div(args.A, args.B)
			}

			// Add result to conversation
			resultMsg := fmt.Sprintf("%.0f %s %.0f = %.0f", args.A, call.Name, args.B, result)
			messages = append(messages,
				llm.Message{Role: "assistant", Content: ""},
				llm.Message{Role: "user", Content: resultMsg},
			)
			fmt.Printf("  Result: %.0f\n", result)
		}
	}
}
