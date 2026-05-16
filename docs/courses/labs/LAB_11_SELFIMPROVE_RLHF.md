# Lab 11: SelfImprove — RLHF and Self-Refinement Loops

## Lab Overview

**Duration**: 30 minutes
**Difficulty**: Advanced
**Module**: S7.1.3 — SelfImprove Module

## Objectives

By completing this lab, you will:
- Collect explicit and implicit feedback using FeedbackCollector
- Score responses using an InMemoryRewardModel
- Build and run a SelfRefinementLoop
- Observe quality improvement across iterations
- Integrate feedback collection with HelixAgent streaming

## Prerequisites

- Module S7.1.2 (LLMOps Lab) completed
- At least one LLM provider configured
- `SelfImprove/` module available in the project

---

## Exercise 1: Feedback Collection (8 minutes)

### Task 1.1: Build the FeedbackCollector

```go
package selfimprove_lab_test

import (
    "context"
    "crypto/sha256"
    "fmt"
    "testing"
    "time"

    "digital.vasic.selfimprove/selfimprove"
)

func hashString(s string) string {
    h := sha256.Sum256([]byte(s))
    return fmt.Sprintf("%x", h[:8])
}

func TestFeedbackCollection(t *testing.T) {
    ctx := context.Background()

    var collectedBatches [][]selfimprove.PreferencePair

    collector := selfimprove.NewFeedbackCollector(
        selfimprove.CollectorConfig{
            BatchSize: 3, // trigger batch processing after 3 feedbacks
            OnBatch: func(pairs []selfimprove.PreferencePair) {
                collectedBatches = append(collectedBatches, pairs)
                t.Logf("Batch received: %d preference pairs", len(pairs))
            },
        },
    )

    // Simulate 4 explicit feedback events
    prompts := []string{
        "Explain Go goroutines",
        "Write a Go HTTP server",
        "What is a Go interface?",
        "How do Go channels work?",
    }
    goodResponses := []string{
        "Goroutines are lightweight threads managed by the Go runtime...",
        "func main() { http.ListenAndServe(':8080', nil) }",
        "An interface defines a set of method signatures...",
        "Channels are typed conduits for communication between goroutines...",
    }
    badResponses := []string{
        "They are like threads",
        "Use net/http",
        "It's a type",
        "They send data",
    }

    for i, prompt := range prompts {
        // Positive feedback for good response
        collector.Record(selfimprove.ExplicitFeedback{
            PromptHash: hashString(prompt),
            ResponseID: fmt.Sprintf("good-%d", i),
            Signal:     selfimprove.SignalThumbsUp,
            UserID:     "lab-user",
            Timestamp:  time.Now(),
        })
        // Negative feedback for bad response (creates a preference pair)
        collector.Record(selfimprove.ExplicitFeedback{
            PromptHash: hashString(prompt),
            ResponseID: fmt.Sprintf("bad-%d", i),
            Signal:     selfimprove.SignalThumbsDown,
            UserID:     "lab-user",
            Timestamp:  time.Now(),
        })
        _ = ctx
        _ = goodResponses[i]
        _ = badResponses[i]
    }

    t.Logf("Batches collected: %d", len(collectedBatches))
    t.Logf("Total preference pairs: %d",
        func() int { n := 0; for _, b := range collectedBatches { n += len(b) }; return n }())
}
```

### Task 1.2: Record Feedback Results

| Metric | Value |
|--------|-------|
| Total feedbacks recorded | |
| Batches triggered | |
| Total preference pairs | |
| Batch size trigger | 3 |

---

## Exercise 2: Train and Use a RewardModel (7 minutes)

### Task 2.1: Create and Score with RewardModel

```go
func TestRewardModel(t *testing.T) {
    ctx := context.Background()

    rewardModel := selfimprove.NewInMemoryRewardModel()

    // Provide training data (preference pairs)
    pairs := []selfimprove.PreferencePair{
        {
            Prompt:   "Explain Go goroutines",
            Chosen:   "Goroutines are lightweight threads managed by the Go runtime, enabling concurrency. They use cooperative scheduling and communicate via channels.",
            Rejected: "They are like threads",
        },
        {
            Prompt:   "What is a Go interface?",
            Chosen:   "An interface defines a set of method signatures. Any type implementing all methods satisfies the interface — no explicit declaration needed.",
            Rejected: "It is a type",
        },
        {
            Prompt:   "How do Go channels work?",
            Chosen:   "Channels are typed conduits for goroutine communication. A send blocks until a receiver is ready; a receive blocks until data is available.",
            Rejected: "They send data",
        },
    }

    // Train the model
    err := rewardModel.Train(ctx, pairs)
    if err != nil {
        t.Fatalf("training failed: %v", err)
    }
    t.Log("Training complete")

    // Score several candidate responses
    candidates := []struct {
        description string
        response    string
    }{
        {"Minimal (should score low)",
            "They are lightweight"},
        {"Medium (partial explanation)",
            "Goroutines are lightweight concurrent functions in Go"},
        {"Full (should score high)",
            "Goroutines are lightweight threads managed by the Go runtime. " +
            "They enable concurrent execution. The runtime schedules them " +
            "cooperatively. They communicate via channels."},
    }

    prompt := "Explain Go goroutines"
    t.Log("=== Response Scores ===")
    for _, c := range candidates {
        score, err := rewardModel.Score(ctx, prompt, c.response)
        if err != nil {
            t.Fatalf("scoring failed: %v", err)
        }
        t.Logf("  [%.3f] %s", score, c.description)
    }
}
```

