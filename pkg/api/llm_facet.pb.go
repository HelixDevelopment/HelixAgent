package api

import "context"

// Minimal placeholders for gRPC-like pb bindings (no actual codegen). This stub allows building progress
// until real generated pb.go bindings are available via protoc/protoc-gen-go.

type CompletionRequest struct {
	Prompt         string
	SessionID      string
	UserID         string
	MemoryEnhanced bool
}

type CompletionResponse struct {
	Response   string
	Confidence float64
}

// LLMFacadeServer defines a minimal interface matching the expected gRPC surface.
type LLMFacadeServer interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// LLMFacadeServiceServer is a placeholder for the actual generated server wrapper.
type LLMFacadeServiceServer = LLMFacadeServer

// RegisterLLMFacadeServer is a no-op placeholder to keep compatibility when real generated code is unavailable.
func RegisterLLMFacadeServer(s interface{}, srv LLMFacadeServer) {}

// Additional minimal types to satisfy potential imports in a real-generated file.
type _LLMFacadeServer interface {
	Complete(context.Context, *CompletionRequest) (*CompletionResponse, error)
}
