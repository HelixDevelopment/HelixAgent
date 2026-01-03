/**
 * SuperAgent SDK Client
 */

import type {
  SuperAgentConfig,
  ChatCompletionRequest,
  ChatCompletionResponse,
  ChatCompletionChunk,
  CompletionRequest,
  CompletionResponse,
  CreateDebateRequest,
  DebateResponse,
  DebateStatus,
  DebateResult,
  Model,
  ModelListResponse,
  Provider,
  ProviderListResponse,
  ProviderHealth,
  HealthResponse,
  ErrorResponse,
} from './types';

import {
  SuperAgentError,
  AuthenticationError,
  RateLimitError,
  APIError,
  TimeoutError,
  NetworkError,
} from './errors';

const DEFAULT_BASE_URL = 'http://localhost:8080';
const DEFAULT_TIMEOUT = 60000;
const DEFAULT_MAX_RETRIES = 3;

export class SuperAgent {
  private apiKey?: string;
  private baseUrl: string;
  private timeout: number;
  private maxRetries: number;
  private headers: Record<string, string>;

  public readonly chat: ChatCompletions;
  public readonly completions: Completions;
  public readonly debates: Debates;
  public readonly models: Models;

  constructor(config: SuperAgentConfig = {}) {
    this.apiKey = config.apiKey || process.env.SUPERAGENT_API_KEY;
    this.baseUrl = (config.baseUrl || DEFAULT_BASE_URL).replace(/\/$/, '');
    this.timeout = config.timeout || DEFAULT_TIMEOUT;
    this.maxRetries = config.maxRetries || DEFAULT_MAX_RETRIES;
    this.headers = {
      'Content-Type': 'application/json',
      ...config.headers,
    };

    if (this.apiKey) {
      this.headers['Authorization'] = `Bearer ${this.apiKey}`;
    }

    // Initialize sub-modules
    this.chat = new ChatCompletions(this);
    this.completions = new Completions(this);
    this.debates = new Debates(this);
    this.models = new Models(this);
  }

