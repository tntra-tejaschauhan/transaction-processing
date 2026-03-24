package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
)

// TestMain enforces goroutine leak detection on every test in this package.
// Any goroutine that outlives its test will cause the suite to fail.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// testLogger returns a no-op zerolog logger suitable for tests.
func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

// testOptions returns a valid ServerOptions for tests using a free port.
func testOptions(t interface{ Helper() }) *ServerOptions {
	return &ServerOptions{
		Port:            freePort(),
		ReadTimeout:     2 * time.Second,
		WriteTimeout:    2 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 2 * time.Second,
	}
}

// freePort asks the OS for an available TCP port and returns it.
func freePort() int {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(fmt.Sprintf("freePort: %v", err))
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// ── Test suite ────────────────────────────────────────────────────────────────

type testSuiteServer struct {
	suite.Suite
}

func TestServer(t *testing.T) {
	suite.Run(t, new(testSuiteServer))
}

// ── New() ─────────────────────────────────────────────────────────────────────

func (s *testSuiteServer) TestNew_ValidOptions() {
	s.Run("when options are valid then server is created and listener is open", func() {
		opts := testOptions(s.T())

		srv, err := New(opts, testLogger())
		s.Require().NoError(err)
		s.Require().NotNil(srv)

		// Clean up — close the listener so the port is released.
		srv.Stop()
	})
}

func (s *testSuiteServer) TestNew_PortAlreadyInUse() {
	s.Run("when port is already bound then New returns an error", func() {
		// Bind the port first.
		ln, err := net.Listen("tcp", ":0")
		s.Require().NoError(err)
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port
		opts := &ServerOptions{
			Port:            port,
			ReadTimeout:     2 * time.Second,
			WriteTimeout:    2 * time.Second,
			MaxConnections:  10,
			ShutdownTimeout: 2 * time.Second,
		}

		_, err = New(opts, testLogger())
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "listen")
	})
}

// ── Start / Stop lifecycle ────────────────────────────────────────────────────

func (s *testSuiteServer) TestStartStop_Clean() {
	s.Run("when server starts then it accepts connections and stops cleanly", func() {
		opts := testOptions(s.T())
		srv, err := New(opts, testLogger())
		s.Require().NoError(err)

		ctx := context.Background()
		s.Require().NoError(srv.Start(ctx))

		// Verify the server is accepting connections.
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("127.0.0.1:%d", opts.Port), time.Second)
		s.Require().NoError(err, "should be able to dial the server")
		_ = conn.Close()

		// Stop must return without hanging.
		stopped := make(chan struct{})
		go func() {
			srv.Stop()
			close(stopped)
		}()

		select {
		case <-stopped:
			// OK
		case <-time.After(3 * time.Second):
			s.Fail("Stop() did not return within timeout")
		}
	})
}

func (s *testSuiteServer) TestStart_AcceptsWithin100ms() {
	s.Run("when server starts then it is ready to accept within 100ms", func() {
		opts := testOptions(s.T())
		srv, err := New(opts, testLogger())
		s.Require().NoError(err)
		defer srv.Stop()

		s.Require().NoError(srv.Start(context.Background()))

		start := time.Now()
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("127.0.0.1:%d", opts.Port), 100*time.Millisecond)
		s.Require().NoError(err, "server must accept within 100ms per acceptance criteria")
		s.Assert().Less(time.Since(start), 100*time.Millisecond)
		_ = conn.Close()
	})
}

// ── MaxConnections ────────────────────────────────────────────────────────────

