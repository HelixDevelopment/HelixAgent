# LMQL - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/eth-sri/lmql
- **Language**: Python
- **Purpose**: Query language for LLMs with constraint-guided decoding and speculative execution
- **License**: Apache 2.0

## Core Architecture

### Directory Structure

```
lmql/
├── lmql/
│   ├── language/            # LMQL language parser
│   │   ├── parser.py        # Query parser
│   │   ├── compiler.py      # Query compiler
│   │   └── ast.py           # Abstract syntax tree
│   ├── runtime/             # Execution runtime
│   │   ├── interpreter.py   # Query interpreter
│   │   ├── dclib/           # Decoding library
│   │   │   ├── dclib.py     # Core decoding
│   │   │   └── masks.py     # Token masking
│   │   └── model_registry/  # Model backends
│   ├── ops/                 # Operations
│   │   ├── token_set.py     # Token set operations
│   │   └── follow_map.py    # Follow map computation
│   └── lib/                 # Standard library
│       └── types.py         # Type constraints
```

### Key Components

#### 1. Query Language Parser (`language/parser.py`)

**LMQL Query Syntax**

```python
# Example LMQL query
"""
argmax
    "Q: What is 2+2?[ANSWER]"
from
    "gpt-3.5-turbo"
where
    len(TOKENS(ANSWER)) < 10 and
    ANSWER in ["4", "four", "Four"]
"""

class LMQLParser:
    """Parse LMQL query into AST."""

    def parse(self, query: str) -> QueryAST:
        tokens = self.tokenize(query)
        return self.parse_query(tokens)

    def parse_query(self, tokens: List[Token]) -> QueryAST:
        """Parse full query structure."""
        # Parse decoding strategy (argmax, sample, beam)
        strategy = self.parse_strategy(tokens)

        # Parse prompt template
        prompt = self.parse_prompt(tokens)

        # Parse model specification
        model = self.parse_from_clause(tokens)

        # Parse constraints
        constraints = self.parse_where_clause(tokens)

        # Parse distribution (optional)
        distribution = self.parse_distribution(tokens)

        return QueryAST(
            strategy=strategy,
            prompt=prompt,
            model=model,
            constraints=constraints,
            distribution=distribution
        )


class QueryAST:
    """Abstract syntax tree for LMQL query."""

    def __init__(self, strategy, prompt, model, constraints, distribution=None):
        self.strategy = strategy  # DecodingStrategy
        self.prompt = prompt      # PromptTemplate
        self.model = model        # ModelSpec
        self.constraints = constraints  # List[Constraint]
        self.distribution = distribution  # Optional[Distribution]


class PromptTemplate:
    """Parsed prompt template with holes."""

    def __init__(self, parts: List[Union[str, Variable]]):
        self.parts = parts
        self.variables = [p for p in parts if isinstance(p, Variable)]

    def format(self, values: Dict[str, str]) -> str:
        result = []
        for part in self.parts:
            if isinstance(part, str):
                result.append(part)
            else:
                result.append(values.get(part.name, f"[{part.name}]"))
        return "".join(result)


class Variable:
    """Variable in prompt template."""

    def __init__(self, name: str, constraints: List = None):
        self.name = name
        self.constraints = constraints or []
```

#### 2. Constraint System (`ops/token_set.py`)

