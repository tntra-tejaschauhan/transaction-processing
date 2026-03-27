package gateway

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

