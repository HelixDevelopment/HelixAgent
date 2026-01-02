# Guidance - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/guidance-ai/guidance
- **Language**: Python
- **Purpose**: Pythonic library for controlling LLM output with regex, CFGs, and interleaved control logic
- **License**: MIT

## Core Architecture

### Directory Structure

```
guidance/
├── guidance/
│   ├── _grammar.py          # Grammar node definitions
│   ├── _gen.py              # Generation primitives
│   ├── _select.py           # Selection from options
│   ├── _parser.py           # Grammar parser
│   ├── library/             # Built-in functions
│   │   ├── gen.py           # Generation functions
│   │   ├── select.py        # Selection functions
│   │   └── regex.py         # Regex patterns
│   ├── models/              # Model backends
│   │   ├── _model.py        # Base model class
│   │   ├── transformers/    # HuggingFace integration
│   │   ├── openai.py        # OpenAI integration
│   │   └── llama_cpp/       # llama.cpp integration
│   └── _utils.py            # Utility functions
```

### Key Components

#### 1. Grammar Node System (`_grammar.py`)

**Base Grammar Node**

```python
from abc import ABC, abstractmethod
from typing import Optional, List, Any

class GrammarNode(ABC):
    """Base class for all grammar nodes."""

    def __init__(self, name: Optional[str] = None):
        self.name = name  # Variable name to store result
        self.parent: Optional[GrammarNode] = None
        self.children: List[GrammarNode] = []

    @abstractmethod
    def match(self, text: str, pos: int) -> Optional[int]:
        """Try to match at position, return new position or None."""
        pass

    @abstractmethod
    def get_allowed_tokens(self, state: GenerationState) -> List[int]:
        """Get tokens allowed in current state."""
        pass


class Literal(GrammarNode):
    """Match exact literal text."""

    def __init__(self, value: str, name: Optional[str] = None):
        super().__init__(name)
        self.value = value

    def match(self, text: str, pos: int) -> Optional[int]:
        if text[pos:pos+len(self.value)] == self.value:
            return pos + len(self.value)
        return None

    def get_allowed_tokens(self, state: GenerationState) -> List[int]:
        remaining = self.value[state.matched_len:]
        return state.tokenizer.get_tokens_starting_with(remaining)


class Regex(GrammarNode):
    """Match regex pattern."""

    def __init__(self, pattern: str, name: Optional[str] = None):
        super().__init__(name)
        self.pattern = re.compile(pattern)

    def match(self, text: str, pos: int) -> Optional[int]:
        match = self.pattern.match(text, pos)
        if match:
            return match.end()
        return None

    def get_allowed_tokens(self, state: GenerationState) -> List[int]:
        # Complex: need to check which tokens could continue valid match
        return self._compute_allowed_from_regex_state(state)


class Sequence(GrammarNode):
    """Match sequence of nodes."""

    def __init__(self, *nodes: GrammarNode, name: Optional[str] = None):
        super().__init__(name)
        self.children = list(nodes)

    def match(self, text: str, pos: int) -> Optional[int]:
        current_pos = pos
        for node in self.children:
            result = node.match(text, current_pos)
            if result is None:
                return None
            current_pos = result
        return current_pos


class Choice(GrammarNode):
    """Match one of several options."""

    def __init__(self, *options: GrammarNode, name: Optional[str] = None):
        super().__init__(name)
        self.children = list(options)

    def match(self, text: str, pos: int) -> Optional[int]:
        for option in self.children:
            result = option.match(text, pos)
            if result is not None:
                return result
        return None

    def get_allowed_tokens(self, state: GenerationState) -> List[int]:
        # Union of all options' allowed tokens
        allowed = set()
        for option in self.children:
            allowed.update(option.get_allowed_tokens(state))
        return list(allowed)


class Repeat(GrammarNode):
    """Repeat node min to max times."""

    def __init__(self, node: GrammarNode, min_count: int = 0,
                 max_count: Optional[int] = None, name: Optional[str] = None):
        super().__init__(name)
        self.children = [node]
        self.min_count = min_count
        self.max_count = max_count
```

#### 2. Generation Control (`_gen.py`)

