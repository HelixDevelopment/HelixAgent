"""
LlamaIndex HTTP Bridge Server.
Provides advanced document retrieval and query capabilities.
Integrates with Cognee for indexed data access.
"""

import os
from typing import Any, Optional
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import httpx
import numpy as np

# Configuration
SUPERAGENT_URL = os.getenv("SUPERAGENT_URL", "http://localhost:8080")
COGNEE_URL = os.getenv("COGNEE_URL", "http://localhost:8000")
EMBEDDING_ENDPOINT = os.getenv("EMBEDDING_ENDPOINT", f"{SUPERAGENT_URL}/v1/embeddings")
LLM_ENDPOINT = os.getenv("LLM_ENDPOINT", f"{SUPERAGENT_URL}/v1/chat/completions")


class ServiceClients:
    """HTTP clients for external services."""

    def __init__(self):
        self.client = httpx.AsyncClient(timeout=120.0)

    async def get_embedding(self, text: str) -> list[float]:
        """Get embedding from SuperAgent."""
        try:
            response = await self.client.post(
                EMBEDDING_ENDPOINT,
                json={"input": text, "model": "text-embedding-3-small"}
            )
            response.raise_for_status()
            data = response.json()
            return data["data"][0]["embedding"]
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"Embedding failed: {e}")

    async def generate(self, prompt: str, temperature: float = 0.7) -> str:
        """Generate response from SuperAgent."""
        try:
            response = await self.client.post(
                LLM_ENDPOINT,
                json={
                    "model": "default",
                    "messages": [{"role": "user", "content": prompt}],
                    "temperature": temperature,
                }
            )
            response.raise_for_status()
            data = response.json()
            return data["choices"][0]["message"]["content"]
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"LLM call failed: {e}")

    async def query_cognee(self, query: str, limit: int = 5) -> list[dict]:
        """Query Cognee for relevant documents."""
        try:
            response = await self.client.post(
                f"{COGNEE_URL}/search",
                json={"query": query, "limit": limit}
            )
            if response.status_code == 200:
                return response.json().get("results", [])
            return []
        except Exception:
            return []

    async def close(self):
        await self.client.aclose()


# Global clients
clients: Optional[ServiceClients] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    global clients
    clients = ServiceClients()
    yield
    await clients.close()


app = FastAPI(
    title="LlamaIndex Bridge",
    description="HTTP bridge for LlamaIndex retrieval capabilities",
    version="1.0.0",
    lifespan=lifespan,
)


# Request/Response Models
class QueryRequest(BaseModel):
    """Request for document query."""
    query: str = Field(..., description="The query to answer")
    top_k: int = Field(default=5, description="Number of documents to retrieve")
    use_cognee: bool = Field(default=True, description="Whether to query Cognee index")
    rerank: bool = Field(default=True, description="Whether to rerank results")
    query_transform: Optional[str] = Field(default=None, description="Query transformation: hyde, decompose, step_back")


class QueryResponse(BaseModel):
    """Response from document query."""
    answer: str
    sources: list[dict[str, Any]]
    transformed_query: Optional[str] = None
    confidence: float


class HyDERequest(BaseModel):
    """Request for HyDE (Hypothetical Document Embeddings)."""
    query: str = Field(..., description="The query to expand")
    num_hypotheses: int = Field(default=3, description="Number of hypothetical documents to generate")


class HyDEResponse(BaseModel):
    """Response from HyDE expansion."""
    original_query: str
    hypothetical_documents: list[str]
    combined_embedding: list[float]


class DecomposeQueryRequest(BaseModel):
    """Request for query decomposition."""
    query: str = Field(..., description="Complex query to decompose")
    max_subqueries: int = Field(default=3, description="Maximum number of subqueries")


class DecomposeQueryResponse(BaseModel):
    """Response from query decomposition."""
    original_query: str
    subqueries: list[str]
    reasoning: str


class RerankRequest(BaseModel):
    """Request for document reranking."""
    query: str = Field(..., description="The query")
    documents: list[str] = Field(..., description="Documents to rerank")
    top_k: int = Field(default=5, description="Number of documents to return")


class RerankResponse(BaseModel):
    """Response from reranking."""
    ranked_documents: list[dict[str, Any]]


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    version: str
    cognee_available: bool
    superagent_available: bool


