# Single-Provider Multi-Instance Debate Mode

## Overview

The Single-Provider Multi-Instance Debate Mode allows SuperAgent to conduct AI debates even when only **ONE** LLM provider is available. Instead of requiring multiple different providers (e.g., Claude, DeepSeek, Gemini), this mode uses a single provider with multiple instances to fulfill all debate participant roles.

## When Is This Mode Activated?

The system automatically detects and activates single-provider mode when:

1. A debate is requested with multiple participants
2. All participants are configured to use the same provider (or only one provider is healthy)
3. The system has exactly one healthy/available provider

```go
// The system automatically checks for single-provider mode
isSingle, spc := debateService.IsSingleProviderMode(config)
if isSingle {
    // Automatically uses single-provider multi-instance debate
    result, err = debateService.ConductSingleProviderDebate(ctx, config, spc)
}
```

## Diversity Mechanisms

Since all participants use the same underlying model, diversity is achieved through:

### 1. Model Diversity (When Available)
If the provider supports multiple models, each participant can use a different model:

| Provider | Available Models |
|----------|-----------------|
| DeepSeek | deepseek-chat, deepseek-coder, deepseek-reasoner |
| Claude | claude-3-5-sonnet, claude-3-5-haiku, claude-3-opus |
| OpenAI | gpt-4o, gpt-4o-mini, gpt-4-turbo |
| Gemini | gemini-pro, gemini-1.5-pro, gemini-1.5-flash |

### 2. Temperature Diversity
Each participant uses a different temperature setting to vary response creativity:

| Role | Temperature Offset | Effective Temperature |
|------|-------------------|----------------------|
| Analytical Thinker | +0.0 | 0.70 |
| Creative Explorer | +0.3 | 1.00 |
| Critical Examiner | -0.1 | 0.60 |
| Practical Advisor | +0.1 | 0.80 |
| Systems Thinker | +0.2 | 0.90 |

### 3. System Prompt Diversity
Each participant receives a unique system prompt that enforces a specific perspective:

```
You are Analytical Thinker, participant 1 of 5 in an AI debate panel using the same
underlying model but with distinct perspectives.

YOUR UNIQUE PERSPECTIVE: Focus on data, evidence, and logical reasoning. Be precise and thorough.

IMPORTANT GUIDELINES:
- You MUST maintain your unique viewpoint throughout the debate
- Your perspective should be clearly different from other participants
- Acknowledge valid points from others while contributing your distinct analysis
- Do not simply agree with everything - bring your unique expertise
- Be specific and provide concrete examples from your perspective
```

### 4. Role-Based Perspectives

| Role | Perspective |
|------|-------------|
| analyst | Data-driven, evidence-focused reasoning |
| proposer | Creative, innovative solutions |
| critic | Challenge assumptions, identify weaknesses |
| mediator | Find common ground, practical outcomes |
| debater | Balanced argumentation |
| opponent | Counter-arguments and alternatives |
| moderator | Keep discussion focused |
| strategist | Long-term implications |

## API Usage

### AutoConductDebate (Recommended)
Automatically selects the best debate mode:

```go
result, err := debateService.AutoConductDebate(ctx, config)
```

### Explicit Single-Provider Debate
For direct control:

```go
spc := &services.SingleProviderConfig{
    ProviderName:      "deepseek",
    AvailableModels:   []string{"deepseek-chat", "deepseek-coder"},
    NumParticipants:   5,
    UseModelDiversity: true,
    UseTempDiversity:  true,
}

result, err := debateService.ConductSingleProviderDebate(ctx, config, spc)
```

### Check Single-Provider Mode
To detect if single-provider mode would be used:

```go
isSingle, spc := debateService.IsSingleProviderMode(config)
if isSingle {
    fmt.Printf("Would use single-provider mode with %s\n", spc.ProviderName)
}
```

## REST API

### POST /v1/debates

The standard debate endpoint automatically uses single-provider mode when appropriate:

```json
{
    "debate_id": "my-debate",
    "topic": "Should AI be regulated?",
    "max_rounds": 3,
    "participants": [
        {"participant_id": "p1", "name": "Analyst", "role": "analyst", "llm_provider": "deepseek"},
        {"participant_id": "p2", "name": "Proposer", "role": "proposer", "llm_provider": "deepseek"},
        {"participant_id": "p3", "name": "Critic", "role": "critic", "llm_provider": "deepseek"},
        {"participant_id": "p4", "name": "Mediator", "role": "mediator", "llm_provider": "deepseek"},
        {"participant_id": "p5", "name": "Strategist", "role": "opponent", "llm_provider": "deepseek"}
    ]
}
```

