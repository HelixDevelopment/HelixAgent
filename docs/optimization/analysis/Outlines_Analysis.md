# Outlines - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/dottxt-ai/outlines
- **Language**: Python
- **Purpose**: Library for guaranteed structured outputs (JSON, regex patterns)
- **License**: Apache 2.0

## Core Architecture

### Directory Structure

```
outlines/
├── generate/          # Generation orchestration
│   ├── api.py         # High-level API
│   ├── generator.py   # Core generation logic
│   └── cfg.py         # Context-free grammar support
├── fsm/               # Finite state machine for constraints
│   ├── json_schema.py # JSON schema to FSM conversion
│   ├── regex.py       # Regex to FSM conversion
│   └── guide.py       # FSM guidance during generation
├── models/            # Model integrations
│   ├── transformers.py
│   ├── openai.py
│   └── vllm.py
├── types/             # Type definitions
│   └── json_schema.py # JSON schema types
└── processors/        # Token processors
    └── structured.py  # Structured output processing
```

### Key Components

#### 1. JSON Schema Engine (`outlines/fsm/json_schema.py`)

**Schema to FSM Conversion**

```python
# Core algorithm: Convert JSON schema to finite state machine
class JSONSchemaFSM:
    def __init__(self, schema: dict, tokenizer):
        self.schema = schema
        self.tokenizer = tokenizer
        self.states = self._build_states()
        self.transitions = self._build_transitions()

    def _build_states(self) -> List[State]:
        """Build FSM states from schema."""
        states = [State.START]

        if self.schema.get("type") == "object":
            properties = self.schema.get("properties", {})
            required = self.schema.get("required", [])

            # State for opening brace
            states.append(State.OBJECT_OPEN)

            # States for each property
            for prop_name, prop_schema in properties.items():
                states.append(State(f"key_{prop_name}"))
                states.append(State(f"colon_{prop_name}"))
                states.append(State(f"value_{prop_name}"))

            states.append(State.OBJECT_CLOSE)

        return states

    def _build_transitions(self) -> Dict[State, Dict[str, State]]:
        """Build state transitions."""
        transitions = {}
        # Complex transition logic based on schema structure
        return transitions
```

**Allowed Token Computation**

```python
def get_allowed_tokens(self, state: State) -> List[int]:
    """Get token IDs allowed in current state."""
    allowed = []

    if state == State.OBJECT_OPEN:
        # Only allow '{'
        allowed = self._get_tokens_for_string("{")
    elif state.name.startswith("key_"):
        # Allow property name tokens
        prop_name = state.name[4:]
        allowed = self._get_tokens_for_string(f'"{prop_name}"')
    elif state.name.startswith("value_"):
        # Allow tokens valid for property type
        prop_name = state.name[6:]
        prop_type = self.schema["properties"][prop_name]["type"]
        allowed = self._get_allowed_for_type(prop_type)

    return allowed

def _get_allowed_for_type(self, type_name: str) -> List[int]:
    """Get allowed tokens for a JSON type."""
    if type_name == "string":
        return self._get_string_tokens()
    elif type_name == "number":
        return self._get_number_tokens()
    elif type_name == "boolean":
        return self._get_tokens_for_string("true") + \
               self._get_tokens_for_string("false")
    # ... more types
```

#### 2. Regex Engine (`outlines/fsm/regex.py`)

**Regex to DFA Conversion**

```python
import interegular

class RegexFSM:
    def __init__(self, pattern: str, tokenizer):
        self.pattern = pattern
        self.tokenizer = tokenizer
        # Use interegular for regex -> FSM conversion
        self.fsm = interegular.parse_pattern(pattern).to_fsm()
        self._precompute_token_transitions()

    def _precompute_token_transitions(self):
        """Precompute which tokens are valid from each state."""
        self.state_to_tokens = {}

        for state in self.fsm.states:
            valid_tokens = []
            for token_id, token_str in enumerate(self.tokenizer.vocab):
                if self._token_valid_from_state(state, token_str):
                    valid_tokens.append(token_id)
            self.state_to_tokens[state] = valid_tokens

    def _token_valid_from_state(self, state, token_str: str) -> bool:
        """Check if token is valid from current FSM state."""
        current = state
        for char in token_str:
            next_state = self.fsm.transition(current, char)
            if next_state is None:
                return False
            current = next_state
        return True
```

