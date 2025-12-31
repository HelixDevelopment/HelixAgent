package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type ModelMetadataRepository struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

func NewModelMetadataRepository(pool *pgxpool.Pool, log *logrus.Logger) *ModelMetadataRepository {
	return &ModelMetadataRepository{
		pool: pool,
		log:  log,
	}
}

type ModelMetadata struct {
	ID                      string                 `json:"id"`
	ModelID                 string                 `json:"model_id"`
	ModelName               string                 `json:"model_name"`
	ProviderID              string                 `json:"provider_id"`
	ProviderName            string                 `json:"provider_name"`
	Description             string                 `json:"description"`
	ContextWindow           *int                   `json:"context_window"`
	MaxTokens               *int                   `json:"max_tokens"`
	PricingInput            *float64               `json:"pricing_input"`
	PricingOutput           *float64               `json:"pricing_output"`
	PricingCurrency         string                 `json:"pricing_currency"`
	SupportsVision          bool                   `json:"supports_vision"`
	SupportsFunctionCalling bool                   `json:"supports_function_calling"`
	SupportsStreaming       bool                   `json:"supports_streaming"`
	SupportsJSONMode        bool                   `json:"supports_json_mode"`
	SupportsImageGeneration bool                   `json:"supports_image_generation"`
	SupportsAudio           bool                   `json:"supports_audio"`
	SupportsCodeGeneration  bool                   `json:"supports_code_generation"`
	SupportsReasoning       bool                   `json:"supports_reasoning"`
	BenchmarkScore          *float64               `json:"benchmark_score"`
	PopularityScore         *int                   `json:"popularity_score"`
	ReliabilityScore        *float64               `json:"reliability_score"`
	ModelType               *string                `json:"model_type"`
	ModelFamily             *string                `json:"model_family"`
	Version                 *string                `json:"version"`
	Tags                    []string               `json:"tags"`
	ModelsDevURL            *string                `json:"modelsdev_url"`
	ModelsDevID             *string                `json:"modelsdev_id"`
	ModelsDevAPIVersion     *string                `json:"modelsdev_api_version"`
	RawMetadata             map[string]interface{} `json:"raw_metadata"`
	LastRefreshedAt         time.Time              `json:"last_refreshed_at"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

type ModelBenchmark struct {
	ID              string                 `json:"id"`
	ModelID         string                 `json:"model_id"`
	BenchmarkName   string                 `json:"benchmark_name"`
	BenchmarkType   *string                `json:"benchmark_type"`
	Score           *float64               `json:"score"`
	Rank            *int                   `json:"rank"`
	NormalizedScore *float64               `json:"normalized_score"`
	BenchmarkDate   *time.Time             `json:"benchmark_date"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       time.Time              `json:"created_at"`
}

type ModelsRefreshHistory struct {
	ID              string                 `json:"id"`
	RefreshType     string                 `json:"refresh_type"`
	Status          string                 `json:"status"`
	ModelsRefreshed int                    `json:"models_refreshed"`
	ModelsFailed    int                    `json:"models_failed"`
	ErrorMessage    *string                `json:"error_message"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at"`
	DurationSeconds *int                   `json:"duration_seconds"`
	Metadata        map[string]interface{} `json:"metadata"`
}

