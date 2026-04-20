package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessSpeechOptionsValidateValid(t *testing.T) {
	opts := ProcessSpeechOptions{
		AudioData: []byte{1, 2, 3},
		AudioFormat: "test",
		SampleRate: 16000,
		TargetFormat: "test",
		Material: "PLA",
	}
	assert.NoError(t, opts.Validate())
}

func TestProcessSpeechOptionsValidateEmpty(t *testing.T) {
	opts := ProcessSpeechOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestProcessSpeechOptionsDefaults(t *testing.T) {
	opts := ProcessSpeechOptions{}
	opts.AudioFormat = "wav"
	opts.SampleRate = 16000
	opts.Material = "PLA"
	opts.Defaults()
	assert.Equal(t, 1.0, opts.Scale)
}

func TestGenerateCADOptionsValidateValid(t *testing.T) {
	opts := GenerateCADOptions{
		Description: "test description",
		TargetFormat: "test",
		Complexity: "test",
		Material: "PLA",
	}
	assert.NoError(t, opts.Validate())
}

func TestGenerateCADOptionsValidateEmpty(t *testing.T) {
	opts := GenerateCADOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestGenerateCADOptionsDefaults(t *testing.T) {
	opts := GenerateCADOptions{}
	opts.Description = "test"
	opts.Material = "PLA"
	opts.Defaults()
	assert.Equal(t, 1.0, opts.Scale)
}

func TestExportOptionsDefaults(t *testing.T) {
	opts := ExportOptions{}
	opts.Defaults()
	assert.Equal(t, 1.0, opts.Scale)
}

func TestTextToSpeechOptionsValidateValid(t *testing.T) {
	opts := TextToSpeechOptions{
		Text: "test text",
		VoiceID: "test-voiceid-123",
	}
	assert.NoError(t, opts.Validate())
}

func TestTextToSpeechOptionsValidateEmpty(t *testing.T) {
	opts := TextToSpeechOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestTextToSpeechResultValidateValid(t *testing.T) {
	opts := TextToSpeechResult{
		AudioData: []byte{1, 2, 3},
	}
	assert.NoError(t, opts.Validate())
}

func TestTextToSpeechResultValidateEmpty(t *testing.T) {
	opts := TextToSpeechResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestMaterialValidateValid(t *testing.T) {
	opts := Material{
		Name: "Test Name",
		Type: "test",
		ColorOptions: "test",
		Description: "test description",
	}
	assert.NoError(t, opts.Validate())
}

func TestMaterialValidateEmpty(t *testing.T) {
	opts := Material{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestEstimateOptionsDefaults(t *testing.T) {
	opts := EstimateOptions{}
	opts.VolumeCM3 = 10.0
	opts.Material = "PLA"
	opts.Defaults()
}
