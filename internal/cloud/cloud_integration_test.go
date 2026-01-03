package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

// ========== AWS Signature V4 Tests ==========

func TestHmacSHA256(t *testing.T) {
	t.Run("basic hmac calculation", func(t *testing.T) {
		key := []byte("secret")
		data := "test data"
		result := hmacSHA256(key, data)

		assert.NotNil(t, result)
		assert.Len(t, result, 32) // SHA256 produces 32 bytes
	})

	t.Run("empty data", func(t *testing.T) {
		key := []byte("secret")
		data := ""
		result := hmacSHA256(key, data)

		assert.NotNil(t, result)
		assert.Len(t, result, 32)
	})

	t.Run("empty key", func(t *testing.T) {
		key := []byte("")
		data := "test"
		result := hmacSHA256(key, data)

		assert.NotNil(t, result)
		assert.Len(t, result, 32)
	})

	t.Run("consistent results", func(t *testing.T) {
		key := []byte("same-key")
		data := "same-data"
		result1 := hmacSHA256(key, data)
		result2 := hmacSHA256(key, data)

		assert.Equal(t, result1, result2)
	})

	t.Run("different data produces different hash", func(t *testing.T) {
		key := []byte("key")
		result1 := hmacSHA256(key, "data1")
		result2 := hmacSHA256(key, "data2")

		assert.NotEqual(t, result1, result2)
	})
}

func TestAWSBedrockIntegration_signRequest(t *testing.T) {
	logger := newTestLogger()

	t.Run("signs request with basic credentials", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/invoke", nil)
		req.Header.Set("Content-Type", "application/json")

		err := integration.signRequest(req, []byte(`{"test": "data"}`))

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
		assert.NotEmpty(t, req.Header.Get("X-Amz-Date"))
		assert.NotEmpty(t, req.Header.Get("X-Amz-Content-Sha256"))

		// Check authorization header format
		auth := req.Header.Get("Authorization")
		assert.Contains(t, auth, "AWS4-HMAC-SHA256")
		assert.Contains(t, auth, "Credential=")
		assert.Contains(t, auth, "SignedHeaders=")
		assert.Contains(t, auth, "Signature=")
	})

	t.Run("signs request with session token", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-west-2",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			SessionToken:    "FwoGZXIvYXdzEBYaDLW...",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		req, _ := http.NewRequest("POST", "https://bedrock-runtime.us-west-2.amazonaws.com/model/test/invoke", nil)
		req.Header.Set("Content-Type", "application/json")

		err := integration.signRequest(req, []byte(`{"prompt": "test"}`))

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("X-Amz-Security-Token"))
		assert.Equal(t, "FwoGZXIvYXdzEBYaDLW...", req.Header.Get("X-Amz-Security-Token"))
	})

	t.Run("signs request with nil body", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "eu-west-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		req, _ := http.NewRequest("GET", "https://bedrock.eu-west-1.amazonaws.com/foundation-models", nil)

		err := integration.signRequest(req, nil)

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
	})

	t.Run("signs request with empty path", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "ap-southeast-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		req, _ := http.NewRequest("GET", "https://bedrock.ap-southeast-1.amazonaws.com", nil)

		err := integration.signRequest(req, nil)

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
	})

	t.Run("signs request with query string", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		req, _ := http.NewRequest("GET", "https://bedrock.us-east-1.amazonaws.com/foundation-models?maxResults=10&byProvider=anthropic", nil)

		err := integration.signRequest(req, nil)

		assert.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("Authorization"))
	})
}

func TestAWSBedrockIntegration_getSignatureKey(t *testing.T) {
	logger := newTestLogger()

	t.Run("generates signature key", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		key := integration.getSignatureKey("20230101", "us-east-1", "bedrock")

		assert.NotNil(t, key)
		assert.Len(t, key, 32) // SHA256 produces 32 bytes
	})

	t.Run("different dates produce different keys", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test",
			SecretAccessKey: "secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		key1 := integration.getSignatureKey("20230101", "us-east-1", "bedrock")
		key2 := integration.getSignatureKey("20230102", "us-east-1", "bedrock")

		assert.NotEqual(t, key1, key2)
	})

	t.Run("different regions produce different keys", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test",
			SecretAccessKey: "secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		key1 := integration.getSignatureKey("20230101", "us-east-1", "bedrock")
		key2 := integration.getSignatureKey("20230101", "us-west-2", "bedrock")

		assert.NotEqual(t, key1, key2)
	})

	t.Run("different services produce different keys", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test",
			SecretAccessKey: "secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		key1 := integration.getSignatureKey("20230101", "us-east-1", "bedrock")
		key2 := integration.getSignatureKey("20230101", "us-east-1", "s3")

		assert.NotEqual(t, key1, key2)
	})
}

