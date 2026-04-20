package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyRequestValidateValid(t *testing.T) {
	opts := VerifyRequest{
		ReferenceSources: "test",
		Claim: "test",
		Context: "test",
		CheckType: "test",
		SourceModel: "gpt-4",
	}
	assert.NoError(t, opts.Validate())
}

func TestVerifyRequestValidateEmpty(t *testing.T) {
	opts := VerifyRequest{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestVerifyResultValidateValid(t *testing.T) {
	opts := VerifyResult{
		Claim: "test",
		Verdict: "test",
		Confidence: 0.95,
		SuggestedCorrections: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestVerifyResultValidateEmpty(t *testing.T) {
	opts := VerifyResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestVerifyResultValidateConfidenceRange(t *testing.T) {
	opts := VerifyResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestHallucinationResultValidateConfidenceRange(t *testing.T) {
	opts := HallucinationResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestFactCheckValidateConfidenceRange(t *testing.T) {
	opts := FactCheck{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
