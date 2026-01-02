# LlamaIndex - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/run-llama/llama_index
- **Language**: Python
- **Purpose**: Data framework for indexing and retrieving relevant context from documents
- **License**: MIT

## Core Architecture

### Directory Structure

```
llama_index/
├── core/
│   ├── indices/                 # Index implementations
│   │   ├── vector_store/        # Vector-based retrieval
│   │   ├── keyword/             # Keyword-based retrieval
│   │   └── knowledge_graph/     # Graph-based retrieval
│   ├── retrievers/              # Retrieval strategies
│   │   ├── fusion_retriever.py  # Query fusion
│   │   ├── bm25_retriever.py    # BM25 keyword search
│   │   └── router_retriever.py  # Query routing
│   ├── query_engine/            # Query processing
│   │   ├── transform/           # Query transformations
│   │   └── response_synthesizer/
│   ├── postprocessor/           # Result post-processing
│   │   ├── cohere_rerank.py     # Neural reranking
│   │   └── node_recency.py      # Recency filtering
│   └── embeddings/              # Embedding models
└── integrations/                # External integrations
```

### Key Components

#### 1. Query Fusion Retriever (`core/retrievers/fusion_retriever.py`)

**Multi-Query Generation**

```python
class QueryFusionRetriever(BaseRetriever):
    """Generate multiple queries and fuse results."""

    def __init__(
        self,
        retrievers: List[BaseRetriever],
        llm: LLM,
        num_queries: int = 4,
        similarity_top_k: int = 10
    ):
        self.retrievers = retrievers
        self.llm = llm
        self.num_queries = num_queries
        self.similarity_top_k = similarity_top_k

    async def _aretrieve(self, query: str) -> List[NodeWithScore]:
        # Generate query variations
        queries = await self._generate_queries(query)

        # Retrieve from all sources
        all_results = []
        for q in queries:
            for retriever in self.retrievers:
                results = await retriever.aretrieve(q)
                all_results.extend(results)

        # Fuse results using reciprocal rank fusion
        return self._reciprocal_rank_fusion(all_results)

    async def _generate_queries(self, query: str) -> List[str]:
        """Generate diverse query variations using LLM."""
        prompt = f"""Generate {self.num_queries} different search queries
        that would help answer the following question. Make them diverse
        and cover different aspects of the topic.

        Original question: {query}

        Queries (one per line):"""

        response = await self.llm.acomplete(prompt)
        queries = [query] + response.text.strip().split("\n")
        return queries[:self.num_queries + 1]

    def _reciprocal_rank_fusion(
        self,
        results: List[NodeWithScore],
        k: int = 60
    ) -> List[NodeWithScore]:
        """Fuse results using reciprocal rank fusion (RRF)."""
        # Group by node ID
        node_scores = defaultdict(float)
        node_map = {}

        for i, result in enumerate(results):
            node_id = result.node.node_id
            # RRF formula: 1 / (k + rank)
            rank = i + 1
            node_scores[node_id] += 1.0 / (k + rank)
            if node_id not in node_map:
                node_map[node_id] = result.node

        # Sort by fused score
        sorted_nodes = sorted(
            node_scores.items(),
            key=lambda x: x[1],
            reverse=True
        )

        return [
            NodeWithScore(node=node_map[node_id], score=score)
            for node_id, score in sorted_nodes[:self.similarity_top_k]
        ]
```

#### 2. HyDE (Hypothetical Document Embeddings)

