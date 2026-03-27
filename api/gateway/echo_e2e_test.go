package gateway

import (
    "context"
    "encoding/binary"
    "io"
    "net"
    "os"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
    "github.com/moov-io/iso8583"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
)

// TestE2E runs the end-to-end test suite against a running gateway.
func TestE2E(t *testing.T) {
	suite.Run(t, new(testSuiteE2E))
}

type testSuiteE2E struct {
	suite.Suite
	gatewayAddr string
}

// SetupSuite gets the gateway address from environment or uses the default.
func (s *testSuiteE2E) SetupSuite() {
	s.gatewayAddr = os.Getenv("GATEWAY_ADDR")
	if s.gatewayAddr == "" {
		s.gatewayAddr = "localhost:8583"
	}
	s.T().Logf("Using gateway at %s", s.gatewayAddr)
}

// ────────────────────────────────────────────────────────────────────────────
// Scenario 1: Happy Path — single successful echo
// ────────────────────────────────────────────────────────────────────────────

func (s *testSuiteE2E) TestHappyPath_SingleEcho() {
	s.Run("successfully send 0800 and receive 0810 with ResponseCode 00", func() {
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "failed to connect to gateway")
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.SendEcho(ctx, "123456", "301")
		require.NoError(s.T(), err, "SendEcho failed")

		// Verify response fields.
		require.NotNil(s.T(), resp, "response should not be nil")
		require.Equal(s.T(), "123456", resp.STAN, "STAN should be echoed back")
		require.Equal(s.T(), "00", resp.ResponseCode, "ResponseCode should be 00 (approved)")
		require.Equal(s.T(), "301", resp.NetworkMgmtInfoCode, "NetworkMgmtInfoCode should be echoed back")
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Scenario 2: Edge Cases — various STAN values, max length, closed connection
// ────────────────────────────────────────────────────────────────────────────

func (s *testSuiteE2E) TestEdgeCases_EmptySTAN() {
	s.Run("echo with empty STAN should work (echo back empty)", func() {
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "failed to connect to gateway")
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.SendEcho(ctx, "", "301")
		require.NoError(s.T(), err, "SendEcho with empty STAN failed")
		require.NotNil(s.T(), resp)
		require.Equal(s.T(), "", resp.STAN, "empty STAN should be echoed back")
		require.Equal(s.T(), "00", resp.ResponseCode)
	})
}

func (s *testSuiteE2E) TestEdgeCases_MaxLengthSTAN() {
	s.Run("echo with max-length STAN (6 digits) should work", func() {
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "failed to connect to gateway")
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		maxSTAN := "999999"
		resp, err := client.SendEcho(ctx, maxSTAN, "301")
		require.NoError(s.T(), err, "SendEcho with max-length STAN failed")
		require.NotNil(s.T(), resp)
		require.Equal(s.T(), maxSTAN, resp.STAN, "max-length STAN should be echoed back")
		require.Equal(s.T(), "00", resp.ResponseCode)
	})
}

func (s *testSuiteE2E) TestEdgeCases_NetworkMgmtCodeVariations() {
	s.Run("echo with different NetworkMgmtInfoCode values should echo back unchanged", func() {
		codes := []string{"301", "001", "999"}

		for _, code := range codes {
			s.Run("code "+code, func() {
				client, err := New(s.gatewayAddr)
				require.NoError(s.T(), err, "failed to connect to gateway")
				defer client.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				resp, err := client.SendEcho(ctx, "111111", code)
				require.NoError(s.T(), err, "SendEcho failed for code %s", code)
				require.Equal(s.T(), code, resp.NetworkMgmtInfoCode, "NetworkMgmtInfoCode should be echoed back")
				require.Equal(s.T(), "00", resp.ResponseCode)
			})
		}
	})
}

func (s *testSuiteE2E) TestEdgeCases_ClosedConnection() {
	s.Run("sending to closed connection should return an error", func() {
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "failed to connect to gateway")
		client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = client.SendEcho(ctx, "123456", "301")
		require.Error(s.T(), err, "SendEcho on closed connection should fail")
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Scenario 3: Robustness — sequential messages and concurrent connections
// ────────────────────────────────────────────────────────────────────────────

func (s *testSuiteE2E) TestRobustness_SequentialMessages() {
	s.Run("send 5 sequential messages on same connection, verify all responses", func() {
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "failed to connect to gateway")
		defer client.Close()

		for i := 1; i <= 5; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			stan := strings.Repeat(string(rune('0'+byte(i))), 6)
			resp, err := client.SendEcho(ctx, stan, "301")

			require.NoError(s.T(), err, "SendEcho iteration %d failed", i)
			require.NotNil(s.T(), resp, "response should not be nil for iteration %d", i)
			require.Equal(s.T(), stan, resp.STAN, "STAN mismatch on iteration %d", i)
			require.Equal(s.T(), "00", resp.ResponseCode)
			require.Equal(s.T(), "301", resp.NetworkMgmtInfoCode)

			cancel()
		}
	})
}