Response includes single-provider metadata:

```json
{
    "success": true,
    "debate_id": "my-debate",
    "quality_score": 0.85,
    "metadata": {
        "mode": "single_provider",
        "provider": "deepseek",
        "models_used": ["deepseek-chat", "deepseek-coder"],
        "instance_count": 5,
        "model_diversity": true,
        "temp_diversity": true,
        "effective_diversity": 0.72
    }
}
```

## Logging

All logs include readable participant identifiers in the format `Provider-N`:

```
INFO [DeepSeek-1] Single-provider participant starting response
     participant=DeepSeek-1 participant_id=single-provider-instance-1
     participant_name="Analytical Thinker (Instance 1)" role=analyst
     provider=deepseek model=deepseek-chat round=1 debate_id=my-debate

INFO [DeepSeek-1] Single-provider participant response completed
     participant=DeepSeek-1 response_time_ms=2340 quality_score=0.87
     tokens_used=412 content_length=1823
```

This makes it easy to track each participant's activity:
- `DeepSeek-1`: First instance using DeepSeek
- `DeepSeek-2`: Second instance using DeepSeek
- `Gemini-3`: Third instance using Gemini (in multi-provider mode)

## Testing

### Run Unit Tests
```bash
go test -v ./tests/challenge -run TestSingleProviderMultiInstanceDebate -timeout 300s
```

### Run Integration Challenge
```bash
./challenges/scripts/single_provider_challenge.sh --verbose
```

### Test with Specific Provider
```bash
./challenges/scripts/single_provider_challenge.sh --provider deepseek --participants 5 --rounds 2
```

## Metrics

### Effective Diversity Score
Measures how different the responses are from each other:
- `1.0`: Completely different responses
- `0.5`: Moderately diverse
- `0.0`: Identical responses

Target: `> 0.3` for meaningful diversity

### Quality Score
Measures response quality based on:
- Content length and structure
- Coherence indicators
- Completion status
- Token efficiency

Target: `> 0.5` for acceptable quality

## Best Practices

1. **Use Auto Mode**: Let `AutoConductDebate()` decide the best mode
2. **Enable All Diversity**: Set `UseModelDiversity` and `UseTempDiversity` to `true`
3. **Minimum 3 Participants**: For meaningful debate dynamics
4. **2-3 Rounds**: Allows for response and counter-response
5. **Clear Topics**: Well-defined debate topics improve response quality

## Troubleshooting

### Low Diversity Score
- Increase temperature diversity range
- Use more distinct system prompts
- Try different models from the provider

### Identical Responses
- Check that temperature settings are being applied
- Verify system prompts are unique
- Ensure participant roles are diverse

### Provider Not Found
- Verify API key is set in environment
- Check provider health status
- Run provider verification

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        DebateService                            │
├─────────────────────────────────────────────────────────────────┤
│  AutoConductDebate()                                            │
│         │                                                       │
│         ▼                                                       │
│  IsSingleProviderMode()  ──Yes──▶  ConductSingleProviderDebate()│
│         │                                                       │
│         No                                                      │
│         ▼                                                       │
│  ConductDebate() (standard multi-provider)                      │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│              Single-Provider Multi-Instance Flow                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. CreateSingleProviderParticipants()                          │
│     ├── Assign diverse roles                                    │
│     ├── Set temperature variations                              │
│     ├── Generate unique system prompts                          │
│     └── Cycle through available models                          │
│                                                                 │
│  2. executeSingleProviderRound()                                │
│     ├── Get single provider instance                            │
│     ├── Execute parallel requests (different configs)           │
│     └── Collect and validate responses                          │
│                                                                 │
│  3. Analyze Results                                             │
│     ├── Calculate effective diversity                           │
│     ├── Measure quality scores                                  │
│     └── Build consensus                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration

### Environment Variables

```bash
# Primary provider API keys
DEEPSEEK_API_KEY=sk-xxx
CLAUDE_API_KEY=sk-xxx
GEMINI_API_KEY=xxx

# Server configuration
SUPERAGENT_PORT=8080
```

### Provider Model Lists

Update known models in `debate_service.go`:

```go
knownModels := map[string][]string{
    "deepseek": {"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
    "claude":   {"claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"},
    // Add more...
}
```

## Related Documentation

- [Provider Verification System](./PROVIDER_VERIFICATION.md)
- [AI Debate System](./development/DETAILED_IMPLEMENTATION_GUIDE_PHASE1.md)
- [Challenge Framework](../challenges/README.md)
