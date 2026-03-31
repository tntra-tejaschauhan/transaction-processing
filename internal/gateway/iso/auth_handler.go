package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
	"github.com/rs/zerolog"
)

// AuthHandler handles 0100 -> 0110.
type AuthHandler struct {
	logger *zerolog.Logger
}

// WithLogger allows the router to inject a context logger per-request.
func (h AuthHandler) WithLogger(logger zerolog.Logger) MessageHandler {
	return AuthHandler{logger: &logger}
}

// Handle processes an 0100 Authorization Request and emits a PCI-masked log.
func (h AuthHandler) Handle(msg *iso8583.Message) (*iso8583.Message, error) {
	var req AuthRequest
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("AuthHandler.Handle: unmarshal 0100: %w", err)
	}

	if h.logger != nil {
		h.logger.Debug().
			Str("pan", MaskPAN(req.PAN)). //nolint:gosec // PAN masked
			Str("stan", req.STAN).
			Str("terminal_id", req.TerminalID).
			Msg("auth request received")
	}

	return BuildAuth0110(&req)
}

// buildStubApprovedResponse is a helper that extracts the STAN from a request
// and builds a protocol-compliant response with the specified MTI and F39=00.
func buildStubApprovedResponse(msg *iso8583.Message, respMTI, op string) (*iso8583.Message, error) {
	var req struct {
		STAN string `iso8583:"11"`
	}
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("%s: unmarshal request: %w", op, err)
	}

	resp := responseCodeOnly{
		STAN:         req.STAN,
		ResponseCode: "00",
	}

	out := iso8583.NewMessage(DiscoverSpec)
	if err := out.Marshal(&resp); err != nil {
		return nil, fmt.Errorf("%s: marshal response: %w", op, err)
	}
	out.MTI(respMTI)

	return out, nil
}