# Module 17: Remote Container Distribution

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 17: Remote Container Distribution
- Duration: 60 minutes
- Automatic Container Orchestration Across Hosts

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Configure remote container distribution via `Containers/.env`
- Understand the mandatory container orchestration flow
- Deploy all services to remote hosts automatically
- Verify health checks against remote endpoints

---

## Slide 3: Container Orchestration Philosophy

**CRITICAL: HelixAgent handles ALL container orchestration automatically**

**FORBIDDEN:**
- Manual `docker start/stop/restart/rm` commands
- Manual `docker-compose up/down` commands
- Manual SSH to remote hosts for container deployment
- `make test-infra-start` or similar manual targets

**ONLY acceptable workflow:**
1. `make build` -- Build the HelixAgent binary
2. `./bin/helixagent` -- Run it (ALL orchestration happens automatically)
3. The binary reads `Containers/.env` and orchestrates everything

---

## Slide 4: Centralized Container Management

**Architecture:**

```
cmd/helixagent/main.go
    |
    v
globalContainerAdapter  (internal/adapters/containers/adapter.go)
    |
    v
digital.vasic.containers  (Containers/ submodule)
    |
    +-- Runtime Detection (Docker / Podman / K8s)
    +-- Compose Orchestrator
    +-- Health Checker
    +-- Remote Distribution (SSH)
    +-- Lifecycle Management
```

*All container operations go through the adapter. No direct exec.Command to docker/podman.*

---

## Slide 5: Local vs Remote Mode

**Determined by `Containers/.env` (NOT project root `.env`):**

```bash
# LOCAL MODE (default)
# Containers/.env:
CONTAINERS_REMOTE_ENABLED=false
# Result: All containers start on localhost

# REMOTE MODE
# Containers/.env:
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_HOST_1=user@server1.example.com
CONTAINERS_REMOTE_HOST_2=user@server2.example.com
# Result: ALL containers distributed to remote hosts
# NO local containers started
```

**No mixed mode: either ALL local or ALL remote**

---

## Slide 6: Container Orchestration Flow

**The 5-Step Boot Sequence:**

```
Step 1: HelixAgent boots, initializes Container adapter
    |
Step 2: Adapter reads Containers/.env
    |
Step 3: Based on CONTAINERS_REMOTE_ENABLED:
    |     true  --> SSH to remote hosts, deploy all containers
    |     false --> Start all containers locally
    |
Step 4: Health checks against all endpoints (TCP/HTTP)
    |     Local: localhost:port
    |     Remote: remote-host:port
    |
Step 5: Required services failing health check
        --> Boot failure in strict mode
```

---

## Slide 7: Remote Distribution via SSH

**How containers reach remote hosts:**

- SSH-based deployment using key authentication (no interactive prompts)
- SSH keys must be configured via `ssh-add` or environment variables
- Container images transferred and started on remote hosts
- All secrets provided via environment variables or `.env` files
- Fully automated through the Containers module's SSH executor

```bash
# Pre-requisites for remote distribution:
# 1. SSH key-based auth configured
# 2. Docker/Podman installed on remote host
# 3. Containers/.env configured with remote hosts
```

---

## Slide 8: Service Overrides

**Configuring individual services:**

```bash
# Override service-specific settings
SVC_POSTGRESQL_HOST=db.example.com
SVC_POSTGRESQL_PORT=5432
SVC_REDIS_HOST=cache.example.com
SVC_REDIS_PORT=6379
SVC_REDIS_REMOTE=true

# Pattern: SVC_<SERVICE>_<FIELD>
# These override the default service configuration
```

---

## Slide 9: Health Checking

**BootManager health verification:**

```go
// HealthChecker performs TCP/HTTP checks with retries
// For local: checks localhost:<port>
// For remote: checks <remote-host>:<port>

// Required services (boot fails if unhealthy):
// - PostgreSQL
// - Redis
// - ChromaDB

// Optional services (degraded but operational):
// - Cognee
// - Ollama
// - MCP servers
```

---

## Slide 10: Mandatory Container Rebuild

**After code changes affecting containerized components:**

```bash
# 1. Rebuild affected images
make docker-build
# or
make container-build

# 2. Restart HelixAgent (automatic redeployment)
./bin/helixagent

# 3. If using remote distribution:
# Containers are automatically re-distributed
# No manual SSH required

# WARNING: Failure to rebuild after code changes
# results in outdated code running in production
```

---

## Slide 11: Hands-On Lab

**Lab Exercise 17.1: Remote Container Deployment**

Tasks:
1. Examine `Containers/.env` configuration
2. Configure for local mode and observe boot sequence
3. Configure service overrides for a specific service
4. Verify health checks pass for all required services
5. (Advanced) Configure remote distribution to a test host

Time: 30 minutes

---

## Slide 12: Module Summary

**Key Takeaways:**

- ALL container orchestration is automatic via HelixAgent binary
- `Containers/.env` controls local vs remote deployment
- No mixed mode: all local OR all remote
- SSH-based remote distribution with key authentication
- Health checks verify all services (local or remote)
- Required services must pass or boot fails (strict mode)
- Always rebuild containers after code changes

**Next: Module 18 - HelixMemory Cognitive Engine**

---

## Speaker Notes

### Slide 3 Notes
This is the most important slide. The constitution FORBIDS manual container manipulation.
Drill this into students. The HelixAgent binary is the sole authority for container lifecycle.

### Slide 5 Notes
Emphasize: Containers/.env (inside the Containers/ submodule directory), NOT the project
root .env file. This is a common mistake.

### Slide 6 Notes
Walk through the boot sequence step by step. Show the server log output during boot
to demonstrate each phase.
