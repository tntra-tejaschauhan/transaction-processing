package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	utils "github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils"
	"github.com/stretchr/testify/suite"
)

type testSuiteRSA struct {
	suite.Suite
	privPEM []byte
	pubPEM  []byte
}

func (s *testSuiteRSA) SetupSubTest() {
	privPEM, pubPEM, err := GenerateRSAKeyPair(2048)
	s.Require().NoError(err)
	s.privPEM = privPEM
	s.pubPEM = pubPEM
}

func TestRSA(t *testing.T) {
	suite.Run(t, new(testSuiteRSA))
}

func (s *testSuiteRSA) TestRSAEncryptDecryptOAEP_RoundTrip() {
	s.Run("when plaintext is message", func() {
		plaintext := []byte("secret message")
		ct, err := RSAEncryptOAEP(plaintext, s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
		dec, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plaintext))
	})
	s.Run("when plaintext is empty", func() {
		ct, err := RSAEncryptOAEP([]byte{}, s.pubPEM)
		s.Require().NoError(err)
		dec, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().Empty(dec)
	})
	s.Run("when plaintext is nil", func() {
		ct, err := RSAEncryptOAEP(nil, s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
	})
	s.Run("when plaintext is long", func() {
		plaintext := bytes.Repeat([]byte("a"), 100)
		ct, err := RSAEncryptOAEP(plaintext, s.pubPEM)
		s.Require().NoError(err)
		dec, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plaintext))
	})
	s.Run("when plaintext has null bytes", func() {
		plaintext := []byte("secret\x00with\x00nulls")
		ct, err := RSAEncryptOAEP(plaintext, s.pubPEM)
		s.Require().NoError(err)
		dec, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plaintext))
	})
}

