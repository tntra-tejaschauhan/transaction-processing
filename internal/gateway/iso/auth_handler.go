package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// AuthHandler handles 0100 -> 0110. It is a stub handler returning a valid but
// unprocessed response to allow the card network to receive correctly formed
// responses while EPIC-03 implements the real authorization logic.
type AuthHandler struct{}

// Handle processes an 0100 Authorization Request. Currently returns a stub
// 0110 response with F39=00 (Approved) and echoes the STAN from field 11.
func (h AuthHandler) Handle(msg *iso8583.Message) (*iso8583.Message, error) {
	// TODO EPIC-03: implement real authorization logic.
	return buildStubApprovedResponse(msg, "0110", "AuthHandler.Handle")
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