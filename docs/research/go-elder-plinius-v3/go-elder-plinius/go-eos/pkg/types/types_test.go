package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserProfileValidateValid(t *testing.T) {
	opts := UserProfile{
		UserID: "test-userid-123",
		DiscordUsername: "test",
		DisplayName: "Test DisplayName",
		Skills: "test",
		Languages: "test",
		ExperienceLevel: "test",
		Bio: "test",
		GitHubUsername: "test",
		Availability: "test",
		Interests: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestUserProfileValidateEmpty(t *testing.T) {
	opts := UserProfile{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestProjectValidateValid(t *testing.T) {
	opts := Project{
		ProjectID: "test-projectid-123",
		Name: "Test Name",
		Description: "test description",
		Status: "test",
		TechStack: "test",
		RequiredSkills: "test",
		Difficulty: "test",
		RepositoryURL: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestProjectValidateEmpty(t *testing.T) {
	opts := Project{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestAuthenticateOptionsValidateValid(t *testing.T) {
	opts := AuthenticateOptions{
		DiscordToken: "test",
		GuildID: "test-guildid-123",
	}
	assert.NoError(t, opts.Validate())
}

func TestAuthenticateOptionsValidateEmpty(t *testing.T) {
	opts := AuthenticateOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestMatchSkillsOptionsValidateValid(t *testing.T) {
	opts := MatchSkillsOptions{
		UserID: "test-userid-123",
		SkillFilters: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestMatchSkillsOptionsValidateEmpty(t *testing.T) {
	opts := MatchSkillsOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestMatchSkillsOptionsDefaults(t *testing.T) {
	opts := MatchSkillsOptions{}
	opts.Defaults()
	assert.Equal(t, 10, opts.MaxResults)
}

func TestDiscoverOptionsDefaults(t *testing.T) {
	opts := DiscoverOptions{}
	opts.Defaults()
	assert.Equal(t, 20, opts.PageSize)
}

func TestJoinProjectOptionsValidateValid(t *testing.T) {
	opts := JoinProjectOptions{
		UserID: "test-userid-123",
		ProjectID: "test-projectid-123",
		Role: "test",
		Message: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestJoinProjectOptionsValidateEmpty(t *testing.T) {
	opts := JoinProjectOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}

func TestOnboardOptionsValidateValid(t *testing.T) {
	opts := OnboardOptions{
		UserID: "test-userid-123",
		Skills: "test",
		Languages: "test",
		ExperienceLevel: "test",
		Bio: "test",
		GitHubUsername: "test",
		Availability: "test",
		Interests: "test",
	}
	assert.NoError(t, opts.Validate())
}

func TestOnboardOptionsValidateEmpty(t *testing.T) {
	opts := OnboardOptions{}
	err := opts.Validate()
	assert.Error(t, err)
}