  async request<T>(
    endpoint: string,
    options: {
      method?: string;
      body?: unknown;
      headers?: Record<string, string>;
      timeout?: number;
    } = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const method = options.method || 'GET';
    const controller = new AbortController();
    const timeoutId = setTimeout(
      () => controller.abort(),
      options.timeout || this.timeout
    );

    try {
      const fetchOptions: RequestInit = {
        method,
        headers: { ...this.headers, ...options.headers },
        signal: controller.signal,
      };

      if (options.body) {
        fetchOptions.body = JSON.stringify(options.body);
      }

      const response = await fetch(url, fetchOptions);
      clearTimeout(timeoutId);

      if (!response.ok) {
        await this.handleErrorResponse(response);
      }

      return (await response.json()) as T;
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof SuperAgentError) {
        throw error;
      }

      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new TimeoutError(`Request timeout after ${this.timeout}ms`);
        }
        throw new NetworkError(error.message);
      }

      throw new SuperAgentError('Unknown error occurred');
    }
  }

  async *streamRequest(
    endpoint: string,
    body: unknown
  ): AsyncGenerator<ChatCompletionChunk, void, unknown> {
    const url = `${this.baseUrl}${endpoint}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          ...this.headers,
          Accept: 'text/event-stream',
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        await this.handleErrorResponse(response);
      }

      if (!response.body) {
        throw new NetworkError('Response body is empty');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();

        if (done) {
          break;
        }

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6).trim();

            if (data === '[DONE]') {
              return;
            }

            try {
              const chunk = JSON.parse(data) as ChatCompletionChunk;
              yield chunk;
            } catch {
              // Skip invalid JSON
            }
          }
        }
      }
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof SuperAgentError) {
        throw error;
      }

      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new TimeoutError(`Stream timeout after ${this.timeout}ms`);
        }
        throw new NetworkError(error.message);
      }

      throw new SuperAgentError('Unknown stream error');
    }
  }

  private async handleErrorResponse(response: Response): Promise<never> {
    let errorData: ErrorResponse | null = null;

    try {
      errorData = (await response.json()) as ErrorResponse;
    } catch {
      // Response wasn't JSON
    }

    const message = errorData?.error?.message || response.statusText;

    switch (response.status) {
      case 401:
        throw new AuthenticationError(message);
      case 429:
        const retryAfter = response.headers.get('Retry-After');
        throw new RateLimitError(
          message,
          retryAfter ? parseInt(retryAfter, 10) : null
        );
      default:
        throw new APIError(message, response.status, {
          type: errorData?.error?.type,
          param: errorData?.error?.param,
          code: errorData?.error?.code,
        });
    }
  }

  async health(): Promise<HealthResponse> {
    return this.request<HealthResponse>('/health');
  }

  async providers(): Promise<string[]> {
    const response = await this.request<{ providers: string[] }>('/v1/providers');
    return response.providers;
  }

  async providerDetails(): Promise<ProviderListResponse> {
    return this.request<ProviderListResponse>('/v1/providers');
  }

  async providerHealth(name: string): Promise<ProviderHealth> {
    return this.request<ProviderHealth>(`/v1/providers/${name}/health`);
  }
}

class ChatCompletions {
  constructor(private client: SuperAgent) {}

  async create(request: ChatCompletionRequest): Promise<ChatCompletionResponse> {
    if (request.stream) {
      throw new SuperAgentError(
        'Use createStream() for streaming completions'
      );
    }

    return this.client.request<ChatCompletionResponse>('/v1/chat/completions', {
      method: 'POST',
      body: request,
    });
  }

  async *createStream(
    request: Omit<ChatCompletionRequest, 'stream'>
  ): AsyncGenerator<ChatCompletionChunk, void, unknown> {
    yield* this.client.streamRequest('/v1/chat/completions', {
      ...request,
      stream: true,
    });
  }
}

class Completions {
  constructor(private client: SuperAgent) {}

  async create(request: CompletionRequest): Promise<CompletionResponse> {
    if (request.stream) {
      throw new SuperAgentError(
        'Use createStream() for streaming completions'
      );
    }

    return this.client.request<CompletionResponse>('/v1/completions', {
      method: 'POST',
      body: request,
    });
  }

  async *createStream(
    request: Omit<CompletionRequest, 'stream'>
  ): AsyncGenerator<unknown, void, unknown> {
    yield* this.client.streamRequest('/v1/completions', {
      ...request,
      stream: true,
    });
  }
}

class Debates {
  constructor(private client: SuperAgent) {}

  async create(request: CreateDebateRequest): Promise<DebateResponse> {
    return this.client.request<DebateResponse>('/v1/debates', {
      method: 'POST',
      body: request,
    });
  }

  async get(debateId: string): Promise<DebateResponse> {
    return this.client.request<DebateResponse>(`/v1/debates/${debateId}`);
  }

  async getStatus(debateId: string): Promise<DebateStatus> {
    return this.client.request<DebateStatus>(`/v1/debates/${debateId}/status`);
  }

  async getResults(debateId: string): Promise<DebateResult> {
    return this.client.request<DebateResult>(`/v1/debates/${debateId}/results`);
  }

  async list(status?: string): Promise<{ debates: DebateResponse[]; count: number }> {
    const query = status ? `?status=${status}` : '';
    return this.client.request<{ debates: DebateResponse[]; count: number }>(
      `/v1/debates${query}`
    );
  }

  async delete(debateId: string): Promise<{ message: string; debate_id: string }> {
    return this.client.request<{ message: string; debate_id: string }>(
      `/v1/debates/${debateId}`,
      { method: 'DELETE' }
    );
  }

  async waitForCompletion(
    debateId: string,
    options: { pollInterval?: number; timeout?: number } = {}
  ): Promise<DebateResult> {
    const pollInterval = options.pollInterval || 5000;
    const timeout = options.timeout || 600000; // 10 minutes default
    const startTime = Date.now();

    while (Date.now() - startTime < timeout) {
      const status = await this.getStatus(debateId);

      if (status.status === 'completed') {
        return this.getResults(debateId);
      }

      if (status.status === 'failed') {
        throw new SuperAgentError(
          `Debate ${debateId} failed: ${status.error || 'Unknown error'}`
        );
      }

      await new Promise((resolve) => setTimeout(resolve, pollInterval));
    }

    throw new TimeoutError(`Debate ${debateId} did not complete within timeout`);
  }
}

class Models {
  constructor(private client: SuperAgent) {}

  async list(): Promise<Model[]> {
    const response = await this.client.request<ModelListResponse>('/v1/models');
    return response.data;
  }

  async retrieve(modelId: string): Promise<Model> {
    return this.client.request<Model>(`/v1/models/${modelId}`);
  }
}

export default SuperAgent;