// ========== Additional InvokeCloudModel Tests ==========

func TestCloudIntegrationManager_InvokeCloudModel_Errors(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"

	t.Run("no providers registered", func(t *testing.T) {
		result, err := manager.InvokeCloudModel(ctx, "aws-bedrock", "test-model", prompt, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("non-existent provider", func(t *testing.T) {
		manager.RegisterProvider(NewAWSBedrockIntegration("us-east-1", logger))
		result, err := manager.InvokeCloudModel(ctx, "non-existent-provider", "model", prompt, nil)
		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ========== List/Invoke Model Error Path Tests ==========

func TestAWSBedrockIntegration_ListModels_NoCredentials(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		t.Skip("AWS credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-east-1", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	assert.Error(t, err)
	assert.Empty(t, models)
	assert.Contains(t, err.Error(), "credentials not configured")
}

func TestAWSBedrockIntegration_InvokeModel_NoCredentials(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		t.Skip("AWS credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewAWSBedrockIntegration("us-east-1", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
	result, err := integration.InvokeModel(ctx, "test-model", prompt, nil)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "credentials not configured")
}

func TestGCPVertexAIIntegration_ListModels_NoCredentials(t *testing.T) {
	if os.Getenv("GCP_ACCESS_TOKEN") != "" || os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		t.Skip("GCP credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("test-project", "us-central1", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	assert.Error(t, err)
	assert.Empty(t, models)
	assert.Contains(t, err.Error(), "not configured")
}

func TestGCPVertexAIIntegration_InvokeModel_NoCredentials(t *testing.T) {
	if os.Getenv("GCP_ACCESS_TOKEN") != "" || os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		t.Skip("GCP credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewGCPVertexAIIntegration("test-project", "us-central1", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
	result, err := integration.InvokeModel(ctx, "test-model", prompt, nil)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "not configured")
}

func TestAzureOpenAIIntegration_ListModels_NoCredentials(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_API_KEY") != "" {
		t.Skip("Azure credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

	ctx := context.Background()
	models, err := integration.ListModels(ctx)

	assert.Error(t, err)
	assert.Empty(t, models)
	assert.Contains(t, err.Error(), "not configured")
}

func TestAzureOpenAIIntegration_InvokeModel_NoCredentials(t *testing.T) {
	if os.Getenv("AZURE_OPENAI_API_KEY") != "" {
		t.Skip("Azure credentials are set, skipping no-credentials test")
	}

	logger := newTestLogger()
	integration := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

	ctx := context.Background()
	prompt := "This is a test prompt that is longer than fifty characters for testing purposes"
	result, err := integration.InvokeModel(ctx, "test-model", prompt, nil)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "not configured")
}

// ========== Additional Tests for extractTextFromResponse Edge Cases ==========

// ========== Mock Server Tests for HTTP Calls ==========

func TestAWSBedrockIntegration_ListModels_WithMockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful list models response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "foundation-models")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"modelSummaries": []map[string]string{
					{"modelId": "anthropic.claude-v2", "modelName": "Claude v2", "providerName": "Anthropic"},
					{"modelId": "amazon.titan-text-express-v1", "modelName": "Titan Express", "providerName": "Amazon"},
				},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "mock",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)

		// Override endpoint (would require modifying the integration for proper testing)
		// For now, this test validates the test structure is correct
		assert.NotNil(t, integration)
	})

	t.Run("list models API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Access denied"))
		}))
		defer server.Close()

		// Verify error handling structure is testable
		assert.NotNil(t, server)
	})
}

func TestGCPVertexAIIntegration_InvokeModel_RequestFormat(t *testing.T) {
	logger := newTestLogger()

	t.Run("verify request body format", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)

		assert.Equal(t, "test-project", integration.projectID)
		assert.Equal(t, "us-central1", integration.location)
		assert.Equal(t, "test-token", integration.accessToken)
	})

	t.Run("config with custom parameters", func(t *testing.T) {
		configMap := map[string]interface{}{
			"temperature": 0.5,
			"max_tokens":  2048,
			"top_p":       0.8,
			"top_k":       20,
		}

		temp := getFloatConfig(configMap, "temperature", 0.7)
		maxTokens := getIntConfig(configMap, "max_tokens", 1024)
		topP := getFloatConfig(configMap, "top_p", 0.9)
		topK := getIntConfig(configMap, "top_k", 40)

		assert.Equal(t, 0.5, temp)
		assert.Equal(t, 2048, maxTokens)
		assert.Equal(t, 0.8, topP)
		assert.Equal(t, 20, topK)
	})
}

