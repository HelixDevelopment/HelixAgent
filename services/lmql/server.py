"""
LMQL HTTP Bridge Server.
Provides query language capabilities for LLM interactions.
"""

import os
import re
from typing import Any, Optional
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import httpx

# Configuration
SUPERAGENT_URL = os.getenv("SUPERAGENT_URL", "http://localhost:8080")
LLM_ENDPOINT = os.getenv("LLM_ENDPOINT", f"{SUPERAGENT_URL}/v1/chat/completions")


class LLMClient:
    """HTTP client for SuperAgent LLM."""

    def __init__(self):
        self.client = httpx.AsyncClient(timeout=120.0)

    async def generate(self, prompt: str, temperature: float = 0.7, stop: list[str] = None, max_tokens: int = 500) -> str:
        """Generate response from SuperAgent."""
        try:
            payload = {
                "model": "default",
                "messages": [{"role": "user", "content": prompt}],
                "temperature": temperature,
                "max_tokens": max_tokens,
            }
            if stop:
                payload["stop"] = stop

            response = await self.client.post(LLM_ENDPOINT, json=payload)
            response.raise_for_status()
            data = response.json()
            return data["choices"][0]["message"]["content"]
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"LLM call failed: {e}")

    async def close(self):
        await self.client.aclose()


# Global client
llm_client: Optional[LLMClient] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    global llm_client
    llm_client = LLMClient()
    yield
    await llm_client.close()


app = FastAPI(
    title="LMQL Bridge",
    description="HTTP bridge for LMQL query language capabilities",
    version="1.0.0",
    lifespan=lifespan,
)


# Request/Response Models
class QueryRequest(BaseModel):
    """Request for LMQL-style query execution."""
    query: str = Field(..., description="The LMQL-style query")
    variables: dict[str, Any] = Field(default_factory=dict, description="Input variables")
    temperature: float = Field(default=0.7, description="Sampling temperature")
    max_tokens: int = Field(default=500, description="Maximum tokens to generate")


class QueryResponse(BaseModel):
    """Response from query execution."""
    result: dict[str, Any]
    raw_output: str
    constraints_satisfied: bool


class ConstrainedRequest(BaseModel):
    """Request for constrained generation."""
    prompt: str = Field(..., description="The base prompt")
    constraints: list[dict[str, Any]] = Field(..., description="List of constraints to apply")
    temperature: float = Field(default=0.7, description="Sampling temperature")


class ConstrainedResponse(BaseModel):
    """Response from constrained generation."""
    text: str
    constraints_checked: list[dict[str, Any]]
    all_satisfied: bool


class DecodingRequest(BaseModel):
    """Request for custom decoding strategy."""
    prompt: str = Field(..., description="The prompt")
    strategy: str = Field(default="argmax", description="Decoding strategy: argmax, sample, beam")
    beam_width: int = Field(default=3, description="Beam width for beam search")
    num_samples: int = Field(default=1, description="Number of samples for sample strategy")
    temperature: float = Field(default=0.7, description="Temperature for sampling")


class DecodingResponse(BaseModel):
    """Response from custom decoding."""
    outputs: list[str]
    strategy_used: str
    metadata: dict[str, Any]


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    version: str
    llm_available: bool


