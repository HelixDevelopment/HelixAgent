# CLI Agents Porting - Master Documentation

**Project:** HelixAgent CLI Agents Fusion  
**Scope:** Port features from 47 CLI agents into HelixAgent  
**Last Updated:** 2026-04-03  
**Status:** Planning Complete  

---

## Overview

This directory contains the complete planning documentation for fusing 47 CLI agent features into HelixAgent's modular architecture. The porting strategy uses a **5-pass approach** to ensure comprehensive analysis and optimal integration.

### Key Statistics

| Metric | Value |
|--------|-------|
| CLI Agents Analyzed | 47 |
| Total Features | 156 |
| Critical Features to Port | 20 |
| Implementation Phases | 5 |
| Estimated Duration | 24 weeks |

---

## Quick Navigation

### 📋 Planning Documents (5-Pass Approach)

1. **[Pass 1: Analysis](passes/pass1_analysis.md)** - Feature taxonomy and gap analysis
2. **[Pass 2: Architecture](passes/pass2_architecture.md)** - Component organization and fusion layer
3. **[Pass 3: Integration Strategy](passes/pass3_integration_strategy.md)** - Port vs adapt vs implement
4. **[Pass 4: Optimization](passes/pass_4_optimization.md)** - Performance and resource planning
5. **[Pass 5: Final Implementation Plan](passes/pass_5_finalization.md)** - 24-week roadmap with exact code

### 🎯 Implementation Documents

- **[Implementation Phases](IMPLEMENTATION_PHASES.md)** - Quick-start implementation roadmap
- **[Feature Registry](components/feature_registry.md)** - Complete feature catalog
- **[Architectural Fusion](architectural_fusion.md)** - Fusion layer architecture blueprint

### 📊 Supporting Materials

- `diagrams/` - Architecture diagrams (created as needed)
- `implementation/` - Code templates and examples
- `sql_schemas/` - Database schema definitions

---

## Five-Pass Methodology

Our approach ensures no feature is missed and optimal integration strategies are identified:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        FIVE-PASS PORTING PROCESS                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐│
│  │ Pass 1   │───▶│ Pass 2   │───▶│ Pass 3   │───▶│ Pass 4   │───▶│ Pass 5 ││
│  │ ANALYSIS │    │ ARCHITECT│    │ STRATEGY │    │ OPTIMIZE │    │FINALIZE││
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘    └────────┘│
│       │              │              │              │              │        │
│       ▼              ▼              ▼              ▼              ▼        │
│   Feature        Component       Integration   Performance    Final      │
│   Taxonomy       Organization    Strategy      Tuning         Roadmap    │
│                                                                             │
│   • 156 features • Fusion layer  • Port vs      • Caching     • 24-week   │
│   • 12 domains   • Directory     • adapt vs     • Pooling     • timeline  │
│   • Gaps         • structure     • implement    • Streaming   • Milestones│
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Critical Features (20 Total)

### Priority 1: Foundation (Essential)

| # | Feature | Source | Target | Status |
|---|---------|--------|--------|--------|
| 1 | Instance Management | HelixAgent | `internal/clis/` | Planned |
| 2 | Ensemble Coordination | HelixAgent | `internal/ensemble/` | Planned |
| 3 | SQL Schema | HelixAgent | `sql/001_cli_agents_fusion.sql` | Planned |
| 4 | Synchronization | HelixAgent | `internal/ensemble/synchronization/` | Planned |
| 5 | Event Bus | HelixAgent | `internal/ensemble/event_bus.go` | Planned |

### Priority 2: Code Understanding

| # | Feature | Source | Target | Status |
|---|---------|--------|--------|--------|
| 6 | Repo Map | Aider | `internal/clis/aider/repo_map.go` | Planned |
| 7 | Diff Format | Aider | `internal/clis/aider/diff_format.go` | Planned |
| 8 | Git Integration | Aider | `internal/clis/aider/git_ops.go` | Planned |
| 9 | Terminal UI | Claude Code | `internal/clis/claude_code/terminal_ui.go` | Planned |
| 10 | Tool Use | Claude Code | `internal/clis/claude_code/tool_use.go` | Planned |

### Priority 3: Execution & Security

| # | Feature | Source | Target | Status |
|---|---------|--------|--------|--------|
| 11 | Code Interpreter | Codex | `internal/clis/codex/interpreter.go` | Planned |
| 12 | Browser Automation | Cline | `internal/clis/cline/browser.go` | Planned |
| 13 | Sandbox | OpenHands | `internal/clis/openhands/sandbox.go` | Planned |
| 14 | Project Memory | Kiro | `internal/clis/kiro/memory.go` | Planned |
| 15 | LSP Client | Continue | `internal/clis/continue/lsp.go` | Planned |

### Priority 4: Output & Performance

| # | Feature | Source | Target | Status |
|---|---------|--------|--------|--------|
| 16 | Streaming Pipeline | HelixAgent | `internal/output/pipeline.go` | Planned |
| 17 | Instance Pooling | HelixAgent | `internal/ensemble/pool.go` | Planned |
| 18 | Semantic Caching | HelixAgent | `internal/cache/semantic.go` | Planned |
| 19 | Background Workers | HelixAgent | `internal/ensemble/background/` | Planned |
| 20 | Load Balancing | HelixAgent | `internal/ensemble/load_balancer.go` | Planned |

