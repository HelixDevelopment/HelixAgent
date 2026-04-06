# Lab 20: Remote Container Distribution

## Objective
Configure and verify the container orchestration flow, including remote distribution.

## Prerequisites
- HelixAgent built (`make build`)
- Docker or Podman installed
- (Optional) SSH access to a remote host for remote distribution exercise

## Exercise 1: Examine Container Configuration

```bash
# View the container orchestration config
cat Containers/.env

# Key variables to look for:
# CONTAINERS_REMOTE_ENABLED=false (local mode)
# CONTAINERS_REMOTE_HOST_1= (empty = local only)
```

**Expected:** File exists with local mode configuration.

## Exercise 2: Observe Boot Sequence

```bash
# Start HelixAgent and observe container orchestration in log
./bin/helixagent 2>&1 | head -100

# Look for:
# - "Initializing container adapter"
# - "Reading Containers/.env"
# - "Starting containers locally" or "Distributing to remote hosts"
# - "Health check: PostgreSQL ... OK"
# - "Health check: Redis ... OK"
```

**Expected:** All required services start and pass health checks.

## Exercise 3: Service Override Configuration

```bash
# Override a specific service endpoint
export SVC_POSTGRESQL_HOST=localhost
export SVC_POSTGRESQL_PORT=15432
export SVC_REDIS_HOST=localhost
export SVC_REDIS_PORT=16379

# Start HelixAgent with overrides
./bin/helixagent

# Verify overridden endpoints are used in health checks
```

**Expected:** Health checks target the overridden host/port values.

## Exercise 4: Verify Health Checks

```bash
# After HelixAgent boots, check health status
curl http://localhost:7061/health | python3 -m json.tool

# Check infrastructure-specific health
curl http://localhost:7061/v1/monitoring/status | python3 -m json.tool
```

**Expected:** All required services report healthy.

## Exercise 5: Run Container Challenge

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  ./challenges/scripts/container_lazy_loading_challenge.sh
```

**Expected:** All 13 tests pass.

## Assessment Questions
1. Why does HelixAgent read `Containers/.env` instead of the project root `.env`?
2. What happens when `CONTAINERS_REMOTE_ENABLED=true` but no remote hosts are configured?
3. Why is mixed mode (some local, some remote) forbidden?
4. What is the difference between required and optional services during boot?
