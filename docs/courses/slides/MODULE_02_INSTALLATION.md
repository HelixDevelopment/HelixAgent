# Module 2: Installation and Setup

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 2: Installation and Setup
- Duration: 60 minutes
- Hands-On Lab Included

---

## Slide 2: Learning Objectives

**By the end of this module, you will be able to:**

- Set up a complete development environment
- Install HelixAgent using Docker (recommended)
- Install HelixAgent from source
- Verify successful installation
- Run initial health checks

---

## Slide 3: Prerequisites Overview

**Before You Begin:**

| Requirement | Minimum Version |
|-------------|-----------------|
| Go | 1.23+ |
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| Git | 2.30+ |
| RAM | 8GB recommended |
| Disk Space | 10GB free |

---

## Slide 4: Go Installation

**Installing Go 1.23+:**

```bash
# Linux (using package manager)
sudo apt install golang  # Debian/Ubuntu
sudo dnf install golang  # Fedora

# macOS
brew install go

# Verify installation
go version
# Expected: go version go1.23.x ...
```

*Download from https://go.dev/dl/ for latest version*

---

## Slide 5: Docker Installation

**Installing Docker:**

```bash
# Linux
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# macOS/Windows
# Download Docker Desktop from docker.com

# Verify installation
docker --version
docker compose version
```

---

## Slide 6: IDE Recommendations

**Recommended Development Tools:**

- **VS Code** with Go extension
  - Debugging support
  - IntelliSense
  - Test integration

- **GoLand** (JetBrains)
  - Full Go IDE
  - Advanced refactoring
  - Integrated debugging

- **Vim/Neovim** with gopls
  - Lightweight option
  - LSP support

---

## Slide 7: Installation Methods

**Choose Your Path:**

| Method | Recommended For | Time |
|--------|-----------------|------|
| Docker | Beginners, Quick Start | 10 min |
| Source | Developers, Contributors | 20 min |
| Podman | Container alternatives | 15 min |

*Docker recommended for first-time users*

---

## Slide 8: Docker Quick Start

**Fastest Way to Get Running:**

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Start core services
docker-compose up -d

# Verify containers are running
docker-compose ps
```

---

## Slide 9: Docker Compose Profiles

**Available Service Profiles:**

```bash
# Core services (PostgreSQL, Redis, Cognee)
docker-compose up -d

# Add AI services (Ollama)
docker-compose --profile ai up -d

# Add monitoring (Prometheus, Grafana)
docker-compose --profile monitoring up -d

# Full stack deployment
docker-compose --profile full up -d
```

---

## Slide 10: Docker Services Overview

**Core Services Started:**

| Service | Port | Purpose |
|---------|------|---------|
| helixagent | 8080 | Main API |
| postgres | 5432 | Database |
| redis | 6379 | Cache |
| cognee | 8001 | Knowledge Graph |
| chromadb | 8000 | Vector DB |

---

## Slide 11: Monitoring Services

**Optional Monitoring Stack:**

| Service | Port | Purpose |
|---------|------|---------|
| prometheus | 9090 | Metrics collection |
| grafana | 3000 | Dashboards |
| ollama | 11434 | Local LLM |

```bash
docker-compose --profile monitoring up -d
```

---

## Slide 12: Manual Installation - Clone

**Step 1: Clone Repository**

```bash
# Clone the repository
git clone https://dev.helix.agent.git

# Navigate to project directory
cd helixagent

# Check out main branch
git checkout main
```

---

## Slide 13: Manual Installation - Dependencies

**Step 2: Install Dependencies**

```bash
# Download Go modules
go mod download

# Install development tools
make install-deps

# Verify tools installed
which golangci-lint
which gosec
```

---

## Slide 14: Manual Installation - Build

**Step 3: Build HelixAgent**

```bash
# Standard build
make build

# Debug build (with symbols)
make build-debug

# Verify build
./bin/helixagent --version
```

---

## Slide 15: Configuration Setup

**Step 4: Configure Environment**

```bash
# Copy example environment file
cp .env.example .env

# Edit with your API keys
nano .env

# Required variables:
# - PORT=8080
# - DB_HOST=localhost
# - REDIS_HOST=localhost
# - CLAUDE_API_KEY=sk-...
```

---

## Slide 16: Running HelixAgent

**Step 5: Start the Server**

```bash
# Production mode
make run

# Development mode (with debug output)
make run-dev

# Or directly
GIN_MODE=debug ./bin/helixagent
```

---

## Slide 17: Podman Alternative

**Using Podman Instead of Docker:**

```bash
# Enable Podman socket
systemctl --user enable --now podman.socket

# Use podman-compose
pip install podman-compose
podman-compose up -d

# Or use container-runtime script
source scripts/container-runtime.sh
./scripts/container-runtime.sh start
```

---

## Slide 18: Verification Checklist

**Verify Your Installation:**

```bash
# Check API is responding
curl http://localhost:7061/health

# Check providers status
curl http://localhost:7061/v1/providers

# Check protocol servers
curl http://localhost:7061/v1/protocols/servers

# Expected: JSON responses with status information
```

---

## Slide 19: Health Check Response

**Expected Health Response:**

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "connected",
    "redis": "connected",
    "providers": {
      "claude": "available",
      "gemini": "available",
      "deepseek": "available"
    }
  }
}
```

---

## Slide 20: Common Installation Issues

**Troubleshooting:**

| Issue | Solution |
|-------|----------|
| Port 8080 in use | Change PORT in .env |
| Docker permission denied | Add user to docker group |
| Go modules fail | Check GOPROXY settings |
| Database connection failed | Verify PostgreSQL running |
| Redis connection failed | Verify Redis running |

---

## Slide 21: Development Workflow

**Daily Development Commands:**

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Run with hot reload (using air)
air
```

---

## Slide 22: Hands-On Lab

**Lab Exercise 2.1: Complete Installation**

Tasks:
1. Choose installation method (Docker recommended)
2. Complete installation steps
3. Start all services
4. Verify API is responding
5. Check provider health status

Time: 30 minutes

---

## Slide 23: Lab Verification

**Verify Your Setup:**

```bash
# All containers running?
docker-compose ps

# API responding?
curl http://localhost:7061/health

# Can make API calls?
curl -X POST http://localhost:7061/v1/completion \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, world!"}'
```

---

## Slide 24: Next Steps

**After Installation:**

1. Configure API keys for your providers
2. Test basic API functionality
3. Explore the configuration options
4. Set up monitoring dashboard

**Next: Module 3 - Configuration**

---

## Slide 25: Module Summary

**Key Takeaways:**

- Docker is the fastest way to get started
- Go 1.23+ required for source installation
- Core services: PostgreSQL, Redis, HelixAgent
- Multiple profiles for different use cases
- Health endpoints for verification
- Development tools for code quality

---

## Speaker Notes

### Slide 8 Notes
Emphasize that Docker is the recommended approach for beginners. It handles all dependencies and configuration automatically.

### Slide 14 Notes
Walk through the Makefile targets. Explain that `make build` creates an optimized binary while `make build-debug` includes debugging symbols.

### Slide 18 Notes
Have participants follow along and verify their installation. This is a critical checkpoint before moving to configuration.

### Slide 20 Notes
Go through each common issue. The most frequent problems are Docker permissions on Linux and port conflicts on all platforms.
