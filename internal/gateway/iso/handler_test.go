package iso
import (
	"testing"
	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── validateMTI ───────────────────────────────────────────────────────────────

func TestValidateMTI_ValidCases(t *testing.T) {
	valid := []string{"0800", "0810", "9999", "0000", "1234"}
	for _, mti := range valid {
		t.Run(mti, func(t *testing.T) {
			assert.True(t, validateMTI(mti), "expected %q to be valid", mti)
		})
	}
}

func TestValidateMTI_InvalidCases(t *testing.T) {
	tests := []struct {
		name string
		mti  string
	}{
		{"non-numeric uppercase", "ABCD"},
		{"non-numeric lowercase", "abcd"},
		{"too short", "080"},
		{"too long", "08000"},
		{"empty", ""},
		{"space inside", "08 0"},
		{"mixed alpha-numeric", "08AB"},
		{"unicode digit", "①②③④"}, // Unicode digits, not ASCII
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, validateMTI(tc.mti), "expected %q to be invalid", tc.mti)
		})
	}
}

// ── HandleMessage — valid echo ────────────────────────────────────────────────


func TestHandleMessage_Valid0800_Returns0810F39_00(t *testing.T) {
	req := EchoRequest{STAN: "123456", NetworkMgmtInfoCode: "301"}
	msg := iso8583.NewMessage(DiscoverSpec)
	require.NoError(t, msg.Marshal(&req))
	msg.MTI("0800")
	resp, err := HandleMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	mti, err := resp.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti)
	var got EchoResponse
	require.NoError(t, resp.Unmarshal(&got))
	assert.Equal(t, "00", got.ResponseCode)
	assert.Equal(t, "123456", got.STAN)
	assert.Equal(t, "301", got.NetworkMgmtInfoCode)
}

// ── HandleMessage — unknown MTI (numeric, but unsupported) ───────────────────

func TestHandleMessage_Unknown9999_Returns0810F39_12(t *testing.T) {
	msg := iso8583.NewMessage(DiscoverSpec)
	msg.MTI("9999")
	// MOD-72: must return (msg, nil) — NOT (nil, error).
	resp, err := HandleMessage(msg)
	require.NoError(t, err, "unknown numeric MTI must not return a Go error")
	require.NotNil(t, resp)
	mti, err := resp.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti)
	var got EchoResponse
	require.NoError(t, resp.Unmarshal(&got))
	assert.Equal(t, "12", got.ResponseCode, "F39 must be 12 for unsupported MTI")
}

// ── HandleMessage — non-numeric MTI ──────────────────────────────────────────

func TestHandleMessage_NonNumericABCD_Returns0810F39_12(t *testing.T) {
	msg := iso8583.NewMessage(DiscoverSpec)
	msg.MTI("ABCD")
	resp, err := HandleMessage(msg)
	require.NoError(t, err, "non-numeric MTI must not return a Go error")
	require.NotNil(t, resp)
	mti, err := resp.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti)
	var got EchoResponse
	require.NoError(t, resp.Unmarshal(&got))
	assert.Equal(t, "12", got.ResponseCode)
}

// ── HandleMessage — wrong-length MTI ─────────────────────────────────────────

func TestHandleMessage_ShortMTI_Returns0810F39_12(t *testing.T) {
	msg := iso8583.NewMessage(DiscoverSpec)
	msg.MTI("080") // 3 digits instead of 4
	resp, err := HandleMessage(msg)
	require.NoError(t, err)
	require.NotNil(t, resp)
	var got EchoResponse
	require.NoError(t, resp.Unmarshal(&got))
	assert.Equal(t, "12", got.ResponseCode)
}

// ── buildErrorResponse ────────────────────────────────────────────────────────

func TestBuildErrorResponse_SetsF39AndMTI(t *testing.T) {
	// Pass nil for the source message — buildErrorResponse ignores it.
	resp, err := buildErrorResponse(nil, "12")
	require.NoError(t, err)
	require.NotNil(t, resp)
	mti, err := resp.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0810", mti)
	var got EchoResponse
	require.NoError(t, resp.Unmarshal(&got))
	assert.Equal(t, "12", got.ResponseCode)
	// STAN and F70 are zero values — not copied from the source message.
	assert.Equal(t, "", got.STAN)
	assert.Equal(t, "", got.NetworkMgmtInfoCode)
}
