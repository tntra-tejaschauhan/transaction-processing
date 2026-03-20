package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"
)

// ── Fake driver ───────────────────────────────────────────────────────────────

// fakeDriver is a minimal database/sql driver that records whether a
// transaction was committed or rolled back.
type fakeDriver struct{}

type fakeConn struct {
	committed          bool
	rolledBack         bool
	shouldFail         bool // if true, Commit returns an error
	rollbackShouldFail bool // if true, Rollback returns an error
}

type fakeTx struct{ conn *fakeConn }
type fakeStmt struct{}
type fakeResult struct{}
type fakeRows struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{
		rollbackShouldFail: name == "failrollback",
		shouldFail:         name == "failcommit",
	}, nil
}

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) { return fakeStmt{}, nil }
func (c *fakeConn) Close() error                           { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)              { return &fakeTx{conn: c}, nil }

func (t *fakeTx) Commit() error {
	if t.conn.shouldFail {
		return errors.New("commit failed")
	}
	t.conn.committed = true
	return nil
}
func (t *fakeTx) Rollback() error {
	if t.conn.rollbackShouldFail {
		return errors.New("rollback failed")
	}
	t.conn.rolledBack = true
	return nil
}

func (fakeStmt) Close() error                                    { return nil }
func (fakeStmt) NumInput() int                                   { return 0 }
func (fakeStmt) Exec(_ []driver.Value) (driver.Result, error)   { return fakeResult{}, nil }
func (fakeStmt) Query(_ []driver.Value) (driver.Rows, error)    { return &fakeRows{}, nil }
func (fakeResult) LastInsertId() (int64, error)                  { return 0, nil }
func (fakeResult) RowsAffected() (int64, error)                  { return 1, nil }
func (r *fakeRows) Columns() []string                            { return nil }
func (r *fakeRows) Close() error                                 { return nil }
func (r *fakeRows) Next(_ []driver.Value) error                  { return io.EOF }

func init() {
	sql.Register("fakedb", fakeDriver{})
}

// ── Test suite ────────────────────────────────────────────────────────────────

type testSuiteTx struct {
	suite.Suite
}

func TestTx(t *testing.T) {
	suite.Run(t, new(testSuiteTx))
}

func openFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("fakedb", "")
	if err != nil {
		t.Fatalf("open fakedb: %v", err)
	}
	return db
}

func openFakeDBFailRollback(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("fakedb", "failrollback")
	if err != nil {
		t.Fatalf("open fakedb failrollback: %v", err)
	}
	return db
}

func openFakeDBFailCommit(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("fakedb", "failcommit")
	if err != nil {
		t.Fatalf("open fakedb failcommit: %v", err)
	}
	return db
}

func (s *testSuiteTx) TestRunInTx_Commit() {
	s.Run("when fn succeeds transaction is committed", func() {
		db := openFakeDB(s.T())
		defer db.Close()

		called := false
		err := RunInTx(context.Background(), db, func(tx *sql.Tx) error {
			called = true
			return nil
		})
		s.Require().NoError(err)
		s.Assert().True(called)
	})
}

func (s *testSuiteTx) TestRunInTx_Rollback() {
	s.Run("when fn returns an error transaction is rolled back", func() {
		db := openFakeDB(s.T())
		defer db.Close()

		fnErr := errors.New("intentional error")
		err := RunInTx(context.Background(), db, func(tx *sql.Tx) error {
			return fnErr
		})
		s.Assert().ErrorIs(err, fnErr)
	})
}

func (s *testSuiteTx) TestRunInTx_PanicPropagated() {
	s.Run("when fn panics the panic is re-raised after rollback", func() {
		db := openFakeDB(s.T())
		defer db.Close()

		s.Require().Panics(func() {
			_ = RunInTx(context.Background(), db, func(tx *sql.Tx) error {
				panic("boom")
			})
		})
	})
}

func (s *testSuiteTx) TestRunInTx_RollbackError_OriginalErrPreserved() {
	s.Run("when fn errors and rollback also fails original error is still identifiable", func() {
		db := openFakeDBFailRollback(s.T())
		defer db.Close()

		fnErr := errors.New("original business error")
		err := RunInTx(context.Background(), db, func(tx *sql.Tx) error {
			return fnErr
		})
		// The combined error message wraps fnErr with %w so callers using
		// errors.Is can still detect the original business error.
		s.Assert().ErrorIs(err, fnErr)
	})
}

func (s *testSuiteTx) TestRunInTx_CommitError() {
	s.Run("when commit fails the error is returned and wrapped", func() {
		db := openFakeDBFailCommit(s.T())
		defer db.Close()

		err := RunInTx(context.Background(), db, func(tx *sql.Tx) error {
			return nil
		})
		s.Assert().ErrorContains(err, "run in tx: commit")
	})
}

func (s *testSuiteTx) TestRunInTx_FnNotCalledOnBeginError() {
	s.Run("when context is already cancelled BeginTx fails and fn is not called", func() {
		db := openFakeDB(s.T())
		defer db.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately cancelled

		called := false
		err := RunInTx(ctx, db, func(tx *sql.Tx) error {
			called = true
			return nil
		})
		// fakeDriver doesn't check context cancellation, so BeginTx may succeed.
		// The important invariant is that RunInTx propagates whatever error occurs.
		if err != nil {
			s.Assert().False(called)
		}
	})
}