```python
class HyDEQueryTransform(BaseQueryTransform):
    """Generate hypothetical document for better embedding matching."""

    def __init__(self, llm: LLM, include_original: bool = True):
        self.llm = llm
        self.include_original = include_original

    async def _arun(self, query: str) -> str:
        """Generate hypothetical document that would answer the query."""
        prompt = f"""Write a detailed paragraph that would be found in a
        document that perfectly answers the following question. Write it
        as if it were an excerpt from such a document.

        Question: {query}

        Document excerpt:"""

        response = await self.llm.acomplete(prompt)
        hypothetical_doc = response.text.strip()

        if self.include_original:
            return f"{query}\n\n{hypothetical_doc}"
        return hypothetical_doc


class HyDERetriever(BaseRetriever):
    """Retriever using HyDE transformation."""

    def __init__(
        self,
        vector_store: VectorStore,
        llm: LLM,
        embed_model: BaseEmbedding
    ):
        self.vector_store = vector_store
        self.transform = HyDEQueryTransform(llm)
        self.embed_model = embed_model

    async def _aretrieve(self, query: str) -> List[NodeWithScore]:
        # Transform query to hypothetical document
        hyde_query = await self.transform._arun(query)

        # Embed the hypothetical document
        embedding = await self.embed_model.aget_query_embedding(hyde_query)

        # Search vector store
        results = await self.vector_store.aquery(
            VectorStoreQuery(query_embedding=embedding, similarity_top_k=10)
        )

        return results
```

#### 3. Neural Reranking (`core/postprocessor/cohere_rerank.py`)

```python
class CohereRerank(BaseNodePostprocessor):
    """Rerank results using Cohere's neural reranker."""

    def __init__(self, api_key: str, model: str = "rerank-english-v2.0", top_n: int = 5):
        self.client = cohere.Client(api_key)
        self.model = model
        self.top_n = top_n

    def postprocess_nodes(
        self,
        nodes: List[NodeWithScore],
        query: str
    ) -> List[NodeWithScore]:
        """Rerank nodes using neural model."""
        if not nodes:
            return nodes

        # Prepare documents for reranking
        documents = [node.node.get_content() for node in nodes]

        # Call Cohere rerank API
        results = self.client.rerank(
            query=query,
            documents=documents,
            model=self.model,
            top_n=self.top_n
        )

        # Reorder nodes based on rerank scores
        reranked = []
        for result in results:
            original_node = nodes[result.index]
            reranked.append(NodeWithScore(
                node=original_node.node,
                score=result.relevance_score
            ))

        return reranked


class SentenceTransformerRerank(BaseNodePostprocessor):
    """Rerank using local sentence transformer model."""

    def __init__(self, model_name: str = "cross-encoder/ms-marco-MiniLM-L-6-v2", top_n: int = 5):
        from sentence_transformers import CrossEncoder
        self.model = CrossEncoder(model_name)
        self.top_n = top_n

    def postprocess_nodes(
        self,
        nodes: List[NodeWithScore],
        query: str
    ) -> List[NodeWithScore]:
        """Rerank using cross-encoder."""
        if not nodes:
            return nodes

        # Prepare pairs for cross-encoder
        pairs = [(query, node.node.get_content()) for node in nodes]

        # Get scores
        scores = self.model.predict(pairs)

        # Sort by score
        scored_nodes = list(zip(nodes, scores))
        scored_nodes.sort(key=lambda x: x[1], reverse=True)

        return [
            NodeWithScore(node=node.node, score=float(score))
            for node, score in scored_nodes[:self.top_n]
        ]
```

#### 4. Hybrid Search (BM25 + Vector)

