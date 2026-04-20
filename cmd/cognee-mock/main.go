// Cognee Mock Server - Provides local Cognee API compatibility
// This ensures Cognee is ALWAYS available for HelixAgent
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// MemoryEntry represents stored memory
type MemoryEntry struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Dataset   string                 `json:"dataset"`
	Type      string                 `json:"type"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
}

// CogneeMockServer is an in-memory Cognee implementation.
//
// Concurrency model (CONST-029): memories is a *safe.Store; appends
// use Update for read-modify-write atomicity.
type CogneeMockServer struct {
	memories *safe.Store[string, []MemoryEntry]
	port     int
}

func NewCogneeMockServer(port int) *CogneeMockServer {
	return &CogneeMockServer{
		memories: safe.NewStore[string, []MemoryEntry](),
		port:     port,
	}
}

func (s *CogneeMockServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "cognee-mock",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *CogneeMockServer) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Content     string                 `json:"content"`
		Dataset     string                 `json:"dataset"`
		ContentType string                 `json:"content_type"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dataset := req.Dataset
	if dataset == "" {
		dataset = "default"
	}

	entry := MemoryEntry{
		ID:        fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		Content:   req.Content,
		Dataset:   dataset,
		Type:      req.ContentType,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
	}

	s.memories.Update(dataset, func(cur []MemoryEntry, _ bool) ([]MemoryEntry, bool) {
		return append(cur, entry), true
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"id":     entry.ID,
	})

	log.Printf("[COGNEE] Added memory: dataset=%s, type=%s, content_len=%d", dataset, req.ContentType, len(req.Content))
}

func (s *CogneeMockServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query      string   `json:"query"`
		Dataset    string   `json:"dataset"`
		Limit      int      `json:"limit"`
		SearchType []string `json:"search_type"`
	}

	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.Query = r.URL.Query().Get("query")
		req.Dataset = r.URL.Query().Get("dataset")
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	dataset := req.Dataset
	if dataset == "" {
		dataset = "default"
	}

	memories, _ := s.memories.Get(dataset)

	// Simple keyword matching for search
	var results []map[string]interface{}
	for _, mem := range memories {
		if len(results) >= req.Limit {
			break
		}
		// Simple relevance - just return recent memories
		results = append(results, map[string]interface{}{
			"id":         mem.ID,
			"content":    mem.Content,
			"score":      0.85,
			"created_at": mem.CreatedAt,
			"metadata":   mem.Metadata,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

func (s *CogneeMockServer) handleCognify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Datasets []string `json:"datasets"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"message":  "Knowledge graph updated",
		"datasets": req.Datasets,
	})

	log.Printf("[COGNEE] Cognify triggered for datasets: %v", req.Datasets)
}

func (s *CogneeMockServer) handleGraphQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": []map[string]interface{}{},
		"edges": []map[string]interface{}{},
	})
}

func (s *CogneeMockServer) handleInsights(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"insights": []map[string]interface{}{
			{
				"type":       "summary",
				"text":       "Local Cognee mock service active",
				"confidence": 1.0,
			},
		},
	})
}

func (s *CogneeMockServer) handleDatasets(w http.ResponseWriter, r *http.Request) {
	datasets := s.memories.Keys()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"datasets": datasets,
	})
}

func (s *CogneeMockServer) Run() error {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// Memory operations
	mux.HandleFunc("/api/v1/add", s.handleAdd)
	mux.HandleFunc("/api/v1/search", s.handleSearch)

	// Knowledge graph operations
	mux.HandleFunc("/api/v1/cognify", s.handleCognify)
	mux.HandleFunc("/api/v1/graph", s.handleGraphQuery)
	mux.HandleFunc("/api/v1/insights", s.handleInsights)

	// Dataset management
	mux.HandleFunc("/api/v1/datasets", s.handleDatasets)

	// Catch-all for other endpoints
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"service": "cognee-mock",
			"path":    r.URL.Path,
		})
	})

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[COGNEE-MOCK] Starting server on %s", addr)
	log.Printf("[COGNEE-MOCK] Endpoints: /health, /api/v1/add, /api/v1/search, /api/v1/cognify, /api/v1/graph, /api/v1/insights")

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return server.ListenAndServe()
}

func main() {
	port := flag.Int("port", 8000, "Port to listen on")
	flag.Parse()

	server := NewCogneeMockServer(*port)
	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
