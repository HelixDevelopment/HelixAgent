package build

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerProjectRoot returns the project root directory relative to this test file.
// This avoids a redeclaration conflict with projectRoot() in release_build_test.go
// since both files share the same package.
func providerProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// readSourceFile reads a source file relative to the project root.
func readSourceFile(t *testing.T, relPath string) string {
	t.Helper()
	root := providerProjectRoot()
	path := filepath.Join(root, relPath)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read %s", relPath)
	return string(data)
}

// countInStringLiterals counts occurrences of substr that appear inside Go string
// literals (between double quotes) in the source content, excluding comments.
// This avoids false positives from code comments mentioning incorrect domains.
func countInStringLiterals(content, substr string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comment-only lines
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Remove inline comments (simple heuristic: after // outside of quotes)
		// For our purposes, just check if substr appears inside quoted strings
		inString := false
		escaped := false
		for i := 0; i < len(line); i++ {
			if escaped {
				escaped = false
				continue
			}
			if line[i] == '\\' {
				escaped = true
				continue
			}
			if line[i] == '"' {
				inString = !inString
				continue
			}
			if inString && i+len(substr) <= len(line) && line[i:i+len(substr)] == substr {
				count++
				// Skip past this match to avoid double-counting overlapping matches
				i += len(substr) - 1
			}
		}
	}
	return count
}

// --- Test 1: Chutes URL Consistency ---

func TestProviderURLConsistency_ChutesUsesCorrectDomain(t *testing.T) {
	// Chutes inference API uses llm.chutes.ai, NOT api.chutes.ai.
	// Verify all source files reference the correct domain in string literals (not comments).

	filesToCheck := []string{
		"internal/verifier/provider_types.go",
		"internal/services/provider_discovery.go",
		"internal/verifier/provider_access.go",
	}

	for _, relPath := range filesToCheck {
		t.Run(relPath, func(t *testing.T) {
			content := readSourceFile(t, relPath)

			// Must contain the correct domain llm.chutes.ai in string literals
			if strings.Contains(content, "chutes") {
				correctCount := countInStringLiterals(content, "llm.chutes.ai")
				incorrectCount := countInStringLiterals(content, "api.chutes.ai")

				assert.Zero(t, incorrectCount,
					"%s contains api.chutes.ai in string literals (incorrect domain); should be llm.chutes.ai",
					relPath)

				// At least one string literal should use the correct domain if the file
				// has chutes URLs in string literals
				if countInStringLiterals(content, "chutes.ai") > 0 {
					assert.Greater(t, correctCount, 0,
						"%s has chutes.ai in string literals but does not use llm.chutes.ai", relPath)
				}
			}
		})
	}
}

// --- Test 2: Cohere URL Consistency ---

func TestProviderURLConsistency_CohereUsesCorrectDomain(t *testing.T) {
	// Cohere uses api.cohere.com (not api.cohere.ai) and v2 API.

	filesToCheck := []string{
		"internal/verifier/provider_types.go",
		"internal/services/provider_discovery.go",
		"internal/verifier/provider_access.go",
	}

	for _, relPath := range filesToCheck {
		t.Run(relPath, func(t *testing.T) {
			content := readSourceFile(t, relPath)

			if strings.Contains(content, "cohere") {
				// Must use api.cohere.com, not api.cohere.ai
				incorrectCount := strings.Count(content, "api.cohere.ai")
				assert.Zero(t, incorrectCount,
					"%s contains api.cohere.ai (incorrect domain); should be api.cohere.com", relPath)

				// Verify the correct domain is present
				if strings.Contains(content, "cohere.com") || strings.Contains(content, "cohere.ai") {
					correctCount := strings.Count(content, "api.cohere.com")
					assert.Greater(t, correctCount, 0,
						"%s references cohere but does not use api.cohere.com", relPath)
				}

				// Verify v2 API usage where URLs reference cohere.com
				if strings.Contains(content, "api.cohere.com") {
					assert.True(t,
						strings.Contains(content, "api.cohere.com/v2"),
						"%s uses api.cohere.com but does not reference v2 API", relPath)
				}
			}
		})
	}
}

// --- Test 3: ZAI URL Consistency ---