```python
class HybridRetriever(BaseRetriever):
    """Combine BM25 keyword search with vector search."""

    def __init__(
        self,
        vector_retriever: BaseRetriever,
        bm25_retriever: BM25Retriever,
        alpha: float = 0.5  # Weight for vector vs BM25
    ):
        self.vector_retriever = vector_retriever
        self.bm25_retriever = bm25_retriever
        self.alpha = alpha

    async def _aretrieve(self, query: str) -> List[NodeWithScore]:
        # Get results from both retrievers
        vector_results = await self.vector_retriever.aretrieve(query)
        bm25_results = await self.bm25_retriever.aretrieve(query)

        # Normalize scores
        vector_results = self._normalize_scores(vector_results)
        bm25_results = self._normalize_scores(bm25_results)

        # Combine with weighted average
        combined = self._combine_results(vector_results, bm25_results)

        return combined

    def _normalize_scores(self, results: List[NodeWithScore]) -> List[NodeWithScore]:
        """Normalize scores to [0, 1] range."""
        if not results:
            return results

        scores = [r.score for r in results]
        min_score, max_score = min(scores), max(scores)
        range_score = max_score - min_score

        if range_score == 0:
            return [NodeWithScore(node=r.node, score=1.0) for r in results]

        return [
            NodeWithScore(
                node=r.node,
                score=(r.score - min_score) / range_score
            )
            for r in results
        ]

    def _combine_results(
        self,
        vector_results: List[NodeWithScore],
        bm25_results: List[NodeWithScore]
    ) -> List[NodeWithScore]:
        """Combine results with weighted average."""
        node_scores = {}

        for result in vector_results:
            node_id = result.node.node_id
            node_scores[node_id] = {
                "node": result.node,
                "vector": result.score,
                "bm25": 0.0
            }

        for result in bm25_results:
            node_id = result.node.node_id
            if node_id in node_scores:
                node_scores[node_id]["bm25"] = result.score
            else:
                node_scores[node_id] = {
                    "node": result.node,
                    "vector": 0.0,
                    "bm25": result.score
                }

        # Calculate combined scores
        combined = []
        for node_id, data in node_scores.items():
            score = self.alpha * data["vector"] + (1 - self.alpha) * data["bm25"]
            combined.append(NodeWithScore(node=data["node"], score=score))

        combined.sort(key=lambda x: x.score, reverse=True)
        return combined
```

## Go Client Implementation (with Cognee Adapter)

Since LlamaIndex relies on Python ML frameworks, we implement an HTTP client that integrates with Cognee.

### Client Implementation

```go
// internal/optimization/llamaindex/client.go

package llamaindex

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// LlamaIndexClient communicates with LlamaIndex service
type LlamaIndexClient struct {
    baseURL    string
    httpClient *http.Client
}

// Config holds client configuration
type Config struct {
    BaseURL string
    Timeout time.Duration
}

// NewLlamaIndexClient creates a new client
func NewLlamaIndexClient(config *Config) *LlamaIndexClient {
    return &LlamaIndexClient{
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// QueryRequest is the query request
type QueryRequest struct {
    Query          string            `json:"query"`
    TopK           int               `json:"top_k,omitempty"`
    UseHyDE        bool              `json:"use_hyde,omitempty"`
    UseFusion      bool              `json:"use_fusion,omitempty"`
    NumQueries     int               `json:"num_queries,omitempty"`
    Rerank         bool              `json:"rerank,omitempty"`
    RerankTopN     int               `json:"rerank_top_n,omitempty"`
    Alpha          float64           `json:"alpha,omitempty"`  // Hybrid search weight
    Filters        map[string]any    `json:"filters,omitempty"`
}

// QueryResult is the query result
type QueryResult struct {
    Nodes       []NodeWithScore `json:"nodes"`
    QueryUsed   string          `json:"query_used"`
    HyDEDoc     string          `json:"hyde_doc,omitempty"`
    FusedFrom   []string        `json:"fused_from,omitempty"`
    LatencyMS   float64         `json:"latency_ms"`
}

// NodeWithScore represents a retrieved node
type NodeWithScore struct {
    ID       string         `json:"id"`
    Content  string         `json:"content"`
    Score    float64        `json:"score"`
    Metadata map[string]any `json:"metadata,omitempty"`
}

// Query performs a retrieval query
func (c *LlamaIndexClient) Query(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/query", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
    }

    var result QueryResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// QueryWithFusion performs query fusion retrieval
func (c *LlamaIndexClient) QueryWithFusion(ctx context.Context, query string, numQueries int, topK int) (*QueryResult, error) {
    return c.Query(ctx, &QueryRequest{
        Query:      query,
        UseFusion:  true,
        NumQueries: numQueries,
        TopK:       topK,
    })
}

// QueryWithHyDE performs HyDE retrieval
func (c *LlamaIndexClient) QueryWithHyDE(ctx context.Context, query string, topK int) (*QueryResult, error) {
    return c.Query(ctx, &QueryRequest{
        Query:   query,
        UseHyDE: true,
        TopK:    topK,
    })
}

// QueryHybrid performs hybrid BM25 + vector search
func (c *LlamaIndexClient) QueryHybrid(ctx context.Context, query string, alpha float64, topK int) (*QueryResult, error) {
    return c.Query(ctx, &QueryRequest{
        Query: query,
        Alpha: alpha,
        TopK:  topK,
    })
}
```

