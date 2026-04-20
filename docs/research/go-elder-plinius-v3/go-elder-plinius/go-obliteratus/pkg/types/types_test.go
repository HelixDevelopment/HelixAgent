package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObliterateOptionsValidateValid(t *testing.T) {
	opts := ObliterateOptions{
		ModelName: "Test ModelName",
		OutputDir: "test",
		Method: "test",
		Device: "test",
		DataType: "test",
		HarmfulPrompts: "test harmfulprompts",
		HarmlessPrompts: "test harmlessprompts",
	}
	assert.NoError(t, opts.Validate())
}

func TestObliterateOptionsValidateEmpty(t *testing.T) {
	opts := ObliterateOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestMethodInfoValidateValid(t *testing.T) {
	opts := MethodInfo{
		Name: "Test Name",
		Label: "test",
		Description: "test description",
	}
	assert.NoError(t, opts.Validate())
}

func TestMethodInfoValidateEmpty(t *testing.T) {
	opts := MethodInfo{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAnalysisResultValidateValid(t *testing.T) {
	opts := AnalysisResult{
		ModelName: "Test ModelName",
		Architecture: "test",
		DetectedAlignment: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestAnalysisResultValidateEmpty(t *testing.T) {
	opts := AnalysisResult{}
	err := opts.Validate()
	assert.Error(t, err)
}
