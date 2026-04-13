# AGENTS.md

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

---

# HelixAgent: AI-Powered Ensemble LLM Service

## Project Overview

HelixAgent is a production-ready, AI-powered ensemble LLM service written in Go (1.25+) that aggregates responses from multiple language models to provide the most accurate and reliable outputs. It provides OpenAI-compatible APIs with support for 47+ LLM providers, debate orchestration, MCP adapters, and containerized infrastructure. (Authoritative count: `ls internal/llm/providers/ | grep -v common`.)

**Module**: `dev.helix.agent`

**Main Binary**: `helixagent` (built from `cmd/helixagent/`)

**Additional Applications**:
- `api` - Standalone API server
- `grpc-server` - gRPC service endpoint
- `cognee-mock` - Mock Cognee service for testing
- `sanity-check` - System validation tool
- `mcp-bridge` - MCP protocol bridge
- `generate-constitution` - Constitution file generator

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│             