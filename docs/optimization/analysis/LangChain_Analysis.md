# LangChain - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/langchain-ai/langchain
- **Language**: Python
- **Purpose**: Framework for orchestrating multi-step LLM calls, tools, and memory
- **License**: MIT

## Core Architecture

### Directory Structure

```
langchain/
├── libs/
│   ├── langchain/
│   │   ├── chains/              # Chain implementations
│   │   │   ├── base.py          # Base chain class
│   │   │   ├── llm_chain.py     # Basic LLM chain
│   │   │   ├── sequential.py    # Sequential chains
│   │   │   └── router/          # Router chains
│   │   ├── agents/              # Agent implementations
│   │   │   ├── agent.py         # Base agent
│   │   │   ├── react/           # ReAct agent
│   │   │   └── structured_chat/ # Structured chat agent
│   │   ├── memory/              # Memory systems
│   │   │   ├── buffer.py        # Buffer memory
│   │   │   ├── summary.py       # Summary memory
│   │   │   └── entity.py        # Entity memory
│   │   └── tools/               # Tool definitions
│   └── langchain-core/
│       ├── runnables/           # LCEL primitives
│       └── prompts/             # Prompt templates
```

### Key Components

#### 1. Chain Architecture (`chains/base.py`)

**Base Chain Interface**

```python
from abc import ABC, abstractmethod
from typing import Dict, Any, List, Optional

class Chain(ABC):
    """Base class for all chains."""

    @property
    @abstractmethod
    def input_keys(self) -> List[str]:
        """Input keys this chain expects."""
        pass

    @property
    @abstractmethod
    def output_keys(self) -> List[str]:
        """Output keys this chain produces."""
        pass

    @abstractmethod
    def _call(self, inputs: Dict[str, Any]) -> Dict[str, Any]:
        """Execute the chain."""
        pass

    def __call__(self, inputs: Dict[str, Any]) -> Dict[str, Any]:
        """Run the chain with callbacks and validation."""
        # Validate inputs
        self._validate_inputs(inputs)

        # Run chain
        outputs = self._call(inputs)

        # Validate outputs
        self._validate_outputs(outputs)

        return outputs

    async def acall(self, inputs: Dict[str, Any]) -> Dict[str, Any]:
        """Async version of __call__."""
        self._validate_inputs(inputs)
        outputs = await self._acall(inputs)
        self._validate_outputs(outputs)
        return outputs


class LLMChain(Chain):
    """Chain that calls an LLM with a prompt."""

    def __init__(self, llm: LLM, prompt: PromptTemplate):
        self.llm = llm
        self.prompt = prompt

    @property
    def input_keys(self) -> List[str]:
        return self.prompt.input_variables

    @property
    def output_keys(self) -> List[str]:
        return ["text"]

    def _call(self, inputs: Dict[str, Any]) -> Dict[str, Any]:
        # Format prompt
        prompt_value = self.prompt.format(**inputs)

        # Call LLM
        response = self.llm.generate([prompt_value])

        return {"text": response.generations[0][0].text}
```

#### 2. Sequential Chain (`chains/sequential.py`)

```python
class SequentialChain(Chain):
    """Chain that runs multiple chains in sequence."""

    def __init__(self, chains: List[Chain], input_variables: List[str], output_variables: List[str]):
        self.chains = chains
        self._input_keys = input_variables
        self._output_keys = output_variables

        # Validate chain connections
        self._validate_chain_connections()

    def _validate_chain_connections(self):
        """Ensure outputs of each chain connect to inputs of next."""
        available_keys = set(self._input_keys)

        for chain in self.chains:
            missing = set(chain.input_keys) - available_keys
            if missing:
                raise ValueError(f"Chain requires keys not available: {missing}")
            available_keys.update(chain.output_keys)

    @property
    def input_keys(self) -> List[str]:
        return self._input_keys

    @property
    def output_keys(self) -> List[str]:
        return self._output_keys

    def _call(self, inputs: Dict[str, Any]) -> Dict[str, Any]:
        known_values = dict(inputs)

        for chain in self.chains:
            # Get only required inputs for this chain
            chain_inputs = {k: known_values[k] for k in chain.input_keys}

            # Run chain
            outputs = chain(chain_inputs)

            # Add outputs to known values
            known_values.update(outputs)

        # Return only requested output variables
        return {k: known_values[k] for k in self._output_keys}
```

