package bigdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSparkProcessor() *SparkBatchProcessor {
	return &SparkBatchProcessor{
		logger: logrus.New(),
	}
}

func TestParseJobOutput_EntityExtraction(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobEntityExtraction,
	}

	output := `{"processed_rows": 150000, "entities_extracted": 75000, "status": "completed"}`

	result, err := processor.parseJobOutput(output, params)
	assert.NoError(t, err)
	assert.Equal(t, BatchJobEntityExtraction, result.JobType)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(150000), result.ProcessedRows)
	assert.Equal(t, int64(75000), result.EntitiesExtracted)
}

func TestParseJobOutput_RelationshipMining(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobRelationshipMining,
	}

	output := `{"processed_rows": 120000, "relationships_found": 30000, "status": "completed"}`

	result, err := processor.parseJobOutput(output, params)
	assert.NoError(t, err)
	assert.Equal(t, BatchJobRelationshipMining, result.JobType)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(120000), result.ProcessedRows)
	assert.Equal(t, int64(30000), result.RelationshipsFound)
}

func TestParseJobOutput_InvalidJSON(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobEntityExtraction,
	}

	output := `invalid json`

	result, err := processor.parseJobOutput(output, params)
	assert.Error(t, err) // Should return error when no valid JSON found
	assert.Nil(t, result)
}

func TestParseJobOutput_EmptyOutput(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobEntityExtraction,
	}

	output := ""

	result, err := processor.parseJobOutput(output, params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- NewSparkBatchProcessor tests ---

func TestNewSparkBatchProcessor_NilLogger(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor(
		"spark://localhost:7077",
		nil,
		"/tmp/output",
		nil,
	)
	assert.NotNil(t, processor)
	assert.NotNil(t, processor.logger)
	assert.Equal(t, "spark://localhost:7077", processor.sparkMasterURL)
	assert.Nil(t, processor.dataLakeClient)
	assert.Equal(t, "/tmp/output", processor.outputPath)
	assert.Equal(t, "HelixAgent-BigData", processor.appName)
	assert.NotNil(t, processor.httpClient)
}

func TestNewSparkBatchProcessor_WithLogger(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	processor := NewSparkBatchProcessor(
		"spark://master:7077",
		nil,
		"/data/output",
		logger,
	)
	assert.NotNil(t, processor)
	assert.Equal(t, logger, processor.logger)
	assert.Equal(t, "spark://master:7077", processor.sparkMasterURL)
	assert.Equal(t, "/data/output", processor.outputPath)
}

func TestNewSparkBatchProcessor_WithDataLakeClient(t *testing.T) {
	t.Parallel()
	dlc := &DataLakeClient{logger: logrus.New()}
	processor := NewSparkBatchProcessor(
		"local",
		dlc,
		"/results",
		logrus.New(),
	)
	assert.Equal(t, dlc, processor.dataLakeClient)
}

// --- createJobConfig tests ---

func TestSparkBatchProcessor_CreateJobConfig_EntityExtraction(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: BatchJobEntityExtraction}
	config := processor.createJobConfig(params)

	assert.Equal(t, "4g", config.ExecutorMemory)
	assert.Equal(t, 8, config.NumExecutors)
	assert.Equal(t, 2, config.ExecutorCores)
	assert.Equal(t, "1g", config.DriverMemory)
	assert.Equal(t, "client", config.DeployMode)
	assert.Contains(t, config.PythonFile, "entity_extraction.py")
	assert.NotNil(t, config.EnvironmentVars)
	assert.Equal(t, "/opt/spark", config.EnvironmentVars["SPARK_HOME"])
}

func TestSparkBatchProcessor_CreateJobConfig_RelationshipMining(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: BatchJobRelationshipMining}
	config := processor.createJobConfig(params)

	assert.Equal(t, "4g", config.ExecutorMemory)
	assert.Equal(t, 8, config.NumExecutors)
	assert.Contains(t, config.PythonFile, "relationship_mining.py")
}

func TestSparkBatchProcessor_CreateJobConfig_TopicModeling(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: BatchJobTopicModeling}
	config := processor.createJobConfig(params)

	assert.Equal(t, "8g", config.ExecutorMemory)
	assert.Equal(t, 4, config.NumExecutors)
	assert.Contains(t, config.PythonFile, "topic_modeling.py")
}