```python
from abc import ABC, abstractmethod
from typing import Set, List, Optional

class Constraint(ABC):
    """Base class for LMQL constraints."""

    @abstractmethod
    def get_mask(self, context: GenerationContext) -> TokenMask:
        """Get token mask for current context."""
        pass

    @abstractmethod
    def check(self, value: str) -> bool:
        """Check if final value satisfies constraint."""
        pass


class LengthConstraint(Constraint):
    """Constraint on output length."""

    def __init__(self, variable: str, op: str, value: int):
        self.variable = variable
        self.op = op  # '<', '<=', '>', '>=', '=='
        self.value = value

    def get_mask(self, context: GenerationContext) -> TokenMask:
        current_len = len(context.get_tokens(self.variable))

        if self.op == '<' and current_len >= self.value - 1:
            # Only allow EOS
            return TokenMask.only_eos()
        elif self.op == '<=' and current_len >= self.value:
            return TokenMask.only_eos()
        else:
            return TokenMask.all_allowed()

    def check(self, value: str) -> bool:
        length = len(value.split())
        if self.op == '<':
            return length < self.value
        elif self.op == '<=':
            return length <= self.value
        # ... other operators


class ChoiceConstraint(Constraint):
    """Constraint: value must be one of choices."""

    def __init__(self, variable: str, choices: List[str]):
        self.variable = variable
        self.choices = choices
        self._precompute_prefixes()

    def _precompute_prefixes(self):
        """Precompute valid prefixes for efficiency."""
        self.valid_prefixes = set()
        for choice in self.choices:
            for i in range(1, len(choice) + 1):
                self.valid_prefixes.add(choice[:i])

    def get_mask(self, context: GenerationContext) -> TokenMask:
        current = context.get_value(self.variable)
        allowed_tokens = []

        for token_id, token_str in context.tokenizer.vocab.items():
            candidate = current + token_str

            # Check if candidate could lead to valid choice
            could_match = False
            for choice in self.choices:
                if choice.startswith(candidate) or candidate == choice:
                    could_match = True
                    break

            if could_match:
                allowed_tokens.append(token_id)

        return TokenMask(allowed_tokens)

    def check(self, value: str) -> bool:
        return value.strip() in self.choices


class RegexConstraint(Constraint):
    """Constraint: value must match regex."""

    def __init__(self, variable: str, pattern: str):
        self.variable = variable
        self.pattern = re.compile(pattern)
        self.dfa = self._build_dfa(pattern)

    def _build_dfa(self, pattern: str) -> DFA:
        """Build DFA from regex for efficient matching."""
        import interegular
        return interegular.parse_pattern(pattern).to_fsm()

    def get_mask(self, context: GenerationContext) -> TokenMask:
        current = context.get_value(self.variable)
        current_state = self.dfa.get_state(current)

        allowed_tokens = []
        for token_id, token_str in context.tokenizer.vocab.items():
            next_state = self.dfa.transition(current_state, token_str)
            if next_state is not None:  # Valid transition exists
                allowed_tokens.append(token_id)

        return TokenMask(allowed_tokens)


class TokenMask:
    """Mask indicating allowed tokens."""

    def __init__(self, allowed_tokens: List[int] = None):
        self.allowed = set(allowed_tokens) if allowed_tokens else None

    @classmethod
    def all_allowed(cls) -> "TokenMask":
        return cls(None)

    @classmethod
    def only_eos(cls) -> "TokenMask":
        return cls([EOS_TOKEN_ID])

    def intersect(self, other: "TokenMask") -> "TokenMask":
        if self.allowed is None:
            return other
        if other.allowed is None:
            return self
        return TokenMask(list(self.allowed & other.allowed))

    def apply_to_logits(self, logits: np.ndarray) -> np.ndarray:
        if self.allowed is None:
            return logits
        mask = np.full(len(logits), float('-inf'))
        for token_id in self.allowed:
            mask[token_id] = 0
        return logits + mask
```

#### 3. Decoding Library (`runtime/dclib/dclib.py`)

```python
class ConstrainedDecoder:
    """Decoder with constraint satisfaction."""

    def __init__(self, model: Model, constraints: List[Constraint]):
        self.model = model
        self.constraints = constraints

    def decode(self, prompt: str, variables: List[str]) -> Dict[str, str]:
        """Decode with constraints."""
        context = GenerationContext(self.model, prompt)
        results = {}

        for var in variables:
            context.current_variable = var
            value = self._decode_variable(context, var)
            results[var] = value
            context.set_value(var, value)

        return results

    def _decode_variable(self, context: GenerationContext, var: str) -> str:
        """Decode single variable with constraints."""
        generated_tokens = []

        while True:
            # Get combined mask from all constraints
            mask = TokenMask.all_allowed()
            for constraint in self.constraints:
                if constraint.variable == var:
                    var_mask = constraint.get_mask(context)
                    mask = mask.intersect(var_mask)

            # Get logits and apply mask
            logits = self.model.get_logits(context.get_context())
            masked_logits = mask.apply_to_logits(logits)

            # Sample token
            token_id = self._sample(masked_logits)

            if token_id == EOS_TOKEN_ID:
                break

            generated_tokens.append(token_id)
            context.append_token(token_id)

        return self.model.decode(generated_tokens)

    def _sample(self, logits: np.ndarray) -> int:
        """Sample from logits based on strategy."""
        if self.strategy == "argmax":
            return np.argmax(logits)
        elif self.strategy == "sample":
            probs = softmax(logits)
            return np.random.choice(len(probs), p=probs)
```

#### 4. Speculative Execution (`runtime/interpreter.py`)

