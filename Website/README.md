# HelixAgent Website

Documentation and content for the HelixAgent project website.

## Structure

- `user-manuals/` — Step-by-step user guides (47 manuals)
- `video-courses/` — Video course content and scripts (84+ courses)
- `scripts/` — Build and validation scripts
- `styles/` — Website stylesheet assets
- `public/` — Static public assets
- `build.sh` — Website build script

## Project Stats

- **LLM Providers:** 43+ supported (Claude, Chutes, DeepSeek, Gemini, Mistral, OpenRouter,
  Qwen, ZAI, Zen, Cerebras, Ollama, AI21, Anthropic, Cohere, Fireworks, GitHub Models,
  Groq, HuggingFace, OpenAI, Perplexity, Replicate, Together, Venice, xAI, Junie,
  Cloudflare, Codestral, Hyperbolic, Kilo, Kimi, KimiCode, Modal, Nia, NLPCloud, Novita,
  Nvidia, PublicAI, SambaNova, Sarvam, SiliconFlow, Upstage, VulaVula, Zhipu)
- **Extracted Modules:** 41 independent Go modules with their own tests, docs, and challenges
- **Test Coverage:** 800+ unit test files, 47 fuzz functions, 23+ stress test suites
- **Challenge Scripts:** 60+ validation scripts in `challenges/scripts/`
- **CLI Agents:** 48 supported agents with auto-generated configurations

## User Manuals

Comprehensive guides covering:
- Getting started and API reference
- Provider configuration and deployment
- AI debate system and protocols
- Security, performance, and plugin development
- BigData, gRPC, memory, and code formatters
- Automated security scanning and performance monitoring
- Concurrency patterns and testing strategies
- Challenge development and custom provider guides
- Observability, backup, and disaster recovery
- Enterprise architecture and compliance
- Agentic workflows, LLMOps, planning algorithms
- Module-specific guides (DocProcessor, HelixQA, LLMOrchestrator, VisionEngine, etc.)

Current manuals: 01 through 47 (`user-manuals/01-getting-started.md` through
`user-manuals/47-stress-testing-guide.md`).

## Video Courses

Structured learning paths covering:
- Fundamentals, AI debate, deployment, and protocols
- Advanced providers, plugin development, and MCP mastery
- Security scanning, performance tuning, and stress testing
- Memory management, cloud providers, and enterprise deployment
- HelixMemory, HelixSpecifier, and module deep dives
- Goroutine safety, router completeness, and lazy loading patterns
- Agentic workflows, LLMOps, planning algorithms, and more
- Safety and concurrency patterns, performance baselines
- Security scanning and vulnerability management
- Monitoring, dashboards, and alerting

Course files use two naming conventions:
- `course-NN-<topic>.md` — Primary course series (01-18, 66-84)
- `video-course-NN-<topic>.md` — Extended video series (53-65)
- `courses-NN-MM/` — Batch course subdirectories

### Primary Course Series (course-NN)

| # | Title |
|---|-------|
| 01 | Fundamentals |
| 02 | AI Debate |
| 03 | Deployment |
| 04 | Custom Integration |
| 05 | Protocols |
| 06 | Testing |
| 07 | Advanced Providers |
| 08 | Plugin Development |
| 09 | Production Operations |
| 10 | Security Best Practices |
| 11 | MCP Mastery |
| 12 | Advanced Workflows |
| 13 | Enterprise Deployment |
| 14 | Certification Prep |
| 15 | BigData Analytics |
| 16 | Memory Management |
| 17 | Cloud Providers |
| 18 | Security Scanning |
| 66 | Agentic Workflows |
| 67 | LLMOps Experimentation (overview) |
| 68 | Planning Algorithms (overview) |
| 69 | Concurrency Safety (overview) |
| 70 | DocProcessor Module |
| 71 | HelixQA Module |
| 72 | LLMOrchestrator Module |
| 73 | VisionEngine Module |
| 74 | Security Scanning (deep dive) |
| 75 | Performance Tuning |
| 76 | Agentic Ensemble |
| 77 | Agentic Workflows Deep Dive |
| 78 | LLMOps Experimentation (deep dive) |
| 79 | Planning Algorithms Masterclass |
| 80 | Benchmarking & Provider Evaluation |
| 81 | Safety & Concurrency Patterns |
| 82 | Performance Tuning & Baselines |
| 83 | Security Scanning & Vulnerability Management |
| 84 | Monitoring, Dashboards & Alerting |

### Extended Video Series (video-course-NN)

| # | Title |
|---|-------|
| 53 | HelixMemory Deep Dive |
| 54 | HelixSpecifier Workflow |
| 55 | Security Scanning Pipeline |
| 56 | Performance Optimization |
| 57 | Stress Testing Guide |
| 58 | Chaos Engineering |
| 59 | Monitoring & Observability |
| 60 | Enterprise Deployment |
| 61 | Goroutine Safety |
| 62 | Router Completeness |
| 63 | Automated Security Scanning |
| 64 | Fuzz Testing Mastery |
| 65 | Lazy Loading Patterns |

## Contributing

- Follow standard Markdown formatting
- Preserve ALL existing content when updating files
- Add new content at the end of existing files, never remove
- Number new manuals sequentially (next: 48 — `48-<topic>.md`)
- Number new video courses sequentially (next: 85)
- Keep user manuals practical with real examples and curl commands
- Cross-reference related manuals and courses where relevant
