package services

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDebateResultForReport(debateID string, success bool, qualityScore float64) *DebateResult {
	now := time.Now()
	return &DebateResult{
		DebateID:        debateID,
		Topic:           "Test Debate Topic",
		StartTime:       now.Add(-time.Hour),
		EndTime:         now,
		Duration:        time.Hour,
		TotalRounds:     5,
		RoundsConducted: 5,
		QualityScore:    qualityScore,
		Success:         success,
		CogneeEnhanced:  true,
		MemoryUsed:      true,
		Participants: []ParticipantResponse{
			{ParticipantID: "p1", ParticipantName: "Participant 1"},
			{ParticipantID: "p2", ParticipantName: "Participant 2"},
		},
		AllResponses: []ParticipantResponse{
			{
				ParticipantID:   "p1",
				ParticipantName: "Participant 1",
				Round:           1,
				Response:        "This is a test response from participant 1",
				Confidence:      0.9,
				ResponseTime:    100 * time.Millisecond,
			},
			{
				ParticipantID:   "p2",
				ParticipantName: "Participant 2",
				Round:           1,
				Response:        "This is a test response from participant 2",
				Confidence:      0.85,
				ResponseTime:    150 * time.Millisecond,
			},
		},
		Consensus: &ConsensusResult{
			Achieved:       true,
			AgreementScore: 0.85,
			KeyPoints:      []string{"Point 1", "Point 2"},
		},
		QualityMetrics: &QualityMetrics{
			OverallScore: qualityScore,
			Coherence:    0.9,
			Relevance:    0.85,
			Accuracy:     0.88,
			Completeness: 0.82,
		},
		BestResponse: &ParticipantResponse{
			ParticipantID:   "p1",
			ParticipantName: "Participant 1",
			Confidence:      0.9,
		},
	}
}

func TestDebateReportingService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.reports)
	assert.NotNil(t, svc.templates)
	assert.Contains(t, svc.templates, "html")
	assert.Contains(t, svc.templates, "markdown")
	assert.Contains(t, svc.templates, "md")
}

func TestDebateReportingService_Generate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	t.Run("generate report for successful debate", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-success", true, 0.85)

		report, err := svc.GenerateReport(ctx, result)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.Contains(t, report.ReportID, "report-")
		assert.Equal(t, "debate-success", report.DebateID)
		assert.NotEmpty(t, report.Summary)
		assert.NotEmpty(t, report.KeyFindings)
		assert.NotEmpty(t, report.Recommendations)
	})

	t.Run("generate report for failed debate", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-failed", false, 0.4)

		report, err := svc.GenerateReport(ctx, result)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.Contains(t, report.Summary, "partial completion")
	})

	t.Run("generate report for nil result", func(t *testing.T) {
		report, err := svc.GenerateReport(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, report)
		assert.Contains(t, err.Error(), "debate result is required")
	})

	t.Run("report includes performance metrics", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-metrics", true, 0.9)

		report, err := svc.GenerateReport(ctx, result)
		assert.NoError(t, err)
		assert.Equal(t, result.Duration, report.Metrics.Duration)
		assert.Equal(t, result.TotalRounds, report.Metrics.TotalRounds)
		assert.Equal(t, result.QualityScore, report.Metrics.QualityScore)
	})
}

func TestDebateReportingService_GenerateSummary(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	tests := []struct {
		name           string
		success        bool
		cogneeEnhanced bool
		consensus      bool
		checkStrings   []string
	}{
		{
			name:           "successful with cognee and consensus",
			success:        true,
			cogneeEnhanced: true,
			consensus:      true,
			checkStrings:   []string{"successfully completed", "Cognee", "Consensus"},
		},
		{
			name:           "failed debate",
			success:        false,
			cogneeEnhanced: false,
			consensus:      false,
			checkStrings:   []string{"partial completion"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTestDebateResultForReport("debate-"+tt.name, tt.success, 0.85)
			result.CogneeEnhanced = tt.cogneeEnhanced
			if !tt.consensus {
				result.Consensus = nil
			}

			report, err := svc.GenerateReport(ctx, result)
			require.NoError(t, err)

			for _, str := range tt.checkStrings {
				assert.Contains(t, report.Summary, str)
			}
		})
	}
}

