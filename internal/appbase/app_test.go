package appbase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_NoOptions verifies New() with no options returns a blank AppBase.
func TestNew_NoOptions(t *testing.T) {
	app := New()
	assert.NotNil(t, app)
	assert.Nil(t, app.Config)
	assert.Nil(t, app.Injector)
	assert.Empty(t, app.ServiceName)
}

// TestNew_WithInit verifies that Init option wires up config and service name.
func TestNew_WithInit(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "debug")

	app := New(Init("test-service"))

	require.NotNil(t, app.Config)
	assert.Equal(t, "test-service", app.ServiceName)
	assert.Equal(t, "development", app.Config.Env)
}

// TestInit_ProductionLogger covers the production logger branch.
func TestInit_ProductionLogger(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "info")

	app := New(Init("prod-service"))

	require.NotNil(t, app.Config)
	assert.Equal(t, "production", app.Config.Env)
}

// TestInit_InvalidLogLevel ensures a bad log level falls back to info.
func TestInit_InvalidLogLevel(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "INVALID_LEVEL")

	app := New(Init("test-service"))
	require.NotNil(t, app.Config)
	// Should not panic — falls back to InfoLevel
}

// TestShutdown_NilInjector verifies Shutdown is safe when Injector is nil.
func TestShutdown_NilInjector(t *testing.T) {
	app := New() // no options → Injector is nil
	assert.NotPanics(t, func() {
		app.Shutdown()
	})
}

// TestShutdown_WithInjector verifies Shutdown works when Injector exists.
func TestShutdown_WithInjector(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("GCP_PROJECT_ID", "test-project")

	app := New(Init("shutdown-test"), WithDependencyInjector())

	require.NotNil(t, app.Injector)
	assert.NotPanics(t, func() {
		app.Shutdown()
	})
}

// TestLoadGatewayConfig_ValidFile verifies LoadGatewayConfig reads from YAML.
func TestLoadGatewayConfig_ValidFile(t *testing.T) {
	yaml := `
port: 9090
read_timeout_ms: 5000
write_timeout_ms: 5000
max_connections: 100
shutdown_timeout_ms: 2000
buf_size: 4096
idle_timeout_ms: 10000
`
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o644))

	cfg, err := LoadGatewayConfig(path)
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 5000, cfg.ReadTimeoutMs)
	assert.Equal(t, 100, cfg.MaxConnections)
	assert.Equal(t, 4096, cfg.BufSize)
	assert.Equal(t, 10000, cfg.IdleTimeoutMs)
}

// TestLoadGatewayConfig_MissingFile verifies an error is returned when file
// does not exist.
func TestLoadGatewayConfig_MissingFile(t *testing.T) {
	_, err := LoadGatewayConfig("/nonexistent/path/gateway.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load gateway config:")
}
