"""
Guidance HTTP Bridge Server.
Provides CFG-based constrained text generation.
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

    async def generate(self, prompt: str, temperature: float = 0.7, stop: list[str] = None) -> str:
        """Generate response from SuperAgent."""
        try:
            payload = {
                "model": "default",
                "messages": [{"role": "user", "content": prompt}],
                "temperature": temperature,
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
    title="Guidance Bridge",
    description="HTTP bridge for Guidance CFG-based generation",
    version="1.0.0",
    lifespan=lifespan,
)


# Request/Response Models
class GrammarRequest(BaseModel):
    """Request for grammar-constrained generation."""
    prompt: str = Field(..., description="The prompt to complete")
    grammar: str = Field(..., description="Grammar specification in simplified BNF")
    max_tokens: int = Field(default=500, description="Maximum tokens to generate")
    temperature: float = Field(default=0.7, description="Sampling temperature")


class GrammarResponse(BaseModel):
    """Response from grammar-constrained generation."""
    text: str
    parsed: dict[str, Any]
    valid: bool


class TemplateRequest(BaseModel):
    """Request for template-based generation."""
    template: str = Field(..., description="Template with {{placeholders}}")
    variables: dict[str, Any] = Field(default_factory=dict, description="Known variables")
    constraints: dict[str, str] = Field(default_factory=dict, description="Constraints for placeholders: regex or select")


class TemplateResponse(BaseModel):
    """Response from template generation."""
    filled_template: str
    generated_values: dict[str, str]


class SelectRequest(BaseModel):
    """Request for constrained selection."""
    prompt: str = Field(..., description="Context prompt")
    options: list[str] = Field(..., description="Options to select from")
    allow_multiple: bool = Field(default=False, description="Allow multiple selections")


class SelectResponse(BaseModel):
    """Response from selection."""
    selected: list[str]
    reasoning: str


class RegexRequest(BaseModel):
    """Request for regex-constrained generation."""
    prompt: str = Field(..., description="The prompt")
    pattern: str = Field(..., description="Regex pattern to match")
    max_attempts: int = Field(default=5, description="Maximum generation attempts")


class RegexResponse(BaseModel):
    """Response from regex generation."""
    text: str
    matches: bool
    match_groups: list[str]


class HealthResponse(BaseModel):
    """Health check response."""
    status: str
    version: str
    llm_available: bool


# Grammar parsing helpers
def parse_simple_grammar(grammar: str) -> dict:
    """Parse a simplified BNF grammar specification."""
    rules = {}
    for line in grammar.strip().split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "::=" in line:
            name, rule = line.split("::=", 1)
            rules[name.strip()] = rule.strip()
    return rules


def validate_against_grammar(text: str, rules: dict, start: str = "start") -> tuple[bool, dict]:
    """Validate text against grammar rules (simplified)."""
    parsed = {}

    # This is a simplified validator
    # In practice, Guidance uses a full CFG parser

    if start not in rules:
        return True, {"text": text}

    rule = rules[start]

    # Handle alternatives (|)
    alternatives = [alt.strip() for alt in rule.split("|")]

    for alt in alternatives:
        # Handle sequence of terminals and non-terminals
        parts = alt.split()
        remaining = text
        alt_parsed = {}
        matched = True

        for part in parts:
            if part.startswith("<") and part.endswith(">"):
                # Non-terminal
                nt_name = part[1:-1]
                if nt_name in rules:
                    # Recursively validate
                    valid, sub_parsed = validate_against_grammar(remaining, rules, nt_name)
                    if valid:
                        alt_parsed[nt_name] = sub_parsed
                        # Consume matched portion (simplified: take first word)
                        words = remaining.split(None, 1)
                        remaining = words[1] if len(words) > 1 else ""
                    else:
                        matched = False
                        break
                else:
                    # Unknown non-terminal, accept anything
                    words = remaining.split(None, 1)
                    alt_parsed[nt_name] = words[0] if words else ""
                    remaining = words[1] if len(words) > 1 else ""
            else:
                # Terminal
                if remaining.startswith(part):
                    remaining = remaining[len(part):].lstrip()
                else:
                    matched = False
                    break

        if matched:
            return True, alt_parsed

    return False, {}


async def generate_with_grammar(prompt: str, grammar_rules: dict, temperature: float) -> tuple[str, dict]:
    """Generate text following grammar rules."""

    # Build a structured prompt that encourages grammar-following output
    grammar_desc = "\n".join([f"  {name} ::= {rule}" for name, rule in grammar_rules.items()])

    structured_prompt = f"""{prompt}

Your response must follow this grammar:
{grammar_desc}

