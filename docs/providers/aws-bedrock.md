# AWS Bedrock Provider Setup Guide

## Overview

Amazon Bedrock is a fully managed service that provides access to foundation models from leading AI companies including Anthropic, Amazon, Meta, Cohere, and others through a unified API. SuperAgent integrates with AWS Bedrock to provide enterprise-grade access to these models.

### Supported Models

**Anthropic Claude:**
- `anthropic.claude-3-sonnet-20240229-v1:0`
- `anthropic.claude-3-haiku-20240307-v1:0`
- `anthropic.claude-v2:1`

**Amazon Titan:**
- `amazon.titan-text-express-v1`
- `amazon.titan-text-lite-v1`

**Meta Llama:**
- `meta.llama3-70b-instruct-v1:0`
- `meta.llama3-8b-instruct-v1:0`

**Cohere:**
- `cohere.command-text-v14`
- `cohere.command-light-text-v14`

### Key Features

- Enterprise-grade security and compliance
- AWS IAM integration
- VPC support for private deployments
- Serverless - no infrastructure to manage
- Pay-per-use pricing
- Model customization and fine-tuning

## Prerequisites

Before setting up AWS Bedrock:

1. An AWS account with billing enabled
2. IAM permissions for Bedrock access
3. AWS CLI installed and configured
4. Model access enabled in the Bedrock console

## API Key Setup

### Step 1: Create an AWS Account

