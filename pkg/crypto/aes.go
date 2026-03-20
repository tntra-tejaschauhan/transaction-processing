package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"strings"

	utils "github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils"
)

const (
	AESKeySizeBytes   = 32
	AESBlockSizeBytes = 16
	KeyIndexLen       = 4
	IVLenFieldLen     = 4
)

var zeroIV = [AESBlockSizeBytes]byte{}

// AESEncrypt encrypts plaintext with AES-256-CBC. keyBase64 must decode to 32 bytes; keyIndex 0–9999.
// Payload format: keyIndex(4) + ivLen(4) + ivBase64 + ciphertextBase64.
func AESEncrypt(plaintext string, keyBase64 string, keyIndex int) (string, error) {
	key, err := decodeKey(keyBase64)
	if err != nil {
		return "", fmt.Errorf("aes encrypt: %w", err)
	}
	iv, err := utils.RandBytes(AESBlockSizeBytes)
	if err != nil {
		return "", fmt.Errorf("aes encrypt: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes encrypt: %w", err)
	}
	padded := pkcs7Pad([]byte(plaintext), AESBlockSizeBytes)
	ciphertext := make([]byte, len(iv)+len(padded))
	copy(ciphertext, iv)
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[AESBlockSizeBytes:], padded)
	ivB64 := utils.EncodeBase64(iv)
	ciphertextB64 := utils.EncodeBase64(ciphertext[AESBlockSizeBytes:])
	var builder strings.Builder
	builder.Grow(KeyIndexLen + IVLenFieldLen + len(ivB64) + len(ciphertextB64))
	builder.WriteString(formatKeyIndex(keyIndex))
	builder.WriteString(fmt.Sprintf("%04d", len(ivB64)))
	builder.WriteString(ivB64)
	builder.WriteString(ciphertextB64)
	return builder.String(), nil
}

// AESEncryptLegacy encrypts with AES-256-CBC using a fixed zero IV.
// Output is base64(ciphertext) only. Matches Legacy SymmetricEncryptionService (BuyPass) for side-by-side interop.
func AESEncryptLegacy(plaintext string, keyBase64 string) (string, error) {
	key, err := decodeKey(keyBase64)
	if err != nil {
		return "", fmt.Errorf("aes encrypt legacy: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes encrypt legacy: %w", err)
	}
	padded := pkcs7Pad([]byte(plaintext), AESBlockSizeBytes)
	mode := cipher.NewCBCEncrypter(block, zeroIV[:])
	mode.CryptBlocks(padded, padded)
	return utils.EncodeBase64(padded), nil
}

// AESDecryptLegacy decrypts base64(ciphertext) produced by AESEncryptLegacy with static zero IV.
func AESDecryptLegacy(ciphertextBase64 string, keyBase64 string) (string, error) {
	key, err := decodeKey(keyBase64)
	if err != nil {
		return "", fmt.Errorf("aes decrypt legacy: %w", err)
	}
	ciphertext, err := utils.DecodeBase64(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("aes decrypt legacy: base64: %w", err)
	}
	if len(ciphertext)%AESBlockSizeBytes != 0 {
		return "", errors.New("aes decrypt legacy: ciphertext length not multiple of block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes decrypt legacy: %w", err)
	}
	mode := cipher.NewCBCDecrypter(block, zeroIV[:])
	mode.CryptBlocks(ciphertext, ciphertext)
	unpadded, err := pkcs7Unpad(ciphertext, AESBlockSizeBytes)
	if err != nil {
		return "", fmt.Errorf("aes decrypt legacy: %w", err)
	}
	return string(unpadded), nil
}

// AESDecrypt decrypts a payload from AESEncrypt. keyBase64 must decode to 32 bytes.
func AESDecrypt(payload string, keyBase64 string) (string, error) {
	minLen := KeyIndexLen + IVLenFieldLen
	if len(payload) < minLen {
		return "", errors.New("aes decrypt: payload too short")
	}
	key, err := decodeKey(keyBase64)
	if err != nil {
		return "", fmt.Errorf("aes decrypt: %w", err)
	}
	ivLenStr := payload[KeyIndexLen : KeyIndexLen+IVLenFieldLen]
	var ivLen int
	if _, err := fmt.Sscanf(ivLenStr, "%d", &ivLen); err != nil || ivLen <= 0 {
		return "", errors.New("aes decrypt: invalid iv length field")
	}
	headerLen := KeyIndexLen + IVLenFieldLen + ivLen
	if len(payload) < headerLen {
		return "", errors.New("aes decrypt: payload shorter than iv")
	}
	ivB64 := payload[KeyIndexLen+IVLenFieldLen : headerLen]
	ciphertextB64 := payload[headerLen:]
	iv, err := utils.DecodeBase64(ivB64)
	if err != nil {
		return "", fmt.Errorf("aes decrypt: iv decode: %w", err)
	}
	ciphertext, err := utils.DecodeBase64(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("aes decrypt: ciphertext decode: %w", err)
	}
	if len(iv) != AESBlockSizeBytes {
		return "", fmt.Errorf("aes decrypt: iv length %d, want %d", len(iv), AESBlockSizeBytes)
	}
	if len(ciphertext)%AESBlockSizeBytes != 0 {
		return "", errors.New("aes decrypt: ciphertext length not multiple of block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes decrypt: %w", err)
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	unpadded, err := pkcs7Unpad(ciphertext, AESBlockSizeBytes)
	if err != nil {
		return "", fmt.Errorf("aes decrypt: %w", err)
	}
	return string(unpadded), nil
}

// GenerateAES256Key returns a new 32-byte key encoded as base64.
func GenerateAES256Key() (string, error) {
	key, err := utils.RandBytes(AESKeySizeBytes)
	if err != nil {
		return "", fmt.Errorf("generate aes key: %w", err)
	}
	return utils.EncodeBase64(key), nil
}

// GenerateIV returns a new 16-byte IV encoded as base64.
func GenerateIV() (string, error) {
	iv, err := utils.RandBytes(AESBlockSizeBytes)
	if err != nil {
		return "", fmt.Errorf("generate iv: %w", err)
	}
	return utils.EncodeBase64(iv), nil
}

func decodeKey(keyBase64 string) ([]byte, error) {
	if keyBase64 == "" {
		return nil, errors.New("key is empty")
	}
	key, err := utils.DecodeBase64(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("key base64: %w", err)
	}
	if len(key) != AESKeySizeBytes {
		return nil, fmt.Errorf("key length %d, want %d", len(key), AESKeySizeBytes)
	}
	return key, nil
}

func formatKeyIndex(idx int) string {
	if idx < 0 {
		idx = 0
	}
	if idx > 9999 {
		idx = 9999
	}
	return fmt.Sprintf("%04d", idx)
}

// pkcs7Pad and pkcs7Unpad delegate to utils; unexported names preserved for same-package tests.
func pkcs7Pad(data []byte, blockSize int) []byte {
	return utils.PKCS7Pad(data, blockSize)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	return utils.PKCS7Unpad(data, blockSize)
}
