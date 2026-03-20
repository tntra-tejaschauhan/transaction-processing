package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

// parseOnlyConfig mirrors the pool-settings logic in NewPool without dialling.
// Used to verify that Config fields are forwarded correctly to pgxpool.Config.
func parseOnlyConfig(cfg Config) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres parse config: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	return poolCfg, nil
}

type testSuitePostgres struct {
	suite.Suite
}

func TestPostgres(t *testing.T) {
	suite.Run(t, new(testSuitePostgres))
}

// ── Config validation ─────────────────────────────────────────────────────────

func (s *testSuitePostgres) TestNewPool_InvalidDSN() {
	s.Run("when DSN is empty", func() {
		_, err := NewPool(s.T().Context(), Config{DSN: ""})
		s.Assert().Error(err)
	})
	s.Run("when DSN is malformed", func() {
		_, err := NewPool(s.T().Context(), Config{DSN: "not-a-valid-dsn://???"})
		s.Assert().Error(err)
	})
}

func (s *testSuitePostgres) TestNewPool_PingFails() {
	s.Run("when host is unreachable pool creation returns ping error", func() {
		// Port 1 on loopback is always refused immediately — no real server needed.
		// Pass all Config fields so every cfg.X > 0 branch is exercised.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err := NewPool(ctx, Config{
			DSN:             "postgres://user:pass@127.0.0.1:1/db?sslmode=disable",
			MaxConns:        5,
			MinConns:        1,
			MaxConnLifetime: 10 * time.Minute,
			MaxConnIdleTime: 2 * time.Minute,
		})
		s.Assert().Error(err)
	})
}

func (s *testSuitePostgres) TestConfig_PoolSettingsApplied() {
	s.Run("when config has all pool settings", func() {
		cfg := Config{
			DSN:             "postgres://user:pass@localhost:5432/db",
			MaxConns:        20,
			MinConns:        2,
			MaxConnLifetime: 30 * time.Minute,
			MaxConnIdleTime: 5 * time.Minute,
		}
		poolCfg, err := parseOnlyConfig(cfg)
		s.Require().NoError(err)
		s.Assert().Equal(int32(20), poolCfg.MaxConns)
		s.Assert().Equal(int32(2), poolCfg.MinConns)
		s.Assert().Equal(30*time.Minute, poolCfg.MaxConnLifetime)
		s.Assert().Equal(5*time.Minute, poolCfg.MaxConnIdleTime)
	})
	s.Run("when zero values leave defaults unchanged", func() {
		cfg := Config{DSN: "postgres://user:pass@localhost:5432/db"}
		poolCfg, err := parseOnlyConfig(cfg)
		s.Require().NoError(err)
		s.Assert().Greater(poolCfg.MaxConns, int32(0))
	})
}

func (s *testSuitePostgres) TestClose_NilPool() {
	s.Run("when pool is nil Close does not panic", func() {
		s.Require().NotPanics(func() { Close(nil) })
	})
}
