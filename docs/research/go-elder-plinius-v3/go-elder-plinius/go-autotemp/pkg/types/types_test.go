package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunOptionsValidateValid(t *testing.T) {
	opts := RunOptions{
		Prompt:       "test prompt",
		ModelVersion: "gpt-4",
	}
	assert.NoError(t, opts.Validate())
}

func TestRunOptionsValidateEmpty(t *testing.T) {
	opts := RunOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestRunOptionsDefaults(t *testing.T) {
	opts := RunOptions{}
	opts.Prompt = "test"
	opts.Defaults()
}

func TestEvaluateOptionsValidateValid(t *testing.T) {
	opts := EvaluateOptions{
		Prompt:       "test prompt",
		Output:       "test",
		ModelVersion: "gpt-4",
	}
	assert.NoError(t, opts.Validate())
}

func TestEvaluateOptionsValidateEmpty(t *testing.T) {
	opts := EvaluateOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestEvaluateOptionsDefaults(t *testing.T) {
	opts := EvaluateOptions{}
	opts.Prompt = "test"
	opts.Defaults()
	assert.Equal(t, 0.7, opts.Temperature)
}

func TestBenchmarkOptionsDefaults(t *testing.T) {
	opts := BenchmarkOptions{}
	opts.Defaults()
}

func TestBenchmarkItemValidateValid(t *testing.T) {
	opts := BenchmarkItem{
		Prompt:    "test prompt",
		Reference: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestBenchmarkItemValidateEmpty(t *testing.T) {
	opts := BenchmarkItem{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestModelBenchmarkValidateValid(t *testing.T) {
	opts := ModelBenchmark{
		ModelName: "Test ModelName",
	}
	assert.NoError(t, opts.Validate())
}

func TestModelBenchmarkValidateEmpty(t *testing.T) {
	opts := ModelBenchmark{}
	err := opts.Validate()
	assert.Error(t, err)
}