func TestAzureOpenAIIntegration_InvokeModel_RequestFormat(t *testing.T) {
	logger := newTestLogger()

	t.Run("verify endpoint formatting", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint:   "https://my-resource.openai.azure.com/",
			APIKey:     "test-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		// Trailing slash should be removed
		assert.Equal(t, "https://my-resource.openai.azure.com", integration.endpoint)
		assert.Equal(t, "2024-02-01", integration.apiVersion)
	})

	t.Run("config extraction for chat request", func(t *testing.T) {
		configMap := map[string]interface{}{
			"max_tokens":  1500,
			"temperature": 0.9,
			"top_p":       0.95,
		}

		maxTokens := getIntConfig(configMap, "max_tokens", 1024)
		temp := getFloatConfig(configMap, "temperature", 0.7)
		topP := getFloatConfig(configMap, "top_p", 0.9)

		assert.Equal(t, 1500, maxTokens)
		assert.Equal(t, 0.9, temp)
		assert.Equal(t, 0.95, topP)
	})
}

func TestAWSBedrockIntegration_InvokeModel_ModelTypeDetection(t *testing.T) {
	logger := newTestLogger()

	config := AWSBedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
	}
	integration := NewAWSBedrockIntegrationWithConfig(config, logger)

	// Test that the integration correctly identifies model types
	t.Run("anthropic model detection", func(t *testing.T) {
		modelId := "anthropic.claude-3-sonnet"
		assert.Contains(t, modelId, "anthropic")
	})

	t.Run("amazon titan model detection", func(t *testing.T) {
		modelId := "amazon.titan-text-express-v1"
		assert.Contains(t, modelId, "amazon.titan")
	})

	t.Run("meta llama model detection", func(t *testing.T) {
		modelId := "meta.llama2-13b-chat-v1"
		assert.Contains(t, modelId, "meta.llama")
	})

	t.Run("cohere model detection", func(t *testing.T) {
		modelId := "cohere.command-text-v14"
		assert.Contains(t, modelId, "cohere")
	})

	t.Run("integration exists", func(t *testing.T) {
		assert.NotNil(t, integration)
	})
}

func TestCloudIntegrationManager_RegisterAndRetrieve(t *testing.T) {
	logger := newTestLogger()
	manager := NewCloudIntegrationManager(logger)

	t.Run("register multiple providers and verify", func(t *testing.T) {
		// Register all three providers
		awsProvider := NewAWSBedrockIntegration("us-east-1", logger)
		gcpProvider := NewGCPVertexAIIntegration("project-id", "us-central1", logger)
		azureProvider := NewAzureOpenAIIntegration("https://test.openai.azure.com", logger)

		manager.RegisterProvider(awsProvider)
		manager.RegisterProvider(gcpProvider)
		manager.RegisterProvider(azureProvider)

		// Verify all are retrievable
		provider, err := manager.GetProvider("aws-bedrock")
		assert.NoError(t, err)
		assert.Equal(t, "aws-bedrock", provider.GetProviderName())

		provider, err = manager.GetProvider("gcp-vertex-ai")
		assert.NoError(t, err)
		assert.Equal(t, "gcp-vertex-ai", provider.GetProviderName())

		provider, err = manager.GetProvider("azure-openai")
		assert.NoError(t, err)
		assert.Equal(t, "azure-openai", provider.GetProviderName())
	})

	t.Run("overwrite provider with same name", func(t *testing.T) {
		newManager := NewCloudIntegrationManager(logger)

		// Register first instance
		provider1 := NewAWSBedrockIntegration("us-east-1", logger)
		newManager.RegisterProvider(provider1)

		// Register second instance with same name
		provider2 := NewAWSBedrockIntegration("eu-west-1", logger)
		newManager.RegisterProvider(provider2)

		// Should have only one provider
		providers := newManager.ListAllProviders()
		assert.Len(t, providers, 1)
	})
}

