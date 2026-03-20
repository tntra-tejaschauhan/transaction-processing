package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"

	utils "github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils"
)

func rsaEncryptOAEPWithHash(plaintext []byte, publicKeyPEM []byte, h hash.Hash) ([]byte, error) {
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("rsa encrypt: %w", err)
	}
	ciphertext, err := rsa.EncryptOAEP(h, rand.Reader, publicKey, plaintext, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa encrypt: %w", err)
	}
	return []byte(utils.EncodeBase64(ciphertext)), nil
}

func rsaDecryptOAEPWithHash(ciphertextBase64 []byte, privateKeyPEM []byte, h hash.Hash) ([]byte, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	ciphertext, err := utils.DecodeBase64(string(ciphertextBase64))
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: base64: %w", err)
	}
	plaintext, err := rsa.DecryptOAEP(h, rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	return plaintext, nil
}

// RSAEncryptOAEP encrypts with RSA public key (PEM). OAEP SHA-256; result base64. Matches BimPKI / modern legacy.
func RSAEncryptOAEP(plaintext []byte, publicKeyPEM []byte) ([]byte, error) {
	return rsaEncryptOAEPWithHash(plaintext, publicKeyPEM, sha256.New())
}

// RSADecryptOAEP decrypts base64 ciphertext with RSA private key (PEM). OAEP SHA-256.
func RSADecryptOAEP(ciphertextBase64 []byte, privateKeyPEM []byte) ([]byte, error) {
	return rsaDecryptOAEPWithHash(ciphertextBase64, privateKeyPEM, sha256.New())
}

// RSAEncryptOAEPSha1 encrypts with OAEP SHA-1. Legacy interop (e.g. BuyPass ConnectCode).
func RSAEncryptOAEPSha1(plaintext []byte, publicKeyPEM []byte) ([]byte, error) {
	return rsaEncryptOAEPWithHash(plaintext, publicKeyPEM, sha1.New())
}

// RSADecryptOAEPSha1 decrypts base64 ciphertext with OAEP SHA-1. Legacy interop.
func RSADecryptOAEPSha1(ciphertextBase64 []byte, privateKeyPEM []byte) ([]byte, error) {
	return rsaDecryptOAEPWithHash(ciphertextBase64, privateKeyPEM, sha1.New())
}

// parseRSAPublicKey decodes the first PEM block as an RSA public key.
func parseRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, err := utils.DecodePEMBlock(pemBytes)
	if err != nil {
		return nil, err
	}
	parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPublicKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPublicKey, nil
}

// parseRSAPrivateKey decodes the first PEM block as an RSA private key (PKCS1 or PKCS8).
func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, err := utils.DecodePEMBlock(pemBytes)
	if err != nil {
		return nil, err
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsedKey, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, err
		}
		var ok bool
		privateKey, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
	}
	return privateKey, nil
}
