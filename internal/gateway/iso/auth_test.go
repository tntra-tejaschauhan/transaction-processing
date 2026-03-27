package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestAuthRequest_RoundTrip verifies that an AuthRequest with all 18 fields
// survives a full Marshal → Pack → Unpack → Unmarshal round-trip without
// data loss. Also confirms F2 PAN is extracted correctly (acceptance criterion).
func TestAuthRequest_RoundTrip(t *testing.T) {
	original := iso.AuthRequest{
		PAN:                    "4111111111111111",
		ProcessingCode:         "000000",
		Amount:                 "000000010000",
		TransmissionDateTime:   "0326143000",
		STAN:                   "123456",
		LocalTime:              "143000",
		LocalDate:              "0326",
		ExpiryDate:             "2812",
		POSEntryMode:           "051",
		POSConditionCode:       "00",
		AcquiringInstitutionID: "12345",
		Track2Data:             "4111111111111111=2812",
		RRN:                    "123456789012",
		TerminalID:             "TERM0001",
		MerchantID:             "MERCHANT000001 ",
		AdditionalData:         "ADDITIONALDATA001",
		CurrencyCode:           "840",
		PINBlock:               "\x00\x00\x00\x00\x00\x00\x00\x00", // 8 zero bytes
	}

	// Marshal into a message and pack to raw bytes.
	sendMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, sendMsg.Marshal(&original), "Marshal must succeed for a valid AuthRequest")
	sendMsg.MTI("0100")

	packed, err := sendMsg.Pack()
	require.NoError(t, err, "Pack must succeed")
	require.NotEmpty(t, packed)

	// Unpack from raw bytes and unmarshal back.
	recvMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, recvMsg.Unpack(packed), "Unpack must succeed for a validly packed 0100")

	var received iso.AuthRequest
	require.NoError(t, recvMsg.Unmarshal(&received))

	// Verify MTI survived.
	mti, err := recvMsg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0100", mti, "MTI must survive round-trip")

	// Verify all fields survive (acceptance criteria from MOD-73 TASK-1).
	assert.Equal(t, original.PAN, received.PAN, "F2 PAN must survive round-trip")
	assert.Equal(t, original.ProcessingCode, received.ProcessingCode, "F3 must survive")
	assert.Equal(t, original.Amount, received.Amount, "F4 must survive")
	assert.Equal(t, original.TransmissionDateTime, received.TransmissionDateTime, "F7 must survive")
	assert.Equal(t, original.STAN, received.STAN, "F11 must survive")
	assert.Equal(t, original.LocalTime, received.LocalTime, "F12 must survive")
	assert.Equal(t, original.LocalDate, received.LocalDate, "F13 must survive")
	assert.Equal(t, original.ExpiryDate, received.ExpiryDate, "F14 must survive")
	assert.Equal(t, original.POSEntryMode, received.POSEntryMode, "F22 must survive")
	assert.Equal(t, original.POSConditionCode, received.POSConditionCode, "F25 must survive")
	assert.Equal(t, original.AcquiringInstitutionID, received.AcquiringInstitutionID, "F32 must survive")
	assert.Equal(t, original.Track2Data, received.Track2Data, "F35 must survive")
	assert.Equal(t, original.RRN, received.RRN, "F37 must survive")
	assert.Equal(t, original.TerminalID, received.TerminalID, "F41 must survive")
	assert.Equal(t, original.MerchantID, received.MerchantID, "F42 must survive")
	assert.Equal(t, original.AdditionalData, received.AdditionalData, "F48 must survive")
	assert.Equal(t, original.CurrencyCode, received.CurrencyCode, "F49 must survive")

	// Acceptance criterion: F2 PAN '4111111111111111' extracted correctly.
	assert.Equal(t, "4111111111111111", received.PAN,
		"F2 PAN '4111111111111111' must be extracted correctly as string")
}

// TestAuthRequest_SecondaryBitmap verifies that moov-io/iso8583 correctly
// decodes a message containing a secondary bitmap field (F90, Original Data
// Elements — field 90 is in the secondary bitmap range 65–128).
//
// moov-io handles secondary bitmap parsing automatically; this test is the
// explicit guard that DiscoverSpec has F90 declared so Unpack never fails
// with "field not defined in spec" for secondary-bitmap messages.
func TestAuthRequest_SecondaryBitmap(t *testing.T) {
	// authRequestWithF90 extends AuthRequest with F90 so we can build a
	// message that triggers secondary bitmap encoding.
	type authRequestWithF90 struct {
		iso.AuthRequest
		OriginalData string `iso8583:"90"`
	}

	// F90 value: MTI(4) + STAN(12) + datetime(10) + acquirer(11) + 5 zeros
	originalData := "0100" + "123456789012" + "0326143000" + "12345678901" + "00000"

	req := authRequestWithF90{
		AuthRequest: iso.AuthRequest{
			PAN:    "4111111111111111",
			STAN:   "123456",
			Amount: "000000010000",
		},
		OriginalData: originalData,
	}

	// Build and pack — moov-io should include secondary bitmap automatically.
	sendMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, sendMsg.Marshal(&req))
	sendMsg.MTI("0100")

	packed, err := sendMsg.Pack()
	require.NoError(t, err, "Pack must succeed with secondary bitmap field present")

	// Unpack — must not return "field not defined in spec" error for F90.
	recvMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, recvMsg.Unpack(packed),
		"Unpack must handle secondary bitmap fields without error")

	// Unmarshal and verify F90 survived.
	var received authRequestWithF90
	require.NoError(t, recvMsg.Unmarshal(&received))
	assert.Equal(t, originalData, received.OriginalData,
		"F90 (secondary bitmap field) must survive Pack/Unpack round-trip")
}

// TestBuildAuth0110_BasicResponse verifies all required fields in the
// generated 0110 response for a standard 0100 auth request.
func TestBuildAuth0110_BasicResponse(t *testing.T) {
	req := &iso.AuthRequest{
		STAN:         "654321",
		RRN:          "098765432109",
		TerminalID:   "TERM0001",
		PAN:          "4111111111111111",
		Amount:       "000000010000",
		CurrencyCode: "840",
	}

	msg, err := iso.BuildAuth0110(req)
	require.NoError(t, err)
	require.NotNil(t, msg)

	mti, err := msg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0110", mti, "response MTI must be 0110")

	var resp iso.AuthResponse
	require.NoError(t, msg.Unmarshal(&resp))
	assert.Equal(t, "654321", resp.STAN, "F11 STAN must be echoed from request")
	assert.Equal(t, "098765432109", resp.RRN, "F37 RRN must be echoed from request")
	assert.Equal(t, "00", resp.ResponseCode, "F39 ResponseCode must be '00' (approved)")
}

// TestHandleMessage_Auth0100 tests the full dispatch path via HandleMessage
// for a valid 0100 authorization request.
func TestHandleMessage_Auth0100(t *testing.T) {
	req := &iso.AuthRequest{
		PAN:    "4111111111111111",
		STAN:   "111111",
		RRN:    "111111111111",
		Amount: "000000050000",
	}

	inMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, inMsg.Marshal(req))
	inMsg.MTI("0100")

	outMsg, err := iso.HandleMessage(inMsg, zerolog.Nop())
	require.NoError(t, err)
	require.NotNil(t, outMsg)

	mti, err := outMsg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0110", mti)

	var resp iso.AuthResponse
	require.NoError(t, outMsg.Unmarshal(&resp))
	assert.Equal(t, "00", resp.ResponseCode)
	assert.Equal(t, req.STAN, resp.STAN)
}
