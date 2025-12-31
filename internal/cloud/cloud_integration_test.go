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

// ========== Helper Function Tests ==========

func TestGetIntConfig(t *testing.T) {
	t.Run("nil config returns default", func(t *testing.T) {
		result := getIntConfig(nil, "key", 42)
		assert.Equal(t, 42, result)
	})

	t.Run("missing key returns default", func(t *testing.T) {
		config := map[string]interface{}{"other": 10}
		result := getIntConfig(config, "key", 42)
		assert.Equal(t, 42, result)
	})

	t.Run("int value", func(t *testing.T) {
		config := map[string]interface{}{"key": 100}
		result := getIntConfig(config, "key", 42)
		assert.Equal(t, 100, result)
	})

	t.Run("int64 value", func(t *testing.T) {
		config := map[string]interface{}{"key": int64(200)}
		result := getIntConfig(config, "key", 42)
		assert.Equal(t, 200, result)
	})

	t.Run("float64 value", func(t *testing.T) {
		config := map[string]interface{}{"key": float64(300.7)}
		result := getIntConfig(config, "key", 42)
		assert.Equal(t, 300, result)
	})

	t.Run("string value returns default", func(t *testing.T) {
		config := map[string]interface{}{"key": "not an int"}
		result := getIntConfig(config, "key", 42)
		assert.Equal(t, 42, result)
	})
}

func TestGetFloatConfig(t *testing.T) {
	t.Run("nil config returns default", func(t *testing.T) {
		result := getFloatConfig(nil, "key", 0.5)
		assert.Equal(t, 0.5, result)
	})

	t.Run("missing key returns default", func(t *testing.T) {
		config := map[string]interface{}{"other": 0.9}
		result := getFloatConfig(config, "key", 0.5)
		assert.Equal(t, 0.5, result)
	})

	t.Run("float64 value", func(t *testing.T) {
		config := map[string]interface{}{"key": 0.7}
		result := getFloatConfig(config, "key", 0.5)
		assert.Equal(t, 0.7, result)
	})

	t.Run("float32 value", func(t *testing.T) {
		config := map[string]interface{}{"key": float32(0.8)}
		result := getFloatConfig(config, "key", 0.5)
		assert.InDelta(t, 0.8, result, 0.001)
	})

	t.Run("int value", func(t *testing.T) {
		config := map[string]interface{}{"key": 1}
		result := getFloatConfig(config, "key", 0.5)
		assert.Equal(t, 1.0, result)
	})

	t.Run("string value returns default", func(t *testing.T) {
		config := map[string]interface{}{"key": "not a float"}
		result := getFloatConfig(config, "key", 0.5)
		assert.Equal(t, 0.5, result)
	})
}

func TestExtractTextFromResponse(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result := extractTextFromResponse(map[string]interface{}{})
		assert.Empty(t, result)
	})

	t.Run("text field", func(t *testing.T) {
		data := map[string]interface{}{"text": "hello world"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "hello world", result)
	})

	t.Run("content field", func(t *testing.T) {
		data := map[string]interface{}{"content": "hello content"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "hello content", result)
	})

	t.Run("output field", func(t *testing.T) {
		data := map[string]interface{}{"output": "hello output"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "hello output", result)
	})

	t.Run("generated_text field", func(t *testing.T) {
		data := map[string]interface{}{"generated_text": "generated text"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "generated text", result)
	})

	t.Run("response field", func(t *testing.T) {
		data := map[string]interface{}{"response": "response text"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "response text", result)
	})

	t.Run("result field", func(t *testing.T) {
		data := map[string]interface{}{"result": "result text"}
		result := extractTextFromResponse(data)
		assert.Equal(t, "result text", result)
	})

	t.Run("predictions array with map", func(t *testing.T) {
		data := map[string]interface{}{
			"predictions": []interface{}{
				map[string]interface{}{"text": "predicted text"},
			},
		}
		result := extractTextFromResponse(data)
		assert.Equal(t, "predicted text", result)
	})

	t.Run("predictions array with string", func(t *testing.T) {
		data := map[string]interface{}{
			"predictions": []interface{}{"string prediction"},
		}
		result := extractTextFromResponse(data)
		assert.Equal(t, "string prediction", result)
	})

	t.Run("choices array with message", func(t *testing.T) {
		data := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"content": "message content",
					},
				},
			},
		}
		result := extractTextFromResponse(data)
		assert.Equal(t, "message content", result)
	})

	t.Run("choices array with text", func(t *testing.T) {
		data := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"text": "choice text",
				},
			},
		}
		result := extractTextFromResponse(data)
		assert.Equal(t, "choice text", result)
	})
}

