package services

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackChainValidator_Creation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("NewFallbackChainValidator creates validator", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		require.NotNil(t, validator)
		assert.Nil(t, validator.debateTeamConfig)
	})
}

func TestFallbackChainValidator_AlertListener(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("AddAlertListener adds listener", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		alertReceived := make(chan FallbackChainAlert, 1)
		validator.AddAlertListener(func(alert FallbackChainAlert) {
			alertReceived <- alert
		})

		assert.Len(t, validator.listeners, 1)
	})
}

func TestFallbackChainValidator_Validate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("Validate returns invalid when debateTeamConfig is nil", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		result := validator.Validate()

		assert.False(t, result.Valid)
		assert.Len(t, result.Issues, 1)
		assert.Equal(t, "critical", result.Issues[0].Severity)
		assert.Contains(t, result.Issues[0].Description, "not initialized")
	})
}

func TestFallbackChainValidator_GetStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetStatus returns not validated when no validation done", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		status := validator.GetStatus()

		assert.False(t, status.Validated)
		assert.Equal(t, "Validation not yet performed", status.Message)
	})

	t.Run("GetStatus returns result after validation", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		// Trigger validation
		validator.Validate()

		status := validator.GetStatus()

		assert.True(t, status.Validated)
		assert.False(t, status.Valid)
	})
}

func TestFallbackChainValidator_GetLastValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("GetLastValidation returns nil before validation", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		result := validator.GetLastValidation()

		assert.Nil(t, result)
	})

	t.Run("GetLastValidation returns result after validation", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		// Trigger validation
		validator.Validate()

		result := validator.GetLastValidation()

		require.NotNil(t, result)
		assert.False(t, result.Valid)
	})
}

func TestFallbackChainValidator_ValidateOnStartup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("ValidateOnStartup returns error when config is nil", func(t *testing.T) {
		validator := NewFallbackChainValidator(logger, nil)

		err := validator.ValidateOnStartup()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "critical issues")
	})
}

func TestIsFreeProvider(t *testing.T) {
	t.Run("zen is a free provider", func(t *testing.T) {
		assert.True(t, isFreeProvider("zen"))
	})

	t.Run("openrouter is a free provider", func(t *testing.T) {
		assert.True(t, isFreeProvider("openrouter"))
	})

	t.Run("cerebras is not a free provider", func(t *testing.T) {
		assert.False(t, isFreeProvider("cerebras"))
	})

	t.Run("mistral is not a free provider", func(t *testing.T) {
		assert.False(t, isFreeProvider("mistral"))
	})

	t.Run("claude is not a free provider", func(t *testing.T) {
		assert.False(t, isFreeProvider("claude"))
	})
}

func TestFallbackChainIssue_Severities(t *testing.T) {
	t.Run("Issue severities are valid", func(t *testing.T) {
		validSeverities := []string{"warning", "critical"}

		issue := FallbackChainIssue{
			Severity:    "critical",
			Description: "Test issue",
		}

		assert.Contains(t, validSeverities, issue.Severity)
	})
}

func TestFallbackChainPositionInfo(t *testing.T) {
	t.Run("PositionInfo contains all required fields", func(t *testing.T) {
		info := FallbackChainPositionInfo{
			Position:        1,
			PositionName:    "Analyst",
			PrimaryProvider: "claude",
			PrimaryModel:    "claude-3-opus",
			PrimaryIsOAuth:  true,
			Fallbacks:       []string{"cerebras/llama-3.3-70b", "mistral/mistral-large"},
			HasDiversity:    true,
		}

		assert.Equal(t, 1, info.Position)
		assert.Equal(t, "Analyst", info.PositionName)
		assert.Equal(t, "claude", info.PrimaryProvider)
		assert.True(t, info.PrimaryIsOAuth)
		assert.Len(t, info.Fallbacks, 2)
		assert.True(t, info.HasDiversity)
	})
}

func TestFallbackChainValidationResult(t *testing.T) {
	t.Run("ValidationResult contains all required fields", func(t *testing.T) {
		result := &FallbackChainValidationResult{
			Valid:             true,
			DiversityScore:    85.0,
			Issues:            []FallbackChainIssue{},
			Positions:         []FallbackChainPositionInfo{},
			UniqueProviders:   5,
			HasOAuthPrimaries: true,
			HasAPIFallbacks:   true,
		}

		assert.True(t, result.Valid)
		assert.Equal(t, 85.0, result.DiversityScore)
		assert.Equal(t, 5, result.UniqueProviders)
		assert.True(t, result.HasOAuthPrimaries)
		assert.True(t, result.HasAPIFallbacks)
	})
}

func TestFallbackChainAlert(t *testing.T) {
	t.Run("Alert contains all required fields", func(t *testing.T) {
		alert := FallbackChainAlert{
			Type:     "validation_failed",
			Message:  "Fallback chain validation failed",
			Position: 1,
			Details:  "OAuth primary has no non-OAuth fallbacks",
			Severity: "critical",
		}

		assert.Equal(t, "validation_failed", alert.Type)
		assert.Equal(t, "critical", alert.Severity)
		assert.Equal(t, 1, alert.Position)
	})
}
