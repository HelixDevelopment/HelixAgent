// Package types defines Go types for the AlmechE library.
// Go client for AlmechE -- the Idea-to-Object speech-to-CAD generation service. Transforms spoken descriptions into physical 3D models ready for 3D printing.
package types

import (
	"fmt"
	"strings"
)

// ProcessSpeechOptions represents processspeechoptions data.
type ProcessSpeechOptions struct {
	AudioData []byte
	AudioFormat string
	SampleRate int
	TargetFormat string
	IncludeEstimate bool
	Material string
	Scale float64
}

// Validate checks that the ProcessSpeechOptions is valid.
func (o *ProcessSpeechOptions) Validate() error {
	if len(o.AudioData) == 0 {
		return fmt.Errorf("audiodata is required")
	}
	if o.SampleRate < 8000 || o.SampleRate > 192000 {
		return fmt.Errorf("sample_rate must be between 8000 and 192000")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *ProcessSpeechOptions) Defaults() {
	if o.Scale == 0 { o.Scale = 1.0 }
}

// GenerateCADOptions represents generatecadoptions data.
type GenerateCADOptions struct {
	Description string
	TargetFormat string
	Complexity string
	IncludeEstimate bool
	Material string
	Scale float64
}

// Validate checks that the GenerateCADOptions is valid.
func (o *GenerateCADOptions) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *GenerateCADOptions) Defaults() {
	if o.Scale == 0 { o.Scale = 1.0 }
}

// GenerateCADResult represents generatecadresult data.
type GenerateCADResult struct {
	CADPrompt string
	ModelData []byte
	ModelFormat string
	DimensionsMM []float64
	VolumeCM3 float64
	FaceCount int
	EstimatedPrintTimeMin float64
	CostEstimate *CostEstimate
}

// Validate checks that the GenerateCADResult is valid.
func (o *GenerateCADResult) Validate() error {
	if o.VolumeCM3 <= 0 {
		return fmt.Errorf("volume_cm3 must be positive")
	}
	return nil
}

// CostEstimate represents costestimate data.
type CostEstimate struct {
	MaterialWeightG float64
	MaterialCostUSD float64
	PrintTimeMin float64
	TotalCostUSD float64
}

// ExportOptions represents exportoptions data.
type ExportOptions struct {
	ModelData []byte
	SourceFormat string
	TargetFormat string
	Binary bool
	Precision int
	Scale float64
	RotationDeg []float64
}

// Defaults applies default values for unset fields.
func (o *ExportOptions) Defaults() {
	if o.Scale == 0 { o.Scale = 1.0 }
}

// ExportResult represents exportresult data.
type ExportResult struct {
	ExportedData []byte
	TargetFormat string
	FileSize int64
}

// TextToSpeechOptions represents texttospeechoptions data.
type TextToSpeechOptions struct {
	Text string
	VoiceID string
	Speed float64
}

// Validate checks that the TextToSpeechOptions is valid.
func (o *TextToSpeechOptions) Validate() error {
	if strings.TrimSpace(o.Text) == "" {
		return fmt.Errorf("text is required")
	}
	if o.Speed != 0 && (o.Speed < 0.5 || o.Speed > 2.0) {
		return fmt.Errorf("speed must be between 0.5 and 2.0")
	}
	return nil
}

// TextToSpeechResult represents texttospeechresult data.
type TextToSpeechResult struct {
	AudioData []byte
	DurationSec float64
}

// Validate checks that the TextToSpeechResult is valid.
func (o *TextToSpeechResult) Validate() error {
	if len(o.AudioData) == 0 {
		return fmt.Errorf("audiodata is required")
	}
	return nil
}

// Material represents material data.
type Material struct {
	Name string
	Type string
	DensityGCM3 float64
	CostPerKGUSD float64
	MinLayerHeightMM float64
	MaxTempC float64
	ColorOptions string
	Description string
	SupportsMulticolor bool
}

// Validate checks that the Material is valid.
func (o *Material) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// EstimateOptions represents estimateoptions data.
type EstimateOptions struct {
	VolumeCM3 float64
	Material string
	InfillPercent float64
	LayerHeightMM float64
}

// Validate checks that the EstimateOptions is valid.
func (o *EstimateOptions) Validate() error {
	if o.VolumeCM3 <= 0 {
		return fmt.Errorf("volume_cm3 must be positive")
	}
	if o.InfillPercent < 0 || o.InfillPercent > 100 {
		return fmt.Errorf("infill_percent must be between 0 and 100")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *EstimateOptions) Defaults() {
	if o.InfillPercent == 0 { o.InfillPercent = 20 }
	if o.LayerHeightMM == 0 { o.LayerHeightMM = 0.2 }
}

// EstimateResult represents estimateresult data.
type EstimateResult struct {
	MaterialWeightG float64
	MaterialCostUSD float64
	PrintTimeMin float64
	ElectricityCostUSD float64
	TotalCostUSD float64
}