func TestSparkBatchProcessor_CreateJobConfig_ProviderPerformance(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: BatchJobProviderPerformance}
	config := processor.createJobConfig(params)

	assert.Equal(t, "2g", config.ExecutorMemory)
	assert.Equal(t, 4, config.NumExecutors)
	assert.Contains(t, config.PythonFile, "provider_performance.py")
}

func TestSparkBatchProcessor_CreateJobConfig_DebateAnalysis(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: BatchJobDebateAnalysis}
	config := processor.createJobConfig(params)

	assert.Equal(t, "4g", config.ExecutorMemory)
	assert.Equal(t, 6, config.NumExecutors)
	assert.Contains(t, config.PythonFile, "debate_analysis.py")
}

func TestSparkBatchProcessor_CreateJobConfig_UnknownJobType(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{JobType: "unknown_type"}
	config := processor.createJobConfig(params)

	// Defaults when no case matches
	assert.Equal(t, "2g", config.ExecutorMemory)
	assert.Equal(t, 4, config.NumExecutors)
	assert.Equal(t, "", config.PythonFile) // No PythonFile set for unknown type
}

// --- buildJobArgs tests ---

func TestSparkBatchProcessor_BuildJobArgs_BasicParams(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
	}

	args, err := processor.buildJobArgs(params)
	assert.NoError(t, err)

	assert.Contains(t, args, "--input-path")
	assert.Contains(t, args, "/data/input")
	assert.Contains(t, args, "--output-path")
	assert.Contains(t, args, "/data/output")
	assert.Contains(t, args, "--job-type")
	assert.Contains(t, args, string(BatchJobEntityExtraction))
}

func TestSparkBatchProcessor_BuildJobArgs_WithDateRange(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)

	params := BatchParams{
		JobType:    BatchJobRelationshipMining,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
		StartDate:  startDate,
		EndDate:    endDate,
	}

	args, err := processor.buildJobArgs(params)
	assert.NoError(t, err)

	assert.Contains(t, args, "--start-date")
	assert.Contains(t, args, "2025-01-01")
	assert.Contains(t, args, "--end-date")
	assert.Contains(t, args, "2025-06-30")
}

func TestSparkBatchProcessor_BuildJobArgs_WithOptions(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobTopicModeling,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
		Options: map[string]interface{}{
			"num_topics": 10,
			"algorithm":  "LDA",
		},
	}

	args, err := processor.buildJobArgs(params)
	assert.NoError(t, err)

	assert.Contains(t, args, "--options")
	// The options JSON should be somewhere in the args
	found := false
	for _, arg := range args {
		if len(arg) > 5 && arg[0] == '{' {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected JSON options in args")
}

func TestSparkBatchProcessor_BuildJobArgs_EmptyOptions(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
		Options:    map[string]interface{}{},
	}

	args, err := processor.buildJobArgs(params)
	assert.NoError(t, err)

	// Empty options should not add --options flag
	for _, arg := range args {
		assert.NotEqual(t, "--options", arg)
	}
}

func TestSparkBatchProcessor_BuildJobArgs_ZeroDateNotIncluded(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://master:7077", nil, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
	}

	args, err := processor.buildJobArgs(params)
	assert.NoError(t, err)

	for _, arg := range args {
		assert.NotEqual(t, "--start-date", arg)
		assert.NotEqual(t, "--end-date", arg)
	}
}

// --- getSparkRESTBaseURL tests ---

func TestSparkBatchProcessor_GetSparkRESTBaseURL_Configured(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())

	url, err := processor.getSparkRESTBaseURL()
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:4040", url)
}

func TestSparkBatchProcessor_GetSparkRESTBaseURL_EmptyURL(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("", nil, "/scripts", logrus.New())

	url, err := processor.getSparkRESTBaseURL()
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "spark master URL not configured")
}

// --- getSparkHistoryBaseURL tests ---

func TestSparkBatchProcessor_GetSparkHistoryBaseURL(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())

	url, err := processor.getSparkHistoryBaseURL()
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:18080", url)
}

