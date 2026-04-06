# Lab 22: HelixSpecifier Spec-Driven Development

## Objective
Execute the HelixSpecifier 7-phase specification pipeline and observe DebateFunc integration.

## Prerequisites
- HelixAgent built and running (`make build && ./bin/helixagent`)
- HelixSpecifier active (default, no special build tags needed)
- At least one LLM provider configured for debate

## Exercise 1: Verify HelixSpecifier is Active

```bash
# HelixSpecifier is active by default
# Check health status
curl http://localhost:7061/health | python3 -m json.tool
```

**Expected:** HelixSpecifier components visible in system status.

## Exercise 2: Trigger Auto-Activation

```bash
# Submit a task with "refactoring" granularity to trigger full 7-phase pipeline
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "Refactor the authentication module to support OAuth2 and SAML in addition to JWT. This requires changes across the middleware, handlers, and configuration layers."}
    ]
  }'
```

**Observe:** The response should show evidence of specification phases:
- Constitution alignment
- Specification authoring
- Risk analysis

## Exercise 3: Small Task (Minimal Ceremony)

```bash
# Submit a small task - should skip most phases
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "Add a unit test for the health check handler."}
    ]
  }'
```

**Observe:** Less ceremony for small tasks. Adaptive scaling skips unnecessary phases.

## Exercise 4: Check Spec Cache

```bash
# HelixSpecifier caches phases for resumption
ls -la .speckit/cache/ 2>/dev/null || echo "No cache yet (first run)"

# After running exercises 2 and 3, check again
ls -la .speckit/cache/ 2>/dev/null
```

**Expected:** Cache files present after phase execution.

## Exercise 5: Run HelixSpecifier Challenge

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  ./challenges/scripts/helixspecifier_challenge.sh
```

**Expected:** 138 tests pass.

## Assessment Questions
1. What are the 5 work granularity levels and how does HelixSpecifier detect them?
2. Which granularity levels trigger the full 7-phase pipeline?
3. How does DebateFunc inject real multi-LLM debate into the specification process?
4. What is stored in `.speckit/cache/` and why does it matter for resumption?
5. How do the 3 pillars (SpecKit, Superpowers, GSD) complement each other?
