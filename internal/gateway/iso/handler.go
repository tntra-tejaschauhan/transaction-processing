package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
	"github.com/rs/zerolog"
)

// HandleMessage receives a parsed, unpacked ISO 8583 message and returns an
// appropriate response message.
//
// Routing is done by MTI:
//   - "0800" → BuildEcho0810  (Network Management Request → Response)
//   - "0100" → BuildAuth0110  (Authorization Request → Response)
//   - anything else → descriptive error
//
// The caller is responsible for packing the returned message and writing it
// to the TCP connection via NetworkHeader framing.
//
// The logger parameter is used to emit a masked-PAN debug log for 0100
// messages (PCI requirement). Pass zerolog.Nop() in tests that do not need
// log output.
func HandleMessage(msg *iso8583.Message, logger zerolog.Logger) (*iso8583.Message, error) {
	mti, err := msg.GetMTI()
	if err != nil {
		return nil, fmt.Errorf("HandleMessage: get MTI: %w", err)
	}

	switch mti {
	case "0800":
		return handleEchoRequest(msg)
	case "0100":
		return handleAuthRequest(msg, logger)
	default:
		return nil, fmt.Errorf("HandleMessage: unsupported MTI %q", mti)
	}
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

// handleAuthRequest processes a 0100 Authorization Request.
//
// PCI rule: F2 (PAN) is logged as a masked value via MaskPAN. F52 (PIN Block)
// is never logged. The // nolint:gosec comments below mark the intentional,
// controlled handling of PAN and PIN Block for processing purposes only.
func handleAuthRequest(msg *iso8583.Message, logger zerolog.Logger) (*iso8583.Message, error) {
	var req AuthRequest
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("handleAuthRequest: unmarshal 0100: %w", err)
	}

	// PCI requirement: log PAN in masked form only — never log plaintext PAN.
	// MaskPAN returns "411111******1111" style for 16-digit PANs.
	logger.Debug().
		Str("pan", MaskPAN(req.PAN)). //nolint:gosec // PAN masked before logging; plaintext never written to log
		Str("stan", req.STAN).
		Str("terminal_id", req.TerminalID).
		Msg("auth request received")

	resp, err := BuildAuth0110(&req)
	if err != nil {
		return nil, fmt.Errorf("handleAuthRequest: build 0110: %w", err)
	}

	return resp, nil
}