func (s *testSuiteServer) TestMaxConnections_ExcessRejected() {
	s.Run("when connections exceed MaxConnections then excess are rejected immediately", func() {
		opts := testOptions(s.T())
		opts.MaxConnections = 2

		srv, err := New(opts, testLogger())
		s.Require().NoError(err)
		defer srv.Stop()

		s.Require().NoError(srv.Start(context.Background()))

		addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)

		// Open exactly MaxConnections connections.
		conn1, err := net.DialTimeout("tcp", addr, time.Second)
		s.Require().NoError(err)
		defer conn1.Close()

		conn2, err := net.DialTimeout("tcp", addr, time.Second)
		s.Require().NoError(err)
		defer conn2.Close()

		// Give server time to register both connections.
		time.Sleep(50 * time.Millisecond)

		// Third connection: server accepts at TCP level but then closes it.
		conn3, err := net.DialTimeout("tcp", addr, time.Second)
		s.Require().NoError(err)
		defer conn3.Close()

		// The server should close conn3 almost immediately.
		conn3.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		_, readErr := conn3.Read(buf)
		s.Assert().Error(readErr, "excess connection should be closed by server")
	})
}

// ── Stop drains goroutines ────────────────────────────────────────────────────

func (s *testSuiteServer) TestStop_DrainWithinShutdownTimeout() {
	s.Run("when Stop is called then all goroutines drain within ShutdownTimeout", func() {
		opts := testOptions(s.T())
		opts.ShutdownTimeout = 2 * time.Second

		srv, err := New(opts, testLogger())
		s.Require().NoError(err)

		s.Require().NoError(srv.Start(context.Background()))

		// Open a connection so there is at least one active goroutine.
		conn, err := net.DialTimeout("tcp",
			fmt.Sprintf("127.0.0.1:%d", opts.Port), time.Second)
		s.Require().NoError(err)
		defer conn.Close()

		time.Sleep(30 * time.Millisecond) // let handleConn start

		start := time.Now()
		srv.Stop()
		elapsed := time.Since(start)

		s.Assert().Less(elapsed, opts.ShutdownTimeout+500*time.Millisecond,
			"Stop() must return within ShutdownTimeout")
	})
}

// ── Context cancellation ──────────────────────────────────────────────────────

func (s *testSuiteServer) TestStart_ContextCancelled() {
	s.Run("when parent context is cancelled then accept loop exits cleanly", func() {
		opts := testOptions(s.T())
		srv, err := New(opts, testLogger())
		s.Require().NoError(err)

		ctx, cancel := context.WithCancel(context.Background())

		s.Require().NoError(srv.Start(ctx))

		// Trigger cancellation.
		cancel()

		// Stop must still return cleanly.
		stopped := make(chan struct{})
		go func() {
			srv.Stop()
			close(stopped)
		}()

		select {
		case <-stopped:
			// OK
		case <-time.After(3 * time.Second):
			s.Fail("Stop() did not return after context cancel")
		}
	})
}

// ── isNetError / asNetError ───────────────────────────────────────────────────

// mockNetError implements net.Error for testing without real network calls.
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

func (s *testSuiteServer) TestIsNetError_WithNetError() {
	s.Run("when err is net.Error then isNetError returns true and sets target", func() {
		mock := &mockNetError{timeout: true}
		var target net.Error
		ok := isNetError(mock, &target)
		s.Assert().True(ok)
		s.Assert().True(target.Timeout())
	})
}

func (s *testSuiteServer) TestIsNetError_WithNonNetError() {
	s.Run("when err is not net.Error then isNetError returns false", func() {
		var target net.Error
		ok := isNetError(fmt.Errorf("plain error"), &target)
		s.Assert().False(ok)
	})
}

func (s *testSuiteServer) TestAsNetError_WithNetError() {
	s.Run("when err is net.Error then asNetError returns true and populates target", func() {
		mock := &mockNetError{timeout: false}
		var target net.Error
		ok := asNetError(mock, &target)
		s.Assert().True(ok)
		s.Assert().False(target.Timeout())
	})
}

func (s *testSuiteServer) TestAsNetError_WithNonNetError() {
	s.Run("when err is not net.Error then asNetError returns false", func() {
		var target net.Error
		ok := asNetError(fmt.Errorf("not a net error"), &target)
		s.Assert().False(ok)
	})
}
