package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// DocumentationSync manages synchronization between Constitution, AGENTS.md, and CLAUDE.md
type DocumentationSync struct {
	logger *logrus.Logger
}

// NewDocumentationSync creates a new documentation sync manager
func NewDocumentationSync(logger *logrus.Logger) *DocumentationSync {
	return &DocumentationSync{
		logger: logger,
	}
}

// SyncConstitutionToDocumentation synchronizes Constitution rules to AGENTS.md and CLAUDE.md
func (ds *DocumentationSync) SyncConstitutionToDocumentation(projectRoot string, constitution *Constitution) error {
	ds.logger.Info("[DocSync] Starting Constitution synchronization")

	// Generate Constitution section content
	constitutionSection := ds.generateConstitutionSection(constitution)

	// Sync to AGENTS.md
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if err := ds.syncToFile(agentsPath, "Constitution", constitutionSection); err != nil {
		return fmt.Errorf("failed to sync to AGENTS.md: %w", err)
	}
	ds.logger.Info("[DocSync] Synced to AGENTS.md")

	// Sync to CLAUDE.md
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")
	if err := ds.syncToFile(claudePath, "Constitution", constitutionSection); err != nil {
		return fmt.Errorf("failed to sync to CLAUDE.md: %w", err)
	}
	ds.logger.Info("[DocSync] Synced to CLAUDE.md")

	// Save Constitution as JSON
	constitutionManager := NewConstitutionManager(ds.logger)
	if err := constitutionManager.SaveConstitution(projectRoot, constitution); err != nil {
		return fmt.Errorf("failed to save CONSTITUTION.json: %w", err)
	}
	ds.logger.Info("[DocSync] Saved CONSTITUTION.json")

	// Save Constitution as markdown
	constitutionMDPath := filepath.Join(projectRoot, "CONSTITUTION.md")
	markdown := constitutionManager.ExportConstitutionMarkdown(constitution)
	// #nosec G306 -- documentation files are intentionally world-readable (not sensitive)
	if err := os.WriteFile(constitutionMDPath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("failed to save CONSTITUTION.md: %w", err)
	}
	ds.logger.Info("[DocSync] Saved CONSTITUTION.md")

	return nil
}

// generateConstitutionSection generates the Constitution section content for markdown files
func (ds *DocumentationSync) generateConstitutionSection(constitution *Constitution) string {
	var content strings.Builder

	content.WriteString("# Project Constitution\n\n")
	content.WriteString(fmt.Sprintf("**Version:** %s | **Updated:** %s\n\n",
		constitution.Version,
		constitution.UpdatedAt.Format("2006-01-02 15:04")))
	content.WriteString(fmt.Sprintf("%s\n\n", constitution.Summary))

	content.WriteString("## Mandatory Principles\n\n")
	content.WriteString("**All development MUST adhere to these non-negotiable principles:**\n\n")

	// Group mandatory rules by category
	mandatoryByCategory := make(map[string][]ConstitutionRule)
	for _, rule := range constitution.Rules {
		if rule.Mandatory {
			mandatoryByCategory[rule.Category] = append(mandatoryByCategory[rule.Category], rule)
		}
	}

	// Output mandatory rules by category (all 14 categories)
	categories := []string{
		"Architecture", "Testing", "Documentation", "Quality", "Safety", "Security",
		"Performance", "Principles", "Stability", "Containerization", "Configuration",
		"Observability", "GitOps", "CI/CD",
	}
	for _, category := range categories {
		rules, exists := mandatoryByCategory[category]
		if !exists || len(rules) == 0 {
			continue
		}

		content.WriteString(fmt.Sprintf("### %s\n\n", category))
		for _, rule := range rules {
			content.WriteString(fmt.Sprintf("**%s** (Priority: %d)\n", rule.Title, rule.Priority))
			content.WriteString(fmt.Sprintf("- %s\n\n", rule.Description))
		}
	}

	// Add context rules (non-mandatory)
	contextRules := []ConstitutionRule{}
	for _, rule := range constitution.Rules {
		if !rule.Mandatory {
			contextRules = append(contextRules, rule)
		}
	}

	if len(contextRules) > 0 {
		content.WriteString("## Context-Specific Guidelines\n\n")
		content.WriteString("**Additional guidelines derived from project context:**\n\n")

		contextByCategory := make(map[string][]ConstitutionRule)
		for _, rule := range contextRules {
			contextByCategory[rule.Category] = append(contextByCategory[rule.Category], rule)
		}

		for category, rules := range contextByCategory {
			content.WriteString(fmt.Sprintf("### %s\n\n", category))
			for _, rule := range rules {
				content.WriteString(fmt.Sprintf("**%s** (Priority: %d)\n", rule.Title, rule.Priority))
				content.WriteString(fmt.Sprintf("- %s\n\n", rule.Description))
			}
		}
	}

	content.WriteString("---\n\n")
	content.WriteString("*This Constitution is automatically synchronized with AGENTS.md, CLAUDE.md, and CONSTITUTION.json.*\n\n")

	return content.String()
}

