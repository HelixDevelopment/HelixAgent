// Package types defines Go types for the P4RS3LT0NGV3 library.
// Go implementation of P4RS3LT0NGV3 providing 159+ text transforms including encodings (Base64, ROT13, URL encoding), classical/modern ciphers (Caesar, Vigenere, Atbash), Unicode styles (bold, italic, strikethrough, script), formatting, and niche alphabets.
package types

import (
	"fmt"
	"strings"
)

// TransformResult represents transformresult data.
type TransformResult struct {
	Transformed string
	Category string
	TransformName string
	Original string
}

// TransformConfig represents transformconfig data.
type TransformConfig struct {
	Unicode bool
	NeedsKey bool
	Description string
	Category string
	Reversible bool
	Name string
}

// Validate checks that the TransformConfig is valid.
func (o *TransformConfig) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// MultiTransformOptions represents multitransformoptions data.
type MultiTransformOptions struct {
	Transforms []string
	Text string
	Parallel bool
}

// Validate checks that the MultiTransformOptions is valid.
func (o *MultiTransformOptions) Validate() error {
	if strings.TrimSpace(o.Text) == "" {
		return fmt.Errorf("text is required")
	}
	return nil
}

// EncodeDecodeOptions represents encodedecodeoptions data.
type EncodeDecodeOptions struct {
	Key string
	Encoding string
	Text string
}

// Validate checks that the EncodeDecodeOptions is valid.
func (o *EncodeDecodeOptions) Validate() error {
	if strings.TrimSpace(o.Text) == "" {
		return fmt.Errorf("text is required")
	}
	return nil
}

