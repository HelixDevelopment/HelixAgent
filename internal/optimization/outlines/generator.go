package outlines

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// StructuredResponse contains the result of structured generation.
type StructuredResponse struct {
	Content    string      `json:"content"`
	ParsedData interface{} `json:"parsed_data,omitempty"`
	Valid      bool        `json:"valid"`
	Errors     []string    `json:"errors,omitempty"`
	Retries    int         `json:"retries"`
}

// GeneratorConfig configures the structured generator.
type GeneratorConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// StrictMode fails immediately on validation error instead of retrying.
	StrictMode bool
	// IncludeSchemaInPrompt includes the schema in the prompt.
	IncludeSchemaInPrompt bool
	// IncludeErrorFeedback includes error messages in retry prompts.
	IncludeErrorFeedback bool
}

// DefaultGeneratorConfig returns a default generator configuration.
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		MaxRetries:            3,
		StrictMode:            false,
		IncludeSchemaInPrompt: true,
		IncludeErrorFeedback:  true,
	}
}

// LLMProvider defines the interface for LLM providers.
type LLMProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// StructuredGenerator generates structured output conforming to a schema.
type StructuredGenerator struct {
	provider  LLMProvider
	config    *GeneratorConfig
	validator *SchemaValidator
	schema    *JSONSchema
}