#### 3. ReAct Agent (`agents/react/agent.py`)

**ReAct Pattern Implementation**

```python
class ReActAgent:
    """Agent using ReAct (Reasoning + Acting) pattern."""

    PROMPT_TEMPLATE = """Answer the following question using the available tools.

Question: {question}

Available tools:
{tool_descriptions}

Use this format:
Thought: I need to think about what to do
Action: tool_name
Action Input: input to the tool
Observation: result from the tool
... (repeat Thought/Action/Observation as needed)
Thought: I now know the final answer
Final Answer: the final answer

Begin!

{agent_scratchpad}"""

    def __init__(self, llm: LLM, tools: List[Tool], max_iterations: int = 10):
        self.llm = llm
        self.tools = {tool.name: tool for tool in tools}
        self.max_iterations = max_iterations

    def run(self, question: str) -> str:
        """Execute ReAct loop."""
        agent_scratchpad = ""

        for i in range(self.max_iterations):
            # Format prompt
            prompt = self.PROMPT_TEMPLATE.format(
                question=question,
                tool_descriptions=self._format_tools(),
                agent_scratchpad=agent_scratchpad
            )

            # Get LLM response
            response = self.llm.generate([prompt]).generations[0][0].text

            # Parse response
            action, action_input, is_final = self._parse_response(response)

            if is_final:
                return action_input  # Final answer

            # Execute tool
            if action not in self.tools:
                observation = f"Tool '{action}' not found. Available: {list(self.tools.keys())}"
            else:
                try:
                    observation = self.tools[action].run(action_input)
                except Exception as e:
                    observation = f"Error: {str(e)}"

            # Update scratchpad
            agent_scratchpad += f"\n{response}\nObservation: {observation}\n"

        return "Max iterations reached without final answer"

    def _parse_response(self, response: str) -> Tuple[str, str, bool]:
        """Parse LLM response to extract action."""
        if "Final Answer:" in response:
            answer = response.split("Final Answer:")[-1].strip()
            return "", answer, True

        # Extract Action and Action Input
        action_match = re.search(r"Action:\s*(.+)", response)
        input_match = re.search(r"Action Input:\s*(.+)", response)

        action = action_match.group(1).strip() if action_match else ""
        action_input = input_match.group(1).strip() if input_match else ""

        return action, action_input, False

    def _format_tools(self) -> str:
        """Format tool descriptions for prompt."""
        return "\n".join([
            f"- {name}: {tool.description}"
            for name, tool in self.tools.items()
        ])
```

#### 4. Memory Systems (`memory/`)

```python
class ConversationBufferMemory:
    """Simple buffer memory storing full conversation."""

    def __init__(self, memory_key: str = "history"):
        self.memory_key = memory_key
        self.buffer: List[Dict[str, str]] = []

    def add_user_message(self, message: str):
        self.buffer.append({"role": "user", "content": message})

    def add_ai_message(self, message: str):
        self.buffer.append({"role": "assistant", "content": message})

    def load_memory_variables(self) -> Dict[str, str]:
        """Load memory as string for prompt."""
        history = "\n".join([
            f"{msg['role']}: {msg['content']}"
            for msg in self.buffer
        ])
        return {self.memory_key: history}

    def clear(self):
        self.buffer = []


class ConversationSummaryMemory:
    """Memory that summarizes conversation to save tokens."""

    def __init__(self, llm: LLM, memory_key: str = "summary"):
        self.llm = llm
        self.memory_key = memory_key
        self.summary = ""
        self.buffer: List[Dict[str, str]] = []

    def add_message(self, role: str, content: str):
        self.buffer.append({"role": role, "content": content})

        # Summarize when buffer gets large
        if len(self.buffer) > 10:
            self._summarize()

    def _summarize(self):
        """Summarize buffer into summary."""
        conversation = "\n".join([
            f"{msg['role']}: {msg['content']}"
            for msg in self.buffer
        ])

        prompt = f"""Summarize the following conversation, preserving key facts and context:

Previous summary: {self.summary}

New conversation:
{conversation}

Updated summary:"""

        response = self.llm.generate([prompt]).generations[0][0].text
        self.summary = response.strip()
        self.buffer = []

    def load_memory_variables(self) -> Dict[str, str]:
        return {self.memory_key: self.summary}
```

#### 5. Task Decomposition