func TestDebateReportingService_GenerateKeyFindings(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	tests := []struct {
		name         string
		qualityScore float64
		expected     string
	}{
		{"high quality", 0.85, "High quality responses"},
		{"moderate quality", 0.6, "Moderate quality responses"},
		{"low quality", 0.3, "Low quality responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTestDebateResultForReport("debate-"+tt.name, true, tt.qualityScore)

			report, err := svc.GenerateReport(ctx, result)
			require.NoError(t, err)

			found := false
			for _, finding := range report.KeyFindings {
				if strings.Contains(finding, tt.expected) {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected finding '%s' not found in %v", tt.expected, report.KeyFindings)
		})
	}
}

func TestDebateReportingService_GenerateRecommendations(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	t.Run("low quality recommendations", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-low-quality", true, 0.3)

		report, err := svc.GenerateReport(ctx, result)
		require.NoError(t, err)

		found := false
		for _, rec := range report.Recommendations {
			if strings.Contains(rec, "more capable LLM") || strings.Contains(rec, "Increase the number") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("no consensus recommendations", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-no-consensus", true, 0.85)
		result.Consensus.Achieved = false

		report, err := svc.GenerateReport(ctx, result)
		require.NoError(t, err)

		found := false
		for _, rec := range report.Recommendations {
			if strings.Contains(rec, "mediator") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("long duration recommendations", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-long", true, 0.85)
		result.Duration = 10 * time.Minute

		report, err := svc.GenerateReport(ctx, result)
		require.NoError(t, err)

		found := false
		for _, rec := range report.Recommendations {
			if strings.Contains(rec, "Optimize provider response times") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("good performance recommendation", func(t *testing.T) {
		result := createTestDebateResultForReport("debate-good", true, 0.9)
		result.Duration = 2 * time.Minute
		result.CogneeEnhanced = true
		result.MemoryUsed = true

		report, err := svc.GenerateReport(ctx, result)
		require.NoError(t, err)

		found := false
		for _, rec := range report.Recommendations {
			if strings.Contains(rec, "performed well") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDebateReportingService_Export(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-export", true, 0.85)
	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	t.Run("export as JSON", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "json")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		var exported map[string]interface{}
		err = json.Unmarshal(data, &exported)
		assert.NoError(t, err)
		assert.Equal(t, report.ReportID, exported["report_id"])
	})

	t.Run("export as HTML", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "html")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "<!DOCTYPE html>")
		assert.Contains(t, string(data), report.ReportID)
	})

	t.Run("export as Markdown", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "markdown")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "# Debate Report")
		assert.Contains(t, string(data), report.ReportID)
	})

	t.Run("export as md (alias for markdown)", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "md")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "# Debate Report")
	})

	t.Run("export as text", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "text")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "=== DEBATE REPORT ===")
	})

	t.Run("export as txt", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("export with unsupported format", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, report.ReportID, "pdf")
		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "unsupported format")
	})

	t.Run("export non-existing report", func(t *testing.T) {
		data, err := svc.ExportReport(ctx, "nonexistent", "json")
		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "report not found")
	})
}

func TestDebateReportingService_GetReport(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-get", true, 0.85)
	generated, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	t.Run("get existing report", func(t *testing.T) {
		report, err := svc.GetReport(ctx, generated.ReportID)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.Equal(t, generated.ReportID, report.ReportID)
	})

	t.Run("get non-existing report", func(t *testing.T) {
		report, err := svc.GetReport(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, report)
		assert.Contains(t, err.Error(), "report not found")
	})
}

func TestDebateReportingService_GetExtendedReport(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-extended", true, 0.85)
	generated, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	t.Run("get extended report", func(t *testing.T) {
		report, err := svc.GetExtendedReport(ctx, generated.ReportID)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotEmpty(t, report.Participants)
		assert.NotNil(t, report.Consensus)
		assert.NotNil(t, report.Quality)
	})

	t.Run("get non-existing extended report", func(t *testing.T) {
		report, err := svc.GetExtendedReport(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, report)
	})
}

