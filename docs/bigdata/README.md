# Big Data Integration Guide

HelixAgent includes comprehensive Big Data technology integrations for scalable data processing, storage, analytics, and vector search capabilities.

## Overview

The Big Data stack consists of four core components:

| Component | Technology | Purpose |
|-----------|------------|---------|
| Stream Processing | Apache Flink | Real-time event processing and analytics |
| Object Storage | MinIO | S3-compatible storage for artifacts and data |
| Vector Database | Qdrant | Similarity search and RAG capabilities |
| Data Lakehouse | Apache Iceberg | Analytics with time-travel and schema evolution |

## Quick Start

### 1. Start Infrastructure

```bash
# Start all Big Data services
docker-compose -f docker-compose.bigdata.yml up -d

# Or with analytics (Superset)
docker-compose -f docker-compose.bigdata.yml -f docker-compose.analytics.yml up -d
```

### 2. Verify Services

```bash
# Check service health
curl http://localhost:9000/minio/health/live  # MinIO
curl http://localhost:8082/overview           # Flink
curl http://localhost:6333/health             # Qdrant
curl http://localhost:8181/v1/config          # Iceberg
```

## Components

### Apache Flink (Stream Processing)

Flink provides real-time stream processing for LLM events, debate rounds, and verification results.

**Configuration:**
```go
import "dev.helix.agent/internal/streaming/flink"

config := flink.DefaultConfig()
config.JobManagerHost = "localhost"
config.WebUIPort = 8082
config.CheckpointEnabled = true
config.StateBackend = "rocksdb"

client, err := flink.NewClient(config, logger)
client.Connect(ctx)
```

**Key Features:**
- Job submission and management via REST API
- Checkpointing with exactly-once semantics
- RocksDB state backend for large state
- Kafka integration for event streaming

**Endpoints:**
- Web UI: `http://localhost:8082`
- REST API: `http://localhost:8082`
- JobManager RPC: `localhost:6123`

### MinIO (Object Storage)

MinIO provides S3-compatible object storage for debate artifacts, model outputs, and logs.

**Configuration:**
```go
import "dev.helix.agent/internal/storage/minio"

config := minio.DefaultConfig()
config.Endpoint = "localhost:9000"
config.AccessKey = "minioadmin"
config.SecretKey = "minioadmin123"

client, err := minio.NewClient(config, logger)
client.Connect(ctx)
```

**Key Features:**
- Bucket management with versioning
- Object lifecycle rules
- Presigned URLs for secure sharing
- Multipart uploads for large files

**Default Buckets:**
- `helixagent-debates`: Debate transcripts and artifacts
- `helixagent-models`: Model configurations and weights
- `helixagent-logs`: Application logs
- `helixagent-data`: General data storage

**Endpoints:**
- API: `http://localhost:9000`
- Console: `http://localhost:9001`

### Qdrant (Vector Database)

Qdrant enables similarity search for RAG, semantic search, and embedding-based retrieval.

**Configuration:**
```go
import "dev.helix.agent/internal/vectordb/qdrant"

config := qdrant.DefaultConfig()
config.Host = "localhost"
config.HTTPPort = 6333

client, err := qdrant.NewClient(config, logger)
client.Connect(ctx)
```

**Key Features:**
- HNSW indexing for fast similarity search
- Multiple distance metrics (cosine, euclidean, dot)
- Payload filtering
- Batch operations and snapshots

**Default Collections:**
- `debate_embeddings`: Debate round embeddings
- `model_embeddings`: Model capability embeddings
- `document_embeddings`: Document chunks for RAG

**Endpoints:**
- HTTP API: `http://localhost:6333`
- gRPC: `localhost:6334`
- Dashboard: `http://localhost:6333/dashboard`

### Apache Iceberg (Data Lakehouse)

Iceberg provides analytics capabilities with time-travel, schema evolution, and efficient queries.

**Configuration:**
```go
import "dev.helix.agent/internal/lakehouse/iceberg"

config := iceberg.DefaultConfig()
config.CatalogURI = "http://localhost:8181"
config.Warehouse = "s3://helixagent-iceberg/warehouse"

client, err := iceberg.NewClient(config, logger)
client.Connect(ctx)
```

