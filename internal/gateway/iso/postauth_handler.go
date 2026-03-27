package iso

import "github.com/moov-io/iso8583"

// PostAuthHandler handles 0120 -> 0130. It is a stub handler returning a valid but
// unprocessed response to allow the card network to receive correctly formed
// responses while EPIC-03 implements the real post-authorization logic.
type PostAuthHandler struct{}

// Handle processes an 0120 Post-Authorization Request. Currently returns a stub
// 0130 response with F39=00 (Approved) and echoes the STAN from field 11.
func (h PostAuthHandler) Handle(msg *iso8583.Message) (*iso8583.Message, error) {
	// TODO EPIC-03: implement real post-authorization logic.
	return buildStubApprovedResponse(msg, "0130", "PostAuthHandler.Handle")
}