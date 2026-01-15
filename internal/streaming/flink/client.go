package flink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Client provides an interface to interact with Apache Flink's REST API
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger
	mu         sync.RWMutex
	connected  bool
}

// NewClient creates a new Flink REST API client
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		logger:    logger,
		connected: false,
	}, nil
}

// Connect verifies connectivity to the Flink cluster
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.getOverviewLocked(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Flink cluster: %w", err)
	}

	c.connected = true
	c.logger.Info("Connected to Flink cluster")
	return nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// HealthCheck checks the health of the Flink cluster
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.GetOverview(ctx)
	return err
}

// ClusterOverview represents the Flink cluster overview
type ClusterOverview struct {
	FlinkVersion     string `json:"flink-version"`
	FlinkCommit      string `json:"flink-commit"`
	TaskManagers     int    `json:"taskmanagers"`
	SlotsTotal       int    `json:"slots-total"`
	SlotsAvailable   int    `json:"slots-available"`
	JobsRunning      int    `json:"jobs-running"`
	JobsFinished     int    `json:"jobs-finished"`
	JobsCancelled    int    `json:"jobs-cancelled"`
	JobsFailed       int    `json:"jobs-failed"`
}

// GetOverview returns the cluster overview
func (c *Client) GetOverview(ctx context.Context) (*ClusterOverview, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getOverviewLocked(ctx)
}

func (c *Client) getOverviewLocked(ctx context.Context) (*ClusterOverview, error) {
	url := fmt.Sprintf("%s/overview", c.config.GetRESTURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var overview ClusterOverview
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &overview, nil
}

// ClusterConfig represents a Flink configuration entry
type ClusterConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetConfig returns the cluster configuration
func (c *Client) GetConfig(ctx context.Context) ([]ClusterConfig, error) {
	url := fmt.Sprintf("%s/config", c.config.GetRESTURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var config []ClusterConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return config, nil
}

// TaskManager represents a Flink TaskManager
type TaskManager struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	DataPort      int    `json:"dataPort"`
	TimeSinceLastHeartbeat int64 `json:"timeSinceLastHeartbeat"`
	SlotsNumber   int    `json:"slotsNumber"`
	FreeSlots     int    `json:"freeSlots"`
	Hardware      struct {
		CPUCores            int   `json:"cpuCores"`
		PhysicalMemory      int64 `json:"physicalMemory"`
		FreeMemory          int64 `json:"freeMemory"`
		ManagedMemory       int64 `json:"managedMemory"`
	} `json:"hardware"`
}

// TaskManagerList represents a list of TaskManagers
type TaskManagerList struct {
	TaskManagers []TaskManager `json:"taskmanagers"`
}

// GetTaskManagers returns the list of TaskManagers
func (c *Client) GetTaskManagers(ctx context.Context) ([]TaskManager, error) {
	url := fmt.Sprintf("%s/taskmanagers", c.config.GetRESTURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var list TaskManagerList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return list.TaskManagers, nil
}

// Job represents a Flink job
type Job struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartTime int64  `json:"start-time"`
	EndTime   int64  `json:"end-time"`
	Duration  int64  `json:"duration"`
}

// JobList represents a list of jobs
type JobList struct {
	Jobs []Job `json:"jobs"`
}

// GetJobs returns the list of all jobs
func (c *Client) GetJobs(ctx context.Context) ([]Job, error) {
	url := fmt.Sprintf("%s/jobs", c.config.GetRESTURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var list JobList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return list.Jobs, nil
}

// GetRunningJobs returns only running jobs
func (c *Client) GetRunningJobs(ctx context.Context) ([]Job, error) {
	jobs, err := c.GetJobs(ctx)
	if err != nil {
		return nil, err
	}

	var running []Job
	for _, job := range jobs {
		if job.State == "RUNNING" {
			running = append(running, job)
		}
	}
	return running, nil
}

// JobDetails represents detailed information about a job
type JobDetails struct {
	JID              string            `json:"jid"`
	Name             string            `json:"name"`
	IsStoppable      bool              `json:"isStoppable"`
	State            string            `json:"state"`
	StartTime        int64             `json:"start-time"`
	EndTime          int64             `json:"end-time"`
	Duration         int64             `json:"duration"`
	Now              int64             `json:"now"`
	Timestamps       map[string]int64  `json:"timestamps"`
	Vertices         []JobVertex       `json:"vertices"`
	StatusCounts     map[string]int    `json:"status-counts"`
	Plan             JobPlan           `json:"plan"`
}

// JobVertex represents a vertex in a job
type JobVertex struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Parallelism int             `json:"parallelism"`
	Status      string          `json:"status"`
	StartTime   int64           `json:"start-time"`
	EndTime     int64           `json:"end-time"`
	Duration    int64           `json:"duration"`
	Tasks       map[string]int  `json:"tasks"`
	Metrics     JobVertexMetrics `json:"metrics"`
}

// JobVertexMetrics represents metrics for a job vertex
type JobVertexMetrics struct {
	ReadBytes        int64 `json:"read-bytes"`
	ReadBytesComplete bool `json:"read-bytes-complete"`
	WriteBytes       int64 `json:"write-bytes"`
	WriteBytesComplete bool `json:"write-bytes-complete"`
	ReadRecords      int64 `json:"read-records"`
	ReadRecordsComplete bool `json:"read-records-complete"`
	WriteRecords     int64 `json:"write-records"`
	WriteRecordsComplete bool `json:"write-records-complete"`
}

// JobPlan represents the execution plan of a job
type JobPlan struct {
	JID   string     `json:"jid"`
	Name  string     `json:"name"`
	Nodes []PlanNode `json:"nodes"`
}

// PlanNode represents a node in the job plan
type PlanNode struct {
	ID          string   `json:"id"`
	Parallelism int      `json:"parallelism"`
	Operator    string   `json:"operator"`
	OperatorStrategy string `json:"operator_strategy"`
	Description string   `json:"description"`
	Inputs      []PlanInput `json:"inputs"`
}

// PlanInput represents an input to a plan node
type PlanInput struct {
	Num        int    `json:"num"`
	ID         string `json:"id"`
	ShipStrategy string `json:"ship_strategy"`
	Exchange   string `json:"exchange"`
}

// GetJob returns details for a specific job
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobDetails, error) {
	url := fmt.Sprintf("%s/jobs/%s", c.config.GetRESTURL(), jobID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var details JobDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &details, nil
}

// JarInfo represents information about an uploaded JAR
type JarInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Uploaded int64 `json:"uploaded"`
	Entry []JarEntry `json:"entry"`
}

