package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
)

func main() {
	// Get project root
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// Default to current directory's parent
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
			os.Exit(1)
		}
		projectRoot = filepath.Dir(filepath.Dir(cwd)) // Go up two levels from cmd/generate-constitution
	}

	fmt.Printf("Generating Constitution for project: %s\n", projectRoot)

	// Create logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create Constitution manager
	constitutionManager := services.NewConstitutionManager(logger)

	// Create or load Constitution
	ctx := context.Background()
	constitution, err := constitutionManager.LoadOrCreateConstitution(ctx, projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Constitution: %v\n", err)
		os.Exit(1)
	}

	// Save Constitution
	if err := constitutionManager.SaveConstitution(projectRoot, constitution); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save Constitution: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Constitution saved to: %s/CONSTITUTION.json\n", projectRoot)
	fmt.Printf("   Version: %s\n", constitution.Version)
	fmt.Printf("   Total Rules: %d\n", len(constitution.Rules))

	mandatoryCount := 0
	for _, rule := range constitution.Rules {
		if rule.Mandatory {
			mandatoryCount++
		}
	}
	fmt.Printf("   Mandatory Rules: %d\n", mandatoryCount)

	// Create documentation sync
	documentationSync := services.NewDocumentationSync(logger)

	// Sync to documentation files
	if err := documentationSync.SyncConstitutionToDocumentation(projectRoot, constitution); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync Constitution to documentation: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Constitution synchronized to:\n")
	fmt.Printf("   - %s/CONSTITUTION.md\n", projectRoot)
	fmt.Printf("   - %s/AGENTS.md\n", projectRoot)
	fmt.Printf("   - %s/CLAUDE.md\n", projectRoot)

	// Generate validation report
	report := documentationSync.GenerateConstitutionReport(projectRoot, constitution)
	reportPath := filepath.Join(projectRoot, "CONSTITUTION_REPORT.md")
	// #nosec G703 -- reportPath is always projectRoot/CONSTITUTION_REPORT.md,
	// rooted at the operator-supplied projectRoot CLI arg; never from LLM or
	// HTTP input.
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save Constitution report: %v\n", err)
	} else {
		fmt.Printf("✅ Constitution report saved to: %s\n", reportPath)
	}

	fmt.Println("\n✨ Constitution generation complete!")
}
