// RAG Bridge Service for HelixAgent
// Provides unified API for document processing, embedding, reranking, and generation
// Supports both NVIDIA Nemotron (paid/free tier) and open-source alternatives
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Round-29 §11.4 anti-bluff sentinels (forensic anchors).
//
// Both errors replace silent contract-bluffs that previously made the
// rag-bridge appear functional while skipping the work the caller had
// asked for. See close-out commit message for the full forensic
// anchor (Article XI §11.9 / CONST-035 / CONST-050(A)).
var (
	// ErrMilvusRetrievalNotWired is returned by retrieveDocuments when
	// the Milvus vector-DB client has not been wired in. Previously
	// the function returned []Source{} unconditionally with the
	// comment "Simplified retrieval - in production this would query
	// Milvus" — the LLM downstream produced answers grounded in NO
	// context whatsoever, and the caller could not tell "no documents
	// matched the query" from "we never asked Milvus in the first
	// place". CRITICAL §11.4 contract-bluff. Wire pkg/milvus client
	// before re-enabling RAG retrieval.
	ErrMilvusRetrievalNotWired = errors.New("rag-bridge: Milvus retrieval has not been wired — retrieveDocuments previously returned an empty []Source{} regardless of query, producing LLM answers grounded in NO context (§11.4 CRITICAL contract-bluff: caller cannot tell 'no documents matched' from 'we never asked Milvus'). Wire pkg/milvus client before re-enabling RAG retrieval")

	// ErrRerankerNotWired is returned by rerankSources when the
	// reranker client has not been wired in but the caller asked for
	// reranking. Previously the function returned the input sources
	// unchanged while pretending rerank had occurred — the response
	// scoring was identical to the pre-rerank input but presented as
	// reranked output. CRITICAL §11.4 contract-bluff. Wire
	// pkg/reranker client before invoking rerankSources, or call
	// with reranker explicitly disabled in the request.
	ErrRerankerNotWired = errors.New("rag-bridge: reranker has not been wired — rerankSources previously returned input sources unchanged while pretending rerank had occurred (§11.4 CONTRACT-bluff). Wire pkg/reranker client before invoking rerankSources, or call with reranker explicitly disabled in the request")
)

// Config holds service configuration
type Config struct {
	// Vector Database
	MilvusHost string
	MilvusPort int

	// Extraction Service
	ExtractionURL  string
	ExtractionType string // "nemo", "tika"

	// Embedding Service
	EmbeddingURL       string
	EmbeddingType      string // "nemotron", "sentence-transformers", "nvidia-nim"
	EmbeddingDimension int

	// Reranking Service
	RerankerURL  string
	RerankerType string // "nemotron", "none"

	// Generation Service
	GenerationURL   string
	GenerationType  string // "nemotron", "ollama", "nvidia-nim"
	GenerationModel string

	// NVIDIA NIM Cloud (optional paid tier)
	NVIDIAAPIKey  string
	NVIDIABaseURL string

	// Processing Options
	ChunkSize     int
	ChunkOverlap  int
	ExtractTables bool
	ExtractCharts bool
}

