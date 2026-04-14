1: # AGENTS.md
2: 
3: ## MANDATORY: No CI/CD Pipelines
4: 
5: **NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**
6: 
7: - No `.github/workflows/` directory
8: - No `.gitlab-ci.yml` file
9: - No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
10: - **NO Git hooks (pre-commit, pre-push, post-commit, etc.)** may be installed or configured
11: - All builds and tests are run manually or via Makefile targets
12: - This rule is permanent and non-negotiable
13: 
14: ---
15: 
16: # HelixAgent: AI-Powered Ensemble LLM Service
17: 
18: ## Project Overview
19: 
20: HelixAgent is a production-ready, AI-powered ensemble LLM service written in Go (1.25+) that aggregates responses from multiple language models to provide the most accurate and reliable outputs. It provides OpenAI-compatible APIs with support for 47+ LLM providers, debate orchestration, MCP adapters, and containerized infrastructure. (Authoritative count: `ls internal/llm/providers/ | grep -v common`.)
21: 
22: **Module**: `dev.helix.agent`
23: 
24: **Main Binary**: `helixagent` (built from `cmd/helixagent/`)
25: 
26: **Additional Applications**:
27: - `api` - Standalone API server
28: - `grpc-server` - gRPC service endpoint
29: - `cognee-mock` - Mock Cognee service for testing
30: - `sanity-check` - System validation tool
31: - `mcp-bridge` - MCP protocol bridge
32: - `generate-constitution` - Constitution file generator
33: 
34: ## Architecture
35: 
36: ```
37: ┌─────────────────────────────────────────────────────────────────────────────┐
38: │             
