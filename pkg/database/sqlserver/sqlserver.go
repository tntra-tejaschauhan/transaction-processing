package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/microsoft/go-mssqldb" // register mssql driver
)

var driverName = "sqlserver"

// Config holds the parameters used to open a SQL Server database handle.
type Config struct {
	// DSN is the SQL Server connection string, e.g.
	// "sqlserver://user:pass@host:1433?database=mydb"
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	ConnLifetime time.Duration
}

// applyPoolConfig applies the optional connection-pool settings from cfg to db.
func applyPoolConfig(db *sql.DB, cfg Config) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnLifetime)
	}
}

// NewDB opens a *sql.DB backed by the SQL Server driver, applies pool settings,
// and verifies connectivity with a ping. The caller is responsible for closing
// the handle via Close when done.
func NewDB(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlserver new db: open: %w", err)
	}

	applyPoolConfig(db, cfg)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlserver new db: ping: %w", err)
	}

	return db, nil
}

// Close releases all resources associated with the database handle.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
