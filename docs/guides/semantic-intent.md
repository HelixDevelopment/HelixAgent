# Semantic Intent Detection Guide

## Overview

HelixAgent uses **LLM-based semantic intent classification** to understand user messages. When a user confirms, refuses, or asks questions, the system uses AI to understand the semantic meaning - not pattern matching.

## Key Principle: ZERO Hardcoding

The intent detection system follows a strict no-hardcoding policy:

- **User intent detected by semantic meaning**, not exact string matching
- **Short positive responses with context** = likely confirmation
- **LLM classifies with JSON structured output**
- **Fallback uses semantic roots and word stems**, not exact patterns

## Architecture

### Primary: LLM-Based Classification

```go
// internal/services/llm_intent_classifier.go
type LLMIntentClassifier struct {
    provider    LLMProvider
    debateTeam  *DebateTeamConfig
}

type ClassificationResult struct {
    Intent       IntentType `json:"intent"`
    Confidence   float64    `json:"confidence"`
    IsActionable bool       `json:"is_actionable"`
    ShouldProceed bool      `json:"should_proceed"`
}
```

### Fallback: Pattern-Based Classification

Used only when LLM is unavailable:

```go
// internal/services/intent_classifier.go
type IntentClassifier struct {
    semanticRoots map[string]IntentType
    wordStems     map[string]IntentType
}
```

## Intent Types

| Intent | Description | Example Messages |
|--------|-------------|------------------|
| `confirmation` | User approves/confirms action | "Yes", "Go ahead", "Let's do all points!" |
| `refusal` | User declines/refuses action | "No", "Stop", "Cancel that" |
| `question` | User asks for information | "What do you mean?", "How does this work?" |
| `request` | User makes a new request | "Help me with X" |
| `clarification` | User needs more info | "I'm confused about this" |
| `unclear` | Cannot determine intent | Ambiguous messages |

## API Usage

### Classify User Intent

```bash
curl -X POST http://localhost:8080/v1/intent/classify \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Yes, let'"'"'s proceed with all the changes!",
    "context": "User was asked to confirm 5 code changes"
  }'
```

### Response

```json
{
  "intent": "confirmation",
  "confidence": 0.95,
  "is_actionable": true,
  "should_proceed": true,
  "reasoning": "User explicitly confirms with 'Yes' and references 'all the changes'"
}
```

## How Classification Works

### Step 1: Context Analysis

The classifier examines:
- The user's message content
- Previous conversation context
- What action was proposed

### Step 2: Semantic Understanding

The LLM interprets meaning, not keywords:

| Message | Intent | Reasoning |
|---------|--------|-----------|
| "Yeah, do it" | confirmation | Affirmative with action verb |
| "Sounds good!" | confirmation | Positive sentiment |
| "I'm not sure" | clarification | Uncertainty expression |
| "Wait, what?" | question | Interrogative |
| "Nope" | refusal | Negative response |

### Step 3: Confidence Scoring

Each classification includes confidence:
- **> 0.9**: High confidence, proceed automatically
- **0.7 - 0.9**: Moderate confidence, may verify
- **< 0.7**: Low confidence, ask for clarification

## Integration with AI Debate

The intent classifier is integrated into the debate service:

```go
// internal/services/debate_service.go
func (s *DebateService) processUserMessage(msg string) {
    result := s.classifyUserIntent(msg)

    switch result.Intent {
    case IntentConfirmation:
        s.proceedWithAction()
    case IntentRefusal:
        s.cancelAction()
    case IntentQuestion:
        s.provideExplanation()
    case IntentClarification:
        s.requestMoreDetails()
    }
}
```

## Challenge Validation

Run the semantic intent challenge to verify zero hardcoding:

```bash
./challenges/scripts/semantic_intent_challenge.sh
```

This runs 19 tests to validate:
- No exact string matching for intents
- Semantic understanding works correctly
- Edge cases are handled properly
- LLM fallback works when needed

## Best Practices

### 1. Provide Context

Always include conversation context for better classification:

```json
{
  "message": "Go ahead",
  "context": "User was asked: 'Should I refactor the authentication module?'"
}
```

### 2. Handle Low Confidence

When confidence is low, ask for clarification:

```go
if result.Confidence < 0.7 {
    return "I'm not sure I understand. Could you clarify?"
}
```

### 3. Use Structured Confirmations

For important actions, use structured confirmation:

```json
{
  "message": "Please confirm: Apply 5 changes to auth module?",
  "expected_response": "confirmation",
  "timeout": 30
}
```

## Troubleshooting

### Misclassified Intents

If intents are being misclassified:
1. Check that LLM provider is healthy
2. Verify context is being passed
3. Review the message for ambiguity

### Low Confidence Scores

If confidence is consistently low:
1. Add more context to messages
2. Check LLM provider response quality
3. Consider using more explicit language

### Fallback to Pattern Matching

If falling back to patterns too often:
1. Check LLM provider availability
2. Verify API keys are configured
3. Review timeout settings

## Related Documentation

- [AI Debate System](/docs/ai-debate.html)
- [Intent Classifier Tests](/internal/services/intent_classifier_test.go)
- [LLM Intent Classifier](/internal/services/llm_intent_classifier.go)
