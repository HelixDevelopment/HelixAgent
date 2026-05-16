# MCP Server Containerization

This document describes the containerized MCP (Model Context Protocol) server infrastructure for HelixAgent, which eliminates all npm/npx dependencies and provides consistent, reproducible MCP server deployments.

## Overview

HelixAgent's MCP infrastructure has been fully containerized, replacing all 60+ npx-based MCP invocations with Docker containers built from Git submodules. This provides:

- **Zero npm/npx dependencies** - No runtime downloads
- **Consistent environments** - Pre-built images with all dependencies
- **Offline capability** - Works without network after build
- **Version control** - Git submodules pin specific versions
- **Multi-architecture support** - Build once, run anywhere

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI Agents                                │
│                (OpenCode, Crush, ClaudeCode, etc.)              │
└─────────────────────────────────┬───────────────────────────────┘
                                  │ HTTP/SSE
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                     HelixAgent                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │           ContainerMCPConfigGenerator                     │  │
│  │  - Generates URLs for all 65 containerized MCPs          │  │
│  │  - Zero npx commands                                      │  │
│  │  - Environment-based enable/disable                       │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────┬───────────────────────────────┘
                                  │ HTTP/SSE (ports 9101-9999)
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│               Docker/Podman Container Network                    │
│                 (helixagent-mcp-network)                        │
│                                                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ mcp-fetch   │ │ mcp-git     │ │ mcp-time    │   ...65+      │
│  │ :9101       │ │ :9102       │ │ :9103       │   containers  │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

## Port Allocation Scheme

All MCP servers are exposed on ports in the 9101-9999 range, organized by category:

| Port Range | Category | Description |
|------------|----------|-------------|
| 9101-9120 | Core | Official MCP servers (fetch, git, time, filesystem, memory, etc.) |
| 9201-9220 | Database | MongoDB, Redis, MySQL, Elasticsearch, Supabase |
| 9301-9320 | Vector | Qdrant, Chroma, Pinecone, Weaviate |
| 9401-9440 | DevOps | GitHub, GitLab, Sentry, Kubernetes, Docker, AWS, GCP, etc. |
| 9501-9520 | Browser | Playwright, Browserbase, Firecrawl, Crawl4AI |
| 9601-9620 | Communication | Slack, Discord, Telegram |
| 9701-9740 | Productivity | Notion, Linear, Jira, Asana, Trello, Todoist, Monday, etc. |
| 9801-9840 | Search/AI | Brave Search, Exa, Tavily, Perplexity, LlamaIndex, LangChain |
| 9901-9920 | Google | Google Drive, Calendar, Maps, YouTube, Gmail |
| 9921-9940 | Monitoring | Datadog, Grafana, Prometheus |
| 9941-9960 | Finance | Stripe, HubSpot, Zendesk |
| 9961-9999 | Design | Figma |

### Complete Port Mapping

| MCP Server | Port | Category |
|------------|------|----------|
| fetch | 9101 | core |
| git | 9102 | core |
| time | 9103 | core |
| filesystem | 9104 | core |
| memory | 9105 | core |
| everything | 9106 | core |
| sequential-thinking | 9107 | core |
| sqlite | 9108 | core |
| puppeteer | 9109 | core |
| postgres | 9110 | core |
| mongodb | 9201 | database |
| redis | 9202 | database |
| mysql | 9203 | database |
| elasticsearch | 9204 | database |
| supabase | 9205 | database |
| qdrant | 9301 | vector |
| chroma | 9302 | vector |
| pinecone | 9303 | vector |
| weaviate | 9304 | vector |
| github | 9401 | devops |
| gitlab | 9402 | devops |
| sentry | 9403 | devops |
| kubernetes | 9404 | devops |
| docker | 9405 | devops |
| ansible | 9406 | devops |
| aws | 9407 | devops |
| gcp | 9408 | devops |
| heroku | 9409 | devops |
| cloudflare | 9410 | devops |
| vercel | 9411 | devops |
| workers | 9412 | devops |
| jetbrains | 9413 | devops |
| playwright | 9501 | browser |
| browserbase | 9502 | browser |
| firecrawl | 9503 | browser |
| crawl4ai | 9504 | browser |
| slack | 9601 | communication |
| discord | 9602 | communication |
| telegram | 9603 | communication |
| notion | 9701 | productivity |
| linear | 9702 | productivity |
| jira | 9703 | productivity |
| asana | 9704 | productivity |
| trello | 9705 | productivity |
| todoist | 9706 | productivity |
| monday | 9707 | productivity |
| airtable | 9708 | productivity |
| obsidian | 9709 | productivity |
| atlassian | 9710 | productivity |
| brave-search | 9801 | search |
| exa | 9802 | search |
| tavily | 9803 | search |
| perplexity | 9804 | search |
| kagi | 9805 | search |
| omnisearch | 9806 | search |
| context7 | 9807 | search |
| llamaindex | 9808 | search |
| langchain | 9809 | search |
| openai | 9810 | search |
| google-drive | 9901 | google |
| google-calendar | 9902 | google |
| google-maps | 9903 | google |
| youtube | 9904 | google |
| gmail | 9905 | google |
| datadog | 9921 | monitoring |
| grafana | 9922 | monitoring |
| prometheus | 9923 | monitoring |
| stripe | 9941 | finance |
| hubspot | 9942 | finance |
| zendesk | 9943 | finance |
| figma | 9961 | design |

