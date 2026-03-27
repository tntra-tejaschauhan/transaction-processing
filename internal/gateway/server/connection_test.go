package server

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
	"github.com/moov-io/iso8583"
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
		BufSize:         8192,
		IdleTimeout:     3 * time.Second,
	}
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
		s.Assert().Contains(err.Error(), "write length prefix")
	})
}

// ── Framing — read ────────────────────────────────────────────────────────────

func (s *testSuiteConn) TestReadLoop_ReceivesFrameAndEchoes() {
	s.Run("when client sends a valid frame then server echoes it back", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Build a real packed 0800 message.
		req := iso.EchoRequest{
			STAN:                "123456",
			NetworkMgmtInfoCode: "301",
		}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(msg.Marshal(&req))
		msg.MTI("0800")
		packed, err := msg.Pack()
		s.Require().NoError(err)

		// Send it with 2-byte length prefix using moov-io framer.
		header := iso.NewNetworkHeader()
		s.Require().NoError(header.SetLength(len(packed)))
		_, err = header.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(packed)
		s.Require().NoError(err)

		// Read the 0810 response — 2-byte prefix first.
		respHeader := iso.NewNetworkHeader()
		_, err = respHeader.ReadFrom(client)
		s.Require().NoError(err)

		respBuf := make([]byte, respHeader.Length())
		_, err = io.ReadFull(client, respBuf)
		s.Require().NoError(err)

		// Unpack and verify it is a valid 0810.
		respMsg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg.Unpack(respBuf))

		mti, err := respMsg.GetMTI()
		s.Require().NoError(err)
		s.Assert().Equal("0810", mti)

		var resp iso.EchoResponse
		s.Require().NoError(respMsg.Unmarshal(&resp))
		s.Assert().Equal("123456", resp.STAN)
		s.Assert().Equal("00", resp.ResponseCode)

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

func (s *testSuiteConn) TestReadLoop_ZeroLengthFrame_RespondsWithF39_30() {
	s.Run("when client sends zero-length frame then server responds with 0810 F39=30 and stays open", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Send a zero-length frame: 2-byte header {0x00, 0x00}.
		_, err := client.Write([]byte{0x00, 0x00})
		s.Require().NoError(err)

		// Server must respond with 0810 F39=30 — connection must stay open.
		respBody, err := readFrame(client)
		s.Require().NoError(err, "expected 0810 F39=30 response, got read error")

		respMsg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg.Unpack(respBody))
		mti, err := respMsg.GetMTI()
		s.Require().NoError(err)
		s.Assert().Equal("0810", mti)
		code, err := respMsg.GetString(39)
		s.Require().NoError(err)
		s.Assert().Equal("30", code, "F39 must be 30 for zero-length frame")

		// Verify connection is still open: send a valid 0800 and get a real 0810 back.
		req := iso.EchoRequest{STAN: "654321", NetworkMgmtInfoCode: "301"}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(msg.Marshal(&req))
		msg.MTI("0800")
		packed, err := msg.Pack()
		s.Require().NoError(err)
		header := iso.NewNetworkHeader()
		s.Require().NoError(header.SetLength(len(packed)))
		_, err = header.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(packed)
		s.Require().NoError(err)

		respBody2, err := readFrame(client)
		s.Require().NoError(err, "connection should still be open after zero-length rejection")
		respMsg2 := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg2.Unpack(respBody2))
		mti2, _ := respMsg2.GetMTI()
		s.Assert().Equal("0810", mti2)

		cancel()
		client.Close()
		<-handleDone
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

func (s *testSuiteConn) TestReadLoop_IdleTimeoutEnforced() {
	s.Run("when client sends nothing then connection is closed after idle timeout", func() {
		opts := testConnOpts()
		opts.ReadTimeout = 50 * time.Millisecond
		opts.IdleTimeout = 100 * time.Millisecond
		c, client := testConn(opts)

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(context.Background())
		}()

		// Do not send anything. Wait just past the idle timeout.
		select {
		case <-handleDone:
			// handle timed out and exited — correct
		case <-time.After(300 * time.Millisecond):
			s.Fail("handle did not exit after idle timeout")
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

func (s *testSuiteConn) TestIsGracefulClose_NetErrClosed() {
	s.Run("when error contains net.ErrClosed then isGracefulClose returns true", func() {
		// Simulate what net returns when we close a connection ourselves.
		// Dial a real port, close it, then read — gives an OpError wrapping ErrClosed.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		s.Require().NoError(err)
		defer ln.Close()

		conn, err := net.Dial("tcp", ln.Addr().String())
		s.Require().NoError(err)
		conn.Close() // close our side first

		// Reading from a closed conn returns an OpError containing net.ErrClosed.
		_, readErr := conn.Read(make([]byte, 1))
		s.Require().Error(readErr)
		s.Assert().True(isGracefulClose(readErr), "OpError wrapping ErrClosed must be graceful")
	})
}

// ── processFrame ──────────────────────────────────────────────────────────────

func (s *testSuiteConn) TestProcessFrame_WriteDeadlineError() {
	s.Run("when write deadline is too short then processFrame returns a write error", func() {
		opts := testConnOpts()
		opts.WriteTimeout = 1 * time.Millisecond // too short to complete write

		c, client := testConn(opts)

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(context.Background())
		}()

		// Build and send a valid 0800 so processFrame gets to the write path.
		req := iso.EchoRequest{STAN: "111111", NetworkMgmtInfoCode: "301"}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(msg.Marshal(&req))
		msg.MTI("0800")
		packed, err := msg.Pack()
		s.Require().NoError(err)

		header := iso.NewNetworkHeader()
		s.Require().NoError(header.SetLength(len(packed)))
		_, err = header.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(packed)
		s.Require().NoError(err)

		// Do NOT read the response — this causes the server's write to block
		// and eventually fail when the 1ms write deadline fires.
		select {
		case <-handleDone:
			// handle exited due to write error — correct
		case <-time.After(2 * time.Second):
			s.Fail("handle did not exit after write deadline")
		}
		client.Close()
	})
}

func (s *testSuiteConn) TestProcessFrame_UnpackError() {
	s.Run("when client sends garbage data then connection is closed with unpack error", func() {
		c, client := testConn(testConnOpts())

		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(context.Background())
		}()

		// Send 5 bytes of garbage
		garbage := []byte("hello")
		header := iso.NewNetworkHeader()
		s.Require().NoError(header.SetLength(len(garbage)))
		_, err := header.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(garbage)
		s.Require().NoError(err)

		select {
		case <-handleDone:
			// handle exited due to unpack error — correct
		case <-time.After(2 * time.Second):
			s.Fail("handle did not exit after unpack error")
		}
		client.Close()
	})
}

func (s *testSuiteConn) TestWrite_ExcessLength() {
	s.Run("when payload is too large then write returns error", func() {
		c, client := testConn(testConnOpts())
		defer client.Close()
		hugePayload := make([]byte, 70000)
		err := c.write(hugePayload)
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "set frame length:")
	})
}

func (s *testSuiteConn) TestProcessFrame_DeadlineError() {
	s.Run("when connection is closed then processFrame returns deadline error", func() {
		c, client := testConn(testConnOpts())
		c.conn.Close()
		client.Close()
		_, err := c.processFrame()
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "set idle read deadline")
	})
}

