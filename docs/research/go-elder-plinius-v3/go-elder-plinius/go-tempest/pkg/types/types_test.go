package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWeatherDataDefaults(t *testing.T) {
	opts := WeatherData{}
	opts.Defaults()
	assert.Equal(t, 0.7, opts.Temperature)
}

func TestAugmentOptionsValidateValid(t *testing.T) {
	opts := AugmentOptions{
		AugmentationType: "test",
		Prompt:           "test prompt",
	}
	assert.NoError(t, opts.Validate())
}

func TestAugmentOptionsValidateEmpty(t *testing.T) {
	opts := AugmentOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}
