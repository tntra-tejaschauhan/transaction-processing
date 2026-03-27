package iso_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestMaskPAN verifies that MaskPAN produces PCI-compliant masked output for
// all required input categories: standard 16-digit PAN, long 19-digit PAN,
// short PAN (< 10 chars), and empty string.
func TestMaskPAN(t *testing.T) {
	tests := []struct {
		name string
		pan  string
		want string
	}{
		{
			name: "16-digit PAN — standard Visa/Discover card",
			pan:  "4111111111111111",
			want: "411111******1111",
		},
		{
			name: "19-digit PAN — long-PAN card scheme",
			pan:  "4111222233334444555",
			want: "411122******4555",
		},
		{
			name: "short PAN (<10 chars) — all digits masked",
			pan:  "12345",
			want: "*****",
		},
		{
			name: "exactly 9 chars — boundary of short PAN rule",
			pan:  "123456789",
			want: "*********",
		},
		{
			name: "empty string — returns **",
			pan:  "",
			want: "**",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := iso.MaskPAN(tc.pan)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestMaskPAN_DoesNotLeakPAN is the explicit PCI compliance assertion:
// the masked output must never contain the original PAN substring.
func TestMaskPAN_DoesNotLeakPAN(t *testing.T) {
	pan := "4111111111111111"
	masked := iso.MaskPAN(pan)
	assert.NotContains(t, masked, pan,
		"masked PAN must not contain the original PAN string (PCI requirement)")
}