### Cognee Adapter

```go
// internal/optimization/llamaindex/cognee_adapter.go

package llamaindex

import (
    "context"
    "fmt"

    "superagent/internal/services"
)

// CogneeAdapter adapts Cognee for use with LlamaIndex retrieval patterns
type CogneeAdapter struct {
    cogneeService *services.CogneeService
    llamaIndex    *LlamaIndexClient
}

// NewCogneeAdapter creates a new adapter
func NewCogneeAdapter(cognee *services.CogneeService, llamaIndex *LlamaIndexClient) *CogneeAdapter {
    return &CogneeAdapter{
        cogneeService: cognee,
        llamaIndex:    llamaIndex,
    }
}

// EnhancedQuery combines Cognee's knowledge graph with LlamaIndex retrieval
func (a *CogneeAdapter) EnhancedQuery(ctx context.Context, query string, opts *QueryOptions) (*EnhancedQueryResult, error) {
    if opts == nil {
        opts = DefaultQueryOptions()
    }

    result := &EnhancedQueryResult{
        Query: query,
    }

    // 1. Query Cognee's knowledge graph for related concepts
    cogneeResults, err := a.cogneeService.SearchMemory(ctx, query, opts.TopK)
    if err != nil {
        // Log but don't fail - continue with LlamaIndex only
        result.Warnings = append(result.Warnings, fmt.Sprintf("Cognee search failed: %v", err))
    } else {
        result.KnowledgeGraphNodes = cogneeResults
    }

    // 2. If fusion is enabled, generate query variations
    var finalQuery string
    if opts.UseFusion && a.llamaIndex != nil {
        fusionResult, err := a.llamaIndex.QueryWithFusion(ctx, query, opts.NumQueries, opts.TopK)
        if err != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("Query fusion failed: %v", err))
            finalQuery = query
        } else {
            result.FusedQueries = fusionResult.FusedFrom
            finalQuery = fusionResult.QueryUsed
        }
    } else {
        finalQuery = query
    }

    // 3. If HyDE is enabled, generate hypothetical document
    if opts.UseHyDE && a.llamaIndex != nil {
        hydeResult, err := a.llamaIndex.QueryWithHyDE(ctx, finalQuery, opts.TopK)
        if err != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("HyDE failed: %v", err))
        } else {
            result.HypotheticalDoc = hydeResult.HyDEDoc
            result.VectorResults = hydeResult.Nodes
        }
    }

    // 4. Merge results from Cognee and LlamaIndex
    result.MergedResults = a.mergeResults(result.KnowledgeGraphNodes, result.VectorResults, opts.Alpha)

    // 5. If reranking is enabled, rerank merged results
    if opts.Rerank && a.llamaIndex != nil {
        reranked, err := a.llamaIndex.Rerank(ctx, query, result.MergedResults, opts.RerankTopN)
        if err != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("Reranking failed: %v", err))
        } else {
            result.MergedResults = reranked
        }
    }

    return result, nil
}

func (a *CogneeAdapter) mergeResults(cogneeResults []*services.CogneeSearchResult, vectorResults []NodeWithScore, alpha float64) []NodeWithScore {
    // Convert Cognee results to NodeWithScore
    nodeScores := make(map[string]*NodeWithScore)

    for _, cr := range cogneeResults {
        nodeScores[cr.ID] = &NodeWithScore{
            ID:       cr.ID,
            Content:  cr.Content,
            Score:    cr.Score * (1 - alpha), // Cognee weight
            Metadata: cr.Metadata,
        }
    }

    for _, vr := range vectorResults {
        if existing, ok := nodeScores[vr.ID]; ok {
            existing.Score += vr.Score * alpha // Add vector weight
        } else {
            nodeScores[vr.ID] = &NodeWithScore{
                ID:       vr.ID,
                Content:  vr.Content,
                Score:    vr.Score * alpha,
                Metadata: vr.Metadata,
            }
        }
    }

    // Convert to slice and sort
    results := make([]NodeWithScore, 0, len(nodeScores))
    for _, node := range nodeScores {
        results = append(results, *node)
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    return results
}

// QueryOptions configures query behavior
type QueryOptions struct {
    TopK       int
    UseFusion  bool
    NumQueries int
    UseHyDE    bool
    Rerank     bool
    RerankTopN int
    Alpha      float64 // Weight for vector vs knowledge graph (0-1)
}

// DefaultQueryOptions returns default options
func DefaultQueryOptions() *QueryOptions {
    return &QueryOptions{
        TopK:       10,
        UseFusion:  false,
        NumQueries: 4,
        UseHyDE:    false,
        Rerank:     false,
        RerankTopN: 5,
        Alpha:      0.5,
    }
}

// EnhancedQueryResult contains enhanced query results
type EnhancedQueryResult struct {
    Query              string
    FusedQueries       []string
    HypotheticalDoc    string
    KnowledgeGraphNodes []*services.CogneeSearchResult
    VectorResults      []NodeWithScore
    MergedResults      []NodeWithScore
    Warnings           []string
}
```