# LMQL-style query parsing
def parse_lmql_query(query: str) -> dict:
    """Parse a simplified LMQL-style query."""
    result = {
        "variables": [],
        "constraints": [],
        "prompt_template": "",
        "where_clause": "",
        "distribution": None,
    }

    # Extract variable declarations [VAR: type]
    var_pattern = r"\[(\w+)(?:\s*:\s*(\w+))?\]"
    variables = re.findall(var_pattern, query)
    for var_name, var_type in variables:
        result["variables"].append({
            "name": var_name,
            "type": var_type or "str",
        })

    # Extract WHERE constraints
    where_match = re.search(r"where\s+(.+?)(?:$|distribution)", query, re.IGNORECASE | re.DOTALL)
    if where_match:
        result["where_clause"] = where_match.group(1).strip()
        # Parse individual constraints
        constraints = re.split(r"\s+and\s+", result["where_clause"], flags=re.IGNORECASE)
        for c in constraints:
            c = c.strip()
            if c:
                result["constraints"].append(parse_constraint(c))

    # Extract DISTRIBUTION clause
    dist_match = re.search(r"distribution\s+(\w+)", query, re.IGNORECASE)
    if dist_match:
        result["distribution"] = dist_match.group(1)

    # Extract prompt template (everything before WHERE or at the start)
    prompt_match = re.search(r"^(.+?)(?:where|$)", query, re.IGNORECASE | re.DOTALL)
    if prompt_match:
        template = prompt_match.group(1).strip()
        # Remove variable declarations for the raw template
        template = re.sub(var_pattern, lambda m: f"{{{m.group(1)}}}", template)
        result["prompt_template"] = template

    return result


def parse_constraint(constraint_str: str) -> dict:
    """Parse a single constraint expression."""
    constraint = {"raw": constraint_str, "type": "unknown", "satisfied": None}

    # len(VAR) < N
    len_match = re.match(r"len\((\w+)\)\s*([<>=!]+)\s*(\d+)", constraint_str)
    if len_match:
        constraint["type"] = "length"
        constraint["variable"] = len_match.group(1)
        constraint["operator"] = len_match.group(2)
        constraint["value"] = int(len_match.group(3))
        return constraint

    # VAR in ["a", "b", "c"]
    in_match = re.match(r"(\w+)\s+in\s+\[(.+)\]", constraint_str)
    if in_match:
        constraint["type"] = "choices"
        constraint["variable"] = in_match.group(1)
        options = re.findall(r'"([^"]+)"', in_match.group(2))
        constraint["options"] = options
        return constraint

    # VAR matches "regex"
    regex_match = re.match(r'(\w+)\s+matches\s+"([^"]+)"', constraint_str)
    if regex_match:
        constraint["type"] = "regex"
        constraint["variable"] = regex_match.group(1)
        constraint["pattern"] = regex_match.group(2)
        return constraint

    # STOPS_AT(VAR, "token")
    stops_match = re.match(r'STOPS_AT\((\w+),\s*"([^"]+)"\)', constraint_str)
    if stops_match:
        constraint["type"] = "stops_at"
        constraint["variable"] = stops_match.group(1)
        constraint["stop_token"] = stops_match.group(2)
        return constraint

    # INT(VAR)
    int_match = re.match(r"INT\((\w+)\)", constraint_str)
    if int_match:
        constraint["type"] = "integer"
        constraint["variable"] = int_match.group(1)
        return constraint

    return constraint


def check_constraint(constraint: dict, value: Any) -> bool:
    """Check if a value satisfies a constraint."""
    ctype = constraint.get("type", "unknown")

    if ctype == "length":
        op = constraint["operator"]
        target = constraint["value"]
        length = len(str(value))
        if op == "<":
            return length < target
        elif op == "<=":
            return length <= target
        elif op == ">":
            return length > target
        elif op == ">=":
            return length >= target
        elif op == "==":
            return length == target
        elif op == "!=":
            return length != target

    elif ctype == "choices":
        return str(value) in constraint.get("options", [])

    elif ctype == "regex":
        pattern = constraint.get("pattern", ".*")
        return bool(re.match(pattern, str(value)))

    elif ctype == "integer":
        try:
            int(value)
            return True
        except (ValueError, TypeError):
            return False

    elif ctype == "stops_at":
        # Can't really check this post-hoc
        return True

    return True  # Unknown constraints pass by default


# API Endpoints
@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    llm_ok = False
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(f"{SUPERAGENT_URL}/health")
            llm_ok = resp.status_code == 200
    except Exception:
        pass

    return HealthResponse(
        status="healthy",
        version="1.0.0",
        llm_available=llm_ok,
    )