func TestDebateReportingService_ListReports(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		reports := svc.ListReports()
		assert.Empty(t, reports)
	})

	t.Run("list multiple reports", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			result := createTestDebateResultForReport("debate-list-"+string(rune('A'+i)), true, 0.85)
			_, err := svc.GenerateReport(ctx, result)
			require.NoError(t, err)
		}

		reports := svc.ListReports()
		assert.Equal(t, 5, len(reports))
	})
}

func TestDebateReportingService_DeleteReport(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-delete", true, 0.85)
	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	t.Run("delete existing report", func(t *testing.T) {
		err := svc.DeleteReport(ctx, report.ReportID)
		assert.NoError(t, err)

		_, err = svc.GetReport(ctx, report.ReportID)
		assert.Error(t, err)
	})

	t.Run("delete non-existing report", func(t *testing.T) {
		err := svc.DeleteReport(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "report not found")
	})
}

func TestDebateReportingService_ParticipantReports(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-participants", true, 0.85)
	// Add more responses
	result.AllResponses = append(result.AllResponses, ParticipantResponse{
		ParticipantID:   "p1",
		ParticipantName: "Participant 1",
		Round:           2,
		Response:        "Second response from participant 1",
		Confidence:      0.85,
		ResponseTime:    200 * time.Millisecond,
	})
	result.AllResponses = append(result.AllResponses, ParticipantResponse{
		ParticipantID:   "p2",
		ParticipantName: "Participant 2",
		Round:           2,
		Response:        "", // Empty response - should count as error
		Confidence:      0.05,
		ResponseTime:    100 * time.Millisecond,
	})

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	assert.Equal(t, 2, len(extReport.Participants))

	// Check participant stats
	for _, p := range extReport.Participants {
		assert.NotEmpty(t, p.ID)
		assert.NotEmpty(t, p.Name)
		assert.Greater(t, p.ResponseCount, 0)
		assert.GreaterOrEqual(t, p.AverageConfidence, float64(0))
	}
}

func TestDebateReportingService_QualityReport(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-quality", true, 0.85)

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	assert.NotNil(t, extReport.Quality)
	assert.Equal(t, result.QualityMetrics.OverallScore, extReport.Quality.OverallScore)
	assert.Equal(t, result.QualityMetrics.Coherence, extReport.Quality.CoherenceScore)
	assert.Equal(t, result.QualityMetrics.Relevance, extReport.Quality.RelevanceScore)
}

func TestDebateReportingService_ConsensusReport(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-consensus", true, 0.85)

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	assert.NotNil(t, extReport.Consensus)
	assert.True(t, extReport.Consensus.ConsensusReached)
	assert.Equal(t, result.Consensus.AgreementScore, extReport.Consensus.AgreementLevel)
	assert.Equal(t, result.Consensus.KeyPoints, extReport.Consensus.KeyAgreements)
}

func TestDebateReportingService_NilConsensusAndQualityMetrics(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-nil-extras", true, 0.85)
	result.Consensus = nil
	result.QualityMetrics = nil

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	assert.Nil(t, extReport.Consensus)
	assert.Nil(t, extReport.Quality)
}

func TestDebateReportingService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 20

	// Concurrent report generation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result := createTestDebateResultForReport("debate-concurrent-"+string(rune('A'+id)), true, 0.85)
			_, _ = svc.GenerateReport(ctx, result)
		}(i)
	}

	wg.Wait()

	// Concurrent reads
	reports := svc.ListReports()
	for _, reportID := range reports {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, _ = svc.GetReport(ctx, id)
			_, _ = svc.ExportReport(ctx, id, "json")
		}(reportID)
	}

	wg.Wait()
}

func TestParticipantReport_Structure(t *testing.T) {
	report := &ParticipantReport{
		ID:                "p-123",
		Name:              "Test Participant",
		ResponseCount:     10,
		AverageConfidence: 0.85,
		TotalTokens:       5000,
		AverageLatency:    200 * time.Millisecond,
		ErrorCount:        1,
		TopContributions:  []string{"contribution 1", "contribution 2"},
	}

	assert.Equal(t, "p-123", report.ID)
	assert.Equal(t, "Test Participant", report.Name)
	assert.Equal(t, 10, report.ResponseCount)
	assert.Equal(t, 0.85, report.AverageConfidence)
	assert.Equal(t, 1, report.ErrorCount)
	assert.Len(t, report.TopContributions, 2)
}