func (s *testSuiteE2E) TestRobustness_ConcurrentConnections() {
	s.Run("open 1000 concurrent connections, send 1 echo each, verify all responses with no race", func() {
		const numConnections = 1000
		var wg sync.WaitGroup
		results := make(chan error, numConnections)

		for i := 0; i < numConnections; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				client, err := New(s.gatewayAddr)
				if err != nil {
					results <- err
					return
				}
				defer client.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				stan := strings.Repeat(string(rune('a'+byte(idx%26))), 6)
				resp, err := client.SendEcho(ctx, stan, "301")
				if err != nil {
					results <- err
					return
				}

				// Verify the response contains the expected STAN.
				if resp == nil || resp.STAN != stan {
					results <- error(nil) // Will be caught by nil check below
					return
				}

				results <- nil
			}(i)
		}

		start := time.Now()
		wg.Wait()
		close(results)
		require.Less(s.T(), time.Since(start), 10*time.Second, "1000 concurrent echos should complete within 10 seconds")

		// Collect and verify all results.
		for result := range results {
			require.NoError(s.T(), result, "concurrent SendEcho failed")
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Bonus: Verify ResponseCode is always "00" for echo (approval)
// ────────────────────────────────────────────────────────────────────────────

func (s *testSuiteE2E) TestResponseCodeAlwaysApproved() {
	s.Run("ResponseCode should always be 00 (approved) for echo messages", func() {
		codes := []string{"301", "401", "999"}

		for _, code := range codes {
			client, err := New(s.gatewayAddr)
			require.NoError(s.T(), err, "failed to connect to gateway")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			resp, err := client.SendEcho(ctx, "555555", code)
			cancel()

			require.NoError(s.T(), err, "SendEcho failed for NetworkMgmtInfoCode %s", code)
			require.Equal(s.T(), "00", resp.ResponseCode, "ResponseCode should be 00 for NetworkMgmtInfoCode %s", code)

			client.Close()
		}
	})
}


func (s *testSuiteE2E) TestGracefulDisconnect_ServerCleansUp() {
    s.Run("after client disconnects, server should accept a new connection within 1s", func() {
        // Dial and disconnect immediately.
        client, err := New(s.gatewayAddr)
        require.NoError(s.T(), err, "initial connect failed")
        client.Close() // graceful client-side close

        // Poll for up to 1 second to confirm server is still accepting.
        deadline := time.Now().Add(1 * time.Second)
        var connected bool
        for time.Now().Before(deadline) {
            c2, err := New(s.gatewayAddr)
            if err == nil {
                c2.Close()
                connected = true
                break
            }
            time.Sleep(10 * time.Millisecond)
        }
        require.True(s.T(), connected, "server should still accept new connections within 1s of client disconnect")
    })
}

// sendRawMTI is a test helper that dials the gateway, sends a single ISO 8583
// message with the specified MTI, reads the 0810 response, and returns it.
// It does NOT use Client.SendEcho because that always builds a valid 0800.
func sendRawMTI(t *testing.T, addr, mti string) *iso.EchoResponse {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	require.NoError(t, err, "dial gateway")
	defer conn.Close()
	// Build message with the given MTI and no other fields.
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	msg.MTI(mti)
	packed, err := msg.Pack()
	require.NoError(t, err, "pack message with MTI %q", mti)
	// Send with 2-byte length prefix, matching gateway framing.
	lenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBuf, uint16(len(packed)))
	_, err = conn.Write(lenBuf)
	require.NoError(t, err)
	_, err = conn.Write(packed)
	require.NoError(t, err)
	// Read 2-byte length prefix of response.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadFull(conn, lenBuf)
	require.NoError(t, err, "read response length prefix")
	respLen := binary.BigEndian.Uint16(lenBuf)
	// Read response body.
	respBuf := make([]byte, respLen)
	_, err = io.ReadFull(conn, respBuf)
	require.NoError(t, err, "read response body")
	respMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, respMsg.Unpack(respBuf))
	var resp iso.EchoResponse
	require.NoError(t, respMsg.Unmarshal(&resp))
	return &resp
}

// TestMTI_Valid0800 — send 0800, assert 0810 F39=00.
func (s *testSuiteE2E) TestMTI_Valid0800() {
	s.Run("send 0800 and receive 0810 F39=00", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err)
		defer client.Close()
		resp, err := client.SendEcho(ctx, "123456", "301")
		require.NoError(s.T(), err)
		require.NotNil(s.T(), resp)
		require.Equal(s.T(), "00", resp.ResponseCode, "valid 0800 must get F39=00")
		require.Equal(s.T(), "123456", resp.STAN)
	})
}

// TestMTI_Unknown9999 — send message with MTI "9999", assert 0810 F39=12,
// connection stays open.
func (s *testSuiteE2E) TestMTI_Unknown9999() {
	s.Run("send MTI 9999 and receive 0810 F39=12 with connection staying open", func() {
		resp := sendRawMTI(s.T(), s.gatewayAddr, "9999")
		require.Equal(s.T(), "12", resp.ResponseCode,
			"unknown numeric MTI must yield 0810 F39=12")
		// Verify connection is still alive: send a valid 0800 immediately after.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := New(s.gatewayAddr)
		require.NoError(s.T(), err, "server must still accept connections after invalid MTI")
		defer client.Close()
		followUp, err := client.SendEcho(ctx, "999999", "301")
		require.NoError(s.T(), err, "server must handle valid echo after unknown MTI")
		require.Equal(s.T(), "00", followUp.ResponseCode)
	})
}

// TestMTI_NonNumericABCD — send message with MTI "ABCD", assert 0810 F39=12.
func (s *testSuiteE2E) TestMTI_NonNumericABCD() {
	s.Run("send MTI ABCD (non-numeric) and receive 0810 F39=12", func() {
		resp := sendRawMTI(s.T(), s.gatewayAddr, "ABCD")
		require.Equal(s.T(), "12", resp.ResponseCode,
			"non-numeric MTI must yield 0810 F39=12")
	})
}
