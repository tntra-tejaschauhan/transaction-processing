// Package iso provides ISO 8583 message types, specifications, and handling
// for the Discover card network TCP gateway.
package iso

import (
	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
)

// DiscoverSpec is the ISO 8583 MessageSpec describing all fields that the
// Discover card network may send over the TCP connection.
//
// The spec covers:
//   - F0  : MTI (4 digits, fixed)
//   - F1  : Primary Bitmap (8 bytes, binary)
//   - F2  : Primary Account Number (up to 19 digits, LLVAR)
//   - F3  : Processing Code (6 digits, fixed)
//   - F4  : Amount, Transaction (12 digits, fixed)
//   - F7  : Transmission Date and Time (10 digits, fixed)
//   - F11 : Systems Trace Audit Number / STAN (6 digits, fixed)
//   - F12 : Local Transaction Time (6 digits, fixed)
//   - F13 : Local Transaction Date (4 digits, fixed)
//   - F14 : Expiration Date (4 digits, fixed)
//   - F22 : Point-Of-Service Entry Mode (3 digits, fixed)
//   - F25 : Point-Of-Service Condition Code (2 digits, fixed)
//   - F32 : Acquiring Institution ID Code (LLVAR, up to 11)
//   - F35 : Track 2 Data (LLVAR, up to 37)
//   - F37 : Retrieval Reference Number (12 chars, fixed)
//   - F38 : Authorization ID Response (6 chars, fixed)
//   - F39 : Response Code (2 chars, fixed)
//   - F41 : Card Acceptor Terminal ID (8 chars, fixed)
//   - F42 : Card Acceptor ID Code / Merchant ID (15 chars, fixed)
//   - F43 : Card Acceptor Name/Location (40 chars, fixed)
//   - F49 : Currency Code, Transaction (3 digits, fixed)
//   - F63 : Reserved Private (LLLVAR, up to 999)
//   - F70 : Network Management Information Code (3 digits, fixed)
//
// A full spec is defined so that moov-io/iso8583 never fails during Unpack
// due to an undeclared field present in the incoming message.
var DiscoverSpec = &iso8583.MessageSpec{
	Name: "Discover ISO 8583",
	Fields: map[int]field.Field{
		// MTI
		0: field.NewString(&field.Spec{
			Length:      4,
			Description: "Message Type Indicator",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// Primary Bitmap (moov-io handles the bitmap automatically)
		1: field.NewBitmap(&field.Spec{
			Length:      8,
			Description: "Bitmap",
			Enc:         encoding.Binary,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F2 – Primary Account Number (LLVAR, up to 19 digits)
		2: field.NewString(&field.Spec{
			Length:      19,
			Description: "Primary Account Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// F3 – Processing Code (6 digits, fixed)
		3: field.NewString(&field.Spec{
			Length:      6,
			Description: "Processing Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F4 – Amount, Transaction (12 digits, fixed)
		4: field.NewString(&field.Spec{
			Length:      12,
			Description: "Amount, Transaction",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F7 – Transmission Date and Time (10 digits, fixed — MMDDhhmmss)
		7: field.NewString(&field.Spec{
			Length:      10,
			Description: "Transmission Date and Time",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F11 – STAN (6 digits, fixed)
		11: field.NewString(&field.Spec{
			Length:      6,
			Description: "Systems Trace Audit Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F12 – Local Transaction Time (6 digits, fixed — hhmmss)
		12: field.NewString(&field.Spec{
			Length:      6,
			Description: "Local Transaction Time",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F13 – Local Transaction Date (4 digits, fixed — MMDD)
		13: field.NewString(&field.Spec{
			Length:      4,
			Description: "Local Transaction Date",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F14 – Expiration Date (4 digits, fixed — YYMM)
		14: field.NewString(&field.Spec{
			Length:      4,
			Description: "Expiration Date",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F22 – Point-Of-Service Entry Mode (3 digits, fixed)
		22: field.NewString(&field.Spec{
			Length:      3,
			Description: "Point-Of-Service Entry Mode",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F25 – Point-Of-Service Condition Code (2 digits, fixed)
		25: field.NewString(&field.Spec{
			Length:      2,
			Description: "Point-Of-Service Condition Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F32 – Acquiring Institution ID Code (LLVAR, up to 11 digits)
		32: field.NewString(&field.Spec{
			Length:      11,
			Description: "Acquiring Institution ID Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// F35 – Track 2 Data (LLVAR, up to 37 chars)
		35: field.NewString(&field.Spec{
			Length:      37,
			Description: "Track 2 Data",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LL,
		}),
		// F37 – Retrieval Reference Number (12 chars, fixed)
		37: field.NewString(&field.Spec{
			Length:      12,
			Description: "Retrieval Reference Number",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F38 – Authorization ID Response (6 chars, fixed)
		38: field.NewString(&field.Spec{
			Length:      6,
			Description: "Authorization ID Response",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F39 – Response Code (2 chars, fixed)
		39: field.NewString(&field.Spec{
			Length:      2,
			Description: "Response Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F41 – Card Acceptor Terminal ID (8 chars, fixed)
		41: field.NewString(&field.Spec{
			Length:      8,
			Description: "Card Acceptor Terminal ID",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F42 – Card Acceptor ID Code / Merchant ID (15 chars, fixed)
		42: field.NewString(&field.Spec{
			Length:      15,
			Description: "Card Acceptor ID Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F43 – Card Acceptor Name/Location (40 chars, fixed)
		43: field.NewString(&field.Spec{
			Length:      40,
			Description: "Card Acceptor Name/Location",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F49 – Currency Code, Transaction (3 digits, fixed)
		49: field.NewString(&field.Spec{
			Length:      3,
			Description: "Currency Code, Transaction",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		// F63 – Reserved Private (LLLVAR, up to 999 chars)
		63: field.NewString(&field.Spec{
			Length:      999,
			Description: "Reserved Private",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		// F70 – Network Management Information Code (3 digits, fixed)
		70: field.NewString(&field.Spec{
			Length:      3,
			Description: "Network Management Information Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
	},
}
