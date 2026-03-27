package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// AuthRequest holds the fields from an ISO 8583 0100 Authorization Request
// message for the Discover card network.
//
// iso8583 struct tags map each exported field to its ISO field number.
// Fields present in the incoming message but absent from this struct are
// silently ignored during Unmarshal — the spec (DiscoverSpec) still defines
// them so that Unpack never fails.
//
// PCI note: PAN (F2) and PINBlock (F52) are sensitive. Never log these fields
// directly — use MaskPAN(req.PAN) for F2; do not log F52 at all.
type AuthRequest struct {
	// F2 – Primary Account Number (LLVAR, up to 19 digits).
	// PCI-sensitive: use MaskPAN(req.PAN) in all log statements.
	PAN string `iso8583:"2"` //nolint:gosec // PAN intentionally held for processing, never logged in plaintext

	// F3 – Processing Code (6 digits, fixed). Identifies the type of
	// transaction: first 2 = transaction type, middle 2 = from account,
	// last 2 = to account. Example: "000000" = purchase.
	ProcessingCode string `iso8583:"3"`

	// F4 – Amount, Transaction (12 digits, fixed, implied decimal).
	// Example: "000000010000" = $100.00.
	Amount string `iso8583:"4"`

	// F7 – Transmission Date and Time (10 digits, MMDDhhmmss).
	TransmissionDateTime string `iso8583:"7"`

	// F11 – Systems Trace Audit Number / STAN (6 digits, fixed).
	// Uniquely identifies the transaction within a session.
	STAN string `iso8583:"11"`

	// F12 – Local Transaction Time (6 digits, hhmmss).
	LocalTime string `iso8583:"12"`

	// F13 – Local Transaction Date (4 digits, MMDD).
	LocalDate string `iso8583:"13"`

	// F14 – Expiration Date (4 digits, YYMM).
	ExpiryDate string `iso8583:"14"`

	// F22 – Point-Of-Service Entry Mode (3 digits, fixed).
	POSEntryMode string `iso8583:"22"`

	// F25 – Point-Of-Service Condition Code (2 digits, fixed).
	POSConditionCode string `iso8583:"25"`

	// F32 – Acquiring Institution ID Code (LLVAR, up to 11 digits).
	AcquiringInstitutionID string `iso8583:"32"`

	// F35 – Track 2 Data (LLVAR, up to 37 chars).
	Track2Data string `iso8583:"35"`

	// F37 – Retrieval Reference Number (12 chars, fixed).
	// Must be echoed back unchanged in the 0110 response.
	RRN string `iso8583:"37"`

	// F41 – Card Acceptor Terminal ID (8 chars, fixed).
	TerminalID string `iso8583:"41"`

	// F42 – Card Acceptor ID Code / Merchant ID (15 chars, fixed).
	MerchantID string `iso8583:"42"`

	// F48 – Additional Data, Private Use (LLLVAR, up to 999 chars).
	AdditionalData string `iso8583:"48"`

	// F49 – Currency Code, Transaction (3 digits, ISO 4217).
	CurrencyCode string `iso8583:"49"`

	// F52 – PIN Block (8 bytes, binary).
	// PCI-sensitive: must never appear in any log output.
	PINBlock string `iso8583:"52"` //nolint:gosec // PIN Block intentionally held for processing, never logged
}

// AuthResponse holds the fields populated in an ISO 8583 0110 Authorization
// Response message sent back to the Discover network.
type AuthResponse struct {
	// F11 – Systems Trace Audit Number / STAN. Echoed from the request.
	STAN string `iso8583:"11"`

	// F37 – Retrieval Reference Number. Echoed from the request.
	RRN string `iso8583:"37"`

	// F38 – Authorization ID Response (6 chars). Set by the authoriser.
	// For gateway-level stub responses this is left blank.
	AuthID string `iso8583:"38"`

	// F39 – Response Code (2 chars). "00" = approved.
	ResponseCode string `iso8583:"39"`
}

// BuildAuth0110 constructs an ISO 8583 0110 Authorization Response message
// from an incoming 0100 AuthRequest.
//
// The response:
//   - Sets MTI to "0110"
//   - Echoes F11 (STAN) unchanged from the request
//   - Echoes F37 (RRN) unchanged from the request
//   - Sets F39 (ResponseCode) to "00" (approved)
//
// PAN (F2) and PIN Block (F52) are intentionally NOT included in the response
// to prevent sensitive data from leaving the gateway.
func BuildAuth0110(req *AuthRequest) (*iso8583.Message, error) {
	resp := AuthResponse{
		STAN:         req.STAN,
		RRN:          req.RRN,
		AuthID:       "",  // stub: no upstream authoriser in this story
		ResponseCode: "00",
	}

	msg := iso8583.NewMessage(DiscoverSpec)

	if err := msg.Marshal(&resp); err != nil {
		return nil, fmt.Errorf("BuildAuth0110: marshal response: %w", err)
	}

	msg.MTI("0110")

	return msg, nil
}