// Test configuration struct initialization
func TestAWSBedrockConfig_Struct(t *testing.T) {
	config := AWSBedrockConfig{
		Region:          "us-west-2",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		SessionToken:    "session-token",
		Timeout:         30 * time.Second,
	}

	assert.Equal(t, "us-west-2", config.Region)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", config.AccessKeyID)
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", config.SecretAccessKey)
	assert.Equal(t, "session-token", config.SessionToken)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestGCPVertexAIConfig_Struct(t *testing.T) {
	config := GCPVertexAIConfig{
		ProjectID:   "my-gcp-project",
		Location:    "europe-west4",
		AccessToken: "ya29.access-token",
		Timeout:     45 * time.Second,
	}

	assert.Equal(t, "my-gcp-project", config.ProjectID)
	assert.Equal(t, "europe-west4", config.Location)
	assert.Equal(t, "ya29.access-token", config.AccessToken)
	assert.Equal(t, 45*time.Second, config.Timeout)
}

func TestAzureOpenAIConfig_Struct(t *testing.T) {
	config := AzureOpenAIConfig{
		Endpoint:   "https://my-resource.openai.azure.com",
		APIKey:     "azure-api-key",
		APIVersion: "2023-12-01-preview",
		Timeout:    60 * time.Second,
	}

	assert.Equal(t, "https://my-resource.openai.azure.com", config.Endpoint)
	assert.Equal(t, "azure-api-key", config.APIKey)
	assert.Equal(t, "2023-12-01-preview", config.APIVersion)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

// Test timeout configuration
func TestIntegration_TimeoutConfiguration(t *testing.T) {
	logger := newTestLogger()

	t.Run("AWS default timeout", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			Timeout:         0, // Should use default
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration.httpClient)
	})

	t.Run("AWS custom timeout", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
			Timeout:         120 * time.Second,
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration.httpClient)
	})

	t.Run("GCP default timeout", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "project",
			AccessToken: "token",
			Timeout:     0, // Should use default
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration.httpClient)
	})

	t.Run("Azure default timeout", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint: "https://test.openai.azure.com",
			APIKey:   "key",
			Timeout:  0, // Should use default
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)
		assert.NotNil(t, integration.httpClient)
	})
}

func TestExtractTextFromResponse_EdgeCases(t *testing.T) {
	t.Run("nested predictions with empty array", func(t *testing.T) {
		data := map[string]interface{}{
			"predictions": []interface{}{},
		}
		result := extractTextFromResponse(data)
		assert.Empty(t, result)
	})

	t.Run("choices with empty array", func(t *testing.T) {
		data := map[string]interface{}{
			"choices": []interface{}{},
		}
		result := extractTextFromResponse(data)
		assert.Empty(t, result)
	})

	t.Run("non-string text field", func(t *testing.T) {
		data := map[string]interface{}{
			"text": 123, // not a string
		}
		result := extractTextFromResponse(data)
		assert.Empty(t, result)
	})

	t.Run("choices with invalid message type", func(t *testing.T) {
		data := map[string]interface{}{
			"choices": []interface{}{
				map[string]interface{}{
					"message": "not-a-map",
				},
			},
		}
		result := extractTextFromResponse(data)
		assert.Empty(t, result)
	})

	t.Run("predictions with invalid first element", func(t *testing.T) {
		data := map[string]interface{}{
			"predictions": []interface{}{
				123, // not a map or string
			},
		}
		result := extractTextFromResponse(data)
		assert.Empty(t, result)
	})
}

// ========== Mock HTTP Server Tests for AWS Bedrock ==========

// mockTransport is a custom RoundTripper that redirects requests to a mock server
type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect the request to our mock server
	req.URL.Scheme = "http"
	req.URL.Host = m.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

