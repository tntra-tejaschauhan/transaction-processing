package appbase

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

// HSMConfig holds GCP Cloud HSM key ring and key name configuration.
type HSMConfig struct {
	KeyRing  string `env:"GCP_HSM_KEY_RING"`
	KeyNames string `env:"GCP_HSM_KEY_NAMES"`
}
type GatewayConfig struct {
	Port              int `yaml:"port"                env:"GATEWAY_PORT"                env-default:"8583"`
	ReadTimeoutMs     int `yaml:"read_timeout_ms"     env:"GATEWAY_READ_TIMEOUT_MS"     env-default:"30000"`
	WriteTimeoutMs    int `yaml:"write_timeout_ms"    env:"GATEWAY_WRITE_TIMEOUT_MS"    env-default:"30000"`
	MaxConnections    int `yaml:"max_connections"     env:"GATEWAY_MAX_CONNECTIONS"     env-default:"10000"`
	ShutdownTimeoutMs int `yaml:"shutdown_timeout_ms" env:"GATEWAY_SHUTDOWN_TIMEOUT_MS" env-default:"5000"`
}

// Config is the top-level application configuration read from environment variables.
type Config struct {
	Env          string `env:"APP_ENV"      env-default:"development"`
	GCPProjectID string `env:"GCP_PROJECT_ID"`
	LogLevel     string `env:"LOG_LEVEL"    env-default:"info"`
	HSM          HSMConfig
	Gateway      GatewayConfig // ← add this line
}

// LoadConfig reads all environment variables into a Config struct using cleanenv.
func LoadConfig() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadGatewayConfig reads gateway configuration from a YAML file at configPath,
// then overwrites any field for which a GATEWAY_* env var is set.
// Priority: env var > yaml value > env-default tag.
func LoadGatewayConfig(configPath string) (*GatewayConfig, error) {
	var cfg GatewayConfig
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("load gateway config: %w", err)
	}
	return &cfg, nil
}