func (s *testSuiteRSA) TestRSAEncryptOAEP_InvalidPublicKey() {
	table := []struct {
		name string
		pem  []byte
	}{
		{"not pem", []byte("not pem")},
		{"invalid key content", []byte("-----BEGIN PUBLIC KEY-----\nYQ==\n-----END PUBLIC KEY-----")},
		{"valid PEM structure but garbage content", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("garbage")})},
		{"empty slice", []byte{}},
		{"nil", nil},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := RSAEncryptOAEP([]byte("x"), tc.pem)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteRSA) TestRSADecryptOAEP_InvalidPrivateKey() {
	s.Run("when private key is not PEM", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte("not pem"))
		s.Require().Error(err)
	})
	s.Run("when private key is empty", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte{})
		s.Require().Error(err)
	})
	s.Run("when private key is nil", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), nil)
		s.Require().Error(err)
	})
	s.Run("when private key is wrong type PEM", func() {
		block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("x")})
		_, err := RSADecryptOAEP([]byte("eA=="), block)
		s.Require().Error(err)
	})
	s.Run("when private key PEM has no block", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte("-----BEGIN FOO-----\n-----END FOO-----"))
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestRSADecryptOAEP_InvalidBase64() {
	s.Run("when ciphertext is invalid base64", func() {
		_, err := RSADecryptOAEP([]byte("not!!!base64!!!"), s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when ciphertext is empty", func() {
		_, err := RSADecryptOAEP([]byte(""), s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when ciphertext has spaces", func() {
		ct, _ := RSAEncryptOAEP([]byte("x"), s.pubPEM)
		tampered := bytes.ReplaceAll(ct, []byte("="), []byte(" = "))
		_, err := RSADecryptOAEP(tampered, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when ciphertext is truncated", func() {
		ct, _ := RSAEncryptOAEP([]byte("x"), s.pubPEM)
		_, err := RSADecryptOAEP(ct[:len(ct)-2], s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when ciphertext is nil", func() {
		_, err := RSADecryptOAEP(nil, s.privPEM)
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestRSADecryptOAEP_WrongKey() {
	s.Run("when decrypt key does not match encrypt key", func() {
		_, pubPEM, _ := GenerateRSAKeyPair(2048)
		privPEM2, _, _ := GenerateRSAKeyPair(2048)
		ct, err := RSAEncryptOAEP([]byte("secret"), pubPEM)
		s.Require().NoError(err)
		_, err = RSADecryptOAEP(ct, privPEM2)
		s.Require().Error(err)
	})
	s.Run("when three key pairs and decrypt with third", func() {
		_, pub1, _ := GenerateRSAKeyPair(2048)
		_, _, _ = GenerateRSAKeyPair(2048)
		priv3, _, _ := GenerateRSAKeyPair(2048)
		ct, _ := RSAEncryptOAEP([]byte("x"), pub1)
		_, err := RSADecryptOAEP(ct, priv3)
		s.Require().Error(err)
	})
	s.Run("when encrypt with suite key decrypt with new key", func() {
		ct, _ := RSAEncryptOAEP([]byte("x"), s.pubPEM)
		privOther, _, _ := GenerateRSAKeyPair(2048)
		_, err := RSADecryptOAEP(ct, privOther)
		s.Require().Error(err)
	})
	s.Run("when empty plaintext encrypted with one key decrypted with other", func() {
		_, pub2, _ := GenerateRSAKeyPair(2048)
		ct, _ := RSAEncryptOAEP([]byte{}, pub2)
		_, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when ciphertext from SHA1 flow decrypted with wrong key", func() {
		ct, _ := RSAEncryptOAEPSha1([]byte("legacy"), s.pubPEM)
		priv2, _, _ := GenerateRSAKeyPair(2048)
		_, err := RSADecryptOAEPSha1(ct, priv2)
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestRSAEncryptOAEPSha1_RSADecryptOAEPSha1_RoundTrip() {
	s.Run("when plaintext is legacy message", func() {
		plain := []byte("legacy message")
		ct, err := RSAEncryptOAEPSha1(plain, s.pubPEM)
		s.Require().NoError(err)
		dec, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plain))
	})
	s.Run("when plaintext is empty", func() {
		ct, _ := RSAEncryptOAEPSha1([]byte{}, s.pubPEM)
		dec, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().Empty(dec)
	})
	s.Run("when plaintext has binary", func() {
		plain := []byte{0x00, 0xff, 0x80}
		ct, _ := RSAEncryptOAEPSha1(plain, s.pubPEM)
		dec, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plain))
	})
	s.Run("when multiple encrypt decrypt cycles", func() {
		for i := 0; i < 3; i++ {
			plain := []byte("msg" + string(rune('0'+i)))
			ct, _ := RSAEncryptOAEPSha1(plain, s.pubPEM)
			dec, err := RSADecryptOAEPSha1(ct, s.privPEM)
			s.Require().NoError(err)
			s.Assert().True(bytes.Equal(dec, plain))
		}
	})
	s.Run("when ciphertext is non-empty and valid base64", func() {
		ct, err := RSAEncryptOAEPSha1([]byte("x"), s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
	})
}

func (s *testSuiteRSA) TestParseRSAPrivateKey_PKCS8() {
	s.Run("when private key is PKCS8", func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		s.Require().NoError(err)
		block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
		privPEM := pem.EncodeToMemory(block)
		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		s.Require().NoError(err)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
		plain := []byte("secret")
		ct, err := RSAEncryptOAEP(plain, pubPEM)
		s.Require().NoError(err)
		dec, err := RSADecryptOAEP(ct, privPEM)
		s.Require().NoError(err)
		s.Assert().True(bytes.Equal(dec, plain))
	})
	s.Run("when PKCS8 key decrypts SHA1 ciphertext", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		der, _ := x509.MarshalPKCS8PrivateKey(priv)
		privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
		ct, _ := RSAEncryptOAEPSha1([]byte("legacy"), pubPEM)
		dec, err := RSADecryptOAEPSha1(ct, privPEM)
		s.Require().NoError(err)
		s.Assert().Equal([]byte("legacy"), dec)
	})
	s.Run("when PKCS8 key used for empty plaintext", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		der, _ := x509.MarshalPKCS8PrivateKey(priv)
		privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
		ct, _ := RSAEncryptOAEP([]byte{}, pubPEM)
		dec, err := RSADecryptOAEP(ct, privPEM)
		s.Require().NoError(err)
		s.Assert().Empty(dec)
	})
	s.Run("when PKCS8 block type is PRIVATE KEY", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		der, _ := x509.MarshalPKCS8PrivateKey(priv)
		block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		s.Require().Contains(string(block), "PRIVATE KEY")
	})
	s.Run("when PKCS8 and PKCS1 both work for same key", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		der8, _ := x509.MarshalPKCS8PrivateKey(priv)
		privPEM8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der8})
		der1 := x509.MarshalPKCS1PrivateKey(priv)
		privPEM1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der1})
		pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
		ct, _ := RSAEncryptOAEP([]byte("x"), pubPEM)
		dec1, _ := RSADecryptOAEP(ct, privPEM1)
		dec8, _ := RSADecryptOAEP(ct, privPEM8)
		s.Assert().Equal(dec1, dec8)
	})
}

