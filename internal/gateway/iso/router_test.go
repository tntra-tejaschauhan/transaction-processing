package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// testHandler is a mock MessageHandler for testing the registry dispatch.
type testHandler struct {
	respMTI string
}

func (h testHandler) Handle(msg *iso8583.Message) (*iso8583.Message, error) {
	out := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(&testing.T{}, out.Marshal(&struct {
		STAN         string `iso8583:"11"`
		ResponseCode string `iso8583:"39"`
	}{STAN: "123456", ResponseCode: "00"}))
	out.MTI(h.respMTI)
	return out, nil
}

// buildRequest is a test helper that creates an ISO 8583 message with
// a specific MTI and STAN.
func buildRequest(t *testing.T, mti, stan string) *iso8583.Message {
	t.Helper()
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, msg.Marshal(&struct {
		STAN string `iso8583:"11"`
	}{STAN: stan}))
	msg.MTI(mti)
	return msg
}

// TestRegistry_Dispatch0800 verifies that the registry correctly routes
// an 0800 MTI to the EchoHandler (or a mock) and returns a valid response.
func TestRegistry_Dispatch0800(t *testing.T) {
	reg := iso.NewHandlerRegistry()
	req := buildRequest(t, "0800", "123456")

	resp, err := reg.Dispatch("0800", req)
	require.NoError(t, err)

	mti, err := resp.GetMTI()
	require.NoError(t, err)
	require.Equal(t, "0810", mti)

	var out struct {
		ResponseCode string `iso8583:"39"`
	}
	require.NoError(t, resp.Unmarshal(&out))
	require.Equal(t, "00", out.ResponseCode)
}

// TestRegistry_DispatchUnknown0300 verifies that dispatching an unknown MTI
// returns an 0810 response with F39=12, per MOD-74 requirements.
func TestRegistry_DispatchUnknown0300(t *testing.T) {
	reg := iso.NewHandlerRegistry()
	req := buildRequest(t, "0300", "654321")

	resp, err := reg.Dispatch("0300", req)
	require.NoError(t, err)

	mti, err := resp.GetMTI()
	require.NoError(t, err)
	require.Equal(t, "0810", mti)

	var out struct {
		STAN         string `iso8583:"11"`
		ResponseCode string `iso8583:"39"`
	}
	require.NoError(t, resp.Unmarshal(&out))
	require.Equal(t, "654321", out.STAN)
	require.Equal(t, "12", out.ResponseCode)
}