package cloud

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stderr)
	return logger
}

// AWS Bedrock Integration Tests
func TestNewAWSBedrockIntegration(t *testing.T) {
	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-east-1", logger)

	assert.NotNil(t, integration)
	assert.Equal(t, "us-east-1", integration.region)
	assert.NotNil(t, integration.logger)
}

func TestAWSBedrockIntegration_ListModels(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping AWS Bedrock ListModels test: AWS credentials not configured")
	}

	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-east-1", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, models)
}

func TestAWSBedrockIntegration_InvokeModel(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping AWS Bedrock InvokeModel test: AWS credentials not configured")
	}

	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-west-2", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	result, err := integration.InvokeModel(ctx, "amazon.titan-text-express-v1", prompt, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestAWSBedrockIntegration_GetProviderName(t *testing.T) {
	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("eu-west-1", logger)

	assert.Equal(t, "aws-bedrock", integration.GetProviderName())
}

// GCP Vertex AI Integration Tests
func TestNewGCPVertexAIIntegration(t *testing.T) {
	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("my-project", "us-central1", logger)

	assert.NotNil(t, integration)
	assert.Equal(t, "my-project", integration.projectID)
	assert.Equal(t, "us-central1", integration.location)
	assert.NotNil(t, integration.logger)
}

func TestGCPVertexAIIntegration_ListModels(t *testing.T) {
	if os.Getenv("GCP_ACCESS_TOKEN") == "" && os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping GCP Vertex AI ListModels test: GCP credentials not configured")
	}

	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("my-project", "us-central1", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, models)
}

func TestGCPVertexAIIntegration_InvokeModel(t *testing.T) {
	if os.Getenv("GCP_ACCESS_TOKEN") == "" && os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping GCP Vertex AI InvokeModel test: GCP credentials not configured")
	}

	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("test-project", "europe-west1", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	result, err := integration.InvokeModel(ctx, "text-bison", prompt, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGCPVertexAIIntegration_GetProviderName(t *testing.T) {
	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("my-project", "us-central1", logger)

	assert.Equal(t, "gcp-vertex-ai", integration.GetProviderName())
}

// Azure OpenAI Integration Tests
func TestNewAzureOpenAIIntegration(t *testing.T) {
	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://my-resource.openai.azure.com", logger)

	assert.NotNil(t, integration)
	assert.Equal(t, "https://my-resource.openai.azure.com", integration.endpoint)
	assert.NotNil(t, integration.logger)
}

func TestAzureOpenAIIntegration_ListModels(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		t.Skip("Skipping Azure OpenAI ListModels test: Azure credentials not configured")
	}

	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, models)
}

func TestAzureOpenAIIntegration_InvokeModel(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		t.Skip("Skipping Azure OpenAI InvokeModel test: Azure credentials not configured")
	}

	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://my-resource.openai.azure.com", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	result, err := integration.InvokeModel(ctx, "gpt-4", prompt, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestAzureOpenAIIntegration_GetProviderName(t *testing.T) {
	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

	assert.Equal(t, "azure-openai", integration.GetProviderName())
}

// Cloud Integration Manager Tests
func TestNewCloudIntegrationManager(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.providers)
	assert.Empty(t, manager.providers)
	assert.NotNil(t, manager.logger)
}

func TestCloudIntegrationManager_RegisterProvider(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	// Register AWS provider
	awsProvider := NewAWSBedrockIntegration("us-east-1", logger)
	manager.RegisterProvider(awsProvider)

	providers := manager.ListAllProviders()
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, "aws-bedrock")

	// Register GCP provider
	gcpProvider := NewGCPVertexAIIntegration("project-id", "us-central1", logger)
	manager.RegisterProvider(gcpProvider)

	providers = manager.ListAllProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "gcp-vertex-ai")
}