// syncToFile updates a section in a markdown file
func (ds *DocumentationSync) syncToFile(filePath, sectionName, newContent string) error {
	// Read existing file
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist, create it with content
		ds.logger.WithField("file", filePath).Warn("File doesn't exist, creating new file")
		return ds.createFileWithSection(filePath, sectionName, newContent)
	}

	content := string(existingContent)

	// Find Constitution section markers
	startMarker := fmt.Sprintf("<!-- BEGIN_%s -->", strings.ToUpper(sectionName))
	endMarker := fmt.Sprintf("<!-- END_%s -->", strings.ToUpper(sectionName))

	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	var updatedContent string

	if startIdx == -1 || endIdx == -1 {
		// Section doesn't exist, append at end
		ds.logger.WithField("file", filePath).Info("Section not found, appending to end")
		updatedContent = content + "\n\n" + ds.wrapSection(sectionName, newContent)
	} else {
		// Section exists, replace content between markers
		ds.logger.WithField("file", filePath).Info("Section found, updating content")
		before := content[:startIdx]
		after := content[endIdx+len(endMarker):]
		updatedContent = before + ds.wrapSection(sectionName, newContent) + after
	}

	// Write updated content.
	// #nosec G306 G703 -- filePath is supplied by the constitution_watcher
	// from a fixed allow-list of project-root docs (AGENTS.md, CLAUDE.md,
	// CONSTITUTION.md); never from user input. Files are intentionally
	// world-readable.
	if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// createFileWithSection creates a new file with the section
func (ds *DocumentationSync) createFileWithSection(filePath, sectionName, content string) error {
	fileName := filepath.Base(filePath)

	var fileContent strings.Builder
	fileContent.WriteString(fmt.Sprintf("# %s\n\n", strings.TrimSuffix(fileName, ".md")))
	fileContent.WriteString(fmt.Sprintf("*Auto-generated: %s*\n\n", time.Now().Format("2006-01-02 15:04")))
	fileContent.WriteString(ds.wrapSection(sectionName, content))

	// #nosec G306 -- documentation files are intentionally world-readable (not sensitive)
	return os.WriteFile(filePath, []byte(fileContent.String()), 0644)
}

// wrapSection wraps content with section markers
func (ds *DocumentationSync) wrapSection(sectionName, content string) string {
	startMarker := fmt.Sprintf("<!-- BEGIN_%s -->", strings.ToUpper(sectionName))
	endMarker := fmt.Sprintf("<!-- END_%s -->", strings.ToUpper(sectionName))
	return fmt.Sprintf("%s\n%s%s\n", startMarker, content, endMarker)
}

// ExtractConstitutionFromDocumentation extracts Constitution content from markdown files
func (ds *DocumentationSync) ExtractConstitutionFromDocumentation(projectRoot string) (string, error) {
	// Try AGENTS.md first
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if content, err := ds.extractSectionFromFile(agentsPath, "Constitution"); err == nil && content != "" {
		ds.logger.Info("[DocSync] Extracted Constitution from AGENTS.md")
		return content, nil
	}

	// Try CLAUDE.md
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")
	if content, err := ds.extractSectionFromFile(claudePath, "Constitution"); err == nil && content != "" {
		ds.logger.Info("[DocSync] Extracted Constitution from CLAUDE.md")
		return content, nil
	}

	return "", fmt.Errorf("no Constitution content found in documentation files")
}

// extractSectionFromFile extracts a section from a markdown file
func (ds *DocumentationSync) extractSectionFromFile(filePath, sectionName string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	startMarker := fmt.Sprintf("<!-- BEGIN_%s -->", strings.ToUpper(sectionName))
	endMarker := fmt.Sprintf("<!-- END_%s -->", strings.ToUpper(sectionName))

	contentStr := string(content)
	startIdx := strings.Index(contentStr, startMarker)
	endIdx := strings.Index(contentStr, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return "", fmt.Errorf("section not found")
	}

	// Extract content between markers
	extracted := contentStr[startIdx+len(startMarker) : endIdx]
	return strings.TrimSpace(extracted), nil
}

