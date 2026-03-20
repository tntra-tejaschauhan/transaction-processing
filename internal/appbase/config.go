package appbase

import "github.com/ilyakaznacheev/cleanenv"

// HSMConfig holds GCP Cloud HSM key ring and key name configuration.
type HSMConfig struct {
	KeyRing  string `env:"GCP_HSM_KEY_RING"`
	KeyNames string `env:"GCP_HSM_KEY_NAMES"`
}
type GatewayConfig struct {
	Port              int `env:"GATEWAY_PORT"                env-default:"8583"`
	ReadTimeoutMs     int `env:"GATEWAY_READ_TIMEOUT_MS"     env-default:"30000"`
	WriteTimeoutMs    int `env:"GATEWAY_WRITE_TIMEOUT_MS"    env-default:"30000"`
	MaxConnections    int `env:"GATEWAY_MAX_CONNECTIONS"     env-default:"10000"`
	ShutdownTimeoutMs int `env:"GATEWAY_SHUTDOWN_TIMEOUT_MS" env-default:"5000"`
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
