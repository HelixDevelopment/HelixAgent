# HelixAgent API Reference Examples

This document provides practical examples for using the HelixAgent API with various programming languages and tools.

## Table of Contents
1. [Quick Start](#quick-start)
2. [Authentication Examples](#authentication-examples)
3. [Chat Completion Examples](#chat-completion-examples)
4. [Ensemble Configuration Examples](#ensemble-configuration-examples)
5. [Provider Management Examples](#provider-management-examples)
6. [Monitoring Examples](#monitoring-examples)
7. [Protocol Examples](#protocol-examples)
8. [Planning and Agentic Examples](#planning-and-agentic-examples)
9. [RAG and Search Examples](#rag-and-search-examples)
10. [LLMOps and Benchmark Examples](#llmops-and-benchmark-examples)
11. [Integration Examples](#integration-examples)

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

## Protocol Examples

### MCP Tool Call

```bash
# List available MCP tools
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/mcp/tools

# Call an MCP tool
curl -X POST http://localhost:7061/v1/mcp/tools/call \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Read",
    "arguments": {
      "file_path": "/path/to/file.go"
    }
  }'

# Search MCP tools
curl "http://localhost:7061/v1/mcp/tools/search?q=file" \
  -H "Authorization: Bearer $TOKEN"
```

### ACP Agent Communication

```bash
# List ACP agents
curl http://localhost:7061/v1/acp/agents

# Execute agent task
curl -X POST http://localhost:7061/v1/acp/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "code-analyzer",
    "task": "analyze",
    "payload": {"file": "/path/to/file.go"}
  }'

# JSON-RPC 2.0 call
curl -X POST http://localhost:7061/v1/acp/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "agent/execute",
    "params": {"task": "analyze"},
    "id": 1
  }'
```

### LSP Integration

```bash
# List LSP servers
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/lsp/servers

# Execute LSP request (go to definition)
curl -X POST http://localhost:7061/v1/lsp/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "language": "go",
    "method": "textDocument/definition",
    "params": {
      "textDocument": {"uri": "file:///path/to/file.go"},
      "position": {"line": 10, "character": 5}
    }
  }'
```

### Vision Analysis

```bash
# Analyze an image
curl -X POST http://localhost:7061/v1/vision/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "image": "data:image/png;base64,iVBOR...",
    "prompt": "Describe the UI elements in this screenshot"
  }'

# OCR extraction
curl -X POST http://localhost:7061/v1/vision/ocr \
  -H "Content-Type: application/json" \
  -d '{
    "image": "data:image/png;base64,iVBOR..."
  }'
```

### Cognee Knowledge Graph

```bash
# Add to knowledge graph
curl -X POST http://localhost:7061/v1/cognee/memory \
  -H "Content-Type: application/json" \
  -d '{
    "content": "The authentication system uses JWT tokens with refresh capability.",
    "metadata": {"source": "auth.md", "tags": ["auth", "jwt"]}
  }'

# Search knowledge
curl -X POST http://localhost:7061/v1/cognee/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does authentication work?",
    "limit": 5
  }'
```

## Planning and Agentic Examples

### Hierarchical Planning (HiPlan)

```bash
curl -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Migrate database from PostgreSQL 12 to 15",
    "config": {
      "max_milestones": 5,
      "max_steps_per_milestone": 8,
      "enable_parallel_milestones": true,
      "timeout_seconds": 300
    }
  }'
```

### Monte Carlo Tree Search (MCTS)

```bash
curl -X POST http://localhost:7061/v1/planning/mcts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Optimize API response time from 500ms to under 100ms",
    "config": {
      "max_iterations": 500,
      "exploration_weight": 1.414,
      "max_depth": 8,
      "timeout_seconds": 120
    }
  }'
```

### Tree of Thoughts (ToT)

```bash
curl -X POST http://localhost:7061/v1/planning/tot \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Design a caching strategy for the recommendation engine",
    "config": {
      "branching_factor": 3,
      "max_depth": 5,
      "evaluation_strategy": "vote",
      "timeout_seconds": 180
    }
  }'
```

### Agentic Workflow

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-review-pipeline",
    "nodes": [
      {"id": "analyze", "name": "Static Analysis", "type": "llm"},
      {"id": "review", "name": "Code Review", "type": "llm"},
      {"id": "report", "name": "Report Generation", "type": "llm"}
    ],
    "edges": [
      {"from": "analyze", "to": "review"},
      {"from": "review", "to": "report"}
    ],
    "entry_point": "analyze",
    "end_nodes": ["report"],
    "input": {
      "query": "Review this Go function for best practices"
    }
  }'
```

## RAG and Search Examples

### Document Ingestion

```bash
# Ingest a single document
curl -X POST http://localhost:7061/v1/rag/documents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "The circuit breaker pattern prevents cascading failures...",
    "metadata": {
      "source": "architecture.md",
      "type": "documentation",
      "tags": ["patterns", "resilience"]
    },
    "chunk_strategy": "semantic"
  }'
```

### RAG Search

```bash
# Basic search
curl -X POST http://localhost:7061/v1/rag/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does the circuit breaker work?",
    "limit": 10,
    "threshold": 0.7
  }'

# Hybrid search
curl -X POST http://localhost:7061/v1/rag/search/hybrid \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "circuit breaker pattern implementation",
    "dense_weight": 0.7,
    "sparse_weight": 0.3,
    "limit": 10
  }'
```

### Semantic Code Search

```bash
curl -X POST http://localhost:7061/v1/search/semantic \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "error handling middleware",
    "language": "go",
    "file_pattern": "*.go",
    "top_k": 10,
    "min_score": 0.7
  }'
```

### Embeddings

```bash
# Generate embeddings
curl -X POST http://localhost:7061/v1/embeddings/generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "input": ["How does authentication work?", "What is JWT?"],
    "model": "bge-m3"
  }'
```

## LLMOps and Benchmark Examples

### Create A/B Experiment

```bash
curl -X POST http://localhost:7061/v1/llmops/experiments \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prompt-v2-test",
    "description": "Compare v1 vs v2 prompt templates",
    "variants": [
      {"name": "control", "model": "claude-3-opus", "prompt_template": "Original prompt..."},
      {"name": "optimized", "model": "claude-3-opus", "prompt_template": "Improved prompt..."}
    ],
    "traffic_split": {"control": 0.5, "optimized": 0.5},
    "metrics": ["quality_score", "latency"],
    "target_metric": "quality_score"
  }'

# List experiments
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/llmops/experiments
```

### Run Benchmark

```bash
# Start a HumanEval benchmark
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "claude-humaneval-q2",
    "benchmark_type": "humaneval",
    "provider_name": "claude",
    "model_name": "claude-3-opus"
  }'

# Check results
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/benchmark/results
```

### Discovery and Scoring

```bash
# Get discovered models
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/discovery/models

# Get top-scored models
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:7061/v1/scoring/top?limit=5"

# Get specific model score
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:7061/v1/scoring/model/claude-3-opus
```

### QA Session

```bash
# Start a web QA session
curl -X POST http://localhost:7061/v1/qa/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "project_root": "/home/user/myapp",
    "platforms": ["web"],
    "web_url": "http://localhost:3000",
    "output_dir": "/tmp/qa-run"
  }'

# List findings
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:7061/v1/qa/findings?status=open"
```

### Background Tasks

```bash
# Create a task
curl -X POST http://localhost:7061/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "command",
    "command": "make build",
    "working_dir": "/home/user/project",
    "priority": "high"
  }'

# Check task status
curl http://localhost:7061/v1/tasks/task-xyz789/status

# Analyze for stuck detection
curl http://localhost:7061/v1/tasks/task-xyz789/analyze
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

For more information, refer to the [main API documentation](api-documentation.md) and [deployment guide](../deployment/DEPLOYMENT_GUIDE.md).