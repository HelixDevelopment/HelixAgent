# HelixAgent Video Course Materials

This directory contains comprehensive video course materials for HelixAgent training.

## Directory Structure

```
docs/courses/
|-- COURSE_OUTLINE.md          # Complete 19-module course outline
|-- INSTRUCTOR_GUIDE.md        # Guide for course instructors
|-- README.md                  # This file
|-- slides/                    # Presentation slides for each module
|   |-- MODULE_01_INTRODUCTION.md
|   |-- MODULE_02_INSTALLATION.md
|   |-- MODULE_03_CONFIGURATION.md
|   |-- MODULE_04_PROVIDERS.md
|   |-- MODULE_05_ENSEMBLE.md
|   |-- MODULE_06_AI_DEBATE.md
|   |-- MODULE_07_PLUGINS.md
|   |-- MODULE_08_PROTOCOLS.md
|   |-- MODULE_09_OPTIMIZATION.md
|   |-- MODULE_10_SECURITY.md
|   |-- MODULE_11_TESTING_CICD.md
|   |-- MODULE_S71_AGENTIC.md
|   |-- MODULE_S71_LLMOPS.md
|   |-- MODULE_S71_SELFIMPROVE.md
|   |-- MODULE_S72_PLANNING.md
|   |-- MODULE_S72_BENCHMARK.md
|   |-- MODULE_15_HELIXLLM_AGENTIC_ENSEMBLE.md  # NEW
|   |-- MODULE_16_HTTP3_BROTLI.md                # NEW
|   |-- MODULE_17_REMOTE_CONTAINERS.md           # NEW
|   |-- MODULE_18_HELIXMEMORY.md                 # NEW
|   |-- MODULE_19_HELIXSPECIFIER.md              # NEW
|-- labs/                      # Hands-on lab exercises (22 labs)
|   |-- README.md
|   |-- LAB_01 through LAB_17
|   |-- LAB_18_helixllm_agentic_ensemble.md      # NEW
|   |-- LAB_19_http3_brotli.md                   # NEW
|   |-- LAB_20_remote_containers.md              # NEW
|   |-- LAB_21_helixmemory.md                    # NEW
|   |-- LAB_22_helixspecifier.md                 # NEW
|-- reference/                 # Quick reference materials
|   |-- QUICK_REFERENCE.md
|-- assessments/               # Quizzes and certifications
    |-- QUIZ_MODULE_1_3.md
    |-- QUIZ_MODULE_4_6.md
    |-- QUIZ_MODULE_7_9.md
    |-- QUIZ_MODULE_10_11.md
    |-- QUIZ_MODULE_12_14.md
    |-- QUIZ_MODULE_S71_S72.md
```

## Course Overview

**Title**: Mastering HelixAgent: Multi-Provider AI Orchestration
**Duration**: 20+ hours across 19 comprehensive modules (plus S7.1/S7.2)
**Target Audience**: Developers, DevOps engineers, AI engineers, technical decision-makers
**Skill Level**: Beginner to Advanced

### Key Highlights (v5.0)
- **43 LLM Providers**: Dynamic selection via LLMsVerifier verification scores
- **AgenticEnsemble**: Dual-mode unified LLM (Reason + Execute)
- **HelixLLM Provider**: Local AI ensemble with RAG capabilities
- **HTTP/3 (QUIC)**: Modern transport with Brotli compression
- **Remote Container Distribution**: Automatic deployment to remote hosts
- **HelixMemory**: Unified cognitive memory engine (Mem0 + Cognee + Letta + Graphiti)
- **HelixSpecifier**: Spec-driven development with 7-phase pipeline
- **25 LLM AI Debate**: 8-phase protocol with 6 voting methods and 4 topologies
- **48 CLI Agent Support**: Full config generation via HelixAgent binary
- **60+ MCP Servers**: Containerized, zero npx
- **Manual CI/CD**: Container-based builds with no automated pipelines

## Modules

| Module | Title | Duration | NEW |
|--------|-------|----------|-----|
| 1 | Introduction to HelixAgent | 45 min | |
| 2 | Installation and Setup | 60 min | |
| 3 | Configuration | 60 min | |
| 4 | LLM Provider Integration (43 providers) | 75 min | |
| 5 | Ensemble Strategies | 60 min | |
| 6 | AI Debate System | 90 min | |
| 7 | Plugin Development | 75 min | |
| 8 | MCP/LSP Integration | 60 min | |
| 9 | Optimization Features | 75 min | |
| 10 | Security Best Practices | 60 min | |
| 11 | Testing and Quality Validation | 75 min | |
| 12 | Challenge System and Validation | 90 min | |
| 13 | MCP Tool Search and Discovery | 60 min | |
| 14 | AI Debate System Advanced | 90 min | |
| S7.1 | Advanced AI/ML Part 1 (Agentic, LLMOps, SelfImprove) | 150 min | |
| S7.2 | Advanced AI/ML Part 2 (Planning, Benchmark) | 60 min | |
| 15 | HelixLLM and AgenticEnsemble | 90 min | NEW |
| 16 | HTTP/3 (QUIC) and Brotli Compression | 60 min | NEW |
| 17 | Remote Container Distribution | 60 min | NEW |
| 18 | HelixMemory Cognitive Engine | 75 min | NEW |
| 19 | HelixSpecifier Spec-Driven Development | 75 min | NEW |

## Using the Materials

### Presentation Slides

Each module has a corresponding slide deck in the `slides/` directory. The slides are written in Markdown format and include:

