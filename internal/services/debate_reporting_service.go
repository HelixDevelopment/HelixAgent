package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// ParticipantReport holds report data for a participant
type ParticipantReport struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	ResponseCount     int           `json:"response_count"`
	AverageConfidence float64       `json:"average_confidence"`
	TotalTokens       int           `json:"total_tokens"`
	AverageLatency    time.Duration `json:"average_latency"`
	ErrorCount        int           `json:"error_count"`
	TopContributions  []string      `json:"top_contributions,omitempty"`
}

// ConsensusReport holds consensus analysis data
type ConsensusReport struct {
	ConsensusReached bool     `json:"consensus_reached"`
	AgreementLevel   float64  `json:"agreement_level"`
	DissenterCount   int      `json:"dissenter_count"`
	KeyAgreements    []string `json:"key_agreements"`
	KeyDisagreements []string `json:"key_disagreements"`
}

// QualityReport holds quality analysis data
type QualityReport struct {
	OverallScore    float64 `json:"overall_score"`
	CoherenceScore  float64 `json:"coherence_score"`
	RelevanceScore  float64 `json:"relevance_score"`
	DepthScore      float64 `json:"depth_score"`
	FactualityScore float64 `json:"factuality_score"`
	NoveltyScore    float64 `json:"novelty_score"`
}

// ExtendedDebateReport extends the base DebateReport with additional fields
type ExtendedDebateReport struct {
	DebateReport
	Participants []ParticipantReport `json:"participants"`
	Consensus    *ConsensusReport    `json:"consensus,omitempty"`
	Quality      *QualityReport      `json:"quality,omitempty"`
}

// DebateReportingService provides reporting capabilities.
// Concurrent-safe by construction (CONST-029 Tier 1): reports and
// templates are independent safe.Stores. Templates are populated once
// at construction and thereafter read-only; reports grow over time.
type DebateReportingService struct {
	logger    *logrus.Logger
	reports   *safe.Store[string, *ExtendedDebateReport]
	templates *safe.Store[string, *template.Template]
}

// NewDebateReportingService creates a new reporting service
func NewDebateReportingService(logger *logrus.Logger) *DebateReportingService {
	svc := &DebateReportingService{
		logger:    logger,
		reports:   safe.NewStore[string, *ExtendedDebateReport](),
		templates: safe.NewStore[string, *template.Template](),
	}
	svc.initTemplates()
	return svc
}

