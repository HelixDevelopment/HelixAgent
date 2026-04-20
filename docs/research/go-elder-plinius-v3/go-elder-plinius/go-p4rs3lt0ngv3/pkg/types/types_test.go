package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformConfigValidateValid(t *testing.T) {
	opts := TransformConfig{
		Description: "test description",
		Category: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestTransformConfigValidateEmpty(t *testing.T) {
	opts := TransformConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestMultiTransformOptionsValidateValid(t *testing.T) {
	opts := MultiTransformOptions{
		Transforms: "test",
		Text: "test text",
	}
	assert.NoError(t, opts.Validate())
}

func TestMultiTransformOptionsValidateEmpty(t *testing.T) {
	opts := MultiTransformOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestEncodeDecodeOptionsValidateValid(t *testing.T) {
	opts := EncodeDecodeOptions{
		Key: "test",
		Encoding: "test",
		Text: "test text",
	}
	assert.NoError(t, opts.Validate())
}

func TestEncodeDecodeOptionsValidateEmpty(t *testing.T) {
	opts := EncodeDecodeOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}