```python
class Gen(GrammarNode):
    """Generate text with constraints."""

    def __init__(
        self,
        name: str,
        regex: Optional[str] = None,
        stop: Optional[str] = None,
        max_tokens: int = 100,
        temperature: float = 0.0
    ):
        super().__init__(name)
        self.regex = re.compile(regex) if regex else None
        self.stop = stop
        self.max_tokens = max_tokens
        self.temperature = temperature

    def generate(self, model: Model, state: GenerationState) -> str:
        """Generate text using model with constraints."""
        generated = []
        tokens_generated = 0

        while tokens_generated < self.max_tokens:
            # Get allowed tokens based on constraints
            if self.regex:
                allowed = self._get_regex_allowed_tokens(state)
            else:
                allowed = None  # All tokens allowed

            # Sample next token
            logits = model.get_logits(state.context)
            if allowed is not None:
                logits = self._mask_logits(logits, allowed)

            token_id = self._sample(logits, self.temperature)
            token = state.tokenizer.decode([token_id])

            # Check stop condition
            if self.stop and token == self.stop:
                break

            generated.append(token)
            state.context.append(token_id)
            tokens_generated += 1

            # Check if regex is complete
            if self.regex:
                current = "".join(generated)
                if self.regex.fullmatch(current):
                    break

        return "".join(generated)

    def _get_regex_allowed_tokens(self, state: GenerationState) -> List[int]:
        """Get tokens that could continue matching the regex."""
        current = state.get_current_generation()
        allowed = []

        for token_id, token_str in enumerate(state.tokenizer.vocab):
            candidate = current + token_str
            # Check if candidate could still match
            if self._could_match_regex(candidate):
                allowed.append(token_id)

        return allowed

    def _could_match_regex(self, text: str) -> bool:
        """Check if text could be prefix of regex match."""
        # This is simplified - real implementation uses DFA
        return self.regex.match(text) is not None or \
               any(self.regex.match(text + c) for c in "abcdefghij...")
```

#### 3. Select Function (`_select.py`)

```python
class Select(GrammarNode):
    """Select from predefined options."""

    def __init__(self, options: List[str], name: str):
        super().__init__(name)
        self.options = options

    def generate(self, model: Model, state: GenerationState) -> str:
        """Select best option based on model probabilities."""
        # Get log probabilities for each option
        option_scores = []

        for option in self.options:
            tokens = state.tokenizer.encode(option)
            score = 0.0

            temp_context = state.context.copy()
            for token in tokens:
                logits = model.get_logits(temp_context)
                log_probs = F.log_softmax(torch.tensor(logits), dim=-1)
                score += log_probs[token].item()
                temp_context.append(token)

            option_scores.append((option, score))

        # Select highest scoring option
        best_option = max(option_scores, key=lambda x: x[1])[0]
        return best_option

    def get_allowed_tokens(self, state: GenerationState) -> List[int]:
        """Get first tokens of all options."""
        current_gen = state.get_current_generation()
        allowed = set()

        for option in self.options:
            if option.startswith(current_gen):
                remaining = option[len(current_gen):]
                if remaining:
                    first_tokens = state.tokenizer.get_tokens_starting_with(remaining[0])
                    allowed.update(first_tokens)

        return list(allowed)
```

#### 4. Program Execution (`_parser.py`)

