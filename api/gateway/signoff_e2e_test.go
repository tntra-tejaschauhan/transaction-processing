//go:build e2e

package gateway


import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// gatewayAddrForSignOff returns the gateway address from the environment or
// falls back to the default localhost address.
func gatewayAddrForSignOff(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("GATEWAY_ADDR")
	if addr == "" {
		addr = "localhost:8583"
	}
	return addr
}

// ────────────────────────────────────────────────────────────────────────────
// Formal Discover Sign-off Suite
// These 4 tests map directly to the Discover acceptance criteria for MOD-75.
// ────────────────────────────────────────────────────────────────────────────

// TestSignOff_EchoSuccess verifies that a standard 0800 echo (F70=301) receives
// a valid 0810 response with ResponseCode 00 and the same F70 echoed back.
func TestSignOff_EchoSuccess(t *testing.T) {
	addr := gatewayAddrForSignOff(t)
	client, err := New(addr)
	require.NoError(t, err, "failed to connect to gateway")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.SendEcho(ctx, "301301", "301")
	require.NoError(t, err, "echo SendEcho failed")
	require.NotNil(t, resp)
	require.Equal(t, "00", resp.ResponseCode, "F39 must be 00 for echo")
	require.Equal(t, "301", resp.NetworkMgmtInfoCode, "F70 must be echoed back")
	require.Equal(t, "301301", resp.STAN, "F11 STAN must be echoed back")
}

// TestSignOff_SignOnSuccess verifies that a sign-on 0800 (F70=001) receives
// a valid 0810 response with ResponseCode 00 and F70=001 echoed back.
func TestSignOff_SignOnSuccess(t *testing.T) {
	addr := gatewayAddrForSignOff(t)
	client, err := New(addr)
	require.NoError(t, err, "failed to connect to gateway")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.SendEcho(ctx, "001001", "001")
	require.NoError(t, err, "sign-on SendEcho failed")
	require.NotNil(t, resp)
	require.Equal(t, "00", resp.ResponseCode, "F39 must be 00 for sign-on")
	require.Equal(t, "001", resp.NetworkMgmtInfoCode, "F70 must be 001 echoed back")
	require.Equal(t, "001001", resp.STAN, "F11 STAN must be echoed back")
}

// TestSignOff_ResponseFramed verifies that the gateway correctly frames the
// 0810 response (i.e. the message can be received without a read error),
// confirming correct length-prefixed framing over the TCP connection.
func TestSignOff_ResponseFramed(t *testing.T) {
	addr := gatewayAddrForSignOff(t)
	client, err := New(addr)
	require.NoError(t, err, "failed to connect to gateway")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// If framing is broken, SendEcho will return a read/unmarshal error.
	resp, err := client.SendEcho(ctx, "123456", "301")
	require.NoError(t, err, "response must be correctly framed and readable")
	require.NotNil(t, resp, "a framed response must not be nil")
}

// TestSignOff_ResponseWithin100ms verifies that both echo and sign-on responses
// are returned within the Discover-mandated 100 ms SLA, measured over 10
// back-to-back requests on the same connection.
func TestSignOff_ResponseWithin100ms(t *testing.T) {
	addr := gatewayAddrForSignOff(t)

	tests := []struct {
		name string
		stan string
		f70  string
	}{
		{"echo (F70=301)", "301000", "301"},
		{"sign-on (F70=001)", "001000", "001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(addr)
			require.NoError(t, err, "failed to connect to gateway")
			defer client.Close()

			const iterations = 10
			for i := 0; i < iterations; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				start := time.Now()
				_, err := client.SendEcho(ctx, tc.stan, tc.f70)
				elapsed := time.Since(start)
				cancel()

				require.NoError(t, err, "%s iteration %d failed", tc.name, i+1)
				require.Less(t, elapsed, 100*time.Millisecond,
					"%s exceeded 100ms SLA on iteration %d (got %s)", tc.name, i+1, elapsed)
			}
		})
	}
}
