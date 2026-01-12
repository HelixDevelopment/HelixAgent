# Background Command Execution System

Comprehensive background command execution system for HelixAgent that enables parallel task execution triggered by Tooling and AI Debate Team, with real-time progress monitoring, resource tracking, and full observability.

## Features

- **Adaptive Concurrency**: Auto-detect based on CPU cores, memory, and system load
- **PostgreSQL Persistence**: Full task state persistence for pause/resume/recovery
- **All Notification Methods**: Webhooks, SSE, WebSocket, Polling API
- **Full Observability**: Prometheus metrics, resource tracking, stuck detection
- **Long-Running Support**: Endless process monitoring with graceful shutdown
- **Resource Monitoring**: CPU, memory, network, filesystem per process
- **CLI Rendering**: Progress bars, status tables, resource gauges for AI coding agents

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer                                 │
│  POST /v1/tasks  │  GET /v1/tasks/:id  │  SSE/WebSocket/Webhooks │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Background Task Handler                       │
│         internal/handlers/background_task_handler.go             │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Task Queue    │  │   Worker Pool   │  │ Notification Hub │
│  (PostgreSQL)   │  │   (Adaptive)    │  │  (SSE/WS/Hook)  │
└─────────────────┘  └─────────────────┘  └─────────────────┘
          │                   │                   │
          ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ Resource Monitor│  │ Stuck Detector  │  │   CLI Renderer  │
│   (gopsutil)    │  │  (Heartbeat)    │  │ (Progress Bars) │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Quick Start

### Create a Background Task

```bash
curl -X POST http://localhost:7061/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task_type": "command",
    "task_name": "My Background Task",
    "priority": "normal",
    "payload": {"command": "echo Hello World"},
    "config": {
      "timeout_seconds": 300,
      "allow_cancel": true
    },
    "notification_config": {
      "sse": {"enabled": true},
      "websocket": {"enabled": true}
    }
  }'
```

### Check Task Status

```bash
curl http://localhost:7061/v1/tasks/{task_id}/status
```

### Stream Events (SSE)

```bash
curl -N http://localhost:7061/v1/tasks/{task_id}/events
```

### Cancel a Task

```bash
curl -X POST http://localhost:7061/v1/tasks/{task_id}/cancel
```

## API Reference

### Task Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/tasks` | Create a new background task |
| GET | `/v1/tasks` | List tasks with filters |
| GET | `/v1/tasks/:id` | Get task details |
| GET | `/v1/tasks/:id/status` | Get task status |
| GET | `/v1/tasks/:id/logs` | Get task logs (tail) |
| GET | `/v1/tasks/:id/resources` | Get resource snapshots |
| GET | `/v1/tasks/:id/events` | SSE event stream |
| GET | `/v1/tasks/:id/analyze` | Analyze task for stuck indicators |
| POST | `/v1/tasks/:id/pause` | Pause a running task |
| POST | `/v1/tasks/:id/resume` | Resume a paused task |
| POST | `/v1/tasks/:id/cancel` | Cancel a task |
| DELETE | `/v1/tasks/:id` | Delete a task |

### Queue Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/tasks/queue/stats` | Get queue statistics |

### Webhook Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/webhooks` | Register a webhook |
| GET | `/v1/webhooks` | List registered webhooks |
| DELETE | `/v1/webhooks/:id` | Delete a webhook |

### WebSocket Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/ws/tasks/:id` | WebSocket connection for task events |

## Task States

```
pending → queued → running → completed
                          → failed
                          → stuck
                          → cancelled
                          → dead_letter
              ↓
           paused → running (resume)
```

| State | Description |
|-------|-------------|
| `pending` | Task created, not yet queued |
| `queued` | Task in queue, waiting for worker |
| `running` | Task being executed by worker |
| `paused` | Task paused, can be resumed |
| `completed` | Task finished successfully |
| `failed` | Task failed with error |
| `stuck` | Task detected as stuck (no heartbeat/activity) |
| `cancelled` | Task cancelled by user |
| `dead_letter` | Task moved to dead-letter queue after max retries |

