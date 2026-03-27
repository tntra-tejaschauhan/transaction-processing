package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestPostAuthHandler_0120To0130 verifies that PostAuthHandler correctly handles
// an 0120 post-authorization request and returns a stub 0130 approved response.
func TestPostAuthHandler_0120To0130(t *testing.T) {
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, msg.Marshal(&struct {
		STAN string `iso8583:"11"`
	}{STAN: "222222"}))
	msg.MTI("0120")

	resp, err := (iso.PostAuthHandler{}).Handle(msg)
	require.NoError(t, err)

	mti, err := resp.GetMTI()
	require.NoError(t, err)
	require.Equal(t, "0130", mti)

	var out struct {
		STAN         string `iso8583:"11"`
		ResponseCode string `iso8583:"39"`
	}
	require.NoError(t, resp.Unmarshal(&out))
	require.Equal(t, "222222", out.STAN)
	require.Equal(t, "00", out.ResponseCode)
}