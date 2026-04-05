package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapterRegistry_Search_ExactNameMatch(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()
	InitializeDefaultRegistry()

	results := registry.Search(AdapterSearchOptions{
		Query:      "postgresql",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	assert.Equal(t, "postgresql", results[0].Adapter.Name)
	assert.Equal(t, 1.0, results[0].Score)
	assert.Equal(t, "name", results[0].MatchType)
}

func TestAdapterRegistry_Search_PartialNameMatch(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		Query:      "post",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	// Should find postgresql
	found := false
	for _, r := range results {
		if r.Adapter.Name == "postgresql" {
			found = true
			// Match type can be name or description depending on scoring
			assert.Contains(t, []string{"name", "description"}, r.MatchType)
			break
		}
	}
	assert.True(t, found, "Should find postgresql with partial match")
}

func TestAdapterRegistry_Search_DescriptionMatch(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		Query:      "database operations",
		MaxResults: 10,
	})

	require.NotEmpty(t, results)
	// Should find database-related adapters
	for _, r := range results {
		if r.MatchType == "description" {
			assert.Contains(t, r.Adapter.Description, "database",
				"Description match should contain 'database'")
		}
	}
}

func TestAdapterRegistry_Search_CategoryFilter(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		Query:      "",
		Categories: []AdapterCategory{CategoryDatabase},
		MaxResults: 50,
	})

	require.NotEmpty(t, results)
	// All results should be in database category
	for _, r := range results {
		assert.Equal(t, CategoryDatabase, r.Adapter.Category,
			"All results should be in database category")
	}
}

func TestAdapterRegistry_Search_AuthTypeFilter(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		AuthTypes:  []string{"api_key"},
		MaxResults: 50,
	})

	require.NotEmpty(t, results)
	// All results should have api_key auth type
	for _, r := range results {
		assert.Equal(t, "api_key", r.Adapter.AuthType,
			"All results should have api_key auth type")
	}
}

func TestAdapterRegistry_Search_OfficialFilter(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	official := true
	results := registry.Search(AdapterSearchOptions{
		Official:   &official,
		MaxResults: 50,
	})

	require.NotEmpty(t, results)
	// All results should be official
	for _, r := range results {
		assert.True(t, r.Adapter.Official, "All results should be official")
	}
}

func TestAdapterRegistry_Search_SupportedFilter(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	supported := true
	results := registry.Search(AdapterSearchOptions{
		Supported:  &supported,
		MaxResults: 50,
	})

	require.NotEmpty(t, results)
	// All results should be supported
	for _, r := range results {
		assert.True(t, r.Adapter.Supported, "All results should be supported")
	}
}

func TestAdapterRegistry_Search_CombinedFilters(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	official := true
	results := registry.Search(AdapterSearchOptions{
		Categories: []AdapterCategory{CategorySearch},
		Official:   &official,
		MaxResults: 50,
	})

	// All results should match all filters
	for _, r := range results {
		assert.Equal(t, CategorySearch, r.Adapter.Category)
		assert.True(t, r.Adapter.Official)
	}
}

func TestAdapterRegistry_Search_MaxResults(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		MaxResults: 3,
	})

	assert.LessOrEqual(t, len(results), 3, "Should respect MaxResults limit")
}

func TestAdapterRegistry_Search_MinScore(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.Search(AdapterSearchOptions{
		Query:    "post",
		MinScore: 0.5,
	})

	for _, r := range results {
		assert.GreaterOrEqual(t, r.Score, 0.5, "All results should have score >= 0.5")
	}
}