### Retriever Interface

```go
// internal/optimization/llamaindex/retriever.go

package llamaindex

import (
    "context"
)

// Retriever defines the retrieval interface
type Retriever interface {
    Retrieve(ctx context.Context, query string, opts *RetrieveOptions) ([]NodeWithScore, error)
}

// RetrieveOptions configures retrieval
type RetrieveOptions struct {
    TopK    int
    Filters map[string]any
}

// FusionRetriever implements query fusion locally
type FusionRetriever struct {
    client     *LlamaIndexClient
    numQueries int
}

// NewFusionRetriever creates a new fusion retriever
func NewFusionRetriever(client *LlamaIndexClient, numQueries int) *FusionRetriever {
    return &FusionRetriever{
        client:     client,
        numQueries: numQueries,
    }
}

// Retrieve performs fusion retrieval
func (r *FusionRetriever) Retrieve(ctx context.Context, query string, opts *RetrieveOptions) ([]NodeWithScore, error) {
    result, err := r.client.QueryWithFusion(ctx, query, r.numQueries, opts.TopK)
    if err != nil {
        return nil, err
    }
    return result.Nodes, nil
}

// HyDERetriever implements HyDE retrieval
type HyDERetriever struct {
    client *LlamaIndexClient
}

// NewHyDERetriever creates a new HyDE retriever
func NewHyDERetriever(client *LlamaIndexClient) *HyDERetriever {
    return &HyDERetriever{client: client}
}

// Retrieve performs HyDE retrieval
func (r *HyDERetriever) Retrieve(ctx context.Context, query string, opts *RetrieveOptions) ([]NodeWithScore, error) {
    result, err := r.client.QueryWithHyDE(ctx, query, opts.TopK)
    if err != nil {
        return nil, err
    }
    return result.Nodes, nil
}

// RecipocalRankFusion performs RRF on multiple result sets
func ReciprocalRankFusion(resultSets [][]NodeWithScore, k int, topN int) []NodeWithScore {
    if k == 0 {
        k = 60 // Default RRF constant
    }

    nodeScores := make(map[string]float64)
    nodeMap := make(map[string]NodeWithScore)

    for _, results := range resultSets {
        for rank, node := range results {
            nodeScores[node.ID] += 1.0 / float64(k+rank+1)
            if _, ok := nodeMap[node.ID]; !ok {
                nodeMap[node.ID] = node
            }
        }
    }

    // Sort by fused score
    type scoredNode struct {
        id    string
        score float64
    }

    sorted := make([]scoredNode, 0, len(nodeScores))
    for id, score := range nodeScores {
        sorted = append(sorted, scoredNode{id, score})
    }

    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].score > sorted[j].score
    })

    // Build result
    results := make([]NodeWithScore, 0, topN)
    for i, sn := range sorted {
        if i >= topN {
            break
        }
        node := nodeMap[sn.id]
        node.Score = sn.score
        results = append(results, node)
    }

    return results
}
```

