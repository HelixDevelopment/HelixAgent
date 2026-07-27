//go:build ignore

// Package main demonstrates RAG (Retrieval-Augmented Generation)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"dev.helix.agent/internal/rag"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fmt.Println("=== HelixAgent RAG Example ===")
	fmt.Println()

	// Create RAG pipeline
	pipeline := rag.NewPipeline(&rag.Config{
		EmbeddingProvider: "openai",
		EmbeddingModel:    "text-embedding-3-small",
		VectorDB:          "chromadb",
		TopK:              5,
		Logger:            logger,
	})

	ctx := context.Background()

	// Example 1: Add documents to knowledge base
	fmt.Println("--- Document Ingestion ---")

	documents := []rag.Document{
		{
			ID:      "doc1",
			Content: "HelixAgent is a multi-provider LLM orchestration platform that supports 20+ providers including OpenAI, Anthropic, DeepSeek, and Groq.",
			Metadata: map[string]string{
				"source":   "documentation",
				"category": "overview",
			},
		},
		{
			ID:      "doc2",
			Content: "The DebateOrchestrator module enables AI debates between multiple models to find consensus on complex questions.",
			Metadata: map[string]string{
				"source":   "documentation",
				"category": "modules",
			},
		},
		{
			ID:      "doc3",
			Content: "MCP (Model Context Protocol) adapters allow HelixAgent to connect to external services like databases, file systems, and APIs.",
			Metadata: map[string]string{
				"source":   "documentation",
				"category": "protocols",
			},
		},
		{
			ID:      "doc4",
			Content: "The ensemble system aggregates responses from multiple LLMs to provide more accurate and reliable outputs.",
			Metadata: map[string]string{
				"source":   "documentation",
				"category": "features",
			},
		},
		{
			ID:      "doc5",
			Content: "Tool calling allows LLMs to invoke external functions, enabling them to perform actions like calculations, API calls, and database queries.",
			Metadata: map[string]string{
				"source":   "documentation",
				"category": "features",
			},
		},
	}

	start := time.Now()
	for _, doc := range documents {
		if err := pipeline.AddDocument(ctx, doc); err != nil {
			fmt.Printf("Error adding document %s: %v\n", doc.ID, err)
		}
	}
	fmt.Printf("Added %d documents in %v\n", len(documents), time.Since(start))
	fmt.Println()

	// Example 2: Basic RAG query
	fmt.Println("--- Basic RAG Query ---")

	queries := []string{
		"What is HelixAgent?",
		"How does the debate system work?",
		"What are MCP adapters?",
		"How can LLMs invoke external functions?",
	}

	for _, query := range queries {
		fmt.Printf("Query: %s\n", query)

		start := time.Now()
		result, err := pipeline.Query(ctx, query, &rag.QueryOptions{
			TopK:         3,
			MinScore:     0.7,
			IncludeText:  true,
			GenerateText: true,
		})
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		fmt.Printf("  Retrieved %d chunks in %v\n", len(result.Chunks), time.Since(start))
		for i, chunk := range result.Chunks {
			fmt.Printf("  %d. [score: %.2f] %s...\n", i+1, chunk.Score, chunk.Text[:min(80, len(chunk.Text))])
		}
		if result.GeneratedText != "" {
			fmt.Printf("  Generated: %s\n", result.GeneratedText)
		}
		fmt.Println()
	}

	// Example 3: RAG with filtering
	fmt.Println("--- Filtered RAG Query ---")

	query := "Tell me about HelixAgent features"
	fmt.Printf("Query: %s (filtered to category='features')\n", query)

	result, err := pipeline.Query(ctx, query, &rag.QueryOptions{
		TopK:         5,
		MinScore:     0.5,
		IncludeText:  true,
		GenerateText: true,
		Filters: map[string]string{
			"category": "features",
		},
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Retrieved %d chunks (filtered)\n", len(result.Chunks))
		for _, chunk := range result.Chunks {
			fmt.Printf("  - %s...\n", chunk.Text[:min(60, len(chunk.Text))])
		}
	}
	fmt.Println()

	// Example 4: Streaming RAG
	fmt.Println("--- Streaming RAG ---")

	query = "Explain how HelixAgent works"
	fmt.Printf("Query: %s (streaming)\n", query)

	stream, err := pipeline.QueryStream(ctx, query, &rag.QueryOptions{
		TopK:         3,
		GenerateText: true,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Print("  Response: ")
		for chunk := range stream {
			if chunk.Error != nil {
				fmt.Printf("\n  Error: %v\n", chunk.Error)
				break
			}
			fmt.Print(chunk.Text)
		}
		fmt.Println()
	}
	fmt.Println()

	// Example 5: Batch document processing
	fmt.Println("--- Batch Processing ---")

	batchDocs := []rag.Document{
		{ID: "batch1", Content: "RAG improves LLM accuracy by grounding responses in retrieved documents."},
		{ID: "batch2", Content: "Vector databases store embeddings for efficient similarity search."},
		{ID: "batch3", Content: "Embeddings convert text into dense vector representations."},
	}

	start = time.Now()
	if err := pipeline.AddDocumentsBatch(ctx, batchDocs); err != nil {
		fmt.Printf("Batch error: %v\n", err)
	} else {
		fmt.Printf("Processed %d documents in batch in %v\n", len(batchDocs), time.Since(start))
	}

	// Example 6: Knowledge base statistics
	fmt.Println("\n--- Knowledge Base Stats ---")
	stats := pipeline.GetStats()
	fmt.Printf("Total documents: %d\n", stats.TotalDocuments)
	fmt.Printf("Total chunks: %d\n", stats.TotalChunks)
	fmt.Printf("Vector dimension: %d\n", stats.VectorDimension)
	fmt.Printf("Index size: %s\n", stats.IndexSize)

	fmt.Println("\nRAG example complete!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