// JarEntry represents an entry class in a JAR
type JarEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// JarList represents a list of uploaded JARs
type JarList struct {
	Address string    `json:"address"`
	Files   []JarInfo `json:"files"`
}

// GetJars returns the list of uploaded JARs
func (c *Client) GetJars(ctx context.Context) ([]JarInfo, error) {
	url := fmt.Sprintf("%s/jars", c.config.GetRESTURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var list JarList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return list.Files, nil
}

// UploadJarResponse represents the response from uploading a JAR
type UploadJarResponse struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// UploadJar uploads a JAR file to the Flink cluster
func (c *Client) UploadJar(ctx context.Context, jarPath string) (*UploadJarResponse, error) {
	file, err := os.Open(jarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JAR file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("jarfile", filepath.Base(jarPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	url := fmt.Sprintf("%s/jars/upload", c.config.GetRESTURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var uploadResp UploadJarResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.WithField("filename", uploadResp.Filename).Info("JAR uploaded successfully")
	return &uploadResp, nil
}

// DeleteJar deletes an uploaded JAR
func (c *Client) DeleteJar(ctx context.Context, jarID string) error {
	url := fmt.Sprintf("%s/jars/%s", c.config.GetRESTURL(), jarID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	c.logger.WithField("jar_id", jarID).Info("JAR deleted successfully")
	return nil
}

// SubmitJobRequest represents a request to submit a job
type SubmitJobRequest struct {
	EntryClass           string   `json:"entryClass,omitempty"`
	Parallelism          int      `json:"parallelism,omitempty"`
	ProgramArgs          string   `json:"programArgs,omitempty"`
	SavepointPath        string   `json:"savepointPath,omitempty"`
	AllowNonRestoredState bool    `json:"allowNonRestoredState,omitempty"`
}

// SubmitJobResponse represents the response from submitting a job
type SubmitJobResponse struct {
	JobID string `json:"jobid"`
}

// SubmitJob submits a job from an uploaded JAR
func (c *Client) SubmitJob(ctx context.Context, jarID string, config *JobConfig) (string, error) {
	submitReq := SubmitJobRequest{
		Parallelism: config.Parallelism,
		AllowNonRestoredState: config.AllowNonRestoredState,
	}

	if config.EntryClass != "" {
		submitReq.EntryClass = config.EntryClass
	}

	if len(config.ProgramArgs) > 0 {
		args := ""
		for i, arg := range config.ProgramArgs {
			if i > 0 {
				args += " "
			}
			args += arg
		}
		submitReq.ProgramArgs = args
	}

	if config.SavepointPath != "" {
		submitReq.SavepointPath = config.SavepointPath
	}

	body, err := json.Marshal(submitReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jars/%s/run", c.config.GetRESTURL(), jarID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var submitResp SubmitJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"job_id": submitResp.JobID,
		"name":   config.Name,
	}).Info("Job submitted successfully")

	return submitResp.JobID, nil
}

// CancelJob cancels a running job
func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	url := fmt.Sprintf("%s/jobs/%s", c.config.GetRESTURL(), jobID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	c.logger.WithField("job_id", jobID).Info("Job cancelled successfully")
	return nil
}

// StopJobWithSavepoint stops a job with a savepoint
type StopJobRequest struct {
	TargetDirectory string `json:"targetDirectory,omitempty"`
	Drain           bool   `json:"drain,omitempty"`
}

type StopJobResponse struct {
	RequestID string `json:"request-id"`
}

// StopJobWithSavepoint stops a job and creates a savepoint
func (c *Client) StopJobWithSavepoint(ctx context.Context, jobID string, savepointDir string, drain bool) (string, error) {
	stopReq := StopJobRequest{
		TargetDirectory: savepointDir,
		Drain:           drain,
	}

	body, err := json.Marshal(stopReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jobs/%s/stop", c.config.GetRESTURL(), jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var stopResp StopJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&stopResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"job_id":     jobID,
		"request_id": stopResp.RequestID,
	}).Info("Job stop with savepoint initiated")

	return stopResp.RequestID, nil
}

// TriggerSavepoint triggers a savepoint for a running job
func (c *Client) TriggerSavepoint(ctx context.Context, jobID string, savepointDir string, cancelJob bool) (string, error) {
	reqBody := map[string]interface{}{
		"target-directory": savepointDir,
		"cancel-job":       cancelJob,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jobs/%s/savepoints", c.config.GetRESTURL(), jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var triggerResp struct {
		RequestID string `json:"request-id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"job_id":     jobID,
		"request_id": triggerResp.RequestID,
	}).Info("Savepoint triggered")

	return triggerResp.RequestID, nil
}

// Metrics represents Flink metrics
type Metric struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// GetJobManagerMetrics returns JobManager metrics
func (c *Client) GetJobManagerMetrics(ctx context.Context, metricNames ...string) ([]Metric, error) {
	url := fmt.Sprintf("%s/jobmanager/metrics", c.config.GetRESTURL())

	if len(metricNames) > 0 {
		url += "?get="
		for i, name := range metricNames {
			if i > 0 {
				url += ","
			}
			url += name
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var metrics []Metric
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return metrics, nil
}

// WaitForJobState waits for a job to reach a specific state
func (c *Client) WaitForJobState(ctx context.Context, jobID string, targetState string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for job %s to reach state %s", jobID, targetState)
			}

			job, err := c.GetJob(ctx, jobID)
			if err != nil {
				c.logger.WithError(err).Warn("Failed to get job status, retrying...")
				continue
			}

			if job.State == targetState {
				return nil
			}

			// Check for terminal failure states
			if job.State == "FAILED" || job.State == "CANCELED" {
				if targetState != job.State {
					return fmt.Errorf("job %s entered terminal state %s while waiting for %s", jobID, job.State, targetState)
				}
			}
		}
	}
}