```python
class SpeculativeExecutor:
    """Execute queries with speculative decoding."""

    def __init__(self, model: Model, speculation_depth: int = 5):
        self.model = model
        self.speculation_depth = speculation_depth
        self.cache = QueryCache()

    def execute(self, query: QueryAST) -> Dict[str, str]:
        """Execute query with speculation."""
        # Check cache for similar queries
        cached = self.cache.get(query)
        if cached:
            return cached

        # Build execution tree
        tree = self._build_execution_tree(query)

        # Execute with speculation
        results = self._speculative_execute(tree)

        # Cache results
        self.cache.put(query, results)

        return results

    def _build_execution_tree(self, query: QueryAST) -> ExecutionTree:
        """Build tree of possible execution paths."""
        root = ExecutionNode(query.prompt)

        for var in query.prompt.variables:
            # For constrained choices, enumerate options
            choice_constraint = self._find_choice_constraint(query, var.name)
            if choice_constraint:
                for choice in choice_constraint.choices:
                    child = ExecutionNode(choice, parent=root)
                    root.children.append(child)
            else:
                # General generation
                child = ExecutionNode(None, parent=root)
                root.children.append(child)

        return ExecutionTree(root)

    def _speculative_execute(self, tree: ExecutionTree) -> Dict[str, str]:
        """Execute tree speculatively."""
        # Generate multiple paths in parallel
        paths = tree.get_top_paths(self.speculation_depth)

        # Score each path
        path_scores = []
        for path in paths:
            score = self._score_path(path)
            path_scores.append((path, score))

        # Return best path
        best_path = max(path_scores, key=lambda x: x[1])[0]
        return best_path.to_dict()


class QueryCache:
    """Cache for query results with semantic similarity."""

    def __init__(self, max_size: int = 1000):
        self.cache = {}
        self.embeddings = {}
        self.max_size = max_size

    def get(self, query: QueryAST) -> Optional[Dict[str, str]]:
        """Get cached result for similar query."""
        query_key = self._compute_key(query)

        # Exact match
        if query_key in self.cache:
            return self.cache[query_key]

        # Semantic similarity search
        query_embedding = self._embed_query(query)
        for key, embedding in self.embeddings.items():
            similarity = cosine_similarity(query_embedding, embedding)
            if similarity > 0.95:  # High similarity threshold
                return self.cache[key]

        return None

    def put(self, query: QueryAST, result: Dict[str, str]):
        """Cache query result."""
        query_key = self._compute_key(query)
        self.cache[query_key] = result
        self.embeddings[query_key] = self._embed_query(query)

        # Evict if needed
        if len(self.cache) > self.max_size:
            oldest_key = next(iter(self.cache))
            del self.cache[oldest_key]
            del self.embeddings[oldest_key]
```

## Go Client Implementation

Since LMQL has complex parsing and constraint systems, we implement an HTTP client with local query building.

### Client Implementation

```go
// internal/optimization/lmql/client.go

package lmql

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// LMQLClient communicates with LMQL server
type LMQLClient struct {
    baseURL    string
    httpClient *http.Client
}

// Config holds client configuration
type Config struct {
    BaseURL string
    Timeout time.Duration
}

// NewLMQLClient creates a new client
func NewLMQLClient(config *Config) *LMQLClient {
    return &LMQLClient{
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// QueryRequest is a query execution request
type QueryRequest struct {
    Query     string            `json:"query"`
    Variables map[string]string `json:"variables,omitempty"`
    Model     string            `json:"model,omitempty"`
}

// QueryResponse is a query execution response
type QueryResponse struct {
    Results   map[string]string `json:"results"`
    LatencyMS float64           `json:"latency_ms"`
    CacheHit  bool              `json:"cache_hit"`
}

// Execute runs an LMQL query
func (c *LMQLClient) Execute(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
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

    var result QueryResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// ValidateQuery validates a query without executing
func (c *LMQLClient) ValidateQuery(ctx context.Context, query string) (*ValidationResult, error) {
    body, err := json.Marshal(map[string]string{"query": query})
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/validate", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    var result ValidationResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// ValidationResult contains query validation result
type ValidationResult struct {
    Valid     bool     `json:"valid"`
    Errors    []string `json:"errors,omitempty"`
    Variables []string `json:"variables,omitempty"`
}
```

### Query Builder

