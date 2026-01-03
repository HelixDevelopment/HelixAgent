# SuperAgent JavaScript/TypeScript SDK

A JavaScript/TypeScript SDK for the SuperAgent AI orchestration platform. Provides OpenAI-compatible API access with support for ensemble LLM strategies and AI debates.

## Installation

```bash
npm install superagent-sdk
```

Or with yarn:

```bash
yarn add superagent-sdk
```

## Quick Start

```typescript
import { SuperAgent } from 'superagent-sdk';

// Initialize client
const client = new SuperAgent({
  apiKey: 'your-api-key',
  baseUrl: 'http://localhost:8080'
});

// Chat completion
const response = await client.chat.create({
  model: 'superagent-ensemble',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'What is the capital of France?' }
  ]
});

console.log(response.choices[0].message.content);
```

## Streaming

```typescript
// Stream responses
const stream = client.chat.createStream({
  model: 'superagent-ensemble',
  messages: [{ role: 'user', content: 'Tell me a story' }]
});

for await (const chunk of stream) {
  if (chunk.choices[0].delta.content) {
    process.stdout.write(chunk.choices[0].delta.content);
  }
}
```

## Ensemble Mode

```typescript
const response = await client.chat.create({
  model: 'superagent-ensemble',
  messages: [{ role: 'user', content: 'Complex question' }],
  ensemble_config: {
    strategy: 'confidence_weighted',
    min_providers: 2,
    confidence_threshold: 0.8,
    preferred_providers: ['openai', 'anthropic']
  }
});

console.log('Selected provider:', response.ensemble?.selected_provider);
console.log('Confidence:', response.ensemble?.selection_score);
```

## AI Debates

```typescript
// Create a debate
const debate = await client.debates.create({
  topic: 'Should AI have ethical constraints built in?',
  participants: [
    { name: 'EthicsExpert', role: 'proponent', llm_provider: 'anthropic' },
    { name: 'PragmaticAI', role: 'critic', llm_provider: 'openai' }
  ],
  max_rounds: 3
});

console.log('Debate created:', debate.debate_id);

// Wait for completion
const result = await client.debates.waitForCompletion(debate.debate_id, {
  pollInterval: 5000,
  timeout: 300000
});

console.log('Consensus reached:', result.consensus?.reached);
console.log('Final position:', result.consensus?.final_position);

// Or poll manually
const status = await client.debates.getStatus(debate.debate_id);
if (status.status === 'completed') {
  const results = await client.debates.getResults(debate.debate_id);
  console.log('Results:', results);
}

// List all debates
const allDebates = await client.debates.list('completed');

// Delete a debate
await client.debates.delete(debate.debate_id);
```

## Configuration

```typescript
// Via constructor
const client = new SuperAgent({
  apiKey: 'your-key',
  baseUrl: 'http://localhost:8080',
  timeout: 60000,
  maxRetries: 3,
  headers: { 'X-Custom-Header': 'value' }
});

// Via environment variable
// SUPERAGENT_API_KEY=your-key
const client = new SuperAgent(); // Uses env var
```

## API Reference

### Client Methods

```typescript
// Health check
const health = await client.health();

// List providers
const providers = await client.providers();

// Provider details
const details = await client.providerDetails();

// Provider health
const providerHealth = await client.providerHealth('openai');
```

### Chat Completions

```typescript
const response = await client.chat.create({
  model: 'model-name',
  messages: [...],
  temperature: 0.7,
  max_tokens: 1000,
  top_p: 1.0,
  stop: ['STOP'],
  stream: false
});

// Streaming
for await (const chunk of client.chat.createStream({...})) {
  // Handle chunk
}
```

### Text Completions

```typescript
const response = await client.completions.create({
  model: 'model-name',
  prompt: 'Complete this...',
  max_tokens: 100
});
```

### Models

```typescript
// List models
const models = await client.models.list();

// Get specific model
const model = await client.models.retrieve('gpt-4');
```

### Debates

```typescript
// Create debate
const debate = await client.debates.create(config);

// Get debate info
const info = await client.debates.get(debateId);

// Get status
const status = await client.debates.getStatus(debateId);

// Get results (when completed)
const results = await client.debates.getResults(debateId);

// List debates
const list = await client.debates.list(status?);

// Delete debate
await client.debates.delete(debateId);

// Wait for completion (with polling)
const result = await client.debates.waitForCompletion(debateId, { pollInterval, timeout });
```

## Error Handling

```typescript
import {
  SuperAgentError,
  AuthenticationError,
  RateLimitError,
  APIError,
  TimeoutError,
  NetworkError
} from 'superagent-sdk';

try {
  const response = await client.chat.create({...});
} catch (error) {
  if (error instanceof AuthenticationError) {
    console.error('Auth failed:', error.message);
  } else if (error instanceof RateLimitError) {
    console.error('Rate limited. Retry after:', error.retryAfter, 'seconds');
  } else if (error instanceof TimeoutError) {
    console.error('Request timed out');
  } else if (error instanceof APIError) {
    console.error(`API error [${error.statusCode}]:`, error.message);
  } else if (error instanceof NetworkError) {
    console.error('Network error:', error.message);
  }
}
```

## OpenAI Compatibility

SuperAgent is fully compatible with the OpenAI API format. You can use the official OpenAI JavaScript client:

```typescript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'your-superagent-key',
  baseURL: 'http://localhost:8080/v1'
});

const response = await openai.chat.completions.create({
  model: 'superagent-ensemble',
  messages: [{ role: 'user', content: 'Hello!' }]
});
```

## Browser Usage

```html
<script src="https://unpkg.com/superagent-sdk/dist/superagent.browser.js"></script>
<script>
const client = new SuperAgent({
  apiKey: 'your-key',
  baseUrl: 'https://api.superagent.ai'
});

client.chat.create({
  model: 'superagent-ensemble',
  messages: [{ role: 'user', content: 'Hello!' }]
}).then(response => {
  console.log(response.choices[0].message.content);
});
</script>
```

## TypeScript Support

Full TypeScript support with type definitions included:

```typescript
import {
  SuperAgent,
  ChatCompletionRequest,
  ChatCompletionResponse,
  CreateDebateRequest,
  DebateResult
} from 'superagent-sdk';

const request: ChatCompletionRequest = {
  model: 'superagent-ensemble',
  messages: [{ role: 'user', content: 'Hello' }]
};

const response: ChatCompletionResponse = await client.chat.create(request);
```

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Lint
npm run lint

# Format
npm run format
```

## License

MIT License
