/**
 * SuperAgent SDK Tests
 */

import { SuperAgent } from '../src/client';
import {
  AuthenticationError,
  RateLimitError,
  APIError,
  TimeoutError,
} from '../src/errors';

// Mock fetch globally
const mockFetch = jest.fn();
global.fetch = mockFetch as unknown as typeof fetch;

describe('SuperAgent', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('constructor', () => {
    it('should create client with default config', () => {
      const client = new SuperAgent();
      expect(client).toBeInstanceOf(SuperAgent);
    });

    it('should create client with custom config', () => {
      const client = new SuperAgent({
        apiKey: 'test-key',
        baseUrl: 'https://custom.api.com',
        timeout: 30000,
      });
      expect(client).toBeInstanceOf(SuperAgent);
    });

    it('should have chat, completions, debates, and models sub-modules', () => {
      const client = new SuperAgent();
      expect(client.chat).toBeDefined();
      expect(client.completions).toBeDefined();
      expect(client.debates).toBeDefined();
      expect(client.models).toBeDefined();
    });
  });

  describe('health', () => {
    it('should return health status', async () => {
      const mockResponse = {
        status: 'healthy',
        providers: { total: 3, healthy: 3, unhealthy: 0 },
        timestamp: Date.now(),
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent();
      const health = await client.health();

      expect(health.status).toBe('healthy');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/health',
        expect.any(Object)
      );
    });
  });

  describe('providers', () => {
    it('should return list of providers', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ providers: ['openai', 'anthropic', 'google'] }),
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const providers = await client.providers();

      expect(providers).toEqual(['openai', 'anthropic', 'google']);
    });
  });

  describe('chat.completions', () => {
    it('should create chat completion', async () => {
      const mockResponse = {
        id: 'chatcmpl-123',
        object: 'chat.completion',
        created: Date.now(),
        model: 'superagent-ensemble',
        choices: [
          {
            index: 0,
            message: {
              role: 'assistant',
              content: 'Hello! How can I help you?',
            },
            finish_reason: 'stop',
          },
        ],
        usage: {
          prompt_tokens: 10,
          completion_tokens: 8,
          total_tokens: 18,
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
      });

      expect(response.id).toBe('chatcmpl-123');
      expect(response.choices[0].message.content).toBe('Hello! How can I help you?');
    });

    it('should throw error when using stream:true with create()', async () => {
      const client = new SuperAgent({ apiKey: 'test-key' });

      await expect(
        client.chat.create({
          model: 'test',
          messages: [{ role: 'user', content: 'test' }],
          stream: true,
        })
      ).rejects.toThrow('Use createStream() for streaming completions');
    });
  });

  describe('debates', () => {
    it('should create debate', async () => {
      const mockResponse = {
        debate_id: 'debate-123',
        status: 'pending',
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
      const debate = await client.debates.create({
        topic: 'Test topic',
        participants: [{ name: 'Alice' }, { name: 'Bob' }],
      });

      expect(debate.debate_id).toBe('debate-123');
      expect(debate.status).toBe('pending');
    });

    it('should get debate status', async () => {
      const mockResponse = {
        debate_id: 'debate-123',
        status: 'running',
        start_time: Date.now(),
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const status = await client.debates.getStatus('debate-123');

      expect(status.status).toBe('running');
    });

    it('should list debates', async () => {
      const mockResponse = {
        debates: [
          { debate_id: 'debate-1', topic: 'Topic 1', status: 'completed' },
          { debate_id: 'debate-2', topic: 'Topic 2', status: 'running' },
        ],
        count: 2,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const result = await client.debates.list();

      expect(result.count).toBe(2);
      expect(result.debates).toHaveLength(2);
    });

    it('should delete debate', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ message: 'Debate deleted', debate_id: 'debate-123' }),
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const result = await client.debates.delete('debate-123');

      expect(result.message).toBe('Debate deleted');
    });
  });

  describe('models', () => {
    it('should list models', async () => {
      const mockResponse = {
        object: 'list',
        data: [
          { id: 'gpt-4', object: 'model', created: Date.now(), owned_by: 'openai' },
          { id: 'claude-3', object: 'model', created: Date.now(), owned_by: 'anthropic' },
        ],
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      });

      const client = new SuperAgent({ apiKey: 'test-key' });
      const models = await client.models.list();

      expect(models).toHaveLength(2);
      expect(models[0].id).toBe('gpt-4');
    });
  });

  describe('error handling', () => {
    it('should throw AuthenticationError on 401', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        headers: new Map(),
        json: async () => ({ error: { message: 'Invalid API key' } }),
      });

      const client = new SuperAgent();

      await expect(client.health()).rejects.toThrow(AuthenticationError);
    });

    it('should throw RateLimitError on 429', async () => {
      const headers = new Map([['Retry-After', '60']]);
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 429,
        statusText: 'Too Many Requests',
        headers: { get: (key: string) => headers.get(key) },
        json: async () => ({ error: { message: 'Rate limit exceeded' } }),
      });

      const client = new SuperAgent({ apiKey: 'test-key' });

      try {
        await client.health();
        fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(RateLimitError);
        expect((error as RateLimitError).retryAfter).toBe(60);
      }
    });

    it('should throw APIError on other errors', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        headers: new Map(),
        json: async () => ({ error: { message: 'Server error', type: 'internal_error' } }),
      });

      const client = new SuperAgent({ apiKey: 'test-key' });

      try {
        await client.health();
        fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(APIError);
        expect((error as APIError).statusCode).toBe(500);
      }
    });
  });
});

describe('Error classes', () => {
  it('AuthenticationError should extend SuperAgentError', () => {
    const error = new AuthenticationError('test');
    expect(error.name).toBe('AuthenticationError');
    expect(error.message).toBe('test');
  });

  it('RateLimitError should include retryAfter', () => {
    const error = new RateLimitError('rate limited', 60);
    expect(error.retryAfter).toBe(60);
  });

  it('APIError should include statusCode', () => {
    const error = new APIError('server error', 500, { type: 'internal' });
    expect(error.statusCode).toBe(500);
    expect(error.type).toBe('internal');
  });

  it('TimeoutError should have correct name', () => {
    const error = new TimeoutError('timed out');
    expect(error.name).toBe('TimeoutError');
  });
});
