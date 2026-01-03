/**
 * SuperAgent SDK Type Definitions
 */

// ==================== Configuration ====================

export interface SuperAgentConfig {
  apiKey?: string;
  baseUrl?: string;
  timeout?: number;
  maxRetries?: number;
  headers?: Record<string, string>;
}

// ==================== Chat Completions ====================

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant' | 'function' | 'tool';
  content: string | null;
  name?: string;
  function_call?: FunctionCall;
  tool_calls?: ToolCall[];
}

export interface FunctionCall {
  name: string;
  arguments: string;
}

export interface ToolCall {
  id: string;
  type: 'function';
  function: FunctionCall;
}

export interface ChatCompletionRequest {
  model: string;
  messages: ChatMessage[];
  temperature?: number;
  top_p?: number;
  n?: number;
  stream?: boolean;
  stop?: string | string[];
  max_tokens?: number;
  presence_penalty?: number;
  frequency_penalty?: number;
  logit_bias?: Record<string, number>;
  user?: string;
  functions?: FunctionDefinition[];
  function_call?: 'none' | 'auto' | { name: string };
  tools?: Tool[];
  tool_choice?: 'none' | 'auto' | { type: 'function'; function: { name: string } };
  response_format?: { type: 'text' | 'json_object' };
  ensemble_config?: EnsembleConfig;
}

export interface FunctionDefinition {
  name: string;
  description?: string;
  parameters?: Record<string, unknown>;
}

export interface Tool {
  type: 'function';
  function: FunctionDefinition;
}

export interface ChatCompletionResponse {
  id: string;
  object: 'chat.completion';
  created: number;
  model: string;
  choices: ChatCompletionChoice[];
  usage?: Usage;
  system_fingerprint?: string;
  ensemble?: EnsembleMetadata;
}

export interface ChatCompletionChoice {
  index: number;
  message: ChatMessage;
  finish_reason: 'stop' | 'length' | 'function_call' | 'tool_calls' | 'content_filter' | null;
  logprobs?: unknown;
}

export interface Usage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

// ==================== Streaming ====================

export interface ChatCompletionChunk {
  id: string;
  object: 'chat.completion.chunk';
  created: number;
  model: string;
  choices: ChatCompletionChunkChoice[];
  system_fingerprint?: string;
}

export interface ChatCompletionChunkChoice {
  index: number;
  delta: Partial<ChatMessage>;
  finish_reason: 'stop' | 'length' | 'function_call' | 'tool_calls' | 'content_filter' | null;
  logprobs?: unknown;
}

// ==================== Ensemble ====================

export interface EnsembleConfig {
  strategy?: 'confidence_weighted' | 'majority_vote' | 'round_robin' | 'best_of_n';
  min_providers?: number;
  confidence_threshold?: number;
  fallback_to_best?: boolean;
  timeout?: number;
  preferred_providers?: string[];
}

export interface EnsembleMetadata {
  voting_method: string;
  responses_count: number;
  scores: Record<string, number>;
  metadata: Record<string, unknown>;
  selected_provider: string;
  selection_score: number;
}

// ==================== Text Completions ====================

export interface CompletionRequest {
  model: string;
  prompt: string | string[];
  suffix?: string;
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  n?: number;
  stream?: boolean;
  logprobs?: number;
  echo?: boolean;
  stop?: string | string[];
  presence_penalty?: number;
  frequency_penalty?: number;
  best_of?: number;
  logit_bias?: Record<string, number>;
  user?: string;
}

export interface CompletionResponse {
  id: string;
  object: 'text_completion';
  created: number;
  model: string;
  choices: CompletionChoice[];
  usage?: Usage;
}

export interface CompletionChoice {
  text: string;
  index: number;
  logprobs?: unknown;
  finish_reason: 'stop' | 'length' | null;
}

// ==================== AI Debate ====================

export interface DebateParticipant {
  participant_id?: string;
  name: string;
  role?: string;
  llm_provider?: string;
  llm_model?: string;
  max_rounds?: number;
  timeout?: number;
  weight?: number;
}

export interface CreateDebateRequest {
  debate_id?: string;
  topic: string;
  participants: DebateParticipant[];
  max_rounds?: number;
  timeout?: number;
  strategy?: string;
  enable_cognee?: boolean;
  metadata?: Record<string, unknown>;
}

export interface DebateResponse {
  debate_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  topic: string;
  max_rounds: number;
  timeout: number;
  participants: number;
  created_at: number;
  message?: string;
}

export interface DebateStatus {
  debate_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  start_time: number;
  end_time?: number;
  duration_seconds?: number;
  error?: string;
  max_rounds?: number;
  timeout_seconds?: number;
}

export interface DebateResult {
  debate_id: string;
  session_id?: string;
  topic?: string;
  start_time: string;
  end_time: string;
  duration: number;
  total_rounds: number;
  rounds_conducted?: number;
  participants: ParticipantResponse[];
  all_responses?: ParticipantResponse[];
  best_response?: ParticipantResponse;
  consensus?: ConsensusResult;
  cognee_insights?: CogneeInsights;
  quality_score: number;
  final_score?: number;
  success: boolean;
}

export interface ParticipantResponse {
  participant_id: string;
  name: string;
  role: string;
  round: number;
  response: string;
  quality_score: number;
  provider: string;
  model: string;
  tokens_used: number;
  latency_ms: number;
  timestamp: string;
}

export interface ConsensusResult {
  reached: boolean;
  achieved: boolean;
  confidence: number;
  consensus_level?: number;
  agreement_level: number;
  agreement_score?: number;
  final_position: string;
  key_points: string[];
  disagreements: string[];
  summary?: string;
}

export interface CogneeInsights {
  summary?: string;
  key_entities?: string[];
  relationships?: Record<string, unknown>[];
  sentiment?: SentimentAnalysis;
}

export interface SentimentAnalysis {
  overall: string;
  confidence: number;
  breakdown?: Record<string, number>;
}

// ==================== Models ====================

export interface Model {
  id: string;
  object: 'model';
  created: number;
  owned_by: string;
  permission?: unknown[];
  root?: string;
  parent?: string;
}

export interface ModelListResponse {
  object: 'list';
  data: Model[];
}

// ==================== Providers ====================

export interface Provider {
  name: string;
  supported_models: string[];
  supported_features: string[];
  supports_streaming: boolean;
  supports_function_calling: boolean;
  supports_vision: boolean;
  metadata: Record<string, unknown>;
}

export interface ProviderListResponse {
  providers: Provider[];
  count: number;
}

export interface ProviderHealth {
  provider: string;
  healthy: boolean;
  error?: string;
  circuit_breaker?: CircuitBreakerState;
}

export interface CircuitBreakerState {
  state: string;
  failure_count: number;
  last_failure?: string;
}

// ==================== Health ====================

export interface HealthResponse {
  status: 'healthy' | 'unhealthy' | 'degraded';
  providers?: {
    total: number;
    healthy: number;
    unhealthy: number;
  };
  timestamp: number;
}

// ==================== Errors ====================

export interface APIError {
  message: string;
  type?: string;
  param?: string;
  code?: string;
}

export interface ErrorResponse {
  error: APIError;
}
