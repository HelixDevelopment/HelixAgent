"""
LangChain HTTP Bridge Server.
Provides task decomposition, chain execution, and ReAct agent capabilities.
"""

import os
import asyncio
from typing import Any, Optional
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import httpx

# LangChain imports
from langchain_core.prompts import ChatPromptTemplate, PromptTemplate
from langchain_core.output_parsers import StrOutputParser, JsonOutputParser
from langchain_core.runnables import RunnablePassthrough, RunnableLambda
from langchain.chains import LLMChain
from langchain.schema import HumanMessage, SystemMessage

# Configuration
SUPERAGENT_URL = os.getenv("SUPERAGENT_URL", "http://localhost:8080")
LLM_ENDPOINT = os.getenv("LLM_ENDPOINT", f"{SUPERAGENT_URL}/v1/chat/completions")


class SuperAgentLLM:
    """Custom LLM that routes requests through SuperAgent."""

    def __init__(self, model: str = "default", temperature: float = 0.7):
        self.model = model
        self.temperature = temperature
        self.client = httpx.AsyncClient(timeout=120.0)

    async def generate(self, prompt: str) -> str:
        """Generate a response from SuperAgent."""
        try:
            response = await self.client.post(
                LLM_ENDPOINT,
                json={
                    "model": self.model,
                    "messages": [{"role": "user", "content": prompt}],
                    "temperature": self.temperature,
                }
            )
            response.raise_for_status()
            data = response.json()
            return data["choices"][0]["message"]["content"]
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"LLM call failed: {e}")

    async def close(self):
        await self.client.aclose()


# Global LLM instance
llm: Optional[SuperAgentLLM] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    global llm
    llm = SuperAgentLLM()
    yield
    await llm.close()


app = FastAPI(
    title="LangChain Bridge",
    description="HTTP bridge for LangChain capabilities",
    version="1.0.0",
    lifespan=lifespan,
)


# Request/Response Models
class DecomposeRequest(BaseModel):
    """Request for task decomposition."""
    task: str = Field(..., description="The task to decompose")
    max_steps: int = Field(default=5, description="Maximum number of steps")
    context: Optional[str] = Field(default=None, description="Additional context")


class DecomposeResponse(BaseModel):
    """Response containing decomposed subtasks."""
    subtasks: list[dict[str, Any]]
    reasoning: str


class ChainRequest(BaseModel):
    """Request for chain execution."""
    chain_type: str = Field(..., description="Type of chain: basic, sequential, or router")
    prompt: str = Field(..., description="Input prompt")
    variables: dict[str, Any] = Field(default_factory=dict, description="Template variables")
    temperature: float = Field(default=0.7, description="LLM temperature")


class ChainResponse(BaseModel):
    """Response from chain execution."""
    result: str
    steps: list[dict[str, Any]] = Field(default_factory=list)


class ReActRequest(BaseModel):
    """Request for ReAct agent execution."""
    goal: str = Field(..., description="The goal to achieve")
    available_tools: list[str] = Field(default_factory=list, description="Available tool names")
    max_iterations: int = Field(default=10, description="Maximum reasoning iterations")
    context: Optional[str] = Field(default=None, description="Additional context")


class ReActResponse(BaseModel):
    """Response from ReAct agent."""
    answer: str
    reasoning_trace: list[dict[str, Any]]
    tools_used: list[str]
    iterations: int


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    version: str
    llm_available: bool


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