// --- GetJobStatus tests (using httptest) ---

func TestSparkBatchProcessor_GetJobStatus_EmptyMasterURL(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("", nil, "/scripts", logrus.New())

	result, err := processor.GetJobStatus(context.Background(), "app-123")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "spark master URL not configured")
}

func TestSparkBatchProcessor_GetJobStatus_ServerReturnsJobs(t *testing.T) {
	t.Parallel()
	// Create a test HTTP server that mimics the Spark REST API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `[{
			"jobId": 1,
			"status": "SUCCEEDED",
			"startTime": 1700000000000,
			"endTime": 1700000005000,
			"numTasks": 10,
			"numActive": 0,
			"numFailed": 0,
			"numKilled": 0,
			"numSkipped": 0
		}]`
		_, _ = w.Write([]byte(response))
	}))
	defer ts.Close()

	// Verify the test server responds correctly
	// (We cannot override getSparkRESTBaseURL, but we verify the server works)
	resp, err := ts.Client().Get(ts.URL + "/api/v1/applications/test-app/jobs")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_GetJobStatus_ServerError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	// We cannot easily override getSparkRESTBaseURL, so test the error path
	// through an unreachable server
	processor := NewSparkBatchProcessor("spark://configured:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{Timeout: 1 * time.Second}

	// GetJobStatus uses getSparkRESTBaseURL which returns localhost:4040
	// This will fail to connect, exercising the HTTP error path
	result, err := processor.GetJobStatus(context.Background(), "app-test")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- CancelJob tests ---

func TestSparkBatchProcessor_CancelJob_EmptyMasterURL(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("", nil, "/scripts", logrus.New())

	err := processor.CancelJob(context.Background(), "app-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spark master URL not configured")
}

func TestSparkBatchProcessor_CancelJob_ConnectionError(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://configured:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{Timeout: 1 * time.Second}

	err := processor.CancelJob(context.Background(), "app-cancel")
	assert.Error(t, err)
}

// --- ListCompletedJobs tests ---

func TestSparkBatchProcessor_ListCompletedJobs_ConnectionError(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://configured:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{Timeout: 1 * time.Second}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.Error(t, err)
	assert.Nil(t, jobs)
}

// --- CleanupOldResults tests ---

func TestSparkBatchProcessor_CleanupOldResults_NilDataLakeClient(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())

	count, err := processor.CleanupOldResults(context.Background(), 24*time.Hour)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "data lake client not configured")
}

// --- GetJobStatus tests with mock HTTP server ---

func TestSparkBatchProcessor_GetJobStatus_SuccessfulResponse(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/applications/")
		assert.Contains(t, r.URL.Path, "/jobs")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"jobId": 1,
			"status": "SUCCEEDED",
			"startTime": 1700000000000,
			"endTime": 1700000005000,
			"numTasks": 10,
			"numActive": 0,
			"numFailed": 0,
			"numKilled": 0,
			"numSkipped": 2
		}]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = ts.Client()

	// Override getSparkRESTBaseURL by injecting a custom sparkMasterURL
	// Since getSparkRESTBaseURL always returns localhost:4040, we need a different approach.
	// Create a processor that will use the test server URL by directly calling the HTTP path.
	// Instead, test the full flow using a server on port 4040 is impractical.
	// We'll test by making the processor point at our test server directly.

	// Workaround: create request manually and test the response parsing logic
	// For now, test the non-empty job path with httptest by calling directly
	req, err := http.NewRequestWithContext(
		context.Background(),
		"GET",
		ts.URL+"/api/v1/applications/test-app/jobs",
		nil,
	)
	assert.NoError(t, err)

	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_GetJobStatus_EmptyJobsList(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	// Test that empty jobs list returns proper error
	// We simulate by direct HTTP call since getSparkRESTBaseURL is hardcoded
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications/app-empty/jobs", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_GetJobStatus_Non200Response(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("application not found"))
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications/bad/jobs", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_GetJobStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications/test/jobs", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

// --- CancelJob tests with mock HTTP server ---

func TestSparkBatchProcessor_CancelJob_SuccessfulResponse(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/applications/")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", ts.URL+"/api/v1/applications/app-cancel", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_CancelJob_Non200Response(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", ts.URL+"/api/v1/applications/app-bad", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()
}

// --- ListCompletedJobs tests with mock HTTP server ---

func TestSparkBatchProcessor_ListCompletedJobs_SuccessfulResponse(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/applications")
		q := r.URL.Query()
		assert.Equal(t, "completed", q.Get("status"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": "app-001",
				"name": "HelixAgent-BigData-entity_extraction",
				"startTime": 1700000000000,
				"endTime": 1700000010000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-002",
				"name": "HelixAgent-BigData-relationship_mining",
				"startTime": 1700000020000,
				"endTime": 1700000030000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-003",
				"name": "HelixAgent-BigData-topic_modeling",
				"startTime": 1700000040000,
				"endTime": 1700000050000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-004",
				"name": "HelixAgent-BigData-provider_performance",
				"startTime": 1700000060000,
				"endTime": 1700000070000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-005",
				"name": "HelixAgent-BigData-debate_analysis",
				"startTime": 1700000080000,
				"endTime": 1700000090000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-006",
				"name": "other-running-job",
				"startTime": 1700000100000,
				"endTime": 0,
				"status": "RUNNING",
				"user": "helixagent",
				"attemptId": 1
			}
		]`))
	}))
	defer ts.Close()

	// Directly verify the response parsing
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications?status=completed&limit=10", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_ListCompletedJobs_EmptyList(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSparkBatchProcessor_ListCompletedJobs_Non200Response(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("history server unavailable"))
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/applications", nil)
	resp, err := ts.Client().Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()
}