@app.post("/query", response_model=QueryResponse)
async def execute_query(request: QueryRequest):
    """Execute an LMQL-style query."""

    try:
        parsed = parse_lmql_query(request.query)

        # Build prompt from template
        prompt = parsed["prompt_template"]
        for key, value in request.variables.items():
            prompt = prompt.replace(f"{{{key}}}", str(value))

        # Add constraint hints to prompt
        if parsed["constraints"]:
            constraint_hints = []
            for c in parsed["constraints"]:
                if c["type"] == "length":
                    constraint_hints.append(f"Keep {c['variable']} under {c['value']} characters")
                elif c["type"] == "choices":
                    constraint_hints.append(f"{c['variable']} must be one of: {', '.join(c['options'])}")
                elif c["type"] == "integer":
                    constraint_hints.append(f"{c['variable']} must be an integer")

            if constraint_hints:
                prompt += f"\n\nConstraints:\n" + "\n".join(f"- {h}" for h in constraint_hints)

        # Determine stop tokens
        stop_tokens = []
        for c in parsed["constraints"]:
            if c["type"] == "stops_at":
                stop_tokens.append(c["stop_token"])

        # Generate
        response = await llm_client.generate(
            prompt,
            temperature=request.temperature,
            stop=stop_tokens if stop_tokens else None,
            max_tokens=request.max_tokens,
        )

        # Extract variable values from response
        result = {}
        for var in parsed["variables"]:
            var_name = var["name"]
            # Simple extraction: try to find the value in the response
            # In a real implementation, this would be more sophisticated
            result[var_name] = response.strip()

        # Check constraints
        all_satisfied = True
        for c in parsed["constraints"]:
            var_name = c.get("variable")
            if var_name and var_name in result:
                c["satisfied"] = check_constraint(c, result[var_name])
                all_satisfied = all_satisfied and c["satisfied"]

        return QueryResponse(
            result=result,
            raw_output=response,
            constraints_satisfied=all_satisfied,
        )

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/constrained", response_model=ConstrainedResponse)
async def constrained_generate(request: ConstrainedRequest):
    """Generate text with explicit constraints."""

    # Build prompt with constraint hints
    constraint_hints = []
    for c in request.constraints:
        ctype = c.get("type", "")
        if ctype == "max_length":
            constraint_hints.append(f"Response must be under {c.get('value', 100)} characters")
        elif ctype == "min_length":
            constraint_hints.append(f"Response must be at least {c.get('value', 10)} characters")
        elif ctype == "contains":
            constraint_hints.append(f"Response must contain: {c.get('value', '')}")
        elif ctype == "not_contains":
            constraint_hints.append(f"Response must NOT contain: {c.get('value', '')}")
        elif ctype == "starts_with":
            constraint_hints.append(f"Response must start with: {c.get('value', '')}")
        elif ctype == "ends_with":
            constraint_hints.append(f"Response must end with: {c.get('value', '')}")
        elif ctype == "regex":
            constraint_hints.append(f"Response must match pattern: {c.get('pattern', '')}")

    prompt = request.prompt
    if constraint_hints:
        prompt += "\n\nIMPORTANT constraints:\n" + "\n".join(f"- {h}" for h in constraint_hints)

    # Generate with retries
    max_attempts = 3
    best_response = ""
    best_score = 0

    for _ in range(max_attempts):
        response = await llm_client.generate(prompt, temperature=request.temperature)
        response = response.strip()

        # Check constraints
        satisfied_count = 0
        for c in request.constraints:
            ctype = c.get("type", "")
            value = c.get("value", "")

            if ctype == "max_length" and len(response) <= int(value):
                satisfied_count += 1
            elif ctype == "min_length" and len(response) >= int(value):
                satisfied_count += 1
            elif ctype == "contains" and value in response:
                satisfied_count += 1
            elif ctype == "not_contains" and value not in response:
                satisfied_count += 1
            elif ctype == "starts_with" and response.startswith(value):
                satisfied_count += 1
            elif ctype == "ends_with" and response.endswith(value):
                satisfied_count += 1
            elif ctype == "regex" and re.match(c.get("pattern", ""), response):
                satisfied_count += 1

        if satisfied_count > best_score:
            best_score = satisfied_count
            best_response = response

        if satisfied_count == len(request.constraints):
            break

    # Final check
    constraints_checked = []
    all_satisfied = True
    for c in request.constraints:
        ctype = c.get("type", "")
        value = c.get("value", "")
        satisfied = False

        if ctype == "max_length":
            satisfied = len(best_response) <= int(value)
        elif ctype == "min_length":
            satisfied = len(best_response) >= int(value)
        elif ctype == "contains":
            satisfied = value in best_response
        elif ctype == "not_contains":
            satisfied = value not in best_response
        elif ctype == "starts_with":
            satisfied = best_response.startswith(value)
        elif ctype == "ends_with":
            satisfied = best_response.endswith(value)
        elif ctype == "regex":
            satisfied = bool(re.match(c.get("pattern", ""), best_response))

        constraints_checked.append({**c, "satisfied": satisfied})
        all_satisfied = all_satisfied and satisfied

    return ConstrainedResponse(
        text=best_response,
        constraints_checked=constraints_checked,
        all_satisfied=all_satisfied,
    )