### Task 2.2: Record Scores

| Response Type | Score |
|--------------|-------|
| Minimal | |
| Medium | |
| Full | |

Did the scores match your expectations? (higher score = better response) ___________

---

## Exercise 3: SelfRefinementLoop (10 minutes)

### Task 3.1: Build the Refinement Loop

```go
func TestSelfRefinementLoop(t *testing.T) {
    ctx := context.Background()

    // Use a mock LLM client for the lab
    // In production: use a real HelixAgent LLM client
    llmClient := newMockLLMClient()
    rewardModel := selfimprove.NewInMemoryRewardModel()

    // Train reward model
    rewardModel.Train(ctx, trainingPairs)

    refiner := selfimprove.NewSelfRefinementLoop(
        selfimprove.SelfRefinementConfig{
            MaxIterations:  3,
            ScoreThreshold: 0.85,
            CritiquePrompt: "Critique this Go explanation for technical accuracy, " +
                "completeness, and clarity. List specific improvements needed: ",
            RefinePrompt: "Improve this Go explanation based on the critique. " +
                "Make it more accurate, complete, and clear: ",
        },
    )

    prompt := "Explain Go goroutines"
    initialResponse := "They are lightweight concurrent things in Go."

    t.Logf("Initial response: %q", initialResponse)

    initial_score, _ := rewardModel.Score(ctx, prompt, initialResponse)
    t.Logf("Initial score: %.3f", initial_score)

    refined, metrics, err := refiner.Refine(
        ctx, prompt, initialResponse, llmClient, rewardModel)
    if err != nil {
        t.Fatalf("refinement failed: %v", err)
    }

    t.Logf("=== Refinement Results ===")
    t.Logf("Iterations:    %d", metrics.Iterations)
    t.Logf("Initial score: %.3f", metrics.InitialScore)
    t.Logf("Final score:   %.3f", metrics.FinalScore)
    t.Logf("Improvement:   %.3f (%.1f%%)",
        metrics.Improvement, metrics.Improvement*100)
    t.Logf("Refined response: %q", refined)

    if metrics.FinalScore <= metrics.InitialScore {
        t.Error("FAIL: refinement did not improve the score")
    }
    if metrics.Improvement < 0.1 {
        t.Error("FAIL: improvement less than 10%")
    }
}
```

### Task 3.2: Record Refinement Progression

| Iteration | Score | Improvement |
|-----------|-------|-------------|
| 0 (initial) | | - |
| 1 | | |
| 2 | | |
| 3 (if reached) | | |

Did the loop stop early (before MaxIterations=3)? ___
If yes, at what iteration and score? ___

---

## Exercise 4: Integration Check (5 minutes)

### Task 4.1: Run Module Tests

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_agent/SelfImprove
GOMAXPROCS=2 nice -n 19 go test ./... -v -short -count=1 2>&1 | tail -30
```

### Task 4.2: Run Challenge

```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
./challenges/scripts/selfimprove_challenge.sh 2>/dev/null | tail -15
```

---

## Lab Completion Checklist

- [ ] FeedbackCollector recorded and batched feedback
- [ ] RewardModel trained on preference pairs
- [ ] Score ordering confirmed (full > medium > minimal)
- [ ] SelfRefinementLoop improved score by at least 10%
- [ ] Module tests pass

**Final Metrics:**
- Initial response score: ____________
- Final response score: ____________
- Total improvement: ____________%
- Iterations used: ____________

---

## Troubleshooting

### "reward model: not trained"
Call `rewardModel.Train(ctx, pairs)` before `rewardModel.Score()`.

### "refinement: no improvement after 3 iterations"
Check that the LLM client is producing varied responses. With a mock client,
ensure the mock generates progressively better responses each iteration.

### Import errors
Ensure `go.mod` has: `replace digital.vasic.selfimprove => ./SelfImprove`

---

*Lab Version: 1.0.0*
*Last Updated: February 2026*
