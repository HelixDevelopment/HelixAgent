# HelixAgent - Ready for Manual Testing

**Date:** 2026-01-29
**Status:** ✅ READY FOR MANUAL TESTING

---

## ✅ Configuration Export Complete

### OpenCode Configuration
**Location:** `~/helixagent-configs/opencode.json`

**Features:**
- HelixAgent AI Debate Ensemble as primary model
- 15 MCP servers configured:
  - **Core Local MCPs:** everything, fetch, filesystem, git, memory, puppeteer, sequential-thinking, sqlite, time
  - **HelixAgent Remote MCPs:** helixagent-mcp, helixagent-acp, helixagent-lsp, helixagent-embeddings, helixagent-vision, helixagent-cognee
- 4 specialized agents (coder, summarizer, task, title)
- OpenAI-compatible provider interface

**Install to OpenCode:**
```bash
# Copy configuration to OpenCode config directory
cp ~/helixagent-configs/opencode.json ~/.config/opencode/config.json

# Or use OpenCode's config path
opencode --config ~/helixagent-configs/opencode.json
```

### Crush Configuration
**Location:** `~/helixagent-configs/crush.json`

**Features:**
- HelixAgent AI Debate Ensemble model
- 6 HelixAgent remote MCPs
- 3 specialized agents (coder, default, reviewer)
- Advanced formatter configuration (29 languages)
- Vision, streaming, function calls, embeddings support

**Install to Crush:**
```bash
# Copy configuration to Crush config directory
cp ~/helixagent-configs/crush.json ~/.config/crush/config.json

# Or specify config path
crush --config ~/helixagent-configs/crush.json
```

---

## ✅ Service Management Scripts Created

Three new comprehensive scripts for managing all HelixAgent services:

### 1. start-all-services.sh
**Location:** `scripts/start-all-services.sh`

**Features:**
- Starts ALL HelixAgent containers in correct order
- 7 phased startup (Core → Messaging → Monitoring → Protocols → Integration → BigData → Security)
- Automatic runtime detection (Docker/Podman)
- Health check verification
- Optional Big Data and Security services

**Usage:**
```bash
# Start core services
./scripts/start-all-services.sh

# Start with Big Data services
START_BIGDATA=true ./scripts/start-all-services.sh

# Start everything
START_BIGDATA=true START_SECURITY=true ./scripts/start-all-services.sh
```

### 2. stop-all-services.sh
**Location:** `scripts/stop-all-services.sh`

**Features:**
- Gracefully stops all services in reverse order
- Cleans up orphaned containers
- Status verification

**Usage:**
```bash
./scripts/stop-all-services.sh
```

### 3. verify-services.sh
**Location:** `scripts/verify-services.sh`

**Features:**
- Comprehensive service health checks
- Container status verification
- Connectivity tests
- Health endpoint verification

**Usage:**
```bash
./scripts/verify-services.sh
```

---

## ✅ Currently Running Services

As of now, these core services are UP and HEALTHY:

| Service | Container | Status | Port |
|---------|-----------|--------|------|
| **PostgreSQL** | helixagent-postgres | ✅ Healthy (6 hours) | 5432 |
| **Redis** | helixagent-redis | ✅ Healthy (6 hours) | 6379 |
| **Cognee** | helixagent-cognee | ✅ Healthy (6 hours) | 8000 |
| **ChromaDB** | helixagent-chromadb | ✅ Running (6 hours) | 8000 |

---

## 🚀 Starting HelixAgent Server

With services running, start the HelixAgent server:

```bash
# Development mode
make run-dev

# Production mode
make run

# With specific config
./bin/helixagent --config configs/development.yaml
```

The server will:
1. Connect to PostgreSQL, Redis, Cognee, ChromaDB
2. Discover all 10 LLM providers (including dynamic Zen models)
3. Run LLMsVerifier verification and scoring
4. Configure AI Debate Team (25 LLMs: 5 positions × 5 LLMs per position)
5. Start API server on port 7061