## Quick Start

### 1. Build All MCP Container Images

```bash
# Build all 65+ MCP container images
docker-compose -f docker/mcp/docker-compose.mcp-full.yml build

# Or build specific services
docker-compose -f docker/mcp/docker-compose.mcp-full.yml build mcp-fetch mcp-git mcp-time
```

### 2. Start MCP Containers

```bash
# Start all MCP containers
docker-compose -f docker/mcp/docker-compose.mcp-full.yml up -d

# Start only core MCPs
docker-compose -f docker/mcp/docker-compose.mcp-full.yml up -d \
  mcp-fetch mcp-git mcp-time mcp-filesystem mcp-memory \
  mcp-everything mcp-sequential-thinking mcp-sqlite mcp-puppeteer

# Start specific categories
docker-compose -f docker/mcp/docker-compose.mcp-full.yml up -d \
  mcp-github mcp-gitlab mcp-slack mcp-notion
```

### 3. Verify Containers

```bash
# Check container status
docker-compose -f docker/mcp/docker-compose.mcp-full.yml ps

# Run validation challenge
./challenges/scripts/mcp_containerized_challenge.sh

# Run with container connectivity tests
RUN_CONTAINER_TESTS=1 ./challenges/scripts/mcp_containerized_challenge.sh
```

### 4. Generate CLI Agent Configs

```bash
# Generate OpenCode config with container URLs
./bin/helixagent --generate-opencode-config --opencode-output=~/.config/opencode/opencode.json

# Generate Crush config with container URLs
./bin/helixagent --generate-agent-config=crush --agent-config-output=~/.config/crush/crush.json
```

## Configuration

### Environment Variables

The containerized MCPs use environment variables for credentials. Create a `.env.mcp` file:

```bash
# Core (no credentials needed)
# fetch, git, time, filesystem, memory, everything, sequential-thinking, sqlite, puppeteer

# Database
POSTGRES_URL=postgresql://user:pass@localhost:5432/db
MONGODB_URI=mongodb://localhost:27017
REDIS_URL=redis://localhost:6379
MYSQL_URL=mysql://user:pass@localhost:3306/db
ELASTICSEARCH_URL=http://localhost:9200

# Vector Databases
QDRANT_URL=http://localhost:6333
CHROMA_URL=http://localhost:8000
PINECONE_API_KEY=your-key

# DevOps
GITHUB_TOKEN=ghp_xxx
GITLAB_TOKEN=glpat-xxx
KUBECONFIG=/path/to/.kube/config
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
CLOUDFLARE_API_TOKEN=xxx

# Communication
SLACK_BOT_TOKEN=xoxb-xxx
SLACK_TEAM_ID=Txxx
DISCORD_TOKEN=xxx
TELEGRAM_BOT_TOKEN=xxx

# Productivity
NOTION_API_KEY=secret_xxx
LINEAR_API_KEY=lin_xxx
JIRA_URL=https://xxx.atlassian.net
JIRA_EMAIL=email@example.com
JIRA_API_TOKEN=xxx

# Search/AI
BRAVE_API_KEY=xxx
OPENAI_API_KEY=sk-xxx
PERPLEXITY_API_KEY=pplx-xxx

# Google Services
GOOGLE_CLIENT_ID=xxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=xxx
GOOGLE_MAPS_API_KEY=xxx
YOUTUBE_API_KEY=xxx
```

### Custom MCP Host

By default, containers are accessed at `localhost`. For remote deployments:

```bash
# Set custom host
export MCP_CONTAINER_HOST=mcp.example.com

# Or in .env.mcp
MCP_CONTAINER_HOST=mcp.example.com
```

## Container Images

### Dockerfile Templates

