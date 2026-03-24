package iso

// EchoRequest holds the fields from an ISO 8583 0800 Network Management
// Request (echo) message relevant to the Discover gateway.
//
// The iso8583 struct tags map each exported field to its ISO field number.
// Unknown or extra fields in the incoming message are ignored during Unmarshal.
type EchoRequest struct {
	// STAN is the Systems Trace Audit Number (field 11), a 6-digit numeric
	// string that uniquely identifies the transaction within a session. It
	// must be echoed back unchanged in the 0810 response.
	STAN string `iso8583:"11"`

	// NetworkMgmtInfoCode is field 70, the Network Management Information
	// Code. For an echo message the Discover network sends "301". It must be
	// echoed back in the response.
	NetworkMgmtInfoCode string `iso8583:"70"`
}

// EchoResponse holds the fields populated in an ISO 8583 0810 Network
// Management Response (echo response) message sent back to the Discover
// network.
type EchoResponse struct {
	// STAN mirrors the STAN from the corresponding EchoRequest (field 11).
	STAN string `iso8583:"11"`

	// ResponseCode is field 39. The value "00" indicates an approved /
	// successful echo acknowledgement.
	ResponseCode string `iso8583:"39"`

	// NetworkMgmtInfoCode mirrors field 70 from the request.
	NetworkMgmtInfoCode string `iso8583:"70"`
}