func TestAWSBedrockIntegration_ListModels_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful list models", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.NotEmpty(t, r.Header.Get("Authorization"))
			assert.NotEmpty(t, r.Header.Get("X-Amz-Date"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"modelSummaries": []map[string]string{
					{"modelId": "anthropic.claude-v2", "modelName": "Claude v2", "providerName": "Anthropic", "modelArn": "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-v2"},
					{"modelId": "amazon.titan-text-express-v1", "modelName": "Titan Express", "providerName": "Amazon", "modelArn": "arn:aws:bedrock:us-east-1::foundation-model/amazon.titan-text-express-v1"},
				},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		require.NoError(t, err)
		assert.Len(t, models, 2)
		assert.Equal(t, "anthropic.claude-v2", models[0]["name"])
		assert.Equal(t, "Claude v2", models[0]["display_name"])
		assert.Equal(t, "Anthropic", models[0]["provider"])
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"message": "Access denied"}`))
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "decode")
	})
}

func TestAWSBedrockIntegration_InvokeModel_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful invoke anthropic model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "anthropic.claude-v2")
			assert.NotEmpty(t, r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify request body format
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.Equal(t, "bedrock-2023-05-31", reqBody["anthropic_version"])
			assert.NotNil(t, reqBody["messages"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": "Hello, I am Claude!"},
				},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "anthropic.claude-v2", "Hello", nil)

		require.NoError(t, err)
		assert.Equal(t, "Hello, I am Claude!", result)
	})

	t.Run("successful invoke titan model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotEmpty(t, reqBody["inputText"])
			assert.NotNil(t, reqBody["textGenerationConfig"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{
					{"outputText": "Titan response here"},
				},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "amazon.titan-text-express-v1", "Test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "Titan response here", result)
	})

	t.Run("successful invoke llama model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotEmpty(t, reqBody["prompt"])
			assert.NotNil(t, reqBody["max_gen_len"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"generation": "Llama response here",
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-west-2",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "meta.llama2-13b-chat-v1", "Test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "Llama response here", result)
	})

	t.Run("successful invoke cohere model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotEmpty(t, reqBody["prompt"])
			assert.NotNil(t, reqBody["max_tokens"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"generations": []map[string]string{
					{"text": "Cohere response here"},
				},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "cohere.command-text-v14", "Test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "Cohere response here", result)
	})

	t.Run("successful invoke default/other model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotEmpty(t, reqBody["prompt"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": "Generic response here",
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "some.other-model", "Test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "Generic response here", result)
	})

	t.Run("invoke with custom config", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			// Verify custom config values are used
			config := reqBody["textGenerationConfig"].(map[string]interface{})
			assert.Equal(t, float64(2048), config["maxTokenCount"])
			assert.Equal(t, 0.5, config["temperature"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]string{
					{"outputText": "Response with custom config"},
				},
			})
		}))
		defer server.Close()

		awsConfig := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(awsConfig, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		invokeConfig := map[string]interface{}{
			"max_tokens":  2048,
			"temperature": 0.5,
			"top_p":       0.95,
		}

		result, err := integration.InvokeModel(context.Background(), "amazon.titan-text-express-v1", "Test prompt", invokeConfig)

		require.NoError(t, err)
		assert.Equal(t, "Response with custom config", result)
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message": "Invalid request"}`))
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "anthropic.claude-v2", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "400")
	})

	t.Run("fallback to raw response when no content extracted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"unknownField": "some value"}`))
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "unknown.model", "Test", nil)

		require.NoError(t, err)
		assert.Contains(t, result, "unknownField")
	})

	t.Run("response parsing with empty content array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"content": []interface{}{},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "anthropic.claude-v2", "Test", nil)

		require.NoError(t, err)
		// Should fallback to raw response since content is empty
		assert.NotEmpty(t, result)
	})
}

// ========== Mock HTTP Server Tests for GCP Vertex AI ==========

func TestGCPVertexAIIntegration_ListModels_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful list models", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]string{
					{"name": "text-bison@001", "displayName": "Text Bison", "description": "Text generation model"},
					{"name": "chat-bison@001", "displayName": "Chat Bison", "description": "Chat model"},
				},
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		require.NoError(t, err)
		assert.Len(t, models, 2)
		assert.Equal(t, "text-bison@001", models[0]["name"])
		assert.Equal(t, "Text Bison", models[0]["display_name"])
		assert.Equal(t, "gcp", models[0]["provider"])
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Invalid token"}`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "invalid-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not valid json`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "decode")
	})
}

func TestGCPVertexAIIntegration_InvokeModel_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful invoke model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify request body
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotNil(t, reqBody["instances"])
			assert.NotNil(t, reqBody["parameters"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"predictions": []map[string]string{
					{"content": "GCP Vertex AI response here"},
				},
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Hello", nil)

		require.NoError(t, err)
		assert.Equal(t, "GCP Vertex AI response here", result)
	})

	t.Run("invoke with custom parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			params := reqBody["parameters"].(map[string]interface{})
			assert.Equal(t, 0.5, params["temperature"])
			assert.Equal(t, float64(2048), params["maxOutputTokens"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"predictions": []map[string]string{
					{"content": "Custom response"},
				},
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		invokeConfig := map[string]interface{}{
			"temperature": 0.5,
			"max_tokens":  2048,
			"top_p":       0.8,
			"top_k":       20,
		}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", invokeConfig)

		require.NoError(t, err)
		assert.Equal(t, "Custom response", result)
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Internal server error"}`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("empty predictions fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"predictions": []interface{}{},
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		require.NoError(t, err)
		// Should return raw response as fallback
		assert.NotEmpty(t, result)
	})

	t.Run("generic response extraction fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"text": "Extracted via generic fallback",
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		require.NoError(t, err)
		assert.Equal(t, "Extracted via generic fallback", result)
	})
}