```python
class TaskDecomposer:
    """Decompose complex tasks into subtasks."""

    DECOMPOSE_PROMPT = """Break down the following complex task into smaller, manageable subtasks.
Each subtask should be specific and actionable.

Task: {task}

Subtasks (numbered list):"""

    def __init__(self, llm: LLM):
        self.llm = llm

    def decompose(self, task: str) -> List[str]:
        """Decompose task into subtasks."""
        prompt = self.DECOMPOSE_PROMPT.format(task=task)
        response = self.llm.generate([prompt]).generations[0][0].text

        # Parse numbered list
        subtasks = []
        for line in response.strip().split("\n"):
            # Remove numbering
            cleaned = re.sub(r"^\d+\.\s*", "", line.strip())
            if cleaned:
                subtasks.append(cleaned)

        return subtasks


class PlanAndExecuteAgent:
    """Agent that plans then executes."""

    def __init__(self, llm: LLM, tools: List[Tool], decomposer: TaskDecomposer):
        self.llm = llm
        self.tools = tools
        self.decomposer = decomposer
        self.executor = ReActAgent(llm, tools)

    def run(self, task: str) -> str:
        """Plan and execute task."""
        # 1. Decompose into subtasks
        subtasks = self.decomposer.decompose(task)

        # 2. Execute each subtask
        results = []
        for subtask in subtasks:
            result = self.executor.run(subtask)
            results.append({"task": subtask, "result": result})

        # 3. Synthesize final answer
        synthesis_prompt = f"""Given the following task and subtask results, provide a final answer.

Original task: {task}

Subtask results:
{self._format_results(results)}

Final answer:"""

        response = self.llm.generate([synthesis_prompt]).generations[0][0].text
        return response.strip()

    def _format_results(self, results: List[Dict]) -> str:
        return "\n".join([
            f"- {r['task']}: {r['result']}"
            for r in results
        ])
```

## Go Client Implementation

Since LangChain has complex Python dependencies, we implement an HTTP client with local chain abstractions.

### Client Implementation

```go
// internal/optimization/langchain/client.go

package langchain

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// LangChainClient communicates with LangChain server
type LangChainClient struct {
    baseURL    string
    httpClient *http.Client
}

// Config holds client configuration
type Config struct {
    BaseURL string
    Timeout time.Duration
}

// NewLangChainClient creates a new client
func NewLangChainClient(config *Config) *LangChainClient {
    return &LangChainClient{
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// ChainRequest is a chain execution request
type ChainRequest struct {
    ChainType string         `json:"chain_type"`
    Inputs    map[string]any `json:"inputs"`
    Config    *ChainConfig   `json:"config,omitempty"`
}

// ChainConfig configures chain execution
type ChainConfig struct {
    MaxIterations int      `json:"max_iterations,omitempty"`
    Tools         []string `json:"tools,omitempty"`
    Memory        string   `json:"memory,omitempty"`
}

// ChainResponse is a chain execution response
type ChainResponse struct {
    Outputs   map[string]any   `json:"outputs"`
    Steps     []ExecutionStep  `json:"steps,omitempty"`
    LatencyMS float64          `json:"latency_ms"`
}

// ExecutionStep represents a single step in chain execution
type ExecutionStep struct {
    Type   string `json:"type"`
    Input  string `json:"input"`
    Output string `json:"output"`
}

// ExecuteChain runs a chain
func (c *LangChainClient) ExecuteChain(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/chain/execute", bytes.NewReader(body))
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

    var result ChainResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// AgentRequest is an agent execution request
type AgentRequest struct {
    Task          string   `json:"task"`
    AgentType     string   `json:"agent_type"` // "react", "plan_execute"
    Tools         []string `json:"tools"`
    MaxIterations int      `json:"max_iterations,omitempty"`
}

// AgentResponse is an agent execution response
type AgentResponse struct {
    Result    string          `json:"result"`
    Steps     []AgentStep     `json:"steps"`
    LatencyMS float64         `json:"latency_ms"`
}

// AgentStep represents an agent reasoning step
type AgentStep struct {
    Thought     string `json:"thought"`
    Action      string `json:"action,omitempty"`
    ActionInput string `json:"action_input,omitempty"`
    Observation string `json:"observation,omitempty"`
}

// RunAgent executes an agent
func (c *LangChainClient) RunAgent(ctx context.Context, req *AgentRequest) (*AgentResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/agent/run", bytes.NewReader(body))
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

    var result AgentResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// DecomposeRequest is a task decomposition request
type DecomposeRequest struct {
    Task string `json:"task"`
}

// DecomposeResponse is a task decomposition response
type DecomposeResponse struct {
    Subtasks []string `json:"subtasks"`
}

// DecomposeTask breaks a task into subtasks
func (c *LangChainClient) DecomposeTask(ctx context.Context, task string) (*DecomposeResponse, error) {
    body, err := json.Marshal(&DecomposeRequest{Task: task})
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/decompose", bytes.NewReader(body))
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

    var result DecomposeResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}
```

