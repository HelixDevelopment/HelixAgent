# HelixAgent API Reference Examples

This document provides practical examples for using the HelixAgent API with various programming languages and tools.

## Table of Contents
1. [Quick Start](#quick-start)
2. [Authentication Examples](#authentication-examples)
3. [Chat Completion Examples](#chat-completion-examples)
4. [Ensemble Configuration Examples](#ensemble-configuration-examples)
5. [Provider Management Examples](#provider-management-examples)
6. [Monitoring Examples](#monitoring-examples)
7. [Integration Examples](#integration-examples)

## Quick Start

### 1. Install and Start HelixAgent

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Build the binary
go build -o helixagent cmd/helixagent/main_multi_provider.go

# Start with multi-provider configuration
./helixagent --config configs/multi-provider.yaml
```

### 2. Test Basic Connectivity

```bash
# Check if server is running
curl http://localhost:7061/health

# Expected response:
# {"status":"ok","timestamp":1703123456,"version":"1.0.0"}
```

## Authentication Examples

### Register a New User

```bash
curl -X POST http://localhost:7061/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "TestPass123!",
    "email": "test@example.com"
  }'
```

### Login and Get Token

```bash
# Login and save token to environment variable
TOKEN=$(curl -s -X POST http://localhost:7061/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "TestPass123!"
  }' | jq -r '.token')

echo "Token: $TOKEN"
```

### Use Token in Requests

```bash
# Make authenticated request
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/models
```

## Chat Completion Examples

### Basic Chat Completion

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful AI assistant."
      },
      {
        "role": "user",
        "content": "What is the capital of France?"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 500
  }'
```

### Streaming Chat Completion

```bash
curl -X POST http://localhost:7061/v1/chat/completions/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [
      {
        "role": "user",
        "content": "Write a short story about a robot learning to paint."
      }
    ],
    "temperature": 0.8,
    "max_tokens": 1000,
    "stream": true
  }'
```

### Chat with Specific Provider

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "messages": [
      {
        "role": "user",
        "content": "Explain binary search algorithm."
      }
    ],
    "temperature": 0.3,
    "max_tokens": 300
  }'
```

## Ensemble Configuration Examples

### Advanced Ensemble Configuration

```bash
curl -X POST http://localhost:7061/v1/ensemble/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Compare and contrast Python and JavaScript for web development.",
    "model": "helixagent-ensemble",
    "temperature": 0.5,
    "max_tokens": 800,
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "min_providers": 3,
      "confidence_threshold": 0.75,
      "fallback_to_best": true,
      "timeout": 45,
      "preferred_providers": ["deepseek", "qwen", "openrouter"]
    },
    "memory_enhanced": true
  }'
```

### Ensemble with Memory Enhancement

```bash
curl -X POST http://localhost:7061/v1/ensemble/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Based on our previous conversation about machine learning, what are the latest trends?",
    "model": "helixagent-ensemble",
    "temperature": 0.6,
    "max_tokens": 600,
    "ensemble_config": {
      "strategy": "majority_vote",
      "min_providers": 2,
      "confidence_threshold": 0.6,
      "fallback_to_best": false,
      "timeout": 30
    },
    "memory_enhanced": true,
    "messages": [
      {
        "role": "system",
        "content": "You are an expert in machine learning and AI trends."
      },
      {
        "role": "user",
        "content": "What are the latest trends in machine learning?"
      }
    ]
  }'
```

## Provider Management Examples

### List All Providers

```bash
# Public endpoint (basic info)
curl http://localhost:7061/v1/providers

# Protected endpoint (detailed info)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/providers
```

### Check Provider Health

```bash
# Check specific provider health
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/providers/deepseek/health

# Check all providers (admin only)
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://localhost:7061/v1/admin/health/all
```

### Get Provider Capabilities

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/providers | jq '.providers[] | select(.name == "deepseek")'
```

## Monitoring Examples

### Check System Health

```bash
# Basic health check
curl http://localhost:7061/health

# Enhanced health check with provider status
curl http://localhost:7061/v1/health
```

### Get Prometheus Metrics

```bash
# Get raw metrics
curl http://localhost:7061/metrics

# Filter specific metrics
curl http://localhost:7061/metrics | grep helixagent_requests_total

# Get metrics with timestamp
curl "http://localhost:7061/metrics?timestamp=$(date +%s)"
```

### Monitor Request Statistics

```bash
# Count total requests
curl http://localhost:7061/metrics | grep 'helixagent_requests_total{' | awk '{print $2}'

# Get average response time
curl http://localhost:7061/metrics | grep 'helixagent_request_duration_seconds_sum' | awk '{print $2}'
```

## Integration Examples

### Python Integration

```python
import requests
import json

class HelixAgentClient:
    def __init__(self, base_url="http://localhost:7061", token=None):
        self.base_url = base_url
        self.token = token
        self.session = requests.Session()
        if token:
            self.session.headers.update({"Authorization": f"Bearer {token}"})
    
    def chat_completion(self, messages, model="helixagent-ensemble", **kwargs):
        url = f"{self.base_url}/v1/chat/completions"
        data = {
            "model": model,
            "messages": messages,
            **kwargs
        }
        response = self.session.post(url, json=data)
        response.raise_for_status()
        return response.json()
    
    def stream_chat_completion(self, messages, model="helixagent-ensemble", **kwargs):
        url = f"{self.base_url}/v1/chat/completions/stream"
        data = {
            "model": model,
            "messages": messages,
            "stream": True,
            **kwargs
        }
        response = self.session.post(url, json=data, stream=True)
        response.raise_for_status()
        
        for line in response.iter_lines():
            if line:
                line = line.decode('utf-8')
                if line.startswith('data: '):
                    data = line[6:]
                    if data == '[DONE]':
                        break
                    try:
                        yield json.loads(data)
                    except json.JSONDecodeError:
                        continue

# Usage example
client = HelixAgentClient(token="your-token")

# Single completion
response = client.chat_completion([
    {"role": "user", "content": "Hello, how are you?"}
])
print(response['choices'][0]['message']['content'])

# Streaming completion
for chunk in client.stream_chat_completion([
    {"role": "user", "content": "Tell me a story"}
]):
    if 'choices' in chunk and chunk['choices']:
        content = chunk['choices'][0].get('delta', {}).get('content', '')
        if content:
            print(content, end='', flush=True)
```

### JavaScript/Node.js Integration

```javascript
const axios = require('axios');

class HelixAgentClient {
  constructor(baseURL = 'http://localhost:7061', token = null) {
    this.client = axios.create({
      baseURL,
      headers: token ? { Authorization: `Bearer ${token}` } : {}
    });
  }

  async chatCompletion(messages, model = 'helixagent-ensemble', options = {}) {
    const response = await this.client.post('/v1/chat/completions', {
      model,
      messages,
      ...options
    });
    return response.data;
  }

  async *streamChatCompletion(messages, model = 'helixagent-ensemble', options = {}) {
    const response = await this.client.post('/v1/chat/completions/stream', {
      model,
      messages,
      stream: true,
      ...options
    }, {
      responseType: 'stream'
    });

    const stream = response.data;
    
    for await (const chunk of stream) {
      const lines = chunk.toString().split('\n');
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          if (data === '[DONE]') return;
          try {
            yield JSON.parse(data);
          } catch (e) {
            // Skip invalid JSON
          }
        }
      }
    }
  }
}

// Usage example
async function main() {
  const client = new HelixAgentClient('http://localhost:7061', 'your-token');
  
  // Single completion
  const response = await client.chatCompletion([
    { role: 'user', content: 'Hello, how are you?' }
  ]);
  console.log(response.choices[0].message.content);
  
  // Streaming completion
  for await (const chunk of client.streamChatCompletion([
    { role: 'user', content: 'Tell me a story' }
  ])) {
    if (chunk.choices && chunk.choices[0].delta?.content) {
      process.stdout.write(chunk.choices[0].delta.content);
    }
  }
}

main().catch(console.error);
```

### Go Integration

```go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HelixAgentClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHelixAgentClient(baseURL, token string) *HelixAgentClient {
	return &HelixAgentClient{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{},
	}
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

func (c *HelixAgentClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := c.baseURL + "/v1/chat/completions"
	
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

func (c *HelixAgentClient) StreamChatCompletion(ctx context.Context, req ChatCompletionRequest, callback func(string) error) error {
	req.Stream = true
	
	url := c.baseURL + "/v1/chat/completions/stream"
	
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			if data == "[DONE]" {
				break
			}
			
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if err := callback(chunk.Choices[0].Delta.Content); err != nil {
					return err
				}
			}
		}
	}
	
	return scanner.Err()
}

// Usage example
func main() {
	client := NewHelixAgentClient("http://localhost:7061", "your-token")
	
	// Single completion
	req := ChatCompletionRequest{
		Model: "helixagent-ensemble",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
	
	resp, err := client.ChatCompletion(context.Background(), req)
	if err != nil {
		panic(err)
	}
	
	fmt.Println(resp.Choices[0].Message.Content)
	
	// Streaming completion
	streamReq := ChatCompletionRequest{
		Model: "helixagent-ensemble",
		Messages: []ChatMessage{
			{Role: "user", Content: "Tell me a story"},
		},
	}
	
	err = client.StreamChatCompletion(context.Background(), streamReq, func(content string) error {
		fmt.Print(content)
		return nil
	})
	if err != nil {
		panic(err)
	}
}
```

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   ```bash
   # Check if token is valid
   curl -H "Authorization: Bearer $TOKEN" http://localhost:7061/v1/auth/me
   
   # If 401, get new token
   TOKEN=$(curl -s -X POST http://localhost:7061/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username": "testuser", "password": "TestPass123!"}' | jq -r '.token')
   ```

2. **Provider Not Responding**
   ```bash
   # Check provider health
   curl http://localhost:7061/v1/health | jq '.providers'
   
   # Check specific provider
   curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:7061/v1/providers/deepseek/health
   ```

3. **Rate Limit Exceeded**
   ```bash
   # Check rate limit headers
   curl -I -H "Authorization: Bearer $TOKEN" \
     http://localhost:7061/v1/chat/completions
   
   # Implement exponential backoff in your client
   ```

### Debugging Tips

1. **Enable Detailed Logging**
   ```bash
   # Start HelixAgent with debug logging
   ./helixagent --config configs/multi-provider.yaml --log-level debug
   
   # Check logs
   tail -f helixagent.log
   ```

2. **Monitor Metrics**
   ```bash
   # Watch request metrics
   watch -n 5 'curl -s http://localhost:7061/metrics | grep helixagent_requests_total'
   
   # Monitor provider responses
   watch -n 5 'curl -s http://localhost:7061/metrics | grep helixagent_provider_responses_total'
   ```

3. **Test Individual Providers**
   ```bash
   # Test DeepSeek directly
   curl -X POST http://localhost:7061/v1/chat/completions \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "deepseek-chat",
       "messages": [{"role": "user", "content": "Test"}],
       "max_tokens": 10
     }'
   ```

## Next Steps

1. **Explore Advanced Features**
   - Try memory-enhanced completions
   - Experiment with different ensemble strategies
   - Test with different provider combinations

2. **Integrate with Your Application**
   - Use the provided client examples
   - Implement error handling and retries
   - Add monitoring and alerting

3. **Scale Your Deployment**
   - Configure multiple HelixAgent instances
   - Set up load balancing
   - Implement high availability

For more information, refer to the [main API documentation](api-documentation.md) and [deployment guide](deployment.md).