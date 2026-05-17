//go:build ignore

// Package main demonstrates vision/multimodal capabilities
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"dev.helix.agent/internal/llm"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fmt.Println("=== HelixAgent Vision Example ===")
	fmt.Println()

	// Check for vision-capable provider
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := "gpt-4o"
	provider := "openai"

	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		model = "claude-3-5-sonnet-20241022"
		provider = "anthropic"
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		model = "gemini-2.0-flash-exp"
		provider = "gemini"
	}
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
		model = "llama-3.2-11b-vision-preview"
		provider = "groq"
	}

	if apiKey == "" {
		fmt.Println("No vision-capable provider configured!")
		fmt.Println("Please set one of:")
		fmt.Println("  - OPENAI_API_KEY (gpt-4o)")
		fmt.Println("  - ANTHROPIC_API_KEY (claude-3.5-sonnet)")
		fmt.Println("  - GEMINI_API_KEY (gemini-2.0-flash)")
		fmt.Println("  - GROQ_API_KEY (llama-3.2-vision)")
		return
	}

	client := llm.NewClient(provider, apiKey, logger)
	ctx := context.Background()

	fmt.Printf("Using provider: %s\n", provider)
	fmt.Printf("Using model: %s\n", model)
	fmt.Println()

	// Example 1: Simple image description
	fmt.Println("--- Image Description ---")

	// Create a simple test image (red square) or load from file
	imageData := getTestImage()
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	prompts := []string{
		"What do you see in this image?",
		"Describe the colors in this image.",
		"What is the main subject of this image?",
	}

	for _, prompt := range prompts {
		fmt.Printf("Prompt: %s\n", prompt)

		start := time.Now()
		response, err := client.Chat(ctx, llm.ChatRequest{
			Model: model,
			Messages: []llm.Message{
				{
					Role:    "user",
					Content: prompt,
					Images:  []string{imageBase64},
				},
			},
			MaxTokens: 200,
		})
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		fmt.Printf("  Response (%v): %s\n\n", time.Since(start), response.Content)
	}

	// Example 2: OCR (Optical Character Recognition)
	fmt.Println("--- OCR Example ---")

	// Create an image with text or load from file
	textImage := createTextImage("Hello\nWorld\n123")
	textImageBase64 := base64.StdEncoding.EncodeToString(textImage)

	ocrPrompt := "Extract all text from this image. List each line separately."
	fmt.Printf("Prompt: %s\n", ocrPrompt)

	start := time.Now()
	response, err := client.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: ocrPrompt,
				Images:  []string{textImageBase64},
			},
		},
		MaxTokens: 200,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Extracted text (%v):\n%s\n\n", time.Since(start), response.Content)
	}

	// Example 3: Image comparison
	fmt.Println("--- Image Comparison ---")

	image1 := getTestImage()
	image2 := getBlueTestImage()

	comparePrompt := "Compare these two images. What are the differences?"
	fmt.Printf("Prompt: %s\n", comparePrompt)

	start = time.Now()
	response, err = client.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: comparePrompt,
				Images: []string{
					base64.StdEncoding.EncodeToString(image1),
					base64.StdEncoding.EncodeToString(image2),
				},
			},
		},
		MaxTokens: 300,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Comparison (%v): %s\n\n", time.Since(start), response.Content)
	}

	// Example 4: Visual reasoning
	fmt.Println("--- Visual Reasoning ---")

	reasoningImage := createChartImage()
	reasoningPrompt := "Analyze this chart. What trends do you observe? What conclusions can you draw?"
	fmt.Printf("Prompt: %s\n", reasoningPrompt)

	start = time.Now()
	response, err = client.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: reasoningPrompt,
				Images:  []string{base64.StdEncoding.EncodeToString(reasoningImage)},
			},
		},
		MaxTokens: 400,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Analysis (%v): %s\n\n", time.Since(start), response.Content)
	}

	// Example 5: Streaming vision response
	fmt.Println("--- Streaming Vision ---")

	streamPrompt := "Describe this image in detail."
	fmt.Printf("Prompt: %s (streaming)\n", streamPrompt)

	stream, err := client.ChatStream(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: streamPrompt,
				Images:  []string{imageBase64},
			},
		},
		MaxTokens: 200,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Print("  Response: ")
		for chunk := range stream {
			if chunk.Error != nil {
				fmt.Printf("\n  Error: %v\n", chunk.Error)
				break
			}
			fmt.Print(chunk.Content)
		}
		fmt.Println("\n")
	}

	// Example 6: Vision with tool calling
	fmt.Println("--- Vision + Tools ---")

	tools := []llm.ToolDefinition{
		{
			Name:        "save_description",
			Description: "Save the image description to a file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]string{"type": "string"},
					"filename":    map[string]string{"type": "string"},
				},
				"required": []string{"description", "filename"},
			},
		},
	}

	toolPrompt := "Describe this image and then save the description to a file."
	fmt.Printf("Prompt: %s (with tools)\n", toolPrompt)

	response, err = client.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: toolPrompt,
				Images:  []string{imageBase64},
			},
		},
		Tools:     tools,
		MaxTokens: 300,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Description: %s\n", response.Content)
		if len(response.ToolCalls) > 0 {
			fmt.Printf("  Tools called: %v\n", len(response.ToolCalls))
			for _, call := range response.ToolCalls {
				fmt.Printf("    - %s\n", call.Name)
			}
		}
	}

	fmt.Println("\nVision example complete!")
}

// getTestImage returns a simple red PNG image as bytes
func getTestImage() []byte {
	// 1x1 red pixel PNG
	data, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	)
	return data
}

// getBlueTestImage returns a simple blue PNG image
func getBlueTestImage() []byte {
	// 1x1 blue pixel PNG
	data, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	)
	return data
}

// createTextImage returns a 1×1 red pixel — this is an example/demo
// scaffold, not a real text renderer. Production text-to-image
// rendering would use image/draw + golang.org/x/image/font.
//
// The function intentionally discards `text` so any caller asserting
// "image contains 'Hello\nWorld\n123'" will FAIL — exposing the
// stub at the assertion boundary instead of certifying the §11.4
// PASS-bluff (the previous version's silent fall-through to a
// 1×1 placeholder while pretending to encode text).
func createTextImage(text string) []byte {
	fmt.Fprintf(os.Stderr, "[example-stub / §11.4] createTextImage: ignoring text %q, returning 1x1 placeholder PNG; real text rendering requires image/draw + font library\n", text)
	return getTestImage()
}

// createChartImage returns a 1×1 blue pixel — example/demo scaffold.
// Real chart rendering would use a charting library
// (e.g. go-echarts, gonum/plot). Marked as stub so any caller's
// assertion on chart structure FAILs explicitly.
func createChartImage() []byte {
	fmt.Fprintln(os.Stderr, "[example-stub / §11.4] createChartImage: returning 1x1 placeholder PNG; real chart rendering requires a charting library")
	return getBlueTestImage()
}
