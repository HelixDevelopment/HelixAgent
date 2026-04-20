package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttackConfigValidateValid(t *testing.T) {
	opts := AttackConfig{
		AttackTypes: "test",
		TargetModel: "gpt-4",
		SuccessCriteria: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestAttackConfigValidateEmpty(t *testing.T) {
	opts := AttackConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAttackResultValidateValid(t *testing.T) {
	opts := AttackResult{
		Response: "test",
		AttackType: "test",
		Evidence: "test",
		Confidence: 0.95,
		Payload: "test",
		Remediation: "test",
		Severity: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestAttackResultValidateEmpty(t *testing.T) {
	opts := AttackResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCampaignConfigValidateValid(t *testing.T) {
	opts := CampaignConfig{
		AttackTypes: "test",
		Duration: "test",
		TargetModels: "gpt-4",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestCampaignConfigValidateEmpty(t *testing.T) {
	opts := CampaignConfig{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestCampaignResultValidateValid(t *testing.T) {
	opts := CampaignResult{
		EndTime: "test",
		StartTime: "test",
		Duration: "test",
		Name: "Test Name",
	}
	assert.NoError(t, opts.Validate())
}

func TestCampaignResultValidateEmpty(t *testing.T) {
	opts := CampaignResult{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestVulnerabilityReportValidateValid(t *testing.T) {
	opts := VulnerabilityReport{
		Model: "gpt-4",
		Recommendations: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestVulnerabilityReportValidateEmpty(t *testing.T) {
	opts := VulnerabilityReport{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestVulnerabilityValidateValid(t *testing.T) {
	opts := Vulnerability{
		Remediation: "test",
		Description: "test description",
		Evidence: "test",
		Severity: "test",
		Type: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestVulnerabilityValidateEmpty(t *testing.T) {
	opts := Vulnerability{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAttackResultValidateConfidenceRange(t *testing.T) {
	opts := AttackResult{ID: "test", Confidence: 1.5}
	assert.Error(t, opts.Validate())
	opts.Confidence = -0.1
	assert.Error(t, opts.Validate())
}
