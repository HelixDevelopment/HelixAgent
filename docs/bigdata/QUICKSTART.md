# Big Data Quick Start Guide

Get the HelixAgent Big Data stack running in under 5 minutes.

## Prerequisites

- Docker 20.10+ or Podman 4.0+
- Docker Compose v2 or podman-compose
- 8GB RAM minimum (16GB recommended)
- 20GB disk space

## Step 1: Start Services

```bash
cd /path/to/HelixAgent

# Start core Big Data services
docker-compose -f docker-compose.bigdata.yml up -d

# Wait for services to be ready (about 30 seconds)
sleep 30
```

## Step 2: Verify Services

```bash
# Check all services are running
docker-compose -f docker-compose.bigdata.yml ps

# Expected output:
# NAME                    STATUS
# helixagent-minio        running (healthy)
# helixagent-flink-jm     running (healthy)
# helixagent-flink-tm     running
# helixagent-qdrant       running (healthy)
# helixagent-iceberg      running (healthy)
```

## Step 3: Access Web UIs

| Service | URL | Credentials |
|---------|-----|-------------|
| MinIO Console | http://localhost:9001 | minioadmin / minioadmin123 |
| Flink Dashboard | http://localhost:8082 | - |
| Qdrant Dashboard | http://localhost:6333/dashboard | - |

## Step 4: Test Connections

### Test MinIO

```bash
# Using curl
curl -s http://localhost:9000/minio/health/live

# Or using mc client
mc alias set helixagent http://localhost:9000 minioadmin minioadmin123
mc ls helixagent
```

### Test Flink

```bash
curl -s http://localhost:8082/overview | jq
```

### Test Qdrant

```bash
curl -s http://localhost:6333/collections | jq
```

### Test Iceberg

```bash
curl -s http://localhost:8181/v1/namespaces | jq
```

## Step 5: Run Integration Tests

```bash
# Run the Big Data challenge suite
./challenges/scripts/run_bigdata_challenges.sh
```

## Step 6: Use in Code

```go
package main

import (
    "context"
    "dev.helix.agent/internal/storage/minio"
    "dev.helix.agent/internal/vectordb/qdrant"
    "dev.helix.agent/internal/streaming/flink"
    "dev.helix.agent/internal/lakehouse/iceberg"
)

func main() {
    ctx := context.Background()

    // MinIO - Object Storage
    minioClient, _ := minio.NewClient(minio.DefaultConfig(), nil)
    minioClient.Connect(ctx)
    defer minioClient.Close()

    // Qdrant - Vector Database
    qdrantClient, _ := qdrant.NewClient(qdrant.DefaultConfig(), nil)
    qdrantClient.Connect(ctx)
    defer qdrantClient.Close()

    // Flink - Stream Processing
    flinkClient, _ := flink.NewClient(flink.DefaultConfig(), nil)
    flinkClient.Connect(ctx)
    defer flinkClient.Close()

    // Iceberg - Data Lakehouse
    icebergClient, _ := iceberg.NewClient(iceberg.DefaultConfig(), nil)
    icebergClient.Connect(ctx)
    defer icebergClient.Close()

    // All connected! Start using the clients...
}
```

## Stopping Services

```bash
# Stop all services
docker-compose -f docker-compose.bigdata.yml down

# Stop and remove volumes (WARNING: deletes all data)
docker-compose -f docker-compose.bigdata.yml down -v
```

## Troubleshooting

### Services Won't Start

```bash
# Check Docker resources
docker system df
docker system prune -f

# Increase Docker memory limit to at least 8GB
```

### Connection Refused

```bash
# Check if services are running
docker-compose -f docker-compose.bigdata.yml ps

# Check service logs
docker-compose -f docker-compose.bigdata.yml logs minio
docker-compose -f docker-compose.bigdata.yml logs flink-jobmanager
docker-compose -f docker-compose.bigdata.yml logs qdrant
docker-compose -f docker-compose.bigdata.yml logs iceberg-rest
```

### Port Conflicts

If you have other services on the default ports, edit `.env` or use environment variables:

```bash
MINIO_PORT=19000 FLINK_PORT=18082 QDRANT_PORT=16333 \
docker-compose -f docker-compose.bigdata.yml up -d
```

## Next Steps

1. Read the full [Big Data Integration Guide](README.md)
2. Explore the [Flink Documentation](flink.md)
3. Learn about [MinIO Storage Patterns](minio.md)
4. Understand [Qdrant Vector Search](qdrant.md)
5. Master [Iceberg Analytics](iceberg.md)
