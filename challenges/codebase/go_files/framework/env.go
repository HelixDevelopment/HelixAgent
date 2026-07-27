// Package framework provides environment variable handling with security features.
package framework

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// EnvLoader implements EnvironmentLoader for secure environment handling.
type EnvLoader struct {
	values    map[string]string
	loaded    bool
	providers map[string]string // provider name -> env var suffix
}

// NewEnvLoader creates a new environment loader.
func NewEnvLoader() *EnvLoader {
	return &EnvLoader{
		values: make(map[string]string),
		providers: map[string]string{
			// HelixAgent primary providers
			"anthropic":  "ANTHROPIC_API_KEY",
			"openai":     "OPENAI_API_KEY",
			"deepseek":   "DEEPSEEK_API_KEY",
			"gemini":     "GEMINI_API_KEY",
			"openrouter": "OPENROUTER_API_KEY",
			"qwen":       "QWEN_API_KEY",
			"zai":        "ZAI_API_KEY",
			"ollama":     "OLLAMA_BASE_URL",

			// LLMsVerifier extended providers
			"huggingface": "HUGGINGFACE_API_KEY",
			"nvidia":      "NVIDIA_API_KEY",
			"chutes":      "CHUTES_API_KEY",
			"siliconflow": "SILICONFLOW_API_KEY",
			"kimi":        "KIMI_API_KEY",
			"mistral":     "MISTRAL_API_KEY",
			"codestral":   "CODESTRAL_API_KEY",
			"vercel":      "VERCEL_AI_API_KEY",
			"cerebras":    "CEREBRAS_API_KEY",
			"cloudflare":  "CLOUDFLARE_API_KEY",
			"fireworks":   "FIREWORKS_API_KEY",
			"baseten":     "BASETEN_API_KEY",
			"novita":      "NOVITA_API_KEY",
			"upstage":     "UPSTAGE_API_KEY",
			"nlp_cloud":   "NLP_CLOUD_API_KEY",
			"modal":       "MODAL_API_KEY",
			"inference":   "INFERENCE_API_KEY",
		},
	}
}

// Load loads environment variables from a .env file.
func (e *EnvLoader) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No .env file is okay, we'll use system env
			e.loaded = true
			return nil
		}
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		value = trimQuotes(value)

		e.values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	e.loaded = true
	return nil
}

// Get retrieves an environment variable value.
// First checks loaded values, then falls back to system environment.
func (e *EnvLoader) Get(key string) string {
	if val, exists := e.values[key]; exists {
		return val
	}
	return os.Getenv(key)
}

// GetRequired retrieves a required environment variable.
func (e *EnvLoader) GetRequired(key string) (string, error) {
	val := e.Get(key)
	if val == "" {
		return "", fmt.Errorf("required environment variable not set: %s", key)
	}
	return val, nil
}

// GetWithDefault retrieves an environment variable with a default value.
func (e *EnvLoader) GetWithDefault(key, defaultValue string) string {
	if val := e.Get(key); val != "" {
		return val
	}
	return defaultValue
}

// GetAPIKey retrieves an API key for a specific provider.
func (e *EnvLoader) GetAPIKey(provider string) string {
	provider = strings.ToLower(provider)

	// Check provider mapping
	if envVar, exists := e.providers[provider]; exists {
		return e.Get(envVar)
	}

	// Try common patterns
	patterns := []string{
		strings.ToUpper(provider) + "_API_KEY",
		strings.ToUpper(provider) + "_KEY",
		strings.ToUpper(provider) + "_TOKEN",
	}

	for _, pattern := range patterns {
		if val := e.Get(pattern); val != "" {
			return val
		}
	}

	return ""
}

// ListConfiguredProviders returns providers with configured API keys.
func (e *EnvLoader) ListConfiguredProviders() []string {
	var configured []string

	for provider := range e.providers {
		if e.GetAPIKey(provider) != "" {
			configured = append(configured, provider)
		}
	}

	return configured
}

// Redact returns a redacted version of a sensitive value.
func (e *EnvLoader) Redact(value string) string {
	return RedactAPIKey(value)
}

// GetAll returns all loaded environment variables.
func (e *EnvLoader) GetAll() map[string]string {
	result := make(map[string]string)
	for k, v := range e.values {
		result[k] = v
	}
	return result
}

// GetAllRedacted returns all loaded environment variables with values redacted.
func (e *EnvLoader) GetAllRedacted() map[string]string {
	result := make(map[string]string)
	for k, v := range e.values {
		if isSecretKey(k) {
			result[k] = RedactAPIKey(v)
		} else {
			result[k] = v
		}
	}
	return result
}