---

## Architecture

### Component Organization

```
helix_agent/
├── internal/
│   ├── clis/                    # CLI agent integrations
│   │   ├── types.go             # Shared types
│   │   ├── instance_manager.go  # Instance lifecycle
│   │   ├── aider/               # Aider components
│   │   ├── claude_code/         # Claude Code components
│   │   ├── codex/               # Codex components
│   │   ├── cline/               # Cline components
│   │   ├── openhands/           # OpenHands components
│   │   ├── kiro/                # Kiro components
│   │   └── continue/            # Continue components
│   ├── ensemble/                # Ensemble system
│   │   ├── multi_instance/      # Multi-instance coordination
│   │   ├── background/          # Background workers
│   │   └── synchronization/     # Distributed sync
│   └── output/                  # Output system
│       ├── pipeline.go          # Processing pipeline
│       ├── formatters/          # Content formatters
│       └── renderers/           # Output renderers
└── sql/
    └── 001_cli_agents_fusion.sql # Database schema
```

### Fusion Layer

The **Fusion Layer** provides the integration abstraction between HelixAgent and CLI agent components:

```
┌─────────────────────────────────────────────────────────────────┐
│                         FUSION LAYER                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   Adapter   │  │   State     │  │   Event     │             │
│  │   Pattern   │  │   Bridge    │  │   Bus       │             │
│  │             │  │             │  │             │             │
│  │ • Normalize │  │ • Synchronize│  │ • Pub/Sub   │             │
│  │ • Translate │  │ • Persist   │  │ • Broadcast │             │
│  │ • Route     │  │ • Cache     │  │ • Filter    │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Timeline

```
Week:  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24
       ├──── Phase 1: Foundation ────┤
                                   ├──── Phase 2: Ensemble ────┤
                                                               ├──────── Phase 3: CLI ───────┤
                                                                                             ├─ 4 ─┤
                                                                                                   ├5┤

Phase 1 (Weeks 1-4): Foundation
• SQL Schema & Core Types
• Instance Management
• Synchronization Primitives
• Event Bus

Phase 2 (Weeks 5-8): Ensemble Extension
• Multi-Instance Coordinator
• Worker Pool
• Load Balancer
• Health Monitor

Phase 3 (Weeks 9-16): CLI Agent Integration
• Aider components (Repo Map, Diff Format, Git)
• Claude Code components (Terminal UI, Tool Use)
• Codex components (Interpreter, Reasoning)
• Cline components (Browser, Computer Use)
• OpenHands components (Sandbox, Security)
• Kiro components (Memory)
• Continue components (LSP)

Phase 4 (Weeks 17-20): Output System
• Formatting Pipeline
• Terminal Enhancements
• Streaming Optimization

Phase 5 (Weeks 21-24): Testing & Deployment
• Comprehensive Testing
• Performance Tuning
• Documentation
• Production Deployment
```

---

## Performance Targets

| Metric | Current | Target | Improvement |
|--------|---------|--------|-------------|
| Instance Startup | 5s | 100ms | 98% |
| Request Latency (p99) | 2s | 500ms | 75% |
| Memory per Instance | 500MB | 200MB | 60% |
| Cache Hit Rate | 10% | 50% | 5x |
| Throughput | 100 req/s | 1000 req/s | 10x |

---

## Getting Started

### For Developers

1. **Read the 5-pass documents** - Understand the full scope
2. **Review [Implementation Phases](IMPLEMENTATION_PHASES.md)** - Get the roadmap
3. **Check out Phase 1 branch** - Start with foundation
4. **Run database migration** - `sql/001_cli_agents_fusion.sql`
5. **Implement core types** - Start with `internal/clis/types.go`

### For Project Managers

1. **Review [Pass 5](passes/pass_5_finalization.md)** - Full 24-week plan
2. **Check [Implementation Phases](IMPLEMENTATION_PHASES.md)** - Quick reference
3. **Track against milestones** - Weekly check-ins
4. **Monitor metrics** - Performance targets

### For Architects

1. **Study [Pass 2](passes/pass2_architecture.md)** - Component organization
2. **Review [Architectural Fusion](architectural_fusion.md)** - Fusion layer design
3. **Examine SQL schemas** - Data model
4. **Validate approach** - Ensure alignment with HelixAgent goals

---

## Contributing

When adding to this documentation:

1. **Follow the 5-pass structure** for new analysis
2. **Update the feature registry** when features change
3. **Keep code examples current** with implementation
4. **Update this README** when adding new documents

---

## Related Documentation

- [HelixAgent AGENTS.md](../../../../AGENTS.md) - Project overview
- [HelixAgent README](../../../../README.md) - Main project documentation
- [Research Master Index](../MASTER_INDEX.md) - All research documents

---

## Document History

| Date | Change | Author |
|------|--------|--------|
| 2026-04-03 | Created master index | AI Research Team |
| 2026-04-03 | Completed Pass 1-5 | AI Research Team |
| 2026-04-03 | Added implementation roadmap | AI Research Team |

---

**Document Owner:** HelixAgent Core Team  
**Review Cycle:** Weekly during implementation  
**Next Review:** Upon Phase 1 completion (Week 4)
