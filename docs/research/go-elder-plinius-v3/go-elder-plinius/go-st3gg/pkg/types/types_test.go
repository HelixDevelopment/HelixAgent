package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbedOptionsValidateValid(t *testing.T) {
	opts := EmbedOptions{
		Method:   "test",
		Secret:   []byte{1, 2, 3},
		Password: "test",
		Carrier:  []byte{1, 2, 3},
	}
	assert.NoError(t, opts.Validate())
}

func TestEmbedOptionsValidateEmpty(t *testing.T) {
	opts := EmbedOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestExtractOptionsValidateValid(t *testing.T) {
	opts := ExtractOptions{
		Carrier:  []byte{1, 2, 3},
		Password: "test",
		Method:   "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestExtractOptionsValidateEmpty(t *testing.T) {
	opts := ExtractOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestStegoMethodValidateValid(t *testing.T) {
	opts := StegoMethod{
		Description: "test description",
		Category:    "test",
		Name:        "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestStegoMethodValidateEmpty(t *testing.T) {
	opts := StegoMethod{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestExtractResultValidateValid(t *testing.T) {
	opts := ExtractResult{
		Secret:     []byte{1, 2, 3},
		Confidence: 0.95,
		Method:     "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestExtractResultValidateEmpty(t *testing.T) {
	opts := ExtractResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestExtractResultValidateConfidenceRange(t *testing.T) {
	opts := ExtractResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestAnalyzeResultValidateConfidenceRange(t *testing.T) {
	opts := AnalyzeResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
