# HelixAgent MCP Server Dockerfile
# Provides MCP protocol access to all HelixAgent features for CLI agents

FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json tsconfig.json ./
COPY src ./src

# Install dependencies and build
RUN npm install && npm run build

# Production stage
FROM node:20-alpine

WORKDIR /app

# Install runtime dependencies only
COPY package.json ./
RUN npm install --production

# Copy built files
COPY --from=builder /app/dist ./dist

# Environment variables.
# MCP_TRANSPORT=sse is load-bearing, not cosmetic: the stdio transport (the
# code default, correct for CLI agents) binds no port and serves no /health,
# so under stdio the published port below and the health check would both
# address something that cannot exist. The server reads all three of these.
ENV NODE_ENV=production
ENV MCP_TRANSPORT=sse
ENV MCP_PORT=9100
ENV HELIXAGENT_URL=http://helixagent:7061

# Health check. Uses `node` rather than curl/wget: curl is NOT present in
# node:20-alpine (verified against the image), and node certainly is, since
# this is a node image. ${MCP_PORT} expands at runtime because HEALTHCHECK
# shell form runs under /bin/sh, so the probe follows the port the server
# actually binds instead of a separately-maintained literal.
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD node -e "require('http').get('http://127.0.0.1:'+(process.env.MCP_PORT||9100)+'/health',r=>process.exit(r.statusCode===200?0:1)).on('error',()=>process.exit(1))" || exit 1

# Expose port
EXPOSE 9100

# Start server
CMD ["node", "dist/index.js"]
