# Multi-stage build for optimized production image
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata curl make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Run tests to ensure code quality
RUN CGO_ENABLED=0 go test -v ./...

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o superagent \
    ./cmd/superagent

# Production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    jq \
    dumb-init \
    && rm -rf /var/cache/apk/*

# Create non-root user with proper UID/GID
RUN addgroup -g 1001 -S superagent && \
    adduser -u 1001 -S superagent -G superagent -h /app -s /bin/sh

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/superagent .
COPY --from=builder /app/*.md ./

# Create necessary directories with proper permissions
RUN mkdir -p /app/plugins /app/logs /app/config /app/data && \
    chown -R superagent:superagent /app && \
    chmod 755 /app

# Switch to non-root user
USER superagent

# Add labels for metadata
LABEL org.opencontainers.image.title="SuperAgent" \
      org.opencontainers.image.description="AI-powered ensemble LLM service" \
      org.opencontainers.image.version="1.0.0" \
      org.opencontainers.image.vendor="SuperAgent" \
      org.opencontainers.image.licenses="MIT"

# Expose port
EXPOSE 8080

# Health check with better reliability
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f -s http://localhost:8080/health > /dev/null || exit 1

# Use dumb-init for proper signal handling
ENTRYPOINT ["/usr/bin/dumb-init", "--"]

# Run the application
CMD ["./superagent"]