```python
class GuidanceProgram:
    """Executable guidance program."""

    def __init__(self, template: str):
        self.template = template
        self.nodes = self._parse(template)

    def _parse(self, template: str) -> List[GrammarNode]:
        """Parse template into grammar nodes."""
        nodes = []
        current_pos = 0

        while current_pos < len(template):
            # Look for {{...}} blocks
            start = template.find("{{", current_pos)

            if start == -1:
                # Rest is literal
                nodes.append(Literal(template[current_pos:]))
                break

            # Add literal before block
            if start > current_pos:
                nodes.append(Literal(template[current_pos:start]))

            # Parse block
            end = template.find("}}", start)
            block_content = template[start+2:end]
            nodes.append(self._parse_block(block_content))

            current_pos = end + 2

        return nodes

    def _parse_block(self, content: str) -> GrammarNode:
        """Parse a {{...}} block."""
        content = content.strip()

        if content.startswith("gen "):
            return self._parse_gen(content[4:])
        elif content.startswith("select "):
            return self._parse_select(content[7:])
        elif "=" in content:
            return self._parse_assignment(content)
        else:
            return Literal(f"{{{{{content}}}}}")

    def __call__(self, model: Model, **variables) -> Dict[str, Any]:
        """Execute program with model."""
        state = GenerationState(model, variables)

        for node in self.nodes:
            if isinstance(node, Literal):
                state.output += node.value
            elif isinstance(node, Gen):
                result = node.generate(model, state)
                state.output += result
                if node.name:
                    state.variables[node.name] = result
            elif isinstance(node, Select):
                result = node.generate(model, state)
                state.output += result
                if node.name:
                    state.variables[node.name] = result

        return state.variables


# Example usage:
program = GuidanceProgram("""
The answer is {{gen 'answer' regex='[A-D]'}} because {{gen 'explanation' stop='.'}}
""")

result = program(model)
# result = {'answer': 'B', 'explanation': 'the formula shows...'}
```

#### 5. Model Abstraction (`models/_model.py`)

```python
class Model(ABC):
    """Abstract base for model backends."""

    @abstractmethod
    def get_logits(self, context: List[int]) -> np.ndarray:
        """Get logits for next token."""
        pass

    @abstractmethod
    def encode(self, text: str) -> List[int]:
        """Encode text to tokens."""
        pass

    @abstractmethod
    def decode(self, tokens: List[int]) -> str:
        """Decode tokens to text."""
        pass

    @property
    @abstractmethod
    def vocab_size(self) -> int:
        """Get vocabulary size."""
        pass


class OpenAIModel(Model):
    """OpenAI API model backend."""

    def __init__(self, model: str = "gpt-4"):
        self.model = model
        self.client = openai.Client()

    def get_logits(self, context: List[int]) -> np.ndarray:
        """Get logits via API (using logprobs)."""
        # OpenAI doesn't expose raw logits, but we can use logprobs
        text = self.decode(context)
        response = self.client.completions.create(
            model=self.model,
            prompt=text,
            max_tokens=1,
            logprobs=100  # Get top 100 token logprobs
        )
        # Convert logprobs to full logit vector
        return self._logprobs_to_logits(response.choices[0].logprobs)


class TransformersModel(Model):
    """HuggingFace Transformers backend."""

    def __init__(self, model_name: str):
        from transformers import AutoModelForCausalLM, AutoTokenizer
        self.tokenizer = AutoTokenizer.from_pretrained(model_name)
        self.model = AutoModelForCausalLM.from_pretrained(model_name)

    def get_logits(self, context: List[int]) -> np.ndarray:
        input_ids = torch.tensor([context])
        with torch.no_grad():
            outputs = self.model(input_ids)
        return outputs.logits[0, -1].numpy()
```

## Go Client Implementation

Since Guidance requires deep model integration, we implement an HTTP client.

### Client Implementation

```go
// internal/optimization/guidance/client.go

package guidance

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// GuidanceClient communicates with Guidance server
type GuidanceClient struct {
    baseURL    string
    httpClient *http.Client
}

// Config holds client configuration
type Config struct {
    BaseURL string
    Timeout time.Duration
}

// NewGuidanceClient creates a new client
func NewGuidanceClient(config *Config) *GuidanceClient {
    return &GuidanceClient{
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// ProgramRequest is a program execution request
type ProgramRequest struct {
    Template  string         `json:"template"`
    Variables map[string]any `json:"variables,omitempty"`
    Model     string         `json:"model,omitempty"`
}

// ProgramResponse is a program execution response
type ProgramResponse struct {
    Output    string         `json:"output"`
    Variables map[string]any `json:"variables"`
    LatencyMS float64        `json:"latency_ms"`
}

// Execute runs a guidance program
func (c *GuidanceClient) Execute(ctx context.Context, req *ProgramRequest) (*ProgramResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/execute", bytes.NewReader(body))
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

    var result ProgramResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// GenerateRequest is a simple generation request
type GenerateRequest struct {
    Prompt      string  `json:"prompt"`
    Regex       string  `json:"regex,omitempty"`
    Stop        string  `json:"stop,omitempty"`
    MaxTokens   int     `json:"max_tokens,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`
}

