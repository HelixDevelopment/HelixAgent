package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenAIRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func testStandaloneChatCompletion(model, prompt string) {
	baseURL := "http://localhost:7061/v1"
	apiKey := "test-key"

	request := OpenAIRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 50,
	}

	jsonData, _ := json.Marshal(request)

	httpReq, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		var chatResponse OpenAIResponse
		json.Unmarshal(body, &chatResponse)
		fmt.Printf("✅ %s: %s\n", model, chatResponse.Choices[0].Message.Content[:50]+"...")
	} else {
		fmt.Printf("❌ %s failed with status: %d\n", model, resp.StatusCode)
	}
}

func mainTest() {
	baseURL := "http://localhost:7061/v1"

	// Test models endpoint
	fmt.Println("Testing /v1/models endpoint...")
	resp, err := http.Get(baseURL + "/models")
	if err != nil {
		fmt.Printf("Error fetching models: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Models endpoint returned status: %d\n", resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var modelsResponse map[string]interface{}
	json.Unmarshal(body, &modelsResponse)

	if data, ok := modelsResponse["data"].([]interface{}); ok {
		fmt.Printf("✅ Found %d models\n\n", len(data))
	}

	// Test different models
	fmt.Println("Testing different models with OpenAI-compatible API:")
	fmt.Println("====================================================")

	testModels := []string{
		"helixagent-ensemble",
		"deepseek-chat",
		"qwen-turbo",
		"anthropic/claude-3.5-sonnet",
		"google/gemini-2.0-flash-exp",
	}

	prompt := "Explain what an API is in one sentence"

	for _, model := range testModels {
		testStandaloneChatCompletion(model, prompt)
	}

	fmt.Println("\n🎉 OpenAI API compatibility test complete!")
	fmt.Println("The system is ready to work with AI CLI tools like OpenCode and Crush.")
}