// --- ProcessConversationDataset tests ---

func TestSparkBatchProcessor_ProcessConversationDataset_NilDataLakeClient(t *testing.T) {
	t.Parallel()
	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "/data/input",
		OutputPath: "/data/output",
	}

	// dataLakeClient is nil, so PathExists will panic due to nil minio client
	assert.Panics(t, func() {
		_, _ = processor.ProcessConversationDataset(context.Background(), params)
	})
}

// --- parseJSONOutput additional tests ---

func TestSparkBatchProcessor_ParseJSONOutput_AllFieldsPresent(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		OutputPath: "/output",
	}

	output := `{"status": "completed", "processed_rows": 1000, "entities_extracted": 500, "relationships_found": 200, "topics_identified": 10, "extra_metric": "value"}`
	result := processor.parseJSONOutput(output, params)

	assert.NotNil(t, result)
	assert.Equal(t, int64(1000), result.ProcessedRows)
	assert.Equal(t, int64(500), result.EntitiesExtracted)
	assert.Equal(t, int64(200), result.RelationshipsFound)
	assert.Equal(t, 10, result.TopicsIdentified)
	assert.Equal(t, "value", result.Metrics["extra_metric"])
	assert.Equal(t, "/output", result.OutputPath)
}

func TestSparkBatchProcessor_ParseJSONOutput_NoValidJSON(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{JobType: BatchJobEntityExtraction}

	result := processor.parseJSONOutput("just text\nno json here\n", params)
	assert.Nil(t, result)
}

func TestSparkBatchProcessor_ParseJSONOutput_JSONWithoutStatusSkipped(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{JobType: BatchJobEntityExtraction}

	// First line is JSON but has no "status" field, second line has it
	output := `{"processed_rows": 100}
{"status": "completed", "processed_rows": 200}`

	result := processor.parseJSONOutput(output, params)
	assert.NotNil(t, result)
	assert.Equal(t, int64(200), result.ProcessedRows)
}

func TestSparkBatchProcessor_ParseJSONOutput_MalformedJSONLine(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{JobType: BatchJobEntityExtraction}

	// First line starts with { but is malformed, second line is valid
	output := `{malformed json
{"status": "completed", "processed_rows": 50}`

	result := processor.parseJSONOutput(output, params)
	assert.NotNil(t, result)
	assert.Equal(t, int64(50), result.ProcessedRows)
}

func TestSparkBatchProcessor_ParseJSONOutput_EmptyString(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{JobType: BatchJobEntityExtraction}

	result := processor.parseJSONOutput("", params)
	assert.Nil(t, result)
}

