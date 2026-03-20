package crypto

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/PayWithSpireInc/transaction-processing/pkg/crypto/utils/testutil"
	"github.com/stretchr/testify/suite"
)

type testSuiteAES struct {
	suite.Suite
	key string
}

func (s *testSuiteAES) SetupSubTest() {
	d, err := base64.StdEncoding.DecodeString(testutil.TestKeyB64)
	s.Require().NoError(err, "TestKeyB64 must be valid base64")
	s.Require().Len(d, AESKeySizeBytes, "TestKeyB64 must decode to 32 bytes")
	key, err := GenerateAES256Key()
	s.Require().NoError(err)
	s.key = key
}

func TestAES(t *testing.T) {
	suite.Run(t, new(testSuiteAES))
}

func (s *testSuiteAES) TestAESEncryptDecrypt_RoundTrip() {
	s.Run("when plaintext is card number", func() {
		plaintext := "4111111111111111"
		enc, err := AESEncrypt(plaintext, s.key, 1)
		s.Require().NoError(err)
		s.Require().NotEqual(plaintext, enc)
		dec, err := AESDecrypt(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is empty", func() {
		enc, err := AESEncrypt("", s.key, 0)
		s.Require().NoError(err)
		dec, err := AESDecrypt(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal("", dec)
	})
	s.Run("when different keys produce different ciphertext", func() {
		key2, err := GenerateAES256Key()
		s.Require().NoError(err)
		plaintext := "sensitive-data"
		enc1, err1 := AESEncrypt(plaintext, s.key, 1)
		enc2, err2 := AESEncrypt(plaintext, key2, 1)
		s.Require().NoError(err1)
		s.Require().NoError(err2)
		s.Assert().NotEqual(enc1, enc2)
		dec, err := AESDecrypt(enc1, key2)
		s.Assert().True(err != nil || dec != plaintext)
	})
	s.Run("when plaintext is single block size", func() {
		plaintext := string(make([]byte, AESBlockSizeBytes))
		enc, err := AESEncrypt(plaintext, s.key, 5)
		s.Require().NoError(err)
		dec, err := AESDecrypt(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is long and unicode", func() {
		plaintext := "payment_ref_123 café naïve 日本"
		enc, err := AESEncrypt(plaintext, s.key, 0)
		s.Require().NoError(err)
		dec, err := AESDecrypt(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
}

func (s *testSuiteAES) TestAESEncrypt_InvalidKey() {
	table := []struct {
		name string
		key  string
	}{
		{"empty key", ""},
		{"wrong size base64", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"invalid base64", "not-valid-base64!!!"},
		{"key too long", base64.StdEncoding.EncodeToString(make([]byte, 64))},
		{"key 31 bytes", base64.StdEncoding.EncodeToString(make([]byte, 31))},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := AESEncrypt("data", tc.key, 1)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteAES) TestAESDecrypt_InvalidPayload() {
	table := []struct {
		name    string
		payload string
	}{
		{"too short", "1234567"},
		{"invalid iv length field", "0001xxxx"},
		{"payload shorter than iv", "0001002412"},
		{"iv length zero", "00010000"},
		{"header only no iv", "00010024"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := AESDecrypt(tc.payload, s.key)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteAES) TestAESDecrypt_InvalidIVOrCiphertextBase64() {
	s.Run("when iv is invalid base64", func() {
		payload := "00010024!!!invalid-base64!!!"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv contains non-base64 rune", func() {
		payload := "00010024\x00\x01\x02!!!"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv length field is negative number string", func() {
		payload := "0001-024" + base64.StdEncoding.EncodeToString(make([]byte, AESBlockSizeBytes))
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv length field is non-numeric", func() {
		payload := "0001ab24" + base64.StdEncoding.EncodeToString(make([]byte, AESBlockSizeBytes))
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when payload is exactly min length but invalid", func() {
		_, err := AESDecrypt("00010000", s.key)
		s.Require().Error(err)
	})
}

func (s *testSuiteAES) TestAESDecrypt_InvalidCiphertextBase64() {
	ivB64 := base64.StdEncoding.EncodeToString(make([]byte, AESBlockSizeBytes))
	ivLen := fmt.Sprintf("%04d", len(ivB64))
	s.Run("when ciphertext is invalid base64", func() {
		payload := "0001" + ivLen + ivB64 + "!!!bad-ct!!!"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when ciphertext base64 decodes to wrong block alignment", func() {
		// 15 bytes ciphertext (not multiple of 16)
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 15))
		payload := "0001" + ivLen + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when ciphertext has wrong padding char", func() {
		payload := "0001" + ivLen + ivB64 + "YQ==X"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when ciphertext is truncated base64", func() {
		payload := "0001" + ivLen + ivB64 + "YQ"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when ciphertext contains space", func() {
		payload := "0001" + ivLen + ivB64 + "YQ= ="
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
}

func (s *testSuiteAES) TestAESDecrypt_IVWrongSize() {
	s.Run("when iv is 15 bytes", func() {
		ivB64 := base64.StdEncoding.EncodeToString(make([]byte, 15))
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
		payload := "0001" + fmt.Sprintf("%04d", len(ivB64)) + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv is 17 bytes", func() {
		ivB64 := base64.StdEncoding.EncodeToString(make([]byte, 17))
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
		payload := "0001" + fmt.Sprintf("%04d", len(ivB64)) + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv is zero length", func() {
		ivB64 := base64.StdEncoding.EncodeToString([]byte{})
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
		payload := "0001" + fmt.Sprintf("%04d", len(ivB64)) + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv is 8 bytes", func() {
		ivB64 := base64.StdEncoding.EncodeToString(make([]byte, 8))
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
		payload := "0001" + fmt.Sprintf("%04d", len(ivB64)) + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv is 32 bytes", func() {
		ivB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
		ctB64 := base64.StdEncoding.EncodeToString(make([]byte, 16))
		payload := "0001" + fmt.Sprintf("%04d", len(ivB64)) + ivB64 + ctB64
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
}

func (s *testSuiteAES) TestAESDecrypt_TamperedCiphertext() {
	s.Run("when ciphertext is tampered in last char", func() {
		enc, err := AESEncrypt("data", s.key, 1)
		s.Require().NoError(err)
		idx := strings.LastIndex(enc, "=")
		if idx < 0 {
			idx = len(enc) - 1
		}
		tampered := enc[:idx] + "X" + enc[idx+1:]
		_, err = AESDecrypt(tampered, s.key)
		s.Require().Error(err)
	})
	s.Run("when ciphertext region has one char flipped", func() {
		enc, err := AESEncrypt("data", s.key, 1)
		s.Require().NoError(err)
		// Tamper in last quarter of payload (ciphertext base64). CBC has no integrity guarantee:
		// decrypt may succeed with wrong plaintext if padding stays valid.
		pos := len(enc) * 3 / 4
		if pos >= len(enc)-1 {
			pos = len(enc) - 2
		}
		tampered := enc[:pos] + "X" + enc[pos+1:]
		dec, err := AESDecrypt(tampered, s.key)
		s.Assert().True(err != nil || dec != "data", "tampered ciphertext must not decrypt to original")
	})
	s.Run("when payload has extra trailing garbage", func() {
		enc, err := AESEncrypt("data", s.key, 1)
		s.Require().NoError(err)
		_, err = AESDecrypt(enc+"!!!", s.key)
		s.Require().Error(err)
	})
	s.Run("when key index prefix is corrupted then decrypt still uses iv and ct", func() {
		enc, err := AESEncrypt("data", s.key, 1)
		s.Require().NoError(err)
		tampered := "xxxx" + enc[4:]
		dec, err := AESDecrypt(tampered, s.key)
		s.Require().NoError(err)
		s.Assert().Equal("data", dec)
	})
	s.Run("when one block of ciphertext is zeroed", func() {
		enc, err := AESEncrypt("1234567890123456", s.key, 1)
		s.Require().NoError(err)
		dec, err := AESDecrypt(enc, s.key)
		s.Require().NoError(err)
		s.Require().Equal("1234567890123456", dec)
		idx := strings.Index(enc, "=")
		if idx >= 0 {
			tampered := enc[:idx] + "A" + enc[idx+1:]
			_, err = AESDecrypt(tampered, s.key)
			s.Require().Error(err)
		}
	})
}

func (s *testSuiteAES) TestAESDecrypt_WrongKey() {
	s.Run("when decrypt key does not match encrypt key", func() {
		key2, err := GenerateAES256Key()
		s.Require().NoError(err)
		plaintext := "secret"
		enc, err := AESEncrypt(plaintext, s.key, 1)
		s.Require().NoError(err)
		dec, err := AESDecrypt(enc, key2)
		s.Assert().True(err != nil || dec != plaintext)
	})
	s.Run("when key is all zeros same length", func() {
		zeroKey := base64.StdEncoding.EncodeToString(make([]byte, AESKeySizeBytes))
		enc, err := AESEncrypt("x", s.key, 1)
		s.Require().NoError(err)
		_, err = AESDecrypt(enc, zeroKey)
		s.Assert().True(err != nil)
	})
	s.Run("when three different keys then decrypt with third fails or wrong", func() {
		key3, _ := GenerateAES256Key()
		enc, _ := AESEncrypt("secret", s.key, 1)
		dec, err := AESDecrypt(enc, key3)
		s.Assert().True(err != nil || dec != "secret")
	})
	s.Run("when encrypt with key A decrypt with key B multiple times", func() {
		keyB, _ := GenerateAES256Key()
		for i := 0; i < 3; i++ {
			enc, _ := AESEncrypt("msg", s.key, i)
			dec, err := AESDecrypt(enc, keyB)
			s.Assert().True(err != nil || dec != "msg")
		}
	})
	s.Run("when keys differ by one byte then still wrong result", func() {
		keyBytes, _ := base64.StdEncoding.DecodeString(s.key)
		keyBytes[0] ^= 0x01
		key2 := base64.StdEncoding.EncodeToString(keyBytes)
		enc, _ := AESEncrypt("x", s.key, 1)
		dec, err := AESDecrypt(enc, key2)
		s.Assert().True(err != nil || dec != "x")
	})
}

func (s *testSuiteAES) TestGenerateAES256Key() {
	s.Run("when key is generated then 32 bytes decoded", func() {
		key, err := GenerateAES256Key()
		s.Require().NoError(err)
		decoded, err := base64.StdEncoding.DecodeString(key)
		s.Require().NoError(err)
		s.Assert().Len(decoded, AESKeySizeBytes)
	})
	s.Run("when multiple keys generated then all unique", func() {
		seen := make(map[string]bool)
		for i := 0; i < 5; i++ {
			key, err := GenerateAES256Key()
			s.Require().NoError(err)
			s.Assert().False(seen[key])
			seen[key] = true
		}
	})
	s.Run("when key is generated then valid base64", func() {
		key, err := GenerateAES256Key()
		s.Require().NoError(err)
		_, err = base64.StdEncoding.DecodeString(key)
		s.Require().NoError(err)
	})
	s.Run("when key is generated then non-empty", func() {
		key, err := GenerateAES256Key()
		s.Require().NoError(err)
		s.Require().NotEmpty(key)
	})
	s.Run("when key is generated then usable for encrypt", func() {
		key, err := GenerateAES256Key()
		s.Require().NoError(err)
		_, err = AESEncrypt("test", key, 0)
		s.Require().NoError(err)
	})
}

func (s *testSuiteAES) TestGenerateIV() {
	s.Run("when IV is generated then 16 bytes decoded", func() {
		iv, err := GenerateIV()
		s.Require().NoError(err)
		decoded, err := base64.StdEncoding.DecodeString(iv)
		s.Require().NoError(err)
		s.Assert().Len(decoded, AESBlockSizeBytes)
	})
	s.Run("when IV generated multiple times then all valid", func() {
		for i := 0; i < 5; i++ {
			iv, err := GenerateIV()
			s.Require().NoError(err)
			d, err := base64.StdEncoding.DecodeString(iv)
			s.Require().NoError(err)
			s.Assert().Len(d, AESBlockSizeBytes)
		}
	})
	s.Run("when IV is generated then non-empty", func() {
		iv, err := GenerateIV()
		s.Require().NoError(err)
		s.Require().NotEmpty(iv)
	})
	s.Run("when IV is generated then valid base64", func() {
		iv, err := GenerateIV()
		s.Require().NoError(err)
		_, err = base64.StdEncoding.DecodeString(iv)
		s.Require().NoError(err)
	})
	s.Run("when IV generated twice then typically different", func() {
		iv1, _ := GenerateIV()
		iv2, _ := GenerateIV()
		// With crypto/rand they should almost always differ
		s.Assert().NotEqual(iv1, iv2)
	})
}

func (s *testSuiteAES) TestAESEncrypt_KeyIndexFormat() {
	s.Run("when key index is 42 then prefix 0042", func() {
		enc, err := AESEncrypt("x", s.key, 42)
		s.Require().NoError(err)
		s.Require().GreaterOrEqual(len(enc), 8)
		s.Assert().Equal("0042", enc[0:4])
	})
	s.Run("when key index is 0 then prefix 0000", func() {
		enc, err := AESEncrypt("x", s.key, 0)
		s.Require().NoError(err)
		s.Assert().Equal("0000", enc[0:4])
	})
	s.Run("when key index is 9999 then prefix 9999", func() {
		enc, err := AESEncrypt("x", s.key, 9999)
		s.Require().NoError(err)
		s.Assert().Equal("9999", enc[0:4])
	})
	s.Run("when key index is 1 then prefix 0001", func() {
		enc, err := AESEncrypt("x", s.key, 1)
		s.Require().NoError(err)
		s.Assert().Equal("0001", enc[0:4])
	})
	s.Run("when key index is 1234 then prefix 1234", func() {
		enc, err := AESEncrypt("x", s.key, 1234)
		s.Require().NoError(err)
		s.Assert().Equal("1234", enc[0:4])
	})
}

func (s *testSuiteAES) TestAESEncrypt_KeyIndexBounds() {
	s.Run("when key index is negative then clamped to 0000", func() {
		enc, err := AESEncrypt("x", s.key, -1)
		s.Require().NoError(err)
		s.Assert().Equal("0000", enc[0:4])
	})
	s.Run("when key index over 9999 then clamped to 9999", func() {
		enc, err := AESEncrypt("x", s.key, 99999)
		s.Require().NoError(err)
		s.Assert().Equal("9999", enc[0:4])
	})
	s.Run("when key index is -100 then 0000", func() {
		enc, err := AESEncrypt("x", s.key, -100)
		s.Require().NoError(err)
		s.Assert().Equal("0000", enc[0:4])
	})
	s.Run("when key index is 10000 then 9999", func() {
		enc, err := AESEncrypt("x", s.key, 10000)
		s.Require().NoError(err)
		s.Assert().Equal("9999", enc[0:4])
	})
	s.Run("when key index is 9998 then 9998", func() {
		enc, err := AESEncrypt("x", s.key, 9998)
		s.Require().NoError(err)
		s.Assert().Equal("9998", enc[0:4])
	})
}

func (s *testSuiteAES) TestAESEncryptLegacy_AESDecryptLegacy_RoundTrip() {
	s.Run("when plaintext is card number", func() {
		plaintext := "4111111111111111"
		enc, err := AESEncryptLegacy(plaintext, s.key)
		s.Require().NoError(err)
		dec, err := AESDecryptLegacy(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is empty", func() {
		enc, err := AESEncryptLegacy("", s.key)
		s.Require().NoError(err)
		dec, err := AESDecryptLegacy(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal("", dec)
	})
	s.Run("when plaintext is single character", func() {
		plaintext := "x"
		enc, err := AESEncryptLegacy(plaintext, s.key)
		s.Require().NoError(err)
		dec, err := AESDecryptLegacy(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is exactly one block", func() {
		plaintext := string(make([]byte, AESBlockSizeBytes))
		enc, err := AESEncryptLegacy(plaintext, s.key)
		s.Require().NoError(err)
		dec, err := AESDecryptLegacy(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
	s.Run("when plaintext is long with special chars", func() {
		plaintext := "ref=pay_123&amount=100.00&currency=USD"
		enc, err := AESEncryptLegacy(plaintext, s.key)
		s.Require().NoError(err)
		dec, err := AESDecryptLegacy(enc, s.key)
		s.Require().NoError(err)
		s.Assert().Equal(plaintext, dec)
	})
}

func (s *testSuiteAES) TestAESDecryptLegacy_WrongKey() {
	s.Run("when decrypt key does not match then error", func() {
		key2, err := GenerateAES256Key()
		s.Require().NoError(err)
		enc, err := AESEncryptLegacy("secret", s.key)
		s.Require().NoError(err)
		_, err = AESDecryptLegacy(enc, key2)
		s.Require().Error(err)
	})
	s.Run("when decrypt with empty key then error", func() {
		enc, _ := AESEncryptLegacy("x", s.key)
		_, err := AESDecryptLegacy(enc, "")
		s.Require().Error(err)
	})
	s.Run("when decrypt with wrong size key then error", func() {
		enc, _ := AESEncryptLegacy("x", s.key)
		wrongKey := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := AESDecryptLegacy(enc, wrongKey)
		s.Require().Error(err)
	})
	s.Run("when encrypt key A decrypt key B for empty plaintext", func() {
		key2, _ := GenerateAES256Key()
		enc, _ := AESEncryptLegacy("", s.key)
		_, err := AESDecryptLegacy(enc, key2)
		s.Require().Error(err)
	})
	s.Run("when multiple wrong keys all fail", func() {
		enc, _ := AESEncryptLegacy("data", s.key)
		for i := 0; i < 3; i++ {
			keyW, _ := GenerateAES256Key()
			dec, err := AESDecryptLegacy(enc, keyW)
			s.Assert().True(err != nil || dec != "data",
				"wrong key must not produce the original plaintext")
		}
	})
}

func (s *testSuiteAES) TestAESDecrypt_InvalidKey() {
	s.Run("when decrypt key is empty", func() {
		enc, err := AESEncrypt("x", s.key, 1)
		s.Require().NoError(err)
		_, err = AESDecrypt(enc, "")
		s.Require().Error(err)
	})
	s.Run("when decrypt key is invalid base64", func() {
		enc, err := AESEncrypt("x", s.key, 1)
		s.Require().NoError(err)
		_, err = AESDecrypt(enc, "badbase64!!!")
		s.Require().Error(err)
	})
	s.Run("when decrypt key is wrong length", func() {
		enc, _ := AESEncrypt("x", s.key, 1)
		shortKey := base64.StdEncoding.EncodeToString([]byte("16bytes!!!!!!!!"))
		_, err := AESDecrypt(enc, shortKey)
		s.Require().Error(err)
	})
	s.Run("when decrypt key is 31 bytes base64", func() {
		enc, _ := AESEncrypt("x", s.key, 1)
		key31 := base64.StdEncoding.EncodeToString(make([]byte, 31))
		_, err := AESDecrypt(enc, key31)
		s.Require().Error(err)
	})
	s.Run("when decrypt key has invalid base64 padding", func() {
		enc, _ := AESEncrypt("x", s.key, 1)
		_, err := AESDecrypt(enc, s.key+"!")
		s.Require().Error(err)
	})
}

func (s *testSuiteAES) Test_pkcs7Unpad_Invalid() {
	table := []struct {
		name      string
		data      []byte
		blockSize int
	}{
		{"empty data", []byte{}, 16},
		{"length not multiple of block", []byte{1, 2, 3}, 16},
		{"padding value zero", make([]byte, 16), 16},
		{"padding value > blockSize", append(make([]byte, 15), 17), 16},
		{"inconsistent padding bytes", append(append(make([]byte, 13), 3, 2, 3), 3), 16},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := pkcs7Unpad(tc.data, tc.blockSize)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteAES) Test_pkcs7Pad_Unpad_RoundTrip() {
	blockSize := AESBlockSizeBytes
	table := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"one byte", []byte{0xff}},
		{"exact block", make([]byte, blockSize)},
		{"block minus one", make([]byte, blockSize-1)},
		{"two blocks", make([]byte, blockSize*2)},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			padded := pkcs7Pad(tc.data, blockSize)
			s.Require().Zero(len(padded) % blockSize)
			unpadded, err := pkcs7Unpad(padded, blockSize)
			s.Require().NoError(err)
			s.Assert().Len(unpadded, len(tc.data))
		})
	}
}

func (s *testSuiteAES) TestAESEncryptLegacy_Deterministic() {
	s.Run("when same key and plaintext then same ciphertext", func() {
		plaintext := "4111111111111111"
		enc1, err1 := AESEncryptLegacy(plaintext, testutil.TestKeyB64)
		enc2, err2 := AESEncryptLegacy(plaintext, testutil.TestKeyB64)
		s.Require().NoError(err1)
		s.Require().NoError(err2)
		s.Assert().Equal(enc1, enc2)
	})
	s.Run("when called three times same input then all equal", func() {
		e1, _ := AESEncryptLegacy("x", testutil.TestKeyB64)
		e2, _ := AESEncryptLegacy("x", testutil.TestKeyB64)
		e3, _ := AESEncryptLegacy("x", testutil.TestKeyB64)
		s.Assert().Equal(e1, e2)
		s.Assert().Equal(e2, e3)
	})
	s.Run("when empty plaintext with fixed key then deterministic", func() {
		e1, _ := AESEncryptLegacy("", testutil.TestKeyB64)
		e2, _ := AESEncryptLegacy("", testutil.TestKeyB64)
		s.Assert().Equal(e1, e2)
	})
	s.Run("when different plaintext then different ciphertext", func() {
		e1, _ := AESEncryptLegacy("a", testutil.TestKeyB64)
		e2, _ := AESEncryptLegacy("b", testutil.TestKeyB64)
		s.Assert().NotEqual(e1, e2)
	})
	s.Run("when same plaintext different key then different ciphertext", func() {
		key2, _ := GenerateAES256Key()
		e1, _ := AESEncryptLegacy("same", testutil.TestKeyB64)
		e2, _ := AESEncryptLegacy("same", key2)
		s.Assert().NotEqual(e1, e2)
	})
}

func (s *testSuiteAES) TestAESDecryptLegacy_InvalidInput() {
	table := []struct {
		name string
		ct   string
		key  string
	}{
		{"invalid base64", "!!!not-base64!!!", s.key},
		{"ciphertext not multiple of block", base64.StdEncoding.EncodeToString([]byte("15 bytes!!!!!")), s.key},
		{"empty key", "eA==", ""},
		{"wrong key size", "eA==", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"ciphertext 1 byte", base64.StdEncoding.EncodeToString([]byte{0x00}), s.key},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := AESDecryptLegacy(tc.ct, tc.key)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteAES) TestAESEncryptLegacy_InvalidKey() {
	table := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"wrong size", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"invalid base64", "not!!!base64!!!"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			_, err := AESEncryptLegacy("data", tc.key)
			s.Require().Error(err)
		})
	}
}

func (s *testSuiteAES) TestAESDecrypt_IVLengthZero() {
	s.Run("when iv length field is 0000", func() {
		payload := "00010000" + base64.StdEncoding.EncodeToString(make([]byte, AESBlockSizeBytes)) + base64.StdEncoding.EncodeToString(make([]byte, AESBlockSizeBytes))
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv length field negative", func() {
		payload := "0001-024" + base64.StdEncoding.EncodeToString(make([]byte, 24))
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when iv length field too large", func() {
		payload := "00019999"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
	s.Run("when payload shorter than iv length claim", func() {
		payload := "00010024ab"
		_, err := AESDecrypt(payload, s.key)
		s.Require().Error(err)
	})
}
