# HelixAgent Provider Validation Report

**Generated:** Thu, 16 Apr 2026 01:46:09 MSK

**HelixAgent Server:** http://localhost:7061 (healthy: true)

**Total Providers:** 36 | **With API Keys:** 29 | **Without Keys:** 7

**Healthy:** 7 | **Auth Failed:** 11 | **Unreachable:** 1 | **Other Failures:** 10

---

## Summary Table

| # | Provider | Env Var | Key | Status | Latency | Root Cause |
|---|----------|---------|-----|--------|---------|------------|
| 1 | claude | `ANTHROPIC_API_KEY` | N/A | KEY no_key |  | Environment variable ANTHROPIC_API_KEY is not set. Set it with: export ANTHRO... |
| 2 | openai | `OPENAI_API_KEY` | N/A | KEY no_key |  | Environment variable OPENAI_API_KEY is not set. Set it with: export OPENAI_AP... |
| 3 | gemini | `GEMINI_API_KEY` | AIza...w8Lg (39 chars) | FAIL bad_request | 428ms | Provider gemini (GEMINI_API_KEY) bad request (HTTP 400). Error: API Key not f... |
| 4 | deepseek | `DEEPSEEK_API_KEY` | sk-c...3935 (35 chars) | OK healthy | 452ms |  |
| 5 | mistral | `MISTRAL_API_KEY` | XBne...kSAH (32 chars) | OK healthy | 600ms |  |
| 6 | codestral | `CODESTRAL_API_KEY` | 3AFH...yiII (32 chars) | OK healthy | 324ms |  |
| 7 | qwen | `QWEN_API_KEY` | N/A | KEY no_key |  | Environment variable QWEN_API_KEY is not set. Set it with: export QWEN_API_KE... |
| 8 | xai | `XAI_API_KEY` | N/A | KEY no_key |  | Environment variable XAI_API_KEY is not set. Set it with: export XAI_API_KEY=... |
| 9 | zai | `ZAI_API_KEY` | 8dd4...Bps0 (49 chars) | RATE rate_limited | 705ms | Provider zai (ZAI_API_KEY) rate limited. The API key is valid but quota is ex... |
| 10 | github-models | `GITHUB_MODELS_API_KEY` | gith...hkbp (93 chars) | AUTH auth_failed | 489ms | Provider github-models (GITHUB_MODELS_API_KEY) authentication failed (HTTP 40... |
| 11 | cohere | `COHERE_API_KEY` | banh...mI9o (40 chars) | AUTH auth_failed | 440ms | Provider cohere (COHERE_API_KEY) authentication failed (HTTP 403). The API ke... |
| 12 | perplexity | `PERPLEXITY_API_KEY` | N/A | KEY no_key |  | Environment variable PERPLEXITY_API_KEY is not set. Set it with: export PERPL... |
| 13 | ai21 | `AI21_API_KEY` | N/A | KEY no_key |  | Environment variable AI21_API_KEY is not set. Set it with: export AI21_API_KE... |
| 14 | groq | `GROQ_API_KEY` | gsk_...7xhr (56 chars) | AUTH auth_failed | 139ms | Provider groq (GROQ_API_KEY) authentication failed (HTTP 403). The API key is... |
| 15 | cerebras | `CEREBRAS_API_KEY` | csk-...htj2 (52 chars) | AUTH auth_failed | 317ms | Provider cerebras (CEREBRAS_API_KEY) authentication failed (HTTP 403). The AP... |
| 16 | sambanova | `SAMBANOVA_API_KEY` | 3eb0...1533 (36 chars) | FAIL model_not_found | 766ms | Provider sambanova (SAMBANOVA_API_KEY) model Meta-Llama-3.1-70B-Instruct not ... |
| 17 | fireworks | `FIREWORKS_API_KEY` | fw_J...ezWb (25 chars) | FAIL model_not_found | 324ms | Provider fireworks (FIREWORKS_API_KEY) model accounts/fireworks/models/llama-... |
| 18 | together | `TOGETHER_API_KEY` | N/A | KEY no_key |  | Environment variable TOGETHER_API_KEY is not set. Set it with: export TOGETHE... |
| 19 | hyperbolic | `HYPERBOLIC_API_KEY` | sk_l...W720 (73 chars) | AUTH auth_failed | 209ms | Provider hyperbolic (HYPERBOLIC_API_KEY) authentication failed (HTTP 403). Th... |
| 20 | replicate | `REPLICATE_API_KEY` | r8_4...uxlG (40 chars) | AUTH auth_failed | 262ms | Provider replicate (REPLICATE_API_KEY) authentication failed (HTTP 401). The ... |
| 21 | siliconflow | `SILICONFLOW_API_KEY` | sk-r...kdno (51 chars) | AUTH auth_failed | 1665ms | Provider siliconflow (SILICONFLOW_API_KEY) authentication failed (HTTP 401). ... |
| 22 | cloudflare | `CLOUDFLARE_API_KEY` | h30D...RPhR (40 chars) | FAIL bad_request | 370ms | Provider cloudflare (CLOUDFLARE_API_KEY) bad request (HTTP 400). Error: {"err... |
| 23 | nvidia | `NVIDIA_API_KEY` | nvap...-Tlx (70 chars) | OK healthy | 474ms |  |
| 24 | kimi | `KIMI_API_KEY` | sk-k...JvbE (72 chars) | AUTH auth_failed | 1923ms | Provider kimi (KIMI_API_KEY) authentication failed (HTTP 401). The API key is... |
| 25 | huggingface | `HUGGINGFACE_API_KEY` | hf_e...CpNo (37 chars) | FAIL model_not_found | 309ms | Provider huggingface (HUGGINGFACE_API_KEY) model meta-llama/Llama-3.2-3B-Inst... |
| 26 | novita | `NOVITA_API_KEY` | sk_l...eheo (46 chars) | FAIL model_not_found | 549ms | Provider novita (NOVITA_API_KEY) model meta-llama/llama-3.1-70b-instruct not ... |
| 27 | upstage | `UPSTAGE_API_KEY` | up_S...xhCu (32 chars) | AUTH auth_failed | 1141ms | Provider upstage (UPSTAGE_API_KEY) authentication failed (HTTP 401). The API ... |
| 28 | chutes | `CHUTES_API_KEY` | cpk_...OyQR (102 chars) | FAIL model_not_found | 475ms | Provider chutes (CHUTES_API_KEY) model chutesai/Chutes-Mistral-Nemo-2407 not ... |
| 29 | openrouter | `OPENROUTER_API_KEY` | sk-o...4462 (73 chars) | FAIL model_not_found | 454ms | Provider openrouter (OPENROUTER_API_KEY) model x-ai/grok-4 not found (HTTP 40... |
| 30 | venice | `VENICE_API_KEY` | VENI...GKbU (59 chars) | FAIL model_not_found | 287ms | Provider venice (VENICE_API_KEY) model llama-3.1-70b-instruct not found (HTTP... |
| 31 | sarvam | `SARVAM_API_KEY` | sk_1...bjPY (36 chars) | OK healthy | 1397ms |  |
| 32 | kilo | `KILO_API_KEY` | eyJh...MozU (268 chars) | AUTH auth_failed | 197ms | Provider kilo (KILO_API_KEY) authentication failed (HTTP 403). The API key is... |
| 33 | publicai | `PUBLICAI_API_KEY` | zpka...1d43 (46 chars) | AUTH auth_failed | 329ms | Provider publicai (PUBLICAI_API_KEY) authentication failed (HTTP 401). The AP... |
| 34 | modal | `MODAL_API_KEY` | as-c...3x8Z (25 chars) | OK healthy | 644ms |  |
| 35 | nia | `NIA_API_KEY` | nk_h...PEsw (35 chars) | NET dns_failure | 230ms | Provider nia (NIA_API_KEY) DNS resolution failed for https://api.nia.ai/v1/ch... |
| 36 | nvidia | `NVIDIA_API_KEY` | nvap...-Tlx (70 chars) | OK healthy | 835ms |  |

---

## Healthy Providers

- **deepseek** (`DEEPSEEK_API_KEY`) — OK in 452ms
- **mistral** (`MISTRAL_API_KEY`) — OK in 600ms
- **codestral** (`CODESTRAL_API_KEY`) — OK in 324ms
- **nvidia** (`NVIDIA_API_KEY`) — OK in 474ms
- **sarvam** (`SARVAM_API_KEY`) — OK in 1397ms
- **modal** (`MODAL_API_KEY`) — OK in 644ms
- **nvidia** (`NVIDIA_API_KEY`) — OK in 835ms

## Failed Providers — Detailed Analysis

### gemini (`GEMINI_API_KEY`)

- **Status:** bad_request
- **HTTP Status:** 400
- **Error:** HTTP 400: API Key not found. Please pass a valid API key.
- **Latency:** 428ms
- **Root Cause:** Provider gemini (GEMINI_API_KEY) bad request (HTTP 400). Error: API Key not found. Please pass a valid API key.. Action: Request format may be wrong for this provider's API version

### zai (`ZAI_API_KEY`)

- **Status:** rate_limited
- **HTTP Status:** 429
- **Error:** HTTP 429: Insufficient balance or no resource package. Please recharge.
- **Latency:** 705ms
- **Root Cause:** Provider zai (ZAI_API_KEY) rate limited. The API key is valid but quota is exhausted. Error: Insufficient balance or no resource package. Please recharge.. Action: Wait for rate limit reset or upgrade plan

### github-models (`GITHUB_MODELS_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: (no response body)
- **Latency:** 489ms
- **Root Cause:** Provider github-models (GITHUB_MODELS_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: (no response body). Action: Verify the key at the provider's dashboard and update GITHUB_MODELS_API_KEY

### cohere (`COHERE_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 403
- **Error:** HTTP 403: (no response body)
- **Latency:** 440ms
- **Root Cause:** Provider cohere (COHERE_API_KEY) authentication failed (HTTP 403). The API key is invalid, expired, or revoked. Error: (no response body). Action: Verify the key at the provider's dashboard and update COHERE_API_KEY

### groq (`GROQ_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 403
- **Error:** HTTP 403: Forbidden
- **Latency:** 139ms
- **Root Cause:** Provider groq (GROQ_API_KEY) authentication failed (HTTP 403). The API key is invalid, expired, or revoked. Error: Forbidden. Action: Verify the key at the provider's dashboard and update GROQ_API_KEY

### cerebras (`CEREBRAS_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 403
- **Error:** HTTP 403: (no response body)
- **Latency:** 317ms
- **Root Cause:** Provider cerebras (CEREBRAS_API_KEY) authentication failed (HTTP 403). The API key is invalid, expired, or revoked. Error: (no response body). Action: Verify the key at the provider's dashboard and update CEREBRAS_API_KEY

### sambanova (`SAMBANOVA_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 402
- **Error:** HTTP 404: A payment method is required. Please set up a payment method to continue.
- **Latency:** 766ms
- **Root Cause:** Provider sambanova (SAMBANOVA_API_KEY) model Meta-Llama-3.1-70B-Instruct not found (HTTP 404). Error: A payment method is required. Please set up a payment method to continue.. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### fireworks (`FIREWORKS_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 404
- **Error:** HTTP 404: Model not found, inaccessible, and/or not deployed
- **Latency:** 324ms
- **Root Cause:** Provider fireworks (FIREWORKS_API_KEY) model accounts/fireworks/models/llama-v3p1-70b-instruct not found (HTTP 404). Error: Model not found, inaccessible, and/or not deployed. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### hyperbolic (`HYPERBOLIC_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 403
- **Error:** HTTP 403: (no response body)
- **Latency:** 209ms
- **Root Cause:** Provider hyperbolic (HYPERBOLIC_API_KEY) authentication failed (HTTP 403). The API key is invalid, expired, or revoked. Error: (no response body). Action: Verify the key at the provider's dashboard and update HYPERBOLIC_API_KEY

### replicate (`REPLICATE_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: You did not pass a valid authentication token
- **Latency:** 262ms
- **Root Cause:** Provider replicate (REPLICATE_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: You did not pass a valid authentication token. Action: Verify the key at the provider's dashboard and update REPLICATE_API_KEY

### siliconflow (`SILICONFLOW_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: (no response body)
- **Latency:** 1665ms
- **Root Cause:** Provider siliconflow (SILICONFLOW_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: (no response body). Action: Verify the key at the provider's dashboard and update SILICONFLOW_API_KEY

### cloudflare (`CLOUDFLARE_API_KEY`)

- **Status:** bad_request
- **HTTP Status:** 400
- **Error:** HTTP 400: {"errors":[{"code":7000,"message":"No route for that URI"}],"messages":[],"result":null,"success":false}
- **Latency:** 370ms
- **Root Cause:** Provider cloudflare (CLOUDFLARE_API_KEY) bad request (HTTP 400). Error: {"errors":[{"code":7000,"message":"No route for that URI"}],"messages":[],"result":null,"success":false}. Action: Request format may be wrong for this provider's API version

### kimi (`KIMI_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: Invalid Authentication
- **Latency:** 1923ms
- **Root Cause:** Provider kimi (KIMI_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: Invalid Authentication. Action: Verify the key at the provider's dashboard and update KIMI_API_KEY

### huggingface (`HUGGINGFACE_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 404
- **Error:** HTTP 404: (no response body)
- **Latency:** 309ms
- **Root Cause:** Provider huggingface (HUGGINGFACE_API_KEY) model meta-llama/Llama-3.2-3B-Instruct not found (HTTP 404). Error: (no response body). Action: The model may have been renamed or deprecated. Check provider docs for current model names

### novita (`NOVITA_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 404
- **Error:** HTTP 404: model not found
- **Latency:** 549ms
- **Root Cause:** Provider novita (NOVITA_API_KEY) model meta-llama/llama-3.1-70b-instruct not found (HTTP 404). Error: model not found. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### upstage (`UPSTAGE_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: API key suspended due to insufficient credit. Register your payment method at https://console.upstage.ai/billing to continue.
- **Latency:** 1141ms
- **Root Cause:** Provider upstage (UPSTAGE_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: API key suspended due to insufficient credit. Register your payment method at https://console.upstage.ai/billing to continue.. Action: Verify the key at the provider's dashboard and update UPSTAGE_API_KEY

### chutes (`CHUTES_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 404
- **Error:** HTTP 404: model not found: chutesai/Chutes-Mistral-Nemo-2407
- **Latency:** 475ms
- **Root Cause:** Provider chutes (CHUTES_API_KEY) model chutesai/Chutes-Mistral-Nemo-2407 not found (HTTP 404). Error: model not found: chutesai/Chutes-Mistral-Nemo-2407. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### openrouter (`OPENROUTER_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 402
- **Error:** HTTP 404: This request requires more credits, or fewer max_tokens. You requested up to 50 tokens, but can only afford 1. To increase, visit https://openrouter.ai/settings/credits and add more credits
- **Latency:** 454ms
- **Root Cause:** Provider openrouter (OPENROUTER_API_KEY) model x-ai/grok-4 not found (HTTP 404). Error: This request requires more credits, or fewer max_tokens. You requested up to 50 tokens, but can only afford 1. To increase, visit https://openrouter.ai/settings/credits and add more credits. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### venice (`VENICE_API_KEY`)

- **Status:** model_not_found
- **HTTP Status:** 404
- **Error:** HTTP 404: {"error":"Specified model not found: llama-3.1-70b-instruct. Did you mean: llama-3.3-70b, mistral-small-3-2-24b-instruct, olafangensan-glm-4.7-flash-heretic?"}
- **Latency:** 287ms
- **Root Cause:** Provider venice (VENICE_API_KEY) model llama-3.1-70b-instruct not found (HTTP 404). Error: {"error":"Specified model not found: llama-3.1-70b-instruct. Did you mean: llama-3.3-70b, mistral-small-3-2-24b-instruct, olafangensan-glm-4.7-flash-heretic?"}. Action: The model may have been renamed or deprecated. Check provider docs for current model names

### kilo (`KILO_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 403
- **Error:** HTTP 403: Forbidden
- **Latency:** 197ms
- **Root Cause:** Provider kilo (KILO_API_KEY) authentication failed (HTTP 403). The API key is invalid, expired, or revoked. Error: Forbidden. Action: Verify the key at the provider's dashboard and update KILO_API_KEY

### publicai (`PUBLICAI_API_KEY`)

- **Status:** auth_failed
- **HTTP Status:** 401
- **Error:** HTTP 401: Session check failed
- **Latency:** 329ms
- **Root Cause:** Provider publicai (PUBLICAI_API_KEY) authentication failed (HTTP 401). The API key is invalid, expired, or revoked. Error: Session check failed. Action: Verify the key at the provider's dashboard and update PUBLICAI_API_KEY

### nia (`NIA_API_KEY`)

- **Status:** dns_failure
- **Error:** Post "https://api.nia.ai/v1/chat/completions": dial tcp: lookup api.nia.ai on 192.168.0.1:53: no such host
- **Latency:** 230ms
- **Root Cause:** Provider nia (NIA_API_KEY) DNS resolution failed for https://api.nia.ai/v1/chat/completions. Check network connectivity.


## Providers Without API Keys

These providers need API keys to be configured:

| Provider | Env Var | Action |
|----------|---------|--------|
| claude | `ANTHROPIC_API_KEY` | `export ANTHROPIC_API_KEY=<key>` |
| openai | `OPENAI_API_KEY` | `export OPENAI_API_KEY=<key>` |
| qwen | `QWEN_API_KEY` | `export QWEN_API_KEY=<key>` |
| xai | `XAI_API_KEY` | `export XAI_API_KEY=<key>` |
| perplexity | `PERPLEXITY_API_KEY` | `export PERPLEXITY_API_KEY=<key>` |
| ai21 | `AI21_API_KEY` | `export AI21_API_KEY=<key>` |
| together | `TOGETHER_API_KEY` | `export TOGETHER_API_KEY=<key>` |

---

*This report is auto-generated by HelixAgent Provider Validation Test*