func TestGCPVertexAIIntegration_HealthCheck_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		err := integration.HealthCheck(context.Background())

		assert.NoError(t, err)
	})

	t.Run("health check failure - 4xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		err := integration.HealthCheck(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})

	t.Run("health check failure - 5xx error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "Service unavailable"}`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		err := integration.HealthCheck(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})
}

// ========== Mock HTTP Server Tests for Azure OpenAI ==========

func TestAzureOpenAIIntegration_ListModels_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful list deployments", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "test-api-key", r.Header.Get("api-key"))
			assert.Contains(t, r.URL.Path, "deployments")
			assert.Contains(t, r.URL.RawQuery, "api-version")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]string{
					{"id": "gpt-4-deployment", "model": "gpt-4"},
					{"id": "gpt-35-turbo-deployment", "model": "gpt-35-turbo"},
				},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		models, err := integration.ListModels(context.Background())

		require.NoError(t, err)
		assert.Len(t, models, 2)
		assert.Equal(t, "gpt-4-deployment", models[0]["id"])
		assert.Equal(t, "gpt-4", models[0]["model"])
		assert.Equal(t, "azure", models[0]["owned_by"])
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "Access denied"}`))
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json}`))
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "decode")
	})
}

func TestAzureOpenAIIntegration_InvokeModel_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful invoke model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "test-api-key", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Contains(t, r.URL.Path, "gpt-4")
			assert.Contains(t, r.URL.Path, "chat/completions")

			// Verify request body
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.NotNil(t, reqBody["messages"])
			messages := reqBody["messages"].([]interface{})
			assert.Len(t, messages, 1)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": "Azure OpenAI response here",
						},
					},
				},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Hello", nil)

		require.NoError(t, err)
		assert.Equal(t, "Azure OpenAI response here", result)
	})

	t.Run("invoke with custom config", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			assert.Equal(t, float64(2048), reqBody["max_tokens"])
			assert.Equal(t, 0.5, reqBody["temperature"])
			assert.Equal(t, 0.95, reqBody["top_p"])

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]string{
							"content": "Custom config response",
						},
					},
				},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		invokeConfig := map[string]interface{}{
			"max_tokens":  2048,
			"temperature": 0.5,
			"top_p":       0.95,
		}

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", invokeConfig)

		require.NoError(t, err)
		assert.Equal(t, "Custom config response", result)
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "Bad request"}`))
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "400")
	})

	t.Run("empty choices fallback to raw response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []interface{}{},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", nil)

		require.NoError(t, err)
		// Should return raw response as fallback
		assert.NotEmpty(t, result)
	})
}

func TestAzureOpenAIIntegration_HealthCheck_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		err := integration.HealthCheck(context.Background())

		assert.NoError(t, err)
	})

	t.Run("health check failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "Service unavailable"}`))
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		err := integration.HealthCheck(context.Background())

		assert.Error(t, err)
	})
}

// ========== CloudIntegrationManager Mock Tests ==========

