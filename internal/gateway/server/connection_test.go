package server

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

// Note: TestMain with goleak is already declared in server_test.go.
// Both files share the same package — only one TestMain per package.

// testConn builds a Conn backed by the client side of a net.Pipe().
// Returns the Conn and the client end of the pipe for test interaction.
func testConn(opts *ServerOptions) (*Conn, net.Conn) {
	client, server := net.Pipe()
	c := newConn(server, opts, zerolog.Nop())
	return c, client
}

// testConnOpts returns ServerOptions suitable for connection tests.
func testConnOpts() *ServerOptions {
	return &ServerOptions{
		Port:            8583,
		ReadTimeout:     2 * time.Second,
		WriteTimeout:    2 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 2 * time.Second,
	}
}

// writeFrame writes a framed message to w: 2-byte big-endian length + body.
// Helper used by tests to simulate a card network client sending a message.
func writeFrame(w io.Writer, body []byte) error {
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(body)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// readFrame reads a framed message from r: 2-byte length prefix + body.
// Helper used by tests to read the server's response.
func readFrame(r io.Reader) ([]byte, error) {
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint16(lenBuf)
	body := make([]byte, msgLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// ── Test suite ────────────────────────────────────────────────────────────────

type testSuiteConn struct {
	suite.Suite
}

func TestConn(t *testing.T) {
	suite.Run(t, new(testSuiteConn))
}

// ── Framing — write ───────────────────────────────────────────────────────────

func (s *testSuiteConn) TestWrite_FramesDataWithLengthPrefix() {
	s.Run("when write is called then response is prefixed with 2-byte big-endian length", func() {
		c, client := testConn(testConnOpts())
		defer client.Close()

		payload := []byte("hello")
		done := make(chan error, 1)
		go func() {
			done <- c.write(payload)
		}()

		received, err := readFrame(client)
		s.Require().NoError(err)
		s.Assert().Equal(payload, received)
		s.Assert().NoError(<-done)
	})
}

func (s *testSuiteConn) TestWrite_SetsWriteDeadline() {
	s.Run("when write deadline elapses before client reads then write returns error", func() {
		opts := testConnOpts()
		opts.WriteTimeout = 1 * time.Millisecond // very short deadline
		c, client := testConn(opts)
		defer client.Close()

		// Do not read from client — write will time out.
		// net.Pipe is synchronous: Write blocks until the other side reads.
		err := c.write([]byte("data"))
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "write frame")
	})
}

// ── Framing — read ────────────────────────────────────────────────────────────

func (s *testSuiteConn) TestReadLoop_ReceivesFrameAndEchoes() {
	s.Run("when client sends a valid frame then server echoes it back", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())

		// Run handle in background — it drives the read loop.
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		payload := []byte("test-message")
		s.Require().NoError(writeFrame(client, payload))

		// Read the echoed response.
		received, err := readFrame(client)
		s.Require().NoError(err)
		s.Assert().Equal(payload, received)

		// Cancel context → handle exits cleanly.
		cancel()
		client.Close()
		<-handleDone
	})
}

func (s *testSuiteConn) TestReadLoop_GracefulClientDisconnect() {
	s.Run("when client closes connection then handle exits without error log", func() {
		c, client := testConn(testConnOpts())

		ctx := context.Background()
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Close the client side — server sees io.EOF.
		client.Close()

		select {
		case <-handleDone:
			// handle exited cleanly — correct
		case <-time.After(2 * time.Second):
			s.Fail("handle did not exit after client disconnect")
		}
	})
}

func (s *testSuiteConn) TestReadLoop_ContextCancelExitsCleanly() {
	s.Run("when context is cancelled then read loop exits without error", func() {
		c, client := testConn(testConnOpts())
		defer client.Close()

		ctx, cancel := context.WithCancel(context.Background())

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		cancel() // trigger cancellation

		select {
		case <-handleDone:
			// OK
		case <-time.After(2 * time.Second):
			s.Fail("handle did not exit after context cancel")
		}
	})
}

func (s *testSuiteConn) TestReadLoop_ZeroLengthFrameReturnsError() {
	s.Run("when client sends a zero-length frame then handle exits with error", func() {
		c, client := testConn(testConnOpts())

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(context.Background())
		}()

		// Send a zero-length frame.
		_, err := client.Write([]byte{0x00, 0x00})
		s.Require().NoError(err)
		client.Close()

		select {
		case <-handleDone:
			// handle detected zero-length frame and exited
		case <-time.After(2 * time.Second):
			s.Fail("handle did not exit after zero-length frame")
		}
	})
}

func (s *testSuiteConn) TestReadLoop_ReadDeadlineEnforced() {
	s.Run("when client sends prefix but no body then read times out", func() {
		opts := testConnOpts()
		opts.ReadTimeout = 100 * time.Millisecond
		c, client := testConn(opts)

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(context.Background())
		}()

		// Send only the length prefix — no body follows.
		_, err := client.Write([]byte{0x00, 0x05})
		s.Require().NoError(err)
		// Do not send the body — read deadline should fire.

		select {
		case <-handleDone:
			// handle timed out reading body and exited — correct
		case <-time.After(time.Second):
			s.Fail("handle did not exit after read timeout")
		}

		client.Close()
	})
}

// ── isGracefulClose ───────────────────────────────────────────────────────────

func (s *testSuiteConn) TestIsGracefulClose_EOF() {
	s.Run("when error is io.EOF then isGracefulClose returns true", func() {
		s.Assert().True(isGracefulClose(io.EOF))
	})
}

func (s *testSuiteConn) TestIsGracefulClose_OtherError() {
	s.Run("when error is not EOF or ErrClosed then isGracefulClose returns false", func() {
		_, err := net.Dial("tcp", "127.0.0.1:1") // port 1 always refused
		s.Assert().False(isGracefulClose(err))
	})
}
