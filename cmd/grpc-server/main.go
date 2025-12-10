package main

import (
	"context"
	"log"
	"net"
	"time"

	llm "github.com/superagent/superagent/internal/llm"
	models "github.com/superagent/superagent/internal/models"
	pb "github.com/superagent/superagent/pkg/api"
	"google.golang.org/grpc"
)

type grpcServer struct{}

func (s *grpcServer) Complete(ctx context.Context, req *pb.CompletionRequest) (*pb.CompletionResponse, error) {
	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             "",
		SessionID:      req.SessionId,
		UserID:         "", // Not in proto, set empty
		Prompt:         req.Prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		EnsembleConfig: nil,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	responses, selected, err := llm.RunEnsemble(internal)
	if err != nil {
		return &pb.CompletionResponse{Content: "", Confidence: 0}, err
	}
	var out pb.CompletionResponse
	if len(responses) > 0 && responses[0] != nil {
		out.Content = responses[0].Content
		out.Confidence = responses[0].Confidence
		out.ProviderName = responses[0].ProviderName
	}
	if selected != nil {
		out.Content = selected.Content
		out.Confidence = selected.Confidence
	}
	return &out, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	// pb.RegisterLLMFacadeServer(s, &grpcServer{}) // enable when pb.go is generated
	log.Println("gRPC-like server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