// LoadConfig loads configuration from environment
func LoadConfig() *Config {
	return &Config{
		MilvusHost:         getEnv("MILVUS_HOST", "milvus"),
		MilvusPort:         getEnvInt("MILVUS_PORT", 19530),
		ExtractionURL:      getEnv("EXTRACTION_URL", "http://tika-extraction:9998"),
		ExtractionType:     getEnv("EXTRACTION_TYPE", "tika"),
		EmbeddingURL:       getEnv("EMBEDDING_URL", "http://sentence-embedding:80"),
		EmbeddingType:      getEnv("EMBEDDING_TYPE", "sentence-transformers"),
		EmbeddingDimension: getEnvInt("EMBEDDING_DIMENSION", 384),
		RerankerURL:        getEnv("RERANKER_URL", ""),
		RerankerType:       getEnv("RERANKER_TYPE", "none"),
		GenerationURL:      getEnv("GENERATION_URL", "http://ollama-generation:11434"),
		GenerationType:     getEnv("GENERATION_TYPE", "ollama"),
		GenerationModel:    getEnv("GENERATION_MODEL", "llama3.2"),
		NVIDIAAPIKey:       getEnv("NVIDIA_API_KEY", ""),
		NVIDIABaseURL:      getEnv("NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1"),
		ChunkSize:          getEnvInt("CHUNK_SIZE", 512),
		ChunkOverlap:       getEnvInt("CHUNK_OVERLAP", 100),
		ExtractTables:      getEnvBool("EXTRACT_TABLES", true),
		ExtractCharts:      getEnvBool("EXTRACT_CHARTS", false),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

// Server handles HTTP requests
type Server struct {
	config *Config
	client *http.Client
}

// NewServer creates a new server
func NewServer(config *Config) *Server {
	return &Server{
		config: config,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// DocumentRequest represents a document processing request
type DocumentRequest struct {
	DocumentID    string                 `json:"document_id,omitempty"`
	Content       string                 `json:"content,omitempty"`
	Source        string                 `json:"source,omitempty"`
	ExtractTables *bool                  `json:"extract_tables,omitempty"`
	ExtractCharts *bool                  `json:"extract_charts,omitempty"`
	TableFormat   string                 `json:"table_format,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentResult represents document processing result
type DocumentResult struct {
	DocumentID  string    `json:"document_id"`
	Chunks      []Chunk   `json:"chunks"`
	Tables      []Table   `json:"tables,omitempty"`
	Charts      []Chart   `json:"charts,omitempty"`
	TotalPages  int       `json:"total_pages,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
}

// Chunk represents a document chunk
type Chunk struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Page     int    `json:"page,omitempty"`
	Section  string `json:"section,omitempty"`
	Position int    `json:"position"`
}

// Table represents an extracted table
type Table struct {
	ID      string   `json:"id"`
	Page    int      `json:"page"`
	Content string   `json:"content"` // Markdown format
	Headers []string `json:"headers,omitempty"`
}

// Chart represents an extracted chart
type Chart struct {
	ID       string `json:"id"`
	Page     int    `json:"page"`
	Type     string `json:"type"`
	ImageURL string `json:"image_url,omitempty"`
}

// RAGQueryRequest represents a RAG query
type RAGQueryRequest struct {
	Query            string                 `json:"query"`
	Collection       string                 `json:"collection,omitempty"`
	TopK             int                    `json:"top_k,omitempty"`
	RequireCitations bool                   `json:"require_citations,omitempty"`
	Filters          map[string]interface{} `json:"filters,omitempty"`
}

// RAGQueryResult represents RAG query result.
//
// Warnings surfaces every degraded-path the bridge took during this
// query (e.g. "retrieval unavailable: ErrMilvusRetrievalNotWired",
// "reranking unavailable: ErrRerankerNotWired") so the caller can
// disclose to the end user that the answer is partially or fully
// uncontextualised instead of trusting the Confidence field blindly.
// Added round-29 §11.4 anti-bluff fix.
type RAGQueryResult struct {
	Answer     string     `json:"answer"`
	Citations  []Citation `json:"citations,omitempty"`
	Sources    []Source   `json:"sources"`
	Confidence float64    `json:"confidence"`
	QueryTime  int64      `json:"query_time_ms"`
	Warnings   []string   `json:"warnings,omitempty"`
}

// Citation represents a source citation
type Citation struct {
	Text     string `json:"text"`
	SourceID string `json:"source_id"`
	Page     int    `json:"page,omitempty"`
	Section  string `json:"section,omitempty"`
}

// Source represents a source document
type Source struct {
	ID       string  `json:"id"`
	Document string  `json:"document"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	Page     int     `json:"page,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
	Config    map[string]string `json:"config"`
	Timestamp time.Time         `json:"timestamp"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Services:  make(map[string]string),
		Config:    make(map[string]string),
		Timestamp: time.Now(),
	}

	// Check each service
	services := map[string]string{
		"extraction": s.config.ExtractionURL,
		"embedding":  s.config.EmbeddingURL,
		"generation": s.config.GenerationURL,
	}

	if s.config.RerankerURL != "" {
		services["reranker"] = s.config.RerankerURL
	}

	for name, url := range services {
		if err := s.checkService(ctx, url); err != nil {
			response.Services[name] = "unhealthy: " + err.Error()
			response.Status = "degraded"
		} else {
			response.Services[name] = "healthy"
		}
	}

	// Add configuration info (without sensitive data)
	response.Config["extraction_type"] = s.config.ExtractionType
	response.Config["embedding_type"] = s.config.EmbeddingType
	response.Config["embedding_dimension"] = fmt.Sprintf("%d", s.config.EmbeddingDimension)
	response.Config["generation_type"] = s.config.GenerationType
	response.Config["generation_model"] = s.config.GenerationModel
	response.Config["reranker_type"] = s.config.RerankerType
	response.Config["nvidia_nim_available"] = fmt.Sprintf("%v", s.config.NVIDIAAPIKey != "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) checkService(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		// Try alternative endpoints
		altEndpoints := []string{"/v1/health", "/api/tags", "/version"}
		for _, endpoint := range altEndpoints {
			req, _ = http.NewRequestWithContext(ctx, "GET", url+endpoint, nil)
			if resp, err = s.client.Do(req); err == nil {
				resp.Body.Close()
				return nil
			}
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) handleProcessDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	var req DocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.DocumentID == "" {
		req.DocumentID = uuid.New().String()
	}

	// Use config defaults if not specified
	extractTables := s.config.ExtractTables
	if req.ExtractTables != nil {
		extractTables = *req.ExtractTables
	}
	extractCharts := s.config.ExtractCharts
	if req.ExtractCharts != nil {
		extractCharts = *req.ExtractCharts
	}

	// Process based on extraction type
	var result *DocumentResult
	var err error

	switch s.config.ExtractionType {
	case "nemo":
		result, err = s.processWithNemo(ctx, &req, extractTables, extractCharts)
	default:
		result, err = s.processWithTika(ctx, &req, extractTables)
	}

	if err != nil {
		log.Printf("Document processing failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result.DocumentID = req.DocumentID
	result.ProcessedAt = time.Now()

	log.Printf("Processed document %s in %v", req.DocumentID, time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) processWithNemo(ctx context.Context, req *DocumentRequest, extractTables, extractCharts bool) (*DocumentResult, error) {
	// NVIDIA NeMo Retriever processing
	nemoReq := map[string]interface{}{
		"source":         req.Source,
		"content":        req.Content,
		"extract_tables": extractTables,
		"extract_charts": extractCharts,
		"table_format":   req.TableFormat,
	}

	jsonData, _ := json.Marshal(nemoReq)
	nemoURL := s.config.ExtractionURL + "/v1/extract"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", nemoURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nemo extraction failed: %s", string(body))
	}

	var nemoResp struct {
		Text   string `json:"text"`
		Pages  int    `json:"pages"`
		Tables []struct {
			Page    int      `json:"page"`
			Content string   `json:"content"`
			Headers []string `json:"headers"`
		} `json:"tables"`
		Charts []struct {
			Page int    `json:"page"`
			Type string `json:"type"`
		} `json:"charts"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nemoResp); err != nil {
		return nil, err
	}

	// Convert to chunks
	chunks := s.chunkText(nemoResp.Text, req.DocumentID)

	result := &DocumentResult{
		Chunks:     chunks,
		TotalPages: nemoResp.Pages,
	}

	for i, t := range nemoResp.Tables {
		result.Tables = append(result.Tables, Table{
			ID:      fmt.Sprintf("%s-table-%d", req.DocumentID, i),
			Page:    t.Page,
			Content: t.Content,
			Headers: t.Headers,
		})
	}

	for i, c := range nemoResp.Charts {
		result.Charts = append(result.Charts, Chart{
			ID:   fmt.Sprintf("%s-chart-%d", req.DocumentID, i),
			Page: c.Page,
			Type: c.Type,
		})
	}

	return result, nil
}

func (s *Server) processWithTika(ctx context.Context, req *DocumentRequest, extractTables bool) (*DocumentResult, error) {
	// Apache Tika processing (open source)
	tikaURL := s.config.ExtractionURL + "/tika"

	var contentReader io.Reader
	if req.Content != "" {
		contentReader = strings.NewReader(req.Content)
	} else if req.Source != "" {
		// Fetch from source URL
		resp, err := s.client.Get(req.Source)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		contentReader = resp.Body
	}

	tikaReq, err := http.NewRequestWithContext(ctx, "PUT", tikaURL, contentReader)
	if err != nil {
		return nil, err
	}
	tikaReq.Header.Set("Accept", "text/plain")

	resp, err := s.client.Do(tikaReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	text := string(body)
	chunks := s.chunkText(text, req.DocumentID)

	result := &DocumentResult{
		Chunks: chunks,
	}

	// Try to extract tables if requested
	if extractTables {
		// Use Tika's recursive JSON content handler
		tables, err := s.extractTablesWithTika(ctx, req)
		if err == nil {
			result.Tables = tables
		}
	}

	return result, nil
}

func (s *Server) extractTablesWithTika(ctx context.Context, req *DocumentRequest) ([]Table, error) {
	tikaURL := s.config.ExtractionURL + "/rmeta/text"

	var contentReader io.Reader
	if req.Content != "" {
		contentReader = strings.NewReader(req.Content)
	} else if req.Source != "" {
		resp, err := s.client.Get(req.Source)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		contentReader = resp.Body
	}

	tikaReq, err := http.NewRequestWithContext(ctx, "PUT", tikaURL, contentReader)
	if err != nil {
		return nil, err
	}
	tikaReq.Header.Set("Accept", "application/json")
	tikaReq.Header.Set("X-Tika-Recursive-JSON", "true")

	resp, err := s.client.Do(tikaReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse Tika output to extract tables
	// This is simplified - real implementation would parse the full structure
	return []Table{}, nil
}

func (s *Server) chunkText(text, docID string) []Chunk {
	// Simple sentence-based chunking
	words := strings.Fields(text)
	var chunks []Chunk

	chunkSize := s.config.ChunkSize
	overlap := s.config.ChunkOverlap

	for i := 0; i < len(words); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")

		chunks = append(chunks, Chunk{
			ID:       fmt.Sprintf("%s-chunk-%d", docID, len(chunks)),
			Text:     chunkText,
			Position: len(chunks),
		})

		if end >= len(words) {
			break
		}
	}

	return chunks
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	var req RAGQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.TopK == 0 {
		req.TopK = 5
	}

	// Step 1: Generate embedding for query
	queryEmbedding, err := s.generateEmbedding(ctx, req.Query)
	if err != nil {
		log.Printf("Embedding generation failed: %v", err)
		http.Error(w, "embedding failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// warnings collects degraded-path messages so the caller (and end
	// user) can disclose that the answer is partially or fully
	// uncontextualised — round-29 §11.4 anti-bluff fix.
	var warnings []string

	// Step 2: Retrieve similar documents
	sources, err := s.retrieveDocuments(ctx, queryEmbedding, req.Collection, req.TopK*2)
	if err != nil {
		log.Printf("Document retrieval failed: %v", err)
		// Continue with empty sources but record the degraded-path
		// warning so the caller is NOT misled into trusting an
		// uncontextualised answer (round-29 anti-bluff fix —
		// retrieveDocuments now surfaces ErrMilvusRetrievalNotWired
		// when the Milvus client is missing).
		if errors.Is(err, ErrMilvusRetrievalNotWired) {
			warnings = append(warnings, "retrieval unavailable (Milvus client not wired) — answer is uncontextualised")
		} else {
			warnings = append(warnings, fmt.Sprintf("retrieval failed: %v — answer is uncontextualised", err))
		}
		sources = []Source{}
	}

	// Step 3: Rerank if reranker is available
	if s.config.RerankerURL != "" && s.config.RerankerType != "none" {
		reranked, err := s.rerankSources(ctx, req.Query, sources, req.TopK)
		if err != nil {
			log.Printf("Reranking failed: %v", err)
			// Record degraded-path warning rather than silently
			// pretending reranking succeeded (round-29 anti-bluff
			// fix — rerankSources now surfaces ErrRerankerNotWired
			// when the reranker client is missing).
			if errors.Is(err, ErrRerankerNotWired) {
				warnings = append(warnings, "reranking unavailable (reranker client not wired) — source ranking is pre-rerank")
			} else {
				warnings = append(warnings, fmt.Sprintf("reranking failed: %v — source ranking is pre-rerank", err))
			}
			// Trim to top-K manually since rerank produced no output.
			if len(sources) > req.TopK {
				sources = sources[:req.TopK]
			}
		} else {
			sources = reranked
		}
	} else {
		// Take top K without reranking
		if len(sources) > req.TopK {
			sources = sources[:req.TopK]
		}
	}

	// Step 4: Generate answer
	answer, citations, err := s.generateAnswer(ctx, req.Query, sources, req.RequireCitations)
	if err != nil {
		log.Printf("Answer generation failed: %v", err)
		http.Error(w, "generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate confidence based on source scores
	confidence := 0.0
	if len(sources) > 0 {
		for _, s := range sources {
			confidence += s.Score
		}
		confidence /= float64(len(sources))
	}

	result := RAGQueryResult{
		Answer:     answer,
		Citations:  citations,
		Sources:    sources,
		Confidence: confidence,
		QueryTime:  time.Since(start).Milliseconds(),
		Warnings:   warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	var url string
	var body io.Reader

	switch s.config.EmbeddingType {
	case "nemotron":
		url = s.config.EmbeddingURL + "/v1/embeddings"
		reqBody := map[string]interface{}{
			"input": text,
			"model": "nvidia/llama-nemotron-embed-vl-1b-v2",
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))

	case "nvidia-nim":
		url = s.config.NVIDIABaseURL + "/embeddings"
		reqBody := map[string]interface{}{
			"input": text,
			"model": "nvidia/llama-nemotron-embed-vl-1b-v2",
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))

	default: // sentence-transformers
		url = s.config.EmbeddingURL + "/embed"
		reqBody := map[string]interface{}{
			"inputs": text,
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if s.config.EmbeddingType == "nvidia-nim" && s.config.NVIDIAAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.NVIDIAAPIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding failed: %s", string(respBody))
	}

	// Parse response based on embedding type
	if s.config.EmbeddingType == "sentence-transformers" {
		var result [][]float32
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		if len(result) > 0 {
			return result[0], nil
		}
		return nil, fmt.Errorf("empty embedding response")
	}

	// OpenAI-compatible format (NVIDIA NIM)
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) > 0 {
		return result.Data[0].Embedding, nil
	}

	return nil, fmt.Errorf("empty embedding response")
}

// retrieveDocuments queries the Milvus vector DB for documents whose
// embeddings are nearest to `embedding`. Round-29 §11.4 fix: the
// Milvus client wiring is not yet in place; rather than silently
// returning []Source{} (the previous CRITICAL contract-bluff —
// callers received an empty source list indistinguishable from "the
// query genuinely had no matches"), the function now surfaces
// ErrMilvusRetrievalNotWired so the caller can either degrade
// gracefully (continue with empty sources + emit "RAG unavailable")
// or fail loudly. Wire pkg/milvus client and the real similarity
// search before removing this sentinel.
func (s *Server) retrieveDocuments(ctx context.Context, embedding []float32, collection string, topK int) ([]Source, error) {
	return nil, fmt.Errorf("retrieveDocuments(collection=%q, topK=%d): %w", collection, topK, ErrMilvusRetrievalNotWired)
}

// rerankSources runs the configured reranker over the candidate
// sources and returns the top-K. Round-29 §11.4 fix: when the
// reranker URL/type indicates "none", the previous behaviour of
// returning input sources untouched IS the contract (rerank
// disabled). When the reranker IS configured (URL set, type != none)
// but the reranker client wiring is missing, the function now
// surfaces ErrRerankerNotWired instead of silently passing input
// through as if it had been reranked (CRITICAL §11.4 contract-bluff
// the previous "return sources, nil" introduced). Wire the real
// reranker client before removing this sentinel.
func (s *Server) rerankSources(ctx context.Context, query string, sources []Source, topK int) ([]Source, error) {
	if s.config.RerankerURL == "" || s.config.RerankerType == "none" {
		// Reranker explicitly disabled — pass-through is the contract.
		return sources, nil
	}
	return nil, fmt.Errorf("rerankSources(query=%q, topK=%d, rerankerType=%q): %w", query, topK, s.config.RerankerType, ErrRerankerNotWired)
}

func (s *Server) generateAnswer(ctx context.Context, query string, sources []Source, requireCitations bool) (string, []Citation, error) {
	// Build context from sources
	var contextBuilder strings.Builder
	for i, src := range sources {
		contextBuilder.WriteString(fmt.Sprintf("[Document %d]\n%s\n\n", i+1, src.Content))
	}

	prompt := fmt.Sprintf(`Based on the following documents, answer the question. %s

Documents:
%s

Question: %s

Answer:`, func() string {
		if requireCitations {
			return "Include citations to specific documents in your answer."
		}
		return ""
	}(), contextBuilder.String(), query)

	var url string
	var body io.Reader

	switch s.config.GenerationType {
	case "nemotron":
		url = s.config.GenerationURL + "/v1/chat/completions"
		reqBody := map[string]interface{}{
			"model": s.config.GenerationModel,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.3,
			"max_tokens":  4096,
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))

	case "nvidia-nim":
		url = s.config.NVIDIABaseURL + "/chat/completions"
		reqBody := map[string]interface{}{
			"model": "nvidia/llama-3.3-nemotron-super-49b-v1.5",
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.3,
			"max_tokens":  4096,
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))

	default: // ollama
		url = s.config.GenerationURL + "/api/generate"
		reqBody := map[string]interface{}{
			"model":  s.config.GenerationModel,
			"prompt": prompt,
			"stream": false,
			"options": map[string]interface{}{
				"temperature": 0.3,
				"num_predict": 4096,
			},
		}
		jsonData, _ := json.Marshal(reqBody)
		body = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if s.config.GenerationType == "nvidia-nim" && s.config.NVIDIAAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.NVIDIAAPIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("generation failed: %s", string(respBody))
	}

	var answer string

	if s.config.GenerationType == "ollama" {
		var result struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", nil, err
		}
		answer = result.Response
	} else {
		// OpenAI-compatible format
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", nil, err
		}
		if len(result.Choices) > 0 {
			answer = result.Choices[0].Message.Content
		}
	}

	// Extract citations if required
	var citations []Citation
	if requireCitations {
		citations = extractCitations(answer, sources)
	}

	return answer, citations, nil
}

func extractCitations(answer string, sources []Source) []Citation {
	// Simple citation extraction based on document references
	var citations []Citation
	// Implementation would parse answer for [Document X] references
	return citations
}

func main() {
	config := LoadConfig()
	server := NewServer(config)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/v1/process", server.handleProcessDocument)
	mux.HandleFunc("/v1/query", server.handleRAGQuery)

	port := getEnv("PORT", "8500")
	addr := ":" + port

	log.Printf("RAG Bridge starting on %s", addr)
	log.Printf("Configuration:")
	log.Printf("  Extraction: %s (%s)", config.ExtractionType, config.ExtractionURL)
	log.Printf("  Embedding: %s (%s, dim=%d)", config.EmbeddingType, config.EmbeddingURL, config.EmbeddingDimension)
	log.Printf("  Generation: %s (%s, model=%s)", config.GenerationType, config.GenerationURL, config.GenerationModel)
	log.Printf("  Reranker: %s (%s)", config.RerankerType, config.RerankerURL)
	log.Printf("  NVIDIA NIM: available=%v", config.NVIDIAAPIKey != "")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
