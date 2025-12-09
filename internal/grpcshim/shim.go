package grpcshim

import (
	"time"

	"github.com/superagent/superagent/internal/models"
)

type ShimCompleteRequest struct {
	Prompt         string                 `json:"prompt"`
	SessionID      string                 `json:"session_id"`
	UserID         string                 `json:"user_id"`
	MemoryEnhanced bool                   `json:"memory_enhanced"`
	ModelParams    models.ModelParameters `json:"model_params"`
	EnsembleConfig *models.EnsembleConfig `json:"ensemble_config"`
	RequestType    string                 `json:"request_type"`
}

func (s *ShimCompleteRequest) ToLLMRequest() *models.LLMRequest {
	return &models.LLMRequest{
		ID:             s.SessionID + ":shim",
		SessionID:      s.SessionID,
		UserID:         s.UserID,
		Prompt:         s.Prompt,
		ModelParams:    s.ModelParams,
		EnsembleConfig: s.EnsembleConfig,
		MemoryEnhanced: s.MemoryEnhanced,
		RequestType:    s.RequestType,
		CreatedAt:      time.Now(),
	}
}
