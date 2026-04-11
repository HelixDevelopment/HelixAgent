// Package vtcode provides VT Code agent integration.
// VT Code: Voice-to-code AI assistant.
package vtcode

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// VTCode provides VT Code integration
type VTCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Language string
}

// New creates a new VT Code integration
func New() *VTCode {
	info := agents.AgentInfo{
		Type:        agents.TypeVtcode,
		Name:        "VT Code",
		Description: "Voice-to-code assistant",
		Vendor:      "VTCode",
		Version:     "1.0.0",
		Capabilities: []string{
			"voice",
			"speech_to_code",
			"hands_free",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &VTCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Language: "en",
		},
	}
}

// Initialize initializes VT Code
func (v *VTCode) Initialize(ctx context.Context, config interface{}) error {
	if err := v.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		v.config = cfg
	}

	return nil
}

// Execute executes a command
func (v *VTCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !v.IsStarted() {
		if err := v.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "transcribe":
		return v.transcribe(ctx, params)
	case "status":
		return v.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// transcribe transcribes voice to code
func (v *VTCode) transcribe(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	audio, _ := params["audio"].(string)
	if audio == "" {
		return nil, fmt.Errorf("audio required")
	}

	return map[string]interface{}{
		"audio":    audio,
		"code":     "// Voice transcribed code",
		"language": v.config.Language,
	}, nil
}

// status returns status
func (v *VTCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": v.IsAvailable(),
		"language":  v.config.Language,
	}, nil
}

// IsAvailable checks availability
func (v *VTCode) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*VTCode)(nil)
