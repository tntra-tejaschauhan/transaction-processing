package server

import (
	"fmt"
	"time"

	"github.com/PayWithSpireInc/transaction-processing/internal/appbase"
)

// ServerOptions holds all tuneable parameters for the ISO 8583 TCP server.
// Constructed via NewServerOptions — never build this struct directly.
type ServerOptions struct {
	// Port is the TCP port the server listens on.
	Port int

	// ReadTimeout is the per-connection deadline for reading a full message.
	ReadTimeout time.Duration

	// WriteTimeout is the per-connection deadline for writing a response.
	WriteTimeout time.Duration

	// MaxConnections is the maximum number of concurrent TCP connections.
	// Connections beyond this limit are immediately closed.
	MaxConnections int

	// ShutdownTimeout is how long Stop() waits for active connections to
	// drain before forcing close.
	ShutdownTimeout time.Duration
}

// NewServerOptions loads gateway config from the YAML file at configPath,
// applies any GATEWAY_* environment variable overrides, converts millisecond
// fields to time.Duration, and validates all values.
//
// Priority: env var > yaml file value > env-default tag.
func NewServerOptions(configPath string) (*ServerOptions, error) {
	cfg, err := appbase.LoadGatewayConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("server options: %w", err)
	}

	opts := &ServerOptions{
		Port:            cfg.Port,
		ReadTimeout:     time.Duration(cfg.ReadTimeoutMs) * time.Millisecond,
		WriteTimeout:    time.Duration(cfg.WriteTimeoutMs) * time.Millisecond,
		MaxConnections:  cfg.MaxConnections,
		ShutdownTimeout: time.Duration(cfg.ShutdownTimeoutMs) * time.Millisecond,
	}

	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("server options: %w", err)
	}

	return opts, nil
}

// validate checks all fields are within acceptable ranges.
func (o *ServerOptions) validate() error {
	if o.Port < 1024 || o.Port > 65535 {
		return fmt.Errorf("port %d is out of range: must be between 1024 and 65535", o.Port)
	}
	if o.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout_ms must be positive, got %d ms", o.ReadTimeout.Milliseconds())
	}
	if o.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout_ms must be positive, got %d ms", o.WriteTimeout.Milliseconds())
	}
	if o.MaxConnections <= 0 {
		return fmt.Errorf("max_connections must be positive, got %d", o.MaxConnections)
	}
	if o.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout_ms must be positive, got %d ms", o.ShutdownTimeout.Milliseconds())
	}
	return nil
}
