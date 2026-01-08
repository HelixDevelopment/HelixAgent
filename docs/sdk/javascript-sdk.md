# HelixAgent JavaScript SDK

> **Status: Available**
>
> The TypeScript/JavaScript SDK is implemented at `/sdk/web/`.
> Install from source or wait for npm publication.

A comprehensive JavaScript/TypeScript SDK for the HelixAgent AI orchestration platform, providing type-safe access to multi-provider LLM capabilities, AI debates, and advanced features.

## Installation

```bash
# Install from source
cd sdk/web
npm install
npm run build

# Or when published to npm:
# npm install helixagent-sdk
```

## Quick Start

```javascript
import { HelixAgent } from '@helixagent/sdk';

const client = new HelixAgent({
  apiKey: 'your-api-key',
  baseURL: 'https://api.helixagent.ai'
});

// Simple chat completion
const response = await client.chat.completions.create({
  model: 'helixagent-ensemble',
  messages: [
    { role: 'user', content: 'Explain quantum computing' }
  ]
});

console.log(response.choices[0].message.content);
```

## TypeScript Support

```typescript
import { HelixAgent, ChatCompletionRequest, DebateConfig } from '@helixagent/sdk';

const client = new HelixAgent({
  apiKey: process.env.HELIXAGENT_API_KEY!
});

interface CustomDebateConfig extends DebateConfig {
  customField: string;
}
```

## Authentication

```javascript
// API Key authentication
const client = new HelixAgent({
  apiKey: 'your-api-key'
});

// JWT Token authentication
const client = new HelixAgent({
  token: 'your-jwt-token'
});

// Custom configuration
const client = new HelixAgent({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:7061',
  timeout: 30000,
  maxRetries: 3
});
```

## Chat Completions

### Basic Chat Completion

```javascript
const response = await client.chat.completions.create({
  model: 'helixagent-ensemble',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'What is machine learning?' }
  ],
  max_tokens: 500,
  temperature: 0.7
});

console.log(response.choices[0].message.content);
console.log(`Usage: ${response.usage.total_tokens} tokens`);
```

### Streaming Chat Completion

```javascript
const stream = await client.chat.completions.create({
  model: 'deepseek-chat',
  messages: [{ role: 'user', content: 'Tell me a story' }],
  stream: true
});

for await (const chunk of stream) {
  if (chunk.choices[0].delta.content) {
    process.stdout.write(chunk.choices[0].delta.content);
  }
}
```

### Ensemble Completion

```javascript
const response = await client.ensemble.completions.create({
  messages: [{ role: 'user', content: 'What is the future of AI?' }],
  ensemble_config: {
    strategy: 'confidence_weighted',
    min_providers: 3,
    confidence_threshold: 0.8,
    fallback_to_best: true
  }
});

console.log(`Ensemble result: ${response.choices[0].message.content}`);
console.log(`Providers used: ${response.ensemble.providers_used.join(', ')}`);
console.log(`Confidence: ${response.ensemble.confidence_score}`);
```

## Text Completions

### Basic Text Completion

```javascript
const response = await client.completions.create({
  model: 'qwen-max',
  prompt: 'The future of technology is',
  max_tokens: 100,
  temperature: 0.8,
  stop: ['\n', '.']
});

console.log(response.choices[0].text);
```

### Streaming Text Completion

```javascript
const stream = await client.completions.create({
  model: 'openrouter/grok-4',
  prompt: 'Write a haiku about programming:',
  stream: true
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0].text);
}
```

## AI Debate System

### Creating a Basic Debate

```javascript
const debateConfig = {
  debateId: 'ai-ethics-debate-001',
  topic: 'Should AI systems have ethical constraints built into their core architecture?',
  maximal_repeat_rounds: 5,
  consensus_threshold: 0.75,
  participants: [
    {
      name: 'EthicsExpert',
      role: 'AI Ethics Specialist',
      llms: [{
        provider: 'claude',
        model: 'claude-3-5-sonnet-20241022',
        api_key: process.env.CLAUDE_API_KEY
      }]
    },
    {
      name: 'AIScientist',
      role: 'AI Research Scientist',
      llms: [{
        provider: 'deepseek',
        model: 'deepseek-coder'
      }]
    }
  ]
};

const debate = await client.debates.create(debateConfig);
console.log(`Debate created: ${debate.debateId}`);
```