| Template | Purpose | Base Image |
|----------|---------|------------|
| `Dockerfile.mcp-server` | Core MCP servers from MCP-Servers monorepo | node:20-alpine |
| `Dockerfile.mcp-submodule` | TypeScript/Node.js submodule servers | node:20-alpine |
| `Dockerfile.mcp-python` | Python MCP servers | python:3.11-slim |
| `Dockerfile.mcp-go` | Go MCP servers | golang:1.22-alpine |
| `Dockerfile.mcp-playwright` | Browser automation with Playwright | mcr.microsoft.com/playwright |

### Build Sources

| Source | Location | Services |
|--------|----------|----------|
| MCP-Servers monorepo | `mcp_servers/` | fetch, git, time, filesystem, memory, everything, sequentialthinking, sqlite, puppeteer, postgres |
| Git submodules | `MCP/submodules/` | All other 55+ services |

## Config Generator

### ContainerMCPConfigGenerator

The `ContainerMCPConfigGenerator` in `internal/mcp/config/generator_container.go` generates MCP configurations using container URLs:

```go
import "dev.helix.agent/internal/mcp/config"

gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

// Get all MCPs (65+)
allMCPs := gen.GenerateContainerMCPs()

// Get only enabled MCPs
enabledMCPs := gen.GetEnabledContainerMCPs()

// Get MCPs by category
byCategory := gen.GetMCPsByCategory()

// Get summary
summary := gen.GenerateSummary()
// summary["npx_dependencies"] == 0  // Always zero!

// Validate port allocations
if err := gen.ValidatePortAllocations(); err != nil {
    log.Fatal(err)
}
```

### Generated Config Format

The generator produces configs in this format:

```json
{
  "mcps": {
    "fetch": {
      "type": "remote",
      "url": "http://localhost:9101/sse",
      "enabled": true
    },
    "github": {
      "type": "remote",
      "url": "http://localhost:9401/sse",
      "enabled": true
    }
  }
}
```

## Testing

### Unit Tests

```bash
# Run all container generator tests
go test -v ./internal/mcp/config/... -run TestContainerMCPConfigGenerator

# Run specific tests
go test -v ./internal/mcp/config/... -run TestContainerMCPConfigGenerator_ZeroNPXCommands
go test -v ./internal/mcp/config/... -run TestContainerMCPConfigGenerator_PortAllocationUnique
```

### Integration Tests

```bash
# Run integration tests (requires running containers)
RUN_CONTAINER_TESTS=1 go test -v ./tests/integration/... -run TestMCPContainer
```

### Validation Challenge

```bash
# Run full validation (65 tests)
./challenges/scripts/mcp_containerized_challenge.sh

# With container connectivity tests
RUN_CONTAINER_TESTS=1 ./challenges/scripts/mcp_containerized_challenge.sh
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose -f docker/mcp/docker-compose.mcp-full.yml logs mcp-fetch

# Check if port is in use
lsof -i :9101

# Rebuild specific container
docker-compose -f docker/mcp/docker-compose.mcp-full.yml build --no-cache mcp-fetch
```

### Health Check Failures

```bash
# Check health status
docker inspect helixagent-mcp-fetch | jq '.[0].State.Health'

# Test connectivity manually
nc -z localhost 9101 && echo "OK" || echo "FAILED"
```

### Environment Variable Issues

```bash
# Verify env vars in container
docker exec helixagent-mcp-github env | grep GITHUB

# Check compose variable substitution
docker-compose -f docker/mcp/docker-compose.mcp-full.yml config | grep GITHUB
```

## Migration from NPX

If migrating from npx-based MCP configuration:

1. **Build container images**: `docker-compose -f docker/mcp/docker-compose.mcp-full.yml build`
2. **Start containers**: `docker-compose -f docker/mcp/docker-compose.mcp-full.yml up -d`
3. **Update config generator usage**: Switch from `NewFullMCPConfigGenerator` to `NewContainerMCPConfigGenerator`
4. **Regenerate CLI configs**: Run `--generate-opencode-config` or similar
5. **Verify**: Run challenge script to validate

## Key Files

| File | Purpose |
|------|---------|
| `internal/mcp/config/generator_container.go` | Container config generator |
| `internal/mcp/config/generator_container_test.go` | Unit tests |
| `docker/mcp/docker-compose.mcp-full.yml` | Complete compose file (65 services) |
| `docker/mcp/Dockerfile.mcp-*` | Dockerfile templates |
| `tests/integration/mcp_container_test.go` | Integration tests |
| `challenges/scripts/mcp_containerized_challenge.sh` | Validation challenge |

## Success Criteria

| Criterion | Target | Verified By |
|-----------|--------|-------------|
| Containerized MCPs | 65+ | Challenge test 61 |
| NPX Dependencies | 0 | Challenge tests 6-10 |
| Port Conflicts | 0 | Challenge test 64 |
| Health Checks | All pass | Container status |
| Config Generation | Container URLs only | Generator tests |