func TestProviderURLConsistency_ZAIUsesCorrectDomain(t *testing.T) {
	// ZAI (Zhipu GLM) uses api.z.ai (international endpoint) across all source files.
	// Switched from open.bigmodel.cn (China endpoint) to api.z.ai in commit 189f830e.

	filesToCheck := []string{
		"internal/verifier/provider_types.go",
		"internal/services/provider_discovery.go",
		"internal/verifier/provider_access.go",
	}

	for _, relPath := range filesToCheck {
		t.Run(relPath, func(t *testing.T) {
			content := readSourceFile(t, relPath)

			if strings.Contains(content, `"zai"`) {
				// All ZAI URLs must reference api.z.ai (international endpoint)
				correctCount := strings.Count(content, "api.z.ai")
				assert.Greater(t, correctCount, 0,
					"%s references zai provider but does not contain api.z.ai", relPath)

				// No other ZAI-related legacy domains should appear
				assert.False(t, strings.Contains(content, "api.zhipu.ai"),
					"%s contains incorrect domain api.zhipu.ai; should be api.z.ai", relPath)
			}
		})
	}
}

// --- Test 4: Kimi/Moonshot URL Consistency ---

func TestProviderURLConsistency_KimiUsesConsistentDomain(t *testing.T) {
	// Kimi (Moonshot) should use api.moonshot.cn consistently.

	filesToCheck := []string{
		"internal/verifier/provider_types.go",
		"internal/services/provider_discovery.go",
	}

	for _, relPath := range filesToCheck {
		t.Run(relPath, func(t *testing.T) {
			content := readSourceFile(t, relPath)

			if strings.Contains(content, `"kimi"`) || strings.Contains(content, "moonshot") {
				// All Kimi/Moonshot URLs must reference api.moonshot.cn
				correctCount := strings.Count(content, "api.moonshot.cn")
				assert.Greater(t, correctCount, 0,
					"%s references kimi/moonshot but does not contain api.moonshot.cn", relPath)

				// Verify all moonshot URLs use the same domain
				assert.False(t, strings.Contains(content, "api.kimi.ai"),
					"%s contains api.kimi.ai; should use api.moonshot.cn", relPath)
				assert.False(t, strings.Contains(content, "api.moonshot.ai"),
					"%s contains api.moonshot.ai; should use api.moonshot.cn", relPath)
			}
		})
	}
}

// --- Test 5: Qwen Uses Compatible Mode ---

func TestProviderURLConsistency_QwenUsesCompatibleMode(t *testing.T) {
	// Qwen must use compatible-mode in its DashScope URLs for OpenAI-compatible API.

	filesToCheck := []string{
		"internal/services/provider_discovery.go",
		"internal/verifier/provider_types.go",
		"internal/verifier/provider_access.go",
		"internal/verifier/startup.go",
	}

	for _, relPath := range filesToCheck {
		t.Run(relPath, func(t *testing.T) {
			content := readSourceFile(t, relPath)

			if strings.Contains(content, "dashscope.aliyuncs.com") {
				assert.True(t, strings.Contains(content, "compatible-mode"),
					"%s references dashscope.aliyuncs.com but missing compatible-mode path segment", relPath)
			}
		})
	}
}

// --- Test 6: All Providers Have Models ---

func TestProviderTypes_AllProvidersHaveModels(t *testing.T) {
	// Verify that key providers in SupportedProviders (provider_types.go) have non-empty model lists.
	// At minimum, providers that participate in the debate team must have models defined.

	content := readSourceFile(t, "internal/verifier/provider_types.go")

	// Providers that MUST have models for debate team functionality
	coreProviders := []string{
		"claude", "deepseek", "gemini", "mistral", "groq",
		"cerebras", "zai", "chutes", "cohere", "ai21",
		"zen", "ollama",
	}

	for _, provider := range coreProviders {
		t.Run(provider, func(t *testing.T) {
			// Find the provider block in SupportedProviders
			providerKey := `"` + provider + `"`
			idx := strings.Index(content, providerKey+":")
			if idx == -1 {
				// Try alternate format: just the key in the map
				idx = strings.Index(content, providerKey+": {")
			}
			require.NotEqual(t, -1, idx,
				"provider %s not found in SupportedProviders map", provider)

			// Extract a block of text after the provider definition to check Models field
			endIdx := idx + 500
			if endIdx > len(content) {
				endIdx = len(content)
			}
			block := content[idx:endIdx]

			// The Models field should not be an empty slice for core providers
			assert.False(t, strings.Contains(block, `Models:      []string{}`),
				"provider %s has empty Models slice in SupportedProviders", provider)
		})
	}
}