### Monitoring Debate Progress

```javascript
// Get debate status
const status = await client.debates.getStatus('ai-ethics-debate-001');
console.log(`Status: ${status.status}`);
console.log(`Progress: Round ${status.current_round}/${status.total_rounds}`);

// Wait for completion
while (!['completed', 'failed'].includes(status.status)) {
  await new Promise(resolve => setTimeout(resolve, 5000));
  status = await client.debates.getStatus('ai-ethics-debate-001');
}
```

### Getting Debate Results

```javascript
const results = await client.debates.getResults('ai-ethics-debate-001');

console.log(`Topic: ${results.topic}`);
console.log(`Consensus achieved: ${results.consensus.achieved}`);
console.log(`Final position: ${results.consensus.final_position}`);

results.participants.forEach(participant => {
  console.log(`${participant.name}: ${participant.total_responses} responses, ` +
              `avg quality: ${participant.avg_quality_score}`);
});
```

### Advanced Debate with Cognee Enhancement

```javascript
const debateConfig = {
  debateId: 'enhanced-debate-001',
  topic: 'How should society regulate artificial general intelligence?',
  enable_cognee: true,
  cognee_config: {
    dataset_name: 'agi_regulation_debate',
    enhancement_strategy: 'hybrid',
    max_enhancement_time: 30000
  },
  participants: [
    {
      name: 'PolicyMaker',
      role: 'Government Policy Advisor',
      enable_cognee: true,
      cognee_settings: {
        enhance_responses: true,
        analyze_sentiment: true,
        dataset_name: 'policy_debate_data'
      }
    },
    {
      name: 'AIRiskExpert',
      role: 'AI Safety Researcher',
      enable_cognee: true
    }
  ]
};

const debate = await client.debates.create(debateConfig);
```

## Model Context Protocol (MCP)

### Getting MCP Capabilities

```javascript
const capabilities = await client.mcp.capabilities();
console.log(`MCP Version: ${capabilities.version}`);
console.log(`Available providers: ${capabilities.providers.join(', ')}`);
```

### Listing MCP Tools

```javascript
const tools = await client.mcp.tools();
tools.tools.forEach(tool => {
  console.log(`Tool: ${tool.name} - ${tool.description}`);
});
```

### Executing MCP Tools

```javascript
const result = await client.mcp.tools.call({
  name: 'read_file',
  arguments: { path: '/etc/hostname' }
});
console.log(`Result: ${result.result}`);
```

### MCP Prompts

```javascript
const prompts = await client.mcp.prompts();
prompts.prompts.forEach(prompt => {
  console.log(`Prompt: ${prompt.name} - ${prompt.description}`);
});
```

### MCP Resources

```javascript
const resources = await client.mcp.resources();
resources.resources.forEach(resource => {
  console.log(`Resource: ${resource.name} - ${resource.description}`);
});
```

## Provider Management

### Listing Available Providers

```javascript
const providers = await client.providers.list();
providers.providers.forEach(provider => {
  console.log(`${provider.name}: ${provider.status} - ${provider.models.length} models`);
});
```

### Provider Health Check

```javascript
const health = await client.providers.health();
console.log(`Overall status: ${health.status}`);
Object.entries(health.providers).forEach(([name, status]) => {
  console.log(`${name}: ${status.status} (response time: ${status.response_time}s)`);
});
```

## Error Handling

```javascript
import { HelixAgentError, AuthenticationError, RateLimitError } from '@helixagent/sdk';

try {
  const response = await client.chat.completions.create({
    model: 'invalid-model',
    messages: [{ role: 'user', content: 'Hello' }]
  });
} catch (error) {
  if (error instanceof AuthenticationError) {
    console.error('Authentication failed:', error.message);
  } else if (error instanceof RateLimitError) {
    console.error('Rate limit exceeded:', error.message);
  } else if (error instanceof HelixAgentError) {
    console.error('HelixAgent error:', error.message);
  } else {
    console.error('Unknown error:', error);
  }
}
```

## Advanced Configuration

### Custom HTTP Client

```javascript
import axios from 'axios';

const customAxios = axios.create({
  proxy: {
    host: 'proxy.company.com',
    port: 8080
  }
});

const client = new HelixAgent({
  apiKey: 'your-api-key',
  axiosInstance: customAxios
});
```

### Timeout and Retry Configuration

