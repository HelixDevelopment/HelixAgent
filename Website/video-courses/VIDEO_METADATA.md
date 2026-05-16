# Video Course Metadata

Centralized metadata for all HelixAgent video courses. Use this file to track course details, dependencies, and production status.

---

## Course Registry

| ID | Title | Duration | Level | Status |
|----|-------|----------|-------|--------|
| 01 | HelixAgent Fundamentals | 1h | Beginner | Published |
| 02 | AI Debate System Mastery | 1.5h | Intermediate | Published |
| 03 | Production Deployment | 1.25h | Advanced | Published |
| 04 | Custom Integration | 0.75h | Developer | Published |
| 05 | Protocol Integration | 1h | Intermediate | Published |
| 06 | Testing Strategies | 3.5h | Intermediate | Published |
| 07 | Advanced Provider Configuration | 4h | Advanced | Published |
| 08 | Plugin Development Deep Dive | 4.5h | Advanced | Published |
| 09 | Production Operations | 5h | Advanced | Published |
| 10 | Security Best Practices | 4.5h | Advanced | Published |
| 11 | Challenge Validation System | 3h | Advanced | Published |
| 12 | Hybrid RAG System | 3.5h | Advanced | Published |
| 13 | Multi-Pass Validation | 2.5h | Advanced | Published |
| 14 | MCP Mastery | 3.5h | Intermediate-Advanced | Published |
| 15 | BigData Analytics | -- | Advanced | Published |
| 16 | Memory Management | -- | Advanced | Published |
| 17 | Cloud Providers | -- | Advanced | Published |
| 18 | Security Scanning | -- | Advanced | Published |
| 53 | HelixMemory Deep Dive | -- | Advanced | Published |
| 54 | HelixSpecifier Workflow | -- | Advanced | Published |
| 55 | Security Scanning Pipeline | -- | Advanced | Published |
| 56 | Performance Optimization | -- | Advanced | Published |
| 57 | Stress Testing Guide | -- | Advanced | Published |
| 58 | Chaos Engineering | -- | Advanced | Published |
| 59 | Monitoring & Observability | -- | Advanced | Published |
| 60 | Enterprise Deployment | -- | Advanced | Published |
| 61 | Goroutine Safety & Lifecycle | 3h | Advanced | Published |
| 62 | Router Completeness | -- | Advanced | Published |
| 63 | Automated Security Scanning | -- | Advanced | Published |
| 64 | Fuzz Testing Mastery | -- | Advanced | Published |
| 65 | Lazy Loading Patterns | 2.5h | Intermediate-Advanced | Published |
| 66 | Agentic Workflows Deep Dive | 3h | Intermediate-Advanced | Published |
| 67 | LLMOps & A/B Experimentation | 2.5h | Intermediate-Advanced | Published |
| 68 | AI Planning Algorithms | 3h | Advanced | Published |
| 69 | Concurrency Safety Patterns | 3h | Advanced | Published |
| 70 | DocProcessor Deep Dive | 2.5h | Intermediate-Advanced | Published |
| 71 | HelixQA Orchestration Framework | 3h | Advanced | Published |
| 72 | LLMOrchestrator Mastery | 2.5h | Advanced | Published |
| 73 | VisionEngine for UI Analysis | 2.5h | Advanced | Published |
| 74 | Security Scanning Pipeline | 3h | Advanced | Published |
| 75 | Performance Tuning and Profiling | 3h | Advanced | Published |

---

## New Courses (66-75)

### Course 66: Agentic Workflows Deep Dive

- **File:** `course-66-agentic-workflows.md`
- **Duration:** 3 hours
- **Level:** Intermediate to Advanced
- **Prerequisites:** Course 01, Course 12, Course 15
- **Modules:** 6 (What Are Agentic Workflows, Node Types and Graph Design, The REST API, Configuration and Checkpointing, Integration with Debate System, Hands-On Labs)
- **Key Topics:** Graph-based workflow orchestration, 6 node types (agent, tool, condition, parallel, human, subgraph), REST API (POST/GET /v1/agentic/workflows), checkpointing, self-correction, debate integration
- **Source Modules:** `Agentic/` (`digital.vasic.agentic`), `internal/handlers/agentic_handler.go`
- **Assessment:** Quiz (10 questions) + practical workflow build

### Course 67: LLMOps & A/B Experimentation

- **File:** `course-67-llmops-experimentation.md`
- **Duration:** 2.5 hours
- **Level:** Intermediate to Advanced
- **Prerequisites:** Course 01, Course 07
- **Modules:** 6 (What Is LLMOps, A/B Experiments, Continuous Evaluation, Prompt Versioning, API Endpoints Walkthrough, Hands-On Labs)
- **Key Topics:** A/B experiments with variants and traffic split, continuous evaluation pipelines, prompt versioning and rendering, REST API (POST/GET /v1/llmops/experiments, /v1/llmops/evaluate, /v1/llmops/prompts)
- **Source Modules:** `LLMOps/` (`digital.vasic.llmops`), `internal/handlers/llmops_handler.go`
- **Assessment:** Quiz (10 questions) + practical LLMOps pipeline setup

