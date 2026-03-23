package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog"
)

// Conn handles the lifecycle of a single accepted TCP connection.
// It owns the read loop, write path, deadline management, and clean teardown.
//
// Design rules (per coding standards §4.3, §4.6):
//   - context.Context is never stored as a field — always passed as argument.
//   - Every read and write sets an explicit deadline from ServerOptions.
//   - Errors are logged once at this boundary — never re-logged up the stack.
//   - Goroutine exit: readLoop returns on io.EOF, net error, or ctx cancel.
type Conn struct {
	conn   net.Conn
	opts   *ServerOptions
	logger zerolog.Logger
}

// newConn constructs a Conn for a single accepted TCP connection.
// The logger is enriched with remote_addr so every log line is traceable.
func newConn(c net.Conn, opts *ServerOptions, logger zerolog.Logger) *Conn {
	return &Conn{
		conn:   c,
		opts:   opts,
		logger: logger.With().Str("remote_addr", c.RemoteAddr().String()).Logger(),
	}
}

// handle is the entry point for a connection goroutine.
// It runs the read loop and guarantees conn.Close() on exit.
//
// Goroutine exit condition: handle returns when readLoop exits — on
// io.EOF (client disconnect), a net error, or ctx cancellation.
func (c *Conn) handle(ctx context.Context) {
	defer c.conn.Close() //nolint:errcheck // close error not actionable here

	c.logger.Info().Msg("connection accepted")

	if err := c.readLoop(ctx); err != nil {
		if isGracefulClose(err) {
			c.logger.Info().Msg("connection closed by client")
		} else {
			// Log once at boundary — do not re-log up the stack (§4.4).
			c.logger.Warn().Err(err).Msg("connection error")
		}
		return
	}

	c.logger.Info().Msg("connection closed")
}

// readLoop reads ISO 8583 frames until the connection closes or ctx is cancelled.
//
// Frame format: 2-byte big-endian length prefix + message body.
// ISO 8583 message handling via moov-io is wired in TASK-5.
//
// Exit condition: returns nil on ctx cancel, non-nil error on read/write failure.
func (c *Conn) readLoop(ctx context.Context) error {
	for {
		// Check context cancellation before blocking on a read.
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// HARD RULE (§4.6): set explicit read deadline before every blocking read.
		if err := c.conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		// Read the 2-byte big-endian length prefix.
		lenBuf := make([]byte, 2)
		if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
			return fmt.Errorf("read length prefix: %w", err)
		}

		msgLen := int(lenBuf[0])<<8 | int(lenBuf[1])
		if msgLen == 0 {
			return fmt.Errorf("read length prefix: zero-length frame received")
		}

		// Set a fresh read deadline for the body read.
		if err := c.conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout)); err != nil {
			return fmt.Errorf("set read deadline for body: %w", err)
		}

		// Read the full message body.
		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(c.conn, msgBuf); err != nil {
			return fmt.Errorf("read message body (%d bytes): %w", msgLen, err)
		}

		c.logger.Debug().Int("msg_len", msgLen).Msg("frame received")

		// TODO TASK-5: replace with iso.HandleMessage(msgBuf) → responseBytes.
		// Placeholder: echo the raw bytes back.
		if err := c.write(msgBuf); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

// write sends data to the client prefixed with a 2-byte big-endian length.
// Sets an explicit write deadline before every write — mandatory per §4.6.
func (c *Conn) write(data []byte) error {
	// HARD RULE (§4.6): explicit deadline before every blocking write.
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	msgLen := len(data)
	frame := make([]byte, 2+msgLen)
	frame[0] = byte(msgLen >> 8)
	frame[1] = byte(msgLen)
	copy(frame[2:], data)

	if _, err := c.conn.Write(frame); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}

	c.logger.Debug().Int("msg_len", msgLen).Msg("frame sent")
	return nil
}

// isGracefulClose reports whether err is a normal client disconnect.
// io.EOF = client closed cleanly. net.ErrClosed = our side closed it (Stop()).
func isGracefulClose(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return errors.Is(netErr.Err, net.ErrClosed)
	}
	return false
}
