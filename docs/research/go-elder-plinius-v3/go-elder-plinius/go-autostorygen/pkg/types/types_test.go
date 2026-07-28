package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStoryConfigValidateValid(t *testing.T) {
	opts := StoryConfig{
		Setting:        "test",
		Theme:          "test",
		Genre:          "test",
		Title:          "Test Title",
		Tone:           "test",
		PlotPoints:     "test",
		TargetAudience: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestStoryConfigValidateEmpty(t *testing.T) {
	opts := StoryConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCharacterConfigValidateValid(t *testing.T) {
	opts := CharacterConfig{
		Role:        "test",
		Arc:         "test",
		Motivation:  "test",
		Description: "test description",
		Name:        "Test Name",
		Traits:      "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestCharacterConfigValidateEmpty(t *testing.T) {
	opts := CharacterConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestStoryValidateValid(t *testing.T) {
	opts := Story{
		Title: "Test Title",
	}
	assert.NoError(t, opts.Validate())
}

func TestStoryValidateEmpty(t *testing.T) {
	opts := Story{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestChapterValidateValid(t *testing.T) {
	opts := Chapter{
		Summary: "test",
		Content: "test",
		Title:   "Test Title",
	}
	assert.NoError(t, opts.Validate())
}

func TestChapterValidateEmpty(t *testing.T) {
	opts := Chapter{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCharacterValidateValid(t *testing.T) {
	opts := Character{
		Role:        "test",
		Description: "test description",
		Traits:      "test",
		Development: "test",
		Name:        "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestCharacterValidateEmpty(t *testing.T) {
	opts := Character{}
	err := opts.Validate()
	assert.Error(t, err)
}