---

## 🧪 Manual Testing Checklist

### 1. Verify Service Health

```bash
# Run verification script
./scripts/verify-services.sh

# Or manual checks
podman exec helixagent-postgres pg_isready
podman exec helixagent-redis redis-cli ping
curl http://localhost:8000/health  # Cognee
```

### 2. Start HelixAgent

```bash
make run-dev
```

**Expected Output:**
- LLMsVerifier discovering providers
- Dynamic Zen model discovery (6+ models)
- Provider verification and scoring
- AI Debate Team configured (25 LLMs)
- Server listening on :7061

### 3. Test API Endpoints

```bash
# Health check
curl http://localhost:7061/health

# Startup verification
curl http://localhost:7061/v1/startup/verification | jq

# AI Debate endpoint
curl -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${HELIXAGENT_API_KEY}" \
  -d '{
    "topic": "Should we use microservices architecture?",
    "enable_multi_pass_validation": true
  }'
```

### 4. Test OpenCode Integration

```bash
# With HelixAgent server running
opencode --config ~/helixagent-configs/opencode.json

# Test a simple query
opencode "Write a Python function to calculate fibonacci numbers"
```

**Expected:**
- OpenCode connects to HelixAgent on localhost:7061
- Uses HelixAgent AI Debate Ensemble
- Accesses 15 MCP servers
- Returns AI-generated code

### 5. Test Crush Integration

```bash
# With HelixAgent server running
crush --config ~/helixagent-configs/crush.json

# Test a query
crush "Review this code for security issues: [paste code]"
```

**Expected:**
- Crush connects to HelixAgent
- Uses debate ensemble for code review
- Applies formatter preferences
- Returns formatted, reviewed code

### 6. Test MCP Servers

```bash
# Test HelixAgent MCP endpoints
curl http://localhost:7061/v1/mcp
curl http://localhost:7061/v1/acp
curl http://localhost:7061/v1/lsp
curl http://localhost:7061/v1/embeddings
curl http://localhost:7061/v1/vision
curl http://localhost:7061/v1/cognee
```

### 7. Test AI Debate System

```bash
# Run debate challenge
./challenges/scripts/ai_debate_team_challenge.sh

# Run multipass validation challenge
./challenges/scripts/multipass_validation_challenge.sh

# Run semantic intent challenge
./challenges/scripts/semantic_intent_challenge.sh
```

### 8. Test Dynamic Zen Discovery

```bash
# Run Zen provider challenge
./challenges/scripts/zen_provider_challenge.sh

# Verify dynamic discovery in logs
grep "Dynamically discovered Zen models" /var/log/helixagent.log
```

---

## 📊 System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI Agents                                │
│         OpenCode              Crush                          │
│    (15 MCPs configured)  (6 MCPs + Formatters)             │
└─────────────────┬───────────────────┬──────────────────────┘
                  │                   │
                  ▼                   ▼
┌─────────────────────────────────────────────────────────────┐
│                  HelixAgent Server (Port 7061)               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐ │
│  │ AI Debate    │  │ LLMsVerifier │  │ Dynamic Discovery│ │
│  │ 25 LLMs      │  │ Scoring      │  │ Zen Models       │ │
│  └──────────────┘  └──────────────┘  └──────────────────┘ │
│  ┌────────────────────────────────────────────────────────┐ │
│  │            Protocol Endpoints                           │ │
│  │  MCP │ ACP │ LSP │ Embeddings │ Vision │ Cognee       │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────┬────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────┐
│                    Core Services                             │
│  ┌──────────┐  ┌────────┐  ┌─────────┐  ┌────────────┐   │
│  │PostgreSQL│  │ Redis  │  │ Cognee  │  │  ChromaDB  │   │
│  │  (5432)  │  │ (6379) │  │ (8000)  │  │   (8000)   │   │
│  └──────────┘  └────────┘  └─────────┘  └────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 📚 Documentation

