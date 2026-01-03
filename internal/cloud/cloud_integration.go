package cloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// CloudProvider represents a cloud AI provider
type CloudProvider interface {
	ListModels(ctx context.Context) ([]map[string]interface{}, error)
	InvokeModel(ctx context.Context, modelName, prompt string, config map[string]interface{}) (string, error)
	GetProviderName() string
	HealthCheck(ctx context.Context) error
}

// ========== AWS Bedrock Integration ==========

// AWSBedrockIntegration provides AWS Bedrock AI service integration
type AWSBedrockIntegration struct {
	region           string
	accessKeyID      string
	secretAccessKey  string
	sessionToken     string
	endpointOverride string
	httpClient       *http.Client
	logger           *logrus.Logger
}

// AWSBedrockConfig holds configuration for AWS Bedrock
type AWSBedrockConfig struct {
	Region           string
	AccessKeyID      string
	SecretAccessKey  string
	SessionToken     string
	Timeout          time.Duration
	EndpointOverride string // For testing with mock servers
	HTTPClient       *http.Client
}

// NewAWSBedrockIntegration creates a new AWS Bedrock integration
func NewAWSBedrockIntegration(region string, logger *logrus.Logger) *AWSBedrockIntegration {
	return NewAWSBedrockIntegrationWithConfig(AWSBedrockConfig{
		Region:          region,
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		Timeout:         60 * time.Second,
	}, logger)
}

// NewAWSBedrockIntegrationWithConfig creates AWS Bedrock integration with explicit config
func NewAWSBedrockIntegrationWithConfig(config AWSBedrockConfig, logger *logrus.Logger) *AWSBedrockIntegration {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	return &AWSBedrockIntegration{
		region:           config.Region,
		accessKeyID:      config.AccessKeyID,
		secretAccessKey:  config.SecretAccessKey,
		sessionToken:     config.SessionToken,
		endpointOverride: config.EndpointOverride,
		httpClient:       httpClient,
		logger:           logger,
	}
}

