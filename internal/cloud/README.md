# Cloud Package

The cloud package provides integration with major cloud AI providers, implementing their authentication and API protocols.

## Overview

This package supports three cloud providers:
- **AWS Bedrock** - Amazon's managed LLM service
- **GCP Vertex AI** - Google Cloud's AI platform
- **Azure OpenAI** - Microsoft's OpenAI service

## Providers

### AWS Bedrock

Implements AWS Signature V4 authentication for Bedrock models:

```go
provider := cloud.NewAWSBedrockProvider(config)
response, err := provider.Complete(ctx, request)
```

**Supported Models:**
- Claude (via Anthropic on Bedrock)
- Amazon Titan
- Llama 2
- Cohere Command

**Configuration:**
```yaml
aws:
  region: "us-east-1"
  access_key_id: "${AWS_ACCESS_KEY_ID}"
  secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
```

### GCP Vertex AI

Implements OAuth2 bearer token authentication:

```go
provider := cloud.NewGCPVertexProvider(config)
response, err := provider.Complete(ctx, request)
```

**Supported Models:**
- PaLM 2
- Gemini Pro
- Gemini Ultra

**Configuration:**
```yaml
gcp:
  project_id: "${GCP_PROJECT_ID}"
  location: "us-central1"
  access_token: "${GOOGLE_ACCESS_TOKEN}"
```

### Azure OpenAI

Implements Azure-specific API key authentication:

```go
provider := cloud.NewAzureOpenAIProvider(config)
response, err := provider.Complete(ctx, request)
```

**Supported Models:**
- GPT-4
- GPT-3.5 Turbo
- Custom fine-tuned models

**Configuration:**
```yaml
azure:
  endpoint: "${AZURE_OPENAI_ENDPOINT}"
  api_key: "${AZURE_OPENAI_API_KEY}"
  api_version: "2024-02-15-preview"
```

## Common Interface

All cloud providers implement the `CloudProvider` interface:

```go
type CloudProvider interface {
    Complete(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error)
    CompleteStream(ctx context.Context, request *CompletionRequest) (<-chan StreamChunk, error)
    GetModels(ctx context.Context) ([]Model, error)
    HealthCheck(ctx context.Context) error
}
```

## Authentication

Each provider handles authentication differently:

| Provider | Auth Method | Credentials |
|----------|-------------|-------------|
| AWS Bedrock | Signature V4 | Access Key + Secret Key |
| GCP Vertex AI | OAuth2 Bearer | Service Account / User Token |
| Azure OpenAI | API Key | API Key Header |

## Error Handling

The package provides unified error types for cloud operations:

- `ErrAuthentication` - Invalid credentials
- `ErrQuotaExceeded` - Rate limit or quota exceeded
- `ErrModelNotFound` - Requested model not available
- `ErrServiceUnavailable` - Cloud service is down