# Utility functions
def cosine_similarity(a: list[float], b: list[float]) -> float:
    """Calculate cosine similarity between two vectors."""
    a_np = np.array(a)
    b_np = np.array(b)
    return float(np.dot(a_np, b_np) / (np.linalg.norm(a_np) * np.linalg.norm(b_np)))


async def rerank_documents(query: str, documents: list[dict], query_embedding: list[float]) -> list[dict]:
    """Rerank documents based on query similarity."""
    scored = []
    for doc in documents:
        if "embedding" in doc:
            score = cosine_similarity(query_embedding, doc["embedding"])
        else:
            # Get embedding for document
            doc_text = doc.get("content", doc.get("text", ""))[:1000]
            if doc_text:
                doc_embedding = await clients.get_embedding(doc_text)
                score = cosine_similarity(query_embedding, doc_embedding)
            else:
                score = 0.0
        scored.append({**doc, "score": score})

    return sorted(scored, key=lambda x: x["score"], reverse=True)


# API Endpoints
@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    cognee_ok = False
    superagent_ok = False

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(f"{COGNEE_URL}/health")
            cognee_ok = resp.status_code == 200
    except Exception:
        pass

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(f"{SUPERAGENT_URL}/health")
            superagent_ok = resp.status_code == 200
    except Exception:
        pass

    return HealthResponse(
        status="healthy",
        version="1.0.0",
        cognee_available=cognee_ok,
        superagent_available=superagent_ok,
    )


@app.post("/query", response_model=QueryResponse)
async def query_documents(request: QueryRequest):
    """Query documents and generate an answer."""

    query = request.query
    transformed_query = None

    # Apply query transformation if requested
    if request.query_transform == "hyde":
        hyde_result = await hyde_expand(HyDERequest(query=query))
        # Use the first hypothetical document as the query
        if hyde_result.hypothetical_documents:
            transformed_query = hyde_result.hypothetical_documents[0]
            query = transformed_query

    elif request.query_transform == "decompose":
        decompose_result = await decompose_query(DecomposeQueryRequest(query=query))
        # Combine subqueries
        transformed_query = " ".join(decompose_result.subqueries)
        query = transformed_query

    elif request.query_transform == "step_back":
        # Step-back prompting: ask a more general question first
        step_back_prompt = f"""Given the question: "{query}"
What is a more general, higher-level question that would help answer this?
Provide only the question, no explanation."""
        transformed_query = await clients.generate(step_back_prompt, temperature=0.3)
        query = f"{transformed_query.strip()} {request.query}"

    # Get query embedding
    query_embedding = await clients.get_embedding(query)

    # Retrieve documents
    documents = []

    if request.use_cognee:
        cognee_results = await clients.query_cognee(query, request.top_k * 2)
        documents.extend(cognee_results)

    # Rerank if requested
    if request.rerank and documents:
        documents = await rerank_documents(query, documents, query_embedding)
        documents = documents[:request.top_k]

    # Generate answer with retrieved context
    context = "\n\n".join([
        f"[Source {i+1}]: {doc.get('content', doc.get('text', ''))[:500]}"
        for i, doc in enumerate(documents[:request.top_k])
    ])

    answer_prompt = f"""Answer the following question based on the provided context.
If the context doesn't contain relevant information, say so and provide your best answer.

Context:
{context if context else "No relevant documents found."}

Question: {request.query}

Answer:"""

    answer = await clients.generate(answer_prompt, temperature=0.3)

    # Calculate confidence based on document relevance
    confidence = 0.0
    if documents:
        scores = [doc.get("score", 0.5) for doc in documents[:request.top_k]]
        confidence = sum(scores) / len(scores)

    sources = [
        {
            "content": doc.get("content", doc.get("text", ""))[:200],
            "score": doc.get("score", 0.0),
            "metadata": doc.get("metadata", {}),
        }
        for doc in documents[:request.top_k]
    ]

    return QueryResponse(
        answer=answer.strip(),
        sources=sources,
        transformed_query=transformed_query,
        confidence=confidence,
    )