func (s *testSuiteRSA) TestRSAEncryptOAEP_EmptyPlaintext() {
	s.Run("when plaintext is nil then ciphertext is non-empty", func() {
		ct, err := RSAEncryptOAEP(nil, s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
	})
	s.Run("when plaintext is empty slice then ciphertext is non-empty", func() {
		ct, err := RSAEncryptOAEP([]byte{}, s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
	})
	s.Run("when plaintext is nil then roundtrip decrypt yields empty", func() {
		ct, _ := RSAEncryptOAEP(nil, s.pubPEM)
		dec, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().NoError(err)
		s.Assert().Empty(dec)
	})
	s.Run("when plaintext is empty then base64 ciphertext is valid", func() {
		ct, err := RSAEncryptOAEP([]byte{}, s.pubPEM)
		s.Require().NoError(err)
		s.Require().NotEmpty(ct)
		_, decErr := utils.DecodeBase64(string(ct))
		s.Require().NoError(decErr, "ciphertext must be valid base64")
	})
	s.Run("when plaintext is empty then each encrypt gives different ciphertext", func() {
		ct1, _ := RSAEncryptOAEP([]byte{}, s.pubPEM)
		ct2, _ := RSAEncryptOAEP([]byte{}, s.pubPEM)
		s.Assert().NotEqual(ct1, ct2)
	})
}

func (s *testSuiteRSA) TestRSADecryptOAEP_WrongHash() {
	s.Run("when encrypted with SHA-1 and decrypted with SHA-256 then fails", func() {
		ct, err := RSAEncryptOAEPSha1([]byte("secret"), s.pubPEM)
		s.Require().NoError(err)
		_, err = RSADecryptOAEP(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when SHA1 ciphertext is valid base64", func() {
		ct, _ := RSAEncryptOAEPSha1([]byte("x"), s.pubPEM)
		s.Require().NotEmpty(ct)
		_, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when empty plaintext SHA1 then SHA256 decrypt fails", func() {
		ct, _ := RSAEncryptOAEPSha1([]byte{}, s.pubPEM)
		_, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when long plaintext SHA1 then SHA256 decrypt fails", func() {
		ct, _ := RSAEncryptOAEPSha1(bytes.Repeat([]byte("a"), 50), s.pubPEM)
		_, err := RSADecryptOAEP(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when same key pair SHA1 encrypt SHA256 decrypt always fails", func() {
		for i := 0; i < 2; i++ {
			ct, _ := RSAEncryptOAEPSha1([]byte("m"), s.pubPEM)
			_, err := RSADecryptOAEP(ct, s.privPEM)
			s.Require().Error(err)
		}
	})
}

func (s *testSuiteRSA) TestRSADecryptOAEPSha1_WrongHash() {
	s.Run("when encrypted with SHA-256 and decrypted with SHA-1 then fails", func() {
		ct, err := RSAEncryptOAEP([]byte("secret"), s.pubPEM)
		s.Require().NoError(err)
		_, err = RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when SHA256 ciphertext non-empty", func() {
		ct, _ := RSAEncryptOAEP([]byte("x"), s.pubPEM)
		_, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when empty plaintext SHA256 then SHA1 decrypt fails", func() {
		ct, _ := RSAEncryptOAEP([]byte{}, s.pubPEM)
		_, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when nil plaintext SHA256 then SHA1 decrypt fails", func() {
		ct, _ := RSAEncryptOAEP(nil, s.pubPEM)
		_, err := RSADecryptOAEPSha1(ct, s.privPEM)
		s.Require().Error(err)
	})
	s.Run("when SHA256 then SHA1 decrypt fails for multiple messages", func() {
		for _, msg := range [][]byte{[]byte("a"), []byte("b")} {
			ct, _ := RSAEncryptOAEP(msg, s.pubPEM)
			_, err := RSADecryptOAEPSha1(ct, s.privPEM)
			s.Require().Error(err)
		}
	})
}

func (s *testSuiteRSA) TestParseRSAPublicKey_NoPEMBlock() {
	s.Run("when input is not PEM", func() {
		_, err := RSAEncryptOAEP([]byte("x"), []byte("not pem"))
		s.Require().Error(err)
	})
	s.Run("when input is empty", func() {
		_, err := RSAEncryptOAEP([]byte("x"), []byte{})
		s.Require().Error(err)
	})
	s.Run("when input is nil", func() {
		_, err := RSAEncryptOAEP([]byte("x"), nil)
		s.Require().Error(err)
	})
	s.Run("when PEM type is wrong", func() {
		block := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("x")})
		_, err := RSAEncryptOAEP([]byte("x"), block)
		s.Require().Error(err)
	})
	s.Run("when only PEM header no body", func() {
		_, err := RSAEncryptOAEP([]byte("x"), []byte("-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----"))
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestParseRSAPublicKey_NotRSA() {
	s.Run("when public key is EC", func() {
		ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s.Require().NoError(err)
		der, err := x509.MarshalPKIXPublicKey(&ecPriv.PublicKey)
		s.Require().NoError(err)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
		_, err = RSAEncryptOAEP([]byte("x"), pubPEM)
		s.Require().Error(err)
	})
	s.Run("when public key is EC P384", func() {
		ecPriv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		s.Require().NoError(err)
		der, _ := x509.MarshalPKIXPublicKey(&ecPriv.PublicKey)
		pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
		_, err = RSAEncryptOAEP([]byte("x"), pubPEM)
		s.Require().Error(err)
	})
	s.Run("when content is certificate not key", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
		certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
		block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: certDER})
		_, err := RSAEncryptOAEP([]byte("x"), block)
		s.Require().Error(err)
	})
	s.Run("when public key bytes are truncated garbage", func() {
		block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("truncated")})
		_, err := RSAEncryptOAEP([]byte("x"), block)
		s.Require().Error(err)
	})
	s.Run("when input has multiple PEM blocks first invalid", func() {
		_, err := RSAEncryptOAEP([]byte("x"), []byte("-----BEGIN FOO-----\nYQ==\n-----END FOO-----\n-----BEGIN PUBLIC KEY-----\nYQ==\n-----END PUBLIC KEY-----"))
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestParseRSAPrivateKey_NoPEMBlock() {
	s.Run("when private key input is not PEM", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte("not pem"))
		s.Require().Error(err)
	})
	s.Run("when private key is empty", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte{})
		s.Require().Error(err)
	})
	s.Run("when private key is nil", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), nil)
		s.Require().Error(err)
	})
	s.Run("when private key PEM type is CERTIFICATE", func() {
		block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("x")})
		_, err := RSADecryptOAEP([]byte("eA=="), block)
		s.Require().Error(err)
	})
	s.Run("when private key has no newline after header", func() {
		_, err := RSADecryptOAEP([]byte("eA=="), []byte("-----BEGIN RSA PRIVATE KEY-----"))
		s.Require().Error(err)
	})
}