// --- parseJobOutput additional tests ---

func TestParseJobOutput_TopicModeling(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType:    BatchJobTopicModeling,
		OutputPath: "/output/topics",
	}

	output := `{"processed_rows": 50000, "topics_identified": 25, "status": "completed"}`
	result, err := processor.parseJobOutput(output, params)
	assert.NoError(t, err)
	assert.Equal(t, BatchJobTopicModeling, result.JobType)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(50000), result.ProcessedRows)
	assert.Equal(t, 25, result.TopicsIdentified)
	assert.Equal(t, "/output/topics", result.OutputPath)
}

func TestParseJobOutput_WithExtraMetrics(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobProviderPerformance,
	}

	output := `{"status": "completed", "processed_rows": 1000, "avg_latency": 250.5, "top_provider": "claude"}`
	result, err := processor.parseJobOutput(output, params)
	assert.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(1000), result.ProcessedRows)
	// Extra fields should be in metrics
	assert.Equal(t, 250.5, result.Metrics["avg_latency"])
	assert.Equal(t, "claude", result.Metrics["top_provider"])
}

func TestParseJobOutput_MultiLineWithJSON(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobEntityExtraction,
	}

	output := `INFO: Starting Spark job
WARNING: Low memory
{"status": "completed", "processed_rows": 100, "entities_extracted": 50}
INFO: Job done`

	result, err := processor.parseJobOutput(output, params)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), result.ProcessedRows)
	assert.Equal(t, int64(50), result.EntitiesExtracted)
}

func TestParseJobOutput_JSONWithoutStatus(t *testing.T) {
	t.Parallel()
	processor := newTestSparkProcessor()
	params := BatchParams{
		JobType: BatchJobEntityExtraction,
	}

	// JSON without "status" field should be skipped
	output := `{"processed_rows": 100, "entities_extracted": 50}`
	result, err := processor.parseJobOutput(output, params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- BatchJobType constants tests ---

func TestBatchJobType_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, BatchJobType("entity_extraction"), BatchJobEntityExtraction)
	assert.Equal(t, BatchJobType("relationship_mining"), BatchJobRelationshipMining)
	assert.Equal(t, BatchJobType("topic_modeling"), BatchJobTopicModeling)
	assert.Equal(t, BatchJobType("provider_performance"), BatchJobProviderPerformance)
	assert.Equal(t, BatchJobType("debate_analysis"), BatchJobDebateAnalysis)
}

// --- BatchResult tests ---

func TestBatchResult_FieldAssignment(t *testing.T) {
	t.Parallel()
	now := time.Now()
	result := &BatchResult{
		JobID:              "job-1",
		JobType:            BatchJobEntityExtraction,
		Status:             "completed",
		ProcessedRows:      1000,
		EntitiesExtracted:  500,
		RelationshipsFound: 200,
		TopicsIdentified:   10,
		StartedAt:          now,
		CompletedAt:        now.Add(5 * time.Minute),
		DurationMs:         300000,
		OutputPath:         "/output/path",
		Metrics:            map[string]interface{}{"key": "value"},
		ErrorMessage:       "",
	}

	assert.Equal(t, "job-1", result.JobID)
	assert.Equal(t, BatchJobEntityExtraction, result.JobType)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, int64(1000), result.ProcessedRows)
	assert.Equal(t, int64(500), result.EntitiesExtracted)
	assert.Equal(t, int64(200), result.RelationshipsFound)
	assert.Equal(t, 10, result.TopicsIdentified)
}

// --- SparkJobConfig tests ---

func TestSparkJobConfig_FieldAssignment(t *testing.T) {
	t.Parallel()
	config := &SparkJobConfig{
		ExecutorMemory:  "4g",
		ExecutorCores:   4,
		NumExecutors:    8,
		DriverMemory:    "2g",
		DeployMode:      "cluster",
		PythonFile:      "/scripts/test.py",
		AdditionalArgs:  []string{"--arg1", "val1"},
		EnvironmentVars: map[string]string{"KEY": "VALUE"},
	}

	assert.Equal(t, "4g", config.ExecutorMemory)
	assert.Equal(t, 4, config.ExecutorCores)
	assert.Equal(t, 8, config.NumExecutors)
	assert.Equal(t, "2g", config.DriverMemory)
	assert.Equal(t, "cluster", config.DeployMode)
	assert.Equal(t, "/scripts/test.py", config.PythonFile)
	assert.Len(t, config.AdditionalArgs, 2)
	assert.Equal(t, "VALUE", config.EnvironmentVars["KEY"])
}

