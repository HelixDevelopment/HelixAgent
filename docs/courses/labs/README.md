# HelixAgent Course Labs

Hands-on lab exercises for the HelixAgent training course.

## Lab Overview

| Lab | Module | Duration | Difficulty | NEW |
|-----|--------|----------|------------|-----|
| [Lab 1: Getting Started](LAB_01_GETTING_STARTED.md) | 1-2 | 45 min | Beginner | |
| [Lab 2: Provider Setup](LAB_02_PROVIDER_SETUP.md) | 4 | 60 min | Intermediate | |
| [Lab 3: AI Debate](LAB_03_AI_DEBATE.md) | 6 | 75 min | Intermediate | |
| [Lab 4: MCP Integration](LAB_04_MCP_INTEGRATION.md) | 8 | 60 min | Intermediate | |
| [Lab 5: Production Deployment](LAB_05_PRODUCTION_DEPLOYMENT.md) | 10-11 | 120 min | Advanced | |
| [Lab 6: Challenge Scripts](LAB_06_CHALLENGE_SCRIPTS.md) | 12 | 90 min | Intermediate | NEW |
| [Lab 7: MCP Tool Search](LAB_07_MCP_TOOL_SEARCH.md) | 13 | 60 min | Intermediate | NEW |
| [Lab 8: Multi-Pass Validation](LAB_08_MULTIPASS_VALIDATION.md) | 14 | 75 min | Advanced | |
| [Lab 9: Agentic Workflows](LAB_09_AGENTIC_WORKFLOWS.md) | S7.1 | 45 min | Advanced | |
| [Lab 10: LLMOps Evaluation](LAB_10_LLMOPS_EVALUATION.md) | S7.1 | 30 min | Advanced | |
| [Lab 11: SelfImprove RLHF](LAB_11_SELFIMPROVE_RLHF.md) | S7.1 | 30 min | Advanced | |
| [Lab 12: Planning Algorithms](LAB_12_PLANNING_ALGORITHMS.md) | S7.2 | 30 min | Advanced | |
| [Lab 13: Benchmark Evaluation](LAB_13_BENCHMARK_EVALUATION.md) | S7.2 | 30 min | Advanced | |
| [Lab 14: Agentic Workflows (Ext)](LAB_14_agentic_workflows.md) | S7.1 | 30 min | Advanced | |
| [Lab 15: LLMOps Experiments](LAB_15_llmops_experiments.md) | S7.1 | 30 min | Advanced | |
| [Lab 16: Stress Testing](LAB_16_stress_testing.md) | 11 | 30 min | Advanced | |
| [Lab 17: Security Scanning](LAB_17_security_scanning.md) | 10 | 45 min | Advanced | |
| [Lab 18: HelixLLM and AgenticEnsemble](LAB_18_helixllm_agentic_ensemble.md) | 15 | 45 min | Advanced | NEW |
| [Lab 19: HTTP/3 and Brotli](LAB_19_http3_brotli.md) | 16 | 30 min | Intermediate | NEW |
| [Lab 20: Remote Containers](LAB_20_remote_containers.md) | 17 | 30 min | Advanced | NEW |
| [Lab 21: HelixMemory](LAB_21_helixmemory.md) | 18 | 30 min | Advanced | NEW |
| [Lab 22: HelixSpecifier](LAB_22_helixspecifier.md) | 19 | 30 min | Advanced | NEW |

## Prerequisites

Before starting the labs, ensure you have:

- [ ] Git installed
- [ ] Go 1.25+ installed (for source builds)
- [ ] Docker and Docker Compose (recommended)
- [ ] Text editor (VS Code recommended)
- [ ] Terminal access
- [ ] Internet connection
- [ ] At least one LLM API key

## Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-org/helix-agent.git
   cd helix-agent
   ```

2. **Set up environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your API keys
   ```

3. **Start the labs**:
   Begin with [Lab 1: Getting Started](LAB_01_GETTING_STARTED.md)

## Lab Completion Tracking

Track your progress through the labs:

- [ ] Lab 1: Getting Started
  - [ ] Repository cloned
  - [ ] Server running
  - [ ] Health check passing

- [ ] Lab 2: Provider Setup
  - [ ] Multiple providers configured
  - [ ] Provider health verified
  - [ ] Fallback chain working