## Python Service

```python
# services/llamaindex/server.py

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Any, Optional
import time

from llama_index.core import VectorStoreIndex, Settings
from llama_index.core.retrievers import QueryFusionRetriever
from llama_index.core.query_engine import RetrieverQueryEngine
from llama_index.postprocessor.cohere_rerank import CohereRerank

app = FastAPI()

class QueryRequest(BaseModel):
    query: str
    top_k: int = 10
    use_hyde: bool = False
    use_fusion: bool = False
    num_queries: int = 4
    rerank: bool = False
    rerank_top_n: int = 5
    alpha: float = 0.5
    filters: Optional[Dict[str, Any]] = None

class NodeResult(BaseModel):
    id: str
    content: str
    score: float
    metadata: Dict[str, Any] = {}

class QueryResponse(BaseModel):
    nodes: List[NodeResult]
    query_used: str
    hyde_doc: Optional[str] = None
    fused_from: Optional[List[str]] = None
    latency_ms: float

@app.post("/query", response_model=QueryResponse)
async def query(request: QueryRequest):
    start = time.time()

    # Build retriever based on options
    # (Implementation depends on index configuration)

    latency = (time.time() - start) * 1000

    return QueryResponse(
        nodes=[],  # Populated by retriever
        query_used=request.query,
        latency_ms=latency
    )

@app.post("/rerank")
async def rerank(query: str, nodes: List[NodeResult], top_n: int = 5):
    reranker = CohereRerank(top_n=top_n)
    # Rerank implementation
    pass
```

## Test Coverage Requirements

```go
// tests/optimization/unit/llamaindex/client_test.go

func TestLlamaIndexClient_Query(t *testing.T)
func TestLlamaIndexClient_QueryWithFusion(t *testing.T)
func TestLlamaIndexClient_QueryWithHyDE(t *testing.T)
func TestLlamaIndexClient_QueryHybrid(t *testing.T)

func TestCogneeAdapter_EnhancedQuery(t *testing.T)
func TestCogneeAdapter_MergeResults(t *testing.T)

func TestReciprocalRankFusion(t *testing.T)
func TestFusionRetriever_Retrieve(t *testing.T)
func TestHyDERetriever_Retrieve(t *testing.T)

// tests/optimization/integration/llamaindex_cognee_integration_test.go
func TestLlamaIndex_Cognee_Integration(t *testing.T)
```

## Conclusion

LlamaIndex requires an HTTP bridge due to its reliance on Python ML frameworks (sentence-transformers, cross-encoders). The Go client focuses on orchestration and result merging, while the Python service handles the heavy lifting.

**Key Integration Point**: The CogneeAdapter ensures Cognee remains the primary indexing system while LlamaIndex provides advanced retrieval patterns (HyDE, fusion, reranking).

**Estimated Implementation Time**: 1.5 weeks
**Risk Level**: Medium
**Dependencies**: Python service, Cognee integration
