// Package types defines Go types for the AutoStoryGen library.
// Go library for AutoStoryGen providing automatic agentic story generation with plot planning, character development, scene generation, narrative arc management, and multi-chapter story creation.
package types

import (
	"fmt"
	"strings"
)

// StoryConfig represents storyconfig data.
type StoryConfig struct {
	Characters []CharacterConfig
	Setting string
	Theme string
	Genre string
	Title string
	Tone string
	PlotPoints []string
	WordCountPerChapter int
	TargetAudience string
	ChapterCount int
}

// Validate checks that the StoryConfig is valid.
func (o *StoryConfig) Validate() error {
	if strings.TrimSpace(o.Title) == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

// CharacterConfig represents characterconfig data.
type CharacterConfig struct {
	Role string
	Arc string
	Motivation string
	Description string
	Name string
	Traits []string
}

// Validate checks that the CharacterConfig is valid.
func (o *CharacterConfig) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// Story represents story data.
type Story struct {
	Metadata StoryMetadata
	Title string
	Characters []Character
	Chapters []Chapter
}

// Validate checks that the Story is valid.
func (o *Story) Validate() error {
	if strings.TrimSpace(o.Title) == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

// StoryMetadata represents storymetadata data.
type StoryMetadata struct {
	Tone string
	Setting string
	GeneratedAt string
	Theme string
	TotalWordCount int
	Genre string
}

// Chapter represents chapter data.
type Chapter struct {
	Summary string
	WordCount int
	Content string
	Scenes []Scene
	Title string
	Number int
}

// Validate checks that the Chapter is valid.
func (o *Chapter) Validate() error {
	if strings.TrimSpace(o.Title) == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

// Scene represents scene data.
type Scene struct {
	Mood string
	Setting string
	Content string
	Characters []string
	Number int
}

// Character represents character data.
type Character struct {
	Role string
	Description string
	Traits []string
	Development string
	Name string
}

// Validate checks that the Character is valid.
func (o *Character) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// PlotArc represents plotarc data.
type PlotArc struct {
	FallingAction []string
	Climax string
	Resolution string
	RisingAction []string
	Exposition string
}