```javascript
const client = new HelixAgent({
  apiKey: 'your-api-key',
  timeout: 30000, // 30 seconds
  maxRetries: 3,
  retryDelay: 1000 // 1 second base delay
});
```

### Event Emitters

```javascript
client.on('request', (request) => {
  console.log(`Making request to: ${request.url}`);
});

client.on('response', (response) => {
  console.log(`Response received: ${response.status}`);
});

client.on('error', (error) => {
  console.error('Request failed:', error);
});
```

## Best Practices

### 1. Error Handling

```javascript
async function safeCompletion(model, messages) {
  try {
    const response = await client.chat.completions.create({
      model,
      messages,
      max_tokens: 1000
    });
    return response.choices[0].message.content;
  } catch (error) {
    if (error instanceof RateLimitError) {
      await new Promise(resolve => setTimeout(resolve, 60000));
      return safeCompletion(model, messages);
    }
    if (error instanceof ProviderError && model.startsWith('claude')) {
      return safeCompletion('deepseek-chat', messages);
    }
    throw error;
  }
}
```

### 2. Streaming with Backpressure

```javascript
async function handleStreaming() {
  const stream = await client.chat.completions.create({
    model: 'helixagent-ensemble',
    messages: [{ role: 'user', content: 'Write a long story' }],
    stream: true
  });

  const chunks = [];
  let totalTokens = 0;

  for await (const chunk of stream) {
    chunks.push(chunk);

    if (chunk.usage) {
      totalTokens = chunk.usage.total_tokens;
    }

    // Process chunk
    if (chunk.choices[0].delta.content) {
      process.stdout.write(chunk.choices[0].delta.content);

      // Implement backpressure if needed
      if (chunks.length > 100) {
        await new Promise(resolve => setTimeout(resolve, 10));
      }
    }
  }

  console.log(`\nTotal tokens used: ${totalTokens}`);
}
```

### 3. Debate Orchestration

```javascript
class DebateOrchestrator {
  constructor(client) {
    this.client = client;
    this.activeDebates = new Map();
  }

  async createDebate(config) {
    const debate = await this.client.debates.create(config);
    this.activeDebates.set(debate.debateId, {
      ...debate,
      startTime: Date.now()
    });
    return debate;
  }

  async monitorDebate(debateId) {
    const status = await this.client.debates.getStatus(debateId);

    if (status.status === 'completed') {
      const results = await this.client.debates.getResults(debateId);
      this.activeDebates.delete(debateId);
      return results;
    }

    return status;
  }

  async getActiveDebates() {
    return Array.from(this.activeDebates.values());
  }
}
```

### 4. Resource Management

```javascript
// Connection pooling
const client = new HelixAgent({
  apiKey: 'your-api-key',
  maxConnections: 10,
  keepAlive: true
});

// Cleanup on application shutdown
process.on('SIGTERM', async () => {
  await client.close();
  process.exit(0);
});
```

## API Reference

### Classes

- `HelixAgent`: Main client class
- `ChatCompletions`: Chat completion operations
- `Completions`: Text completion operations
- `Ensemble`: Ensemble operations
- `Debates`: AI debate operations
- `MCP`: Model Context Protocol operations
- `Providers`: Provider management

### Types

```typescript
interface ChatCompletionRequest {
  model: string;
  messages: ChatMessage[];
  max_tokens?: number;
  temperature?: number;
  stream?: boolean;
}

interface DebateConfig {
  debateId: string;
  topic: string;
  maximal_repeat_rounds: number;
  consensus_threshold: number;
  participants: DebateParticipant[];
  enable_cognee?: boolean;
  cognee_config?: CogneeConfig;
}

interface MCPToolCall {
  name: string;
  arguments: Record<string, any>;
}
```

### Exceptions

- `HelixAgentError`: Base exception
- `AuthenticationError`: Authentication failures
- `RateLimitError`: Rate limit exceeded
- `ProviderError`: Provider-specific errors
- `ValidationError`: Input validation errors
- `NetworkError`: Network connectivity issues

## Requirements

- Node.js 16+
- npm or yarn
- TypeScript 4.5+ (for TypeScript support)

## Browser Support

The SDK can be used in browsers with bundlers like Webpack or Rollup:

```javascript
// webpack.config.js
module.exports = {
  resolve: {
    fallback: {
      "fs": false,
      "path": false,
      "crypto": false
    }
  }
};
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new features
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details.