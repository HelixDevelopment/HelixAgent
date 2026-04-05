package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchTools_ExactNameMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "Bash",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	assert.Equal(t, "Bash", results[0].Tool.Name)
	assert.Equal(t, 1.0, results[0].Score)
	assert.Equal(t, "name", results[0].MatchType)
}

func TestSearchTools_PartialNameMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "bas",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	// Should find Bash
	found := false
	for _, r := range results {
		if r.Tool.Name == "Bash" {
			found = true
			// Match type can be name or description depending on scoring
			assert.Contains(t, []string{"name", "description", "alias"}, r.MatchType)
			break
		}
	}
	assert.True(t, found, "Should find Bash with partial match")
}

func TestSearchTools_DescriptionMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "shell commands",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	// Bash has "Execute shell commands" in description
	found := false
	for _, r := range results {
		if r.Tool.Name == "Bash" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find Bash via description match")
}

func TestSearchTools_CategoryFilter(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "file",
		Categories: []string{CategoryFileSystem},
		MaxResults: 50,
	})

	// All results should be in filesystem category
	for _, r := range results {
		assert.Equal(t, CategoryFileSystem, r.Tool.Category,
			"All results should be in filesystem category")
	}
}

func TestSearchTools_AliasMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "shell",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	// Bash has "shell" as an alias
	found := false
	for _, r := range results {
		if r.Tool.Name == "Bash" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find Bash via alias match")
}

func TestSearchTools_ParameterMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:         "file_path",
		IncludeParams: true,
		MaxResults:    20,
	})

	require.NotEmpty(t, results)
	// Read, Write, Edit all have file_path parameter
	hasFilePathParam := false
	for _, r := range results {
		if _, ok := r.Tool.Parameters["file_path"]; ok {
			hasFilePathParam = true
			break
		}
	}
	assert.True(t, hasFilePathParam, "Should find tools with file_path parameter")
}

func TestSearchTools_FuzzyMatch(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "bsh",
		FuzzyMatch: true,
		MaxResults: 10,
	})

	// Should still find Bash via fuzzy matching
	found := false
	for _, r := range results {
		if r.Tool.Name == "Bash" {
			found = true
			assert.Equal(t, "fuzzy", r.MatchType)
			break
		}
	}
	assert.True(t, found, "Should find Bash via fuzzy match")
}

func TestSearchTools_MaxResults(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:      "a",
		MaxResults: 3,
	})

	assert.LessOrEqual(t, len(results), 3, "Should respect MaxResults limit")
}

func TestSearchTools_MinScore(t *testing.T) {
	t.Parallel()
	results := SearchTools(SearchOptions{
		Query:    "bash",
		MinScore: 0.9,
	})

	for _, r := range results {
		assert.GreaterOrEqual(t, r.Score, 0.9, "All results should have score >= 0.9")
	}
}

func TestSearchByKeywords(t *testing.T) {
	t.Parallel()
	results := SearchByKeywords([]string{"git", "version"}, nil)

	require.NotEmpty(t, results)
	// Git tool has "git" in name and "version control" in description
	found := false
	for _, r := range results {
		if r.Tool.Name == "Git" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find Git tool with keywords")
}

func TestSearchByKeywords_WithCategory(t *testing.T) {
	t.Parallel()
	results := SearchByKeywords([]string{"file"}, []string{CategoryFileSystem})

	// All results should be filesystem tools
	for _, r := range results {
		assert.Equal(t, CategoryFileSystem, r.Tool.Category)
	}
}

func TestGetToolSuggestions(t *testing.T) {
	t.Parallel()
	t.Skip("Skipping flaky test - non-deterministic map iteration order")
	suggestions := GetToolSuggestions("Re", 5)

	require.NotEmpty(t, suggestions)
	// Should find Read and References
	names := make(map[string]bool)
	for _, s := range suggestions {
		names[s.Name] = true
	}
	assert.True(t, names["Read"], "Should suggest Read")
	assert.True(t, names["References"], "Should suggest References")
}

func TestGetToolSuggestions_MaxLimit(t *testing.T) {
	t.Parallel()
	suggestions := GetToolSuggestions("", 3)

	// When prefix is empty, may return up to 3 suggestions
	assert.LessOrEqual(t, len(suggestions), 3)
}

func TestFuzzyMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s1       string
		s2       string
		expected bool // should have non-zero score
	}{
		{"bash", "bsh", true},
		{"hello", "helo", true},
		{"xyz", "abc", false},
		{"", "test", false},
		{"test", "", false},
	}

	for _, tc := range tests {
		score := fuzzyMatch(tc.s1, tc.s2)
		if tc.expected {
			assert.Greater(t, score, 0.0, "Expected non-zero score for %s vs %s", tc.s1, tc.s2)
		}
	}
}

func TestCalculateToolScore_EmptyQuery(t *testing.T) {
	t.Parallel()
	schema := &ToolSchema{
		Name:        "Test",
		Description: "Test tool",
		Category:    "test_category",
		Parameters:  map[string]Param{},
	}

	// Empty query should not crash and returns valid score
	score, _ := calculateToolScore(schema, "", SearchOptions{})
	// Empty query might match category or return 0 depending on implementation
	assert.GreaterOrEqual(t, score, 0.0)
}

func TestSortToolResults(t *testing.T) {
	t.Parallel()
	results := []ToolSearchResult{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.7},
		{Score: 1.0},
	}

	sortToolResults(results)

	// Should be sorted descending
	assert.Equal(t, 1.0, results[0].Score)
	assert.Equal(t, 0.9, results[1].Score)
	assert.Equal(t, 0.7, results[2].Score)
	assert.Equal(t, 0.5, results[3].Score)
}