// ========== Config Constructor Tests ==========

func TestNewAWSBedrockIntegrationWithConfig(t *testing.T) {
	logger := newTestLogger()

	t.Run("default timeout", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-west-2",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration)
		assert.Equal(t, "us-west-2", integration.region)
		assert.Equal(t, "test-key", integration.accessKeyID)
		assert.Equal(t, "test-secret", integration.secretAccessKey)
	})

	t.Run("with session token", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "eu-west-1",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			SessionToken:    "session-token",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		assert.Equal(t, "session-token", integration.sessionToken)
	})
}

func TestNewGCPVertexAIIntegrationWithConfig(t *testing.T) {
	logger := newTestLogger()

	t.Run("default location", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "my-project",
			AccessToken: "token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration)
		assert.Equal(t, "my-project", integration.projectID)
		assert.Equal(t, "us-central1", integration.location) // default
	})

	t.Run("custom location", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "my-project",
			Location:    "europe-west4",
			AccessToken: "token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		assert.Equal(t, "europe-west4", integration.location)
	})
}

func TestNewAzureOpenAIIntegrationWithConfig(t *testing.T) {
	logger := newTestLogger()

	t.Run("default api version", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint: "https://test.openai.azure.com",
			APIKey:   "test-key",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration)
		assert.Equal(t, "https://test.openai.azure.com", integration.endpoint)
		assert.Equal(t, "2024-02-01", integration.apiVersion) // default
	})

	t.Run("custom api version", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint:   "https://test.openai.azure.com/",
			APIKey:     "test-key",
			APIVersion: "2023-05-15",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)
		assert.Equal(t, "https://test.openai.azure.com", integration.endpoint) // trailing slash removed
		assert.Equal(t, "2023-05-15", integration.apiVersion)
	})
}

// ========== HealthCheck Tests ==========

func TestAWSBedrockIntegration_HealthCheck(t *testing.T) {
	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-east-1", logger)

	ctx := context.Background()
	err := integration.HealthCheck(ctx)

	// Without credentials, health check should fail
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "credentials not configured")
	}
}

func TestGCPVertexAIIntegration_HealthCheck(t *testing.T) {
	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("my-project", "us-central1", logger)

	ctx := context.Background()
	err := integration.HealthCheck(ctx)

	// Without credentials, health check should fail
	if os.Getenv("GOOGLE_ACCESS_TOKEN") == "" {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	}
}

func TestAzureOpenAIIntegration_HealthCheck(t *testing.T) {
	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

	ctx := context.Background()
	err := integration.HealthCheck(ctx)

	// Without credentials, health check should fail
	if os.Getenv("AZURE_OPENAI_API_KEY") == "" {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	}
}

func TestCloudIntegrationManager_HealthCheckAll(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	// Register providers (without real credentials)
	manager.RegisterProvider(NewAWSBedrockIntegration("us-east-1", logger))
	manager.RegisterProvider(NewGCPVertexAIIntegration("project", "us-central1", logger))
	manager.RegisterProvider(NewAzureOpenAIIntegration("https://test.openai.azure.com", logger))

	ctx := context.Background()
	results := manager.HealthCheckAll(ctx)

	// All should fail without credentials
	assert.Len(t, results, 3)
	for name, err := range results {
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" && name == "aws-bedrock" {
			assert.Error(t, err)
		}
		if os.Getenv("GOOGLE_ACCESS_TOKEN") == "" && name == "gcp-vertex-ai" {
			assert.Error(t, err)
		}
		if os.Getenv("AZURE_OPENAI_API_KEY") == "" && name == "azure-openai" {
			assert.Error(t, err)
		}
	}
}
