package gateway

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/server"
)

// testGateway wraps a running Server with its address for easy use in tests.
type testGateway struct {
	*server.Server
	Addr string
}

// startRealGateway boots an in-process gateway server on a random free port.
// The returned *testGateway.Addr is the dial address (host:port).
// Caller must defer g.Stop().
func startRealGateway(t *testing.T) *testGateway {
	t.Helper()
	// Let OS assign a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	opts := &server.ServerOptions{
		Port:            port,
		ReadTimeout:     2 * time.Second,
		WriteTimeout:    2 * time.Second,
		MaxConnections:  10,
		ShutdownTimeout: 2 * time.Second,
		BufSize:         8192,
		IdleTimeout:     10 * time.Second,
	}

	srv, err := server.New(opts, zerolog.Nop())
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	return &testGateway{Server: srv, Addr: fmt.Sprintf("127.0.0.1:%d", port)}
}



func TestClient_DialError(t *testing.T) {
	// Use a non-existent port to force a dial error
	_, err := New("127.0.0.1:1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to dial")
}

func TestClient_ContextCanceled(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	// Wait for a connection and then sleep (simulating slow server)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			time.Sleep(1 * time.Second)
			conn.Close()
		}
	}()

	client, err := New(l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	// Use an already canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = client.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_GarbageResponse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err == nil {
			// Read the request frame Length
			lenBuf := make([]byte, 2)
			io.ReadFull(conn, lenBuf)
			msgLen := binary.BigEndian.Uint16(lenBuf)

			// Read Body
			body := make([]byte, msgLen)
			io.ReadFull(conn, body)

			// Send back an invalid/garbage ISO frame
			conn.Write([]byte{0x00, 0x04, 'j', 'u', 'n', 'k'})
			conn.Close()
		}
	}()

	client, err := New(l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unpack response")
}

func TestClient_ShortRead_EOF(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err == nil {
			// Read the request to ensure the client write succeeds
			lenBuf := make([]byte, 2)
			io.ReadFull(conn, lenBuf)
			msgLen := binary.BigEndian.Uint16(lenBuf)
			body := make([]byte, msgLen)
			io.ReadFull(conn, body)

			// Immediately close connection to cause EOF when client tries to read
			conn.Close()
		}
	}()

	client, err := New(l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}

func TestClient_UnexpectedMTI(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err == nil {
			lenBuf := make([]byte, 2)
			io.ReadFull(conn, lenBuf)
			msgLen := binary.BigEndian.Uint16(lenBuf)
			body := make([]byte, msgLen)
			io.ReadFull(conn, body)

			if string(body[0:4]) == "0800" {
				copy(body[0:4], []byte("0200"))
			}

			outLen := make([]byte, 2)
			binary.BigEndian.PutUint16(outLen, uint16(len(body)))
			conn.Write(outLen)
			conn.Write(body)
			conn.Close()
		}
	}()

	client, err := New(l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected MTI in response")
}

func TestClient_Close_NilConn(t *testing.T) {
	c := &Client{} // conn is nil
	err := c.Close()
	require.NoError(t, err)
}

func TestClient_ReadFrame_BodyError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		if err == nil {
			lenBuf := make([]byte, 2)
			io.ReadFull(conn, lenBuf)
			msgLen := binary.BigEndian.Uint16(lenBuf)
			body := make([]byte, msgLen)
			io.ReadFull(conn, body)

			// Send a valid length (10 bytes) but close connection after sending 2 bytes
			outLen := []byte{0x00, 0x0A}
			conn.Write(outLen)
			conn.Write([]byte{0x01, 0x02})
			conn.Close()
		}
	}()

	client, err := New(l.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}

// mockConn implements net.Conn for forcing errors
type mockConn struct {
	net.Conn
	writeErr        error
	readDeadlineErr error
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return len(b), nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	if m.readDeadlineErr != nil {
		return m.readDeadlineErr
	}
	return nil
}

func TestClient_WriteFrameError(t *testing.T) {
	importErr := io.ErrClosedPipe
	c := &Client{
		conn: &mockConn{writeErr: importErr},
	}
	_, err := c.SendEcho(context.Background(), "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "send frame")
}

func TestClient_SetReadDeadlineError(t *testing.T) {
	importErr := io.ErrClosedPipe
	c := &Client{
		conn: &mockConn{readDeadlineErr: importErr},
	}

	// Create context with deadline so SetReadDeadline is called
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := c.SendEcho(ctx, "123456", "301")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}

// ── SendAuth ──────────────────────────────────────────────────────────────────

// echoServerFunc runs a simple TCP server that calls handler for each connection.
func echoServerFunc(t *testing.T, handler func(net.Conn)) (addr string, stop func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return l.Addr().String(), func() { l.Close() }
}

// readRequestAndRespond reads a framed request and sends back a framed response.
func readRequestAndRespond(conn net.Conn, resp []byte) {
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return
	}
	msgLen := binary.BigEndian.Uint16(lenBuf)
	body := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	outLen := make([]byte, 2)
	binary.BigEndian.PutUint16(outLen, uint16(len(resp)))
	conn.Write(outLen)
	conn.Write(resp)
	conn.Close()
}