// NewStructuredGenerator creates a new structured generator.
func NewStructuredGenerator(provider LLMProvider, schema *JSONSchema, config *GeneratorConfig) (*StructuredGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	validator, err := NewSchemaValidator(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	return &StructuredGenerator{
		provider:  provider,
		config:    config,
		validator: validator,
		schema:    schema,
	}, nil
}

// Generate generates structured output for the given prompt.
func (g *StructuredGenerator) Generate(ctx context.Context, prompt string) (*StructuredResponse, error) {
	enhancedPrompt := g.buildPrompt(prompt, nil)

	for retry := 0; retry <= g.config.MaxRetries; retry++ {
		response, err := g.provider.Complete(ctx, enhancedPrompt)
		if err != nil {
			return nil, fmt.Errorf("LLM completion failed: %w", err)
		}

		// Extract JSON from response
		jsonStr := extractJSON(response)
		if jsonStr == "" {
			if g.config.StrictMode {
				return &StructuredResponse{
					Content: response,
					Valid:   false,
					Errors:  []string{"no valid JSON found in response"},
					Retries: retry,
				}, nil
			}

			// Retry with feedback
			enhancedPrompt = g.buildPrompt(prompt, []string{"Response must be valid JSON"})
			continue
		}

		// Validate against schema
		result := g.validator.Validate(jsonStr)

		if result.Valid {
			return &StructuredResponse{
				Content:    jsonStr,
				ParsedData: result.Data,
				Valid:      true,
				Retries:    retry,
			}, nil
		}

		if g.config.StrictMode {
			return &StructuredResponse{
				Content: jsonStr,
				Valid:   false,
				Errors:  result.ErrorMessages(),
				Retries: retry,
			}, nil
		}

		// Retry with error feedback
		enhancedPrompt = g.buildPrompt(prompt, result.ErrorMessages())
	}

	return &StructuredResponse{
		Content: "",
		Valid:   false,
		Errors:  []string{fmt.Sprintf("failed to generate valid output after %d retries", g.config.MaxRetries)},
		Retries: g.config.MaxRetries,
	}, nil
}

// GenerateWithType generates output and unmarshals to the given type.
func (g *StructuredGenerator) GenerateWithType(ctx context.Context, prompt string, target interface{}) error {
	response, err := g.Generate(ctx, prompt)
	if err != nil {
		return err
	}

	if !response.Valid {
		return fmt.Errorf("validation failed: %v", response.Errors)
	}

	return json.Unmarshal([]byte(response.Content), target)
}

func (g *StructuredGenerator) buildPrompt(userPrompt string, errors []string) string {
	var parts []string

	parts = append(parts, userPrompt)

	if g.config.IncludeSchemaInPrompt {
		schemaJSON, _ := json.MarshalIndent(g.schema, "", "  ")
		parts = append(parts, fmt.Sprintf("\nRespond with valid JSON matching this schema:\n```json\n%s\n```", string(schemaJSON)))
	}

	if g.config.IncludeErrorFeedback && len(errors) > 0 {
		parts = append(parts, fmt.Sprintf("\nPrevious response had errors:\n- %s\nPlease fix these issues.", strings.Join(errors, "\n- ")))
	}

	parts = append(parts, "\nRespond ONLY with the JSON, no additional text.")

	return strings.Join(parts, "\n")
}

// extractJSON extracts JSON content from a response that may contain other text.
func extractJSON(response string) string {
	response = strings.TrimSpace(response)

	// Try to find JSON object
	if start := strings.Index(response, "{"); start != -1 {
		if end := findMatchingBrace(response[start:], '{', '}'); end != -1 {
			return response[start : start+end+1]
		}
	}

	// Try to find JSON array
	if start := strings.Index(response, "["); start != -1 {
		if end := findMatchingBrace(response[start:], '[', ']'); end != -1 {
			return response[start : start+end+1]
		}
	}

	// Check if entire response is valid JSON
	var js interface{}
	if err := json.Unmarshal([]byte(response), &js); err == nil {
		return response
	}

	// Try to extract from markdown code block
	if strings.Contains(response, "```json") {
		start := strings.Index(response, "```json") + 7
		end := strings.Index(response[start:], "```")
		if end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	if strings.Contains(response, "```") {
		start := strings.Index(response, "```") + 3
		// Skip language identifier if present
		if newline := strings.Index(response[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		end := strings.Index(response[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(response[start : start+end])
			var js interface{}
			if err := json.Unmarshal([]byte(candidate), &js); err == nil {
				return candidate
			}
		}
	}

	return ""
}

func findMatchingBrace(s string, open, close byte) int {
	if len(s) == 0 || s[0] != open {
		return -1
	}

	count := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == open {
			count++
		} else if c == close {
			count--
			if count == 0 {
				return i
			}
		}
	}

	return -1
}

// RegexGenerator generates output matching a regex pattern.
type RegexGenerator struct {
	provider LLMProvider
	pattern  *CompiledPattern
	config   *GeneratorConfig
}

// NewRegexGenerator creates a new regex generator.
func NewRegexGenerator(provider LLMProvider, pattern string, config *GeneratorConfig) (*RegexGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	compiled, err := CompilePattern(pattern)
	if err != nil {
		return nil, err
	}

	return &RegexGenerator{
		provider: provider,
		pattern:  compiled,
		config:   config,
	}, nil
}

// Generate generates output matching the regex pattern.
func (g *RegexGenerator) Generate(ctx context.Context, prompt string) (*StructuredResponse, error) {
	enhancedPrompt := fmt.Sprintf("%s\n\nYour response must match this pattern: %s", prompt, g.pattern.Pattern)

	for retry := 0; retry <= g.config.MaxRetries; retry++ {
		response, err := g.provider.Complete(ctx, enhancedPrompt)
		if err != nil {
			return nil, fmt.Errorf("LLM completion failed: %w", err)
		}

		response = strings.TrimSpace(response)

		if g.pattern.Match(response) {
			return &StructuredResponse{
				Content: response,
				Valid:   true,
				Retries: retry,
			}, nil
		}

		if g.config.StrictMode {
			return &StructuredResponse{
				Content: response,
				Valid:   false,
				Errors:  []string{fmt.Sprintf("response does not match pattern %q", g.pattern.Pattern)},
				Retries: retry,
			}, nil
		}

		// Retry with feedback
		enhancedPrompt = fmt.Sprintf("%s\n\nYour response must match this pattern: %s\nYour previous response did not match. Please try again.", prompt, g.pattern.Pattern)
	}

	return &StructuredResponse{
		Content: "",
		Valid:   false,
		Errors:  []string{fmt.Sprintf("failed to generate matching output after %d retries", g.config.MaxRetries)},
		Retries: g.config.MaxRetries,
	}, nil
}

// ChoiceGenerator generates output from a set of choices.
type ChoiceGenerator struct {
	provider LLMProvider
	choices  []string
	config   *GeneratorConfig
}

// NewChoiceGenerator creates a new choice generator.
func NewChoiceGenerator(provider LLMProvider, choices []string, config *GeneratorConfig) *ChoiceGenerator {
	if config == nil {
		config = DefaultGeneratorConfig()
	}

	return &ChoiceGenerator{
		provider: provider,
		choices:  choices,
		config:   config,
	}
}

// Generate generates output that is one of the choices.
func (g *ChoiceGenerator) Generate(ctx context.Context, prompt string) (*StructuredResponse, error) {
	choiceList := strings.Join(g.choices, ", ")
	enhancedPrompt := fmt.Sprintf("%s\n\nRespond with exactly one of these options: %s", prompt, choiceList)

	for retry := 0; retry <= g.config.MaxRetries; retry++ {
		response, err := g.provider.Complete(ctx, enhancedPrompt)
		if err != nil {
			return nil, fmt.Errorf("LLM completion failed: %w", err)
		}

		response = strings.TrimSpace(response)

		// Check exact match
		for _, choice := range g.choices {
			if response == choice {
				return &StructuredResponse{
					Content: response,
					Valid:   true,
					Retries: retry,
				}, nil
			}
		}

		// Check case-insensitive match
		for _, choice := range g.choices {
			if strings.EqualFold(response, choice) {
				return &StructuredResponse{
					Content: choice, // Return the canonical choice
					Valid:   true,
					Retries: retry,
				}, nil
			}
		}

		// Check if response contains a choice
		for _, choice := range g.choices {
			if strings.Contains(strings.ToLower(response), strings.ToLower(choice)) {
				return &StructuredResponse{
					Content: choice,
					Valid:   true,
					Retries: retry,
				}, nil
			}
		}

		if g.config.StrictMode {
			return &StructuredResponse{
				Content: response,
				Valid:   false,
				Errors:  []string{fmt.Sprintf("response must be one of: %s", choiceList)},
				Retries: retry,
			}, nil
		}

		// Retry with stronger instruction
		enhancedPrompt = fmt.Sprintf("%s\n\nRespond with EXACTLY one of these options (no other text): %s\nYour previous response was not a valid option.", prompt, choiceList)
	}

	return &StructuredResponse{
		Content: "",
		Valid:   false,
		Errors:  []string{fmt.Sprintf("failed to generate valid choice after %d retries", g.config.MaxRetries)},
		Retries: g.config.MaxRetries,
	}, nil
}