```go
// internal/optimization/lmql/queries.go

package lmql

import (
    "fmt"
    "strings"
)

// QueryBuilder builds LMQL queries programmatically
type QueryBuilder struct {
    strategy     string
    prompt       string
    model        string
    constraints  []string
    distribution string
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
    return &QueryBuilder{
        strategy:    "argmax",
        constraints: []string{},
    }
}

// Strategy sets the decoding strategy
func (b *QueryBuilder) Strategy(strategy string) *QueryBuilder {
    b.strategy = strategy
    return b
}

// Argmax sets argmax decoding
func (b *QueryBuilder) Argmax() *QueryBuilder {
    b.strategy = "argmax"
    return b
}

// Sample sets sampling decoding
func (b *QueryBuilder) Sample(temperature float64) *QueryBuilder {
    b.strategy = fmt.Sprintf("sample(temperature=%.2f)", temperature)
    return b
}

// Beam sets beam search decoding
func (b *QueryBuilder) Beam(n int) *QueryBuilder {
    b.strategy = fmt.Sprintf("beam(%d)", n)
    return b
}

// Prompt sets the prompt template
func (b *QueryBuilder) Prompt(prompt string) *QueryBuilder {
    b.prompt = prompt
    return b
}

// From sets the model
func (b *QueryBuilder) From(model string) *QueryBuilder {
    b.model = model
    return b
}

// Where adds a constraint
func (b *QueryBuilder) Where(constraint string) *QueryBuilder {
    b.constraints = append(b.constraints, constraint)
    return b
}

// LenLT adds length less-than constraint
func (b *QueryBuilder) LenLT(variable string, max int) *QueryBuilder {
    b.constraints = append(b.constraints, fmt.Sprintf("len(TOKENS(%s)) < %d", variable, max))
    return b
}

// LenLE adds length less-than-or-equal constraint
func (b *QueryBuilder) LenLE(variable string, max int) *QueryBuilder {
    b.constraints = append(b.constraints, fmt.Sprintf("len(TOKENS(%s)) <= %d", variable, max))
    return b
}

// InChoices adds choice constraint
func (b *QueryBuilder) InChoices(variable string, choices []string) *QueryBuilder {
    quoted := make([]string, len(choices))
    for i, c := range choices {
        quoted[i] = fmt.Sprintf(`"%s"`, c)
    }
    b.constraints = append(b.constraints, fmt.Sprintf("%s in [%s]", variable, strings.Join(quoted, ", ")))
    return b
}

// Regex adds regex constraint
func (b *QueryBuilder) Regex(variable string, pattern string) *QueryBuilder {
    b.constraints = append(b.constraints, fmt.Sprintf(`REGEX(%s, r"%s")`, variable, pattern))
    return b
}

// StopAt adds stop sequence constraint
func (b *QueryBuilder) StopAt(variable string, stop string) *QueryBuilder {
    b.constraints = append(b.constraints, fmt.Sprintf(`STOPS_AT(%s, "%s")`, variable, stop))
    return b
}

// Distribution sets return distribution
func (b *QueryBuilder) Distribution(variable string) *QueryBuilder {
    b.distribution = variable
    return b
}

// Build returns the LMQL query string
func (b *QueryBuilder) Build() string {
    var parts []string

    // Strategy
    parts = append(parts, b.strategy)

    // Prompt
    parts = append(parts, fmt.Sprintf(`    "%s"`, b.prompt))

    // From clause
    if b.model != "" {
        parts = append(parts, fmt.Sprintf("from\n    \"%s\"", b.model))
    }

    // Where clause
    if len(b.constraints) > 0 {
        whereClause := "where\n    " + strings.Join(b.constraints, " and\n    ")
        parts = append(parts, whereClause)
    }

    // Distribution
    if b.distribution != "" {
        parts = append(parts, fmt.Sprintf("distribution\n    %s", b.distribution))
    }

    return strings.Join(parts, "\n")
}

// Common query templates
func MultipleChoiceQuery(question string, choices []string) string {
    return NewQueryBuilder().
        Argmax().
        Prompt(fmt.Sprintf("Q: %s\\n[ANSWER]", question)).
        From("gpt-3.5-turbo").
        InChoices("ANSWER", choices).
        Build()
}

func ShortAnswerQuery(question string, maxTokens int) string {
    return NewQueryBuilder().
        Argmax().
        Prompt(fmt.Sprintf("Q: %s\\n[ANSWER]", question)).
        From("gpt-3.5-turbo").
        LenLT("ANSWER", maxTokens).
        Build()
}

func CodeGenerationQuery(description string, language string) string {
    return NewQueryBuilder().
        Argmax().
        Prompt(fmt.Sprintf("Write a %s function that %s:\\n```%s\\n[CODE]\\n```", language, description, language)).
        From("gpt-4").
        StopAt("CODE", "```").
        Build()
}
```

### Constraint Validation (Local)

```go
// internal/optimization/lmql/constraints.go

package lmql

import (
    "regexp"
    "strings"
)

// Constraint defines a local constraint validator
type Constraint interface {
    Validate(value string) bool
    Name() string
}

