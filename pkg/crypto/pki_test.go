package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type testSuitePKI struct {
	suite.Suite
}

func TestPKI(t *testing.T) {
	suite.Run(t, new(testSuitePKI))
}

func (s *testSuitePKI) TestLoadCertificateFromPEM() {
	s.Run("when PEM is valid certificate then returns cert", func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
		}
		certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		s.Require().NoError(err)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		cert, err := LoadCertificateFromPEM(certPEM)
		s.Require().NoError(err)
		s.Require().NotNil(cert)
	})
	s.Run("when cert loaded then has serial number", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := x509.Certificate{SerialNumber: big.NewInt(42), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true}
		certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		cert, err := LoadCertificateFromPEM(certPEM)
		s.Require().NoError(err)
		s.Assert().Equal(big.NewInt(42).String(), cert.SerialNumber.String())
	})
	s.Run("when multiple certs loaded then each valid", func() {
		for i := 0; i < 3; i++ {
			priv, _ := rsa.GenerateKey(rand.Reader, 2048)
			tpl := x509.Certificate{SerialNumber: big.NewInt(int64(i)), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true}
			certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
			certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
			cert, err := LoadCertificateFromPEM(certPEM)
			s.Require().NoError(err)
			s.Require().NotNil(cert)
		}
	})
	s.Run("when cert has key usage then preserved", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageKeyEncipherment, BasicConstraintsValid: true}
		certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		cert, err := LoadCertificateFromPEM(certPEM)
		s.Require().NoError(err)
		s.Assert().True(cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
	})
	s.Run("when cert valid then ValidateCertificate passes", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), BasicConstraintsValid: true}
		certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		cert, _ := LoadCertificateFromPEM(certPEM)
		err := ValidateCertificate(cert)
		s.Require().NoError(err)
	})
}

func (s *testSuitePKI) TestLoadCertificateFromPEM_Invalid() {
	table := []struct {
		name     string
		pemBytes []byte
	}{
		{"not pem", []byte("not pem")},
		{"nil", nil},
		{"empty", []byte{}},
		{"invalid cert content", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-der")})},
		{"wrong PEM type", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("x")})},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := LoadCertificateFromPEM(tc.pemBytes)
			s.Require().Error(err)
		})
	}
}

func (s *testSuitePKI) TestValidateCertificate() {
	s.Run("when cert is valid then no error", func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment,
			BasicConstraintsValid: true,
		}
		certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		s.Require().NoError(err)
		cert, err := x509.ParseCertificate(certDER)
		s.Require().NoError(err)
		err = ValidateCertificate(cert)
		s.Require().NoError(err)
	})
	s.Run("when cert is nil then error", func() {
		err := ValidateCertificate(nil)
		s.Require().Error(err)
	})
	s.Run("when cert is expired then error", func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now().Add(-48 * time.Hour),
			NotAfter:              time.Now().Add(-24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment,
			BasicConstraintsValid: true,
		}
		certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		s.Require().NoError(err)
		cert, err := x509.ParseCertificate(certDER)
		s.Require().NoError(err)
		err = ValidateCertificate(cert)
		s.Require().Error(err)
	})
	s.Run("when cert is not yet valid then error", func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now().Add(2 * time.Hour),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment,
			BasicConstraintsValid: true,
		}
		certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		s.Require().NoError(err)
		cert, err := x509.ParseCertificate(certDER)
		s.Require().NoError(err)
		err = ValidateCertificate(cert)
		s.Require().Error(err)
	})
	s.Run("when cert valid just now at NotBefore then no error", func() {
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		now := time.Now()
		tpl := x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now, NotAfter: now.Add(24 * time.Hour), BasicConstraintsValid: true}
		certDER, _ := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
		cert, _ := x509.ParseCertificate(certDER)
		err := ValidateCertificate(cert)
		s.Require().NoError(err)
	})
}

func (s *testSuitePKI) TestGenerateRSAKeyPair() {
	s.Run("when bits is 2048 then returns valid PEMs", func() {
		privPEM, pubPEM, err := GenerateRSAKeyPair(2048)
		s.Require().NoError(err)
		s.Require().NotEmpty(privPEM)
		s.Require().NotEmpty(pubPEM)
		block, _ := pem.Decode(privPEM)
		s.Require().NotNil(block)
		s.Assert().Equal("RSA PRIVATE KEY", block.Type)
		pubBlock, _ := pem.Decode(pubPEM)
		s.Require().NotNil(pubBlock)
		s.Assert().Equal("PUBLIC KEY", pubBlock.Type)
	})
	s.Run("when bits is 512 then fallback to 2048", func() {
		_, _, err := GenerateRSAKeyPair(512)
		s.Require().NoError(err)
	})
	s.Run("when bits is 4096 then success", func() {
		priv, pub, err := GenerateRSAKeyPair(4096)
		s.Require().NoError(err)
		s.Require().NotEmpty(priv)
		s.Require().NotEmpty(pub)
	})
	s.Run("when generated twice then different keys", func() {
		priv1, pub1, _ := GenerateRSAKeyPair(2048)
		priv2, pub2, _ := GenerateRSAKeyPair(2048)
		s.Assert().NotEqual(priv1, priv2)
		s.Assert().NotEqual(pub1, pub2)
	})
	s.Run("when private PEM decoded then valid PKCS1", func() {
		privPEM, _, err := GenerateRSAKeyPair(2048)
		s.Require().NoError(err)
		block, _ := pem.Decode(privPEM)
		_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		s.Require().NoError(err)
	})
}

func (s *testSuitePKI) TestGenerateRSAKeyPair_BitsFallback() {
	table := []struct {
		name string
		bits int
	}{
		{"zero", 0},
		{"negative", -1},
		{"one", 1},
		{"256", 256},
		{"1024", 1024},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			privPEM, _, err := GenerateRSAKeyPair(tc.bits)
			s.Require().NoError(err)
			block, _ := pem.Decode(privPEM)
			s.Require().NotNil(block)
			priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			s.Require().NoError(err)
			s.Assert().GreaterOrEqual(priv.N.BitLen(), 2048)
		})
	}
}