## Task Priority

| Priority | Weight | Description |
|----------|--------|-------------|
| `critical` | 4 | Immediate execution, preempts other tasks |
| `high` | 3 | High priority, processed before normal |
| `normal` | 2 | Default priority |
| `low` | 1 | Low priority, processed when queue is empty |
| `background` | 0 | Lowest priority, background processing |

## Resource Monitoring

The system tracks per-task resource consumption using gopsutil:

- **CPU**: Percentage utilization per process
- **Memory**: RSS bytes, percentage of total
- **I/O**: Read/write bytes
- **Network**: Bytes sent/received
- **File Descriptors**: Open FD count
- **Threads**: Thread count
- **Process State**: Running, sleeping, zombie, etc.

### Resource Requirements

Tasks can specify resource requirements:

```json
{
  "required_cpu_cores": 2,
  "required_memory_mb": 512
}
```

Workers will only pick up tasks if sufficient resources are available.

## Stuck Detection

The stuck detector identifies problematic tasks using multiple algorithms:

| Algorithm | Description | Threshold |
|-----------|-------------|-----------|
| Heartbeat Timeout | No heartbeat received | 5 minutes default |
| CPU Freeze | Zero CPU activity | Configurable |
| Memory Leak | Monotonic memory increase | Trend analysis |
| I/O Starvation | No I/O activity | Configurable |

### Endless Process Support

For long-running/endless processes, set:

```json
{
  "config": {
    "endless": true,
    "stuck_threshold_secs": 0
  }
}
```

This disables stuck detection for the task.

## Notification System

### SSE (Server-Sent Events)

```bash
curl -N -H "Accept: text/event-stream" \
  http://localhost:7061/v1/tasks/{task_id}/events
```

Events include: `status_change`, `progress`, `log`, `resource`, `error`, `completed`

### WebSocket

```javascript
const ws = new WebSocket('ws://localhost:7061/v1/ws/tasks/{task_id}');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data);
};
```

### Webhooks

Register a webhook to receive POST notifications:

```bash
curl -X POST http://localhost:7061/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-server.com/webhook",
    "events": ["completed", "failed", "stuck"],
    "secret": "your-hmac-secret"
  }'
```

Webhook payloads are signed with HMAC-SHA256 in the `X-Webhook-Signature` header.

### Polling

```bash
curl "http://localhost:7061/v1/tasks/events?limit=10&since=2024-01-01T00:00:00Z"
```

## CLI Rendering

The system supports rich CLI output for AI coding agents:

### Supported Clients

| Client | Detection | Features |
|--------|-----------|----------|
| OpenCode | User-Agent, ENV | Full Unicode, ANSI colors |
| Crush | User-Agent | Full Unicode, ANSI colors |
| HelixCode | User-Agent | Full Unicode, ANSI colors |
| Kilo Code | User-Agent | Full Unicode, ANSI colors |
| Unknown | Fallback | ASCII-only, no colors |

### Output Components

**Progress Bar**:
```
[████████████████░░░░░░░░░░░░░░░░] 50% (5/10 steps)
```

**Status Table**:
```
┌─────────────┬────────────┬──────────┐
│ Task ID     │ Status     │ Progress │
├─────────────┼────────────┼──────────┤
│ abc123      │ running    │ 75%      │
│ def456      │ completed  │ 100%     │
└─────────────┴────────────┴──────────┘
```

**Resource Gauge**:
```
CPU:    [████░░░░░░] 40%
Memory: [██████░░░░] 256MB / 1GB
```

## Prometheus Metrics

All metrics are prefixed with `helixagent_background_`:

### Worker Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `workers_active` | Gauge | Currently active workers |
| `workers_idle` | Gauge | Idle workers |
| `scaling_events_total` | Counter | Worker scaling events (up/down) |