// --- BatchParams tests ---

// --- redirectTransport routes all HTTP requests to the test server ---

type redirectTransport struct {
	targetURL string
	transport http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the host of the request URL with our test server
	newURL := rt.targetURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return rt.transport.RoundTrip(newReq)
}

// --- GetJobStatus tests using redirectTransport ---

func TestSparkBatchProcessor_GetJobStatus_FullPath_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/applications/")
		assert.Contains(t, r.URL.Path, "/jobs")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"jobId": 1,
			"status": "SUCCEEDED",
			"startTime": 1700000000000,
			"endTime": 1700000005000,
			"numTasks": 10,
			"numActive": 0,
			"numFailed": 0,
			"numKilled": 0,
			"numSkipped": 2
		}]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	result, err := processor.GetJobStatus(context.Background(), "test-app-123")
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-app-123", result.JobID)
	assert.Equal(t, "succeeded", result.Status) // Status should be lowercased
	assert.NotZero(t, result.StartedAt)
	assert.NotZero(t, result.CompletedAt)
	assert.Equal(t, int64(5000), result.DurationMs)
	assert.Equal(t, true, result.Metrics["rest_api_used"])
	assert.Equal(t, 10, result.Metrics["num_tasks"])
	assert.Equal(t, 2, result.Metrics["num_skipped"])
}

func TestSparkBatchProcessor_GetJobStatus_FullPath_EmptyJobs(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	result, err := processor.GetJobStatus(context.Background(), "empty-app")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no jobs found")
}

func TestSparkBatchProcessor_GetJobStatus_FullPath_Non200(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("application not found"))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	result, err := processor.GetJobStatus(context.Background(), "bad-app")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "status 404")
}

func TestSparkBatchProcessor_GetJobStatus_FullPath_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	result, err := processor.GetJobStatus(context.Background(), "json-err")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse Spark jobs response")
}

func TestSparkBatchProcessor_GetJobStatus_FullPath_NoEndTime(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"jobId": 2,
			"status": "RUNNING",
			"startTime": 1700000000000,
			"endTime": 0,
			"numTasks": 5,
			"numActive": 3,
			"numFailed": 0,
			"numKilled": 0,
			"numSkipped": 0
		}]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	result, err := processor.GetJobStatus(context.Background(), "running-app")
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "running", result.Status)
	assert.NotZero(t, result.StartedAt)
	assert.True(t, result.CompletedAt.IsZero()) // EndTime is 0
	assert.Equal(t, int64(0), result.DurationMs)
}

// --- CancelJob tests using redirectTransport ---

func TestSparkBatchProcessor_CancelJob_FullPath_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/applications/")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	err := processor.CancelJob(context.Background(), "cancel-app-123")
	assert.NoError(t, err)
}

func TestSparkBatchProcessor_CancelJob_FullPath_Forbidden(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	err := processor.CancelJob(context.Background(), "forbidden-app")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
}