### Course 68: AI Planning Algorithms

- **File:** `course-68-planning-algorithms.md`
- **Duration:** 3 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 66
- **Modules:** 5 (HiPlan, MCTS, Tree of Thoughts, When to Use Each Algorithm, Hands-On Labs)
- **Key Topics:** Hierarchical planning (milestones and steps), Monte Carlo Tree Search (UCB1, 4-phase loop), Tree of Thoughts (BFS/DFS/beam), REST API (POST /v1/planning/hiplan, /v1/planning/mcts, /v1/planning/tot), algorithm selection guide
- **Source Modules:** `Planning/` (`digital.vasic.planning`), `internal/handlers/planning_handler.go`
- **Assessment:** Quiz (10 questions) + practical multi-algorithm design exercise

### Course 69: Concurrency Safety Patterns in Go

- **File:** `course-69-concurrency-safety.md`
- **Duration:** 3 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 61, Course 65
- **Modules:** 7 (sync.Once for Idempotent Shutdown, atomic.Bool for Lock-Free Flags, WaitGroup Goroutine Lifecycle, Panic Recovery in Goroutines, Race Detector and Testing, Putting It All Together, Hands-On Labs)
- **Key Topics:** sync.Once idempotent shutdown, atomic.Bool lock-free flags, WaitGroup lifecycle pattern, panic recovery, race detector, production concurrency checklist
- **Source Modules:** Go standard library (`sync`, `sync/atomic`, `runtime`), HelixAgent internal patterns
- **Assessment:** Quiz (10 questions) + practical concurrent service refactoring

### Course 70: DocProcessor Deep Dive

- **File:** `course-70-docprocessor.md`
- **Duration:** 2.5 hours
- **Level:** Intermediate to Advanced
- **Prerequisites:** Course 01, Course 06
- **Modules:** 6 (Architecture Overview, Feature Map Extraction, Coverage Tracking, DocGraph Visualization, LLM Agent Integration, Hands-On Lab)
- **Key Topics:** Documentation loading pipeline, FeatureMap building (heuristic + LLM), CoverageTracker (thread-safe per-platform), DocGraph with JSON/Mermaid export, LLMAgent interface injection
- **Source Modules:** `DocProcessor/` (`digital.vasic.docprocessor`)
- **Assessment:** Quiz (10 questions) + practical pipeline build

### Course 71: HelixQA Orchestration Framework

- **File:** `course-71-helixqa.md`
- **Duration:** 3 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 06, Course 70
- **Modules:** 6 (QA Orchestrator Setup, Test Bank YAML Format, Crash/ANR Detection, Evidence Collection Pipeline, Ticket Generation, Session Management)
- **Key Topics:** SessionCoordinator lifecycle, YAML test banks with platform/priority filtering, real-time crash/ANR detection (Android/Web/Desktop), evidence artifacts (screenshots/video/logs), Markdown ticket generation, curiosity-driven exploration
- **Source Modules:** `helix_qa/` (`digital.vasic.helixqa`)
- **Assessment:** Quiz (10 questions) + practical QA session build

### Course 72: LLMOrchestrator Mastery

- **File:** `course-72-llmorchestrator.md`
- **Duration:** 2.5 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 07, Course 69
- **Modules:** 6 (Agent Pool Architecture, Hybrid Protocol, CLI Adapters, Circuit Breaker Integration, Health Monitoring, Security Hardening)
- **Key Topics:** AgentPool (sync.Mutex + sync.Cond blocking acquire), PipeTransport (JSON-lines) + FileTransport (inbox/outbox/shared), BaseAdapter pattern with 5 CLI adapters, circuit breaker (3 failures = 60s open), health monitoring, path traversal protection
- **Source Modules:** `LLMOrchestrator/` (`digital.vasic.llmorchestrator`)
- **Assessment:** Quiz (10 questions) + practical agent pool deployment

### Course 73: VisionEngine for UI Analysis

- **File:** `course-73-visionengine.md`
- **Duration:** 2.5 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 70, Course 72
- **Modules:** 6 (Analyzer Interface, NavigationGraph + BFS, LLM Vision Providers, OpenCV Integration, FallbackProvider Pattern, Hands-On Lab)
- **Key Topics:** Analyzer interface (6 methods: AnalyzeScreen, CompareScreens, DetectElements, DetectText, IdentifyScreen, DetectIssues), NavigationGraph with BFS pathfinding, LLM vision providers, OpenCV build tags, FallbackProvider multi-provider resilience
- **Source Modules:** `VisionEngine/` (`digital.vasic.visionengine`)
- **Assessment:** Quiz (10 questions) + practical graph + fallback build

### Course 74: Security Scanning Pipeline

