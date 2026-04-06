# Module 19: HelixSpecifier Spec-Driven Development

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 19: HelixSpecifier Spec-Driven Development
- Duration: 75 minutes
- 3-Pillar Specification-Driven Development Fusion Engine

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand the 3-pillar architecture (SpecKit + Superpowers + GSD)
- Configure auto-activation based on work granularity detection
- Execute the 7-phase SpecKit development flow
- Integrate HelixSpecifier with AI debate via DebateFunc injection

---

## Slide 3: Why Spec-Driven Development?

**The problem with ad-hoc AI-assisted development:**

| Ad-hoc | Spec-Driven (HelixSpecifier) |
|--------|------------------------------|
| Jump straight to code | Constitution alignment first |
| Ambiguous requirements | 7-phase clarification pipeline |
| No ceremony scaling | Adaptive: trivial tasks skip phases |
| Single perspective | Multi-LLM debate on specs |
| No memory of decisions | Spec memory for cross-session context |

---

## Slide 4: Module Identity

**`digital.vasic.helixspecifier`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.helixspecifier` |
| Source directory | `HelixSpecifier/` |
| Packages | 27 (21 core + 6 test suites) |
| Tests | 835+ (unit, integration, E2E, security, stress, benchmark) |
| Default state | Active (opt out with `-tags nohelixspecifier`) |
| Challenge | `challenges/scripts/helixspecifier_challenge.sh` (138 tests) |

---

## Slide 5: 3-Pillar Architecture

**Three complementary methodologies fused:**

```
+------------------+  +------------------+  +------------------+
|    SpecKit       |  |   Superpowers    |  |      GSD         |
| 7-Phase SDD     |  | TDD + Subagents  |  | Milestone-Driven |
+--------+---------+  +--------+---------+  +--------+---------+
         |                     |                     |
         +----------+----------+----------+----------+
                    |                     |
              +-----v-----+        +-----v-----+
              | 3-Pillar  |        |  Intent   |
              |  Fusion   |        | Classifier|
              +-----------+        +-----------+
```

- **SpecKit**: 7-phase specification-driven development
- **Superpowers**: TDD methodology with subagent coordination
- **GSD**: Get Stuff Done -- milestone-based execution tracking

---

## Slide 6: 7-Phase SpecKit Flow

**The complete specification pipeline:**

```
Phase 1: CONSTITUTION   -- Align with project rules and constraints
    |
Phase 2: SPECIFY        -- Author the specification
    |
Phase 3: CLARIFY        -- Disambiguate requirements
    |
Phase 4: PLAN           -- Decompose into implementation plan
    |
Phase 5: TASKS          -- Generate executable tasks
    |
Phase 6: ANALYZE        -- Risk assessment and impact analysis
    |
Phase 7: IMPLEMENT      -- Implementation guidance and execution
```

*Phase caching enables resumption: `.speckit/cache/`*

---

## Slide 7: Adaptive Ceremony Scaling

**Not every task needs all 7 phases:**

| Granularity | Example | Phases Used |
|-------------|---------|-------------|
| Single Action | Fix a typo | Phase 7 only |
| Small Creation | Add a test | Phases 5-7 |
| Big Creation | New API endpoint | Phases 2-7 |
| Whole Functionality | New module | All 7 phases |
| Refactoring | Architecture change | All 7 phases + debate |

**WorkGranularity detection is automatic via intent classifier**

Auto-activation triggers for: `GranularityBigCreation`, `GranularityWholeFunctionality`, `GranularityRefactoring`

---

## Slide 8: DebateFunc Integration

**Real multi-LLM debate within spec workflow:**

```go
// DebateFunc is injected into HelixSpecifier
// When a specification decision needs consensus:

specifier.SetDebateFunc(func(ctx context.Context,
    topic string, context string) (*DebateResult, error) {

    // This triggers an actual AI debate with 5 positions x 5 LLMs
    return debateService.ConductDebate(ctx, topic, context)
})

// Use cases for debate within specs:
// - Architecture decisions (monolith vs microservices)
// - API design choices (REST vs gRPC vs GraphQL)
// - Technology selection (which database, which framework)
// - Risk assessment (security implications, performance impact)
```

---

## Slide 9: Spec Memory

**Cross-session specification context:**

- Previous specifications recalled for consistency
- Decisions documented with rationale
- Architecture evolution tracked over time
- Prevents contradicting earlier specifications
- Integrates with HelixMemory cognitive engine

---

## Slide 10: 10 Power Features

**HelixSpecifier capabilities:**

1. 7-phase SDD pipeline with adaptive ceremony
2. 3-pillar fusion (SpecKit + Superpowers + GSD)
3. DebateFunc injection for real multi-LLM debate
4. Work granularity detection (5 levels)
5. Phase caching for resumption
6. Intent classifier for automatic phase selection
7. Spec memory for cross-session context
8. CLI agent adapters for agent-specific generation
9. Effort classification and estimation
10. Constitution alignment verification

---

## Slide 11: SpecKit Orchestrator in HelixAgent

**Key integration point:**

```
internal/services/speckit_orchestrator.go
    |
    +-- Detects work granularity from user request
    |
    +-- GranularityRefactoring? --> Full 7-phase pipeline
    |   GranularityBigCreation? --> Phases 2-7
    |   GranularitySmall?       --> Phases 5-7
    |
    +-- Each phase may trigger DebateFunc
    |
    +-- Results cached in .speckit/cache/
    |
    +-- Final output: specification + implementation plan
```

---

## Slide 12: Hands-On Lab

**Lab Exercise 19.1: Spec-Driven Development Workflow**

Tasks:
1. Trigger auto-activation with a refactoring task
2. Observe the 7-phase flow and phase indicators
3. Watch DebateFunc trigger a real multi-LLM debate
4. Review the generated specification
5. Examine spec memory for cross-session context

Time: 30 minutes

---

## Slide 13: Module Summary

**Key Takeaways:**

- HelixSpecifier fuses 3 methodologies: SpecKit + Superpowers + GSD
- 7-phase pipeline with adaptive ceremony scaling
- Auto-activates for large changes based on granularity detection
- DebateFunc enables real multi-LLM debate within specifications
- Spec memory ensures cross-session consistency
- Phase caching allows resumption of interrupted workflows
- 27 packages, 835+ tests, active by default

**Course Complete! See Certification Path for next steps.**

---

## Speaker Notes

### Slide 3 Notes
The key insight: for complex changes, spending time on specification SAVES time
on implementation. HelixSpecifier automates the specification process using
multiple LLMs, so the cost of specification is low but the quality is high.

### Slide 7 Notes
Adaptive ceremony is crucial for developer experience. Nobody wants a 7-phase
pipeline for a one-line fix. The granularity detector ensures the right amount
of process for each task.

### Slide 8 Notes
DebateFunc is the integration point between HelixSpecifier and the AI debate
system. When a spec decision is genuinely ambiguous, multiple LLMs debate it
and reach consensus. This is the same debate system from Module 6/14 but
applied to specification decisions rather than user queries.
