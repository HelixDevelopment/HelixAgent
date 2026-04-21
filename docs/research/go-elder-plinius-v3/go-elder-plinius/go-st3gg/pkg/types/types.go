// Package types defines Go types for the ST3GG library.
// Go implementation of ST3GG providing 100+ steganography techniques for hiding data in images (LSB, DCT, DWT), audio (echo hiding, phase coding, spread spectrum), documents (whitespace, metadata), and network packets (TCP/IP headers, DNS tunnels).
package types

import (
	"fmt"
	"strings"
)

// EmbedOptions represents embedoptions data.
type EmbedOptions struct {
	Quality int
	Method string
	Secret []byte
	Password string
	Carrier []byte
}

// Validate checks that the EmbedOptions is valid.
func (o *EmbedOptions) Validate() error {
	if len(o.Secret) == 0 {
		return fmt.Errorf("secret is required")
	}
	if len(o.Carrier) == 0 {
		return fmt.Errorf("carrier is required")
	}
	return nil
}

// ExtractOptions represents extractoptions data.
type ExtractOptions struct {
	Carrier []byte
	Password string
	Method string
}

// Validate checks that the ExtractOptions is valid.
func (o *ExtractOptions) Validate() error {
	if len(o.Carrier) == 0 {
		return fmt.Errorf("carrier is required")
	}
	return nil
}

// StegoMethod represents stegomethod data.
type StegoMethod struct {
	Description string
	Detectability float64
	Category string
	Name string
	Robustness float64
	Capacity float64
	Reversible bool
}

// Validate checks that the StegoMethod is valid.
func (o *StegoMethod) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// EmbedResult represents embedresult data.
type EmbedResult struct {
	Method string
	PSNR float64
	EmbeddingTime int64
	CapacityUsed float64
	Output []byte
}

// ExtractResult represents extractresult data.
type ExtractResult struct {
	Secret []byte
	ExtractionTime int64
	Confidence float64
	Method string
}

// Validate checks that the ExtractResult is valid.
func (o *ExtractResult) Validate() error {
	if len(o.Secret) == 0 {
		return fmt.Errorf("secret is required")
	}
	return nil
}

// AnalyzeResult represents analyzeresult data.
type AnalyzeResult struct {
	Confidence float64
	Suspicious bool
	Details map[string]float64
	DetectedMethods []string
}

// CapacityMB returns capacity in megabytes.
func (s *StegoMethod) CapacityMB() float64 {
	return s.Capacity / (1024 * 1024)
}

// Defaults applies default values for unset fields.
func (o *EmbedOptions) Defaults() {}

// Defaults applies default values for unset fields.
func (o *ExtractOptions) Defaults() {}

// MethodComparison represents a comparison between steganography methods.
type MethodComparison struct {
	Method string
	Capacity int64
	Quality float64
	Detectability float64
}

