# Multi-Pass Validation Guide

## Overview

Multi-Pass Validation is an advanced feature of HelixAgent's AI Debate system that re-evaluates, polishes, and improves debate responses before delivering the final consensus. This results in higher-quality, more accurate responses.

## How It Works

### Validation Phases

The system operates in 4 distinct phases:

| Phase | Icon | Description |
|-------|------|-------------|
| 1. INITIAL RESPONSE | 🔍 | Each AI participant provides their initial perspective |
| 2. VALIDATION | ✓ | Cross-validation of responses for accuracy and completeness |
| 3. POLISH & IMPROVE | ✨ | Refinement and improvement based on validation feedback |
| 4. FINAL CONCLUSION | 📜 | Synthesized consensus with confidence scores |

### Benefits

- **Higher Quality**: Responses are refined through multiple passes
- **Better Accuracy**: Cross-validation catches errors and inconsistencies
- **Stronger Consensus**: Multiple rounds lead to more robust conclusions
- **Confidence Scoring**: Clear indication of response reliability

## API Usage

### Enabling Multi-Pass Validation

```bash
curl -X POST http://localhost:8080/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Should AI have consciousness?",
    "participants": [...],
    "enable_multi_pass_validation": true,
    "validation_config": {
      "enable_validation": true,
      "enable_polish": true,
      "validation_timeout": 120,
      "polish_timeout": 60,
      "min_confidence_to_skip": 0.9,
      "max_validation_rounds": 3,
      "show_phase_indicators": true
    }
  }'
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enable_validation` | bool | true | Enable validation phase |
| `enable_polish` | bool | true | Enable polish & improve phase |
| `validation_timeout` | int | 120 | Max seconds for validation |
| `polish_timeout` | int | 60 | Max seconds for polishing |
| `min_confidence_to_skip` | float | 0.9 | Skip validation if confidence >= this |
| `max_validation_rounds` | int | 3 | Maximum validation iterations |
| `show_phase_indicators` | bool | true | Show phase icons in response |

### Response Format

```json
{
  "id": "debate-123",
  "status": "completed",
  "current_phase": "FINAL_CONCLUSION",
  "multi_pass_result": {
    "phases_completed": 4,
    "overall_confidence": 0.95,
    "quality_improvement": 0.15,
    "final_response": "After careful consideration...",
    "phase_history": [
      {"phase": "INITIAL", "confidence": 0.80},
      {"phase": "VALIDATION", "confidence": 0.88},
      {"phase": "POLISH", "confidence": 0.93},
      {"phase": "FINAL", "confidence": 0.95}
    ]
  }
}
```

## Best Practices

### When to Use

- **Complex topics**: Multi-faceted questions benefit from validation
- **High-stakes decisions**: When accuracy is critical
- **Controversial topics**: Cross-validation helps balance perspectives

### When to Skip

- **Simple queries**: Quick questions don't need multi-pass
- **Time-sensitive**: When latency is a priority
- **High initial confidence**: Skip if `min_confidence_to_skip` is met

### Performance Considerations

Multi-pass validation adds latency:
- Each phase adds ~30-60 seconds
- Full validation can take 2-5 minutes
- Consider timeout settings for your use case

## Integration Examples

### Python SDK

```python
from helixagent import DebateClient

client = DebateClient()
result = client.create_debate(
    topic="What is the future of AI?",
    enable_multi_pass_validation=True,
    validation_config={
        "max_validation_rounds": 2,
        "min_confidence_to_skip": 0.85
    }
)

print(f"Confidence: {result.multi_pass_result.overall_confidence}")
print(f"Quality improvement: {result.multi_pass_result.quality_improvement}")
```

### JavaScript SDK

```javascript
const { DebateClient } = require('@helixagent/sdk');

const client = new DebateClient();
const result = await client.createDebate({
  topic: "What is the future of AI?",
  enableMultiPassValidation: true,
  validationConfig: {
    maxValidationRounds: 2,
    minConfidenceToSkip: 0.85
  }
});

console.log(`Confidence: ${result.multiPassResult.overallConfidence}`);
```

## Troubleshooting

### Low Quality Improvement

If `quality_improvement` is consistently low:
- Increase `max_validation_rounds`
- Lower `min_confidence_to_skip`
- Ensure diverse providers in debate team

### Timeouts

If validation is timing out:
- Increase `validation_timeout` and `polish_timeout`
- Reduce topic complexity
- Check provider response times

### Phase Skipping

If phases are being skipped unexpectedly:
- Check `min_confidence_to_skip` threshold
- Verify `enable_validation` and `enable_polish` are true
- Review initial response confidence levels

## Related Documentation

- [AI Debate System](/docs/ai-debate.html)
- [Debate API Reference](/docs/api/debate-api.md)
- [Provider Configuration](/docs/guides/configuration-guide.md)