func TestConsensusReport_Structure(t *testing.T) {
	report := &ConsensusReport{
		ConsensusReached: true,
		AgreementLevel:   0.9,
		DissenterCount:   1,
		KeyAgreements:    []string{"agreement 1", "agreement 2"},
		KeyDisagreements: []string{"disagreement 1"},
	}

	assert.True(t, report.ConsensusReached)
	assert.Equal(t, 0.9, report.AgreementLevel)
	assert.Equal(t, 1, report.DissenterCount)
	assert.Len(t, report.KeyAgreements, 2)
	assert.Len(t, report.KeyDisagreements, 1)
}

func TestQualityReport_Structure(t *testing.T) {
	report := &QualityReport{
		OverallScore:    0.85,
		CoherenceScore:  0.9,
		RelevanceScore:  0.88,
		DepthScore:      0.82,
		FactualityScore: 0.87,
		NoveltyScore:    0.75,
	}

	assert.Equal(t, 0.85, report.OverallScore)
	assert.Equal(t, 0.9, report.CoherenceScore)
	assert.Equal(t, 0.88, report.RelevanceScore)
	assert.Equal(t, 0.82, report.DepthScore)
	assert.Equal(t, 0.87, report.FactualityScore)
	assert.Equal(t, 0.75, report.NoveltyScore)
}

func TestExtendedDebateReport_Structure(t *testing.T) {
	now := time.Now()
	report := &ExtendedDebateReport{
		DebateReport: DebateReport{
			ReportID:    "report-123",
			DebateID:    "debate-456",
			GeneratedAt: now,
			Summary:     "Test summary",
			KeyFindings: []string{"finding 1", "finding 2"},
		},
		Participants: []ParticipantReport{
			{ID: "p1", Name: "Participant 1"},
		},
		Consensus: &ConsensusReport{ConsensusReached: true},
		Quality:   &QualityReport{OverallScore: 0.85},
	}

	assert.Equal(t, "report-123", report.ReportID)
	assert.Equal(t, "debate-456", report.DebateID)
	assert.Len(t, report.Participants, 1)
	assert.NotNil(t, report.Consensus)
	assert.NotNil(t, report.Quality)
}

func TestDebateReportingService_TopContributionsLimit(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-contributions", true, 0.85)
	// Add many high-confidence responses
	for i := 0; i < 10; i++ {
		result.AllResponses = append(result.AllResponses, ParticipantResponse{
			ParticipantID:   "p1",
			ParticipantName: "Participant 1",
			Round:           i + 1,
			Response:        "High confidence response " + string(rune('A'+i)),
			Confidence:      0.9,
			ResponseTime:    100 * time.Millisecond,
		})
	}

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	for _, p := range extReport.Participants {
		if p.ID == "p1" {
			assert.LessOrEqual(t, len(p.TopContributions), 3)
		}
	}
}

func TestDebateReportingService_LongResponseTruncation(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateReportingService(logger)
	ctx := context.Background()

	result := createTestDebateResultForReport("debate-long-response", true, 0.85)
	// Add a very long response
	longResponse := strings.Repeat("a", 200)
	result.AllResponses = append(result.AllResponses, ParticipantResponse{
		ParticipantID:   "p1",
		ParticipantName: "Participant 1",
		Round:           1,
		Response:        longResponse,
		Confidence:      0.9,
		ResponseTime:    100 * time.Millisecond,
	})

	report, err := svc.GenerateReport(ctx, result)
	require.NoError(t, err)

	extReport, err := svc.GetExtendedReport(ctx, report.ReportID)
	require.NoError(t, err)

	for _, p := range extReport.Participants {
		if p.ID == "p1" && len(p.TopContributions) > 0 {
			// Long responses should be truncated and have "..."
			for _, contrib := range p.TopContributions {
				if len(contrib) > 100 {
					assert.True(t, strings.HasSuffix(contrib, "..."))
				}
			}
		}
	}
}
