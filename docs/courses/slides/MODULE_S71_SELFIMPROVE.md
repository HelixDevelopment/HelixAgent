# Module S7.1.3: SelfImprove — RLHF, Reward Modeling, and Preference Optimization

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module S7.1.3: SelfImprove Module
- Duration: 30 minutes
- AI Systems That Learn From Every Interaction

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Collect explicit and implicit feedback from users
- Build and train RewardModel instances from PreferencePair data
- Implement SelfRefinementLoop for iterative response improvement
- Apply preference optimization at inference time
- Integrate feedback collection into HelixAgent streaming responses

---

## Slide 3: The RLHF Loop

**Reinforcement Learning from Human Feedback:**

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  User Request ──► LLM Response ──► User Feedback        │
│                         │                │              │
│                         │       ┌────────┘              │
│                         │       ▼                       │
│                         │  FeedbackCollector            │
│                         │       │                       │
│                         │       ▼ (nightly batch)       │
│                         │  RewardModel.Train(pairs)     │
│                         │       │                       │
│                         └───────┘                       │
│              (better responses on next request)         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## Slide 4: Module Identity

**`digital.vasic.selfimprove`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.selfimprove` |
| Go version | 1.25+ |
| Source directory | `SelfImprove/` |
| HelixAgent adapter | `internal/adapters/selfimprove/adapter.go` |
| Package | `selfimprove` |
| Challenge | `challenges/scripts/selfimprove_challenge.sh` |

---

## Slide 5: Feedback Types

**Three kinds of feedback signal:**

```go
// Explicit: user directly tells you what they think
type ExplicitFeedback struct {
    PromptHash string
    ResponseID string
    Signal     FeedbackSignal // SignalThumbsUp | SignalThumbsDown | SignalRating
    Rating     float64        // 1.0–5.0 for star ratings
    UserID     string
    Comment    string         // optional free-text
    Timestamp  time.Time
}

// Implicit: inferred from user behavior
type ImplicitFeedback struct {
    PromptHash   string
    ResponseID   string
    Signal       ImplicitSignal // SignalCopied | SignalFollowUp | SignalAbandoned
    SessionDuration time.Duration
    Timestamp    time.Time
}

// PreferencePair: for DPO/RLHF training — one response preferred over another
type PreferencePair struct {
    Prompt    string
    Chosen    string  // preferred response
    Rejected  string  // less preferred response
}
```

---

## Slide 6: RewardModel Interface

**The contract for scoring responses:**

```go
type RewardModel interface {
    // Score a single (prompt, response) pair
    // Returns 0.0–1.0 where 1.0 = best possible
    Score(ctx context.Context,
        prompt, response string) (float64, error)

    // Train on preference pairs (DPO-style)
    Train(ctx context.Context,
        pairs []PreferencePair) error

    // Evaluate model quality on a held-out set
    Evaluate(ctx context.Context,
        dataset []PreferencePair) (*RewardMetrics, error)
}

// Built-in implementation
type InMemoryRewardModel struct{ ... }
func NewInMemoryRewardModel() *InMemoryRewardModel
```

---

## Slide 7: FeedbackCollector

**Buffering and batching feedback signals:**

```go
type FeedbackCollector struct {
    buffer    []FeedbackRecord
    mu        sync.Mutex
    batchSize int
    onBatch   func([]PreferencePair)
}

// Collect feedback from any source
collector := selfimprove.NewFeedbackCollector(
    selfimprove.CollectorConfig{
        BatchSize: 100,
        OnBatch: func(pairs []PreferencePair) {
            // Called when buffer reaches BatchSize
            rewardModel.Train(ctx, pairs)
        },
    },
)

collector.Record(selfimprove.ExplicitFeedback{
    PromptHash: hash(prompt),
    ResponseID: response.ID,
    Signal:     selfimprove.SignalThumbsUp,
})
```

---

## Slide 8: SelfRefinementLoop

**Iterative response improvement:**

```go
type SelfRefinementConfig struct {
    MaxIterations  int     // stop after N iterations
    ScoreThreshold float64 // stop when score exceeds this
    CritiquePrompt string  // template for the critique step
    RefinePrompt   string  // template for the refinement step
}

refiner := selfimprove.NewSelfRefinementLoop(
    selfimprove.SelfRefinementConfig{
        MaxIterations:  3,
        ScoreThreshold: 0.85,
        CritiquePrompt: "Critique for accuracy and clarity: ",
        RefinePrompt:   "Improve based on critique: ",
    },
)

refined, metrics, err := refiner.Refine(
    ctx, prompt, initialResponse, llmClient, rewardModel)

// metrics.Iterations: 2 (stopped early, threshold reached)
// metrics.InitialScore: 0.62
// metrics.FinalScore:   0.89
// metrics.Improvement:  0.27
```

---

## Slide 9: SelfRefinementLoop Execution

**What happens inside each iteration:**

```
Iteration 1:
  Input:    "Quick sort works by partitioning."
  Critique: "Too brief. Missing: base case, time complexity, example."
  Refined:  "Quick sort recursively partitions an array around a pivot.
             Base case: array of size ≤ 1. Average O(n log n).
             Example: [3,1,4] → pivot=3 → [1] + [3] + [4]."
  Score:    0.62 → 0.79

Iteration 2:
  Input:    (refined from iteration 1)
  Critique: "Good. Add space complexity and worst-case analysis."
  Refined:  (further improved)
  Score:    0.79 → 0.89  ✓ threshold reached — STOP
```

---

## Slide 10: HelixAgent Integration

**Wiring SelfImprove into HelixAgent streaming:**

```go
// In streaming response handler
func (h *ChatHandler) handleStream(c *gin.Context) {
    // ... generate response chunks ...
    resp := collectChunks(stream)

    // Record implicit feedback: session duration, follow-up rate
    h.feedbackCollector.Record(selfimprove.ImplicitFeedback{
        PromptHash:      hash(req.Messages),
        ResponseID:      resp.ID,
        Signal:          inferSignal(session),
        SessionDuration: session.Duration(),
    })
}

// Check current self-improvement metrics
curl http://localhost:7061/v1/selfimprove/metrics | jq '{
  "mean_reward_score": .mean_reward_score,
  "feedback_collected": .feedback_collected,
  "training_runs": .training_runs
}'
```

---

## Speaker Notes

### Slide 3 Notes
Emphasize that SelfImprove does NOT train the LLM weights — it manages the data collection
and feedback routing that feeds into fine-tuning pipelines. Think of it as the data layer
for RLHF, not the training layer.

### Slide 8 Notes
The SelfRefinementLoop is a lightweight version of what the AI Debate multi-pass validation
does. The key difference: debate uses multiple agents for critique; SelfRefinementLoop uses
a single agent critiquing its own output. Both are valid approaches.

### Slide 9 Notes
Walk through the iteration trace live. Show that the system stops as soon as the threshold
is exceeded — it does not waste LLM calls on already-good responses.