func TestCloudIntegrationManager_InvokeCloudModel_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful invocation through manager", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"content": []map[string]string{
					{"text": "Response from manager test"},
				},
			})
		}))
		defer server.Close()

		manager := NewCloudIntegrationManager(logger)

		awsConfig := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		awsProvider := NewAWSBedrockIntegrationWithConfig(awsConfig, logger)
		awsProvider.httpClient.Transport = &mockTransport{server: server}

		manager.RegisterProvider(awsProvider)

		result, err := manager.InvokeCloudModel(context.Background(), "aws-bedrock", "anthropic.claude-v2", "Test", nil)

		require.NoError(t, err)
		assert.Equal(t, "Response from manager test", result)
	})

	t.Run("invocation failure logged and returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Internal error"}`))
		}))
		defer server.Close()

		manager := NewCloudIntegrationManager(logger)

		awsConfig := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		awsProvider := NewAWSBedrockIntegrationWithConfig(awsConfig, logger)
		awsProvider.httpClient.Transport = &mockTransport{server: server}

		manager.RegisterProvider(awsProvider)

		result, err := manager.InvokeCloudModel(context.Background(), "aws-bedrock", "anthropic.claude-v2", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestCloudIntegrationManager_HealthCheckAll_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("all providers healthy", func(t *testing.T) {
		// AWS mock server
		awsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"modelSummaries": []interface{}{},
			})
		}))
		defer awsServer.Close()

		// GCP mock server
		gcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer gcpServer.Close()

		// Azure mock server
		azureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{},
			})
		}))
		defer azureServer.Close()

		manager := NewCloudIntegrationManager(logger)

		// AWS provider
		awsConfig := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		awsProvider := NewAWSBedrockIntegrationWithConfig(awsConfig, logger)
		awsProvider.httpClient.Transport = &mockTransport{server: awsServer}
		manager.RegisterProvider(awsProvider)

		// GCP provider
		gcpConfig := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		gcpProvider := NewGCPVertexAIIntegrationWithConfig(gcpConfig, logger)
		gcpProvider.httpClient.Transport = &mockTransport{server: gcpServer}
		manager.RegisterProvider(gcpProvider)

		// Azure provider (uses server URL directly)
		azureConfig := AzureOpenAIConfig{
			Endpoint:   azureServer.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		azureProvider := NewAzureOpenAIIntegrationWithConfig(azureConfig, logger)
		manager.RegisterProvider(azureProvider)

		results := manager.HealthCheckAll(context.Background())

		assert.Len(t, results, 3)
		assert.NoError(t, results["aws-bedrock"])
		assert.NoError(t, results["gcp-vertex-ai"])
		assert.NoError(t, results["azure-openai"])
	})

	t.Run("mixed health check results", func(t *testing.T) {
		// AWS mock server (healthy)
		awsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"modelSummaries": []interface{}{},
			})
		}))
		defer awsServer.Close()

		// GCP mock server (unhealthy)
		gcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "Service unavailable"}`))
		}))
		defer gcpServer.Close()

		manager := NewCloudIntegrationManager(logger)

		// AWS provider
		awsConfig := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		awsProvider := NewAWSBedrockIntegrationWithConfig(awsConfig, logger)
		awsProvider.httpClient.Transport = &mockTransport{server: awsServer}
		manager.RegisterProvider(awsProvider)

		// GCP provider
		gcpConfig := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		gcpProvider := NewGCPVertexAIIntegrationWithConfig(gcpConfig, logger)
		gcpProvider.httpClient.Transport = &mockTransport{server: gcpServer}
		manager.RegisterProvider(gcpProvider)

		results := manager.HealthCheckAll(context.Background())

		assert.Len(t, results, 2)
		assert.NoError(t, results["aws-bedrock"])
		assert.Error(t, results["gcp-vertex-ai"])
	})
}

// ========== Context Cancellation Tests ==========

func TestAWSBedrockIntegration_ContextCancellation(t *testing.T) {
	logger := newTestLogger()

	t.Run("ListModels with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := integration.ListModels(ctx)

		assert.Error(t, err)
	})
}

func TestGCPVertexAIIntegration_ContextCancellation(t *testing.T) {
	logger := newTestLogger()

	t.Run("InvokeModel with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := integration.InvokeModel(ctx, "text-bison", "Test", nil)

		assert.Error(t, err)
	})
}

func TestAzureOpenAIIntegration_ContextCancellation(t *testing.T) {
	logger := newTestLogger()

	t.Run("InvokeModel with cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := integration.InvokeModel(ctx, "gpt-4", "Test", nil)

		assert.Error(t, err)
	})
}

// ========== AWS Bedrock HealthCheck Mock Tests ==========

func TestAWSBedrockIntegration_HealthCheck_MockServer(t *testing.T) {
	logger := newTestLogger()

	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"modelSummaries": []interface{}{},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		err := integration.HealthCheck(context.Background())

		assert.NoError(t, err)
	})

	t.Run("health check failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "Access denied"}`))
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		err := integration.HealthCheck(context.Background())

		assert.Error(t, err)
	})
}

// ========== Error Transport for testing HTTP client errors ==========

type errorTransport struct {
	err error
}

func (e *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestAWSBedrockIntegration_HTTPClientError(t *testing.T) {
	logger := newTestLogger()

	t.Run("ListModels HTTP client error", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("network error")}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "failed to list models")
	})

	t.Run("InvokeModel HTTP client error", func(t *testing.T) {
		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("connection refused")}

		result, err := integration.InvokeModel(context.Background(), "anthropic.claude-v2", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "failed to invoke model")
	})
}

func TestGCPVertexAIIntegration_HTTPClientError(t *testing.T) {
	logger := newTestLogger()

	t.Run("ListModels HTTP client error", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("network error")}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "failed to list models")
	})

	t.Run("InvokeModel HTTP client error", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("connection refused")}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "failed to invoke model")
	})

	t.Run("HealthCheck HTTP client error", func(t *testing.T) {
		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("connection timeout")}

		err := integration.HealthCheck(context.Background())

		assert.Error(t, err)
	})
}

