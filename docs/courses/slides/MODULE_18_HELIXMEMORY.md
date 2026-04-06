# Module 18: HelixMemory Cognitive Engine

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 18: HelixMemory Cognitive Engine
- Duration: 75 minutes
- Unified Cognitive Memory for AI Debate Ensemble

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand the HelixMemory 3-stage fusion pipeline
- Configure Mem0, Cognee, Letta, and Graphiti memory backends
- Integrate HelixMemory with the AI debate ensemble
- Monitor memory operations via circuit breakers and Prometheus metrics

---

## Slide 3: The Memory Problem

**Why AI systems need persistent memory:**

| Without Memory | With HelixMemory |
|----------------|------------------|
| Every conversation starts fresh | Cross-session context |
| No learning from past debates | Accumulated wisdom |
| Repeated mistakes | Knowledge graph enrichment |
| No entity relationships | Temporal knowledge tracking |
| Isolated agent sessions | Shared cognitive state |

---

## Slide 4: Module Identity

**`digital.vasic.helixmemory`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.helixmemory` |
| Source directory | `HelixMemory/` |
| Packages | 12+ packages |
| Default state | Active (opt out with `-tags nohelixmemory`) |
| Challenge | `challenges/scripts/helixmemory_challenge.sh` (80+ tests) |

---

## Slide 5: Four Memory Backends

**Orchestrated through 3-stage fusion:**

| Backend | Purpose | Data Type |
|---------|---------|-----------|
| **Mem0** | Fact-based memory | Structured facts, preferences |
| **Cognee** | Knowledge graphs | Entity relationships, ontologies |
| **Letta** | Stateful agent runtime | Agent state, conversation history |
| **Graphiti** | Temporal knowledge graph | Time-aware entity evolution |

*Each backend provides a different dimension of memory*

---

## Slide 6: 3-Stage Fusion Pipeline

**How HelixMemory combines all backends:**

```
Stage 1: COLLECT
  Mem0     --> facts
  Cognee   --> relationships
  Letta    --> agent state
  Graphiti --> temporal context
         |
Stage 2: FUSE
  Merge overlapping entities
  Resolve conflicts (recency wins)
  Build unified context graph
         |
Stage 3: DELIVER
  Enriched context for LLM prompts
  Memory-augmented debate responses
  Cross-session knowledge continuity
```

---

## Slide 7: 12 Power Features

**HelixMemory capabilities:**

1. Cross-session fact retention (Mem0)
2. Entity graph construction (Cognee)
3. Semantic memory search
4. Temporal knowledge tracking (Graphiti)
5. Memory consolidation and pruning
6. Circuit breakers per backend
7. Prometheus metrics for all operations
8. Memory scopes (session, user, global)
9. Stateful agent runtime integration (Letta)
10. Debate-aware memory enrichment
11. Infrastructure bridge for backend connectivity
12. Graceful degradation when backends are unavailable

---

## Slide 8: Circuit Breakers

**Fault tolerance for memory backends:**

```
Memory Request
    |
    v
+-------------------+
| Circuit Breaker   |
| State: CLOSED     |---> Backend call
+-------------------+
    |
    | 5 failures in 30s
    v
+-------------------+
| Circuit Breaker   |
| State: OPEN       |---> Return cached/empty
+-------------------+     (no backend call)
    |
    | After 60s cooldown
    v
+-------------------+
| Circuit Breaker   |
| State: HALF-OPEN  |---> Test single call
+-------------------+
```

*Each backend has its own circuit breaker*

---

## Slide 9: HelixMemory in AI Debate

**Memory-augmented debate flow:**

```
Debate Topic Received
    |
    v
HelixMemory.Recall(topic, entities)
    |
    +-- Prior debate conclusions on similar topics
    +-- Entity relationships relevant to discussion
    +-- Temporal context (what changed since last debate)
    |
    v
Debate Participants receive enriched context
    |
    v
Debate Concludes
    |
    v
HelixMemory.Store(conclusion, entities, relationships)
    |
    +-- New facts stored in Mem0
    +-- Entity graph updated in Cognee
    +-- Temporal snapshot in Graphiti
```

---

## Slide 10: Prometheus Metrics

**Monitoring memory operations:**

```bash
# Memory operation latency
helixmemory_operation_duration_seconds{backend="mem0",operation="store"}
helixmemory_operation_duration_seconds{backend="cognee",operation="recall"}

# Circuit breaker state
helixmemory_circuit_breaker_state{backend="letta"}

# Cache hits/misses
helixmemory_cache_hits_total
helixmemory_cache_misses_total

# Entity counts
helixmemory_entities_total{scope="global"}
```

---

## Slide 11: Hands-On Lab

**Lab Exercise 18.1: Cognitive Memory Pipeline**

Tasks:
1. Verify HelixMemory is active (default, no build tags needed)
2. Store facts through the memory API
3. Retrieve facts with semantic search
4. Run an AI debate and observe memory enrichment
5. Query cross-session knowledge

Time: 30 minutes

---

## Slide 12: Module Summary

**Key Takeaways:**

- HelixMemory orchestrates 4 memory backends through 3-stage fusion
- Mem0 (facts) + Cognee (graphs) + Letta (state) + Graphiti (temporal)
- Active by default; opt out with `-tags nohelixmemory`
- Circuit breakers protect against backend failures
- Prometheus metrics for operational monitoring
- AI debate sessions are automatically memory-enriched
- Memory scopes: session, user, global

**Next: Module 19 - HelixSpecifier Spec-Driven Development**

---

## Speaker Notes

### Slide 5 Notes
Emphasize that each backend provides a DIFFERENT dimension of memory. Mem0 stores
isolated facts. Cognee builds relationship graphs. Graphiti adds the time dimension.
Letta maintains agent state. The fusion pipeline combines all four.

### Slide 8 Notes
Circuit breakers are essential for production. If Cognee is down, the system should
not fail entirely. It degrades gracefully, using available backends.

### Slide 9 Notes
This is the key integration point. Show how a debate about "Should we migrate to
microservices?" can recall that a previous debate concluded "monolith is preferred
for teams under 10 engineers" and use that as context.
