package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenGenomeValidateValid(t *testing.T) {
	opts := TokenGenome{
		ParentIDs: "test-parentids-123",
		ID: "test-id-123",
	}
	assert.NoError(t, opts.Validate())
}

func TestTokenGenomeValidateEmpty(t *testing.T) {
	opts := TokenGenome{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestEvolutionConfigValidateValid(t *testing.T) {
	opts := EvolutionConfig{
		TargetModel: "gpt-4",
		SelectionMethod: "test",
		FitnessFunction: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestEvolutionConfigValidateEmpty(t *testing.T) {
	opts := EvolutionConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestFitnessTestValidateValid(t *testing.T) {
	opts := FitnessTest{
		Model: "gpt-4",
		Response: "test",
		TestType: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestFitnessTestValidateEmpty(t *testing.T) {
	opts := FitnessTest{}
	err := opts.Validate()
	assert.Error(t, err)
}
