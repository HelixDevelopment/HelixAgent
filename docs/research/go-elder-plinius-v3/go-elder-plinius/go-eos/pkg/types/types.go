// Package types defines Go types for the Eos library.
// Go client for Eos -- a Discord bot that orchestrates open-source developers across multiple servers, facilitating recruitment, skill matching, project discovery, and notifications.
package types

import (
	"fmt"
	"strings"
)

// UserProfile represents userprofile data.
type UserProfile struct {
	UserID string
	DiscordUsername string
	DisplayName string
	Skills []string
	Languages []string
	ExperienceLevel string
	Bio string
	GitHubUsername string
	Availability string
	Interests []string
	OnboardingComplete bool
}

// Validate checks that the UserProfile is valid.
func (o *UserProfile) Validate() error {
	if strings.TrimSpace(o.UserID) == "" {
		return fmt.Errorf("userid is required")
	}
	return nil
}

// Project represents project data.
type Project struct {
	ProjectID string
	Name string
	Description string
	Status string
	TechStack []string
	RequiredSkills []string
	Difficulty string
	MaxContributors int
	CurrentContributors int
	RepositoryURL string
}

// Validate checks that the Project is valid.
func (o *Project) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// ProjectMatch represents projectmatch data.
type ProjectMatch struct {
	Project *Project
	MatchScore float64
	MatchedSkills []string
	MissingSkills []string
}

// AuthenticateOptions represents authenticateoptions data.
type AuthenticateOptions struct {
	DiscordToken string
	GuildID string
}

// Validate checks that the AuthenticateOptions is valid.
func (o *AuthenticateOptions) Validate() error {
	if strings.TrimSpace(o.DiscordToken) == "" {
		return fmt.Errorf("discordtoken is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *AuthenticateOptions) Defaults() {}

// AuthenticateResult represents authenticateresult data.
type AuthenticateResult struct {
	SessionToken string
	User *UserProfile
	IsNewUser bool
}

// MatchSkillsOptions represents matchskillsoptions data.
type MatchSkillsOptions struct {
	UserID string
	MaxResults int
	SkillFilters []string
}

// Validate checks that the MatchSkillsOptions is valid.
func (o *MatchSkillsOptions) Validate() error {
	if strings.TrimSpace(o.UserID) == "" {
		return fmt.Errorf("userid is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *MatchSkillsOptions) Defaults() {
	if o.MaxResults == 0 { o.MaxResults = 10 }
}

// MatchSkillsResponse represents matchskillsresponse data.
type MatchSkillsResponse struct {
	Matches []ProjectMatch
	OverallMatchScore float64
}

// DiscoverOptions represents discoveroptions data.
type DiscoverOptions struct {
	Skills []string
	Languages []string
	Difficulty string
	Status string
	Page int
	PageSize int
}

// Validate checks that the DiscoverOptions is valid.
func (o *DiscoverOptions) Validate() error { return nil }

// Defaults applies default values for unset fields.
func (o *DiscoverOptions) Defaults() {
	if o.PageSize == 0 { o.PageSize = 20 }
}

// JoinProjectOptions represents joinprojectoptions data.
type JoinProjectOptions struct {
	UserID string
	ProjectID string
	Role string
	Message string
}

// Validate checks that the JoinProjectOptions is valid.
func (o *JoinProjectOptions) Validate() error {
	if strings.TrimSpace(o.UserID) == "" {
		return fmt.Errorf("userid is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *JoinProjectOptions) Defaults() {}

// JoinResult represents joinresult data.
type JoinResult struct {
	Success bool
	ProjectChannelID string
	Message string
}

// OnboardOptions represents onboardoptions data.
type OnboardOptions struct {
	UserID string
	Skills []string
	Languages []string
	ExperienceLevel string
	Bio string
	GitHubUsername string
	Availability string
	Interests []string
}

// Validate checks that the OnboardOptions is valid.
func (o *OnboardOptions) Validate() error {
	if strings.TrimSpace(o.UserID) == "" {
		return fmt.Errorf("userid is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *OnboardOptions) Defaults() {}

// OnboardResult represents onboardresult data.
type OnboardResult struct {
	Success bool
	InitialMatches []ProjectMatch
	WelcomeMessage string
}

// IsRecruiting returns true if the project is actively recruiting.
func (p *Project) IsRecruiting() bool {
	return p.Status == "recruiting" && p.CurrentContributors < p.MaxContributors
}

