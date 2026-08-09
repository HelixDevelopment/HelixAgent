# MCP Servers Container
# Builds and runs all Model Context Protocol servers for HelixAgent
#
# This container includes:
# - Archived MCP servers from modelcontextprotocol/servers-archived (Node.js + Python)
#   — always, they live in this build context at servers-archived/.
# - Active MCP servers from modelcontextprotocol/servers (Node.js + Python)
#   — OPTIONALLY, see ACTIVE_SERVERS below.
#
# WHY ACTIVE SERVERS ARE OPTIONAL (2026-08-09)
#
# Commit 6e245ff1 ("chore(P1.5-T03.05): remove MCP-Servers +
# external/mcp-servers/servers (will promote to meta-repo root)") deleted the
# `servers` submodule from THIS directory and promoted it to the consuming
# meta-repo's root. The Containerfile was not updated with it, so it kept an
# unconditional `COPY servers/ /build/servers/` against a path that no longer
# exists in the build context. Measured on the shipped file:
#
#   $ podman build -f Dockerfile .
#   Error: building at STEP "COPY servers/ /build/servers/": checking on
#   sources under ".../external/mcp-servers": copier: stat: "/servers":
#   no such file or directory
#
# i.e. the shipped image could not be built at all. A Containerfile cannot
# reach outside its build context, and this submodule must stay project-
# agnostic (no hardcoded `../../mcp_servers` reach into a specific consumer's
# tree), so the active sources are an OPT-IN input the operator materialises
# INTO this directory as `servers/` before building:
#
#   # from the consuming project root, e.g.
#   cp -a mcp_servers <this-dir>/servers          # or a bind/symlink-free copy
#   podman build --build-arg ACTIVE_SERVERS=present -t helixagent-mcp-servers .
#
# Default (`absent`) builds the archived servers only; /app/servers is created
# empty and start-all.sh skips the seven active servers with an explicit log
# line rather than aborting.
ARG ACTIVE_SERVERS=absent

FROM node:22-alpine AS base

WORKDIR /build

# Install build dependencies including Python
RUN apk add --no-cache git python3 py3-pip make g++ chromium

# --- Active-server source selection ----------------------------------------
# Only the stage named by ACTIVE_SERVERS is reachable from `builder`, and an
# unreachable stage is never executed — so the `COPY servers/` below is not
# evaluated (and cannot fail) unless the operator opts in.
FROM base AS active-absent
RUN mkdir -p /build/servers

FROM base AS active-present
COPY servers/ /build/servers/
# Build active Node.js servers
RUN cd /build/servers && npm ci && npm run build || true
# ---------------------------------------------------------------------------

FROM active-${ACTIVE_SERVERS} AS builder

WORKDIR /build

# Copy archived server sources
COPY servers-archived/ /build/servers-archived/

# Build archived Node.js servers (each has its own package.json)
WORKDIR /build/servers-archived/src
RUN for dir in */; do \
    if [ -f "$dir/package.json" ]; then \
        echo "Installing dependencies for $dir" && \
        cd "$dir" && npm ci && npm run build 2>/dev/null || true && cd ..; \
    fi; \
done

# Production image
FROM node:22-alpine

WORKDIR /app

# Install runtime dependencies including Python and pip
RUN apk add --no-cache chromium postgresql-client redis git curl python3 py3-pip

# Set Puppeteer to use system Chromium
ENV PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium-browser

# Copy built servers from builder
COPY --from=builder /build/servers /app/servers
COPY --from=builder /build/servers-archived /app/servers-archived

# Install Python dependencies for Python-based servers
WORKDIR /app/servers/src
RUN for dir in */; do \
    if [ -f "$dir/pyproject.toml" ]; then \
        echo "Installing Python dependencies for $dir" && \
        cd "$dir" && pip install --break-system-packages -e . 2>/dev/null || true && cd ..; \
    fi; \
done

WORKDIR /app/servers-archived/src
RUN for dir in */; do \
    if [ -f "$dir/pyproject.toml" ]; then \
        echo "Installing Python dependencies for $dir" && \
        cd "$dir" && pip install --break-system-packages -e . 2>/dev/null || true && cd ..; \
    fi; \
done

WORKDIR /app

# Copy startup scripts
COPY scripts/ /app/scripts/
RUN chmod +x /app/scripts/*.sh

# Expose ports for MCP servers (SSE mode)
# Each server can run on its own port in SSE mode
EXPOSE 3001-3020

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3001/health || exit 1

# Default command - starts all servers
CMD ["/app/scripts/start-all.sh"]
