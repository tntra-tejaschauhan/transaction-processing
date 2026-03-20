package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Salted returns SHA256(salt||data) as hex. Interoperable with BIM-API BimHash.
func SHA256Salted(salt, data string) string {
	hasher := sha256.New()
	hasher.Write([]byte(salt))
	hasher.Write([]byte(data))
	var sum [sha256.Size]byte
	return hex.EncodeToString(hasher.Sum(sum[:0]))
}

// SHA256Hex returns SHA256(data) as hex.
func SHA256Hex(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
