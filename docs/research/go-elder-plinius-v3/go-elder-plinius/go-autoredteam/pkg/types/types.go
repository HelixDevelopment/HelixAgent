// Package types defines Go types for the AutoRedTeam library.
// Go library for AutoRedTeam implementing autonomous AI red teaming with attack strategy proposal, automated vulnerability discovery, safety evaluation, and continuous adversarial testing.
package types

import (
	"fmt"
	"strings"
)

// AttackConfig represents attackconfig data.
type AttackConfig struct {
	Parallelism int
	AttackTypes []string
	TargetModel string
	Timeout int
	Iterations int
	SuccessCriteria string
}

// Validate checks that the AttackConfig is valid.
func (o *AttackConfig) Validate() error {
	if strings.TrimSpace(o.TargetModel) == "" {
		return fmt.Errorf("targetmodel is required")
	}
	return nil
}

// AttackResult represents attackresult data.
type AttackResult struct {
	Response string
	AttackType string
	Evidence string
	Confidence float64
	Payload string
	Remediation string
	Severity string
	Success bool
}

// Validate checks that the AttackResult is valid.
func (o *AttackResult) Validate() error {
	if strings.TrimSpace(o.AttackType) == "" {
		return fmt.Errorf("attacktype is required")
	}
	return nil
}

// CampaignConfig represents campaignconfig data.
type CampaignConfig struct {
	Reporting bool
	AttackTypes []string
	MaxIterations int
	Duration string
	TargetModels []string
	Name string
}

// Validate checks that the CampaignConfig is valid.
func (o *CampaignConfig) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// CampaignResult represents campaignresult data.
type CampaignResult struct {
	EndTime string
	Summary CampaignSummary
	Results []AttackResult
	StartTime string
	Duration string
	Name string
}

// Validate checks that the CampaignResult is valid.
func (o *CampaignResult) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// CampaignSummary represents campaignsummary data.
type CampaignSummary struct {
	ByType map[string]int
	Failed int
	SuccessRate float64
	TotalAttacks int
	SeverityCounts map[string]int
	Successful int
}

// VulnerabilityReport represents vulnerabilityreport data.
type VulnerabilityReport struct {
	Model string
	Vulnerabilities []Vulnerability
	RiskScore float64
	Recommendations []string
}

// Validate checks that the VulnerabilityReport is valid.
func (o *VulnerabilityReport) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// Vulnerability represents vulnerability data.
type Vulnerability struct {
	Remediation string
	Description string
	Evidence string
	Severity string
	Reproducible bool
	Type string
}

// Validate checks that the Vulnerability is valid.
func (o *Vulnerability) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

