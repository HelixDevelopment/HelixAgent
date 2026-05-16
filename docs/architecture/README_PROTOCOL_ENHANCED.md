# HelixAgent: Advanced AI Protocol Orchestration Platform

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD1E?style=flat-square&logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue?style=flat-square&logo=docker)](https://www.docker.com)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat-square&logo=kubernetes)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen?style=flat-square)](https://dev.helix.agent/actions/workflows/tests)
[![MCP](https://img.shields.io/badge/MCP-Supported-FF6B35?style=flat-square)](https://modelcontextprotocol.io)
[![LSP](https://img.shields.io/badge/LSP-Supported-007ACC?style=flat-square)](https://microsoft.github.io/language-server-protocol)

**HelixAgent is a comprehensive AI orchestration platform that seamlessly integrates multiple AI protocols (MCP, LSP, ACP, Embeddings) with advanced caching, security, monitoring, and enterprise-grade features.**

## 🚀 What Makes HelixAgent Special

### 🏗️ Multi-Protocol AI Orchestration
HelixAgent uniquely supports **four major AI protocols** through a unified API:

- **🔧 MCP (Model Context Protocol)** - Tool execution and agent integration
- **💻 LSP (Language Server Protocol)** - Code intelligence and language services
- **🤖 ACP (Agent Client Protocol)** - Agent communication and coordination
- **🧠 Embeddings** - Vector operations and semantic search

### ⚡ Enterprise-Grade Features
- **Advanced Protocol-Aware Caching** with intelligent invalidation
- **Real-Time Performance Monitoring** with alerting
- **Enterprise Security** with API keys, RBAC, and rate limiting
- **Production Monitoring** with health checks and metrics
- **Scalable Architecture** supporting horizontal scaling

## 📋 Quick Start

### Using Docker (Recommended)
```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Start with full protocol support
docker-compose -f docker-compose.protocol.yml up -d

# Or use the comprehensive deployment
make docker-protocol-full
```

### Local Development
```bash
# Install dependencies
go mod tidy

# Run with all protocols enabled
go run cmd/helixagent/main.go --protocols=all --config=config.yaml

# Run the demo to see all features
go run demo.go
```

## 🎯 Core Capabilities

### Unified Protocol API
```bash
# Execute any protocol through unified API
curl -X POST http://localhost:7061/v1/protocols/execute \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "protocolType": "mcp",
    "serverId": "filesystem-tools",
    "toolName": "read_file",
    "arguments": {"path": "/etc/hosts"}
  }'
```

### Multi-Protocol Operations
```bash
# MCP Tool Execution
curl -X POST /v1/mcp/servers/filesystem-tools/execute \
  -H "Authorization: Bearer sk-api-key" \
  -d '{"toolName": "list_dir", "arguments": {"path": "/tmp"}}'

# LSP Code Intelligence
curl -X POST /v1/lsp/execute \
  -H "Authorization: Bearer sk-api-key" \
  -d '{"serverId": "typescript-lsp", "toolName": "completion", "arguments": {"file": "app.ts", "line": 10}}'

# ACP Agent Communication
curl -X POST /v1/acp/execute \
  -H "Authorization: Bearer sk-api-key" \
  -d '{"serverId": "ai-agent", "action": "analyze", "parameters": {"text": "Hello world"}}'

# Embedding Generation
curl -X POST /v1/embeddings/generate \
  -H "Authorization: Bearer sk-api-key" \
  -d '{"text": "Advanced AI orchestration platform"}'
```

## 🏗️ System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Load Balancer / API Gateway               │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────┐ │
│  │  Protocol   │ │  Security   │ │ Monitoring  │ │ Caching │ │
│  │Orchestrator │ │   & Auth    │ │   & Alert   │ │  Layer  │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────┘ │
├─────────────────────────────────────────────────────────────┤
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
│  │ MCP │ │ LSP │ │ ACP │ │Embedding│ │  Redis  │ │Postgres │ │
│  │Clients│ │Clients│ │Clients│ │ Service │ │  Cache  │ │  DB   │ │
│  └─────┘ └─────┘ └─────┘ └─────────┘ └─────────┘ └─────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 📊 Features Overview

### 🔧 Protocol Support

| Protocol | Status | Description |
|----------|--------|-------------|
| **MCP** | ✅ Full | Model Context Protocol with JSON-RPC 2.0 |
| **LSP** | ✅ Full | Language Server Protocol for code intelligence |
| **ACP** | ✅ Full | Agent Client Protocol for AI coordination |
| **Embeddings** | ✅ Full | Vector embeddings for semantic search |

### 🛡️ Security & Compliance

- **API Key Authentication** with bcrypt hashing
- **Role-Based Access Control** (RBAC)
- **Rate Limiting** per client and globally
- **JWT Session Management**
- **Audit Logging** for security events
- **Input Validation** and sanitization

### 📈 Monitoring & Observability

- **Real-Time Metrics** (latency, throughput, error rates)
- **Configurable Alerts** with multiple severity levels
- **Health Checks** for all components
- **Performance Dashboards** (Prometheus + Grafana)
- **Resource Usage Tracking** (CPU, Memory, Network)

### ⚡ Performance & Caching

- **Protocol-Aware Caching** with tag-based invalidation
- **LRU Eviction** with configurable TTL
- **Connection Pooling** for protocol clients
- **Async Processing** for non-blocking operations
- **Cache Warming** for common requests

### 🚀 Production Features

- **Horizontal Scaling** with Kubernetes
- **Load Balancing** and failover
- **Database Sharding** support
- **Distributed Caching** (Redis Cluster)
- **Container Orchestration** (Docker + K8s)

## 🔌 API Reference

### Unified Protocol Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/protocols/execute` | POST | Execute any protocol request |
| `/v1/protocols/servers` | GET | List all protocol servers |
| `/v1/protocols/metrics` | GET | Get protocol metrics |
| `/v1/protocols/refresh` | POST | Refresh all protocol servers |
| `/v1/protocols/configure` | POST | Configure protocol settings |

### Protocol-Specific Endpoints

#### MCP (Model Context Protocol)
- `GET /v1/mcp/servers` - List MCP servers
- `POST /v1/mcp/servers/{id}/execute` - Execute MCP tools
- `GET /v1/mcp/servers/{id}/tools` - Get server tools
- `GET /v1/mcp/stats` - MCP usage statistics

#### LSP (Language Server Protocol)
- `GET /v1/lsp/servers` - List LSP servers
- `POST /v1/lsp/execute` - Execute LSP operations
- `GET /v1/lsp/stats` - LSP usage statistics

#### ACP (Agent Client Protocol)
- `GET /v1/acp/servers` - List ACP servers
- `POST /v1/acp/execute` - Execute ACP actions
- `GET /v1/acp/stats` - ACP usage statistics

#### Embeddings
- `POST /v1/embeddings/generate` - Generate embeddings
- `POST /v1/embeddings/generate-batch` - Batch embedding generation
- `POST /v1/embeddings/compare` - Compare embeddings
- `GET /v1/embeddings/providers` - List embedding providers

### Security & Monitoring

#### Security Management
- `GET /v1/security/keys` - List API keys
- `POST /v1/security/keys` - Create API key
- `DELETE /v1/security/revoke` - Revoke API key

#### Monitoring & Health
- `GET /v1/monitoring/metrics` - System metrics
- `GET /v1/monitoring/alerts` - Active alerts
- `GET /v1/monitoring/health` - Health status
- `POST /v1/monitoring/rules` - Configure alert rules

## 🐳 Deployment Options

### Quick Docker Deployment
```bash
# Full protocol stack
docker-compose -f docker-compose.protocol.yml up -d

# Development with hot reload
docker-compose -f docker-compose.dev.yml up -d

# Production with monitoring
docker-compose -f docker-compose.prod.yml up -d
```

### Kubernetes Deployment
```bash
# Apply manifests
kubectl apply -f k8s/

# Check deployment
kubectl get pods -l app=helixagent

# View logs
kubectl logs -l app=helixagent -f
```

### Manual Installation
```bash
# Build from source
go build -o helixagent cmd/helixagent/main.go

# Configure
cp config.example.yaml config.yaml
# Edit config.yaml with your settings

# Run
./helixagent --config config.yaml
```

## ⚙️ Configuration

### Basic Configuration
```yaml
server:
  port: 8080
  host: "0.0.0.0"

protocols:
  enabled: true
  security:
    enabled: true
    jwt_secret: "your-secret-key"
  caching:
    enabled: true
    ttl: "30m"
  monitoring:
    enabled: true

database:
  url: "postgres://user:password@localhost/helixagent"

redis:
  enabled: true
  url: "redis://localhost:6379"
```

### Advanced Protocol Configuration
```yaml
mcp:
  enabled: true
  servers:
    - name: "filesystem-tools"
      command: ["node", "/path/to/mcp-filesystem"]
      timeout: "30s"
    - name: "web-scraper"
      command: ["python", "/path/to/scraper"]
      env: ["API_KEY=secret"]

lsp:
  enabled: true
  servers:
    - name: "typescript-lsp"
      language: "typescript"
      command: "typescript-language-server"
    - name: "gopls"
      language: "go"
      command: "gopls"

acp:
  enabled: true
  servers:
    - name: "ai-assistant"
      url: "ws://localhost:7061/agent"
      reconnect: true

embeddings:
  enabled: true
  provider: "openai"
  model: "text-embedding-ada-002"
  batch_size: 100
```

## 📈 Monitoring & Alerting

### Prometheus Metrics
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'helixagent'
    static_configs:
      - targets: ['localhost:7061']
    metrics_path: '/v1/monitoring/metrics'
    scrape_interval: 15s
```

### Alert Rules
```yaml
groups:
  - name: helixagent
    rules:
      - alert: HighErrorRate
        expr: rate(protocol_requests_total{status="error"}[5m]) > 0.1
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(protocol_request_duration_seconds_bucket[5m])) > 5
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
```

## 🔒 Security Best Practices

### API Key Management
```bash
# Create role-specific keys
curl -X POST /v1/security/keys \
  -H "Authorization: Bearer sk-admin-key" \
  -d '{"name":"client-app","permissions":["mcp:read","embedding:execute"]}'

# Rotate keys regularly
curl -X POST /v1/security/revoke \
  -d '{"key":"sk-old-key"}'

curl -X POST /v1/security/keys \
  -d '{"name":"client-app-v2","permissions":["mcp:read","embedding:execute"]}'
```

### Network Security
- Use HTTPS in production
- Configure firewall rules
- Enable rate limiting
- Monitor for suspicious activity
- Regular security audits

## 🧪 Testing & Validation

### Run Test Suite
```bash
# Run all tests
make test

# Run protocol-specific tests
make test-protocols

# Run integration tests
make test-integration

# Run performance tests
make test-performance
```

### Demo Application
```bash
# Run comprehensive demo
go run demo.go

# This demonstrates:
# - Protocol server management
# - MCP tool execution
# - ACP agent communication
# - Embedding generation
# - Security features
# - Monitoring capabilities
# - Caching performance
# - Rate limiting
```

## 🤝 Contributing

### Development Setup
```bash
# Fork and clone
git clone https://github.com/yourusername/helixagent.git
cd helixagent

# Install dependencies
go mod tidy

# Setup development environment
make setup-dev

# Run tests
make test

# Start development server
make run-dev
```

### Adding New Protocols
1. Implement protocol client in `internal/services/`
2. Add handler in `internal/handlers/`
3. Update router configuration
4. Add tests and documentation
5. Update deployment manifests

## 📚 Documentation

- **[API Documentation](./PROTOCOL_SUPPORT_DOCUMENTATION.md)** - Complete API reference
- **[Deployment Guide](../deployment/PROTOCOL_DEPLOYMENT_GUIDE.md)** - Production deployment
- **[Configuration Guide](../deployment/configuration.md)** - Advanced configuration
- **[Troubleshooting](../deployment/troubleshooting.md)** - Common issues and solutions

## 🏢 Enterprise Support

For enterprise deployments, custom integrations, or priority support:

- 📧 **Email**: enterprise@helixagent.ai
- 💬 **Slack**: [Join our community](https://helixagent.slack.com)
- 📖 **Documentation**: [Enterprise Guide](../deployment/DEPLOYMENT_GUIDE.md)

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Model Context Protocol](https://modelcontextprotocol.io) for the MCP specification
- [Language Server Protocol](https://microsoft.github.io/language-server-protocol) community
- [Agent Client Protocol](https://agentprotocol.org) for ACP standards
- Open source community for the amazing tools and libraries

---

**HelixAgent: Orchestrating the Future of AI Integration** 🚀</content>
<parameter name="filePath">/media/milosvasic/DATA4TB/Projects/helix_agent/README.md