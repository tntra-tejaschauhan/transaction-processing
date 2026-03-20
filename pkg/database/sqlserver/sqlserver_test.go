package sqlserver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ── Fake driver (enables testing NewDB success path without a real SQL Server) ─

type fakeSQLServerDriver struct{}
type fakeSQLServerConn struct{}

func (fakeSQLServerDriver) Open(_ string) (driver.Conn, error) { return fakeSQLServerConn{}, nil }
func (fakeSQLServerConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (fakeSQLServerConn) Close() error                          { return nil }
func (fakeSQLServerConn) Begin() (driver.Tx, error)             { return nil, nil }
func (fakeSQLServerConn) Ping(_ context.Context) error          { return nil }

func init() {
	sql.Register("fakesqlserver", fakeSQLServerDriver{})
}

func swapDriver(t *testing.T, name string) {
	t.Helper()
	orig := driverName
	driverName = name
	t.Cleanup(func() { driverName = orig })
}

func parseOnlyDB(cfg Config) (*sql.DB, error) {
	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlserver parse only db: %w", err)
	}
	applyPoolConfig(db, cfg)
	return db, nil
}

type testSuiteSQLServer struct {
	suite.Suite
}

func TestSQLServer(t *testing.T) {
	suite.Run(t, new(testSuiteSQLServer))
}

// ── NewDB ─────────────────────────────────────────────────────────────────────

func (s *testSuiteSQLServer) TestNewDB_Success() {
	s.Run("when DSN is reachable the pool is returned", func() {
		swapDriver(s.T(), "fakesqlserver")

		db, err := NewDB(context.Background(), Config{
			DSN:          "anything",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			ConnLifetime: 15 * time.Minute,
		})
		s.Require().NoError(err)
		s.Require().NotNil(db)

		stats := db.Stats()
		s.Assert().Equal(10, stats.MaxOpenConnections)
		s.Require().NoError(Close(db))
	})
}

func (s *testSuiteSQLServer) TestNewDB_PingFails() {
	s.Run("when host is unreachable NewDB returns a ping error", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err := NewDB(ctx, Config{DSN: "sqlserver://user:pass@127.0.0.1:1?database=mydb"})
		s.Assert().Error(err)
	})
}

func (s *testSuiteSQLServer) TestNewDB_EmptyDSN() {
	s.Run("when DSN is empty NewDB returns a ping error", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		_, err := NewDB(ctx, Config{DSN: ""})
		s.Assert().Error(err)
	})
}

func (s *testSuiteSQLServer) TestNewDB_PoolSettings() {
	s.Run("when pool settings are applied", func() {
		cfg := Config{
			DSN:          "sqlserver://user:pass@localhost:1433?database=mydb",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			ConnLifetime: 15 * time.Minute,
		}
		db, err := parseOnlyDB(cfg)
		s.Require().NoError(err)
		s.Require().NotNil(db)
		stats := db.Stats()
		s.Assert().Equal(10, stats.MaxOpenConnections)
		s.Require().NoError(Close(db))
	})
}

// ── Close ─────────────────────────────────────────────────────────────────────

func (s *testSuiteSQLServer) TestClose_Nil() {
	s.Run("when db is nil Close returns nil", func() {
		s.Require().NoError(Close(nil))
	})
}
