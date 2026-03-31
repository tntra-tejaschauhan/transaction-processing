package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
	"github.com/rs/zerolog"
)

var defaultRegistry = NewHandlerRegistry()

// HandleMessage receives a parsed, unpacked ISO 8583 message and a logger,
// and returns an appropriate response message.
//
// Routing is done by MTI:
//   - MTI format invalid (not exactly 4 numeric digits) → 0810 F39=12, nil error
//   - "0800" → BuildEcho0810  (Network Management Request → Response)
//   - "0100" → handleAuthRequest with logger (PCI-masked PAN logging)
//   - "0120" → PostAuthHandler stub → 0130 F39=00
//   - "0400" → ReversalHandler stub → 0410 F39=00
//   - unrecognised MTI (e.g. "9999") → 0810 F39=12, nil error
//
// The caller is responsible for packing the returned message and writing it
// to the TCP connection via NetworkHeader framing.
func HandleMessage(msg *iso8583.Message, logger zerolog.Logger) (*iso8583.Message, error) {
	mti, err := msg.GetMTI()
	if err != nil {
		return nil, fmt.Errorf("HandleMessage: get MTI: %w", err)
	}

	// MOD-72: reject MTIs that are not exactly 4 ASCII decimal digits.
	if !validateMTI(mti) {
		return buildErrorResponse(msg, "12")
	}

	// MOD-73: 0100 auth requests need a logger for PCI-masked PAN logging.
	// This is achieved by passing the logger to Dispatch, which injects
	// it into any handler that implements LoggerAwareHandler (e.g. AuthHandler).
	return defaultRegistry.Dispatch(mti, msg, logger)
}

// validateMTI reports whether mti is exactly 4 ASCII decimal digit characters.
// Non-numeric characters, wrong length, or empty strings all return false.
func validateMTI(mti string) bool {
	if len(mti) != 4 {
		return false
	}
	for i := 0; i < len(mti); i++ {
		if mti[i] < '0' || mti[i] > '9' {
			return false
		}
	}
	return true
}

// buildErrorResponse constructs a minimal 0810 response with the given
// responseCode in F39. STAN (F11) and NetworkMgmtInfoCode (F70) are left at
// their zero values — they cannot be reliably parsed from a malformed message.
func buildErrorResponse(_ *iso8583.Message, responseCode string) (*iso8583.Message, error) {
	resp := EchoResponse{
		ResponseCode: responseCode,
	}
	msg := iso8583.NewMessage(DiscoverSpec)
	if err := msg.Marshal(&resp); err != nil {
		return nil, fmt.Errorf("buildErrorResponse: marshal: %w", err)
	}
	msg.MTI("0810")
	return msg, nil
}