**Key Features:**
- REST catalog for table management
- Time-travel queries on historical data
- Schema evolution without rewrites
- Partition pruning for efficient queries

**Default Tables:**
- `helixagent.debates`: Debate history and outcomes
- `helixagent.verifications`: Provider verification results
- `helixagent.metrics`: System metrics and analytics

**Endpoints:**
- REST Catalog: `http://localhost:8181`

## Docker Compose Profiles

The Big Data stack uses profiles for flexible deployment:

```bash
# Core services only
docker-compose -f docker-compose.bigdata.yml up -d

# With Spark for batch processing
docker-compose -f docker-compose.bigdata.yml --profile spark up -d

# With analytics (Superset)
docker-compose -f docker-compose.bigdata.yml -f docker-compose.analytics.yml up -d

# Full stack
docker-compose -f docker-compose.bigdata.yml --profile spark -f docker-compose.analytics.yml up -d
```

## Configuration

All configuration is in `configs/bigdata.yaml`:

```yaml
minio:
  endpoint: "localhost:9000"
  access_key: "${MINIO_ACCESS_KEY:-minioadmin}"
  secret_key: "${MINIO_SECRET_KEY:-minioadmin123}"

flink:
  jobmanager_host: "localhost"
  rest_url: "http://localhost:8082"
  checkpoint_enabled: true

qdrant:
  host: "localhost"
  http_port: 6333
  default_vector_size: 1536

iceberg:
  catalog_uri: "http://localhost:8181"
  warehouse: "s3://helixagent-iceberg/warehouse"
```

## Challenges

Run Big Data integration challenges:

```bash
# Run all challenges
./challenges/scripts/run_bigdata_challenges.sh

# Individual challenges
./challenges/scripts/flink_integration_challenge.sh    # 25 tests
./challenges/scripts/minio_storage_challenge.sh        # 20 tests
./challenges/scripts/qdrant_vector_challenge.sh        # 25 tests
./challenges/scripts/bigdata_pipeline_challenge.sh     # 52 tests
```

## Use Cases

### 1. Debate Analytics Pipeline

```go
// Store debate in MinIO
minioClient.PutObject(ctx, "helixagent-debates", debateID, reader, size)

// Generate embeddings and store in Qdrant
qdrantClient.UpsertPoints(ctx, "debate_embeddings", points)

// Write to Iceberg for analytics
icebergClient.AppendData(ctx, "helixagent.debates", records)

// Stream events via Flink
flinkClient.SubmitJob(ctx, jarID, jobConfig)
```

### 2. Semantic Search for RAG

```go
// Search similar documents
results, err := qdrantClient.Search(ctx, "document_embeddings", vector, 10)

// Retrieve full documents from MinIO
for _, r := range results {
    obj, _ := minioClient.GetObject(ctx, "documents", r.ID)
    // Process document...
}
```

### 3. Provider Verification Analytics

```go
// Query historical verifications with time-travel
table, _ := icebergClient.GetTable(ctx, "helixagent.verifications")
// Query at specific snapshot for reproducibility
```

## Monitoring

### Prometheus Metrics

All components expose Prometheus metrics:

- MinIO: `http://localhost:9000/minio/v2/metrics/cluster`
- Flink: `http://localhost:8082/metrics`
- Qdrant: `http://localhost:6333/metrics`

### Health Checks

```go
// Check all services
minioClient.HealthCheck(ctx)
flinkClient.HealthCheck(ctx)
qdrantClient.HealthCheck(ctx)
icebergClient.HealthCheck(ctx)
```

## Troubleshooting

### MinIO Connection Issues
```bash
# Check MinIO logs
docker logs helixagent-minio

# Verify credentials
mc alias set local http://localhost:9000 minioadmin minioadmin123
mc admin info local
```

### Flink Job Failures
```bash
# Check Flink logs
docker logs helixagent-flink-jobmanager

# View job status via REST
curl http://localhost:8082/jobs/overview
```

### Qdrant Collection Issues
```bash
# Check Qdrant logs
docker logs helixagent-qdrant

# List collections
curl http://localhost:6333/collections
```

## Next Steps

1. Review individual component documentation in `docs/bigdata/`
2. Run the challenge scripts to verify setup
3. Explore the Superset dashboards for analytics
4. Integrate with your application using the Go clients
