# Azure OpenAI Provider Setup Guide

## Overview

Azure OpenAI Service provides access to OpenAI's powerful language models including GPT-4, GPT-3.5, and other models through Microsoft Azure's enterprise-grade cloud infrastructure. SuperAgent integrates with Azure OpenAI to provide secure, compliant access to these models.

### Supported Models

**GPT-4 Family:**
- `gpt-4` - Most capable model
- `gpt-4-32k` - Extended context window
- `gpt-4-turbo` - Faster, cheaper GPT-4
- `gpt-4o` - Latest multimodal model

**GPT-3.5 Family:**
- `gpt-35-turbo` - Fast, cost-effective
- `gpt-35-turbo-16k` - Extended context

**Other Models:**
- `text-embedding-ada-002` - Text embeddings
- `dall-e-3` - Image generation

### Key Features

- Enterprise security and compliance
- Azure Active Directory integration
- Private endpoints and VNet support
- Regional data residency
- Content filtering and safety
- Managed availability and SLA

## Prerequisites

Before setting up Azure OpenAI:

1. An Azure subscription with billing enabled
2. Access to Azure OpenAI Service (requires application approval)
3. Azure CLI installed (optional but recommended)
4. Deployed model in your Azure OpenAI resource

## API Key Setup

### Step 1: Request Access to Azure OpenAI