### Local Chain Abstractions

```go
// internal/optimization/langchain/chains.go

package langchain

import (
    "context"
)

// Chain defines the chain interface
type Chain interface {
    InputKeys() []string
    OutputKeys() []string
    Call(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

// SequentialChain runs chains in sequence
type SequentialChain struct {
    chains        []Chain
    inputKeys     []string
    outputKeys    []string
}

// NewSequentialChain creates a new sequential chain
func NewSequentialChain(chains []Chain, inputKeys, outputKeys []string) *SequentialChain {
    return &SequentialChain{
        chains:     chains,
        inputKeys:  inputKeys,
        outputKeys: outputKeys,
    }
}

func (c *SequentialChain) InputKeys() []string  { return c.inputKeys }
func (c *SequentialChain) OutputKeys() []string { return c.outputKeys }

func (c *SequentialChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    knownValues := make(map[string]any)
    for k, v := range inputs {
        knownValues[k] = v
    }

    for _, chain := range c.chains {
        // Get inputs for this chain
        chainInputs := make(map[string]any)
        for _, key := range chain.InputKeys() {
            if v, ok := knownValues[key]; ok {
                chainInputs[key] = v
            }
        }

        // Run chain
        outputs, err := chain.Call(ctx, chainInputs)
        if err != nil {
            return nil, err
        }

        // Add outputs to known values
        for k, v := range outputs {
            knownValues[k] = v
        }
    }

    // Return only requested outputs
    result := make(map[string]any)
    for _, key := range c.outputKeys {
        if v, ok := knownValues[key]; ok {
            result[key] = v
        }
    }

    return result, nil
}

// RemoteChain wraps a LangChain server chain
type RemoteChain struct {
    client    *LangChainClient
    chainType string
    inputs    []string
    outputs   []string
}

// NewRemoteChain creates a chain that calls the LangChain server
func NewRemoteChain(client *LangChainClient, chainType string, inputs, outputs []string) *RemoteChain {
    return &RemoteChain{
        client:    client,
        chainType: chainType,
        inputs:    inputs,
        outputs:   outputs,
    }
}

func (c *RemoteChain) InputKeys() []string  { return c.inputs }
func (c *RemoteChain) OutputKeys() []string { return c.outputs }

func (c *RemoteChain) Call(ctx context.Context, inputs map[string]any) (map[string]any, error) {
    resp, err := c.client.ExecuteChain(ctx, &ChainRequest{
        ChainType: c.chainType,
        Inputs:    inputs,
    })
    if err != nil {
        return nil, err
    }
    return resp.Outputs, nil
}
```

### Agent Abstraction

