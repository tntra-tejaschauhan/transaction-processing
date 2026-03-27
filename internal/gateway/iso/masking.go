package iso

import "strings"

// MaskPAN returns a PCI-compliant masked representation of a Primary Account
// Number (PAN) suitable for use in log entries and audit trails.
//
// Masking rules (PCI DSS §3.4):
//
//   - PAN ≥ 10 digits  → first 6 + "******" + last 4  (e.g. "411111******1111")
//   - PAN < 10 digits  → all digits replaced with "*"  (e.g. "12345" → "*****")
//   - Empty PAN        → "**"
//
// Usage in log statements:
//
//	logger.Debug().Str("pan", iso.MaskPAN(req.PAN)).Msg("auth request received")
func MaskPAN(pan string) string {
	if len(pan) == 0 {
		return "**"
	}
	if len(pan) < 10 {
		return strings.Repeat("*", len(pan))
	}
	return pan[:6] + "******" + pan[len(pan)-4:]
}
