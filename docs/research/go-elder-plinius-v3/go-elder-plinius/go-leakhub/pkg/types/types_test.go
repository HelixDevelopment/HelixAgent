package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeakEntryValidateValid(t *testing.T) {
	opts := LeakEntry{
		Model: "gpt-4",
		ID: "test-id-123",
		Confidence: 0.95,
		Date: "test",
		LeakedContent: "test",
		LeakType: "test",
		Source: "test",
		Tags: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestLeakEntryValidateEmpty(t *testing.T) {
	opts := LeakEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestDetectionOptionsValidateValid(t *testing.T) {
	opts := DetectionOptions{
		Response: "test",
		Model: "gpt-4",
		KnownSignatures: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestDetectionOptionsValidateEmpty(t *testing.T) {
	opts := DetectionOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestLeakEntryValidateConfidenceRange(t *testing.T) {
	opts := LeakEntry{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestDetectionResultValidateConfidenceRange(t *testing.T) {
	opts := DetectionResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}

func TestLeakMatchValidateConfidenceRange(t *testing.T) {
	opts := LeakMatch{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
