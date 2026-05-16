package integration

import (
	"context"
	"testing"

	testingutils "github.com/HelixDevelopment/helix_agent/Toolkit/Commons/testing"
)

func TestProviderIntegration(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.CleanupSuite()

	// Create mock providers for testing
	mockProvider := testingutils.NewMockProvider("test-provider")
	suite.RegisterProvider("test-provider", mockProvider)

	// Test provider lifecycle
	suite.TestProviderLifecycle(t, "test-provider")
}

func TestProviderCompatibility(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.CleanupSuite()

	mockProvider := testingutils.NewMockProvider("test-provider")
	suite.RegisterProvider("test-provider", mockProvider)

	// Test that the provider implements the interface correctly
	suite.TestProviderCompatibility(t, mockProvider)
}

func TestCrossProviderConsistency(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.CleanupSuite()

	// Register multiple mock providers
	provider1 := testingutils.NewMockProvider("provider1")
	provider2 := testingutils.NewMockProvider("provider2")

	suite.RegisterProvider("provider1", provider1)
	suite.RegisterProvider("provider2", provider2)

	// Test consistency across providers
	suite.TestCrossProviderConsistency(t)
}

func TestErrorHandling(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.CleanupSuite()

	mockProvider := testingutils.NewMockProvider("test-provider")
	mockProvider.SetShouldError(true) // Make it return errors
	suite.RegisterProvider("test-provider", mockProvider)

	// Test error handling
	suite.TestErrorHandling(t, mockProvider)
}

func TestMockProviderIntegration(t *testing.T) {
	// Test that mock providers work correctly in integration scenarios
	mockProvider := testingutils.NewMockProvider("integration-test")

	// Set up mock responses
	chatResp := testingutils.NewTestFixtures().ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	embedResp := testingutils.NewTestFixtures().EmbeddingResponse()
	mockProvider.SetEmbeddingResponse(embedResp)

	rerankResp := testingutils.NewTestFixtures().RerankResponse()
	mockProvider.SetRerankResponse(rerankResp)

	// Test that the mock provider behaves as expected
	if mockProvider.Name() != "integration-test" {
		t.Errorf("Expected provider name 'integration-test', got %s", mockProvider.Name())
	}

	// Test config validation
	err := mockProvider.ValidateConfig(map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected no error for mock provider config validation, got %v", err)
	}

	// Test model discovery
	models, err := mockProvider.DiscoverModels(context.TODO())
	if err != nil {
		t.Errorf("Expected no error for mock provider model discovery, got %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model from mock provider")
	}
}