func (s *testSuiteConn) TestReadLoop_UnknownMTI_ContinuesLoop() {
	s.Run("when client sends unknown MTI then loop continues and does not exit", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Send an 0200 message (unknown MTI to our handler).
		unknownMsg := iso8583.NewMessage(iso.DiscoverSpec)
		unknownMsg.MTI("0200")
		packed, err := unknownMsg.Pack()
		s.Require().NoError(err)

		header := iso.NewNetworkHeader()
		s.Require().NoError(header.SetLength(len(packed)))
		_, err = header.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(packed)
		s.Require().NoError(err)

		// Server should NOT close — give it a moment then cancel context.
		time.Sleep(50 * time.Millisecond)
		select {
		case <-handleDone:
			s.Fail("handle should not have exited on unknown MTI")
		default:
			// correct — still running
		}

		cancel()
		client.Close()
		<-handleDone
	})
}

// ── MOD-70: Frame validation ───────────────────────────────────────────────────

func (s *testSuiteConn) TestReadLoop_OversizedFrame_RespondsWithF39_30() {
	s.Run("when client sends frame >4096 bytes then server responds with 0810 F39=30 and stays open", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Send a length-prefix indicating 5000 bytes (>MaxFrameSize=4096).
		// We only send the 2-byte header; the server validates length before reading body.
		oversizeHeader := make([]byte, 2)
		oversizeHeader[0] = 0x13 // 0x1388 = 5000
		oversizeHeader[1] = 0x88
		_, err := client.Write(oversizeHeader)
		s.Require().NoError(err)

		// Server must respond with 0810 F39=30 and keep connection open.
		respBody, err := readFrame(client)
		s.Require().NoError(err, "expected 0810 F39=30 response for oversized frame")

		respMsg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg.Unpack(respBody))
		mti, err := respMsg.GetMTI()
		s.Require().NoError(err)
		s.Assert().Equal("0810", mti)
		code, err := respMsg.GetString(39)
		s.Require().NoError(err)
		s.Assert().Equal("30", code, "F39 must be 30 for oversized frame")

		// Verify connection is still open: send a valid 0800 and get 0810 back.
		req := iso.EchoRequest{STAN: "111222", NetworkMgmtInfoCode: "301"}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(msg.Marshal(&req))
		msg.MTI("0800")
		validPacked, err := msg.Pack()
		s.Require().NoError(err)
		hdr := iso.NewNetworkHeader()
		s.Require().NoError(hdr.SetLength(len(validPacked)))
		_, err = hdr.WriteTo(client)
		s.Require().NoError(err)
		_, err = client.Write(validPacked)
		s.Require().NoError(err)

		respBody2, err := readFrame(client)
		s.Require().NoError(err, "connection should still be open after oversized rejection")
		respMsg2 := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg2.Unpack(respBody2))
		mti2, _ := respMsg2.GetMTI()
		s.Assert().Equal("0810", mti2)

		cancel()
		client.Close()
		<-handleDone
	})
}