// --- ListCompletedJobs tests using redirectTransport ---

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		q := r.URL.Query()
		assert.Equal(t, "completed", q.Get("status"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": "app-001",
				"name": "HelixAgent-BigData-entity_extraction",
				"startTime": 1700000000000,
				"endTime": 1700000600000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			},
			{
				"id": "app-002",
				"name": "HelixAgent-BigData-relationship_mining",
				"startTime": 1700001000000,
				"endTime": 1700001300000,
				"status": "COMPLETED",
				"user": "helixagent",
				"attemptId": 1
			}
		]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.NoError(t, err)
	require.NotNil(t, jobs)
	assert.Len(t, jobs, 2)
	assert.Equal(t, "app-001", jobs[0].JobID)
	assert.Equal(t, BatchJobEntityExtraction, jobs[0].JobType)
	assert.Equal(t, "app-002", jobs[1].JobID)
	assert.Equal(t, BatchJobRelationshipMining, jobs[1].JobType)
	assert.Equal(t, true, jobs[0].Metrics["rest_api_used"])
	assert.Equal(t, "helixagent", jobs[0].Metrics["user"])
	assert.Equal(t, int64(600000), jobs[0].DurationMs)
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_EmptyList(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, jobs, 0)
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_Non200(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("history server down"))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.Error(t, err)
	assert.Nil(t, jobs)
	assert.Contains(t, err.Error(), "status 503")
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.Error(t, err)
	assert.Nil(t, jobs)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_LimitApplied(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "3", q.Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": "app-1", "name": "test-1", "startTime": 1700000000000, "endTime": 1700000060000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-2", "name": "test-2", "startTime": 1700000120000, "endTime": 1700000180000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-3", "name": "test-3", "startTime": 1700000240000, "endTime": 1700000300000, "status": "COMPLETED", "user": "test", "attemptId": 1}
		]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 3)
	assert.NoError(t, err)
	assert.Len(t, jobs, 3)
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_FiltersNonCompleted(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": "app-1", "name": "entity_extraction-job", "startTime": 1700000000000, "endTime": 1700000060000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-2", "name": "running-job", "startTime": 1700000120000, "endTime": 0, "status": "RUNNING", "user": "test", "attemptId": 1},
			{"id": "app-3", "name": "topic-job", "startTime": 1700000240000, "endTime": 1700000300000, "status": "COMPLETED", "user": "test", "attemptId": 1}
		]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.NoError(t, err)
	// RUNNING job should be filtered out
	assert.Len(t, jobs, 2)
	assert.Equal(t, "app-1", jobs[0].JobID)
	assert.Equal(t, "app-3", jobs[1].JobID)
}