Generate a valid response:"""

    response = await llm_client.generate(structured_prompt, temperature=temperature)

    # Validate and parse
    valid, parsed = validate_against_grammar(response.strip(), grammar_rules)

    return response.strip(), {"valid": valid, "parsed": parsed}


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


@app.post("/grammar", response_model=GrammarResponse)
async def grammar_generate(request: GrammarRequest):
    """Generate text following a grammar specification."""

    try:
        rules = parse_simple_grammar(request.grammar)

        if not rules:
            raise HTTPException(status_code=400, detail="Invalid or empty grammar")

        text, result = await generate_with_grammar(
            request.prompt,
            rules,
            request.temperature,
        )

        return GrammarResponse(
            text=text,
            parsed=result.get("parsed", {}),
            valid=result.get("valid", False),
        )

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/template", response_model=TemplateResponse)
async def template_generate(request: TemplateRequest):
    """Generate text using a template with constraints."""

    template = request.template
    generated = {}

    # Find all placeholders
    placeholders = re.findall(r"\{\{(\w+)\}\}", template)

    for placeholder in placeholders:
        if placeholder in request.variables:
            # Use provided value
            generated[placeholder] = str(request.variables[placeholder])
        else:
            # Generate value
            constraint = request.constraints.get(placeholder, "")

            if constraint.startswith("select:"):
                # Selection constraint
                options = constraint[7:].split(",")
                prompt = f"Choose one of: {', '.join(options)}\nContext: {template}\nSelection:"
                response = await llm_client.generate(prompt, temperature=0.3, stop=["\n"])

                # Find which option was selected
                response_lower = response.lower()
                for opt in options:
                    if opt.lower() in response_lower:
                        generated[placeholder] = opt.strip()
                        break
                else:
                    generated[placeholder] = options[0].strip()  # Default to first

            elif constraint.startswith("regex:"):
                # Regex constraint - generate and validate
                pattern = constraint[6:]
                for _ in range(5):  # Max attempts
                    prompt = f"Generate a value for '{placeholder}' that matches pattern: {pattern}\nContext: {template}\nValue:"
                    response = await llm_client.generate(prompt, temperature=0.5, stop=["\n"])
                    response = response.strip()
                    if re.match(pattern, response):
                        generated[placeholder] = response
                        break
                else:
                    generated[placeholder] = f"[{placeholder}]"

            else:
                # Free generation
                prompt = f"Generate a suitable value for '{placeholder}'.\nContext: {template}\nValue:"
                response = await llm_client.generate(prompt, temperature=0.7, stop=["\n"])
                generated[placeholder] = response.strip()

    # Fill template
    filled = template
    for placeholder, value in generated.items():
        filled = filled.replace(f"{{{{{placeholder}}}}}", value)

    return TemplateResponse(
        filled_template=filled,
        generated_values=generated,
    )


@app.post("/select", response_model=SelectResponse)
async def select_option(request: SelectRequest):
    """Select from constrained options."""

    options_str = "\n".join([f"{i+1}. {opt}" for i, opt in enumerate(request.options)])

    if request.allow_multiple:
        prompt = f"""{request.prompt}

Choose one or more of the following options (list the numbers):
{options_str}

Selected options:"""
    else:
        prompt = f"""{request.prompt}

Choose exactly one of the following options (state the number):
{options_str}

Selected option:"""

    response = await llm_client.generate(prompt, temperature=0.3)

    # Parse selection
    selected = []
    reasoning = response

    # Find numbers in response
    numbers = re.findall(r"\d+", response)
    for num in numbers:
        idx = int(num) - 1
        if 0 <= idx < len(request.options):
            selected.append(request.options[idx])
            if not request.allow_multiple:
                break

    if not selected and request.options:
        # Fallback: look for option text in response
        response_lower = response.lower()
        for opt in request.options:
            if opt.lower() in response_lower:
                selected.append(opt)
                if not request.allow_multiple:
                    break

    return SelectResponse(
        selected=selected if selected else [request.options[0]] if request.options else [],
        reasoning=reasoning,
    )


@app.post("/regex", response_model=RegexResponse)
async def regex_generate(request: RegexRequest):
    """Generate text matching a regex pattern."""

    pattern = request.pattern

    # Describe the pattern for the LLM
    prompt = f"""{request.prompt}

Generate text that matches this pattern: {pattern}
Examples of valid patterns:
- For email: user@domain.com
- For phone: (123) 456-7890
- For date: 2024-01-15

Your response (just the matching text):"""

    for attempt in range(request.max_attempts):
        response = await llm_client.generate(prompt, temperature=0.5 + (attempt * 0.1))
        text = response.strip().split("\n")[0].strip()

        try:
            match = re.match(pattern, text)
            if match:
                return RegexResponse(
                    text=text,
                    matches=True,
                    match_groups=list(match.groups()) if match.groups() else [match.group()],
                )
        except re.error:
            raise HTTPException(status_code=400, detail=f"Invalid regex pattern: {pattern}")

    return RegexResponse(
        text=text,
        matches=False,
        match_groups=[],
    )


@app.post("/json_schema")
async def json_schema_generate(prompt: str, schema: dict[str, Any]):
    """Generate JSON following a schema (bridges to Outlines-style generation)."""

    schema_str = str(schema)

    generation_prompt = f"""{prompt}

Generate a JSON object that follows this schema:
{schema_str}

JSON:"""

    response = await llm_client.generate(generation_prompt, temperature=0.3)

    # Extract JSON from response
    import json
    try:
        # Find JSON in response
        start = response.find("{")
        end = response.rfind("}") + 1
        if start >= 0 and end > start:
            json_str = response[start:end]
            parsed = json.loads(json_str)
            return {"json": parsed, "valid": True}
    except json.JSONDecodeError:
        pass

    return {"json": {}, "valid": False, "raw": response}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8003)