@app.post("/decode", response_model=DecodingResponse)
async def custom_decode(request: DecodingRequest):
    """Apply custom decoding strategy."""

    outputs = []
    metadata = {"strategy": request.strategy}

    if request.strategy == "argmax":
        # Single greedy decode
        response = await llm_client.generate(request.prompt, temperature=0.0)
        outputs = [response.strip()]

    elif request.strategy == "sample":
        # Multiple samples
        for i in range(request.num_samples):
            response = await llm_client.generate(
                request.prompt,
                temperature=request.temperature + (i * 0.1),
            )
            outputs.append(response.strip())
        metadata["temperatures_used"] = [request.temperature + (i * 0.1) for i in range(request.num_samples)]

    elif request.strategy == "beam":
        # Simulated beam search via multiple samples
        candidates = []
        for _ in range(request.beam_width * 2):
            response = await llm_client.generate(
                request.prompt,
                temperature=request.temperature,
            )
            candidates.append(response.strip())

        # Score by length and select top beam_width
        # In a real implementation, this would use log probabilities
        scored = [(c, len(c)) for c in candidates]
        scored.sort(key=lambda x: x[1], reverse=True)
        outputs = [c[0] for c in scored[:request.beam_width]]
        metadata["beam_width"] = request.beam_width
        metadata["candidates_evaluated"] = len(candidates)

    else:
        raise HTTPException(status_code=400, detail=f"Unknown strategy: {request.strategy}")

    return DecodingResponse(
        outputs=outputs,
        strategy_used=request.strategy,
        metadata=metadata,
    )


@app.post("/score")
async def score_completions(prompt: str, completions: list[str]):
    """Score multiple completions for a prompt (simulated)."""

    # In a real implementation, this would use log probabilities
    # Here we simulate by asking the LLM to rank them

    completions_str = "\n".join([f"{i+1}. {c[:100]}" for i, c in enumerate(completions)])

    ranking_prompt = f"""Given this prompt: "{prompt}"

Rank these completions from best to worst (most appropriate to least appropriate):
{completions_str}

Ranking (best to worst, just list the numbers):"""

    response = await llm_client.generate(ranking_prompt, temperature=0.1)

    # Parse ranking
    numbers = re.findall(r"\d+", response)
    ranking = []
    for num in numbers:
        idx = int(num) - 1
        if 0 <= idx < len(completions) and idx not in ranking:
            ranking.append(idx)

    # Fill in any missing
    for i in range(len(completions)):
        if i not in ranking:
            ranking.append(i)

    # Create scores (higher is better)
    scores = {}
    for rank, idx in enumerate(ranking):
        if idx < len(completions):
            scores[completions[idx]] = 1.0 - (rank / len(ranking))

    return {
        "prompt": prompt,
        "scores": scores,
        "ranking": ranking,
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8004)