Comprehensive documentation has been created:

### New Documentation
- **SERVICE_MANAGEMENT.md** - Complete service management guide
  - Quick start
  - All service scripts
  - Service architecture (15+ services)
  - Container management
  - Troubleshooting
  - Advanced configuration

### Service Details
- **Core Services:** PostgreSQL, Redis, Cognee, ChromaDB
- **Messaging:** Kafka, RabbitMQ, Zookeeper
- **Monitoring:** Prometheus, Grafana, Loki, Alertmanager
- **Integration:** Weaviate, Neo4j, LangChain, LlamaIndex, Guidance, LMQL
- **Big Data (Optional):** Flink, MinIO, Qdrant, Iceberg, Spark
- **Security (Optional):** Vault, OWASP ZAP

---

## 🔧 Environment Setup

### Required Environment Variables

```bash
# HelixAgent API
export HELIXAGENT_API_KEY="your-api-key-here"

# PostgreSQL (defaults work for local)
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=helixagent
export DB_PASSWORD=helixagent123
export DB_NAME=helixagent_db

# Redis (defaults work for local)
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=helixagent123

# Cognee (defaults)
export COGNEE_AUTH_EMAIL=admin@helixagent.ai
export COGNEE_AUTH_PASSWORD=HelixAgentPass123

# LLM Provider API Keys (for full functionality)
export CLAUDE_API_KEY="your-claude-key"
export DEEPSEEK_API_KEY="your-deepseek-key"
export GEMINI_API_KEY="your-gemini-key"
# ... etc
```

### Configuration Files

All configurations are in `~/helixagent-configs/`:
- `opencode.json` - OpenCode CLI agent configuration
- `crush.json` - Crush CLI agent configuration

---

## 📦 What's Been Committed

All changes are ready to be committed:

### New Files
1. `scripts/start-all-services.sh` - Comprehensive service startup (6.0KB)
2. `scripts/stop-all-services.sh` - Service shutdown (2.9KB)
3. `scripts/verify-services.sh` - Service verification (4.0KB)
4. `docs/SERVICE_MANAGEMENT.md` - Complete service documentation (16KB)
5. `READY_FOR_TESTING.md` - This file (testing guide)

### Modified Files
None - All new additions

---

## ✅ Final Checklist

Before starting manual testing:

- [x] HelixAgent binary built (`make build`)
- [x] OpenCode configuration exported to `~/helixagent-configs/opencode.json`
- [x] Crush configuration exported to `~/helixagent-configs/crush.json`
- [x] Service management scripts created and executable
- [x] Documentation complete (SERVICE_MANAGEMENT.md)
- [x] Core services running (PostgreSQL, Redis, Cognee, ChromaDB)
- [x] Additional services starting (Kafka, Prometheus, etc.)
- [ ] HelixAgent server started (`make run-dev`)
- [ ] API health check passing
- [ ] OpenCode integration tested
- [ ] Crush integration tested

---

## 🎯 Next Steps

1. **Wait for services to complete startup** (check with `./scripts/verify-services.sh`)
2. **Start HelixAgent server** (`make run-dev`)
3. **Test API endpoints** (health, verification, debates)
4. **Test OpenCode integration** (using exported config)
5. **Test Crush integration** (using exported config)
6. **Run validation challenges** (ensure 100% pass rate)

---

## 🐛 Troubleshooting

### Services not starting?
```bash
# Check logs
podman-compose logs -f helixagent-postgres
podman-compose logs -f helixagent-redis

# Verify script
./scripts/verify-services.sh
```

### Port conflicts?
```bash
# Check what's using ports
ss -tulpn | grep -E '5432|6379|8000|7061'
```

### Need to restart?
```bash
# Full restart
./scripts/stop-all-services.sh
./scripts/start-all-services.sh
./scripts/verify-services.sh
```

---

**System Status:** ✅ READY FOR MANUAL TESTING
**Next Action:** Start HelixAgent server and begin testing!