func TestSendAuth_GarbageResponse(t *testing.T) {
	garbage := []byte("junk")
	addr, stop := echoServerFunc(t, func(conn net.Conn) {
		readRequestAndRespond(conn, garbage)
	})
	defer stop()

	c, err := New(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &iso.AuthRequest{STAN: "111111"}
	_, err = c.SendAuth(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SendAuth: unpack response")
}

func TestSendAuth_EOFResponse(t *testing.T) {
	addr, stop := echoServerFunc(t, func(conn net.Conn) {
		// Read request then close without sending response
		lenBuf := make([]byte, 2)
		io.ReadFull(conn, lenBuf)
		msgLen := binary.BigEndian.Uint16(lenBuf)
		body := make([]byte, msgLen)
		io.ReadFull(conn, body)
		conn.Close()
	})
	defer stop()

	c, err := New(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &iso.AuthRequest{STAN: "222222"}
	_, err = c.SendAuth(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SendAuth: read response")
}

func TestSendAuth_WriteError(t *testing.T) {
	c := &Client{conn: &mockConn{writeErr: io.ErrClosedPipe}}
	req := &iso.AuthRequest{STAN: "333333"}
	_, err := c.SendAuth(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SendAuth: send frame")
}

func TestSendByMTI_GarbageResponse(t *testing.T) {
	garbage := []byte("bad")
	addr, stop := echoServerFunc(t, func(conn net.Conn) {
		readRequestAndRespond(conn, garbage)
	})
	defer stop()

	c, err := New(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.SendByMTI(ctx, "0100", "0110", "444444")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unpack response")
}

func TestSendByMTI_WriteError(t *testing.T) {
	c := &Client{conn: &mockConn{writeErr: io.ErrClosedPipe}}
	_, err := c.SendByMTI(context.Background(), "0100", "0110", "555555")
	require.Error(t, err)
	require.Contains(t, err.Error(), "send frame")
}

// TestSendAuth_Success sends a real 0100 auth request to a real gateway server.
func TestSendAuth_Success(t *testing.T) {
	srv := startRealGateway(t)
	defer srv.Stop()

	c, err := New(srv.Addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &iso.AuthRequest{
		PAN:    "4111111111111111",
		STAN:   "123456",
		RRN:    "123456789012",
		Amount: "000000010000",
	}

	resp, err := c.SendAuth(ctx, req)
	require.NoError(t, err, "SendAuth must succeed against a running gateway")
	require.NotNil(t, resp)
	require.Equal(t, "00", resp.ResponseCode, "auth response code must be approved (00)")
	require.Equal(t, "123456", resp.STAN, "STAN must be echoed back")
}

// TestSendAuth_UnexpectedMTI creates a mock server that replies with the wrong MTI.
func TestSendAuth_UnexpectedMTI(t *testing.T) {
	// Server replies with a 0200 instead of 0110.
	addr, stop := echoServerFunc(t, func(conn net.Conn) {
		lenBuf := make([]byte, 2)
		io.ReadFull(conn, lenBuf)
		msgLen := binary.BigEndian.Uint16(lenBuf)
		body := make([]byte, msgLen)
		io.ReadFull(conn, body)
		// Mangle MTI to 0200
		copy(body[0:4], []byte("0200"))
		outLen := make([]byte, 2)
		binary.BigEndian.PutUint16(outLen, uint16(len(body)))
		conn.Write(outLen)
		conn.Write(body)
		conn.Close()
	})
	defer stop()

	c, err := New(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &iso.AuthRequest{STAN: "777777"}
	_, err = c.SendAuth(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected MTI")
}

// TestSendByMTI_Success sends an 0800 echo and confirms 0810 response.
func TestSendByMTI_Success(t *testing.T) {
	srv := startRealGateway(t)
	defer srv.Stop()

	c, err := New(srv.Addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.SendByMTI(ctx, "0800", "0810", "888888")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "00", resp.ResponseCode)
}

// TestSendByMTI_EOFResponse verifies error handling when server closes without response.
func TestSendByMTI_EOFResponse(t *testing.T) {
	addr, stop := echoServerFunc(t, func(conn net.Conn) {
		lenBuf := make([]byte, 2)
		io.ReadFull(conn, lenBuf)
		msgLen := binary.BigEndian.Uint16(lenBuf)
		body := make([]byte, msgLen)
		io.ReadFull(conn, body)
		conn.Close() // close without responding
	})
	defer stop()

	c, err := New(addr)
	require.NoError(t, err)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.SendByMTI(ctx, "0800", "0810", "999999")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}