// initTemplates initializes report templates
func (drs *DebateReportingService) initTemplates() {
	// HTML template for reports
	htmlTmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Debate Report: {{.DebateID}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        .section { margin: 20px 0; padding: 15px; background: #f5f5f5; border-radius: 5px; }
        .metric { display: inline-block; margin: 10px; padding: 10px; background: #fff; border: 1px solid #ddd; }
        .finding { padding: 5px 0; border-bottom: 1px solid #eee; }
    </style>
</head>
<body>
    <h1>Debate Report</h1>
    <p><strong>Report ID:</strong> {{.ReportID}}</p>
    <p><strong>Debate ID:</strong> {{.DebateID}}</p>
    <p><strong>Generated:</strong> {{.GeneratedAt}}</p>

    <div class="section">
        <h2>Summary</h2>
        <p>{{.Summary}}</p>
    </div>

    <div class="section">
        <h2>Key Findings</h2>
        {{range .KeyFindings}}
        <div class="finding">{{.}}</div>
        {{end}}
    </div>

    <div class="section">
        <h2>Recommendations</h2>
        {{range .Recommendations}}
        <div class="finding">{{.}}</div>
        {{end}}
    </div>

    <div class="section">
        <h2>Metrics</h2>
        <div class="metric"><strong>Duration:</strong> {{.Metrics.Duration}}</div>
        <div class="metric"><strong>Rounds:</strong> {{.Metrics.TotalRounds}}</div>
        <div class="metric"><strong>Quality:</strong> {{printf "%.2f" .Metrics.QualityScore}}</div>
    </div>
</body>
</html>`

	tmpl, err := template.New("html").Parse(htmlTmpl)
	if err == nil {
		drs.templates.Put("html", tmpl)
	}

	// Markdown template
	mdTmpl := `# Debate Report

**Report ID:** {{.ReportID}}
**Debate ID:** {{.DebateID}}
**Generated:** {{.GeneratedAt}}

## Summary
{{.Summary}}

## Key Findings
{{range .KeyFindings}}
- {{.}}
{{end}}

## Recommendations
{{range .Recommendations}}
- {{.}}
{{end}}

## Metrics
| Metric | Value |
|--------|-------|
| Duration | {{.Metrics.Duration}} |
| Total Rounds | {{.Metrics.TotalRounds}} |
| Quality Score | {{printf "%.2f" .Metrics.QualityScore}} |
| Throughput | {{printf "%.2f" .Metrics.Throughput}} |
`

	mdTemplate, err := template.New("markdown").Parse(mdTmpl)
	if err == nil {
		drs.templates.Put("markdown", mdTemplate)
		drs.templates.Put("md", mdTemplate)
	}
}

// GenerateReport generates a comprehensive report for a debate
func (drs *DebateReportingService) GenerateReport(ctx context.Context, result *DebateResult) (*DebateReport, error) {
	if result == nil {
		return nil, fmt.Errorf("debate result is required")
	}

	reportID := "report-" + uuid.New().String()[:8]

	report := &ExtendedDebateReport{
		DebateReport: DebateReport{
			ReportID:    reportID,
			DebateID:    result.DebateID,
			GeneratedAt: time.Now(),
			Metrics: PerformanceMetrics{
				Duration:     result.Duration,
				TotalRounds:  result.TotalRounds,
				QualityScore: result.QualityScore,
			},
		},
	}

	// Generate summary
	report.Summary = drs.generateSummary(result)

	// Generate key findings
	report.KeyFindings = drs.generateKeyFindings(result)

	// Generate recommendations
	report.Recommendations = drs.generateRecommendations(result)

	// Generate participant reports
	report.Participants = drs.generateParticipantReports(result)

	// Generate consensus report
	if result.Consensus != nil {
		report.Consensus = drs.generateConsensusReport(result)
	}

	// Generate quality report
	if result.QualityMetrics != nil {
		report.Quality = drs.generateQualityReport(result)
	}

	// Calculate throughput
	if result.Duration.Minutes() > 0 {
		report.Metrics.Throughput = float64(result.TotalRounds) / result.Duration.Minutes()
	}

	// Store report
	drs.reports.Put(reportID, report)

	drs.logger.WithFields(logrus.Fields{
		"report_id": reportID,
		"debate_id": result.DebateID,
	}).Info("Generated debate report")

	return &report.DebateReport, nil
}

// generateSummary creates a summary of the debate
func (drs *DebateReportingService) generateSummary(result *DebateResult) string {
	var summary string

	if result.Success {
		summary = fmt.Sprintf("The debate on '%s' was successfully completed in %d rounds over %v. ",
			result.Topic, result.TotalRounds, result.Duration)
	} else {
		summary = fmt.Sprintf("The debate on '%s' ended after %d rounds with partial completion. ",
			result.Topic, result.RoundsConducted)
	}

	if result.CogneeEnhanced {
		summary += "The debate was enhanced with Cognee knowledge graph integration. "
	}

	if result.Consensus != nil && result.Consensus.Achieved {
		summary += fmt.Sprintf("Consensus was reached with %.1f%% agreement. ", result.Consensus.AgreementScore*100)
	}

	summary += fmt.Sprintf("Overall quality score: %.2f.", result.QualityScore)

	return summary
}

// generateKeyFindings extracts key findings from the debate
func (drs *DebateReportingService) generateKeyFindings(result *DebateResult) []string {
	findings := make([]string, 0)

	// Finding: Success/failure
	if result.Success {
		findings = append(findings, "Debate completed successfully within the allocated rounds")
	} else {
		findings = append(findings, "Debate did not complete all planned rounds")
	}

	// Finding: Quality assessment
	if result.QualityScore >= 0.8 {
		findings = append(findings, fmt.Sprintf("High quality responses achieved (score: %.2f)", result.QualityScore))
	} else if result.QualityScore >= 0.5 {
		findings = append(findings, fmt.Sprintf("Moderate quality responses (score: %.2f)", result.QualityScore))
	} else {
		findings = append(findings, fmt.Sprintf("Low quality responses detected (score: %.2f)", result.QualityScore))
	}

	// Finding: Consensus
	if result.Consensus != nil {
		if result.Consensus.Achieved {
			findings = append(findings, fmt.Sprintf("Strong consensus achieved (%.1f%% agreement)", result.Consensus.AgreementScore*100))
		} else {
			findings = append(findings, "Participants did not reach consensus")
		}
	}

	// Finding: Best response
	if result.BestResponse != nil {
		findings = append(findings, fmt.Sprintf("Best response identified from %s with %.1f%% confidence",
			result.BestResponse.ParticipantName, result.BestResponse.Confidence*100))
	}

	// Finding: Memory/Cognee usage
	if result.CogneeEnhanced {
		findings = append(findings, "Knowledge graph enhancement was utilized")
	}
	if result.MemoryUsed {
		findings = append(findings, "Conversation memory was utilized")
	}

	return findings
}

// generateRecommendations creates recommendations based on the debate
func (drs *DebateReportingService) generateRecommendations(result *DebateResult) []string {
	recommendations := make([]string, 0)

	// Quality-based recommendations
	if result.QualityScore < 0.5 {
		recommendations = append(recommendations, "Consider using more capable LLM models for better quality")
		recommendations = append(recommendations, "Increase the number of debate rounds for more refined responses")
	}

	// Consensus-based recommendations
	if result.Consensus != nil && !result.Consensus.Achieved {
		recommendations = append(recommendations, "Consider adding a mediator participant to help build consensus")
		recommendations = append(recommendations, "Review divergent viewpoints for potential valid alternative perspectives")
	}

	// Performance recommendations
	if result.Duration > 5*time.Minute {
		recommendations = append(recommendations, "Optimize provider response times to reduce overall debate duration")
	}

	// Enhancement recommendations
	if !result.CogneeEnhanced {
		recommendations = append(recommendations, "Enable Cognee integration for knowledge-enhanced debates")
	}
	if !result.MemoryUsed {
		recommendations = append(recommendations, "Enable memory to maintain context across sessions")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Debate performed well - consider maintaining current configuration")
	}

	return recommendations
}

// generateParticipantReports creates reports for each participant
func (drs *DebateReportingService) generateParticipantReports(result *DebateResult) []ParticipantReport {
	participantStats := make(map[string]*ParticipantReport)

	for _, resp := range result.AllResponses {
		if _, exists := participantStats[resp.ParticipantID]; !exists {
			participantStats[resp.ParticipantID] = &ParticipantReport{
				ID:   resp.ParticipantID,
				Name: resp.ParticipantName,
			}
		}

		stats := participantStats[resp.ParticipantID]
		stats.ResponseCount++
		stats.AverageConfidence += resp.Confidence
		stats.AverageLatency += resp.ResponseTime

		// Check for error-like conditions (empty response or very low confidence)
		if resp.Response == "" || resp.Confidence < 0.1 {
			stats.ErrorCount++
		}

		// Track top contributions (high confidence responses)
		if resp.Confidence > 0.8 && len(stats.TopContributions) < 3 {
			if len(resp.Response) > 100 {
				stats.TopContributions = append(stats.TopContributions, resp.Response[:100]+"...")
			} else if resp.Response != "" {
				stats.TopContributions = append(stats.TopContributions, resp.Response)
			}
		}
	}

	reports := make([]ParticipantReport, 0, len(participantStats))
	for _, stats := range participantStats {
		if stats.ResponseCount > 0 {
			stats.AverageConfidence /= float64(stats.ResponseCount)
			stats.AverageLatency /= time.Duration(stats.ResponseCount)
		}
		reports = append(reports, *stats)
	}

	return reports
}

// generateConsensusReport creates a consensus analysis report
func (drs *DebateReportingService) generateConsensusReport(result *DebateResult) *ConsensusReport {
	if result.Consensus == nil {
		return nil
	}

	return &ConsensusReport{
		ConsensusReached: result.Consensus.Achieved,
		AgreementLevel:   result.Consensus.AgreementScore,
		KeyAgreements:    result.Consensus.KeyPoints,
	}
}

// generateQualityReport creates a quality analysis report
func (drs *DebateReportingService) generateQualityReport(result *DebateResult) *QualityReport {
	if result.QualityMetrics == nil {
		return nil
	}

	return &QualityReport{
		OverallScore:    result.QualityMetrics.OverallScore,
		CoherenceScore:  result.QualityMetrics.Coherence,
		RelevanceScore:  result.QualityMetrics.Relevance,
		DepthScore:      result.QualityMetrics.Completeness, // Use Completeness as proxy for depth
		FactualityScore: result.QualityMetrics.Accuracy,
	}
}

// ExportReport exports a report in the specified format
func (drs *DebateReportingService) ExportReport(ctx context.Context, reportID string, format string) ([]byte, error) {
	report, exists := drs.reports.Get(reportID)
	if !exists {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}

	switch format {
	case "json":
		return json.MarshalIndent(report, "", "  ")

	case "html":
		tmpl, exists := drs.templates.Get("html")
		if !exists {
			return nil, fmt.Errorf("HTML template not available")
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, report); err != nil {
			return nil, fmt.Errorf("failed to generate HTML: %w", err)
		}
		return buf.Bytes(), nil

	case "markdown", "md":
		tmpl, exists := drs.templates.Get("markdown")
		if !exists {
			return nil, fmt.Errorf("Markdown template not available")
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, report); err != nil {
			return nil, fmt.Errorf("failed to generate Markdown: %w", err)
		}
		return buf.Bytes(), nil

	case "text", "txt":
		return drs.generateTextReport(&report.DebateReport), nil

	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: json, html, markdown, text)", format)
	}
}

// generateTextReport creates a plain text report
func (drs *DebateReportingService) generateTextReport(report *DebateReport) []byte {
	var buf bytes.Buffer

	buf.WriteString("=== DEBATE REPORT ===\n\n")
	buf.WriteString(fmt.Sprintf("Report ID: %s\n", report.ReportID))
	buf.WriteString(fmt.Sprintf("Debate ID: %s\n", report.DebateID))
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", report.GeneratedAt.Format(time.RFC3339)))

	buf.WriteString("--- Summary ---\n")
	buf.WriteString(report.Summary + "\n\n")

	buf.WriteString("--- Key Findings ---\n")
	for i, finding := range report.KeyFindings {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, finding))
	}
	buf.WriteString("\n")

	buf.WriteString("--- Recommendations ---\n")
	for i, rec := range report.Recommendations {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
	}
	buf.WriteString("\n")

	buf.WriteString("--- Metrics ---\n")
	buf.WriteString(fmt.Sprintf("Duration: %v\n", report.Metrics.Duration))
	buf.WriteString(fmt.Sprintf("Total Rounds: %d\n", report.Metrics.TotalRounds))
	buf.WriteString(fmt.Sprintf("Quality Score: %.2f\n", report.Metrics.QualityScore))

	return buf.Bytes()
}

// GetReport retrieves a report by ID
func (drs *DebateReportingService) GetReport(ctx context.Context, reportID string) (*DebateReport, error) {
	report, exists := drs.reports.Get(reportID)
	if !exists {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}
	reportCopy := report.DebateReport
	return &reportCopy, nil
}

// GetExtendedReport retrieves the full extended report by ID
func (drs *DebateReportingService) GetExtendedReport(ctx context.Context, reportID string) (*ExtendedDebateReport, error) {
	report, exists := drs.reports.Get(reportID)
	if !exists {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}
	reportCopy := *report
	return &reportCopy, nil
}

// ListReports returns all report IDs
func (drs *DebateReportingService) ListReports() []string {
	return drs.reports.Keys()
}

// DeleteReport removes a report
func (drs *DebateReportingService) DeleteReport(ctx context.Context, reportID string) error {
	if _, ok := drs.reports.Delete(reportID); !ok {
		return fmt.Errorf("report not found: %s", reportID)
	}
	drs.logger.Infof("Deleted report %s", reportID)
	return nil
}