func TestAdapterRegistry_SearchByCapability(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	results := registry.SearchByCapability("database")

	require.NotEmpty(t, results)
	// Should find database-related adapters
	found := false
	for _, r := range results {
		if r.Category == CategoryDatabase {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find database adapters")
}

func TestAdapterRegistry_GetAdapterSuggestions(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	suggestions := registry.GetAdapterSuggestions("git", 5)

	require.NotEmpty(t, suggestions)
	// Should find github, gitlab
	names := make(map[string]bool)
	for _, s := range suggestions {
		names[s.Name] = true
	}
	assert.True(t, names["github"], "Should suggest github")
	assert.True(t, names["gitlab"], "Should suggest gitlab")
}

func TestAdapterRegistry_GetAdapterSuggestions_MaxLimit(t *testing.T) {
	t.Parallel()
	registry := NewAdapterRegistry()

	suggestions := registry.GetAdapterSuggestions("", 3)

	assert.LessOrEqual(t, len(suggestions), 3, "Should respect max limit")
}

func TestGetAllCategories(t *testing.T) {
	t.Parallel()
	categories := GetAllCategories()

	require.NotEmpty(t, categories)
	// Should include expected categories
	catSet := make(map[AdapterCategory]bool)
	for _, c := range categories {
		catSet[c] = true
	}
	assert.True(t, catSet[CategoryDatabase])
	assert.True(t, catSet[CategoryStorage])
	assert.True(t, catSet[CategoryVersionControl])
	assert.True(t, catSet[CategoryProductivity])
}

func TestGetAllAuthTypes(t *testing.T) {
	t.Parallel()
	authTypes := GetAllAuthTypes()

	require.NotEmpty(t, authTypes)
	// Should include common auth types
	typeSet := make(map[string]bool)
	for _, at := range authTypes {
		typeSet[at] = true
	}
	assert.True(t, typeSet["api_key"] || typeSet["token"] || typeSet["oauth2"],
		"Should include common auth types")
}

func TestSearch_GetSupportedAdapters(t *testing.T) {
	t.Parallel()
	supported := GetSupportedAdapters()

	require.NotEmpty(t, supported)
	// All should be supported
	for _, a := range supported {
		assert.True(t, a.Supported, "All adapters should be supported")
	}
}

func TestSearch_GetOfficialAdapters(t *testing.T) {
	t.Parallel()
	official := GetOfficialAdapters()

	require.NotEmpty(t, official)
	// All should be official
	for _, a := range official {
		assert.True(t, a.Official, "All adapters should be official")
	}
}

func TestSortAdapterResults(t *testing.T) {
	t.Parallel()
	results := []AdapterSearchResult{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.7},
		{Score: 1.0},
	}

	sortAdapterResults(results)

	// Should be sorted descending
	assert.Equal(t, 1.0, results[0].Score)
	assert.Equal(t, 0.9, results[1].Score)
	assert.Equal(t, 0.7, results[2].Score)
	assert.Equal(t, 0.5, results[3].Score)
}

func TestCalculateAdapterScore_EmptyQuery(t *testing.T) {
	t.Parallel()
	meta := AdapterMetadata{
		Name:        "test",
		Description: "Test adapter",
	}

	score, matchType := calculateAdapterScore(meta, "")
	assert.Equal(t, 1.0, score)
	assert.Equal(t, "all", matchType)
}

func TestMatchesAdapterFilters_NoFilters(t *testing.T) {
	t.Parallel()
	meta := AdapterMetadata{
		Name:      "test",
		Category:  CategoryDatabase,
		AuthType:  "api_key",
		Official:  true,
		Supported: true,
	}

	opts := AdapterSearchOptions{}
	assert.True(t, matchesAdapterFilters(meta, opts))
}

func TestMatchesAdapterFilters_CategoryMismatch(t *testing.T) {
	t.Parallel()
	meta := AdapterMetadata{
		Name:     "test",
		Category: CategoryDatabase,
	}

	opts := AdapterSearchOptions{
		Categories: []AdapterCategory{CategoryStorage},
	}
	assert.False(t, matchesAdapterFilters(meta, opts))
}

func TestMatchesAdapterFilters_AuthTypeMismatch(t *testing.T) {
	t.Parallel()
	meta := AdapterMetadata{
		Name:     "test",
		AuthType: "api_key",
	}

	opts := AdapterSearchOptions{
		AuthTypes: []string{"oauth2"},
	}
	assert.False(t, matchesAdapterFilters(meta, opts))
}

func TestAdapterRegistry_ListByCategory(t *testing.T) {
	t.Parallel()
	// Use DefaultRegistry which has all adapters initialized
	names := DefaultRegistry.ListByCategory(CategoryDatabase)

	require.NotEmpty(t, names)
	// Should find database adapters in metadata
	for _, name := range names {
		meta, ok := DefaultRegistry.GetMetadata(name)
		require.True(t, ok)
		assert.Equal(t, CategoryDatabase, meta.Category)
	}
}
