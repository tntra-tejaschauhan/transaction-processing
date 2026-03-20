package crypto

import (
	"bytes"
	"testing"

	"github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils/testutil"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/suite"
)

type testSuitePGP struct {
	suite.Suite
	pub  string
	priv string
}

func (s *testSuitePGP) SetupSubTest() {
	pub, priv := s.mustKeyPair()
	s.pub = pub
	s.priv = priv
}

func (s *testSuitePGP) mustKeyPair() (pubArmored, privArmored string) {
	entity, err := openpgp.NewEntity("Test", "", "test@example.com", testutil.PGPConfig)
	s.Require().NoError(err)
	var pubBuf bytes.Buffer
	pubW, err := armor.Encode(&pubBuf, openpgp.PublicKeyType, nil)
	s.Require().NoError(err)
	s.Require().NoError(entity.Serialize(pubW))
	s.Require().NoError(pubW.Close())
	var privBuf bytes.Buffer
	privW, err := armor.Encode(&privBuf, openpgp.PrivateKeyType, nil)
	s.Require().NoError(err)
	s.Require().NoError(entity.SerializePrivate(privW, testutil.PGPConfig))
	s.Require().NoError(privW.Close())
	return pubBuf.String(), privBuf.String()
}

func TestPGP(t *testing.T) {
	suite.Run(t, new(testSuitePGP))
}

func (s *testSuitePGP) TestPGPEncryptDecrypt_RoundTrip() {
	s.Run("when plaintext is secret message", func() {
		plaintext := []byte("secret message for PGP")
		enc, err := PGPEncrypt(plaintext, s.pub)
		s.Require().NoError(err)
		s.Require().NotEqual(plaintext, enc)
		s.Assert().Contains(enc, "BEGIN PGP MESSAGE")
		dec, err := PGPDecrypt(enc, s.priv, nil)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is empty", func() {
		enc, err := PGPEncrypt(nil, s.pub)
		s.Require().NoError(err)
		s.Require().NotEmpty(enc)
		dec, err := PGPDecrypt(enc, s.priv, nil)
		s.Require().NoError(err)
		s.Assert().Empty(dec)
	})
	s.Run("when plaintext is long and unicode", func() {
		plaintext := []byte("payment_ref_123 café naïve 日本")
		enc, err := PGPEncrypt(plaintext, s.pub)
		s.Require().NoError(err)
		dec, err := PGPDecrypt(enc, s.priv, nil)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when different key then decrypt fails or wrong plaintext", func() {
		_, otherPriv := s.mustKeyPair()
		plaintext := []byte("secret")
		enc, err := PGPEncrypt(plaintext, s.pub)
		s.Require().NoError(err)
		dec, err := PGPDecrypt(enc, otherPriv, nil)
		s.Assert().True(err != nil || !bytes.Equal(dec, plaintext))
	})
}

func (s *testSuitePGP) TestPGPEncrypt_InvalidKey() {
	table := []struct {
		name string
		key  string
	}{
		{"empty key", ""},
		{"not valid armor", "not valid armor"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := PGPEncrypt([]byte("data"), tc.key)
			s.Require().Error(err)
		})
	}
	s.Run("when key is empty then error mentions recipient", func() {
		_, err := PGPEncrypt([]byte("data"), "")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "recipient public key is empty")
	})
}

func (s *testSuitePGP) TestPGPDecrypt_InvalidInput() {
	table := []struct {
		name           string
		ciphertext     string
		privateKey     string
		wantErrContain string
	}{
		{"empty ciphertext", "", s.priv, "ciphertext is empty"},
		{"empty private key", "-----BEGIN PGP MESSAGE-----\n\n-----END PGP MESSAGE-----", "", "private key is empty"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := PGPDecrypt(tc.ciphertext, tc.privateKey, nil)
			s.Require().Error(err)
			if tc.wantErrContain != "" {
				s.Assert().Contains(err.Error(), tc.wantErrContain)
			}
		})
	}
}

func (s *testSuitePGP) TestPGPSignVerify_RoundTrip() {
	s.Run("when data is signed then verify succeeds", func() {
		plaintext := []byte("data to sign")
		sig, err := PGPSign(plaintext, s.priv, nil)
		s.Require().NoError(err)
		s.Require().NotEmpty(sig)
		s.Assert().Contains(sig, "BEGIN PGP SIGNATURE")
		err = PGPVerify(plaintext, sig, s.pub)
		s.Require().NoError(err)
	})
	s.Run("when data tampered then verify fails", func() {
		plaintext := []byte("original")
		sig, err := PGPSign(plaintext, s.priv, nil)
		s.Require().NoError(err)
		err = PGPVerify([]byte("tampered"), sig, s.pub)
		s.Require().Error(err)
	})
}

func (s *testSuitePGP) TestPGPSign_InvalidKey() {
	s.Run("when private key is empty then error", func() {
		_, err := PGPSign([]byte("data"), "", nil)
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "private key is empty")
	})
}

func (s *testSuitePGP) TestPGPVerify_InvalidInput() {
	table := []struct {
		name           string
		signature      string
		publicKey      string
		wantErrContain string
	}{
		{"empty signature", "", s.pub, "signature is empty"},
		{"empty public key", "-----BEGIN PGP SIGNATURE-----\n\n-----END PGP SIGNATURE-----", "", "signer public key is empty"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			err := PGPVerify([]byte("data"), tc.signature, tc.publicKey)
			s.Require().Error(err)
			if tc.wantErrContain != "" {
				s.Assert().Contains(err.Error(), tc.wantErrContain)
			}
		})
	}
}