func (s *testSuiteConn) TestReadLoop_FragmentedDelivery_AssemblesCorrectly() {
	s.Run("when message arrives in multiple TCP segments then bufio.Reader assembles it correctly", func() {
		c, client := testConn(testConnOpts())

		ctx, cancel := context.WithCancel(context.Background())
		handleDone := make(chan struct{})
		go func() {
			defer close(handleDone)
			c.handle(ctx)
		}()

		// Build a valid packed 0800 message.
		req := iso.EchoRequest{STAN: "789012", NetworkMgmtInfoCode: "301"}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(msg.Marshal(&req))
		msg.MTI("0800")
		packed, err := msg.Pack()
		s.Require().NoError(err)

		// Prepare the 2-byte header separately.
		hdr := iso.NewNetworkHeader()
		s.Require().NoError(hdr.SetLength(len(packed)))

		// Write header and body as separate Write() calls to simulate fragmentation.
		// net.Pipe is synchronous, so use a goroutine to avoid deadlock.
		writeDone := make(chan error, 1)
		go func() {
			if _, werr := hdr.WriteTo(client); werr != nil {
				writeDone <- werr
				return
			}
			// Small delay to ensure header is dispatched as its own segment.
			time.Sleep(10 * time.Millisecond)
			_, werr := client.Write(packed)
			writeDone <- werr
		}()

		// Read the assembled 0810 response.
		respBody, err := readFrame(client)
		s.Require().NoError(err, "fragmented message should be assembled and echoed")
		s.Require().NoError(<-writeDone, "fragmented write should succeed")

		respMsg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg.Unpack(respBody))
		mti, err := respMsg.GetMTI()
		s.Require().NoError(err)
		s.Assert().Equal("0810", mti)

		var resp iso.EchoResponse
		s.Require().NoError(respMsg.Unmarshal(&resp))
		s.Assert().Equal("789012", resp.STAN)
		s.Assert().Equal("00", resp.ResponseCode)

		cancel()
		client.Close()
		<-handleDone
	})
}

func (s *testSuiteConn) TestSendErrorResponse_WritesValidF39_30() {
	s.Run("when sendErrorResponse is called then a valid 0810 F39=30 is written on the connection", func() {
		c, client := testConn(testConnOpts())
		defer client.Close()

		done := make(chan error, 1)
		go func() {
			done <- c.sendErrorResponse("30")
		}()

		respBody, err := readFrame(client)
		s.Require().NoError(err)
		s.Require().NoError(<-done)

		respMsg := iso8583.NewMessage(iso.DiscoverSpec)
		s.Require().NoError(respMsg.Unpack(respBody))
		mti, err := respMsg.GetMTI()
		s.Require().NoError(err)
		s.Assert().Equal("0810", mti)
		code, err := respMsg.GetString(39)
		s.Require().NoError(err)
		s.Assert().Equal("30", code)
	})
}