#### 3. Token Masking (`outlines/processors/structured.py`)

**Core Masking Logic**

```python
class StructuredProcessor:
    def __init__(self, fsm: FSM, tokenizer):
        self.fsm = fsm
        self.tokenizer = tokenizer
        self.current_state = fsm.initial_state

    def __call__(self, input_ids: List[int], scores: torch.Tensor) -> torch.Tensor:
        """Apply token masking based on FSM state."""
        allowed_tokens = self.fsm.get_allowed_tokens(self.current_state)

        # Create mask
        mask = torch.full_like(scores, float('-inf'))
        mask[allowed_tokens] = 0

        # Apply mask to scores
        masked_scores = scores + mask

        return masked_scores

    def update_state(self, token_id: int):
        """Update FSM state after token selection."""
        token_str = self.tokenizer.decode([token_id])
        self.current_state = self.fsm.transition(self.current_state, token_str)
```

#### 4. Generation Orchestration (`outlines/generate/generator.py`)

**Structured Generation Loop**

```python
class StructuredGenerator:
    def __init__(self, model, processor: StructuredProcessor):
        self.model = model
        self.processor = processor

    def generate(self, prompt: str, max_tokens: int = 1000) -> str:
        """Generate structured output token by token."""
        input_ids = self.model.tokenize(prompt)
        generated = []

        for _ in range(max_tokens):
            # Get logits from model
            logits = self.model.forward(input_ids)

            # Apply structured constraints
            masked_logits = self.processor(input_ids, logits[-1])

            # Sample token
            token_id = self._sample(masked_logits)

            # Check for end condition
            if self.processor.fsm.is_final_state():
                break

            # Update state
            self.processor.update_state(token_id)
            generated.append(token_id)
            input_ids = input_ids + [token_id]

        return self.model.detokenize(generated)
```

### Validation Layer

```python
class SchemaValidator:
    def __init__(self, schema: dict):
        self.schema = schema
        self.validator = jsonschema.Draft7Validator(schema)

    def validate(self, output: str) -> ValidationResult:
        """Validate output against schema."""
        try:
            data = json.loads(output)
            errors = list(self.validator.iter_errors(data))
            if errors:
                return ValidationResult(
                    valid=False,
                    errors=[str(e) for e in errors]
                )
            return ValidationResult(valid=True, data=data)
        except json.JSONDecodeError as e:
            return ValidationResult(valid=False, errors=[str(e)])
```

## Go Port Strategy

### Core Components to Implement

