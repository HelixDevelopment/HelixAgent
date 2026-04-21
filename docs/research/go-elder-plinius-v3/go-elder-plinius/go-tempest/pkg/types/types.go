// Package types defines Go types for the Tempest library.
// Go library for Tempest providing AI-powered environmental context awareness, weather data integration for agent systems, and contextual prompt augmentation based on temporal and environmental conditions.
package types

import (
	"fmt"
	"strings"
)

// WeatherData represents weatherdata data.
type WeatherData struct {
	Humidity float64
	Location string
	Timestamp string
	Conditions string
	WindSpeed float64
	Temperature float64
}

// Defaults applies default values for unset fields.
func (o *WeatherData) Defaults() {
	if o.Temperature == 0 { o.Temperature = 0.7 }
}

// ContextConfig represents contextconfig data.
type ContextConfig struct {
	Location string
	IncludeSeason bool
	IncludeWeather bool
	IncludeTime bool
	CustomFactors map[string]string
}

// ContextResult represents contextresult data.
type ContextResult struct {
	Weather *WeatherData
	ContextString string
	Season string
	SuggestedMood string
	TimeOfDay string
}

// AugmentOptions represents augmentoptions data.
type AugmentOptions struct {
	AugmentationType string
	Context ContextConfig
	Prompt string
}

// Validate checks that the AugmentOptions is valid.
func (o *AugmentOptions) Validate() error {
	if strings.TrimSpace(o.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *AugmentOptions) Defaults() {}

