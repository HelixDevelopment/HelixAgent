package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChallengeSolutionValidateValid(t *testing.T) {
	opts := ChallengeSolution{
		ID: "test-id-123",
		Difficulty: "test",
		Solution: "test",
		Explanation: "test",
		Level: "test",
		Challenge: "test",
		Tags: "test",
		Platform: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestChallengeSolutionValidateEmpty(t *testing.T) {
	opts := ChallengeSolution{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestChallengeEntryValidateValid(t *testing.T) {
	opts := ChallengeEntry{
		Description: "test description",
		Name: "Test Name",
		ID: "test-id-123",
		Difficulty: "test",
		Platform: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestChallengeEntryValidateEmpty(t *testing.T) {
	opts := ChallengeEntry{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsValidateValid(t *testing.T) {
	opts := SearchOptions{
		Difficulties: "test",
		Limit: 10,
		Query: "test query",
		Platforms: "test",
		Tags: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestSearchOptionsValidateEmpty(t *testing.T) {
	opts := SearchOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestSearchOptionsDefaults(t *testing.T) {
	opts := SearchOptions{}
	opts.Query = "test"
	opts.Defaults()
	assert.Equal(t, 50, opts.Limit)
}

func TestSearchOptionsValidateLimitNegative(t *testing.T) {
	opts := SearchOptions{Query: "test", Limit: -1}
	assert.Error(t, opts.Validate())
}