// --- Test 7: Chutes Fallback Models Use Correct org/Model Format ---

func TestProviderDiscovery_FallbackModelsMatchNaming(t *testing.T) {
	// Verify that Chutes fallback models in startup.go use the org/Model-Name format
	// (e.g., deepseek-ai/DeepSeek-V3) as required by the Chutes API.

	content := readSourceFile(t, "internal/verifier/startup.go")

	// Find the Chutes fallback models section
	require.True(t, strings.Contains(content, "fallbackModels"),
		"startup.go must contain fallbackModels for Chutes")

	// Extract the Chutes DiscoverModels section
	chutesIdx := strings.Index(content, `case "chutes"`)
	require.NotEqual(t, -1, chutesIdx, "startup.go must have Chutes case in DiscoverModels")

	// Get a block after the chutes case
	endIdx := chutesIdx + 800
	if endIdx > len(content) {
		endIdx = len(content)
	}
	chutesBlock := content[chutesIdx:endIdx]

	// Chutes models must use org/Model format (containing a slash)
	expectedModels := []string{
		"deepseek-ai/DeepSeek-V3",
		"deepseek-ai/DeepSeek-R1",
	}

	for _, model := range expectedModels {
		assert.True(t, strings.Contains(chutesBlock, model),
			"Chutes fallback models should include %s (org/Model format)", model)
	}

	// Verify no incorrect model names without org prefix exist in fallback
	assert.False(t, strings.Contains(chutesBlock, `"DeepSeek-V3"`),
		"Chutes fallback models should use org/Model format, not bare Model name")
	assert.False(t, strings.Contains(chutesBlock, `"DeepSeek-R1"`),
		"Chutes fallback models should use org/Model format, not bare Model name")

	// Also verify the static model list in provider_types.go uses the same format
	typesContent := readSourceFile(t, "internal/verifier/provider_types.go")
	chutesTypesIdx := strings.Index(typesContent, `"chutes":`)
	require.NotEqual(t, -1, chutesTypesIdx,
		"provider_types.go must contain chutes provider definition")

	typesEndIdx := chutesTypesIdx + 500
	if typesEndIdx > len(typesContent) {
		typesEndIdx = len(typesContent)
	}
	chutesTypesBlock := typesContent[chutesTypesIdx:typesEndIdx]

	// Verify org/Model format in static model list
	assert.True(t, strings.Contains(chutesTypesBlock, "deepseek-ai/DeepSeek-V3"),
		"Chutes models in provider_types.go should use org/Model format (deepseek-ai/DeepSeek-V3)")
	assert.True(t, strings.Contains(chutesTypesBlock, "deepseek-ai/DeepSeek-R1"),
		"Chutes models in provider_types.go should use org/Model format (deepseek-ai/DeepSeek-R1)")
}

// --- Test 8: Base URLs Are Valid ---

