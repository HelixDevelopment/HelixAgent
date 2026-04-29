// HelixAgent Integration Plugin
// Provides core API integration for CLI agents

package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "time"
)

// Plugin metadata
var PluginName = "helix-integration"
var PluginVersion = "1.0.0"

// Config holds plugin configuration
type Config struct {
    HelixAgentURL string `json:"helix_agent_url"`
    APIKey        string `json:"api_key"`
    Timeout       int    `json:"timeout"`
    RetryAttempts int    `json:"retry_attempts"`
    RetryBackoff  int    `json:"retry_backoff_ms"`
}

// HelixClient provides API access to HelixAgent
type HelixClient struct {
    config     Config
    httpClient *http.Client
}

// NewHelixClient creates a new client instance
func NewHelixClient(config Config) *HelixClient {
    return &HelixClient{
        config: config,
        httpClient: &http.Client{
            Timeout: time.Duration(config.Timeout) * time.Millisecond,
        },
    }
}

// ChatCompletion sends a chat completion request
func (c *HelixClient) ChatCompletion(ctx context.Context, messages []map[string]string, model string) (string, error) {
    payload := map[string]interface{}{
        "model":    model,
        "messages": messages,
        "stream":   false,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("marshal request: %w", err)
    }

    url := c.config.HelixAgentURL + "/v1/chat/completions"
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return "", fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    if c.config.APIKey != "" {
        req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
    }

    var lastErr error
    for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
        if attempt > 0 {
            time.Sleep(time.Duration(c.config.RetryBackoff*attempt) * time.Millisecond)
        }

        resp, err := c.httpClient.Do(req)
        if err != nil {
            lastErr = err
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            respBody, _ := io.ReadAll(resp.Body)
            lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
            continue
        }

        var result map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return "", fmt.Errorf("decode response: %w", err)
        }

        choices, ok := result["choices"].([]interface{})
        if !ok || len(choices) == 0 {
            return "", fmt.Errorf("no choices in response")
        }

        choice := choices[0].(map[string]interface{})
        message := choice["message"].(map[string]interface{})
        content := message["content"].(string)

        return content, nil
    }

    return "", fmt.Errorf("all retry attempts failed: %v", lastErr)
}

// Init initializes the plugin
func Init() error {
    fmt.Println("[helix-integration] Plugin initialized")
    return nil
}

// Shutdown gracefully shuts down the plugin
func Shutdown() error {
    fmt.Println("[helix-integration] Plugin shutdown")
    return nil
}

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Printf("%s v%s\n", PluginName, PluginVersion)
        return
    }
    fmt.Println("HelixAgent Integration Plugin")
}
