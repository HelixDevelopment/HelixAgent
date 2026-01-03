/**
 * Completions and Extended Client Tests
 */

import { SuperAgent } from '../src/client';
import { SuperAgentError } from '../src/errors';

// Mock fetch globally
const mockFetch = jest.fn();
global.fetch = mockFetch as unknown as typeof fetch;

describe('Completions', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('completions.create', () => {
    it('should create text completion', async () => {
      const mockResponse = {
        id: 'cmpl-123',
        object: 'text_completion',
        created: Date.now(),
        model: 'gpt-3.5-turbo-instruct',
        choices: [
          {
            text: 'Hello, world!',
            index: 0,
            finish_reason: 'stop',
          },
        ],
        usage: {
          prompt_tokens: 5,
          completion_tokens: 3,
          total_tokens: 8,
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const response = await client.completions.create({
        model: 'gpt-3.5-turbo-instruct',
        prompt: 'Say hello',
      });

      expect(response.id).toBe('cmpl-123');
      expect(response.choices[0].text).toBe('Hello, world!');
      expect(response.usage?.total_tokens).toBe(8);
    });

    it('should throw error when using stream:true with create()', async () => {
      const client = new SuperAgent({ apiKey: 'test-key' });

      await expect(
        client.completions.create({
          model: 'test',
          prompt: 'test',
          stream: true,
        })
      ).rejects.toThrow('Use createStream() for streaming completions');
    });

    it('should create completion with array prompt', async () => {
      const mockResponse = {
        id: 'cmpl-456',
        object: 'text_completion',
        created: Date.now(),
        model: 'gpt-3.5-turbo-instruct',
        choices: [
          { text: 'Response 1', index: 0, finish_reason: 'stop' },
          { text: 'Response 2', index: 1, finish_reason: 'stop' },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const response = await client.completions.create({
        model: 'gpt-3.5-turbo-instruct',
        prompt: ['Prompt 1', 'Prompt 2'],
      });

      expect(response.choices).toHaveLength(2);
    });

    it('should handle completion options', async () => {
      const mockResponse = {
        id: 'cmpl-789',
        object: 'text_completion',
        created: Date.now(),
        model: 'gpt-3.5-turbo-instruct',
        choices: [{ text: 'Response', index: 0, finish_reason: 'length' }],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const response = await client.completions.create({
        model: 'gpt-3.5-turbo-instruct',
        prompt: 'Test',
        max_tokens: 100,
        temperature: 0.5,
        top_p: 0.9,
        n: 1,
        stop: ['\n'],
        presence_penalty: 0.1,
        frequency_penalty: 0.1,
      });

      expect(response.choices[0].finish_reason).toBe('length');
    });
  });
});

describe('Models Extended', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('models.retrieve', () => {
    it('should retrieve a specific model', async () => {
      const mockResponse = {
        id: 'gpt-4',
        object: 'model',
        created: 1687882411,
        owned_by: 'openai',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const model = await client.models.retrieve('gpt-4');

      expect(model.id).toBe('gpt-4');
      expect(model.owned_by).toBe('openai');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/v1/models/gpt-4',
        expect.any(Object)
      );
    });
  });
});

describe('Provider Extended', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('providerDetails', () => {
    it('should return detailed provider information', async () => {
      const mockResponse = {
        providers: [
          {
            name: 'openai',
            supported_models: ['gpt-4', 'gpt-3.5-turbo'],
            supported_features: ['chat', 'completions'],
            supports_streaming: true,
            supports_function_calling: true,
            supports_vision: true,
            metadata: {},
          },
          {
            name: 'anthropic',
            supported_models: ['claude-3'],
            supported_features: ['chat'],
            supports_streaming: true,
            supports_function_calling: false,
            supports_vision: false,
            metadata: {},
          },
        ],
        count: 2,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const details = await client.providerDetails();

      expect(details.providers).toHaveLength(2);
      expect(details.providers[0].name).toBe('openai');
      expect(details.providers[0].supports_streaming).toBe(true);
    });
  });

  describe('providerHealth', () => {
    it('should return provider health status', async () => {
      const mockResponse = {
        provider: 'openai',
        healthy: true,
        circuit_breaker: {
          state: 'closed',
          failure_count: 0,
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const health = await client.providerHealth('openai');

      expect(health.provider).toBe('openai');
      expect(health.healthy).toBe(true);
      expect(health.circuit_breaker?.state).toBe('closed');
    });

    it('should return unhealthy provider status', async () => {
      const mockResponse = {
        provider: 'openai',
        healthy: false,
        error: 'Service unavailable',
        circuit_breaker: {
          state: 'open',
          failure_count: 5,
          last_failure: '2024-01-15T10:30:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const health = await client.providerHealth('openai');

      expect(health.healthy).toBe(false);
      expect(health.error).toBe('Service unavailable');
      expect(health.circuit_breaker?.state).toBe('open');
    });
  });
});

describe('Debates Extended', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('debates.get', () => {
    it('should get debate by id', async () => {
      const mockResponse = {
        debate_id: 'debate-123',
        status: 'running',
        topic: 'Test topic',
        max_rounds: 3,
        timeout: 300,
        participants: 2,
        created_at: Date.now(),
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const debate = await client.debates.get('debate-123');

      expect(debate.debate_id).toBe('debate-123');
      expect(debate.status).toBe('running');
    });
  });

  describe('debates.getResults', () => {
    it('should get debate results', async () => {
      const mockResponse = {
        debate_id: 'debate-123',
        topic: 'Test topic',
        start_time: '2024-01-15T10:00:00Z',
        end_time: '2024-01-15T10:10:00Z',
        duration: 600,
        total_rounds: 3,
        participants: [],
        quality_score: 0.85,
        success: true,
        consensus: {
          reached: true,
          achieved: true,
          confidence: 0.9,
          agreement_level: 0.85,
          final_position: 'Agreement reached',
          key_points: ['Point 1', 'Point 2'],
          disagreements: [],
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const results = await client.debates.getResults('debate-123');

      expect(results.success).toBe(true);
      expect(results.quality_score).toBe(0.85);
      expect(results.consensus?.reached).toBe(true);
    });
  });

  describe('debates.list with status filter', () => {
    it('should list debates with status filter', async () => {
      const mockResponse = {
        debates: [
          { debate_id: 'debate-1', topic: 'Topic 1', status: 'completed' },
        ],
        count: 1,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const result = await client.debates.list('completed');

      expect(result.count).toBe(1);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/v1/debates?status=completed',
        expect.any(Object)
      );
    });
  });

  describe('debates.waitForCompletion', () => {
    it('should wait for debate completion', async () => {
      const statusRunning = {
        debate_id: 'debate-123',
        status: 'running',
        start_time: Date.now(),
      };

      const statusCompleted = {
        debate_id: 'debate-123',
        status: 'completed',
        start_time: Date.now(),
        end_time: Date.now(),
      };

      const results = {
        debate_id: 'debate-123',
        success: true,
        quality_score: 0.9,
        total_rounds: 3,
        start_time: '2024-01-15T10:00:00Z',
        end_time: '2024-01-15T10:05:00Z',
        duration: 300,
        participants: [],
      };

      mockFetch
        .mockResolvedValueOnce({ ok: true, json: async () => statusRunning })
        .mockResolvedValueOnce({ ok: true, json: async () => statusCompleted })
        .mockResolvedValueOnce({ ok: true, json: async () => results });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const result = await client.debates.waitForCompletion('debate-123', {
        pollInterval: 10,
        timeout: 5000,
      });

      expect(result.success).toBe(true);
    });

    it('should throw on debate failure', async () => {
      const statusFailed = {
        debate_id: 'debate-123',
        status: 'failed',
        error: 'Provider error',
        start_time: Date.now(),
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => statusFailed,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });

      await expect(
        client.debates.waitForCompletion('debate-123', {
          pollInterval: 10,
          timeout: 1000,
        })
      ).rejects.toThrow(SuperAgentError);
    });
  });
});

describe('Chat Extended', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('chat.create with ensemble config', () => {
    it('should create chat completion with ensemble config', async () => {
      const mockResponse = {
        id: 'chatcmpl-123',
        object: 'chat.completion',
        created: Date.now(),
        model: 'superagent-ensemble',
        choices: [
          {
            index: 0,
            message: { role: 'assistant', content: 'Response' },
            finish_reason: 'stop',
          },
        ],
        ensemble: {
          voting_method: 'confidence_weighted',
          responses_count: 3,
          scores: { openai: 0.9, anthropic: 0.85 },
          metadata: {},
          selected_provider: 'openai',
          selection_score: 0.9,
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const response = await client.chat.create({
        model: 'superagent-ensemble',
        messages: [{ role: 'user', content: 'Hello!' }],
        ensemble_config: {
          strategy: 'confidence_weighted',
          min_providers: 2,
          confidence_threshold: 0.8,
          preferred_providers: ['openai', 'anthropic'],
        },
      });

      expect(response.ensemble?.voting_method).toBe('confidence_weighted');
      expect(response.ensemble?.selected_provider).toBe('openai');
    });
  });

  describe('chat.create with function calling', () => {
    it('should create chat completion with tools', async () => {
      const mockResponse = {
        id: 'chatcmpl-456',
        object: 'chat.completion',
        created: Date.now(),
        model: 'gpt-4',
        choices: [
          {
            index: 0,
            message: {
              role: 'assistant',
              content: null,
              tool_calls: [
                {
                  id: 'call_123',
                  type: 'function',
                  function: {
                    name: 'get_weather',
                    arguments: '{"location": "London"}',
                  },
                },
              ],
            },
            finish_reason: 'tool_calls',
          },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const response = await client.chat.create({
        model: 'gpt-4',
        messages: [{ role: 'user', content: 'What is the weather in London?' }],
        tools: [
          {
            type: 'function',
            function: {
              name: 'get_weather',
              description: 'Get weather for a location',
              parameters: {
                type: 'object',
                properties: {
                  location: { type: 'string' },
                },
              },
            },
          },
        ],
        tool_choice: 'auto',
      });

      expect(response.choices[0].message.tool_calls).toBeDefined();
      expect(response.choices[0].message.tool_calls?.[0].function.name).toBe('get_weather');
    });
  });
});

describe('Client Configuration', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  it('should strip trailing slash from baseUrl', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: 'healthy' }),
    });

    const client = new SuperAgent({
      apiKey: 'test-key',
      baseUrl: 'https://api.example.com/',
    });
    await client.health();

    expect(mockFetch).toHaveBeenCalledWith(
      'https://api.example.com/health',
      expect.any(Object)
    );
  });

  it('should include custom headers', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: 'healthy' }),
    });

    const client = new SuperAgent({
      apiKey: 'test-key',
      headers: {
        'X-Custom-Header': 'custom-value',
      },
    });
    await client.health();

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          'X-Custom-Header': 'custom-value',
        }),
      })
    );
  });
});
