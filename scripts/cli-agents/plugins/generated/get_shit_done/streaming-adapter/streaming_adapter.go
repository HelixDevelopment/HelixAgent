// Streaming Adapter Plugin
// Handles streaming responses from HelixAgent

package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
)

var PluginName = "streaming-adapter"
var PluginVersion = "1.0.0"

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Index int `json:"index"`
        Delta struct {
            Content string `json:"content"`
        } `json:"delta"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
}

// StreamingClient handles streaming API calls
type StreamingClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

// NewStreamingClient creates a new streaming client
func NewStreamingClient(baseURL, apiKey string) *StreamingClient {
    return &StreamingClient{
        baseURL:    baseURL,
        apiKey:     apiKey,
        httpClient: &http.Client{},
    }
}

// StreamCompletion streams a chat completion
func (c *StreamingClient) StreamCompletion(ctx context.Context, messages []map[string]string, model string, onChunk func(string)) error {
    payload := map[string]interface{}{
        "model":    model,
        "messages": messages,
        "stream":   true,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return err
    }

    url := c.baseURL + "/v1/chat/completions"
    req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "text/event-stream")
    if c.apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+c.apiKey)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    reader := bufio.NewReader(resp.Body)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return err
        }

        line = strings.TrimSpace(line)
        if !strings.HasPrefix(line, "data:") {
            continue
        }

        data := strings.TrimPrefix(line, "data:")
        data = strings.TrimSpace(data)

        if data == "[DONE]" {
            break
        }

        var chunk StreamChunk
        if err := json.Unmarshal([]byte(data), &chunk); err != nil {
            continue
        }

        for _, choice := range chunk.Choices {
            if choice.Delta.Content != "" {
                onChunk(choice.Delta.Content)
            }
        }
    }

    return nil
}

func Init() error {
    fmt.Println("[streaming-adapter] Plugin initialized")
    return nil
}

func Shutdown() error {
    fmt.Println("[streaming-adapter] Plugin shutdown")
    return nil
}

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Printf("%s v%s\n", PluginName, PluginVersion)
        return
    }
    fmt.Println("Streaming Adapter Plugin")
}