// ListModels lists available Bedrock models
func (a *AWSBedrockIntegration) ListModels(ctx context.Context) ([]map[string]interface{}, error) {
	if a.accessKeyID == "" || a.secretAccessKey == "" {
		return nil, fmt.Errorf("AWS credentials not configured")
	}

	var endpoint string
	if a.endpointOverride != "" {
		endpoint = a.endpointOverride + "/foundation-models"
	} else {
		endpoint = fmt.Sprintf("https://bedrock.%s.amazonaws.com/foundation-models", a.region)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Sign the request with AWS Signature V4
	if err := a.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AWS Bedrock API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		ModelSummaries []struct {
			ModelID      string `json:"modelId"`
			ModelName    string `json:"modelName"`
			ProviderName string `json:"providerName"`
			ModelArn     string `json:"modelArn"`
		} `json:"modelSummaries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]map[string]interface{}, 0, len(result.ModelSummaries))
	for _, m := range result.ModelSummaries {
		models = append(models, map[string]interface{}{
			"name":         m.ModelID,
			"display_name": m.ModelName,
			"provider":     m.ProviderName,
			"arn":          m.ModelArn,
		})
	}

	a.logger.WithField("count", len(models)).Info("Listed AWS Bedrock models")
	return models, nil
}

// InvokeModel invokes a Bedrock model
func (a *AWSBedrockIntegration) InvokeModel(ctx context.Context, modelId, prompt string, config map[string]interface{}) (string, error) {
	if a.accessKeyID == "" || a.secretAccessKey == "" {
		return "", fmt.Errorf("AWS credentials not configured")
	}

	var endpoint string
	if a.endpointOverride != "" {
		endpoint = a.endpointOverride + "/model/" + modelId + "/invoke"
	} else {
		endpoint = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke", a.region, modelId)
	}

	// Build request body based on model type
	var requestBody map[string]interface{}

	// Determine model type and build appropriate request
	if strings.Contains(modelId, "anthropic") {
		// Anthropic Claude models on Bedrock
		requestBody = map[string]interface{}{
			"anthropic_version": "bedrock-2023-05-31",
			"max_tokens":        getIntConfig(config, "max_tokens", 1024),
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		}
	} else if strings.Contains(modelId, "amazon.titan") {
		// Amazon Titan models
		requestBody = map[string]interface{}{
			"inputText": prompt,
			"textGenerationConfig": map[string]interface{}{
				"maxTokenCount": getIntConfig(config, "max_tokens", 1024),
				"temperature":   getFloatConfig(config, "temperature", 0.7),
				"topP":          getFloatConfig(config, "top_p", 0.9),
			},
		}
	} else if strings.Contains(modelId, "meta.llama") {
		// Meta Llama models
		requestBody = map[string]interface{}{
			"prompt":      prompt,
			"max_gen_len": getIntConfig(config, "max_tokens", 1024),
			"temperature": getFloatConfig(config, "temperature", 0.7),
			"top_p":       getFloatConfig(config, "top_p", 0.9),
		}
	} else if strings.Contains(modelId, "cohere") {
		// Cohere models
		requestBody = map[string]interface{}{
			"prompt":      prompt,
			"max_tokens":  getIntConfig(config, "max_tokens", 1024),
			"temperature": getFloatConfig(config, "temperature", 0.7),
		}
	} else {
		// Default format (try Anthropic-like)
		requestBody = map[string]interface{}{
			"prompt":     prompt,
			"max_tokens": getIntConfig(config, "max_tokens", 1024),
		}
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Sign the request with AWS Signature V4
	if err := a.signRequest(req, body); err != nil {
		return "", fmt.Errorf("failed to sign request: %w", err)
	}

	startTime := time.Now()
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to invoke model: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(startTime)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AWS Bedrock API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse response based on model type
	var content string
	if strings.Contains(modelId, "anthropic") {
		var claudeResp struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(respBody, &claudeResp); err == nil && len(claudeResp.Content) > 0 {
			content = claudeResp.Content[0].Text
		}
	} else if strings.Contains(modelId, "amazon.titan") {
		var titanResp struct {
			Results []struct {
				OutputText string `json:"outputText"`
			} `json:"results"`
		}
		if err := json.Unmarshal(respBody, &titanResp); err == nil && len(titanResp.Results) > 0 {
			content = titanResp.Results[0].OutputText
		}
	} else if strings.Contains(modelId, "meta.llama") {
		var llamaResp struct {
			Generation string `json:"generation"`
		}
		if err := json.Unmarshal(respBody, &llamaResp); err == nil {
			content = llamaResp.Generation
		}
	} else if strings.Contains(modelId, "cohere") {
		var cohereResp struct {
			Generations []struct {
				Text string `json:"text"`
			} `json:"generations"`
		}
		if err := json.Unmarshal(respBody, &cohereResp); err == nil && len(cohereResp.Generations) > 0 {
			content = cohereResp.Generations[0].Text
		}
	}

	// Fallback: try to extract any text field
	if content == "" {
		var generic map[string]interface{}
		if err := json.Unmarshal(respBody, &generic); err == nil {
			content = extractTextFromResponse(generic)
		}
	}

	if content == "" {
		content = string(respBody)
	}

	a.logger.WithFields(logrus.Fields{
		"model":    modelId,
		"region":   a.region,
		"duration": duration,
	}).Info("AWS Bedrock model invoked successfully")

	return content, nil
}

// signRequest signs an HTTP request with AWS Signature V4
func (a *AWSBedrockIntegration) signRequest(req *http.Request, body []byte) error {
	// AWS Signature V4 implementation
	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	// Set required headers
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	if a.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", a.sessionToken)
	}

	// Create canonical request
	method := req.Method
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuerystring := req.URL.RawQuery

	// Canonical headers
	signedHeaders := []string{"host", "x-amz-date"}
	if a.sessionToken != "" {
		signedHeaders = append(signedHeaders, "x-amz-security-token")
	}
	if req.Header.Get("Content-Type") != "" {
		signedHeaders = append(signedHeaders, "content-type")
	}
	sort.Strings(signedHeaders)

	canonicalHeaders := ""
	for _, h := range signedHeaders {
		canonicalHeaders += strings.ToLower(h) + ":" + strings.TrimSpace(req.Header.Get(h)) + "\n"
	}
	signedHeadersStr := strings.Join(signedHeaders, ";")

	// Payload hash
	var payloadHash string
	if body != nil {
		hash := sha256.Sum256(body)
		payloadHash = hex.EncodeToString(hash[:])
	} else {
		hash := sha256.Sum256([]byte(""))
		payloadHash = hex.EncodeToString(hash[:])
	}
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalRequest := method + "\n" + canonicalURI + "\n" + canonicalQuerystring + "\n" +
		canonicalHeaders + "\n" + signedHeadersStr + "\n" + payloadHash

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	service := "bedrock"
	credentialScope := dateStamp + "/" + a.region + "/" + service + "/aws4_request"

	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := algorithm + "\n" + amzDate + "\n" + credentialScope + "\n" +
		hex.EncodeToString(canonicalRequestHash[:])

	// Calculate signature
	signingKey := a.getSignatureKey(dateStamp, a.region, service)
	signature := hmacSHA256(signingKey, stringToSign)
	signatureHex := hex.EncodeToString(signature)

	// Add authorization header
	authHeader := algorithm + " Credential=" + a.accessKeyID + "/" + credentialScope +
		", SignedHeaders=" + signedHeadersStr + ", Signature=" + signatureHex
	req.Header.Set("Authorization", authHeader)

	return nil
}

func (a *AWSBedrockIntegration) getSignatureKey(dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+a.secretAccessKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// HealthCheck checks AWS Bedrock connectivity
func (a *AWSBedrockIntegration) HealthCheck(ctx context.Context) error {
	if a.accessKeyID == "" || a.secretAccessKey == "" {
		return fmt.Errorf("AWS credentials not configured")
	}

	_, err := a.ListModels(ctx)
	return err
}

// GetProviderName returns the provider name
func (a *AWSBedrockIntegration) GetProviderName() string {
	return "aws-bedrock"
}

// ========== GCP Vertex AI Integration ==========

// GCPVertexAIIntegration provides Google Cloud Vertex AI integration
type GCPVertexAIIntegration struct {
	projectID        string
	location         string
	accessToken      string
	endpointOverride string
	httpClient       *http.Client
	logger           *logrus.Logger
}

// GCPVertexAIConfig holds configuration for GCP Vertex AI
type GCPVertexAIConfig struct {
	ProjectID        string
	Location         string
	AccessToken      string
	Timeout          time.Duration
	EndpointOverride string // For testing with mock servers
	HTTPClient       *http.Client
}

// NewGCPVertexAIIntegration creates a new GCP Vertex AI integration
func NewGCPVertexAIIntegration(projectID, location string, logger *logrus.Logger) *GCPVertexAIIntegration {
	return NewGCPVertexAIIntegrationWithConfig(GCPVertexAIConfig{
		ProjectID:   projectID,
		Location:    location,
		AccessToken: os.Getenv("GOOGLE_ACCESS_TOKEN"),
		Timeout:     60 * time.Second,
	}, logger)
}

// NewGCPVertexAIIntegrationWithConfig creates GCP Vertex AI integration with explicit config
func NewGCPVertexAIIntegrationWithConfig(config GCPVertexAIConfig, logger *logrus.Logger) *GCPVertexAIIntegration {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	location := config.Location
	if location == "" {
		location = "us-central1"
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	return &GCPVertexAIIntegration{
		projectID:        config.ProjectID,
		location:         location,
		accessToken:      config.AccessToken,
		endpointOverride: config.EndpointOverride,
		httpClient:       httpClient,
		logger:           logger,
	}
}

// ListModels lists available Vertex AI models
func (g *GCPVertexAIIntegration) ListModels(ctx context.Context) ([]map[string]interface{}, error) {
	if g.accessToken == "" {
		return nil, fmt.Errorf("GCP access token not configured")
	}

	var endpoint string
	if g.endpointOverride != "" {
		endpoint = g.endpointOverride + "/v1/projects/" + g.projectID + "/locations/" + g.location + "/publishers/google/models"
	} else {
		endpoint = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models",
			g.location, g.projectID, g.location)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GCP Vertex AI API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]map[string]interface{}, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, map[string]interface{}{
			"name":         m.Name,
			"display_name": m.DisplayName,
			"description":  m.Description,
			"provider":     "gcp",
		})
	}

	g.logger.WithField("count", len(models)).Info("Listed GCP Vertex AI models")
	return models, nil
}

// InvokeModel invokes a Vertex AI model
func (g *GCPVertexAIIntegration) InvokeModel(ctx context.Context, modelName, prompt string, config map[string]interface{}) (string, error) {
	if g.accessToken == "" {
		return "", fmt.Errorf("GCP access token not configured")
	}

	// Vertex AI endpoint for text generation
	var endpoint string
	if g.endpointOverride != "" {
		endpoint = g.endpointOverride + "/v1/projects/" + g.projectID + "/locations/" + g.location + "/publishers/google/models/" + modelName + ":predict"
	} else {
		endpoint = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			g.location, g.projectID, g.location, modelName)
	}

	// Build request body for Vertex AI
	requestBody := map[string]interface{}{
		"instances": []map[string]interface{}{
			{"prompt": prompt},
		},
		"parameters": map[string]interface{}{
			"temperature":     getFloatConfig(config, "temperature", 0.7),
			"maxOutputTokens": getIntConfig(config, "max_tokens", 1024),
			"topP":            getFloatConfig(config, "top_p", 0.9),
			"topK":            getIntConfig(config, "top_k", 40),
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.accessToken)
	req.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to invoke model: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(startTime)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GCP Vertex AI API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse Vertex AI response
	var vertexResp struct {
		Predictions []struct {
			Content string `json:"content"`
		} `json:"predictions"`
	}

	if err := json.Unmarshal(respBody, &vertexResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	var content string
	if len(vertexResp.Predictions) > 0 {
		content = vertexResp.Predictions[0].Content
	}

	// Fallback: extract from generic response
	if content == "" {
		var generic map[string]interface{}
		if err := json.Unmarshal(respBody, &generic); err == nil {
			content = extractTextFromResponse(generic)
		}
	}

	if content == "" {
		content = string(respBody)
	}

	g.logger.WithFields(logrus.Fields{
		"model":    modelName,
		"project":  g.projectID,
		"location": g.location,
		"duration": duration,
	}).Info("GCP Vertex AI model invoked successfully")

	return content, nil
}

// HealthCheck checks GCP Vertex AI connectivity
func (g *GCPVertexAIIntegration) HealthCheck(ctx context.Context) error {
	if g.accessToken == "" {
		return fmt.Errorf("GCP access token not configured")
	}

	// Try a simple API call
	var endpoint string
	if g.endpointOverride != "" {
		endpoint = g.endpointOverride + "/v1/projects/" + g.projectID + "/locations/" + g.location
	} else {
		endpoint = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s",
			g.location, g.projectID, g.location)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.accessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetProviderName returns the provider name
func (g *GCPVertexAIIntegration) GetProviderName() string {
	return "gcp-vertex-ai"
}

// ========== Azure OpenAI Integration ==========

// AzureOpenAIIntegration provides Azure OpenAI integration
type AzureOpenAIIntegration struct {
	endpoint   string
	apiKey     string
	apiVersion string
	httpClient *http.Client
	logger     *logrus.Logger
}

// AzureOpenAIConfig holds configuration for Azure OpenAI
type AzureOpenAIConfig struct {
	Endpoint   string
	APIKey     string
	APIVersion string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// NewAzureOpenAIIntegration creates a new Azure OpenAI integration
func NewAzureOpenAIIntegration(endpoint string, logger *logrus.Logger) *AzureOpenAIIntegration {
	return NewAzureOpenAIIntegrationWithConfig(AzureOpenAIConfig{
		Endpoint:   endpoint,
		APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
		APIVersion: os.Getenv("AZURE_OPENAI_API_VERSION"),
		Timeout:    60 * time.Second,
	}, logger)
}

// NewAzureOpenAIIntegrationWithConfig creates Azure OpenAI integration with explicit config
func NewAzureOpenAIIntegrationWithConfig(config AzureOpenAIConfig, logger *logrus.Logger) *AzureOpenAIIntegration {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	apiVersion := config.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-01"
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	return &AzureOpenAIIntegration{
		endpoint:   strings.TrimSuffix(config.Endpoint, "/"),
		apiKey:     config.APIKey,
		apiVersion: apiVersion,
		httpClient: httpClient,
		logger:     logger,
	}
}

// ListModels lists available Azure OpenAI models
func (az *AzureOpenAIIntegration) ListModels(ctx context.Context) ([]map[string]interface{}, error) {
	if az.apiKey == "" {
		return nil, fmt.Errorf("Azure OpenAI API key not configured")
	}

	endpoint := fmt.Sprintf("%s/openai/deployments?api-version=%s", az.endpoint, az.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", az.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := az.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure OpenAI API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]map[string]interface{}, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, map[string]interface{}{
			"id":       m.ID,
			"model":    m.Model,
			"object":   "deployment",
			"owned_by": "azure",
		})
	}

	az.logger.WithField("count", len(models)).Info("Listed Azure OpenAI deployments")
	return models, nil
}

// InvokeModel invokes an Azure OpenAI model
func (az *AzureOpenAIIntegration) InvokeModel(ctx context.Context, deploymentName, prompt string, config map[string]interface{}) (string, error) {
	if az.apiKey == "" {
		return "", fmt.Errorf("Azure OpenAI API key not configured")
	}

	endpoint := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		az.endpoint, deploymentName, az.apiVersion)

	// Build OpenAI-compatible request
	requestBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  getIntConfig(config, "max_tokens", 1024),
		"temperature": getFloatConfig(config, "temperature", 0.7),
		"top_p":       getFloatConfig(config, "top_p", 0.9),
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", az.apiKey)
	req.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := az.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to invoke model: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(startTime)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Azure OpenAI API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI-compatible response
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	var content string
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
	}

	if content == "" {
		content = string(respBody)
	}

	az.logger.WithFields(logrus.Fields{
		"deployment": deploymentName,
		"endpoint":   az.endpoint,
		"duration":   duration,
	}).Info("Azure OpenAI model invoked successfully")

	return content, nil
}

// HealthCheck checks Azure OpenAI connectivity
func (az *AzureOpenAIIntegration) HealthCheck(ctx context.Context) error {
	if az.apiKey == "" {
		return fmt.Errorf("Azure OpenAI API key not configured")
	}

	_, err := az.ListModels(ctx)
	return err
}

// GetProviderName returns the provider name
func (az *AzureOpenAIIntegration) GetProviderName() string {
	return "azure-openai"
}

// ========== Cloud Integration Manager ==========

// CloudIntegrationManager manages multiple cloud provider integrations
type CloudIntegrationManager struct {
	providers map[string]CloudProvider
	logger    *logrus.Logger
}

// NewCloudIntegrationManager creates a new cloud integration manager
func NewCloudIntegrationManager(logger *logrus.Logger) *CloudIntegrationManager {
	return &CloudIntegrationManager{
		providers: make(map[string]CloudProvider),
		logger:    logger,
	}
}

// RegisterProvider registers a cloud provider
func (cim *CloudIntegrationManager) RegisterProvider(provider CloudProvider) {
	cim.providers[provider.GetProviderName()] = provider
	cim.logger.WithField("provider", provider.GetProviderName()).Info("Cloud provider registered")
}

// GetProvider returns a cloud provider by name
func (cim *CloudIntegrationManager) GetProvider(providerName string) (CloudProvider, error) {
	provider, exists := cim.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("cloud provider %s not found", providerName)
	}
	return provider, nil
}

// ListAllProviders returns all registered providers
func (cim *CloudIntegrationManager) ListAllProviders() []string {
	providers := make([]string, 0, len(cim.providers))
	for name := range cim.providers {
		providers = append(providers, name)
	}
	return providers
}

// InvokeCloudModel invokes a model on a cloud provider
func (cim *CloudIntegrationManager) InvokeCloudModel(ctx context.Context, providerName, modelName, prompt string, config map[string]interface{}) (string, error) {
	provider, err := cim.GetProvider(providerName)
	if err != nil {
		return "", err
	}

	startTime := time.Now()
	result, err := provider.InvokeModel(ctx, modelName, prompt, config)
	duration := time.Since(startTime)

	if err != nil {
		cim.logger.WithError(err).WithFields(logrus.Fields{
			"provider": providerName,
			"model":    modelName,
			"duration": duration,
		}).Error("Cloud model invocation failed")
		return "", err
	}

	cim.logger.WithFields(logrus.Fields{
		"provider": providerName,
		"model":    modelName,
		"duration": duration,
	}).Info("Cloud model invoked successfully")

	return result, nil
}

// HealthCheckAll performs health checks on all registered providers
func (cim *CloudIntegrationManager) HealthCheckAll(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, provider := range cim.providers {
		results[name] = provider.HealthCheck(ctx)
	}
	return results
}

// InitializeDefaultProviders initializes default cloud providers from environment
func (cim *CloudIntegrationManager) InitializeDefaultProviders() error {
	// AWS Bedrock
	if region := os.Getenv("AWS_REGION"); region != "" {
		provider := NewAWSBedrockIntegration(region, cim.logger)
		cim.RegisterProvider(provider)
		cim.logger.Info("AWS Bedrock provider initialized")
	}

	// GCP Vertex AI
	if projectID := os.Getenv("GCP_PROJECT_ID"); projectID != "" {
		location := os.Getenv("GCP_LOCATION")
		if location == "" {
			location = "us-central1"
		}
		provider := NewGCPVertexAIIntegration(projectID, location, cim.logger)
		cim.RegisterProvider(provider)
		cim.logger.Info("GCP Vertex AI provider initialized")
	}

	// Azure OpenAI
	if endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT"); endpoint != "" {
		provider := NewAzureOpenAIIntegration(endpoint, cim.logger)
		cim.RegisterProvider(provider)
		cim.logger.Info("Azure OpenAI provider initialized")
	}

	return nil
}

// ========== Helper Functions ==========

func getIntConfig(config map[string]interface{}, key string, defaultVal int) int {
	if config == nil {
		return defaultVal
	}
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

func getFloatConfig(config map[string]interface{}, key string, defaultVal float64) float64 {
	if config == nil {
		return defaultVal
	}
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return defaultVal
}

func extractTextFromResponse(data map[string]interface{}) string {
	// Try common response field names
	for _, key := range []string{"text", "content", "output", "generated_text", "response", "result"} {
		if val, ok := data[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}

	// Try nested structures
	if predictions, ok := data["predictions"].([]interface{}); ok && len(predictions) > 0 {
		if pred, ok := predictions[0].(map[string]interface{}); ok {
			return extractTextFromResponse(pred)
		}
		if str, ok := predictions[0].(string); ok {
			return str
		}
	}

	if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					return content
				}
			}
			if text, ok := choice["text"].(string); ok {
				return text
			}
		}
	}

	return ""
}
