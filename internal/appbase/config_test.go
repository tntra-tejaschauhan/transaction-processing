package appbase

import (
	"os"
	"testing"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any env vars that might interfere.
	os.Unsetenv("APP_ENV")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("GATEWAY_PORT")
	os.Unsetenv("GATEWAY_MAX_CONNECTIONS")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	require.Equal(t, "development", cfg.Env)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, 8583, cfg.Gateway.Port)
	require.Equal(t, 10000, cfg.Gateway.MaxConnections)
	require.Equal(t, 30000, cfg.Gateway.ReadTimeoutMs)
	require.Equal(t, 30000, cfg.Gateway.WriteTimeoutMs)
	require.Equal(t, 5000, cfg.Gateway.ShutdownTimeoutMs)
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("GATEWAY_PORT", "9999")
	t.Setenv("GATEWAY_MAX_CONNECTIONS", "500")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	require.Equal(t, "production", cfg.Env)
	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, 9999, cfg.Gateway.Port)
	require.Equal(t, 500, cfg.Gateway.MaxConnections)
}