```go
// internal/optimization/outlines/schema_engine.go

package outlines

import (
    "encoding/json"
    "fmt"
)

// JSONSchema represents a JSON schema
type JSONSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]*JSONSchema `json:"properties,omitempty"`
    Required   []string               `json:"required,omitempty"`
    Items      *JSONSchema            `json:"items,omitempty"`
    Enum       []any                  `json:"enum,omitempty"`
    MinLength  *int                   `json:"minLength,omitempty"`
    MaxLength  *int                   `json:"maxLength,omitempty"`
    Minimum    *float64               `json:"minimum,omitempty"`
    Maximum    *float64               `json:"maximum,omitempty"`
    Pattern    string                 `json:"pattern,omitempty"`
}

// State represents an FSM state
type State struct {
    Name       string
    IsFinal    bool
    AllowedTokens []int
}

// SchemaFSM is the finite state machine for JSON schema constraints
type SchemaFSM struct {
    schema       *JSONSchema
    states       map[string]*State
    transitions  map[string]map[string]string // state -> token -> next_state
    currentState string
    tokenizer    Tokenizer
}

// NewSchemaFSM creates a new FSM from a JSON schema
func NewSchemaFSM(schema *JSONSchema, tokenizer Tokenizer) (*SchemaFSM, error) {
    fsm := &SchemaFSM{
        schema:      schema,
        states:      make(map[string]*State),
        transitions: make(map[string]map[string]string),
        tokenizer:   tokenizer,
    }

    if err := fsm.buildStates(); err != nil {
        return nil, err
    }

    if err := fsm.buildTransitions(); err != nil {
        return nil, err
    }

    fsm.currentState = "start"
    return fsm, nil
}

func (f *SchemaFSM) buildStates() error {
    // Start state
    f.states["start"] = &State{Name: "start", IsFinal: false}

    switch f.schema.Type {
    case "object":
        return f.buildObjectStates()
    case "array":
        return f.buildArrayStates()
    case "string":
        return f.buildStringStates()
    case "number", "integer":
        return f.buildNumberStates()
    case "boolean":
        return f.buildBooleanStates()
    default:
        return fmt.Errorf("unsupported type: %s", f.schema.Type)
    }
}

func (f *SchemaFSM) buildObjectStates() error {
    // Opening brace
    f.states["object_open"] = &State{
        Name:          "object_open",
        AllowedTokens: f.tokensFor("{"),
    }

    // States for each property
    for propName, propSchema := range f.schema.Properties {
        f.states[fmt.Sprintf("key_%s", propName)] = &State{
            Name:          fmt.Sprintf("key_%s", propName),
            AllowedTokens: f.tokensFor(fmt.Sprintf(`"%s"`, propName)),
        }
        f.states[fmt.Sprintf("colon_%s", propName)] = &State{
            Name:          fmt.Sprintf("colon_%s", propName),
            AllowedTokens: f.tokensFor(":"),
        }
        f.states[fmt.Sprintf("value_%s", propName)] = &State{
            Name:          fmt.Sprintf("value_%s", propName),
            AllowedTokens: f.tokensForType(propSchema.Type),
        }
    }

    // Closing brace
    f.states["object_close"] = &State{
        Name:          "object_close",
        AllowedTokens: f.tokensFor("}"),
        IsFinal:       true,
    }

    return nil
}

// GetAllowedTokens returns tokens allowed in current state
func (f *SchemaFSM) GetAllowedTokens() []int {
    state, ok := f.states[f.currentState]
    if !ok {
        return nil
    }
    return state.AllowedTokens
}

// Transition moves to next state given a token
func (f *SchemaFSM) Transition(tokenID int) error {
    token := f.tokenizer.Decode([]int{tokenID})

    trans, ok := f.transitions[f.currentState]
    if !ok {
        return fmt.Errorf("no transitions from state: %s", f.currentState)
    }

    nextState, ok := trans[token]
    if !ok {
        return fmt.Errorf("invalid token %q in state %s", token, f.currentState)
    }

    f.currentState = nextState
    return nil
}

// IsFinal returns true if current state is final
func (f *SchemaFSM) IsFinal() bool {
    state, ok := f.states[f.currentState]
    return ok && state.IsFinal
}
```

### Token Masking Implementation

```go
// internal/optimization/outlines/token_mask.go

package outlines

import (
    "math"
)

// TokenMask represents a mask for valid tokens
type TokenMask struct {
    mask      []bool
    vocabSize int
}

// NewTokenMask creates a new token mask
func NewTokenMask(vocabSize int) *TokenMask {
    return &TokenMask{
        mask:      make([]bool, vocabSize),
        vocabSize: vocabSize,
    }
}

// Allow marks tokens as allowed
func (m *TokenMask) Allow(tokenIDs []int) {
    for _, id := range tokenIDs {
        if id >= 0 && id < m.vocabSize {
            m.mask[id] = true
        }
    }
}

// ApplyToLogits applies the mask to logits
func (m *TokenMask) ApplyToLogits(logits []float64) []float64 {
    result := make([]float64, len(logits))
    for i, logit := range logits {
        if m.mask[i] {
            result[i] = logit
        } else {
            result[i] = math.Inf(-1) // -infinity
        }
    }
    return result
}

// MaskIndices returns indices of masked (disallowed) tokens
func (m *TokenMask) MaskIndices() []int {
    var indices []int
    for i, allowed := range m.mask {
        if !allowed {
            indices = append(indices, i)
        }
    }
    return indices
}
```

