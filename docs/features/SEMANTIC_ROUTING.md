# Semantic Routing — DEPRECATED (removed 2026-04-11)

The `internal/routing/semantic/` package was removed during the 2026-04-11 dead-code
sweep. It had **zero Go importers** across the entire repository — the package
compiled, its `_test.go` files ran in isolation, but no production code path ever
constructed or called it. It was a stranded branch from an earlier routing
experiment.

## Where routing now lives

- **Layer 1 (deterministic / tool-calls)** — `internal/handlers/handler.go:processWithDirectProvider()`
  bypasses the ensemble for any request that contains tools and routes to a single
  provider (HelixLLM first if enabled, cloud fallback otherwise).
- **Layer 2 (ensemble / debate)** — `internal/llm/ensemble.go` +
  `internal/services/debate_service.go` perform confidence-weighted ensemble
  aggregation across multiple providers.
- **Layer 3 (fallback-chain)** — `internal/services/provider_registry.go` ranks
  providers by LLMsVerifier score and falls back on circuit-breaker trips.

If you need embedding-similarity routing in the future, re-introduce it as a new
module under `internal/routing/` with concrete importers — do not restore the old
package from git history without a caller.

## Historical reference
See git history at commit `97345b4742ea680a5bbc46c8f926ed5c70ed87e8^` for the
original implementation.
