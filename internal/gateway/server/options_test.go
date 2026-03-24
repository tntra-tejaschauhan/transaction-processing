package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type testSuiteServerOptions struct {
	suite.Suite
}

func TestServerOptions(t *testing.T) {
	suite.Run(t, new(testSuiteServerOptions))
}

// writeConfig writes a temporary gateway.yaml for the test and returns its path.
func (s *testSuiteServerOptions) writeConfig(content string) string {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "gateway.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path
}

const validYAML = `
port: 8583
read_timeout_ms: 30000
write_timeout_ms: 30000
max_connections: 10000
shutdown_timeout_ms: 5000
`

// ── YAML loading ──────────────────────────────────────────────────────────────

func (s *testSuiteServerOptions) TestNewServerOptions_LoadsFromYAML() {
	s.Run("when yaml file is valid then options are built with correct values", func() {
		path := s.writeConfig(validYAML)

		opts, err := NewServerOptions(path)

		s.Require().NoError(err)
		s.Assert().Equal(8583, opts.Port)
		s.Assert().Equal(30*time.Second, opts.ReadTimeout)
		s.Assert().Equal(30*time.Second, opts.WriteTimeout)
		s.Assert().Equal(10000, opts.MaxConnections)
		s.Assert().Equal(5*time.Second, opts.ShutdownTimeout)
	})
}

func (s *testSuiteServerOptions) TestNewServerOptions_YAMLNotFound() {
	s.Run("when yaml file does not exist then error is returned", func() {
		_, err := NewServerOptions("/nonexistent/path/gateway.yaml")

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "server options:")
	})
}

func (s *testSuiteServerOptions) TestNewServerOptions_YAMLPartialOverride() {
	s.Run("when yaml sets only port then other fields use env-default values", func() {
		path := s.writeConfig("port: 9090\n")

		opts, err := NewServerOptions(path)

		s.Require().NoError(err)
		s.Assert().Equal(9090, opts.Port)
		// env-defaults kick in for unset fields
		s.Assert().Equal(30*time.Second, opts.ReadTimeout)
		s.Assert().Equal(10000, opts.MaxConnections)
	})
}

// ── Env var override ──────────────────────────────────────────────────────────

func (s *testSuiteServerOptions) TestNewServerOptions_EnvOverridesYAML() {
	s.Run("when GATEWAY_PORT env var is set then it overrides the yaml value", func() {
		path := s.writeConfig(validYAML) // yaml says port: 8583
		s.T().Setenv("GATEWAY_PORT", "9999")

		opts, err := NewServerOptions(path)

		s.Require().NoError(err)
		s.Assert().Equal(9999, opts.Port, "env var must override yaml value")
	})

	s.Run("when GATEWAY_MAX_CONNECTIONS env var is set then it overrides yaml", func() {
		path := s.writeConfig(validYAML) // yaml says max_connections: 10000
		s.T().Setenv("GATEWAY_MAX_CONNECTIONS", "500")

		opts, err := NewServerOptions(path)

		s.Require().NoError(err)
		s.Assert().Equal(500, opts.MaxConnections)
	})
}

// ── Validation ────────────────────────────────────────────────────────────────

func (s *testSuiteServerOptions) TestNewServerOptions_InvalidPort() {
	s.Run("when port is below 1024 then error is returned", func() {
		path := s.writeConfig("port: 80\nread_timeout_ms: 1000\nwrite_timeout_ms: 1000\nmax_connections: 1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "port")
	})

	s.Run("when port is above 65535 then error is returned", func() {
		path := s.writeConfig("port: 99999\nread_timeout_ms: 1000\nwrite_timeout_ms: 1000\nmax_connections: 1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "port")
	})
}

func (s *testSuiteServerOptions) TestNewServerOptions_InvalidTimeouts() {
	s.Run("when read_timeout_ms is negative then error is returned", func() {
		path := s.writeConfig("port: 8583\nread_timeout_ms: -1\nwrite_timeout_ms: 1000\nmax_connections: 1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "read_timeout_ms")
	})

	s.Run("when write_timeout_ms is negative then error is returned", func() {
		path := s.writeConfig("port: 8583\nread_timeout_ms: 1000\nwrite_timeout_ms: -1\nmax_connections: 1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "write_timeout_ms")
	})
}

func (s *testSuiteServerOptions) TestNewServerOptions_InvalidMaxConnections() {
	s.Run("when max_connections is negative then error is returned", func() {
		path := s.writeConfig("port: 8583\nread_timeout_ms: 1000\nwrite_timeout_ms: 1000\nmax_connections: -1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "max_connections")
	})
}

// ── Error wrapping ────────────────────────────────────────────────────────────

func (s *testSuiteServerOptions) TestNewServerOptions_ErrorWrapping() {
	s.Run("when validation fails then error is prefixed with server options context", func() {
		path := s.writeConfig("port: -1\nread_timeout_ms: 1000\nwrite_timeout_ms: 1000\nmax_connections: 1\nshutdown_timeout_ms: 1000\n")

		_, err := NewServerOptions(path)

		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "server options:")
	})
}
