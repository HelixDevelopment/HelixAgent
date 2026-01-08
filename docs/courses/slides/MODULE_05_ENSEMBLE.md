# Module 5: Ensemble Strategies

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 5: Ensemble Strategies
- Duration: 60 minutes
- Combining AI Models for Better Results

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand ensemble learning principles
- Implement different voting strategies
- Create custom voting algorithms
- Optimize ensemble performance

---

## Slide 3: What is Ensemble AI?

**Ensemble Learning Concept:**

- Combine multiple models for better predictions
- Reduces individual model bias
- Improves accuracy and reliability
- Used in production AI systems worldwide

*"The wisdom of crowds applied to AI"*

---

## Slide 4: Why Ensemble Matters

**Benefits of Multi-Model Approach:**

| Single Model | Ensemble |
|--------------|----------|
| Single point of failure | Redundancy |
| Model-specific bias | Balanced perspective |
| Limited capabilities | Combined strengths |
| Inconsistent quality | Stable output |

---

## Slide 5: Ensemble Architecture

**How HelixAgent Ensemble Works:**

```
Request --> +------------------+
            |  Parallel        |
            |  Execution       |
            +--------+---------+
                     |
      +------+-------+-------+------+
      |      |               |      |
   +--v--+ +-v--+         +--v--+ +-v--+
   |Claude| |Gemini|       |Deep | |Qwen|
   +--+--+ +--+--+         +--+--+ +--+--+
      |      |               |      |
      +------+-------+-------+------+
                     |
            +--------v---------+
            |  Voting Engine   |
            +--------+---------+
                     |
                     v
               Best Response
```

---

## Slide 6: Voting Strategies Overview

**Available Voting Strategies:**

1. **Majority** - Simple vote count
2. **Weighted** - Provider weight-based
3. **Consensus** - High agreement required
4. **Confidence-Weighted** - Based on confidence scores
5. **Quality-Weighted** - Based on quality metrics

---

## Slide 7: Majority Voting

**Simplest Strategy:**

```yaml
ensemble:
  voting:
    type: majority
```

**How it works:**
- Each provider gets one vote
- Response with most votes wins
- Ties broken by first responder

*Best for: Simple decisions, equal-quality providers*

---

## Slide 8: Weighted Voting

**Provider Weight-Based:**

```yaml
ensemble:
  voting:
    type: weighted
    weights:
      claude: 1.5
      gemini: 1.2
      deepseek: 1.0
      qwen: 0.8
```

**Higher weight = more influence on final result**

---

## Slide 9: Consensus Voting

**High Agreement Required:**

```yaml
ensemble:
  voting:
    type: consensus
    consensus_threshold: 0.75
```

**How it works:**
- All providers must largely agree
- Threshold defines agreement level
- Falls back if consensus not reached

*Best for: High-stakes decisions*

---

## Slide 10: Confidence-Weighted Voting

**Most Popular Strategy:**

```yaml
ensemble:
  voting:
    type: confidence_weighted
    min_confidence: 0.6
```

**How it works:**
- Each response includes confidence score
- Higher confidence = more weight
- Balances quality with certainty

---

## Slide 11: Quality-Weighted Voting

**Quality Metrics-Based:**

```yaml
ensemble:
  voting:
    type: quality_weighted
    quality_metrics:
      - coherence
      - relevance
      - completeness
```

**Evaluates response quality, not just confidence**

---

## Slide 12: Voting Strategy Interface

**VotingStrategy Interface:**

```go
type VotingStrategy interface {
    // Vote on responses
    Vote(responses []*Response) (*Response, error)

    // Get strategy name
    Name() string

    // Configure strategy
    Configure(config map[string]interface{}) error
}
```

---

## Slide 13: Implementing Custom Strategy

**Custom Voting Example:**

```go
type CustomStrategy struct {
    weights map[string]float64
}

func (s *CustomStrategy) Vote(
    responses []*Response,
) (*Response, error) {
    var bestResponse *Response
    var highestScore float64

    for _, resp := range responses {
        score := s.calculateScore(resp)
        if score > highestScore {
            highestScore = score
            bestResponse = resp
        }
    }

    return bestResponse, nil
}
```

---

## Slide 14: Score Calculation

**Response Scoring Example:**

```go
func (s *CustomStrategy) calculateScore(
    resp *Response,
) float64 {
    score := 0.0

    // Factor 1: Provider weight
    score += s.weights[resp.Provider] * 0.3

    // Factor 2: Confidence
    score += resp.Confidence * 0.4

    // Factor 3: Response quality
    score += s.evaluateQuality(resp) * 0.3

    return score
}
```