@app.post("/hyde", response_model=HyDEResponse)
async def hyde_expand(request: HyDERequest):
    """Generate hypothetical documents for query expansion (HyDE)."""

    hypothetical_docs = []

    for i in range(request.num_hypotheses):
        prompt = f"""Write a short passage that would be a perfect answer to the following question.
The passage should be factual and informative.

Question: {request.query}

Passage {i+1}:"""

        doc = await clients.generate(prompt, temperature=0.7 + (i * 0.1))
        hypothetical_docs.append(doc.strip())

    # Generate embeddings and combine
    embeddings = []
    for doc in hypothetical_docs:
        emb = await clients.get_embedding(doc)
        embeddings.append(emb)

    # Average the embeddings
    combined = np.mean(embeddings, axis=0).tolist()

    return HyDEResponse(
        original_query=request.query,
        hypothetical_documents=hypothetical_docs,
        combined_embedding=combined,
    )


@app.post("/decompose", response_model=DecomposeQueryResponse)
async def decompose_query(request: DecomposeQueryRequest):
    """Decompose a complex query into simpler subqueries."""

    prompt = f"""Break down the following complex question into {request.max_subqueries} simpler, independent questions.
Each subquery should address a specific aspect of the original question.

Original question: {request.query}

Provide your response in this format:
Reasoning: [Brief explanation of your decomposition]
Subquery 1: [First subquery]
Subquery 2: [Second subquery]
...
"""

    response = await clients.generate(prompt, temperature=0.3)

    # Parse response
    lines = response.strip().split("\n")
    reasoning = ""
    subqueries = []

    for line in lines:
        line = line.strip()
        if line.lower().startswith("reasoning:"):
            reasoning = line[10:].strip()
        elif line.lower().startswith("subquery"):
            # Extract the subquery after the colon
            if ":" in line:
                subquery = line.split(":", 1)[1].strip()
                if subquery:
                    subqueries.append(subquery)

    # Fallback if parsing failed
    if not subqueries:
        subqueries = [request.query]

    return DecomposeQueryResponse(
        original_query=request.query,
        subqueries=subqueries[:request.max_subqueries],
        reasoning=reasoning,
    )


@app.post("/rerank", response_model=RerankResponse)
async def rerank(request: RerankRequest):
    """Rerank documents based on query relevance."""

    query_embedding = await clients.get_embedding(request.query)

    documents = [{"content": doc, "text": doc} for doc in request.documents]
    ranked = await rerank_documents(request.query, documents, query_embedding)

    return RerankResponse(
        ranked_documents=[
            {"content": doc["content"], "score": doc["score"], "rank": i + 1}
            for i, doc in enumerate(ranked[:request.top_k])
        ]
    )


@app.post("/query_fusion")
async def query_fusion(query: str, num_variations: int = 3, top_k: int = 5):
    """Generate query variations and fuse results (Reciprocal Rank Fusion)."""

    # Generate query variations
    variation_prompt = f"""Generate {num_variations} different ways to ask the following question.
Each variation should capture the same intent but use different words or perspectives.

Original: {query}

Variations:"""

    variations_response = await clients.generate(variation_prompt, temperature=0.7)
    variations = [query]  # Include original
    for line in variations_response.strip().split("\n"):
        line = line.strip()
        if line and not line.startswith("Variations"):
            # Remove numbering
            if line[0].isdigit() and "." in line[:3]:
                line = line.split(".", 1)[1].strip()
            variations.append(line)

    # Query with each variation
    all_results = []
    for var in variations[:num_variations + 1]:
        cognee_results = await clients.query_cognee(var, top_k)
        all_results.append(cognee_results)

    # Reciprocal Rank Fusion
    doc_scores = {}
    k = 60  # RRF constant

    for results in all_results:
        for rank, doc in enumerate(results):
            doc_id = doc.get("id", doc.get("content", "")[:50])
            if doc_id not in doc_scores:
                doc_scores[doc_id] = {"doc": doc, "score": 0}
            doc_scores[doc_id]["score"] += 1 / (k + rank + 1)

    # Sort by fused score
    fused = sorted(doc_scores.values(), key=lambda x: x["score"], reverse=True)

    return {
        "query": query,
        "variations_used": variations[:num_variations + 1],
        "results": [
            {"content": item["doc"].get("content", ""), "score": item["score"]}
            for item in fused[:top_k]
        ]
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8002)