// GenerateResponse is a generation response
type GenerateResponse struct {
    Text      string  `json:"text"`
    LatencyMS float64 `json:"latency_ms"`
}

// Generate performs constrained generation
func (c *GuidanceClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/generate", bytes.NewReader(body))
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

    var result GenerateResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// SelectRequest is a selection request
type SelectRequest struct {
    Prompt  string   `json:"prompt"`
    Options []string `json:"options"`
}

// SelectResponse is a selection response
type SelectResponse struct {
    Selected  string  `json:"selected"`
    Score     float64 `json:"score"`
    LatencyMS float64 `json:"latency_ms"`
}

// Select chooses from options
func (c *GuidanceClient) Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/select", bytes.NewReader(body))
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

    var result SelectResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}
```

### Program Builder

```go
// internal/optimization/guidance/programs.go

package guidance

import (
    "fmt"
    "strings"
)

// ProgramBuilder builds guidance programs
type ProgramBuilder struct {
    parts []string
}

// NewProgramBuilder creates a new program builder
func NewProgramBuilder() *ProgramBuilder {
    return &ProgramBuilder{
        parts: []string{},
    }
}

// Text adds literal text
func (b *ProgramBuilder) Text(text string) *ProgramBuilder {
    b.parts = append(b.parts, text)
    return b
}

// Gen adds a generation block
func (b *ProgramBuilder) Gen(name string, opts ...GenOption) *ProgramBuilder {
    options := &genOptions{}
    for _, opt := range opts {
        opt(options)
    }

    block := fmt.Sprintf("{{gen '%s'", name)
    if options.regex != "" {
        block += fmt.Sprintf(" regex='%s'", options.regex)
    }
    if options.stop != "" {
        block += fmt.Sprintf(" stop='%s'", options.stop)
    }
    if options.maxTokens > 0 {
        block += fmt.Sprintf(" max_tokens=%d", options.maxTokens)
    }
    block += "}}"

    b.parts = append(b.parts, block)
    return b
}

// Select adds a selection block
func (b *ProgramBuilder) Select(name string, options []string) *ProgramBuilder {
    optStr := strings.Join(options, "', '")
    block := fmt.Sprintf("{{select '%s' options=['%s']}}", name, optStr)
    b.parts = append(b.parts, block)
    return b
}

// Build returns the program template
func (b *ProgramBuilder) Build() string {
    return strings.Join(b.parts, "")
}

// genOptions holds generation options
type genOptions struct {
    regex     string
    stop      string
    maxTokens int
}

// GenOption configures generation
type GenOption func(*genOptions)

// WithRegex sets regex constraint
func WithRegex(pattern string) GenOption {
    return func(o *genOptions) {
        o.regex = pattern
    }
}

// WithStop sets stop sequence
func WithStop(stop string) GenOption {
    return func(o *genOptions) {
        o.stop = stop
    }
}

// WithMaxTokens sets max tokens
func WithMaxTokens(n int) GenOption {
    return func(o *genOptions) {
        o.maxTokens = n
    }
}

// Common program templates
var (
    // MultipleChoice generates multiple choice answer
    MultipleChoiceTemplate = NewProgramBuilder().
        Text("Question: {{question}}\n").
        Text("Options:\nA) {{option_a}}\nB) {{option_b}}\nC) {{option_c}}\nD) {{option_d}}\n").
        Text("The correct answer is ").
        Gen("answer", WithRegex("[A-D]")).
        Text(" because ").
        Gen("explanation", WithStop(".")).
        Build()

    // JSONOutput generates JSON conforming to schema
    JSONOutputTemplate = NewProgramBuilder().
        Text("Generate a JSON object with the following fields:\n{{schema}}\n\n").
        Gen("json", WithRegex(`\{[^}]+\}`)).
        Build()

    // CodeGeneration generates code
    CodeGenerationTemplate = NewProgramBuilder().
        Text("Write a {{language}} function that {{description}}:\n\n```{{language}}\n").
        Gen("code", WithStop("```")).
        Text("\n```").
        Build()
)
```

### Grammar Builder (Local Validation)

```go
// internal/optimization/guidance/grammars.go