@app.post("/decompose", response_model=DecomposeResponse)
async def decompose_task(request: DecomposeRequest):
    """Decompose a complex task into subtasks."""

    decomposition_prompt = f"""You are a task decomposition expert. Break down the following task into smaller, actionable subtasks.

Task: {request.task}
{f'Context: {request.context}' if request.context else ''}

Provide exactly {request.max_steps} or fewer subtasks. For each subtask, provide:
1. A clear description
2. Dependencies on other subtasks (by number)
3. Estimated complexity (low, medium, high)

Format your response as JSON:
{{
    "reasoning": "Brief explanation of your decomposition approach",
    "subtasks": [
        {{
            "id": 1,
            "description": "Subtask description",
            "dependencies": [],
            "complexity": "low|medium|high"
        }}
    ]
}}
"""

    try:
        response = await llm.generate(decomposition_prompt)

        # Parse JSON response
        import json
        # Find JSON in response
        start = response.find("{")
        end = response.rfind("}") + 1
        if start >= 0 and end > start:
            parsed = json.loads(response[start:end])
            return DecomposeResponse(
                subtasks=parsed.get("subtasks", []),
                reasoning=parsed.get("reasoning", ""),
            )

        # Fallback if no valid JSON
        return DecomposeResponse(
            subtasks=[{"id": 1, "description": request.task, "dependencies": [], "complexity": "high"}],
            reasoning="Could not parse decomposition, returning original task",
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/chain", response_model=ChainResponse)
async def execute_chain(request: ChainRequest):
    """Execute a LangChain chain."""

    steps = []

    try:
        if request.chain_type == "basic":
            # Simple prompt -> LLM -> output chain
            prompt = request.prompt
            for key, value in request.variables.items():
                prompt = prompt.replace(f"{{{key}}}", str(value))

            result = await llm.generate(prompt)
            steps.append({"step": "generate", "input": prompt[:100], "output": result[:100]})

        elif request.chain_type == "sequential":
            # Multi-step chain
            current = request.prompt
            for i, var_set in enumerate(request.variables.get("steps", [{"prompt": request.prompt}])):
                step_prompt = var_set.get("prompt", current)
                result = await llm.generate(step_prompt)
                steps.append({"step": f"step_{i+1}", "input": step_prompt[:100], "output": result[:100]})
                current = result
            result = current

        elif request.chain_type == "router":
            # Route to different prompts based on input
            routes = request.variables.get("routes", {})
            default_route = request.variables.get("default", request.prompt)

            # Determine which route to use
            classification_prompt = f"""Given the input, classify it into one of these categories: {list(routes.keys())}
Input: {request.prompt}
Category:"""

            category = (await llm.generate(classification_prompt)).strip().lower()
            steps.append({"step": "classify", "category": category})

            route_prompt = routes.get(category, default_route)
            result = await llm.generate(f"{route_prompt}\n\nInput: {request.prompt}")
            steps.append({"step": "route_execute", "route": category, "output": result[:100]})

        else:
            raise HTTPException(status_code=400, detail=f"Unknown chain type: {request.chain_type}")

        return ChainResponse(result=result, steps=steps)

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/react", response_model=ReActResponse)
async def react_agent(request: ReActRequest):
    """Execute a ReAct (Reason + Act) agent."""

    reasoning_trace = []
    tools_used = []

    # Available tools simulation
    tool_descriptions = {
        "search": "Search for information on a topic",
        "calculate": "Perform mathematical calculations",
        "lookup": "Look up specific facts or data",
        "summarize": "Summarize given text",
    }

    available = [t for t in request.available_tools if t in tool_descriptions]
    tool_desc = "\n".join([f"- {t}: {tool_descriptions[t]}" for t in available])

    react_prompt = f"""You are an AI assistant using the ReAct framework.
Available tools:
{tool_desc if tool_desc else "No tools available - use your knowledge only"}

Goal: {request.goal}
{f'Context: {request.context}' if request.context else ''}

For each step, output in this format:
Thought: [Your reasoning about what to do next]
Action: [tool_name] or [Final Answer]
Action Input: [Input for the tool or your final answer]
Observation: [Result of the action - I'll fill this in]

Begin!"""

    current_prompt = react_prompt
    iteration = 0
    final_answer = None

    while iteration < request.max_iterations:
        iteration += 1

        response = await llm.generate(current_prompt)

        # Parse the response
        lines = response.strip().split("\n")
        thought = ""
        action = ""
        action_input = ""

        for line in lines:
            line = line.strip()
            if line.startswith("Thought:"):
                thought = line[8:].strip()
            elif line.startswith("Action:"):
                action = line[7:].strip().lower()
            elif line.startswith("Action Input:"):
                action_input = line[13:].strip()

        step = {
            "iteration": iteration,
            "thought": thought,
            "action": action,
            "action_input": action_input,
        }

        # Check for final answer
        if "final answer" in action.lower() or action == "":
            final_answer = action_input if action_input else thought
            step["observation"] = "Task completed"
            reasoning_trace.append(step)
            break

        # Simulate tool execution
        if action in available:
            tools_used.append(action)
            observation = f"[Simulated {action} result for: {action_input[:50]}...]"
        else:
            observation = f"Tool '{action}' not available. Available tools: {available}"

        step["observation"] = observation
        reasoning_trace.append(step)

        # Continue with observation
        current_prompt = f"{current_prompt}\n{response}\nObservation: {observation}\n"

    if final_answer is None:
        final_answer = f"Reached maximum iterations ({request.max_iterations}). Last thought: {reasoning_trace[-1].get('thought', 'None')}"

    return ReActResponse(
        answer=final_answer,
        reasoning_trace=reasoning_trace,
        tools_used=list(set(tools_used)),
        iterations=iteration,
    )


@app.post("/summarize")
async def summarize_chain(text: str, max_length: int = 200):
    """Summarize text using a chain."""
    prompt = f"""Summarize the following text in no more than {max_length} words:

{text}

Summary:"""

    result = await llm.generate(prompt)
    return {"summary": result.strip()}


@app.post("/transform")
async def transform_chain(text: str, transformation: str):
    """Transform text according to instructions."""
    prompt = f"""Transform the following text according to these instructions: {transformation}

Text:
{text}

Transformed text:"""

    result = await llm.generate(prompt)
    return {"result": result.strip()}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
