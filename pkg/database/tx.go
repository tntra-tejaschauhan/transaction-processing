package database

import (
	"context"
	"database/sql"
	"fmt"
)

// RunInTx executes fn inside a database transaction obtained from db.
// If fn returns an error the transaction is rolled back; otherwise it is
// committed. The tx passed to fn must not be used after fn returns.
//
// RunInTx works with any *sql.DB (both PostgreSQL via pgx/stdlib and SQL Server
// via go-mssqldb).
func RunInTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("run in tx: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("run in tx: rollback (%v) after: %w", rbErr, err)
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err // deferred rollback handles cleanup
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("run in tx: commit: %w", err)
	}
	return nil
}
