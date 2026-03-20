package testutil

import (
	"crypto"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// TestKeyB64 is a fixed 32-byte key (base64) for deterministic legacy AES tests.
const TestKeyB64 = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="

// PGPConfig is the packet.Config used in PGP tests (SHA256 so no RIPEMD160 dependency).
var PGPConfig = &packet.Config{DefaultHash: crypto.SHA256}