### Regex Engine

```go
// internal/optimization/outlines/regex_engine.go

package outlines

import (
    "regexp"
)

// RegexFSM implements regex-based token constraints
type RegexFSM struct {
    pattern       *regexp.Regexp
    patternStr    string
    tokenizer     Tokenizer
    currentOutput string
    stateCache    map[string][]int // partial output -> allowed tokens
}

// NewRegexFSM creates a new regex FSM
func NewRegexFSM(pattern string, tokenizer Tokenizer) (*RegexFSM, error) {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }

    return &RegexFSM{
        pattern:       re,
        patternStr:    pattern,
        tokenizer:     tokenizer,
        currentOutput: "",
        stateCache:    make(map[string][]int),
    }, nil
}

// GetAllowedTokens returns tokens that could lead to a valid match
func (f *RegexFSM) GetAllowedTokens() []int {
    // Check cache first
    if cached, ok := f.stateCache[f.currentOutput]; ok {
        return cached
    }

    var allowed []int
    vocab := f.tokenizer.Vocabulary()

    for tokenID, tokenStr := range vocab {
        candidate := f.currentOutput + tokenStr
        if f.couldMatch(candidate) {
            allowed = append(allowed, tokenID)
        }
    }

    f.stateCache[f.currentOutput] = allowed
    return allowed
}

// couldMatch checks if string could potentially match the pattern
func (f *RegexFSM) couldMatch(s string) bool {
    // Check if s is a valid prefix of any string matching the pattern
    // This is a simplified check - full implementation would use DFA
    if f.pattern.MatchString(s) {
        return true
    }

    // Check if pattern could still be completed
    // For now, use a heuristic approach
    prefixPattern := "^" + regexp.QuoteMeta(s)
    prefixRe, err := regexp.Compile(prefixPattern)
    if err != nil {
        return false
    }

    // Pattern must not contradict what we have so far
    return prefixRe.MatchString(s) || len(s) == 0
}

// Transition updates state with generated token
func (f *RegexFSM) Transition(tokenID int) {
    token := f.tokenizer.Decode([]int{tokenID})
    f.currentOutput += token
}

// IsFinal checks if current output matches the pattern
func (f *RegexFSM) IsFinal() bool {
    return f.pattern.MatchString(f.currentOutput)
}
```

### Validation