// IsLoaded returns whether the environment has been loaded.
func (e *EnvLoader) IsLoaded() bool {
	return e.loaded
}

// SetProvider adds or updates a provider mapping.
func (e *EnvLoader) SetProvider(name, envVar string) {
	e.providers[strings.ToLower(name)] = envVar
}

// ResolveTemplate resolves ${VAR} placeholders in a string.
func (e *EnvLoader) ResolveTemplate(template string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		varName := match[2 : len(match)-1]
		return e.Get(varName)
	})
}

// Utility functions

// RedactAPIKey returns a redacted version of an API key.
func RedactAPIKey(key string) string {
	if key == "" {
		return ""
	}

	length := len(key)
	if length <= 4 {
		return strings.Repeat("*", length)
	}

	if length <= 8 {
		return key[:2] + strings.Repeat("*", length-2)
	}

	// Show first 4 characters, mask the rest
	return key[:4] + strings.Repeat("*", length-4)
}

// RedactURL redacts sensitive parts of URLs (API keys in query params).
func RedactURL(url string) string {
	// Redact common API key query parameters
	patterns := []string{
		`(api[_-]?key=)[^&]+`,
		`(key=)[^&]+`,
		`(token=)[^&]+`,
		`(secret=)[^&]+`,
	}

	result := url
	for _, pattern := range patterns {
		re := regexp.MustCompile("(?i)" + pattern)
		result = re.ReplaceAllString(result, "${1}****")
	}

	return result
}

// RedactHeaders redacts sensitive headers.
func RedactHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string)

	sensitiveHeaders := map[string]bool{
		"authorization":  true,
		"x-api-key":      true,
		"api-key":        true,
		"x-auth-token":   true,
		"x-access-token": true,
		"bearer":         true,
	}

	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			result[k] = "****"
		} else {
			result[k] = v
		}
	}

	return result
}

// isSecretKey checks if an environment variable name likely contains a secret.
func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	secretPatterns := []string{
		"api_key",
		"apikey",
		"secret",
		"password",
		"token",
		"auth",
		"credential",
		"private",
	}

	for _, pattern := range secretPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// trimQuotes removes surrounding quotes from a string.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ValidateAPIKeyFormat validates the format of an API key for a provider.
func ValidateAPIKeyFormat(provider, key string) error {
	if key == "" {
		return fmt.Errorf("API key is empty")
	}

	provider = strings.ToLower(provider)

	// Known prefixes for validation
	prefixes := map[string][]string{
		"anthropic":   {"sk-ant-"},
		"openai":      {"sk-"},
		"openrouter":  {"sk-or-"},
		"deepseek":    {"sk-"},
		"nvidia":      {"nvapi-"},
		"huggingface": {"hf_"},
	}

	if expectedPrefixes, exists := prefixes[provider]; exists {
		valid := false
		for _, prefix := range expectedPrefixes {
			if strings.HasPrefix(key, prefix) {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid %s API key format: expected prefix %v", provider, expectedPrefixes)
		}
	}

	return nil
}

// ProviderConfig holds configuration for a provider.
type ProviderConfig struct {
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	APIKey    string   `json:"api_key"`     // Actual key (not stored in git-versioned files)
	APIKeyVar string   `json:"api_key_var"` // Environment variable name
	BaseURL   string   `json:"base_url,omitempty"`
	Models    []string `json:"models,omitempty"`
}

// LoadProviderConfigs loads provider configurations with resolved API keys.
func (e *EnvLoader) LoadProviderConfigs() []ProviderConfig {
	var configs []ProviderConfig

	for provider, envVar := range e.providers {
		key := e.Get(envVar)
		config := ProviderConfig{
			Name:      provider,
			Enabled:   key != "",
			APIKey:    key,
			APIKeyVar: envVar,
		}

		// Special handling for providers with base URLs
		if provider == "ollama" {
			config.BaseURL = e.GetWithDefault("OLLAMA_BASE_URL", "http://localhost:11434")
		}

		configs = append(configs, config)
	}

	return configs
}

// WriteRedactedEnv writes a redacted version of the environment to a file.
func (e *EnvLoader) WriteRedactedEnv(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create redacted env file: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, _ = file.WriteString("# Redacted environment configuration\n")
	_, _ = file.WriteString("# Generated for logging/debugging purposes\n")
	_, _ = file.WriteString("# Contains no actual secrets\n\n")

	for k, v := range e.values {
		if isSecretKey(k) {
			_, _ = fmt.Fprintf(file, "%s=%s\n", k, RedactAPIKey(v))
		} else {
			_, _ = fmt.Fprintf(file, "%s=%s\n", k, v)
		}
	}

	return nil
}
