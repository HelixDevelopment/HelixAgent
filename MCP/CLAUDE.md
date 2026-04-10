# CLAUDE.md - MCP Directory (Third-Party MCP Submodules)

## Overview

The `MCP/` directory is the HelixAgent container orchestration layer for 48 MCP (Model Context Protocol) servers. It contains Docker Compose configuration, Dockerfile templates, and 46 third-party git submodules. This directory is read-only for submodule content -- only the orchestration files (`docker-compose.yml`, `dockerfiles/`, `README.md`) are HelixAgent-owned.

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## Directory Structure

```
MCP/
├── docker-compose.yml       # Main Docker Compose for all 48 MCP servers
├── dockerfiles/             # 36 Dockerfile templates for containerized deployment
│   ├── Dockerfile.base-node     # Base Node.js image
│   ├── Dockerfile.base-python   # Base Python image
│   ├── Dockerfile.github        # GitHub MCP server
│   ├── Dockerfile.slack         # Slack MCP server
│   ├── Dockerfile.redis         # Redis MCP server
│   └── ...                      # 31 more server-specific Dockerfiles
├── submodules/              # 46 third-party git submodules
│   ├── github-mcp-server/       # GitHub (official)
│   ├── slack-mcp/               # Slack integration
│   ├── notion-mcp-server/       # Notion integration
│   ├── atlassian-mcp/           # Jira/Confluence
│   ├── redis-mcp/               # Redis (official)
│   ├── mongodb-mcp/             # MongoDB
│   ├── qdrant-mcp/              # Qdrant vector search
│   ├── playwright-mcp/          # Microsoft Playwright
│   ├── sentry-mcp/              # Sentry error tracking
│   ├── aws-mcp/                 # AWS Labs MCP
│   ├── cloudflare-mcp/          # Cloudflare Workers/KV/R2/D1
│   ├── kubernetes-mcp/          # Kubernetes operations
│   ├── context7-mcp/            # Context7 documentation
│   ├── brave-search/            # Brave Search API
│   ├── langchain-mcp/           # LangChain adapters
│   ├── python-sdk/              # MCP Python SDK
│   ├── typescript-sdk/          # MCP TypeScript SDK
│   ├── inspector/               # MCP Inspector tool
│   ├── registry/                # MCP Registry
│   └── ...                      # 27 more submodules
├── servers/                 # Additional server configurations
└── README.md                # Full server catalog with ports, env vars, setup
```

## MCP Server Categories

| Category | Count | Ports | Servers |
|----------|-------|-------|---------|
| Core (no auth) | 7 | 3001-3007 | everything, filesystem, memory, sequential-thinking, fetch, git, time |
| Browser Automation | 2 | 3010-3011 | playwright, browserbase |
| Databases and Storage | 5 | 3020-3024 | redis, mongodb, qdrant, elasticsearch, supabase |
| Version Control and DevOps | 4 | 3030-3033 | github, sentry, heroku, cloudflare |
| Cloud Platforms | 2 | 3040-3041 | aws, kubernetes |
| Productivity and Communication | 7 | 3050-3056 | slack, telegram, notion, airtable, trello, atlassian, obsidian |
| Search and AI | 5 | 3060-3064 | brave-search, perplexity, context7, firecrawl, omnisearch |
| AI Framework Integrations | 3 | 3070-3072 | langchain, llamaindex, docs |
| Microsoft/Azure | 1 | 3080 | microsoft |

## Key Rules

1. **Read-only submodules** -- NEVER commit or push changes to any submodule in `submodules/`. Update via `git submodule update --remote`.
2. **Orchestration files are HelixAgent-owned** -- `docker-compose.yml`, `dockerfiles/`, and `README.md` can be modified as needed.
3. **Container names** -- All containers follow the `mcp-<name>` naming convention.
4. **Port allocation** -- Ports 3001-3099 are reserved for MCP servers. See `README.md` for the full mapping.
5. **Environment variables** -- Server-specific API keys and configuration go in `.env.mcp` (see `README.md` for the template).
6. **Core servers from MCP-Servers** -- The 7 core servers (ports 3001-3007) use source code from the `MCP-Servers/` submodule. Their Dockerfiles in `dockerfiles/` reference `../MCP-Servers/src/<server>` as build context.

## Build and Run

```bash
# Build all MCP server containers
docker-compose -f MCP/docker-compose.yml build

# Start all MCP servers
docker-compose -f MCP/docker-compose.yml up -d

# Start specific servers
docker-compose -f MCP/docker-compose.yml up -d mcp-github mcp-memory mcp-context7

# Stop all
docker-compose -f MCP/docker-compose.yml down

# Initialize submodules
git submodule update --init --recursive

# Update submodules to latest
git submodule update --remote --merge
```

## Commit Style

For HelixAgent-owned files in this directory, use Conventional Commits:

```
feat(mcp): add new MCP server for <service>
fix(mcp): correct port mapping in docker-compose
docs(mcp): update server catalog
```