```go
// internal/optimization/outlines/validators.go

package outlines

import (
    "encoding/json"
    "fmt"
)

// ValidationResult contains validation outcome
type ValidationResult struct {
    Valid  bool
    Errors []string
    Data   any
}

// SchemaValidator validates JSON against a schema
type SchemaValidator struct {
    schema *JSONSchema
}

// NewSchemaValidator creates a new validator
func NewSchemaValidator(schema *JSONSchema) *SchemaValidator {
    return &SchemaValidator{schema: schema}
}

// Validate validates a JSON string against the schema
func (v *SchemaValidator) Validate(output string) *ValidationResult {
    var data any
    if err := json.Unmarshal([]byte(output), &data); err != nil {
        return &ValidationResult{
            Valid:  false,
            Errors: []string{fmt.Sprintf("JSON parse error: %v", err)},
        }
    }

    errors := v.validateValue(data, v.schema, "")
    return &ValidationResult{
        Valid:  len(errors) == 0,
        Errors: errors,
        Data:   data,
    }
}

func (v *SchemaValidator) validateValue(data any, schema *JSONSchema, path string) []string {
    var errors []string

    switch schema.Type {
    case "object":
        errors = append(errors, v.validateObject(data, schema, path)...)
    case "array":
        errors = append(errors, v.validateArray(data, schema, path)...)
    case "string":
        errors = append(errors, v.validateString(data, schema, path)...)
    case "number", "integer":
        errors = append(errors, v.validateNumber(data, schema, path)...)
    case "boolean":
        errors = append(errors, v.validateBoolean(data, schema, path)...)
    }

    return errors
}

func (v *SchemaValidator) validateObject(data any, schema *JSONSchema, path string) []string {
    var errors []string

    obj, ok := data.(map[string]any)
    if !ok {
        return []string{fmt.Sprintf("%s: expected object", path)}
    }

    // Check required properties
    for _, req := range schema.Required {
        if _, ok := obj[req]; !ok {
            errors = append(errors, fmt.Sprintf("%s: missing required property %q", path, req))
        }
    }

    // Validate each property
    for propName, propSchema := range schema.Properties {
        if propValue, ok := obj[propName]; ok {
            propPath := path + "." + propName
            errors = append(errors, v.validateValue(propValue, propSchema, propPath)...)
        }
    }

    return errors
}

func (v *SchemaValidator) validateArray(data any, schema *JSONSchema, path string) []string {
    var errors []string

    arr, ok := data.([]any)
    if !ok {
        return []string{fmt.Sprintf("%s: expected array", path)}
    }

    if schema.Items != nil {
        for i, item := range arr {
            itemPath := fmt.Sprintf("%s[%d]", path, i)
            errors = append(errors, v.validateValue(item, schema.Items, itemPath)...)
        }
    }

    return errors
}

func (v *SchemaValidator) validateString(data any, schema *JSONSchema, path string) []string {
    str, ok := data.(string)
    if !ok {
        return []string{fmt.Sprintf("%s: expected string", path)}
    }

    var errors []string

    if schema.MinLength != nil && len(str) < *schema.MinLength {
        errors = append(errors, fmt.Sprintf("%s: string too short (min %d)", path, *schema.MinLength))
    }

    if schema.MaxLength != nil && len(str) > *schema.MaxLength {
        errors = append(errors, fmt.Sprintf("%s: string too long (max %d)", path, *schema.MaxLength))
    }

    if schema.Pattern != "" {
        re, err := regexp.Compile(schema.Pattern)
        if err == nil && !re.MatchString(str) {
            errors = append(errors, fmt.Sprintf("%s: string does not match pattern %q", path, schema.Pattern))
        }
    }

    return errors
}

func (v *SchemaValidator) validateNumber(data any, schema *JSONSchema, path string) []string {
    var num float64
    switch n := data.(type) {
    case float64:
        num = n
    case int:
        num = float64(n)
    default:
        return []string{fmt.Sprintf("%s: expected number", path)}
    }

    var errors []string

    if schema.Minimum != nil && num < *schema.Minimum {
        errors = append(errors, fmt.Sprintf("%s: number below minimum %f", path, *schema.Minimum))
    }

    if schema.Maximum != nil && num > *schema.Maximum {
        errors = append(errors, fmt.Sprintf("%s: number above maximum %f", path, *schema.Maximum))
    }

    return errors
}

func (v *SchemaValidator) validateBoolean(data any, schema *JSONSchema, path string) []string {
    if _, ok := data.(bool); !ok {
        return []string{fmt.Sprintf("%s: expected boolean", path)}
    }
    return nil
}
```

## Integration with SuperAgent

### Structured Generator Wrapper