func (r *ModelMetadataRepository) CreateModelMetadata(ctx context.Context, metadata *ModelMetadata) error {
	query := `
		INSERT INTO models_metadata (
			model_id, model_name, provider_id, provider_name,
			description, context_window, max_tokens,
			pricing_input, pricing_output, pricing_currency,
			supports_vision, supports_function_calling, supports_streaming,
			supports_json_mode, supports_image_generation, supports_audio,
			supports_code_generation, supports_reasoning,
			benchmark_score, popularity_score, reliability_score,
			model_type, model_family, version, tags,
			modelsdev_url, modelsdev_id, modelsdev_api_version,
			raw_metadata, last_refreshed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24, $25, $26,
			$27, $28, $29, $30
		)
		ON CONFLICT (model_id)
		DO UPDATE SET
			model_name = EXCLUDED.model_name,
			provider_name = EXCLUDED.provider_name,
			description = EXCLUDED.description,
			context_window = EXCLUDED.context_window,
			max_tokens = EXCLUDED.max_tokens,
			pricing_input = EXCLUDED.pricing_input,
			pricing_output = EXCLUDED.pricing_output,
			pricing_currency = EXCLUDED.pricing_currency,
			supports_vision = EXCLUDED.supports_vision,
			supports_function_calling = EXCLUDED.supports_function_calling,
			supports_streaming = EXCLUDED.supports_streaming,
			supports_json_mode = EXCLUDED.supports_json_mode,
			supports_image_generation = EXCLUDED.supports_image_generation,
			supports_audio = EXCLUDED.supports_audio,
			supports_code_generation = EXCLUDED.supports_code_generation,
			supports_reasoning = EXCLUDED.supports_reasoning,
			benchmark_score = EXCLUDED.benchmark_score,
			popularity_score = EXCLUDED.popularity_score,
			reliability_score = EXCLUDED.reliability_score,
			model_type = EXCLUDED.model_type,
			model_family = EXCLUDED.model_family,
			version = EXCLUDED.version,
			tags = EXCLUDED.tags,
			modelsdev_url = EXCLUDED.modelsdev_url,
			modelsdev_id = EXCLUDED.modelsdev_id,
			modelsdev_api_version = EXCLUDED.modelsdev_api_version,
			raw_metadata = EXCLUDED.raw_metadata,
			last_refreshed_at = EXCLUDED.last_refreshed_at,
			updated_at = NOW()
		RETURNING id
	`

	var id string
	tagsJSON, _ := json.Marshal(metadata.Tags)
	rawMetadataJSON, _ := json.Marshal(metadata.RawMetadata)

	err := r.pool.QueryRow(ctx, query,
		metadata.ModelID, metadata.ModelName, metadata.ProviderID, metadata.ProviderName,
		metadata.Description, metadata.ContextWindow, metadata.MaxTokens,
		metadata.PricingInput, metadata.PricingOutput, metadata.PricingCurrency,
		metadata.SupportsVision, metadata.SupportsFunctionCalling, metadata.SupportsStreaming,
		metadata.SupportsJSONMode, metadata.SupportsImageGeneration, metadata.SupportsAudio,
		metadata.SupportsCodeGeneration, metadata.SupportsReasoning,
		metadata.BenchmarkScore, metadata.PopularityScore, metadata.ReliabilityScore,
		metadata.ModelType, metadata.ModelFamily, metadata.Version, tagsJSON,
		metadata.ModelsDevURL, metadata.ModelsDevID, metadata.ModelsDevAPIVersion,
		rawMetadataJSON, metadata.LastRefreshedAt,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create/update model metadata: %w", err)
	}

	metadata.ID = id
	return nil
}