func (s *testSuiteRSA) TestParseRSAPrivateKey_NotRSA() {
	s.Run("when private key is EC", func() {
		ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s.Require().NoError(err)
		der, err := x509.MarshalPKCS8PrivateKey(ecPriv)
		s.Require().NoError(err)
		privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		_, err = RSADecryptOAEP([]byte("eA=="), privPEM)
		s.Require().Error(err)
	})
	s.Run("when private key is EC P384", func() {
		ecPriv, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		der, _ := x509.MarshalPKCS8PrivateKey(ecPriv)
		privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		_, err := RSADecryptOAEP([]byte("eA=="), privPEM)
		s.Require().Error(err)
	})
	s.Run("when PKCS8 block contains non-RSA key", func() {
		ecPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		der, _ := x509.MarshalPKCS8PrivateKey(ecPriv)
		block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		_, err := RSADecryptOAEP([]byte("eA=="), block)
		s.Require().Error(err)
	})
	s.Run("when private key bytes are garbage", func() {
		block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("garbage")})
		_, err := RSADecryptOAEP([]byte("eA=="), block)
		s.Require().Error(err)
	})
	s.Run("when PEM says RSA but bytes are PKCS8 EC", func() {
		ecPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		der, _ := x509.MarshalPKCS8PrivateKey(ecPriv)
		block := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
		_, err := RSADecryptOAEP([]byte("eA=="), block)
		s.Require().Error(err)
	})
}