1. Visit [Azure OpenAI Access Request](https://aka.ms/oai/access)
2. Fill out the application form
3. Wait for Microsoft approval (typically 1-5 business days)
4. You'll receive an email when approved

### Step 2: Create Azure OpenAI Resource

1. Go to [Azure Portal](https://portal.azure.com)
2. Search for "Azure OpenAI"
3. Click **Create**
4. Configure:
   - Subscription
   - Resource group
   - Region
   - Name (this becomes your endpoint)
   - Pricing tier
5. Click **Review + create** then **Create**

### Step 3: Deploy a Model

1. Go to your Azure OpenAI resource
2. Click **Model deployments** > **Manage Deployments**
3. Or go to [Azure OpenAI Studio](https://oai.azure.com)
4. Click **Deployments** > **Create new deployment**
5. Select a model (e.g., gpt-4)
6. Name your deployment
7. Set the tokens-per-minute limit
8. Click **Create**

### Step 4: Get Your API Key and Endpoint

1. In your Azure OpenAI resource, go to **Keys and Endpoint**
2. Copy **KEY 1** or **KEY 2**
3. Copy the **Endpoint** URL

### Step 5: Store Your Credentials Securely

```bash
# Add to your environment or .env file
export AZURE_OPENAI_ENDPOINT=https://your-resource-name.openai.azure.com
export AZURE_OPENAI_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
export AZURE_OPENAI_API_VERSION=2024-02-01
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
AZURE_OPENAI_ENDPOINT=https://your-resource-name.openai.azure.com
AZURE_OPENAI_API_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Optional
AZURE_OPENAI_API_VERSION=2024-02-01
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AZURE_OPENAI_ENDPOINT` | Yes | - | Your Azure OpenAI endpoint URL |
| `AZURE_OPENAI_API_KEY` | Yes | - | Your Azure OpenAI API key |
| `AZURE_OPENAI_API_VERSION` | No | `2024-02-01` | API version to use |

## Basic Usage Example

### Go Code Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/sirupsen/logrus"
    "github.com/superagent/superagent/internal/cloud"
)

func main() {
    logger := logrus.New()

    // Create Azure OpenAI integration
    azureAI := cloud.NewAzureOpenAIIntegration(
        os.Getenv("AZURE_OPENAI_ENDPOINT"),
        logger,
    )

    // Invoke a deployed model
    ctx := context.Background()
    response, err := azureAI.InvokeModel(
        ctx,
        "my-gpt4-deployment",  // Your deployment name
        "Explain quantum computing in simple terms.",
        map[string]interface{}{
            "max_tokens":  1024,
            "temperature": 0.7,
            "top_p":       0.9,
        },
    )
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Response: %s\n", response)
}
```

### List Deployments

```go
deployments, err := azureAI.ListModels(ctx)
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}

for _, deployment := range deployments {
    fmt.Printf("Deployment: %s (Model: %s)\n",
        deployment["id"],
        deployment["model"])
}
```

### With Custom Configuration

```go
config := cloud.AzureOpenAIConfig{
    Endpoint:   os.Getenv("AZURE_OPENAI_ENDPOINT"),
    APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
    APIVersion: "2024-02-01",
    Timeout:    120 * time.Second,
}

azureAI := cloud.NewAzureOpenAIIntegrationWithConfig(config, logger)
```

## Rate Limits and Quotas

### Default Quotas (per deployment)

| Resource | Default Limit |
|----------|---------------|
| Tokens per minute (TPM) | 120,000 |
| Requests per minute | 720 |
| Max tokens per request | Model-specific |

### Model-Specific Limits

| Model | Max Context | Max Output |
|-------|-------------|------------|
| GPT-4 | 8,192 | 4,096 |
| GPT-4-32k | 32,768 | 4,096 |
| GPT-4-Turbo | 128,000 | 4,096 |
| GPT-35-Turbo | 4,096 | 4,096 |
| GPT-35-Turbo-16k | 16,384 | 4,096 |

### Adjusting Quotas

1. Go to your Azure OpenAI resource
2. Navigate to **Model deployments**
3. Select your deployment
4. Click **Edit** to adjust TPM quota
5. Or request a quota increase through Azure Support

## Azure-Specific Features

### Content Filtering

Azure OpenAI includes content filtering by default:

| Category | Default Action |
|----------|---------------|
| Hate | Filter |
| Violence | Filter |
| Sexual | Filter |
| Self-harm | Filter |

You can customize content filtering in Azure OpenAI Studio.

### Private Endpoints

For network isolation:

1. Go to **Networking** in your Azure OpenAI resource
2. Select **Private endpoint**
3. Create a private endpoint in your VNet
4. Update DNS settings
5. Disable public access if needed

### Managed Identity

Use managed identity instead of API keys:

```go
// Use Azure AD authentication
// Set up managed identity on your Azure resource
// No API key needed
```

## Troubleshooting

### Common Errors

#### Authentication Error (401)

```
Azure OpenAI API error: 401 - Unauthorized
```

**Solution:**
- Verify your API key is correct
- Check that the key hasn't been regenerated
- Ensure the API key header is `api-key` (not `Authorization`)

#### Resource Not Found (404)

```
Azure OpenAI API error: 404 - Resource not found
```

**Solution:**
- Verify the endpoint URL is correct
- Check that the deployment name exists
- Ensure the deployment is fully provisioned

#### Deployment Not Found (404)

```
Azure OpenAI API error: 404 - DeploymentNotFound
```

**Solution:**
- Verify the deployment name matches exactly
- Check that the model is deployed
- Ensure you're using the correct API version

#### Rate Limit (429)

```
Azure OpenAI API error: 429 - Rate limit exceeded
```

**Solution:**
- Wait for the rate limit window to reset
- SuperAgent automatically retries with exponential backoff
- Increase your TPM quota
- Use multiple deployments for load distribution

#### Content Filtered (400)

```
Azure OpenAI API error: 400 - Content filtered
```

**Solution:**
- Review your prompt for potentially filtered content
- Adjust content filter settings in Azure OpenAI Studio
- Rephrase the request

#### Quota Exceeded (429)

```
Azure OpenAI API error: 429 - Quota exceeded
```

**Solution:**
- Check your subscription quota
- Request a quota increase through Azure Support
- Use a different region with available quota

### Health Check

```go
err := azureAI.HealthCheck(ctx)
if err != nil {
    fmt.Printf("Azure OpenAI unhealthy: %v\n", err)
}
```

### Debug Logging

Enable debug logging:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### API Version Compatibility

Use a supported API version:

| API Version | Status |
|-------------|--------|
| 2024-02-01 | Stable (default) |
| 2024-05-01 | Latest features |
| 2023-12-01-preview | Preview features |

## Regional Availability

Azure OpenAI is available in select regions:

| Region | GPT-4 | GPT-4 Turbo | GPT-35 |
|--------|-------|-------------|--------|
| East US | Yes | Yes | Yes |
| West US 2 | Yes | Yes | Yes |
| West Europe | Yes | Yes | Yes |
| UK South | Yes | Yes | Yes |
| Australia East | Yes | Yes | Yes |

Check [Azure OpenAI regions](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/models#model-summary-table-and-region-availability) for the latest availability.

## Cost Management

### Pricing Structure

Azure OpenAI uses pay-per-use pricing:

| Model | Input (per 1K tokens) | Output (per 1K tokens) |
|-------|----------------------|------------------------|
| GPT-4 | $0.03 | $0.06 |
| GPT-4-32k | $0.06 | $0.12 |
| GPT-4 Turbo | $0.01 | $0.03 |
| GPT-35-Turbo | $0.0005 | $0.0015 |

### Cost Optimization Tips

1. **Use GPT-35-Turbo** for simple tasks
2. **Set max_tokens limits** to control costs
3. **Monitor usage** in Azure Cost Management
4. **Use provisioned throughput** for predictable workloads
5. **Cache responses** for repeated queries

### Setting Up Cost Alerts

1. Go to **Cost Management + Billing** in Azure Portal
2. Select your subscription
3. Click **Budgets** > **Add**
4. Set your budget threshold
5. Configure email alerts

## Enterprise Security

### Network Security

```bash
# Restrict to specific IP ranges
# Configure in Azure Portal > Networking
```

### Data Privacy

- Data is not used to train models
- Customer data stays within your Azure tenant
- Content filtering protects against harmful content

### Compliance

Azure OpenAI is compliant with:
- SOC 2
- ISO 27001
- HIPAA (with BAA)
- GDPR

## Additional Resources

- [Azure OpenAI Documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Azure OpenAI Studio](https://oai.azure.com)
- [Azure OpenAI Pricing](https://azure.microsoft.com/en-us/pricing/details/cognitive-services/openai-service/)
- [API Reference](https://learn.microsoft.com/en-us/azure/ai-services/openai/reference)
- [Best Practices](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/advanced-prompt-engineering)