func (r *ModelMetadataRepository) GetModelMetadata(ctx context.Context, modelID string) (*ModelMetadata, error) {
	query := `
		SELECT
			id, model_id, model_name, provider_id, provider_name,
			description, context_window, max_tokens,
			pricing_input, pricing_output, pricing_currency,
			supports_vision, supports_function_calling, supports_streaming,
			supports_json_mode, supports_image_generation, supports_audio,
			supports_code_generation, supports_reasoning,
			benchmark_score, popularity_score, reliability_score,
			model_type, model_family, version, tags,
			modelsdev_url, modelsdev_id, modelsdev_api_version,
			raw_metadata, last_refreshed_at, created_at, updated_at
		FROM models_metadata
		WHERE model_id = $1
	`

	metadata := &ModelMetadata{}
	var tagsJSON []byte
	var rawMetadataJSON []byte

	err := r.pool.QueryRow(ctx, query, modelID).Scan(
		&metadata.ID, &metadata.ModelID, &metadata.ModelName, &metadata.ProviderID, &metadata.ProviderName,
		&metadata.Description, &metadata.ContextWindow, &metadata.MaxTokens,
		&metadata.PricingInput, &metadata.PricingOutput, &metadata.PricingCurrency,
		&metadata.SupportsVision, &metadata.SupportsFunctionCalling, &metadata.SupportsStreaming,
		&metadata.SupportsJSONMode, &metadata.SupportsImageGeneration, &metadata.SupportsAudio,
		&metadata.SupportsCodeGeneration, &metadata.SupportsReasoning,
		&metadata.BenchmarkScore, &metadata.PopularityScore, &metadata.ReliabilityScore,
		&metadata.ModelType, &metadata.ModelFamily, &metadata.Version, &tagsJSON,
		&metadata.ModelsDevURL, &metadata.ModelsDevID, &metadata.ModelsDevAPIVersion,
		&rawMetadataJSON, &metadata.LastRefreshedAt, &metadata.CreatedAt, &metadata.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("model metadata not found: %s", modelID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get model metadata: %w", err)
	}

	if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil && len(tagsJSON) > 0 {
		// Non-critical: initialize empty slice if unmarshal fails
		metadata.Tags = []string{}
	}
	if err := json.Unmarshal(rawMetadataJSON, &metadata.RawMetadata); err != nil && len(rawMetadataJSON) > 0 {
		// Non-critical: initialize empty map if unmarshal fails
		metadata.RawMetadata = make(map[string]interface{})
	}

	return metadata, nil
}

func (r *ModelMetadataRepository) ListModels(ctx context.Context, providerID string, modelType string, limit int, offset int) ([]*ModelMetadata, int, error) {
	query := `
		SELECT
			id, model_id, model_name, provider_id, provider_name,
			description, context_window, max_tokens,
			pricing_input, pricing_output, pricing_currency,
			supports_vision, supports_function_calling, supports_streaming,
			supports_json_mode, supports_image_generation, supports_audio,
			supports_code_generation, supports_reasoning,
			benchmark_score, popularity_score, reliability_score,
			model_type, model_family, version, tags,
			modelsdev_url, modelsdev_id, modelsdev_api_version,
			raw_metadata, last_refreshed_at, created_at, updated_at
		FROM models_metadata
		WHERE 1=1
	`

	countQuery := "SELECT COUNT(*) FROM models_metadata WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if providerID != "" {
		query += fmt.Sprintf(" AND provider_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND provider_id = $%d", argIdx)
		args = append(args, providerID)
		argIdx++
	}

	if modelType != "" {
		query += fmt.Sprintf(" AND model_type = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND model_type = $%d", argIdx)
		args = append(args, modelType)
		argIdx++
	}

	query += " ORDER BY last_refreshed_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
		argIdx++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, offset)
		argIdx++
	}

	var total int
	countArgs := args
	if limit > 0 {
		countArgs = args[:len(args)-2]
	} else {
		countArgs = args[:len(args)-1]
	}
	err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count models: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	models := []*ModelMetadata{}
	for rows.Next() {
		metadata := &ModelMetadata{}
		var tagsJSON []byte
		var rawMetadataJSON []byte

		err := rows.Scan(
			&metadata.ID, &metadata.ModelID, &metadata.ModelName, &metadata.ProviderID, &metadata.ProviderName,
			&metadata.Description, &metadata.ContextWindow, &metadata.MaxTokens,
			&metadata.PricingInput, &metadata.PricingOutput, &metadata.PricingCurrency,
			&metadata.SupportsVision, &metadata.SupportsFunctionCalling, &metadata.SupportsStreaming,
			&metadata.SupportsJSONMode, &metadata.SupportsImageGeneration, &metadata.SupportsAudio,
			&metadata.SupportsCodeGeneration, &metadata.SupportsReasoning,
			&metadata.BenchmarkScore, &metadata.PopularityScore, &metadata.ReliabilityScore,
			&metadata.ModelType, &metadata.ModelFamily, &metadata.Version, &tagsJSON,
			&metadata.ModelsDevURL, &metadata.ModelsDevID, &metadata.ModelsDevAPIVersion,
			&rawMetadataJSON, &metadata.LastRefreshedAt, &metadata.CreatedAt, &metadata.UpdatedAt,
		)

		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan model row: %w", err)
		}

		if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil && len(tagsJSON) > 0 {
			metadata.Tags = []string{}
		}
		if err := json.Unmarshal(rawMetadataJSON, &metadata.RawMetadata); err != nil && len(rawMetadataJSON) > 0 {
			metadata.RawMetadata = make(map[string]interface{})
		}

		models = append(models, metadata)
	}

	return models, total, nil
}

