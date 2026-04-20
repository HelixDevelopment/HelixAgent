package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConlangConfigValidateValid(t *testing.T) {
	opts := ConlangConfig{
		MorphologyType: "test",
		Difficulty: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestConlangConfigValidateEmpty(t *testing.T) {
	opts := ConlangConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestGeneratedLanguageValidateValid(t *testing.T) {
	opts := GeneratedLanguage{
		SampleText: "test sampletext",
		Dictionary: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestGeneratedLanguageValidateEmpty(t *testing.T) {
	opts := GeneratedLanguage{}
	err := opts.Validate()
	assert.Error(t, err)
}