func TestCloudIntegrationManager_GetProvider(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	// Register a provider
	awsProvider := NewAWSBedrockIntegration("us-east-1", logger)
	manager.RegisterProvider(awsProvider)

	// Get existing provider
	provider, err := manager.GetProvider("aws-bedrock")
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "aws-bedrock", provider.GetProviderName())

	// Get non-existing provider
	provider, err = manager.GetProvider("non-existent")
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "not found")
}

func TestCloudIntegrationManager_ListAllProviders(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	// Initially empty
	providers := manager.ListAllProviders()
	assert.Empty(t, providers)

	// Add providers
	manager.RegisterProvider(NewAWSBedrockIntegration("us-east-1", logger))
	manager.RegisterProvider(NewGCPVertexAIIntegration("project-id", "us-central1", logger))
	manager.RegisterProvider(NewAzureOpenAIIntegration("https://test.openai.azure.com", logger))

	providers = manager.ListAllProviders()
	assert.Len(t, providers, 3)
	assert.Contains(t, providers, "aws-bedrock")
	assert.Contains(t, providers, "gcp-vertex-ai")
	assert.Contains(t, providers, "azure-openai")
}

func TestCloudIntegrationManager_InvokeCloudModel(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	// Register providers
	manager.RegisterProvider(NewAWSBedrockIntegration("us-east-1", logger))
	manager.RegisterProvider(NewGCPVertexAIIntegration("project-id", "us-central1", logger))

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	// Test invoking AWS model (skip actual invocation if no credentials)
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		result, err := manager.InvokeCloudModel(ctx, "aws-bedrock", "amazon.titan-text-express-v1", prompt, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	}

	// Test invoking GCP model (skip actual invocation if no credentials)
	if os.Getenv("GCP_ACCESS_TOKEN") != "" {
		result, err := manager.InvokeCloudModel(ctx, "gcp-vertex-ai", "text-bison", prompt, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	}

	// Invoke non-existent provider (always test this)
	result, err := manager.InvokeCloudModel(ctx, "non-existent", "model", prompt, nil)
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestCloudIntegrationManager_InvokeCloudModel_WithConfig(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		t.Skip("Skipping Azure OpenAI InvokeCloudModel test: Azure credentials not configured")
	}

	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	manager.RegisterProvider(NewAzureOpenAIIntegration("https://test.openai.azure.com", logger))

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
	config := map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	result, err := manager.InvokeCloudModel(ctx, "azure-openai", "gpt-4", prompt, config)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCloudIntegrationManager_InitializeDefaultProviders(t *testing.T) {
	logger := newTestLogger()

	t.Run("no environment variables set", func(t *testing.T) {
		// Clear env vars
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("GCP_PROJECT_ID")
		os.Unsetenv("GCP_LOCATION")
		os.Unsetenv("AZURE_OPENAI_ENDPOINT")

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		assert.Empty(t, manager.ListAllProviders())
	})

	t.Run("AWS_REGION set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-2")
		defer os.Unsetenv("AWS_REGION")

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		providers := manager.ListAllProviders()
		assert.Contains(t, providers, "aws-bedrock")
	})

	t.Run("GCP_PROJECT_ID set with default location", func(t *testing.T) {
		os.Setenv("GCP_PROJECT_ID", "my-project")
		os.Unsetenv("GCP_LOCATION")
		defer os.Unsetenv("GCP_PROJECT_ID")

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		providers := manager.ListAllProviders()
		assert.Contains(t, providers, "gcp-vertex-ai")
	})

	t.Run("GCP_PROJECT_ID set with custom location", func(t *testing.T) {
		os.Setenv("GCP_PROJECT_ID", "my-project")
		os.Setenv("GCP_LOCATION", "europe-west4")
		defer func() {
			os.Unsetenv("GCP_PROJECT_ID")
			os.Unsetenv("GCP_LOCATION")
		}()

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		providers := manager.ListAllProviders()
		assert.Contains(t, providers, "gcp-vertex-ai")
	})

	t.Run("AZURE_OPENAI_ENDPOINT set", func(t *testing.T) {
		os.Setenv("AZURE_OPENAI_ENDPOINT", "https://test.openai.azure.com")
		defer os.Unsetenv("AZURE_OPENAI_ENDPOINT")

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		providers := manager.ListAllProviders()
		assert.Contains(t, providers, "azure-openai")
	})

	t.Run("all providers set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "eu-west-1")
		os.Setenv("GCP_PROJECT_ID", "test-project")
		os.Setenv("AZURE_OPENAI_ENDPOINT", "https://test.openai.azure.com")
		defer func() {
			os.Unsetenv("AWS_REGION")
			os.Unsetenv("GCP_PROJECT_ID")
			os.Unsetenv("AZURE_OPENAI_ENDPOINT")
		}()

		manager := NewCloudIntegrationManager(logger)
		err := manager.InitializeDefaultProviders()

		require.NoError(t, err)
		providers := manager.ListAllProviders()
		assert.Len(t, providers, 3)
		assert.Contains(t, providers, "aws-bedrock")
		assert.Contains(t, providers, "gcp-vertex-ai")
		assert.Contains(t, providers, "azure-openai")
	})
}