// ValidateSync checks if Constitution is in sync with documentation
func (ds *DocumentationSync) ValidateSync(projectRoot string, constitution *Constitution) []string {
	var issues []string

	// Check AGENTS.md
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		issues = append(issues, "AGENTS.md does not exist")
	} else {
		if content, err := ds.extractSectionFromFile(agentsPath, "Constitution"); err != nil || content == "" {
			issues = append(issues, "AGENTS.md does not contain Constitution section")
		} else {
			// Check if content mentions mandatory rules
			for _, rule := range constitution.Rules {
				if rule.Mandatory && !strings.Contains(content, rule.Title) {
					issues = append(issues, fmt.Sprintf("AGENTS.md missing mandatory rule: %s", rule.Title))
				}
			}
		}
	}

	// Check CLAUDE.md
	claudePath := filepath.Join(projectRoot, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		issues = append(issues, "CLAUDE.md does not exist")
	} else {
		if content, err := ds.extractSectionFromFile(claudePath, "Constitution"); err != nil || content == "" {
			issues = append(issues, "CLAUDE.md does not contain Constitution section")
		} else {
			// Check if content mentions mandatory rules
			for _, rule := range constitution.Rules {
				if rule.Mandatory && !strings.Contains(content, rule.Title) {
					issues = append(issues, fmt.Sprintf("CLAUDE.md missing mandatory rule: %s", rule.Title))
				}
			}
		}
	}

	// Check CONSTITUTION.json
	constitutionPath := filepath.Join(projectRoot, "CONSTITUTION.json")
	if _, err := os.Stat(constitutionPath); os.IsNotExist(err) {
		issues = append(issues, "CONSTITUTION.json does not exist")
	}

	// Check CONSTITUTION.md
	constitutionMDPath := filepath.Join(projectRoot, "CONSTITUTION.md")
	if _, err := os.Stat(constitutionMDPath); os.IsNotExist(err) {
		issues = append(issues, "CONSTITUTION.md does not exist")
	}

	if len(issues) == 0 {
		ds.logger.Info("[DocSync] Constitution is in sync with all documentation")
	} else {
		ds.logger.WithField("issue_count", len(issues)).Warn("[DocSync] Constitution sync issues found")
	}

	return issues
}

// SyncFromDocumentation updates Constitution based on changes in AGENTS.md or CLAUDE.md
func (ds *DocumentationSync) SyncFromDocumentation(projectRoot string, constitution *Constitution) error {
	ds.logger.Info("[DocSync] Syncing Constitution from documentation")

	// Extract content from documentation
	content, err := ds.ExtractConstitutionFromDocumentation(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to extract Constitution from documentation: %w", err)
	}

	// Parse content and identify new rules
	// This is a simplified implementation - full version would parse markdown and extract rules
	ds.logger.WithField("content_length", len(content)).Info("[DocSync] Constitution extracted from documentation (manual review recommended)")

	// For now, just log that manual review is needed
	// Full implementation would parse the markdown and update constitution.Rules
	ds.logger.Warn("[DocSync] Manual review of documentation changes recommended")

	return nil
}

// GenerateConstitutionReport generates a comprehensive Constitution report
func (ds *DocumentationSync) GenerateConstitutionReport(projectRoot string, constitution *Constitution) string {
	var report strings.Builder

	report.WriteString("# Constitution Synchronization Report\n\n")
	report.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Constitution summary
	report.WriteString("## Constitution Summary\n\n")
	report.WriteString(fmt.Sprintf("- **Version:** %s\n", constitution.Version))
	report.WriteString(fmt.Sprintf("- **Total Rules:** %d\n", len(constitution.Rules)))

	mandatoryCount := 0
	for _, rule := range constitution.Rules {
		if rule.Mandatory {
			mandatoryCount++
		}
	}
	report.WriteString(fmt.Sprintf("- **Mandatory Rules:** %d\n", mandatoryCount))
	report.WriteString(fmt.Sprintf("- **Context Rules:** %d\n\n", len(constitution.Rules)-mandatoryCount))

	// Validation results
	report.WriteString("## Synchronization Status\n\n")
	issues := ds.ValidateSync(projectRoot, constitution)
	if len(issues) == 0 {
		report.WriteString("✅ **All documentation is synchronized**\n\n")
	} else {
		report.WriteString(fmt.Sprintf("⚠️ **%d synchronization issues found:**\n\n", len(issues)))
		for _, issue := range issues {
			report.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		report.WriteString("\n")
	}

	// Files status
	report.WriteString("## Documentation Files\n\n")
	files := []string{"CONSTITUTION.json", "CONSTITUTION.md", "AGENTS.md", "CLAUDE.md"}
	for _, file := range files {
		filePath := filepath.Join(projectRoot, file)
		if _, err := os.Stat(filePath); err == nil {
			info, _ := os.Stat(filePath) //nolint:errcheck
			report.WriteString(fmt.Sprintf("- ✅ **%s** (Size: %d bytes, Modified: %s)\n",
				file,
				info.Size(),
				info.ModTime().Format("2006-01-02 15:04")))
		} else {
			report.WriteString(fmt.Sprintf("- ❌ **%s** (Missing)\n", file))
		}
	}
	report.WriteString("\n")

	// Rules by category
	report.WriteString("## Rules by Category\n\n")
	categoryCount := make(map[string]int)
	for _, rule := range constitution.Rules {
		categoryCount[rule.Category]++
	}
	for category, count := range categoryCount {
		report.WriteString(fmt.Sprintf("- **%s:** %d rules\n", category, count))
	}
	report.WriteString("\n")

	return report.String()
}
