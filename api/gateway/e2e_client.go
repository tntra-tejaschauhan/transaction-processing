package gateway

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
	"github.com/moov-io/iso8583"
)

// Client is an ISO TCP test client that connects to the gateway and sends
// echo (0800/0810) messages. It maintains a single persistent connection and
// handles frame marshaling/unmarshaling internally.
type Client struct {
	conn net.Conn
}

// New dials the gateway at addr and returns a Client ready to send messages.
// If the connection fails or times out, an error is returned.
//
// The addr should be in the form "host:port", e.g. "localhost:8583".
// A 5-second dial timeout is applied to prevent hanging connections.
func New(addr string) (*Client, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	return &Client{conn: conn}, nil
}

// SendEcho sends an ISO 0800 echo request with the specified STAN and
// NetworkMgmtInfoCode, and reads the 0810 echo response.
//
// The method marshals an EchoRequest struct into ISO 8583 format, frames it
// with a 2-byte big-endian length prefix, sends it over TCP, and reads the
// response frame. The response is unmarshaled into an EchoResponse struct.
//
// Context cancellation will interrupt the send/receive operation.
func (c *Client) SendEcho(ctx context.Context, stan, networkMgmtCode string) (*iso.EchoResponse, error) {
	// Build and marshal the request.
	req := &iso.EchoRequest{
		STAN:                stan,
		NetworkMgmtInfoCode: networkMgmtCode,
	}

	// Create an ISO 8583 message and marshal the request into it.
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	if err := msg.Marshal(req); err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	msg.MTI("0800")

	// Pack the message into bytes.
	body, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("pack message: %w", err)
	}

	// Send the framed request to the gateway.
	if err := c.writeFrame(body); err != nil {
		return nil, fmt.Errorf("send frame: %w", err)
	}

	// Read the response frame with context timeout.
	respBody, err := c.readFrameWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Unpack the response message.
	respMsg := iso8583.NewMessage(iso.DiscoverSpec)
	if err := respMsg.Unpack(respBody); err != nil {
		return nil, fmt.Errorf("unpack response: %w", err)
	}

	// Verify the MTI is 0810.
	mti, err := respMsg.GetMTI()
	if err != nil {
		return nil, fmt.Errorf("get response MTI: %w", err)
	}
	if mti != "0810" {
		return nil, fmt.Errorf("unexpected MTI in response: got %s, want 0810", mti)
	}

	// Extract the response fields into an EchoResponse struct.
	var resp iso.EchoResponse
	if err := respMsg.Unmarshal(&resp); err != nil {
		return nil, fmt.Errorf("extract response fields: %w", err)
	}

	return &resp, nil
}

// Close gracefully closes the underlying TCP connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// writeFrame writes data to conn as: 2-byte big-endian length + data.
// This matches the framing protocol used by the gateway.
func (c *Client) writeFrame(body []byte) error {
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(body)))

	if _, err := c.conn.Write(lenBuf); err != nil {
		return err
	}
	_, err := c.conn.Write(body)
	return err
}

// readFrame reads a framed message: 2-byte big-endian length + body.
// This is a lower-level helper; most callers should use readFrameWithContext.
func (c *Client) readFrame() ([]byte, error) {
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint16(lenBuf)

	body := make([]byte, msgLen)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

// readFrameWithContext reads a framed message with a context deadline.
// If context is cancelled before the read completes, returns context error.
func (c *Client) readFrameWithContext(ctx context.Context) ([]byte, error) {
	// Set a read deadline based on the context.
	deadline, ok := ctx.Deadline()
	if ok {
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		// Reset deadline after we finish.
		defer c.conn.SetReadDeadline(time.Time{})
	}

	// Check if context is already cancelled before we start.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return c.readFrame()
}