### Task Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `tasks_total` | Counter | Total tasks by type and status |
| `tasks_in_queue` | Gauge | Tasks in queue by priority |
| `task_duration_seconds` | Histogram | Task execution duration |
| `task_retries_total` | Counter | Task retry count |
| `stuck_tasks_total` | Counter | Tasks detected as stuck |
| `dead_letter_tasks_total` | Counter | Tasks moved to dead-letter |

### Resource Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `task_cpu_percent` | Gauge | CPU usage per task |
| `task_memory_bytes` | Gauge | Memory usage per task |

### Notification Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `notifications_sent_total` | Counter | Notifications sent by type |
| `notification_errors_total` | Counter | Notification failures by type |

## Configuration

```yaml
background:
  worker_pool:
    min_workers: 2
    max_workers: 16  # Or CPU_CORES * 2
    scale_up_threshold: 0.7
    scale_down_threshold: 0.3
    scale_interval: 30s
    idle_timeout: 5m
    max_cpu_percent: 80
    max_memory_percent: 80

  queue:
    poll_interval: 1s
    visibility_timeout: 30s
    max_retries: 3
    retry_backoff: 60s

  stuck_detection:
    heartbeat_interval: 10s
    heartbeat_timeout: 5m
    check_interval: 30s
    cpu_threshold: 0.1

  notifications:
    webhook:
      max_retries: 5
      retry_backoff: 1s
      timeout: 30s
    sse:
      heartbeat_interval: 30s
      buffer_size: 100
    websocket:
      ping_interval: 54s
      read_timeout: 60s
```

## Database Schema

The system uses PostgreSQL for persistence:

### Tables

- `background_tasks` - Main task table with status, progress, config
- `task_resource_snapshots` - Resource usage snapshots per task
- `task_execution_history` - Event log for task lifecycle
- `background_tasks_dead_letter` - Failed tasks after max retries

See `internal/database/migrations/011_background_tasks.sql` for full schema.

## Package Structure

```
internal/
├── background/
│   ├── interfaces.go       # Core interfaces
│   ├── task_queue.go       # PostgreSQL-backed queue
│   ├── worker_pool.go      # Adaptive worker pool
│   ├── resource_monitor.go # gopsutil resource tracking
│   ├── stuck_detector.go   # Stuck detection algorithms
│   └── metrics.go          # Prometheus metrics
├── notifications/
│   ├── hub.go              # Central event distribution
│   ├── sse_manager.go      # SSE stream management
│   ├── websocket_server.go # WebSocket server
│   ├── webhook_dispatcher.go # Webhook delivery
│   ├── polling_store.go    # Polling event buffer
│   └── cli/
│       ├── types.go        # CLI render types
│       ├── renderer.go     # Progress bars, tables
│       └── detection.go    # Client detection
├── handlers/
│   └── background_task_handler.go # REST API handler
└── models/
    └── background_task.go  # Data models
```

## Challenges

Run the validation challenges:

```bash
# Individual challenges
./challenges/scripts/background_task_queue_challenge.sh
./challenges/scripts/background_worker_pool_challenge.sh
./challenges/scripts/background_resource_monitor_challenge.sh
./challenges/scripts/background_stuck_detection_challenge.sh
./challenges/scripts/background_notifications_challenge.sh
./challenges/scripts/background_endless_process_challenge.sh
./challenges/scripts/background_cli_rendering_challenge.sh
./challenges/scripts/background_full_integration_challenge.sh

# Run all
for f in challenges/scripts/background_*.sh; do bash "$f"; done
```

## Testing

```bash
# Unit tests
go test -v ./internal/background/...
go test -v ./internal/notifications/...
go test -v ./tests/unit/background/...

# Integration tests
go test -v ./tests/integration/background_*

# E2E tests
go test -v ./tests/e2e/background_*
```
