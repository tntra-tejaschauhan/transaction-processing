package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestBuildEcho0810_BasicResponse verifies all required fields in the
// generated 0810 response for a standard 0800 echo request.
func TestBuildEcho0810_BasicResponse(t *testing.T) {
	req := &iso.EchoRequest{
		STAN:                "123456",
		NetworkMgmtInfoCode: "301",
	}

	msg, err := iso.BuildEcho0810(req)
	require.NoError(t, err)
	require.NotNil(t, msg)

	// Verify MTI is 0810.
	mti, err := msg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti, "response MTI must be 0810")

	// Unmarshal into EchoResponse and verify individual fields.
	var resp iso.EchoResponse
	require.NoError(t, msg.Unmarshal(&resp))

	assert.Equal(t, "123456", resp.STAN, "F11 STAN must match request")
	assert.Equal(t, "00", resp.ResponseCode, "F39 ResponseCode must be '00' (approved)")
	assert.Equal(t, "301", resp.NetworkMgmtInfoCode, "F70 NetworkMgmtInfoCode must match request")
}

// TestBuildEcho0810_STANVariants checks that the STAN value is always echoed
// back correctly for different input values.
func TestBuildEcho0810_STANVariants(t *testing.T) {
	tests := []struct {
		name string
		stan string
		f70  string
	}{
		{"min STAN", "000001", "301"},
		{"max STAN", "999999", "301"},
		{"mid STAN", "543210", "301"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &iso.EchoRequest{STAN: tc.stan, NetworkMgmtInfoCode: tc.f70}
			msg, err := iso.BuildEcho0810(req)
			require.NoError(t, err)

			var resp iso.EchoResponse
			require.NoError(t, msg.Unmarshal(&resp))
			assert.Equal(t, tc.stan, resp.STAN)
			assert.Equal(t, "00", resp.ResponseCode)
			assert.Equal(t, tc.f70, resp.NetworkMgmtInfoCode)
		})
	}
}

// TestHandleMessage_Echo0800 tests the full dispatch path via HandleMessage
// for a valid 0800 echo request.
func TestHandleMessage_Echo0800(t *testing.T) {
	req := &iso.EchoRequest{
		STAN:                "777777",
		NetworkMgmtInfoCode: "301",
	}

	// Build a raw 0800 message to pass through HandleMessage.
	inMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, inMsg.Marshal(req))
	inMsg.MTI("0800")

	outMsg, err := iso.HandleMessage(inMsg)
	require.NoError(t, err)
	require.NotNil(t, outMsg)

	mti, err := outMsg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti)

	var resp iso.EchoResponse
	require.NoError(t, outMsg.Unmarshal(&resp))
	assert.Equal(t, req.STAN, resp.STAN)
	assert.Equal(t, "00", resp.ResponseCode)
	assert.Equal(t, req.NetworkMgmtInfoCode, resp.NetworkMgmtInfoCode)
}

// TestHandleMessage_UnknownMTI ensures that an unsupported MTI returns an
// error and does not panic.
func TestHandleMessage_UnknownMTI(t *testing.T) {
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	msg.MTI("0200")

	_, err := iso.HandleMessage(msg)
	assert.Error(t, err, "unsupported MTI must return an error, not panic")
}