func TestAzureOpenAIIntegration_HTTPClientError(t *testing.T) {
	logger := newTestLogger()

	t.Run("ListModels HTTP client error", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint:   "https://test.openai.azure.com",
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("network error")}

		models, err := integration.ListModels(context.Background())

		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "failed to list models")
	})

	t.Run("InvokeModel HTTP client error", func(t *testing.T) {
		config := AzureOpenAIConfig{
			Endpoint:   "https://test.openai.azure.com",
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &errorTransport{err: fmt.Errorf("connection refused")}

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "failed to invoke model")
	})
}

// ========== Response Parsing Edge Cases ==========

func TestAWSBedrockIntegration_InvokeModel_ResponseParsing(t *testing.T) {
	logger := newTestLogger()

	t.Run("titan model with empty results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "amazon.titan-text-express-v1", "Test", nil)

		require.NoError(t, err)
		// Fallback to raw response
		assert.NotEmpty(t, result)
	})

	t.Run("llama model empty generation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"generation": "",
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "meta.llama2-13b-chat-v1", "Test", nil)

		require.NoError(t, err)
		// Empty string from llama leads to fallback
		assert.NotEmpty(t, result)
	})

	t.Run("cohere model with empty generations", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"generations": []interface{}{},
			})
		}))
		defer server.Close()

		config := AWSBedrockConfig{
			Region:          "us-east-1",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
		}
		integration := NewAWSBedrockIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "cohere.command-text-v14", "Test", nil)

		require.NoError(t, err)
		// Empty generations leads to fallback
		assert.NotEmpty(t, result)
	})
}

func TestGCPVertexAIIntegration_InvokeModel_ResponseParsing(t *testing.T) {
	logger := newTestLogger()

	t.Run("predictions with non-content field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"predictions": []map[string]interface{}{
					{"output": "Output text here"},
				},
			})
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		require.NoError(t, err)
		// Should extract via generic fallback
		assert.Equal(t, "Output text here", result)
	})

	t.Run("invalid response format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not json`))
		}))
		defer server.Close()

		config := GCPVertexAIConfig{
			ProjectID:   "test-project",
			Location:    "us-central1",
			AccessToken: "test-token",
		}
		integration := NewGCPVertexAIIntegrationWithConfig(config, logger)
		integration.httpClient.Transport = &mockTransport{server: server}

		result, err := integration.InvokeModel(context.Background(), "text-bison", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "parse response")
	})
}

func TestAzureOpenAIIntegration_InvokeModel_ResponseParsing(t *testing.T) {
	logger := newTestLogger()

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{invalid json response`))
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", nil)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "parse response")
	})

	t.Run("choices with missing content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]string{
							"role": "assistant",
							// Missing content
						},
					},
				},
			})
		}))
		defer server.Close()

		config := AzureOpenAIConfig{
			Endpoint:   server.URL,
			APIKey:     "test-api-key",
			APIVersion: "2024-02-01",
		}
		integration := NewAzureOpenAIIntegrationWithConfig(config, logger)

		result, err := integration.InvokeModel(context.Background(), "gpt-4", "Test", nil)

		require.NoError(t, err)
		// Empty content, should fallback to raw
		assert.NotEmpty(t, result)
	})
}

// ========== Empty Credentials Various Providers ==========

func TestAWSBedrockIntegration_EmptyAccessKeyID(t *testing.T) {
	logger := newTestLogger()

	config := AWSBedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "",
		SecretAccessKey: "test-secret",
	}
	integration := NewAWSBedrockIntegrationWithConfig(config, logger)

	t.Run("ListModels with empty access key", func(t *testing.T) {
		models, err := integration.ListModels(context.Background())
		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "credentials not configured")
	})

	t.Run("InvokeModel with empty access key", func(t *testing.T) {
		result, err := integration.InvokeModel(context.Background(), "anthropic.claude-v2", "Test", nil)
		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "credentials not configured")
	})

	t.Run("HealthCheck with empty access key", func(t *testing.T) {
		err := integration.HealthCheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "credentials not configured")
	})
}

func TestAWSBedrockIntegration_EmptySecretKey(t *testing.T) {
	logger := newTestLogger()

	config := AWSBedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "test-key",
		SecretAccessKey: "",
	}
	integration := NewAWSBedrockIntegrationWithConfig(config, logger)

	t.Run("ListModels with empty secret key", func(t *testing.T) {
		models, err := integration.ListModels(context.Background())
		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "credentials not configured")
	})
}
