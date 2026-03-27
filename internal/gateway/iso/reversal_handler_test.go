package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestReversalHandler_0400To0410 verifies that ReversalHandler correctly handles
// an 0400 reversal request and returns a stub 0410 approved response.
func TestReversalHandler_0400To0410(t *testing.T) {
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, msg.Marshal(&struct {
		STAN string `iso8583:"11"`
	}{STAN: "333333"}))
	msg.MTI("0400")

	resp, err := (iso.ReversalHandler{}).Handle(msg)
	require.NoError(t, err)

	mti, err := resp.GetMTI()
	require.NoError(t, err)
	require.Equal(t, "0410", mti)

	var out struct {
		STAN         string `iso8583:"11"`
		ResponseCode string `iso8583:"39"`
	}
	require.NoError(t, resp.Unmarshal(&out))
	require.Equal(t, "333333", out.STAN)
	require.Equal(t, "00", out.ResponseCode)
}