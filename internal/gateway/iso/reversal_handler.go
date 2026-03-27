package iso

import "github.com/moov-io/iso8583"

// ReversalHandler handles 0400 -> 0410. It is a stub handler returning a valid but
// unprocessed response to allow the card network to receive correctly formed
// responses while EPIC-03 implements the real reversal logic.
type ReversalHandler struct{}

// Handle processes an 0400 Reversal Request. Currently returns a stub
// 0410 response with F39=00 (Approved) and echoes the STAN from field 11.
func (h ReversalHandler) Handle(msg *iso8583.Message) (*iso8583.Message, error) {
	// TODO EPIC-03: implement real reversal logic.
	return buildStubApprovedResponse(msg, "0410", "ReversalHandler.Handle")
}