- Title slides
- Learning objectives
- Content slides with code examples
- Visual diagrams (described in text)
- Hands-on lab exercises
- Speaker notes

### Converting to Presentation Format

The Markdown slides can be converted to various presentation formats:

**Using Marp (recommended):**
```bash
# Install Marp CLI
npm install -g @marp-team/marp-cli

# Convert to HTML
marp slides/MODULE_01_INTRODUCTION.md -o MODULE_01.html

# Convert to PDF
marp slides/MODULE_01_INTRODUCTION.md -o MODULE_01.pdf

# Convert to PowerPoint
marp slides/MODULE_01_INTRODUCTION.md -o MODULE_01.pptx
```

**Using Pandoc:**
```bash
pandoc slides/MODULE_01_INTRODUCTION.md -o MODULE_01.pptx
```

### Recording Guidelines

See the video production setup guide at:
- `/docs/marketing/VIDEO_PRODUCTION_SETUP.md`

### Related Resources

- **API Documentation**: `/docs/api/`
- **Feature Documentation**: `/docs/features/`
- **Deployment Guides**: `/docs/deployment/`
- **Optimization Guides**: `/docs/optimization/`

## Certification Path

The course supports a 7-level certification path:

1. **Level 1: HelixAgent Fundamentals** - Modules 1-3
2. **Level 2: Provider Expert** - Modules 4-6
3. **Level 3: Advanced Practitioner** - Modules 7-9
4. **Level 4: Master Engineer** - Modules 10-11
5. **Level 5: Challenge Expert** - Modules 12-14
6. **Level 6: AI/ML Systems Architect** - Modules S7.1-S7.2
7. **Level 7: Platform Architect** - Modules 15-19

## Hands-On Labs

The course includes 22 comprehensive hands-on labs. See [labs/README.md](labs/README.md) for the full list.

Key new labs:

| Lab | Title | Duration | Difficulty | NEW |
|-----|-------|----------|------------|-----|
| 18 | [HelixLLM and AgenticEnsemble](labs/LAB_18_helixllm_agentic_ensemble.md) | 45 min | Advanced | NEW |
| 19 | [HTTP/3 and Brotli](labs/LAB_19_http3_brotli.md) | 30 min | Intermediate | NEW |
| 20 | [Remote Containers](labs/LAB_20_remote_containers.md) | 30 min | Advanced | NEW |
| 21 | [HelixMemory](labs/LAB_21_helixmemory.md) | 30 min | Advanced | NEW |
| 22 | [HelixSpecifier](labs/LAB_22_helixspecifier.md) | 30 min | Advanced | NEW |

## Reference Materials

- **[Quick Reference Card](reference/QUICK_REFERENCE.md)** - Essential commands and API endpoints
- **[Instructor Guide](INSTRUCTOR_GUIDE.md)** - Delivery guidelines for trainers

## Assessments

Certification assessments are provided for each level:

| Assessment | Modules | Questions | Passing | NEW |
|------------|---------|-----------|---------|-----|
| [Level 1 Quiz](assessments/QUIZ_MODULE_1_3.md) | 1-3 | 25 | 80% | |
| [Level 2 Quiz](assessments/QUIZ_MODULE_4_6.md) | 4-6 | 30 | 80% | |
| [Level 3 Quiz](assessments/QUIZ_MODULE_7_9.md) | 7-9 | 30 | 80% | |
| [Level 4 Quiz](assessments/QUIZ_MODULE_10_11.md) | 10-11 | 25 | 80% | |
| Level 5 Quiz | 12-14 | 35 | 80% | NEW |

### Level 7 Special Requirements
- HelixLLM provider and AgenticEnsemble dual-mode verified
- HTTP/3 and Brotli compression verified
- Container orchestration flow understood
- HelixMemory integrated with 2+ backends
- HelixSpecifier 7-phase workflow with DebateFunc

## Contributing

To update or improve course materials:

1. Edit the corresponding Markdown file
2. Test slide rendering with Marp
3. Update COURSE_OUTLINE.md if adding new content
4. Submit a pull request

## Version History

- **v5.0.0** (April 2026) - Added Modules 15-19, major updates throughout
  - Module 15: HelixLLM provider and AgenticEnsemble dual-mode
  - Module 16: HTTP/3 (QUIC) transport with Brotli compression
  - Module 17: Remote container distribution
  - Module 18: HelixMemory cognitive engine
  - Module 19: HelixSpecifier spec-driven development
  - Updated provider count from 7 to 43 across all materials
  - Updated CLI agent count from 20 to 48
  - Fixed Module 11 to remove GitHub Actions (per constitution: NO automated pipelines)
  - Updated Go version references from 1.23/1.24 to 1.25
  - Updated Gin version from 1.11.0 to 1.12.0
  - Added 6 voting methods, 4 topologies, 8-phase protocol to debate content
  - 5 new labs (Labs 18-22), 5 new slide modules
  - Level 7 certification path (Platform Architect)
  - Complete Quick Reference rewrite with all 43 providers
- **v3.0.0** (January 2026) - Added Modules 12-14 (Challenge System, MCP Tool Search, Advanced AI Debate)
- **v2.1.0** (January 2026) - Added labs, assessments, quick reference, instructor guide
- **v2.0.0** (January 2026) - Complete 11-module curriculum
- **v1.0.0** (December 2024) - Initial 6-module course

---

*For questions or feedback, please open an issue in the repository.*