```go
// internal/optimization/outlines/generator.go

package outlines

import (
    "context"
    "fmt"

    "superagent/internal/llm"
    "superagent/internal/models"
)

// StructuredGenerator wraps an LLM provider for structured output
type StructuredGenerator struct {
    provider  llm.LLMProvider
    tokenizer Tokenizer
}

// NewStructuredGenerator creates a new structured generator
func NewStructuredGenerator(provider llm.LLMProvider, tokenizer Tokenizer) *StructuredGenerator {
    return &StructuredGenerator{
        provider:  provider,
        tokenizer: tokenizer,
    }
}

// GenerateJSON generates output conforming to a JSON schema
func (g *StructuredGenerator) GenerateJSON(ctx context.Context, req *models.LLMRequest, schema *JSONSchema) (*StructuredResponse, error) {
    fsm, err := NewSchemaFSM(schema, g.tokenizer)
    if err != nil {
        return nil, fmt.Errorf("failed to build schema FSM: %w", err)
    }

    // Add schema instruction to prompt
    schemaJSON, _ := json.Marshal(schema)
    enhancedPrompt := fmt.Sprintf("%s\n\nRespond with valid JSON matching this schema:\n%s",
        req.Prompt, string(schemaJSON))

    // Generate with constraints
    // Note: This requires provider support for logit bias or constrained decoding
    // For providers without native support, we use validation + retry

    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        req.Prompt = enhancedPrompt
        resp, err := g.provider.Complete(ctx, req)
        if err != nil {
            return nil, err
        }

        validator := NewSchemaValidator(schema)
        result := validator.Validate(resp.Content)

        if result.Valid {
            return &StructuredResponse{
                Content:    resp.Content,
                ParsedData: result.Data,
                Valid:      true,
            }, nil
        }

        // Add error feedback to prompt for retry
        enhancedPrompt = fmt.Sprintf("%s\n\nPrevious attempt had errors: %v\nPlease try again:",
            enhancedPrompt, result.Errors)
    }

    return nil, fmt.Errorf("failed to generate valid JSON after %d retries", maxRetries)
}

// GenerateRegex generates output matching a regex pattern
func (g *StructuredGenerator) GenerateRegex(ctx context.Context, req *models.LLMRequest, pattern string) (*StructuredResponse, error) {
    fsm, err := NewRegexFSM(pattern, g.tokenizer)
    if err != nil {
        return nil, fmt.Errorf("failed to build regex FSM: %w", err)
    }

    // Similar approach - generate and validate
    enhancedPrompt := fmt.Sprintf("%s\n\nYour response must match this pattern: %s",
        req.Prompt, pattern)

    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        req.Prompt = enhancedPrompt
        resp, err := g.provider.Complete(ctx, req)
        if err != nil {
            return nil, err
        }

        if fsm.pattern.MatchString(resp.Content) {
            return &StructuredResponse{
                Content: resp.Content,
                Valid:   true,
            }, nil
        }
    }

    return nil, fmt.Errorf("failed to generate matching output after %d retries", maxRetries)
}

// StructuredResponse contains structured generation result
type StructuredResponse struct {
    Content    string
    ParsedData any
    Valid      bool
    Errors     []string
}
```

## Test Coverage Requirements

```go
// tests/optimization/unit/outlines/schema_engine_test.go

func TestSchemaFSM_BuildStates_Object(t *testing.T)
func TestSchemaFSM_BuildStates_Array(t *testing.T)
func TestSchemaFSM_BuildStates_Nested(t *testing.T)
func TestSchemaFSM_GetAllowedTokens(t *testing.T)
func TestSchemaFSM_Transition(t *testing.T)
func TestSchemaFSM_IsFinal(t *testing.T)

func TestRegexFSM_SimplePattern(t *testing.T)
func TestRegexFSM_ComplexPattern(t *testing.T)
func TestRegexFSM_GetAllowedTokens(t *testing.T)

func TestTokenMask_Allow(t *testing.T)
func TestTokenMask_ApplyToLogits(t *testing.T)

func TestSchemaValidator_ValidObject(t *testing.T)
func TestSchemaValidator_InvalidObject(t *testing.T)
func TestSchemaValidator_RequiredFields(t *testing.T)
func TestSchemaValidator_TypeMismatch(t *testing.T)
func TestSchemaValidator_StringConstraints(t *testing.T)
func TestSchemaValidator_NumberConstraints(t *testing.T)
func TestSchemaValidator_NestedValidation(t *testing.T)

func TestStructuredGenerator_GenerateJSON(t *testing.T)
func TestStructuredGenerator_GenerateRegex(t *testing.T)
func TestStructuredGenerator_Retry(t *testing.T)
```

## Conclusion

Outlines is a strong candidate for native Go implementation. The core concepts (FSM for JSON schema, token masking, validation) translate well to Go. The main challenge is building the FSM state machine correctly for complex nested schemas.

**Key Trade-off**: For providers that don't support logit bias or constrained decoding, the Go implementation will use a validation + retry approach rather than true token-by-token masking. This is less efficient but works with any provider.

**Estimated Implementation Time**: 2 weeks
**Risk Level**: Medium (FSM complexity)
**Dependencies**: Tokenizer (can use provider API or simple heuristics)