---

## Slide 15: Ensemble Configuration

**Complete Ensemble Config:**

```yaml
ensemble:
  enabled: true
  strategy: confidence_weighted

  providers:
    min: 2
    max: 5
    required:
      - claude
      - gemini

  execution:
    parallel: true
    timeout: 30s

  fallback:
    strategy: majority
    trigger_on: timeout
```

---

## Slide 16: Parallel Execution

**Performance Optimization:**

```yaml
ensemble:
  execution:
    parallel: true
    timeout: 30s
    early_return: false
```

**Parallel execution:**
- All providers called simultaneously
- Total time = slowest provider
- Much faster than sequential

---

## Slide 17: Caching for Ensemble

**Cache Similar Queries:**

```yaml
ensemble:
  cache:
    enabled: true
    ttl: 1h
    similarity_threshold: 0.85

    # Cache key includes:
    # - prompt hash
    # - provider list
    # - voting strategy
```

---

## Slide 18: Response Aggregation

**Beyond Simple Voting:**

```yaml
ensemble:
  aggregation:
    type: synthesis  # vote, synthesis, merge
    synthesis_prompt: |
      Given these responses from multiple AI models,
      synthesize the best answer:
      {responses}
```

*Synthesis creates a new response combining best parts*

---

## Slide 19: Quality Metrics

**Measuring Ensemble Quality:**

| Metric | Description |
|--------|-------------|
| Agreement Rate | How often providers agree |
| Confidence Distribution | Spread of confidence scores |
| Response Time | Total ensemble latency |
| Cache Hit Rate | Cached vs new responses |

---

## Slide 20: Ensemble API

**Making Ensemble Requests:**

```bash
curl -X POST http://localhost:7061/v1/ensemble \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Explain quantum computing",
    "providers": ["claude", "gemini", "deepseek"],
    "strategy": "confidence_weighted"
  }'
```

---

## Slide 21: Ensemble Response

**Understanding Response:**

```json
{
  "response": {
    "content": "Quantum computing is...",
    "provider": "claude",
    "confidence": 0.92
  },
  "ensemble_metadata": {
    "strategy": "confidence_weighted",
    "providers_used": ["claude", "gemini", "deepseek"],
    "agreement_rate": 0.85,
    "total_time_ms": 1245
  }
}
```

---

## Slide 22: When to Use Ensemble

**Best Use Cases:**

| Use Case | Recommended Strategy |
|----------|---------------------|
| Factual Q&A | Majority |
| Creative Writing | Weighted |
| Critical Decisions | Consensus |
| General Purpose | Confidence-Weighted |
| Quality Focus | Quality-Weighted |

---

## Slide 23: Performance Considerations

**Optimizing Ensemble Performance:**

1. **Limit Providers**: 3-4 optimal for most cases
2. **Set Timeouts**: Don't wait forever
3. **Use Caching**: Cache similar queries
4. **Early Return**: Return when confident
5. **Async Processing**: For non-blocking calls

---

## Slide 24: Cost Optimization

**Managing Ensemble Costs:**

```yaml
ensemble:
  cost_optimization:
    enabled: true
    max_cost_per_request: 0.01

    # Only use expensive providers when needed
    expensive_threshold: 0.8
    cheap_providers:
      - deepseek
      - qwen
```

---

## Slide 25: Hands-On Lab

**Lab Exercise 5.1: Ensemble Implementation**

Tasks:
1. Configure 3-provider ensemble
2. Test different voting strategies
3. Compare results across strategies
4. Measure performance metrics
5. Implement simple custom strategy

Time: 25 minutes

---

## Slide 26: Module Summary

**Key Takeaways:**

- Ensemble improves accuracy and reliability
- 5 built-in voting strategies
- Custom strategies for specific needs
- Parallel execution for performance
- Caching reduces costs and latency
- Choose strategy based on use case

**Next: Module 6 - AI Debate System**

---

## Speaker Notes

### Slide 5 Notes
Draw the architecture on a whiteboard if possible. Explain that parallel execution is key to performance.

### Slide 10 Notes
Confidence-weighted is the most commonly used strategy. Explain how confidence scores are generated by each provider.

### Slide 13 Notes
Walk through the code step by step. Emphasize that implementing VotingStrategy interface is all that's needed.

### Slide 23 Notes
Share real-world performance data if available. Discuss the trade-off between quality and latency.
