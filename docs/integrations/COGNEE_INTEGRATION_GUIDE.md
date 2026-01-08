# Cognee Integration Guide

This guide demonstrates the full Cognee integration with HelixAgent, showcasing all powerful features.

## Quick Start

### Option 1: Automatic Container Startup (Recommended)

The HelixAgent automatically starts required containers when it starts:

```bash
# Start HelixAgent - it will automatically start postgres, redis, cognee, and chromadb
./bin/helixagent

# Or with custom settings
./bin/helixagent -auto-start-docker=true
```

**What gets started automatically:**
- **PostgreSQL** (Database)
- **Redis** (Caching)
- **Cognee** (Knowledge Graph & AI Memory)
- **ChromaDB** (Vector Database)

The system checks which containers are already running and only starts missing ones, then waits for them to be healthy before starting the HTTP server.

### Option 2: Manual Container Management

If you prefer to manage containers manually:

```bash
# Start with AI profile (includes Cognee, ChromaDB, Neo4j)
docker compose --profile ai up -d

# Or start full environment
docker compose --profile full up -d

# Then start HelixAgent without auto-startup
./bin/helixagent -auto-start-docker=false
```

2. **Configure Environment**:
   ```bash
   export COGNEE_BASE_URL=http://cognee:8000
   export COGNEE_API_KEY=your-api-key  # Optional for local
   export COGNEE_AUTO_COGNIFY=true
   ```

## Advanced Features Usage

### Memory Service with Graph Reasoning

```go
// Enhanced memory search with insights
memorySvc := services.NewMemoryService(cfg)

// Graph-powered search
results, err := memorySvc.SearchMemoryWithGraphCompletion(ctx, &services.SearchRequest{
    Query:       "complex reasoning patterns",
    DatasetName: "debate-insights",
    Limit:       10,
})

// Insight-based search
insights, err := memorySvc.SearchMemoryWithInsights(ctx, &services.SearchRequest{
    Query: "sentiment analysis",
    DatasetName: "debate-insights",
})
```

### Dataset Management

```go
// Create organized datasets
err := memorySvc.CreateDataset(ctx, "research-papers", "Academic research collection")

// Switch between datasets
memorySvc.SwitchDataset("research-papers")

// List available datasets
datasets, err := memorySvc.ListDatasets(ctx)
```

### Code Processing Pipeline

```go
// Automatic code detection and processing
err := memorySvc.ProcessCodeForMemory(ctx, `
func calculateFibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return calculateFibonacci(n-1) + calculateFibonacci(n-2)
}
`, "go", "codebase")
```

### Debate Enhancement with Cognee Insights

```go
// Enhanced debate results with graph-powered insights
result := &services.DebateResult{
    DebateID:   "enhanced-debate-001",
    Topic:      "AI Ethics",
    CogneeEnhanced: true,
    CogneeInsights: &services.CogneeInsights{
        // Populated automatically with semantic analysis,
        // entity extraction, knowledge graphs, etc.
    },
}
```

## API Endpoints

### Cognee-Specific Endpoints
```bash
# Get knowledge graph visualization data
GET /v1/cognee/visualize?dataset=default&format=json
Authorization: Bearer <your-jwt-token>

# List available Cognee datasets
GET /v1/cognee/datasets
Authorization: Bearer <your-jwt-token>
```

### Example API Usage
```bash
# Get knowledge graph data
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     "http://localhost:7061/v1/cognee/visualize?dataset=default&format=json"

# List datasets
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     "http://localhost:7061/v1/cognee/datasets"
```

## Architecture Benefits

- **Three-Tier Storage**: Graph (Neo4j) + Vector (ChromaDB) + Relational (PostgreSQL)
- **Graph Reasoning**: Better than RAG with relationship understanding
- **Multi-Modal**: Text, code, images, audio processing
- **Auto-Containerization**: Automatic Cognee deployment
- **Scalable**: Production-ready with monitoring and health checks

## Performance Comparison

| Feature | Traditional RAG | Cognee Graph-RAG |
|---------|----------------|-------------------|
| Context Depth | Limited | Deep relationships |
| Reasoning | Basic | Graph-powered |
| Multi-hop queries | Poor | Excellent |
| Code understanding | Limited | Advanced pipeline |
| Memory persistence | Basic | Dynamic knowledge graphs |

The integration now leverages 100% of Cognee's capabilities, providing superior AI memory and reasoning for your HelixAgent.