func TestProviderTypes_BaseURLsAreValid(t *testing.T) {
	// Verify all BaseURLs in SupportedProviders start with https:// (or http:// for local)
	// and contain expected path patterns.

	content := readSourceFile(t, "internal/verifier/provider_types.go")

	// Extract all BaseURL values
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "BaseURL:") {
			continue
		}

		t.Run("BaseURL_line", func(t *testing.T) {
			// Extract the URL value between quotes
			firstQuote := strings.Index(trimmed, `"`)
			lastQuote := strings.LastIndex(trimmed, `"`)
			if firstQuote == -1 || lastQuote == -1 || firstQuote == lastQuote {
				t.Skipf("Could not extract URL from line: %s (SKIP-OK: #unmarked-skip-needs-ticket)", trimmed)
				return
			}
			url := trimmed[firstQuote+1 : lastQuote]

			// Empty BaseURL is valid for CLI-based providers (e.g., Junie)
			// that use CLI proxy mode instead of direct HTTP API access.
			if url == "" {
				t.Skipf("Empty BaseURL (CLI-based provider): %s (SKIP-OK: #unmarked-skip-needs-ticket)", trimmed)
				return
			}

			// Must start with https:// or http:// (for localhost/ollama)
			isHTTPS := strings.HasPrefix(url, "https://")
			isHTTP := strings.HasPrefix(url, "http://")
			assert.True(t, isHTTPS || isHTTP,
				"BaseURL must start with https:// or http://, got: %s", url)

			// HTTP is only acceptable for localhost (Ollama)
			if isHTTP && !isHTTPS {
				assert.True(t, strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1"),
					"HTTP (non-HTTPS) BaseURL should only be used for localhost, got: %s", url)
			}

			// URL should contain a domain (at least one dot or localhost)
			urlWithoutScheme := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
			assert.True(t,
				strings.Contains(urlWithoutScheme, ".") || strings.Contains(urlWithoutScheme, "localhost"),
				"BaseURL should contain a valid domain, got: %s", url)
		})
	}
}

// --- Test 9: No Mixed Domains for Same Provider ---

func TestProviderURLConsistency_NoMixedDomains(t *testing.T) {
	// Cross-check that for each provider, all source files use the same domain.
	// Read key source files and compare domain patterns for each provider.

	type providerDomainSpec struct {
		name            string
		expectedDomain  string
		forbiddenDomain string
	}

	specs := []providerDomainSpec{
		{
			name:            "chutes",
			expectedDomain:  "llm.chutes.ai",
			forbiddenDomain: "api.chutes.ai",
		},
		{
			name:            "cohere",
			expectedDomain:  "api.cohere.com",
			forbiddenDomain: "api.cohere.ai",
		},
		{
			name:            "zai",
			expectedDomain:  "api.z.ai",
			forbiddenDomain: "api.zhipu.ai",
		},
		{
			name:           "deepseek",
			expectedDomain: "api.deepseek.com",
		},
		{
			name:           "mistral",
			expectedDomain: "api.mistral.ai",
		},
		{
			name:           "groq",
			expectedDomain: "api.groq.com",
		},
		{
			name:           "cerebras",
			expectedDomain: "api.cerebras.ai",
		},
		{
			name:           "anthropic/claude",
			expectedDomain: "api.anthropic.com",
		},
	}

	sourceFiles := []string{
		"internal/verifier/provider_types.go",
		"internal/verifier/provider_access.go",
		"internal/services/provider_discovery.go",
		"internal/verifier/startup.go",
	}

	// Read all files once
	fileContents := make(map[string]string)
	for _, relPath := range sourceFiles {
		fileContents[relPath] = readSourceFile(t, relPath)
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			for relPath, content := range fileContents {
				// Only check files that reference this provider
				providerKey := spec.name
				if strings.Contains(providerKey, "/") {
					// Handle compound names like "anthropic/claude"
					parts := strings.Split(providerKey, "/")
					found := false
					for _, part := range parts {
						if strings.Contains(content, `"`+part+`"`) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				} else if !strings.Contains(content, `"`+providerKey+`"`) {
					continue
				}

				// If file references the provider, verify domain consistency
				// Check string literals (not comments) for the expected domain
				expectedInLiterals := countInStringLiterals(content, spec.expectedDomain)
				forbiddenInLiterals := 0
				if spec.forbiddenDomain != "" {
					forbiddenInLiterals = countInStringLiterals(content, spec.forbiddenDomain)
				}

				if expectedInLiterals > 0 || forbiddenInLiterals > 0 {
					assert.Greater(t, expectedInLiterals, 0,
						"%s: expected domain %s for provider %s not found in string literals",
						relPath, spec.expectedDomain, spec.name)

					if spec.forbiddenDomain != "" {
						assert.Zero(t, forbiddenInLiterals,
							"%s: forbidden domain %s found in string literals for provider %s (should use %s)",
							relPath, spec.forbiddenDomain, spec.name, spec.expectedDomain)
					}
				}
			}
		})
	}
}
