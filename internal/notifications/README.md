# Notifications System

The `notifications` package provides a multi-channel real-time notification system supporting SSE (Server-Sent Events), WebSocket, Webhooks, and HTTP Polling for task progress updates and event broadcasting.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Notification Hub                              │
│  (Central coordinator for all notification channels)            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ SSE Manager  │  │  WebSocket   │  │  Webhook Dispatcher  │  │
│  │              │  │   Server     │  │                      │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│         │                 │                      │              │
│         ▼                 ▼                      ▼              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Polling Store                          │  │
│  │  (Fallback for clients that can't use real-time)         │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### SSE Manager (`sse_manager.go`)

Server-Sent Events for efficient unidirectional streaming:

```go
manager := notifications.NewSSEManager(logger)

// Register a client for task events
client := make(chan []byte, 100)
manager.RegisterClient(taskID, client)
defer manager.UnregisterClient(taskID, client)

// Broadcast events
manager.Broadcast(taskID, []byte(`{"event":"progress","data":{"percent":50}}`))
```

### WebSocket Server (`websocket_server.go`)

Bidirectional real-time communication:

```go
server := notifications.NewWebSocketServer(logger, upgrader)

// Handle WebSocket connections
server.HandleConnection(w, r, taskID)

// Broadcast to all connected clients
server.BroadcastToTask(taskID, message)
```

### Webhook Dispatcher (`webhook_dispatcher.go`)

HTTP callbacks for external integrations:

```go
dispatcher := notifications.NewWebhookDispatcher(logger, httpClient, config)

// Register webhook endpoint
dispatcher.Register(taskID, "https://example.com/webhook", secret)

// Dispatch event (with retry logic)
dispatcher.Dispatch(ctx, taskID, event, payload)
```

Features:
- HMAC signature verification
- Exponential backoff retry
- Configurable timeout and retry limits
- Dead letter queue for failed deliveries

### Polling Store (`polling_store.go`)

Fallback for environments without real-time support:

```go
store := notifications.NewPollingStore(config, logger)

// Store event for polling
store.Store(taskID, event)

// Client polls for events
events, lastSeq := store.Poll(taskID, afterSequence)
```

### Notification Hub (`hub.go`)

Central coordinator for all channels:

```go
hub := notifications.NewHub(sseManager, wsServer, webhookDispatcher, pollingStore, logger)

// Publish to all channels
hub.Publish(ctx, taskID, event, payload)

// Register client for specific channel
hub.RegisterSSE(taskID, client)
hub.RegisterWebSocket(taskID, conn)
hub.RegisterWebhook(taskID, url, secret)
```

## Event Types

| Event | Description |
|-------|-------------|
| `task.created` | Task has been created |
| `task.queued` | Task added to queue |
| `task.started` | Task execution started |
| `task.progress` | Progress update (includes percent and message) |
| `task.heartbeat` | Task is still alive |
| `task.checkpoint` | Checkpoint saved |
| `task.completed` | Task finished successfully |
| `task.failed` | Task execution failed |
| `task.stuck` | Task detected as stuck |
| `task.cancelled` | Task was cancelled |

## Event Format

### SSE Format
```
event: task.progress
data: {"task_id":"abc123","percent":50,"message":"Processing..."}

```

### WebSocket Format
```json
{
    "type": "task.progress",
    "task_id": "abc123",
    "timestamp": "2024-01-15T10:30:00Z",
    "data": {
        "percent": 50,
        "message": "Processing..."
    }
}
```

### Webhook Format
```
POST /webhook HTTP/1.1
Content-Type: application/json
X-Webhook-Signature: sha256=...
X-Task-ID: abc123
X-Event-Type: task.progress

{
    "task_id": "abc123",
    "event_type": "task.progress",
    "timestamp": "2024-01-15T10:30:00Z",
    "data": {
        "percent": 50,
        "message": "Processing..."
    }
}
```

## CLI Subpackage

The `notifications/cli` subpackage provides terminal-based notification rendering:

```go
renderer := cli.NewProgressRenderer(os.Stdout)
renderer.RenderProgress(taskID, 50, "Processing...")
renderer.RenderComplete(taskID, "Done!")
```

## Files

| File | Description |
|------|-------------|
| `hub.go` | Central notification coordinator |
| `sse_manager.go` | SSE client management and broadcasting |
| `websocket_server.go` | WebSocket connection handling |
| `webhook_dispatcher.go` | Webhook delivery with retry logic |
| `polling_store.go` | Event storage for HTTP polling |
| `cli/` | Terminal notification rendering |

## Testing

```bash
go test -v ./internal/notifications/...
```

Tests cover:
- SSE client registration and broadcasting
- WebSocket connection lifecycle
- Webhook delivery and retry logic
- Polling store event sequencing
- Hub coordination across channels
- Concurrent access patterns
