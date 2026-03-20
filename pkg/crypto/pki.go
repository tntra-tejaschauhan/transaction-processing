package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	utils "github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils"
)

const (
	// RSADefaultBits is the default key size for generated RSA keys.
	RSADefaultBits = 2048
)

// LoadCertificateFromPEM parses the first PEM block as an X.509 certificate.
func LoadCertificateFromPEM(pemBytes []byte) (*x509.Certificate, error) {
	block, err := utils.DecodePEMBlock(pemBytes)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return cert, nil
}

// ValidateCertificate checks NotBefore/NotAfter for the current time.
func ValidateCertificate(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not valid until %s", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired at %s", cert.NotAfter)
	}
	return nil
}

// GenerateRSAKeyPair returns PEM-encoded private and public keys. Uses 2048 bits if bits < 2048.
func GenerateRSAKeyPair(bits int) (privateKeyPEM []byte, publicKeyPEM []byte, err error) {
	if bits < 2048 {
		bits = RSADefaultBits
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("generate rsa key: %w", err)
	}
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privateBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER}
	privateKeyPEM = pem.EncodeToMemory(privateBlock)

	publicKey := &privateKey.PublicKey
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}
	publicKeyPEM = pem.EncodeToMemory(publicBlock)
	return privateKeyPEM, publicKeyPEM, nil
}
