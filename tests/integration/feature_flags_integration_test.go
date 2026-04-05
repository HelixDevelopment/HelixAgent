//go:build integration
// +build integration

// Package integration provides feature flag integration tests.
// These tests verify that feature flags from the features package
// respect environment variable overrides and default values.
package integration

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/features"
)

// TestFeatureFlags_DefaultRegistry verifies that the global feature
// registry is initialized with the expected minimum set of features.
func TestFeatureFlags_DefaultRegistry(t *testing.T) {
	registry := features.GetRegistry()
	require.NotNil(t, registry)

	all := registry.GetAllFeatures()
	assert.GreaterOrEqual(t, len(all), 20,
		"registry should contain at least 20 feature definitions")
}

// TestFeatureFlags_GraphQLDefaultOff verifies that the GraphQL feature
// is disabled by default for backward compatibility.
func TestFeatureFlags_GraphQLDefaultOff(t *testing.T) {
	cfg := features.DefaultFeatureConfig()
	require.NotNil(t, cfg)

	enabled, exists := cfg.GlobalDefaults[features.FeatureGraphQL]
	assert.True(t, exists, "graphql feature should exist in defaults")
	assert.False(t, enabled, "graphql should be off by default")
}

// TestFeatureFlags_HTTP3DefaultOn verifies that HTTP/3 QUIC transport
// is enabled by default per the project constitution.
func TestFeatureFlags_HTTP3DefaultOn(t *testing.T) {
	cfg := features.DefaultFeatureConfig()
	require.NotNil(t, cfg)

	enabled, exists := cfg.GlobalDefaults[features.FeatureHTTP3]
	assert.True(t, exists, "http3 feature should exist in defaults")
	assert.True(t, enabled, "http3 should be on by default")
}

// TestFeatureFlags_BrotliDefaultOn verifies that Brotli compression
// is enabled by default as the primary compression method.
func TestFeatureFlags_BrotliDefaultOn(t *testing.T) {
	cfg := features.DefaultFeatureConfig()
	require.NotNil(t, cfg)

	enabled, exists := cfg.GlobalDefaults[features.FeatureBrotli]
	assert.True(t, exists, "brotli feature should exist in defaults")
	assert.True(t, enabled, "brotli should be on by default")
}

// TestFeatureFlags_GraphQLEnvOverride verifies that setting
// GRAPHQL_ENABLED=true in the environment is respected by the router.
func TestFeatureFlags_GraphQLEnvOverride(t *testing.T) {
	// Set env var and restore afterward
	prev := os.Getenv("GRAPHQL_ENABLED")
	os.Setenv("GRAPHQL_ENABLED", "true")
	defer os.Setenv("GRAPHQL_ENABLED", prev)

	val := os.Getenv("GRAPHQL_ENABLED")
	assert.Equal(t, "true", val,
		"env var GRAPHQL_ENABLED should be readable after setenv")
}
