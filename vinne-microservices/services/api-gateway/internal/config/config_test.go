package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DevelopmentEnvironment(t *testing.T) {
	// Test that development environment doesn't trigger production validation
	_ = os.Setenv("ENVIRONMENT", "development")
	_ = os.Setenv("SERVER_PORT", "4000")
	_ = os.Setenv("JWT_SECRET", "test-secret-for-development-testing-only-32chars")
	_ = os.Setenv("REDIS_URL", "redis://localhost:6379/0")
	defer func() {
		_ = os.Unsetenv("ENVIRONMENT")
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("JWT_SECRET")
		_ = os.Unsetenv("REDIS_URL")
	}()

	cfg, err := Load()
	require.NoError(t, err, "development environment should load without production validation errors")
	assert.NotNil(t, cfg)
	assert.Equal(t, "4000", cfg.Server.Port)
}

func TestLoad_LocalEnvironment(t *testing.T) {
	// Test that local environment loads with defaults
	_ = os.Setenv("ENVIRONMENT", "local")
	_ = os.Setenv("JWT_SECRET", "test-secret-for-local-testing-only-32chars")
	defer func() {
		_ = os.Unsetenv("ENVIRONMENT")
		_ = os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	require.NoError(t, err, "local environment should load with defaults")
	assert.NotNil(t, cfg)
	assert.Equal(t, "4000", cfg.Server.Port) // Default from setViperDefaults
}

func TestLoad_ProductionEnvironment(t *testing.T) {
	// Test that production environment requires all settings
	_ = os.Setenv("ENVIRONMENT", "production")
	defer func() {
		_ = os.Unsetenv("ENVIRONMENT")
	}()

	// Should fail without required settings
	_, err := Load()
	require.Error(t, err, "production environment should fail without required settings")
	assert.Contains(t, err.Error(), "production configuration validation failed")

	// Now set required values
	_ = os.Setenv("SERVER_PORT", "4000")
	_ = os.Setenv("JWT_SECRET", "production-secret-very-secure-and-long-32chars")
	_ = os.Setenv("REDIS_URL", "redis://prod-redis:6379/0")
	_ = os.Setenv("CACHE_ENABLED", "false") // Disable cache to avoid needing CACHE_REDIS_URL
	defer func() {
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("JWT_SECRET")
		_ = os.Unsetenv("REDIS_URL")
		_ = os.Unsetenv("CACHE_ENABLED")
	}()

	cfg, err := Load()
	require.NoError(t, err, "production environment should load with all required settings")
	assert.NotNil(t, cfg)
	assert.Equal(t, "4000", cfg.Server.Port)
}

func TestLoad_ServerPortBinding(t *testing.T) {
	// Test that SERVER_PORT environment variable is properly bound
	_ = os.Setenv("ENVIRONMENT", "development")
	_ = os.Setenv("SERVER_PORT", "8080")
	_ = os.Setenv("JWT_SECRET", "test-secret-for-port-binding-test-32chars")
	defer func() {
		_ = os.Unsetenv("ENVIRONMENT")
		_ = os.Unsetenv("SERVER_PORT")
		_ = os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "8080", cfg.Server.Port, "SERVER_PORT should be bound correctly")
}