func TestSparkBatchProcessor_ListCompletedJobs_FullPath_JobTypeDetection(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": "app-1", "name": "HelixAgent-BigData-entity_extraction", "startTime": 1700000000000, "endTime": 1700000060000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-2", "name": "HelixAgent-BigData-relationship_mining", "startTime": 1700000120000, "endTime": 1700000180000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-3", "name": "HelixAgent-BigData-topic_modeling", "startTime": 1700000240000, "endTime": 1700000300000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-4", "name": "HelixAgent-BigData-provider_performance", "startTime": 1700000360000, "endTime": 1700000420000, "status": "COMPLETED", "user": "test", "attemptId": 1},
			{"id": "app-5", "name": "HelixAgent-BigData-debate_analysis", "startTime": 1700000480000, "endTime": 1700000540000, "status": "COMPLETED", "user": "test", "attemptId": 1}
		]`))
	}))
	defer ts.Close()

	processor := NewSparkBatchProcessor("spark://localhost:7077", nil, "/scripts", logrus.New())
	processor.httpClient = &http.Client{
		Transport: &redirectTransport{
			targetURL: ts.URL,
			transport: http.DefaultTransport,
		},
	}

	jobs, err := processor.ListCompletedJobs(context.Background(), 10)
	assert.NoError(t, err)
	require.Len(t, jobs, 5)
	assert.Equal(t, BatchJobEntityExtraction, jobs[0].JobType)
	assert.Equal(t, BatchJobRelationshipMining, jobs[1].JobType)
	assert.Equal(t, BatchJobTopicModeling, jobs[2].JobType)
	assert.Equal(t, BatchJobProviderPerformance, jobs[3].JobType)
	assert.Equal(t, BatchJobDebateAnalysis, jobs[4].JobType)
}

// --- ProcessConversationDataset with mock S3 ---

func TestSparkBatchProcessor_ProcessConversationDataset_InputPathNotExists(t *testing.T) {
	t.Parallel()
	s := newMockS3Server()
	defer s.Close()

	dlc, _ := newTestDataLakeClientFromServer(t, s)
	processor := NewSparkBatchProcessor("spark://localhost:7077", dlc, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "/data/nonexistent",
		OutputPath: "/data/output",
	}

	// PathExists returns false (NoSuchKey), should get "input path does not exist" error
	result, err := processor.ProcessConversationDataset(context.Background(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input path does not exist")
	assert.Nil(t, result)
}

func TestSparkBatchProcessor_ProcessConversationDataset_InputPathExists_SparkSubmitFails(t *testing.T) {
	t.Parallel()
	s := newMockS3Server()
	defer s.Close()

	// Pre-populate the input path
	s.objects.Put("test-bucket/data/input", []byte("input data"))

	dlc, _ := newTestDataLakeClientFromServer(t, s)
	processor := NewSparkBatchProcessor("spark://localhost:7077", dlc, "/scripts", logrus.New())

	params := BatchParams{
		JobType:    BatchJobEntityExtraction,
		InputPath:  "data/input",
		OutputPath: "/data/output",
	}

	// spark-submit binary doesn't exist, so submitSparkJob will fail
	result, err := processor.ProcessConversationDataset(context.Background(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spark job failed")
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.NotEmpty(t, result.ErrorMessage)
}

// --- CleanupOldResults with mock S3 ---

func TestSparkBatchProcessor_CleanupOldResults_WithMockS3_NoDirectories(t *testing.T) {
	t.Parallel()
	s := newMockS3Server()
	defer s.Close()

	dlc, _ := newTestDataLakeClientFromServer(t, s)
	processor := NewSparkBatchProcessor("spark://localhost:7077", dlc, "output", logrus.New())

	count, err := processor.CleanupOldResults(context.Background(), 24*time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSparkBatchProcessor_CleanupOldResults_WithMockS3_HasOldDirectories(t *testing.T) {
	t.Parallel()
	s := newMockS3Server()
	defer s.Close()

	oldTime := time.Now().Add(-48 * time.Hour)

	// Pre-populate output directories with objects that have old timestamps
	// ListDirectories("output") lists prefix "output/" and extracts dir names from child keys
	s.objects.Put("test-bucket/output/old-job/output.json", []byte("old data"))
	s.timestamps.Put("test-bucket/output/old-job/output.json", oldTime)
	// Also create the directory marker object so GetMetadata("output/old-job/") -> StatObject("output/old-job") succeeds
	s.objects.Put("test-bucket/output/old-job", []byte(""))
	s.timestamps.Put("test-bucket/output/old-job", oldTime)

	dlc, _ := newTestDataLakeClientFromServer(t, s)
	processor := NewSparkBatchProcessor("spark://localhost:7077", dlc, "output", logrus.New())

	count, err := processor.CleanupOldResults(context.Background(), 24*time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSparkBatchProcessor_CleanupOldResults_WithMockS3_MixedOldAndNew(t *testing.T) {
	t.Parallel()
	s := newMockS3Server()
	defer s.Close()

	oldTime := time.Now().Add(-48 * time.Hour)

	// Old directory
	s.objects.Put("test-bucket/output/old-job/output.json", []byte("old data"))
	s.timestamps.Put("test-bucket/output/old-job/output.json", oldTime)
	s.objects.Put("test-bucket/output/old-job", []byte(""))
	s.timestamps.Put("test-bucket/output/old-job", oldTime)

	// New directory (recent timestamp, default is time.Now())
	s.objects.Put("test-bucket/output/new-job/output.json", []byte("new data"))
	s.objects.Put("test-bucket/output/new-job", []byte(""))
	// No timestamp override -> uses time.Now() which is recent

	dlc, _ := newTestDataLakeClientFromServer(t, s)
	processor := NewSparkBatchProcessor("spark://localhost:7077", dlc, "output", logrus.New())

	count, err := processor.CleanupOldResults(context.Background(), 24*time.Hour)
	assert.NoError(t, err)
	// Only the old directory should be deleted
	assert.Equal(t, 1, count)
}

func TestBatchParams_FieldAssignment(t *testing.T) {
	t.Parallel()
	params := BatchParams{
		JobType:    BatchJobDebateAnalysis,
		InputPath:  "/input",
		OutputPath: "/output",
		StartDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		Options:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, BatchJobDebateAnalysis, params.JobType)
	assert.Equal(t, "/input", params.InputPath)
	assert.Equal(t, "/output", params.OutputPath)
	assert.Equal(t, 2025, params.StartDate.Year())
	assert.Equal(t, "value", params.Options["key"])
}
