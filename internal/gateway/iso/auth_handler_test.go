package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestAuthHandler_0100To0110 verifies that AuthHandler correctly handles
// an 0100 authorization request and returns a stub 0110 approved response.
func TestAuthHandler_0100To0110(t *testing.T) {
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, msg.Marshal(&struct {
		STAN string `iso8583:"11"`
	}{STAN: "123456"}))
	msg.MTI("0100")

	resp, err := (iso.AuthHandler{}).Handle(msg)
	require.NoError(t, err)

	mti, err := resp.GetMTI()
	require.NoError(t, err)
	require.Equal(t, "0110", mti)

	var out struct {
		STAN         string `iso8583:"11"`
		ResponseCode string `iso8583:"39"`
	}
	require.NoError(t, resp.Unmarshal(&out))
	require.Equal(t, "123456", out.STAN)
	require.Equal(t, "00", out.ResponseCode)
}