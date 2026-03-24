package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
	"github.com/moov-io/iso8583"
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
// Frame format: 2-byte big-endian length prefix + ISO 8583 message body.
// Each frame is unpacked via moov-io/iso8583, dispatched to iso.HandleMessage,
// and the response is packed and written back with the same framing.
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

		// HARD RULE (§4.6): explicit read deadline before every blocking read.
		if err := c.conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		// Read the 2-byte big-endian length prefix using moov-io framer.
		header := iso.NewNetworkHeader()
		if _, err := header.ReadFrom(c.conn); err != nil {
			return fmt.Errorf("read length prefix: %w", err)
		}

		msgLen := header.Length()
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

		c.logger.Debug().Int("msg_len", int(msgLen)).Msg("frame received")

		// Unpack raw bytes into an ISO 8583 message.
		inMsg := iso8583.NewMessage(iso.DiscoverSpec)
		if err := inMsg.Unpack(msgBuf); err != nil {
			return fmt.Errorf("unpack iso8583 message: %w", err)
		}

		// Dispatch to handler — returns the response message.
		outMsg, err := iso.HandleMessage(inMsg)
		if err != nil {
			// Log and continue — unknown MTI should not kill the connection.
			c.logger.Warn().Err(err).Msg("handle message error")
			continue
		}

		// Pack the response message back to raw bytes.
		responseBytes, err := outMsg.Pack()
		if err != nil {
			return fmt.Errorf("pack iso8583 response: %w", err)
		}

		// Write the response with 2-byte length prefix.
		if err := c.write(responseBytes); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

// write sends data to the client using the moov-io 2-byte network header.
// Sets an explicit write deadline before every write — mandatory per §4.6.
func (c *Conn) write(data []byte) error {
	// HARD RULE (§4.6): explicit deadline before every blocking write.
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	// Write 2-byte length prefix using moov-io framer.
	header := iso.NewNetworkHeader()
	if err := header.SetLength(len(data)); err != nil {
		return fmt.Errorf("set frame length: %w", err)
	}
	if _, err := header.WriteTo(c.conn); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}

	// Write the message body.
	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("write frame body: %w", err)
	}

	c.logger.Debug().Int("msg_len", len(data)).Msg("frame sent")
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
