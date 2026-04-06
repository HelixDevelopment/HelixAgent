# Lab 21: HelixMemory Cognitive Engine

## Objective
Explore the HelixMemory unified cognitive memory engine and its integration with AI debate.

## Prerequisites
- HelixAgent built and running (`make build && ./bin/helixagent`)
- HelixMemory active (default, no special build tags needed)

## Exercise 1: Verify HelixMemory is Active

```bash
# HelixMemory is active by default
# Verify by checking health endpoint
curl http://localhost:7061/health | python3 -c "
import json, sys
data = json.load(sys.stdin)
print('HelixMemory status:', json.dumps(data, indent=2))
"
```

**Expected:** HelixMemory components visible in health response.

## Exercise 2: Memory Store and Recall

```bash
# Store a fact via the memory-related completion
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "system", "content": "Remember: The project uses PostgreSQL 15 as the primary database."},
      {"role": "user", "content": "What database does the project use?"}
    ]
  }'
```

**Observe:** The response should incorporate the stored fact.

## Exercise 3: Cross-Session Context

```bash
# First session: establish context
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "We decided to use a microservices architecture for the payment system."}
    ]
  }'

# Second session: recall context
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "What architecture did we decide for the payment system?"}
    ]
  }'
```

**Expected:** Second call recalls the architectural decision from the first call.

## Exercise 4: Check Prometheus Metrics

```bash
# View HelixMemory metrics
curl -s http://localhost:7061/metrics | grep helixmemory
```

**Expected:** Metrics for operation duration, cache hits/misses, entity counts.

## Exercise 5: Run HelixMemory Challenge

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  ./challenges/scripts/helixmemory_challenge.sh
```

**Expected:** 80+ tests pass.

## Assessment Questions
1. What are the four memory backends in HelixMemory and what does each provide?
2. How does the 3-stage fusion pipeline combine data from multiple backends?
3. What happens when a memory backend is unavailable (circuit breaker behavior)?
4. How does HelixMemory enrich AI debate sessions?