- **File:** `course-74-security-scanning.md`
- **Duration:** 3 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 10
- **Modules:** 6 (Scanner Overview (7 tools), Docker Compose Setup, Running Scans, Interpreting Reports, Fixing Findings, Continuous Scanning Workflow)
- **Key Topics:** 7 tools (gosec, Trivy, Semgrep, Snyk, SonarQube, staticcheck, go vet), Docker Compose infrastructure for Snyk and SonarQube, Makefile scan targets, severity classification, fix patterns, continuous workflow integration
- **Source Modules:** `docker/security/snyk/`, `docker/security/sonarqube/`, Makefile targets
- **Assessment:** Quiz (10 questions) + practical scanning pipeline operation

### Course 75: Performance Tuning and Profiling

- **File:** `course-75-performance-tuning.md`
- **Duration:** 3 hours
- **Level:** Advanced
- **Prerequisites:** Course 01, Course 65, Course 69
- **Modules:** 6 (Lazy Loading Patterns, Benchmark Methodology, Monitoring-Driven Optimization, Semaphore & Backpressure, HTTP Pool Tuning, pprof Profiling)
- **Key Topics:** sync.Once lazy initialization, Go benchmarks with benchstat, Prometheus metrics for bottleneck identification, semaphore concurrency limiting, HTTP/3 connection pool configuration, CPU and heap profiling with pprof
- **Source Modules:** `internal/services/debate_performance_optimizer.go`, `internal/adapters/http/`, pprof endpoints
- **Assessment:** Quiz (10 questions) + practical optimization exercise with benchstat comparison

---

## Prerequisite Graph (Courses 61-75)

```
Course 01 (Fundamentals)
  |
  +-- Course 06 (Testing Strategies)
  |     +-- Course 70 (DocProcessor Deep Dive)
  |           +-- Course 71 (HelixQA Orchestration)
  |
  +-- Course 10 (Security Best Practices)
  |     +-- Course 74 (Security Scanning Pipeline)
  |
  +-- Course 61 (Goroutine Safety)
  |     +-- Course 65 (Lazy Loading Patterns)
  |           +-- Course 69 (Concurrency Safety Patterns)
  |                 +-- Course 75 (Performance Tuning and Profiling)
  |
  +-- Course 07 (Advanced Providers)
  |     +-- Course 67 (LLMOps & A/B Experimentation)
  |     +-- Course 69 + Course 07
  |           +-- Course 72 (LLMOrchestrator Mastery)
  |
  +-- Course 70 + Course 72
  |     +-- Course 73 (VisionEngine for UI Analysis)
  |
  +-- Course 12 (Advanced Workflows)
        +-- Course 15 (Advanced Agentic Workflows)
              +-- Course 66 (Agentic Workflows Deep Dive)
                    +-- Course 68 (AI Planning Algorithms)
```

---

## Learning Paths (Updated)

### AI/ML Engineer Path
1. Course 01: Fundamentals
2. Course 02: AI Debate System
3. Course 66: Agentic Workflows Deep Dive
4. Course 67: LLMOps & A/B Experimentation
5. Course 68: AI Planning Algorithms
6. Course 72: LLMOrchestrator Mastery
7. Course 73: VisionEngine for UI Analysis

### Go Developer Path
1. Course 01: Fundamentals
2. Course 61: Goroutine Safety
3. Course 65: Lazy Loading Patterns
4. Course 69: Concurrency Safety Patterns
5. Course 06: Testing Strategies
6. Course 75: Performance Tuning and Profiling

### QA Automation Path
1. Course 01: Fundamentals
2. Course 06: Testing Strategies
3. Course 70: DocProcessor Deep Dive
4. Course 71: HelixQA Orchestration Framework
5. Course 72: LLMOrchestrator Mastery
6. Course 73: VisionEngine for UI Analysis

### Security Engineer Path
1. Course 01: Fundamentals
2. Course 10: Security Best Practices
3. Course 74: Security Scanning Pipeline
4. Course 75: Performance Tuning and Profiling

### Full Stack AI Path
1. Course 01: Fundamentals
2. Course 02: AI Debate System
3. Course 61: Goroutine Safety
4. Course 65: Lazy Loading Patterns
5. Course 66: Agentic Workflows Deep Dive
6. Course 67: LLMOps & A/B Experimentation
7. Course 68: AI Planning Algorithms
8. Course 69: Concurrency Safety Patterns
9. Course 70: DocProcessor Deep Dive
10. Course 71: HelixQA Orchestration Framework
11. Course 72: LLMOrchestrator Mastery
12. Course 73: VisionEngine for UI Analysis
13. Course 74: Security Scanning Pipeline
14. Course 75: Performance Tuning and Profiling

---

## Version History

| Date | Change |
|------|--------|
| 2026-03-25 | Added courses 70-75: DocProcessor, HelixQA, LLMOrchestrator, VisionEngine, Security Scanning, Performance Tuning |
| 2026-03-23 | Added courses 66-69: Agentic Workflows, LLMOps, Planning Algorithms, Concurrency Safety |
| 2026-03-23 | Created VIDEO_METADATA.md with full course registry |
