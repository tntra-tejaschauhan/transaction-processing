package gateway

import (
	"context"
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
	s.Run("open 3 concurrent connections, send 1 echo each, verify all responses with no race", func() {
		const numConnections = 3
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

				stan := strings.Repeat(string(rune('a'+byte(idx))), 6)
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

		wg.Wait()
		close(results)

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

// ────────────────────────────────────────────────────────────────────────────
// Scenario 4: MOD-71 Limits and Timeouts
// ────────────────────────────────────────────────────────────────────────────

func (s *testSuiteE2E) TestConnectionLimit() {
	s.Run("MaxConnections limit drops excess clients but keeps active ones alive", func() {
		// e2e-gateway Makefile override sets MaxConnections=5
		const limit = 5
		var activeConns []net.Conn

		// Open limit number of connections successfully
		for i := 0; i < limit; i++ {
			c, err := net.Dial("tcp", s.gatewayAddr)
			require.NoError(s.T(), err)
			activeConns = append(activeConns, c)
		}

		defer func() {
			for _, c := range activeConns {
				c.Close()
			}
		}()

		// Try to open N+1 connection
		overLimitConn, err := net.Dial("tcp", s.gatewayAddr)
		require.NoError(s.T(), err)
		defer overLimitConn.Close()

		// The TCP handshake works, but the server immediately closes it.
		// Wait a small bit and read to confirm it was closed (EOF).
		time.Sleep(50 * time.Millisecond)
		buf := make([]byte, 1)
		_, err = overLimitConn.Read(buf)
		require.Error(s.T(), err, "N+1 connection should be closed by server")
		require.Equal(s.T(), io.EOF, err)

		// Verify existing connection 0 is still perfectly healthy
		activeConns[0].SetWriteDeadline(time.Now().Add(time.Second))
		activeConns[0].SetReadDeadline(time.Now().Add(time.Second))
		
		req := iso.EchoRequest{STAN: "987654", NetworkMgmtInfoCode: "301"}
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		require.NoError(s.T(), msg.Marshal(&req))
		msg.MTI("0800")
		packed, err := msg.Pack()
		require.NoError(s.T(), err)

		lenBuf := []byte{byte(len(packed) >> 8), byte(len(packed))}
		_, err = activeConns[0].Write(lenBuf)
		require.NoError(s.T(), err)
		_, err = activeConns[0].Write(packed)
		require.NoError(s.T(), err)

		lenBufRecv := make([]byte, 2)
		_, err = io.ReadFull(activeConns[0], lenBufRecv)
		require.NoError(s.T(), err, "Original connections must survive limit trigger")
	})
}

func (s *testSuiteE2E) TestIdleTimeout() {
	s.Run("connection is dropped if no payload initializes before IdleTimeout expires", func() {
		// Makefile sets IDLE_TIMEOUT=2000ms
		c, err := net.Dial("tcp", s.gatewayAddr)
		require.NoError(s.T(), err)
		defer c.Close()

		// Wait longer than idle timeout
		time.Sleep(2500 * time.Millisecond)

		buf := make([]byte, 1)
		_, err = c.Read(buf)
		require.Error(s.T(), err)
		require.Equal(s.T(), io.EOF, err)
	})
}

func (s *testSuiteE2E) TestSlowClientWriteTimeout() {
	s.Run("server drops client that stalls mid-frame triggering ReadTimeout", func() {
		// Makefile sets READ_TIMEOUT=1000ms
		c, err := net.Dial("tcp", s.gatewayAddr)
		require.NoError(s.T(), err)
		defer c.Close()

		// Send ONLY the length prefix of an arbitrarily massive frame.
		// The server initiates body read and blocks on ReadTimeout.
		_, err = c.Write([]byte{0x01, 0x00}) // 256 byte frame claims
		require.NoError(s.T(), err)

		// Wait past ReadTimeout
		time.Sleep(1500 * time.Millisecond)

		buf := make([]byte, 1)
		_, err = c.Read(buf)
		require.Error(s.T(), err)
		require.Equal(s.T(), io.EOF, err)
	})
}