- [ ] Lab 3: AI Debate
  - [ ] Created first debate
  - [ ] Tested different styles
  - [ ] Analyzed consensus results

- [ ] Lab 4: MCP Integration
  - [ ] MCP tools listed
  - [ ] Tool execution working
  - [ ] Resource access verified

- [ ] Lab 5: Production Deployment
  - [ ] Docker stack running
  - [ ] Monitoring configured
  - [ ] Security hardened

- [ ] Lab 6: Challenge Scripts (NEW)
  - [ ] RAGS challenge passed (100%)
  - [ ] MCPS challenge passed (100%)
  - [ ] SKILLS challenge passed (100%)
  - [ ] Understood strict validation

- [ ] Lab 7: MCP Tool Search (NEW)
  - [ ] Tool search working
  - [ ] Suggestions tested
  - [ ] Adapter search working
  - [ ] Discovery workflow created

- [ ] Lab 8: Multi-Pass Validation
  - [ ] 4-phase debate completed
  - [ ] Phase indicators observed
  - [ ] Confidence >0.8 achieved
  - [ ] Configuration tuned

- [ ] Lab 18: HelixLLM and AgenticEnsemble (NEW)
  - [ ] HelixLLM configured as provider
  - [ ] Reason mode tested
  - [ ] Execute mode tested
  - [ ] AgenticEnsemble challenge passed

- [ ] Lab 19: HTTP/3 and Brotli (NEW)
  - [ ] Brotli compression verified
  - [ ] Compression size comparison done
  - [ ] Brotli challenge passed

- [ ] Lab 20: Remote Containers (NEW)
  - [ ] Container orchestration flow observed
  - [ ] Service overrides tested
  - [ ] Health checks verified

- [ ] Lab 21: HelixMemory (NEW)
  - [ ] Memory store and recall tested
  - [ ] Cross-session context verified
  - [ ] Prometheus metrics checked
  - [ ] HelixMemory challenge passed

- [ ] Lab 22: HelixSpecifier (NEW)
  - [ ] Auto-activation triggered
  - [ ] Adaptive ceremony observed
  - [ ] Spec cache verified
  - [ ] HelixSpecifier challenge passed

## Lab Files

Each lab contains:
- **Objectives**: What you'll learn
- **Prerequisites**: What you need
- **Exercises**: Step-by-step tasks
- **Checkpoints**: Verification points
- **Troubleshooting**: Common issues and solutions
- **Challenge**: Optional advanced exercise

## Certification Requirements

| Level | Required Labs |
|-------|---------------|
| Level 1: Fundamentals | Lab 1 |
| Level 2: Provider Expert | Labs 1-3 |
| Level 3: Advanced | Labs 1-4 |
| Level 4: Master | Labs 1-5 |
| Level 5: Challenge Expert | Labs 1-8 |
| Level 6: AI/ML Systems Architect | Labs 1-8, 9-15 |
| Level 7: Platform Architect | All labs (1-22) |

### Level 5 Special Requirements
- 100% pass rate on RAGS challenge
- 100% pass rate on MCPS challenge
- 100% pass rate on SKILLS challenge
- MCP Tool Search integration demonstration
- Multi-pass validation debate with >0.8 confidence

### Level 7 Special Requirements
- HelixLLM provider configured and AgenticEnsemble dual-mode verified
- HTTP/3 (QUIC) transport and Brotli compression verified
- Remote container distribution configured
- HelixMemory integrated with 2+ backends
- HelixSpecifier 7-phase workflow executed with DebateFunc

## Support

If you encounter issues during labs:

1. Check the troubleshooting section in each lab
2. Review the [Quick Reference](../reference/QUICK_REFERENCE.md)
3. Ask in the course discussion forum
4. Open a GitHub issue

## Contributing

To improve the labs:

1. Fork the repository
2. Make your changes
3. Test all exercises
4. Submit a pull request

---

*Labs Version: 3.0.0*
*Last Updated: April 2026*
*New Labs: 18 (HelixLLM/AgenticEnsemble), 19 (HTTP/3/Brotli), 20 (Remote Containers), 21 (HelixMemory), 22 (HelixSpecifier)*
