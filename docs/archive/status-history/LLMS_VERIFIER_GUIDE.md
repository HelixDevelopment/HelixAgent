# LLMsVerifier: Comprehensive Provider Validation Guide

## Overview

The LLMsVerifier is a comprehensive validation tool that tests **every provider** and **every model** individually with detailed error analysis. This ensures we have complete visibility into provider health, performance, and quality.

## Key Features

### 1. Individual Provider Testing
Each of the 47 providers is tested individually:
- Connection health check
- Authentication validation
- Capability detection
- Error categorization

### 2. Per-Model Validation
For each provider, every model is tested with:
- **Simple Query**: Basic functionality test
- **Reasoning**: Step-by-step problem solving
- **Code Generation**: Programming capability
- **Creative**: Content generation

### 3. Detailed Error Analysis
Every failure is categorized with:
- Exact error message
- HTTP status code
- Failure category (auth, connection, rate limit, server, quality)
- Recommendations for resolution

### 4. Quality Scoring
Each model receives a quality score (0-100) based on:
- Response validity
- Content relevance
- Format correctness
- Latency performance

## Usage

### Quick Validation
```bash
# Run complete validation suite
./scripts/run_complete_validation.sh

# Skip build (if already built)
./scripts/run_complete_validation.sh --skip-build

# Quick mode (skip challenges)
./scripts/run_complete_validation.sh --quick
```

### LLMsVerifier Only
```bash
# Make sure HelixAgent is running
./bin/helixagent &

# Run provider validation
./scripts/run_llms_verifier.sh

# Report location:
# docs/reports/llms_verifier/$(date +%Y-%m-%d)/report_latest.md
```

### Manual Provider Testing
```bash
# Test individual provider health
curl http://localhost:7061/v1/providers/claude/health

# List all providers
curl http://localhost:7061/v1/providers | jq '.'

# Test specific model
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Provider: claude" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Report Structure

The generated report includes:

### 1. Executive Summary
- Total providers tested
- Pass/fail counts
- Overall system status

### 2. Provider Details
For each provider:
- Connection status
- Health check latency
- Per-model results table
- Capability support matrix
- Error details (if failed)

### 3. Model Results Table
```
| Model | Simple | Reasoning | Code | Creative | Avg Score |
|-------|--------|-----------|------|----------|-----------|
| gpt-4 | 100    | 95        | 95   | 90       | ✅ 95     |
```

### 4. Error Analysis
Categorized failures:
- **Authentication Failures**: API key issues
- **Connection Failures**: Network/timeout issues
- **Rate Limiting**: Quota exceeded
- **Server Errors**: Provider-side 5xx errors
- **Quality Issues**: Low response scores

### 5. Performance Analysis
Latency metrics per provider:
- Health check latency
- Average model latency
- Performance tier classification

### 6. Recommendations
Actionable tuning suggestions:
- Provider prioritization
- Authentication fixes
- Quality tuning parameters

## Test Prompts

The verifier uses these test prompts:

### Simple Query
```
What is 2+2? Answer with just the number.
```
Expected: Response contains "4" or "four"

### Reasoning
```
Explain step by step how to solve: 
If a train travels 60 km in 30 minutes, what is its average speed in km/h?
```
Expected: Response shows reasoning and mentions "120"

### Code Generation
```
Write a Python function to calculate fibonacci(n) with memoization.
```
Expected: Valid Python function with memoization

### Creative
```
Write a haiku about artificial intelligence.
```
Expected: Creative content with proper structure

## Interpreting Results

### Quality Score Interpretation

| Score | Status | Action |
|-------|--------|--------|
| 90-100 | ✅ Excellent | Production ready |
| 70-89 | ✓ Good | Acceptable for use |
| 50-69 | ⚠️ Fair | May need tuning |
| 30-49 | 🔶 Poor | Review configuration |
| 0-29 | ❌ Failed | Check setup/errors |

### Provider Status

| Status | Meaning |
|--------|---------|
| PASSED_ALL | All models working well |
| PARTIAL | Some models failed |
| FAILED_ALL_MODELS | No models working |
| FAILED_CONNECTION | Can't connect to provider |
| UNKNOWN | Not tested |

## Troubleshooting Common Issues

### Authentication Failures
```bash
# Check API key is set
echo $CLAUDE_API_KEY
echo $OPENAI_API_KEY

# Add to .env file
echo "CLAUDE_API_KEY=your_key_here" >> .env
source .env
```

### Connection Timeouts
```bash
# Check HelixAgent is running
curl http://localhost:7061/health

# Check provider endpoint exists
curl http://localhost:7061/v1/providers

# Check network connectivity
ping api.anthropic.com
```

### Rate Limiting
```bash
# Wait before retrying
sleep 60

# Check rate limit headers in logs
grep -i "rate" logs/test_llms_verifier.log
```

### Quality Issues
If models return low scores:
1. Check temperature settings in provider config
2. Verify max_tokens isn't too low
3. Review prompt formatting for provider
4. Check model availability (some may be deprecated)

## Integration with CI/CD

**Note:** As per project requirements, no automated CI/CD pipelines exist. However, you can manually run:

```bash
# Before releases
./scripts/run_complete_validation.sh

# Quick health check
./scripts/run_llms_verifier.sh

# Check specific provider
curl http://localhost:7061/v1/providers/claude/health
```

## Daily Reports

To generate daily reports automatically (manual execution):

```bash
#!/bin/bash
# Add to crontab for daily runs at 6 AM
# 0 6 * * * /path/to/helixagent/scripts/run_llms_verifier.sh

DATE=$(date +%Y-%m-%d)
cd /path/to/helixagent

# Ensure HelixAgent is running
if ! curl -s http://localhost:7061/health > /dev/null; then
    ./bin/helixagent &
    sleep 10
fi

# Run validation
./scripts/run_llms_verifier.sh

# Archive report
tar -czf docs/reports/archive/llms_verifier_$DATE.tar.gz \
    docs/reports/llms_verifier/$DATE/
```

## Future Tuning Using Reports

The detailed reports enable:

1. **Provider Prioritization**: Use only high-score providers
2. **Fallback Chains**: Configure backup providers
3. **Quality Thresholds**: Set minimum scores for production
4. **Latency Optimization**: Choose fastest providers
5. **Cost Optimization**: Balance quality vs. cost

Example configuration based on reports:
```yaml
providers:
  primary:
    - claude  # Score: 95, Latency: 800ms
    - gpt4    # Score: 92, Latency: 600ms
  secondary:
    - deepseek  # Score: 88, Latency: 1200ms
  fallback:
    - groq  # Score: 75, Latency: 200ms (fast but lower quality)
```

## Summary

The LLMsVerifier provides:
- ✅ **Complete visibility** into all 47 providers
- ✅ **Per-model validation** with quality scores
- ✅ **Detailed error analysis** for troubleshooting
- ✅ **Performance metrics** for optimization
- ✅ **Actionable recommendations** for tuning

All failures are logged with exact reasons, enabling precise fixes and continuous improvement of the provider ecosystem.