package guidance

import (
    "regexp"
)

// Grammar represents a validation grammar
type Grammar interface {
    Validate(text string) bool
    Match(text string) (string, bool)
}

// LiteralGrammar matches exact text
type LiteralGrammar struct {
    value string
}

func Literal(value string) *LiteralGrammar {
    return &LiteralGrammar{value: value}
}

func (g *LiteralGrammar) Validate(text string) bool {
    return text == g.value
}

func (g *LiteralGrammar) Match(text string) (string, bool) {
    if len(text) >= len(g.value) && text[:len(g.value)] == g.value {
        return g.value, true
    }
    return "", false
}

// RegexGrammar matches regex pattern
type RegexGrammar struct {
    pattern *regexp.Regexp
}

func Regex(pattern string) (*RegexGrammar, error) {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }
    return &RegexGrammar{pattern: re}, nil
}

func (g *RegexGrammar) Validate(text string) bool {
    return g.pattern.MatchString(text)
}

func (g *RegexGrammar) Match(text string) (string, bool) {
    loc := g.pattern.FindStringIndex(text)
    if loc != nil && loc[0] == 0 {
        return text[:loc[1]], true
    }
    return "", false
}

// ChoiceGrammar matches one of several options
type ChoiceGrammar struct {
    options []string
}

func Choice(options ...string) *ChoiceGrammar {
    return &ChoiceGrammar{options: options}
}

func (g *ChoiceGrammar) Validate(text string) bool {
    for _, opt := range g.options {
        if text == opt {
            return true
        }
    }
    return false
}

func (g *ChoiceGrammar) Match(text string) (string, bool) {
    for _, opt := range g.options {
        if len(text) >= len(opt) && text[:len(opt)] == opt {
            return opt, true
        }
    }
    return "", false
}

// SequenceGrammar matches sequence of grammars
type SequenceGrammar struct {
    grammars []Grammar
}

func Sequence(grammars ...Grammar) *SequenceGrammar {
    return &SequenceGrammar{grammars: grammars}
}

func (g *SequenceGrammar) Validate(text string) bool {
    remaining := text
    for _, grammar := range g.grammars {
        matched, ok := grammar.Match(remaining)
        if !ok {
            return false
        }
        remaining = remaining[len(matched):]
    }
    return len(remaining) == 0
}

func (g *SequenceGrammar) Match(text string) (string, bool) {
    var matched strings.Builder
    remaining := text

    for _, grammar := range g.grammars {
        m, ok := grammar.Match(remaining)
        if !ok {
            return "", false
        }
        matched.WriteString(m)
        remaining = remaining[len(m):]
    }

    return matched.String(), true
}
```

## Test Coverage Requirements

```go
// tests/optimization/unit/guidance/client_test.go

func TestGuidanceClient_Execute(t *testing.T)
func TestGuidanceClient_Generate(t *testing.T)
func TestGuidanceClient_Select(t *testing.T)

func TestProgramBuilder_Text(t *testing.T)
func TestProgramBuilder_Gen(t *testing.T)
func TestProgramBuilder_Select(t *testing.T)
func TestProgramBuilder_Build(t *testing.T)

func TestLiteralGrammar_Validate(t *testing.T)
func TestLiteralGrammar_Match(t *testing.T)
func TestRegexGrammar_Validate(t *testing.T)
func TestRegexGrammar_Match(t *testing.T)
func TestChoiceGrammar_Validate(t *testing.T)
func TestSequenceGrammar_Validate(t *testing.T)
```

## Conclusion

Guidance's value lies in its ability to interleave control logic with generation. The HTTP bridge approach preserves this capability while providing a clean Go interface. Local grammar validation enables pre-checking outputs.

**Key Benefits**:
- Fine-grained output control with regex/CFG
- Interleaved control flow
- Variable capture for downstream use

**Estimated Implementation Time**: 1 week
**Risk Level**: Medium-High (complex grammar system)
**Dependencies**: Python service with model access
