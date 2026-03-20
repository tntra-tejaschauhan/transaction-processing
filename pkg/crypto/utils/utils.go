// Package utils provides low-level byte, encoding, and cipher helpers shared
// across the crypto packages. All functions are safe for concurrent use.
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
)

// EncodeBase64 encodes data to a standard-encoding base64 string.
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes a standard-encoding base64 string to raw bytes.
func DecodeBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// RandBytes returns n cryptographically random bytes.
func RandBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, fmt.Errorf("rand bytes: %w", err)
	}
	return buf, nil
}

// DecodePEMBlock decodes the first PEM block from pemBytes.
// Returns an error when no valid PEM block is found, keeping callers
// free of the nil-check boilerplate repeated in every key parser.
func DecodePEMBlock(pemBytes []byte) (*pem.Block, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	return block, nil
}

// PKCS7Pad appends PKCS#7 padding to data so its length is a multiple of
// blockSize. blockSize must be between 1 and 255 inclusive.
func PKCS7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	if padLen == 0 {
		padLen = blockSize
	}
	padded := make([]byte, len(data)+padLen)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	return padded
}

// PKCS7Unpad removes and validates PKCS#7 padding from data.
func PKCS7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("pkcs7: invalid padding length")
	}
	padLen := int(data[len(data)-1])
	if padLen <= 0 || padLen > blockSize {
		return nil, errors.New("pkcs7: invalid padding value")
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, errors.New("pkcs7: invalid padding")
		}
	}
	return data[:len(data)-padLen], nil
}