1. Visit [aws.amazon.com](https://aws.amazon.com)
2. Click **Create an AWS Account**
3. Complete the registration process
4. Set up billing information

### Step 2: Enable Bedrock Model Access

1. Go to the [AWS Bedrock Console](https://console.aws.amazon.com/bedrock)
2. Navigate to **Model access** in the left sidebar
3. Click **Edit** to modify access
4. Select the models you want to use
5. Submit the request (some models require approval)
6. Wait for access to be granted

### Step 3: Create IAM Credentials

#### Option A: IAM User with Access Keys

1. Go to [IAM Console](https://console.aws.amazon.com/iam)
2. Navigate to **Users** > **Add users**
3. Create a new user with programmatic access
4. Attach the `AmazonBedrockFullAccess` policy
5. Download the access key ID and secret access key

#### Option B: IAM Role (Recommended for EC2/ECS)

1. Create an IAM role with `AmazonBedrockFullAccess`
2. Attach the role to your EC2 instance or ECS task
3. No explicit credentials needed - uses instance metadata

### Step 4: Store Your Credentials Securely

```bash
# Add to your environment or .env file
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-east-1

# Optional: Session token for temporary credentials
export AWS_SESSION_TOKEN=your_session_token_here
```

## Environment Variable Configuration

Add the following to your `.env` file or environment:

```bash
# Required
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_REGION=us-east-1

# Optional
AWS_SESSION_TOKEN=your_session_token_here
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AWS_ACCESS_KEY_ID` | Yes* | - | AWS access key ID |
| `AWS_SECRET_ACCESS_KEY` | Yes* | - | AWS secret access key |
| `AWS_REGION` | Yes | - | AWS region (e.g., us-east-1) |
| `AWS_SESSION_TOKEN` | No | - | Session token for temporary credentials |

*Not required if using IAM roles

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

    // Create Bedrock integration
    bedrock := cloud.NewAWSBedrockIntegration(
        os.Getenv("AWS_REGION"),
        logger,
    )

    // Invoke Claude on Bedrock
    ctx := context.Background()
    response, err := bedrock.InvokeModel(
        ctx,
        "anthropic.claude-3-sonnet-20240229-v1:0",
        "Explain machine learning in simple terms.",
        map[string]interface{}{
            "max_tokens":  1024,
            "temperature": 0.7,
        },
    )
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Response: %s\n", response)
}
```

### List Available Models

```go
models, err := bedrock.ListModels(ctx)
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return
}

for _, model := range models {
    fmt.Printf("Model: %s (%s)\n", model["name"], model["provider"])
}
```

### Using Different Model Families

```go
// Amazon Titan
response, err := bedrock.InvokeModel(
    ctx,
    "amazon.titan-text-express-v1",
    "Write a product description.",
    map[string]interface{}{
        "max_tokens":  512,
        "temperature": 0.7,
        "top_p":       0.9,
    },
)

// Meta Llama
response, err := bedrock.InvokeModel(
    ctx,
    "meta.llama3-70b-instruct-v1:0",
    "Explain quantum computing.",
    map[string]interface{}{
        "max_tokens":  1024,
        "temperature": 0.7,
        "top_p":       0.9,
    },
)

// Cohere
response, err := bedrock.InvokeModel(
    ctx,
    "cohere.command-text-v14",
    "Summarize this article...",
    map[string]interface{}{
        "max_tokens":  256,
        "temperature": 0.5,
    },
)
```

## Rate Limits and Quotas

### Default Service Quotas

| Quota | Default Value |
|-------|---------------|
| Requests per minute (Claude) | 50 |
| Requests per minute (Titan) | 100 |
| Max concurrent requests | 20 |
| Max input tokens | Model-specific |

### Model-Specific Limits

| Model | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| Claude 3 Sonnet | 200,000 | 4,096 |
| Claude 3 Haiku | 200,000 | 4,096 |
| Titan Express | 8,000 | 4,096 |
| Llama 3 70B | 8,000 | 2,048 |

### Requesting Quota Increases

1. Go to [Service Quotas](https://console.aws.amazon.com/servicequotas)
2. Select **Amazon Bedrock**
3. Find the quota you want to increase
4. Click **Request quota increase**
5. Submit the request with justification

## AWS Signature V4 Authentication

SuperAgent implements AWS Signature V4 for authenticating requests:

```go
// The signRequest method handles:
// 1. Creating canonical request
// 2. Creating string to sign
// 3. Calculating signature
// 4. Adding Authorization header
```

This ensures secure, authenticated access to AWS Bedrock.

## Troubleshooting

### Common Errors

#### Access Denied (403)

```
AWS Bedrock API error: 403 - AccessDeniedException
```

**Solution:**
- Verify IAM permissions include `bedrock:InvokeModel`
- Check that model access is enabled in Bedrock console
- Ensure the model ID is correct
- Verify your credentials are valid

#### Model Not Found (404)

```
AWS Bedrock API error: 404 - ResourceNotFoundException
```

**Solution:**
- Verify the model ID is correct
- Check that you have access to the model
- Ensure the model is available in your region

#### Validation Error (400)

```
AWS Bedrock API error: 400 - ValidationException
```

**Solution:**
- Check request format matches model requirements
- Verify parameters are within valid ranges
- Ensure message format is correct for the model

#### Throttling (429)

```
AWS Bedrock API error: 429 - ThrottlingException
```

**Solution:**
- Wait and retry (SuperAgent handles this automatically)
- Request a quota increase
- Implement request queuing

#### Service Unavailable (503)

```
AWS Bedrock API error: 503 - ServiceUnavailableException
```

**Solution:**
- Wait and retry
- Check AWS service health dashboard
- Try a different region if available

### Health Check

```go
err := bedrock.HealthCheck(ctx)
if err != nil {
    fmt.Printf("AWS Bedrock unhealthy: %v\n", err)
}
```

### Debug Logging

Enable debug logging:

```bash
export GIN_MODE=debug
export LOG_LEVEL=debug
```

### Credential Validation

Verify your credentials are working:

```bash
# Using AWS CLI
aws bedrock list-foundation-models --region us-east-1

# Should list available models
```

### Region Availability

Not all models are available in all regions. Check [AWS Bedrock regions](https://docs.aws.amazon.com/bedrock/latest/userguide/bedrock-regions.html).

| Model Provider | Regions |
|----------------|---------|
| Anthropic | us-east-1, us-west-2, eu-central-1 |
| Amazon Titan | All Bedrock regions |
| Meta | us-east-1, us-west-2 |
| Cohere | us-east-1, us-west-2 |

## VPC Configuration

For private deployments without internet access:

### VPC Endpoint Setup

1. Go to VPC Console > Endpoints
2. Create endpoint for `com.amazonaws.<region>.bedrock-runtime`
3. Select your VPC and subnets
4. Configure security groups
5. Update your application to use the VPC endpoint

## IAM Policy Example

Minimum required permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "bedrock:InvokeModel",
                "bedrock:InvokeModelWithResponseStream",
                "bedrock:ListFoundationModels"
            ],
            "Resource": [
                "arn:aws:bedrock:*::foundation-model/*"
            ]
        }
    ]
}
```

## Cost Management

### Pricing Structure

AWS Bedrock uses pay-per-use pricing:

| Model | Input (per 1K tokens) | Output (per 1K tokens) |
|-------|----------------------|------------------------|
| Claude 3 Sonnet | $0.003 | $0.015 |
| Claude 3 Haiku | $0.00025 | $0.00125 |
| Titan Express | $0.0008 | $0.0016 |

### Cost Optimization Tips

1. **Use smaller models** when appropriate
2. **Set max_tokens limits** to prevent excessive output
3. **Use Haiku** for simple tasks
4. **Enable caching** for repeated queries
5. **Monitor usage** in AWS Cost Explorer

## Additional Resources

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [AWS Bedrock Pricing](https://aws.amazon.com/bedrock/pricing/)
- [AWS Bedrock Model Catalog](https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html)
- [AWS SDK for Go](https://aws.amazon.com/sdk-for-go/)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