```go
// internal/optimization/langchain/agents.go

package langchain

import (
    "context"
)

// Agent defines the agent interface
type Agent interface {
    Run(ctx context.Context, task string) (*AgentResult, error)
}

// AgentResult contains agent execution result
type AgentResult struct {
    Result string
    Steps  []AgentStep
}

// RemoteReActAgent wraps a LangChain ReAct agent
type RemoteReActAgent struct {
    client        *LangChainClient
    tools         []string
    maxIterations int
}

// NewRemoteReActAgent creates a new remote ReAct agent
func NewRemoteReActAgent(client *LangChainClient, tools []string, maxIterations int) *RemoteReActAgent {
    return &RemoteReActAgent{
        client:        client,
        tools:         tools,
        maxIterations: maxIterations,
    }
}

// Run executes the ReAct agent
func (a *RemoteReActAgent) Run(ctx context.Context, task string) (*AgentResult, error) {
    resp, err := a.client.RunAgent(ctx, &AgentRequest{
        Task:          task,
        AgentType:     "react",
        Tools:         a.tools,
        MaxIterations: a.maxIterations,
    })
    if err != nil {
        return nil, err
    }

    return &AgentResult{
        Result: resp.Result,
        Steps:  resp.Steps,
    }, nil
}

// PlanAndExecuteAgent plans then executes subtasks
type PlanAndExecuteAgent struct {
    client        *LangChainClient
    tools         []string
    maxIterations int
}

// NewPlanAndExecuteAgent creates a new plan-and-execute agent
func NewPlanAndExecuteAgent(client *LangChainClient, tools []string, maxIterations int) *PlanAndExecuteAgent {
    return &PlanAndExecuteAgent{
        client:        client,
        tools:         tools,
        maxIterations: maxIterations,
    }
}

// Run executes the plan-and-execute agent
func (a *PlanAndExecuteAgent) Run(ctx context.Context, task string) (*AgentResult, error) {
    // First decompose
    decomp, err := a.client.DecomposeTask(ctx, task)
    if err != nil {
        return nil, err
    }

    // Execute each subtask
    var allSteps []AgentStep
    var results []string

    for _, subtask := range decomp.Subtasks {
        resp, err := a.client.RunAgent(ctx, &AgentRequest{
            Task:          subtask,
            AgentType:     "react",
            Tools:         a.tools,
            MaxIterations: a.maxIterations,
        })
        if err != nil {
            return nil, err
        }

        allSteps = append(allSteps, resp.Steps...)
        results = append(results, resp.Result)
    }

    // Combine results (simple concatenation - could be more sophisticated)
    finalResult := strings.Join(results, "\n\n")

    return &AgentResult{
        Result: finalResult,
        Steps:  allSteps,
    }, nil
}
```

### Tool Interface

```go
// internal/optimization/langchain/tools.go

package langchain

import (
    "context"
)

// Tool defines a tool that agents can use
type Tool interface {
    Name() string
    Description() string
    Run(ctx context.Context, input string) (string, error)
}

// ToolRegistry manages available tools
type ToolRegistry struct {
    tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
    return &ToolRegistry{
        tools: make(map[string]Tool),
    }
}

// Register adds a tool
func (r *ToolRegistry) Register(tool Tool) {
    r.tools[tool.Name()] = tool
}

// Get retrieves a tool
func (r *ToolRegistry) Get(name string) (Tool, bool) {
    tool, ok := r.tools[name]
    return tool, ok
}

// List returns all tool names
func (r *ToolRegistry) List() []string {
    names := make([]string, 0, len(r.tools))
    for name := range r.tools {
        names = append(names, name)
    }
    return names
}

// CogneeTool wraps Cognee as a tool
type CogneeTool struct {
    cogneeService *services.CogneeService
}

func NewCogneeTool(cognee *services.CogneeService) *CogneeTool {
    return &CogneeTool{cogneeService: cognee}
}

func (t *CogneeTool) Name() string { return "cognee_search" }

func (t *CogneeTool) Description() string {
    return "Search the knowledge base for relevant information. Input should be a search query."
}

func (t *CogneeTool) Run(ctx context.Context, input string) (string, error) {
    results, err := t.cogneeService.SearchMemory(ctx, input, 5)
    if err != nil {
        return "", err
    }

    var output strings.Builder
    for _, r := range results {
        output.WriteString(fmt.Sprintf("- %s (score: %.2f)\n", r.Content, r.Score))
    }
    return output.String(), nil
}
```

## Test Coverage Requirements

```go
// tests/optimization/unit/langchain/client_test.go

func TestLangChainClient_ExecuteChain(t *testing.T)
func TestLangChainClient_RunAgent(t *testing.T)
func TestLangChainClient_DecomposeTask(t *testing.T)

func TestSequentialChain_Call(t *testing.T)
func TestRemoteChain_Call(t *testing.T)

func TestRemoteReActAgent_Run(t *testing.T)
func TestPlanAndExecuteAgent_Run(t *testing.T)

func TestToolRegistry_Register(t *testing.T)
func TestToolRegistry_Get(t *testing.T)
func TestCogneeTool_Run(t *testing.T)
```

## Conclusion

LangChain's value lies in its extensive tool ecosystem and agent patterns. The Go client provides clean abstractions for chains and agents while delegating complex orchestration to the Python service.

**Key Benefits**:
- Task decomposition for complex multi-step projects
- ReAct pattern for tool-using agents
- Memory systems for context management

**Estimated Implementation Time**: 1.5 weeks
**Risk Level**: Medium
**Dependencies**: Python service, tool integrations
