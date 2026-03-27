package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// HandleMessage receives a parsed, unpacked ISO 8583 message and returns an
// appropriate response message.
//
// Routing is done by MTI:
//   - "0800" → BuildEcho0810 (Network Management Request → Response)
//   - anything else → descriptive error
//   - unrecognised MTI (e.g. "9999") → 0810 F39=12, nil error
//
// The caller is responsible for packing the returned message and writing it
// to the TCP connection via NetworkHeader framing.
// 
// KEY CONTRACT:
// HandleMessage ALWAYS returns a non-nil *iso8583.Message when error is nil.
// A non-nil error means a truly fatal condition (can't read MTI, can't build
// the error response itself) — the caller must treat this as fatal and close
// the connection.
func HandleMessage(msg *iso8583.Message) (*iso8583.Message, error) {
	mti, err := msg.GetMTI()
	if err != nil {
		return nil, fmt.Errorf("HandleMessage: get MTI: %w", err)
	}

	// MTI format gate: must be exactly 4 ASCII decimal digits.
	if !validateMTI(mti) {
		resp, err := buildErrorResponse(msg, "12")
		if err != nil {
			return nil, fmt.Errorf("HandleMessage: build error response for invalid MTI %q: %w", mti, err)
		}
		return resp, nil
	}

	switch mti {
	case "0800":
		return handleEchoRequest(msg)
	default:
		// Recognised format but unsupported MTI — send 0810 F39=12.
		resp, err := buildErrorResponse(msg, "12")
		if err != nil {
			return nil, fmt.Errorf("HandleMessage: build error response for unsupported MTI %q: %w", mti, err)
		}
		return resp, nil
	}
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

// handleEchoRequest processes an 0800 Network Management Request.
func handleEchoRequest(msg *iso8583.Message) (*iso8583.Message, error) {
	var req EchoRequest
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("handleEchoRequest: unmarshal 0800: %w", err)
	}

	resp, err := BuildEcho0810(&req)
	if err != nil {
		return nil, fmt.Errorf("handleEchoRequest: build 0810: %w", err)
	}

	return resp, nil
}