func (r *ModelMetadataRepository) SearchModels(ctx context.Context, searchTerm string, limit int, offset int) ([]*ModelMetadata, int, error) {
	query := `
		SELECT
			id, model_id, model_name, provider_id, provider_name,
			description, context_window, max_tokens,
			pricing_input, pricing_output, pricing_currency,
			supports_vision, supports_function_calling, supports_streaming,
			supports_json_mode, supports_image_generation, supports_audio,
			supports_code_generation, supports_reasoning,
			benchmark_score, popularity_score, reliability_score,
			model_type, model_family, version, tags,
			modelsdev_url, modelsdev_id, modelsdev_api_version,
			raw_metadata, last_refreshed_at, created_at, updated_at
		FROM models_metadata
		WHERE
			model_name ILIKE '%' || $1 || '%'
			OR provider_name ILIKE '%' || $1 || '%'
			OR description ILIKE '%' || $1 || '%'
			OR $1 = ANY(tags)
		ORDER BY
			CASE
				WHEN model_name ILIKE '%' || $1 || '%' THEN 1
				WHEN provider_name ILIKE '%' || $1 || '%' THEN 2
				ELSE 3
			END,
			benchmark_score DESC NULLS LAST
	`

	countQuery := `
		SELECT COUNT(*)
		FROM models_metadata
		WHERE
			model_name ILIKE '%' || $1 || '%'
			OR provider_name ILIKE '%' || $1 || '%'
			OR description ILIKE '%' || $1 || '%'
			OR $1 = ANY(tags)
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	var total int
	err := r.pool.QueryRow(ctx, countQuery, searchTerm).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, searchTerm)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search models: %w", err)
	}
	defer rows.Close()

	models := []*ModelMetadata{}
	for rows.Next() {
		metadata := &ModelMetadata{}
		var tagsJSON []byte
		var rawMetadataJSON []byte

		err := rows.Scan(
			&metadata.ID, &metadata.ModelID, &metadata.ModelName, &metadata.ProviderID, &metadata.ProviderName,
			&metadata.Description, &metadata.ContextWindow, &metadata.MaxTokens,
			&metadata.PricingInput, &metadata.PricingOutput, &metadata.PricingCurrency,
			&metadata.SupportsVision, &metadata.SupportsFunctionCalling, &metadata.SupportsStreaming,
			&metadata.SupportsJSONMode, &metadata.SupportsImageGeneration, &metadata.SupportsAudio,
			&metadata.SupportsCodeGeneration, &metadata.SupportsReasoning,
			&metadata.BenchmarkScore, &metadata.PopularityScore, &metadata.ReliabilityScore,
			&metadata.ModelType, &metadata.ModelFamily, &metadata.Version, &tagsJSON,
			&metadata.ModelsDevURL, &metadata.ModelsDevID, &metadata.ModelsDevAPIVersion,
			&rawMetadataJSON, &metadata.LastRefreshedAt, &metadata.CreatedAt, &metadata.UpdatedAt,
		)

		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan model row: %w", err)
		}

		if err := json.Unmarshal(tagsJSON, &metadata.Tags); err != nil && len(tagsJSON) > 0 {
			metadata.Tags = []string{}
		}
		if err := json.Unmarshal(rawMetadataJSON, &metadata.RawMetadata); err != nil && len(rawMetadataJSON) > 0 {
			metadata.RawMetadata = make(map[string]interface{})
		}

		models = append(models, metadata)
	}

	return models, total, nil
}

func (r *ModelMetadataRepository) CreateBenchmark(ctx context.Context, benchmark *ModelBenchmark) error {
	query := `
		INSERT INTO model_benchmarks (
			model_id, benchmark_name, benchmark_type, score, rank,
			normalized_score, benchmark_date, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (model_id, benchmark_name)
		DO UPDATE SET
			benchmark_type = EXCLUDED.benchmark_type,
			score = EXCLUDED.score,
			rank = EXCLUDED.rank,
			normalized_score = EXCLUDED.normalized_score,
			benchmark_date = EXCLUDED.benchmark_date,
			metadata = EXCLUDED.metadata
		RETURNING id
	`

	var id string
	metadataJSON, _ := json.Marshal(benchmark.Metadata)

	err := r.pool.QueryRow(ctx, query,
		benchmark.ModelID, benchmark.BenchmarkName, benchmark.BenchmarkType,
		benchmark.Score, benchmark.Rank, benchmark.NormalizedScore,
		benchmark.BenchmarkDate, metadataJSON,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create benchmark: %w", err)
	}

	benchmark.ID = id
	return nil
}

func (r *ModelMetadataRepository) GetBenchmarks(ctx context.Context, modelID string) ([]*ModelBenchmark, error) {
	query := `
		SELECT
			id, model_id, benchmark_name, benchmark_type, score, rank,
			normalized_score, benchmark_date, metadata, created_at
		FROM model_benchmarks
		WHERE model_id = $1
		ORDER BY score DESC NULLS LAST
	`

	rows, err := r.pool.Query(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmarks: %w", err)
	}
	defer rows.Close()

	benchmarks := []*ModelBenchmark{}
	for rows.Next() {
		benchmark := &ModelBenchmark{}
		var metadataJSON []byte

		err := rows.Scan(
			&benchmark.ID, &benchmark.ModelID, &benchmark.BenchmarkName,
			&benchmark.BenchmarkType, &benchmark.Score, &benchmark.Rank,
			&benchmark.NormalizedScore, &benchmark.BenchmarkDate, &metadataJSON, &benchmark.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan benchmark row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &benchmark.Metadata); err != nil && len(metadataJSON) > 0 {
			benchmark.Metadata = make(map[string]interface{})
		}
		benchmarks = append(benchmarks, benchmark)
	}

	return benchmarks, nil
}

func (r *ModelMetadataRepository) CreateRefreshHistory(ctx context.Context, history *ModelsRefreshHistory) error {
	query := `
		INSERT INTO models_refresh_history (
			refresh_type, status, models_refreshed, models_failed,
			error_message, started_at, completed_at, duration_seconds, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var id string
	metadataJSON, _ := json.Marshal(history.Metadata)

	err := r.pool.QueryRow(ctx, query,
		history.RefreshType, history.Status, history.ModelsRefreshed, history.ModelsFailed,
		history.ErrorMessage, history.StartedAt, history.CompletedAt, history.DurationSeconds, metadataJSON,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create refresh history: %w", err)
	}

	history.ID = id
	return nil
}

func (r *ModelMetadataRepository) GetLatestRefreshHistory(ctx context.Context, limit int) ([]*ModelsRefreshHistory, error) {
	query := `
		SELECT
			id, refresh_type, status, models_refreshed, models_failed,
			error_message, started_at, completed_at, duration_seconds, metadata
		FROM models_refresh_history
		ORDER BY started_at DESC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh history: %w", err)
	}
	defer rows.Close()

	histories := []*ModelsRefreshHistory{}
	for rows.Next() {
		history := &ModelsRefreshHistory{}
		var metadataJSON []byte

		err := rows.Scan(
			&history.ID, &history.RefreshType, &history.Status,
			&history.ModelsRefreshed, &history.ModelsFailed, &history.ErrorMessage,
			&history.StartedAt, &history.CompletedAt, &history.DurationSeconds, &metadataJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan refresh history row: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &history.Metadata); err != nil && len(metadataJSON) > 0 {
			history.Metadata = make(map[string]interface{})
		}
		histories = append(histories, history)
	}

	return histories, nil
}

func (r *ModelMetadataRepository) UpdateProviderSyncInfo(ctx context.Context, providerID string, totalModels int, enabledModels int) error {
	query := `
		UPDATE llm_providers
		SET total_models = $2, enabled_models = $3, last_models_sync = NOW()
		WHERE name = $1
	`

	_, err := r.pool.Exec(ctx, query, providerID, totalModels, enabledModels)
	if err != nil {
		return fmt.Errorf("failed to update provider sync info: %w", err)
	}

	return nil
}