// LengthConstraint validates length
type LengthConstraint struct {
    Variable string
    Op       string // "<", "<=", ">", ">="
    Value    int
}

func NewLengthConstraint(variable, op string, value int) *LengthConstraint {
    return &LengthConstraint{
        Variable: variable,
        Op:       op,
        Value:    value,
    }
}

func (c *LengthConstraint) Name() string {
    return fmt.Sprintf("len(%s) %s %d", c.Variable, c.Op, c.Value)
}

func (c *LengthConstraint) Validate(value string) bool {
    tokens := strings.Fields(value)
    length := len(tokens)

    switch c.Op {
    case "<":
        return length < c.Value
    case "<=":
        return length <= c.Value
    case ">":
        return length > c.Value
    case ">=":
        return length >= c.Value
    case "==":
        return length == c.Value
    default:
        return false
    }
}

// ChoiceConstraint validates value is in choices
type ChoiceConstraint struct {
    Variable string
    Choices  []string
}

func NewChoiceConstraint(variable string, choices []string) *ChoiceConstraint {
    return &ChoiceConstraint{
        Variable: variable,
        Choices:  choices,
    }
}

func (c *ChoiceConstraint) Name() string {
    return fmt.Sprintf("%s in choices", c.Variable)
}

func (c *ChoiceConstraint) Validate(value string) bool {
    trimmed := strings.TrimSpace(value)
    for _, choice := range c.Choices {
        if trimmed == choice {
            return true
        }
    }
    return false
}

// RegexConstraint validates value matches regex
type RegexConstraint struct {
    Variable string
    Pattern  *regexp.Regexp
}

func NewRegexConstraint(variable, pattern string) (*RegexConstraint, error) {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }
    return &RegexConstraint{
        Variable: variable,
        Pattern:  re,
    }, nil
}

func (c *RegexConstraint) Name() string {
    return fmt.Sprintf("regex(%s)", c.Variable)
}

func (c *RegexConstraint) Validate(value string) bool {
    return c.Pattern.MatchString(value)
}

// ConstraintSet validates multiple constraints
type ConstraintSet struct {
    constraints []Constraint
}

func NewConstraintSet() *ConstraintSet {
    return &ConstraintSet{
        constraints: []Constraint{},
    }
}

func (cs *ConstraintSet) Add(c Constraint) {
    cs.constraints = append(cs.constraints, c)
}

func (cs *ConstraintSet) Validate(values map[string]string) *ValidationResult {
    result := &ValidationResult{Valid: true}

    for _, c := range cs.constraints {
        // Find variable name
        varName := ""
        switch constraint := c.(type) {
        case *LengthConstraint:
            varName = constraint.Variable
        case *ChoiceConstraint:
            varName = constraint.Variable
        case *RegexConstraint:
            varName = constraint.Variable
        }

        if value, ok := values[varName]; ok {
            if !c.Validate(value) {
                result.Valid = false
                result.Errors = append(result.Errors,
                    fmt.Sprintf("constraint '%s' failed for value '%s'", c.Name(), value))
            }
        }
    }

    return result
}
```

## Test Coverage Requirements

```go
// tests/optimization/unit/lmql/client_test.go

func TestLMQLClient_Execute(t *testing.T)
func TestLMQLClient_ValidateQuery(t *testing.T)

func TestQueryBuilder_Argmax(t *testing.T)
func TestQueryBuilder_Sample(t *testing.T)
func TestQueryBuilder_Beam(t *testing.T)
func TestQueryBuilder_Prompt(t *testing.T)
func TestQueryBuilder_Where(t *testing.T)
func TestQueryBuilder_LenLT(t *testing.T)
func TestQueryBuilder_InChoices(t *testing.T)
func TestQueryBuilder_Regex(t *testing.T)
func TestQueryBuilder_Build(t *testing.T)

func TestMultipleChoiceQuery(t *testing.T)
func TestShortAnswerQuery(t *testing.T)
func TestCodeGenerationQuery(t *testing.T)

func TestLengthConstraint_Validate(t *testing.T)
func TestChoiceConstraint_Validate(t *testing.T)
func TestRegexConstraint_Validate(t *testing.T)
func TestConstraintSet_Validate(t *testing.T)
```

## Conclusion

LMQL provides a powerful query language for constrained LLM generation. The Go client provides query building and local constraint validation, while the Python service handles the complex parsing and constraint-guided decoding.

**Key Benefits**:
- Declarative query language for complex prompts
- Constraint-guided decoding for guaranteed outputs
- Speculative execution for speed

**Estimated Implementation Time**: 1 week
**Risk Level**: High (complex parsing system)
**Dependencies**: Python service with model access
