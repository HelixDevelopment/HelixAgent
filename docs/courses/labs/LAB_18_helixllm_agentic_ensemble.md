# Lab 18: HelixLLM and AgenticEnsemble

## Objective
Configure HelixLLM as a provider and explore AgenticEnsemble dual-mode operation.

## Prerequisites
- HelixAgent built (`make build`)
- At least one LLM provider API key configured
- HelixLLM submodule available (`HelixLLM/`)

## Exercise 1: Configure HelixLLM Provider

```bash
# Add to .env
HELIX_LLM_ENDPOINT=https://localhost:8443
HELIX_LLM_API_KEY=your-helix-llm-key
HELIX_LLM_MODEL=helixllm-default

# Start HelixAgent
./bin/helixagent

# Verify HelixLLM is registered as a provider
curl http://localhost:7061/v1/providers/status | \
  python3 -c "import json,sys; d=json.load(sys.stdin); print('helixllm' in str(d))"
```

**Expected:** HelixLLM appears in provider list.

## Exercise 2: HelixLLM Health Check

```bash
# Check HelixLLM health through HelixAgent
curl http://localhost:7061/v1/providers/health | python3 -m json.tool

# Direct HelixLLM health check (if running locally)
curl -k https://localhost:8443/internal/health
```

**Expected:** Health status returned for HelixLLM.

## Exercise 3: AgenticEnsemble Reason Mode

```bash
# Submit a reasoning task
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "Explain the tradeoffs between microservices and monolith architectures for a team of 5 developers."}
    ]
  }'
```

**Observe:** The response uses debate + reasoning (Reason mode).

## Exercise 4: AgenticEnsemble Execute Mode

```bash
# Submit a multi-step execution task
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "Create a comprehensive test plan for a REST API that handles user authentication, including unit tests, integration tests, and security tests."}
    ]
  }'
```

**Observe:** The response involves task decomposition and verification.

## Exercise 5: Run AgenticEnsemble Challenge

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  ./challenges/scripts/agentic_ensemble_challenge.sh
```

**Expected:** All tests pass.

## Assessment Questions
1. What is the difference between Reason mode and Execute mode?
2. How does the intent classifier determine which mode to use?
3. What happens if the IterativeToolExecutor dependency is nil?
4. How does HelixLLM differ from other providers like OpenAI or Claude?