// Interface compliance test
func TestCloudProviderInterface(t *testing.T) {
	logger := newTestLogger()

	// Verify all implementations satisfy CloudProvider interface at compile time
	var _ CloudProvider = (*AWSBedrockIntegration)(nil)
	var _ CloudProvider = (*GCPVertexAIIntegration)(nil)
	var _ CloudProvider = (*AzureOpenAIIntegration)(nil)

	// Test each implementation's GetProviderName (no credentials needed)
	t.Run("AWS Bedrock Interface", func(t *testing.T) {
		provider := NewAWSBedrockIntegration("us-east-1", logger)

		// GetProviderName should return non-empty
		name := provider.GetProviderName()
		assert.NotEmpty(t, name)
		assert.Equal(t, "aws-bedrock", name)

		// All API calls require credentials - verify proper error handling
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
			// ListModels requires credentials
			_, err := provider.ListModels(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "credentials not configured")

			// InvokeModel requires credentials
			prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
			_, err = provider.InvokeModel(context.Background(), "test-model", prompt, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "credentials not configured")
		}
	})

	t.Run("GCP Vertex AI Interface", func(t *testing.T) {
		provider := NewGCPVertexAIIntegration("project-id", "us-central1", logger)

		// GetProviderName should return non-empty
		name := provider.GetProviderName()
		assert.NotEmpty(t, name)
		assert.Equal(t, "gcp-vertex-ai", name)

		// All API calls require credentials - verify proper error handling
		if os.Getenv("GCP_ACCESS_TOKEN") == "" {
			// ListModels requires credentials
			_, err := provider.ListModels(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not configured")

			// InvokeModel requires credentials
			prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
			_, err = provider.InvokeModel(context.Background(), "test-model", prompt, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not configured")
		}
	})

	t.Run("Azure OpenAI Interface", func(t *testing.T) {
		provider := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

		// GetProviderName should return non-empty
		name := provider.GetProviderName()
		assert.NotEmpty(t, name)
		assert.Equal(t, "azure-openai", name)

		// All API calls require credentials - verify proper error handling
		if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
			// ListModels requires credentials
			_, err := provider.ListModels(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not configured")

			// InvokeModel requires credentials
			prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
			_, err = provider.InvokeModel(context.Background(), "test-model", prompt, nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not configured")
		}
	})
}

// Benchmark tests
func BenchmarkAWSBedrockIntegration_InvokeModel(b *testing.B) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel) // Reduce logging noise

	integration := NewAWSBedrockIntegration("us-east-1", logger)
	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = integration.InvokeModel(ctx, "amazon.titan-text-express-v1", prompt, nil)
	}
}

func BenchmarkCloudIntegrationManager_InvokeCloudModel(b *testing.B) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)

	manager := NewCloudIntegrationManager(logger)
	manager.RegisterProvider(NewAWSBedrockIntegration("us-east-1", logger))

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.InvokeCloudModel(ctx, "aws-bedrock", "amazon.titan-text-express-v1", prompt, nil)
	}
}
