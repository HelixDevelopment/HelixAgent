//go:build ignore

// Package main demonstrates web search capabilities
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"dev.helix.agent/internal/search"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fmt.Println("=== HelixAgent Web Search Example ===")
	fmt.Println()

	// Check for search provider API keys
	tavilyKey := os.Getenv("TAVILY_API_KEY")
	perplexityKey := os.Getenv("PERPLEXITY_API_KEY")
	exaKey := os.Getenv("EXA_API_KEY")

	hasSearchProvider := tavilyKey != "" || perplexityKey != "" || exaKey != ""
	if !hasSearchProvider {
		fmt.Println("No search provider configured!")
		fmt.Println("Please set one of:")
		fmt.Println("  - TAVILY_API_KEY (recommended)")
		fmt.Println("  - PERPLEXITY_API_KEY")
		fmt.Println("  - EXA_API_KEY")
		return
	}

	// Create search manager
	manager := search.NewManager(&search.Config{
		TavilyAPIKey:     tavilyKey,
		PerplexityAPIKey: perplexityKey,
		ExaAPIKey:        exaKey,
		Timeout:          30 * time.Second,
		Logger:           logger,
	})

	ctx := context.Background()

	queries := []string{
		"latest developments in AI 2025",
		"Go 1.25 new features",
		"best practices for microservices",
	}

	// Example 1: Basic search
	fmt.Println("--- Basic Search ---")
	for _, query := range queries {
		fmt.Printf("Query: %s\n", query)

		start := time.Now()
		results, err := manager.Search(ctx, query, &search.Options{
			NumResults:  5,
			IncludeText: true,
		})
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		latency := time.Since(start)
		fmt.Printf("  Found %d results in %v\n", len(results), latency)
		for i, r := range results {
			if i >= 3 {
				fmt.Printf("  ... and %d more\n", len(results)-3)
				break
			}
			fmt.Printf("  %d. %s\n     %s\n", i+1, r.Title, r.URL)
		}
		fmt.Println()
	}

	// Example 2: AI-enhanced search (if Perplexity available)
	if perplexityKey != "" {
		fmt.Println("--- AI-Enhanced Search (Perplexity) ---")
		query := "What are the tradeoffs between REST and GraphQL in 2025?"
		fmt.Printf("Query: %s\n", query)

		start := time.Now()
		answer, err := manager.SearchWithAI(ctx, query, &search.AIOptions{
			NumResults:     5,
			IncludeSources: true,
			Model:          "sonar",
			MaxTokens:      500,
		})
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("Answer (in %v):\n%s\n", time.Since(start), answer.Content)
			if len(answer.Sources) > 0 {
				fmt.Println("\nSources:")
				for i, s := range answer.Sources[:min(3, len(answer.Sources))] {
					fmt.Printf("  %d. %s\n", i+1, s)
				}
			}
		}
		fmt.Println()
	}

	// Example 3: Code search (if Exa available)
	if exaKey != "" {
		fmt.Println("--- Neural Code Search (Exa) ---")
		query := "Go error handling best practices with examples"
		fmt.Printf("Query: %s\n", query)

		start := time.Now()
		results, err := manager.SearchCode(ctx, query, &search.CodeOptions{
			NumResults: 5,
			Language:   "go",
		})
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("Found %d code snippets in %v\n", len(results), time.Since(start))
			for i, r := range results[:min(3, len(results))] {
				fmt.Printf("\n  %d. %s\n", i+1, r.URL)
				if len(r.Code) > 200 {
					fmt.Printf("     %s...\n", r.Code[:200])
				} else {
					fmt.Printf("     %s\n", r.Code)
				}
			}
		}
		fmt.Println()
	}

	// Example 4: Multi-provider search with aggregation
	fmt.Println("--- Multi-Provider Aggregated Search ---")
	query := "machine learning deployment patterns"
	fmt.Printf("Query: %s\n", query)

	start := time.Now()
	aggResults, err := manager.SearchAggregated(ctx, query, &search.AggregatedOptions{
		Providers:   []string{"tavily", "perplexity"},
		NumResults:  10,
		Deduplicate: true,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("Aggregated %d unique results from %d providers in %v\n",
			len(aggResults.Results), len(aggResults.Providers), time.Since(start))
		for i, r := range aggResults.Results[:min(5, len(aggResults.Results))] {
			fmt.Printf("  %d. [%s] %s\n", i+1, r.Source, r.Title)
		}
		if len(aggResults.Results) > 5 {
			fmt.Printf("  ... and %d more\n", len(aggResults.Results)-5)
		}
	}

	fmt.Println("\nSearch complete!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
