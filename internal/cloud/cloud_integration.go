package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/bedrock"
	"github.com/aws/aws-sdk-go/service/bedrockruntime"
	"github.com/sirupsen/logrus"
)

// AWSBedrockIntegration provides AWS Bedrock AI service integration
type AWSBedrockIntegration struct {
	bedrockClient   *bedrock.Bedrock
	runtimeClient   *bedrockruntime.BedrockRuntime
	logger          *logrus.Logger
	modelCache      map[string]*bedrock.ModelSummary
	cacheMutex      sync.RWMutex
	cacheExpiration time.Time
}

// NewAWSBedrockIntegration creates a new AWS Bedrock integration
func NewAWSBedrockIntegration(region string, logger *logrus.Logger) (*AWSBedrockIntegration, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &AWSBedrockIntegration{
		bedrockClient: bedrock.New(sess),
		runtimeClient: bedrockruntime.New(sess),
		logger:        logger,
		modelCache:    make(map[string]*bedrock.ModelSummary),
	}, nil
}

// ListModels lists available Bedrock models
func (a *AWSBedrockIntegration) ListModels(ctx context.Context) ([]*bedrock.ModelSummary, error) {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()

	// Check cache validity
	if time.Now().Before(a.cacheExpiration) && len(a.modelCache) > 0 {
		var models []*bedrock.ModelSummary
		for _, model := range a.modelCache {
			models = append(models, model)
		}
		return models, nil
	}

	input := &bedrock.ListFoundationModelsInput{}
	result, err := a.bedrockClient.ListFoundationModelsWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list Bedrock models: %w", err)
	}

	// Update cache
	a.modelCache = make(map[string]*bedrock.ModelSummary)
	for _, model := range result.ModelSummaries {
		a.modelCache[*model.ModelId] = model
	}
	a.cacheExpiration = time.Now().Add(1 * time.Hour)

	return result.ModelSummaries, nil
}

// InvokeModel invokes a Bedrock model
func (a *AWSBedrockIntegration) InvokeModel(ctx context.Context, modelId, prompt string, config map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"prompt":      prompt,
		"max_tokens":  1000,
		"temperature": 0.7,
	}

	// Override with config
	if maxTokens, ok := config["max_tokens"].(float64); ok {
		body["max_tokens"] = int(maxTokens)
	}
	if temperature, ok := config["temperature"].(float64); ok {
		body["temperature"] = temperature
	}
	if topP, ok := config["top_p"].(float64); ok {
		body["top_p"] = topP
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelId),
		Body:        bodyBytes,
		ContentType: aws.String("application/json"),
	}

	result, err := a.runtimeClient.InvokeModelWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to invoke Bedrock model: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(result.Body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract generated text (this varies by model)
	if completions, ok := response["completions"].([]interface{}); ok && len(completions) > 0 {
		if completion, ok := completions[0].(map[string]interface{}); ok {
			if text, ok := completion["data"].(map[string]interface{})["text"].(string); ok {
				return text, nil
			}
		}
	}

	return "", fmt.Errorf("unable to extract response text")
}

// GCPVertexAIIntegration provides Google Cloud Vertex AI integration
type GCPVertexAIIntegration struct {
	projectID  string
	location   string
	logger     *logrus.Logger
	httpClient *http.Client
}

// NewGCPVertexAIIntegration creates a new GCP Vertex AI integration
func NewGCPVertexAIIntegration(projectID, location string, logger *logrus.Logger) *GCPVertexAIIntegration {
	return &GCPVertexAIIntegration{
		projectID:  projectID,
		location:   location,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ListModels lists available Vertex AI models
func (g *GCPVertexAIIntegration) ListModels(ctx context.Context) ([]map[string]interface{}, error) {
	// In a real implementation, this would call the Vertex AI API
	// For now, return mock data
	return []map[string]interface{}{
		{
			"name":         "text-bison",
			"display_name": "Text Bison",
			"description":  "Fast and efficient text generation",
		},
		{
			"name":         "chat-bison",
			"display_name": "Chat Bison",
			"description":  "Conversational AI model",
		},
	}, nil
}

// InvokeModel invokes a Vertex AI model
func (g *GCPVertexAIIntegration) InvokeModel(ctx context.Context, modelName, prompt string, config map[string]interface{}) (string, error) {
	// In a real implementation, this would call the Vertex AI API
	// For now, return mock response
	return fmt.Sprintf("Mock response from %s: %s", modelName, prompt[:50]+"..."), nil
}

// AzureOpenAIIntegration provides Azure OpenAI integration
type AzureOpenAIIntegration struct {
	endpoint   string
	apiKey     string
	logger     *logrus.Logger
	httpClient *http.Client
}

// NewAzureOpenAIIntegration creates a new Azure OpenAI integration
func NewAzureOpenAIIntegration(endpoint, apiKey string, logger *logrus.Logger) *AzureOpenAIIntegration {
	return &AzureOpenAIIntegration{
		endpoint:   endpoint,
		apiKey:     apiKey,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ListModels lists available Azure OpenAI models
func (az *AzureOpenAIIntegration) ListModels(ctx context.Context) ([]map[string]interface{}, error) {
	// In a real implementation, this would call the Azure OpenAI API
	return []map[string]interface{}{
		{
			"id":       "gpt-4",
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "azure",
		},
		{
			"id":       "gpt-35-turbo",
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "azure",
		},
	}, nil
}

// InvokeModel invokes an Azure OpenAI model
func (az *AzureOpenAIIntegration) InvokeModel(ctx context.Context, modelName, prompt string, config map[string]interface{}) (string, error) {
	// In a real implementation, this would call the Azure OpenAI API
	return fmt.Sprintf("Mock Azure response from %s: %s", modelName, prompt[:50]+"..."), nil
}

// CloudProvider represents a cloud AI provider
type CloudProvider interface {
	ListModels(ctx context.Context) ([]map[string]interface{}, error)
	InvokeModel(ctx context.Context, modelName, prompt string, config map[string]interface{}) (string, error)
	GetProviderName() string
}

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
	var providers []string
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

// InitializeDefaultProviders initializes default cloud providers from environment
func (cim *CloudIntegrationManager) InitializeDefaultProviders() error {
	// AWS Bedrock
	if region := os.Getenv("AWS_REGION"); region != "" {
		if provider, err := NewAWSBedrockIntegration(region, cim.logger); err == nil {
			cim.RegisterProvider(provider)
			cim.logger.Info("AWS Bedrock provider initialized")
		} else {
			cim.logger.WithError(err).Warn("Failed to initialize AWS Bedrock provider")
		}
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
		if apiKey := os.Getenv("AZURE_OPENAI_API_KEY"); apiKey != "" {
			provider := NewAzureOpenAIIntegration(endpoint, apiKey, cim.logger)
			cim.RegisterProvider(provider)
			cim.logger.Info("Azure OpenAI provider initialized")
		}
	}

	return nil
}

// Implement CloudProvider interface for AWS
func (a *AWSBedrockIntegration) GetProviderName() string {
	return "aws-bedrock"
}

// Implement CloudProvider interface for GCP
func (g *GCPVertexAIIntegration) GetProviderName() string {
	return "gcp-vertex-ai"
}

// Implement CloudProvider interface for Azure
func (az *AzureOpenAIIntegration) GetProviderName() string {
	return "azure-openai"
}
