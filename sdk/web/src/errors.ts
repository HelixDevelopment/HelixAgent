/**
 * SuperAgent SDK Error Classes
 */

export class SuperAgentError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'SuperAgentError';
    Object.setPrototypeOf(this, SuperAgentError.prototype);
  }
}

export class AuthenticationError extends SuperAgentError {
  constructor(message: string = 'Authentication failed') {
    super(message);
    this.name = 'AuthenticationError';
    Object.setPrototypeOf(this, AuthenticationError.prototype);
  }
}

export class RateLimitError extends SuperAgentError {
  retryAfter: number | null;

  constructor(message: string = 'Rate limit exceeded', retryAfter: number | null = null) {
    super(message);
    this.name = 'RateLimitError';
    this.retryAfter = retryAfter;
    Object.setPrototypeOf(this, RateLimitError.prototype);
  }
}

export class APIError extends SuperAgentError {
  statusCode: number;
  type?: string;
  param?: string;
  code?: string;

  constructor(
    message: string,
    statusCode: number,
    details?: { type?: string; param?: string; code?: string }
  ) {
    super(message);
    this.name = 'APIError';
    this.statusCode = statusCode;
    this.type = details?.type;
    this.param = details?.param;
    this.code = details?.code;
    Object.setPrototypeOf(this, APIError.prototype);
  }
}

export class ValidationError extends SuperAgentError {
  constructor(message: string = 'Validation failed') {
    super(message);
    this.name = 'ValidationError';
    Object.setPrototypeOf(this, ValidationError.prototype);
  }
}

export class NetworkError extends SuperAgentError {
  constructor(message: string = 'Network error occurred') {
    super(message);
    this.name = 'NetworkError';
    Object.setPrototypeOf(this, NetworkError.prototype);
  }
}

export class TimeoutError extends SuperAgentError {
  constructor(message: string = 'Request timeout') {
    super(message);
    this.name = 'TimeoutError';
    Object.setPrototypeOf(this, TimeoutError.prototype);
  }
}

export class ProviderError extends SuperAgentError {
  provider: string;

  constructor(message: string, provider: string) {
    super(message);
    this.name = 'ProviderError';
    this.provider = provider;
    Object.setPrototypeOf(this, ProviderError.prototype);
  }
}

export class DebateError extends SuperAgentError {
  debateId: string;

  constructor(message: string, debateId: string) {
    super(message);
    this.name = 'DebateError';
    this.debateId = debateId;
    Object.setPrototypeOf(this, DebateError.prototype);
  }
}
