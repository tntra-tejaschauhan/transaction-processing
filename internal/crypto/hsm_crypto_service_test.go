package crypto

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"

	"github.com/PayWithSpireInc/transaction-processing/pkg/secretvault"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const testAES256KeyBase64 = "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="

type mockKeyResolver struct {
	mock.Mock
}

func (m *mockKeyResolver) GetSecretValue(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

var _ secretvault.KeyResolver = (*mockKeyResolver)(nil)

func newMockKeyResolver(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockKeyResolver {
	m := &mockKeyResolver{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

type testSuiteHSMCrypto struct {
	suite.Suite
	encryptionKeyBase64 string
	mockResolver        *mockKeyResolver
	service             HSMCryptoService
}

func TestHSMCrypto(t *testing.T) {
	suite.Run(t, new(testSuiteHSMCrypto))
}

func (s *testSuiteHSMCrypto) SetupSubTest() {
	s.encryptionKeyBase64 = testAES256KeyBase64
	s.mockResolver = newMockKeyResolver(s.T())
	s.service = NewHSMCryptoService(s.mockResolver)
}

func (s *testSuiteHSMCrypto) TestResolveKey_Caching() {
	s.Run("when key is resolved twice then resolver is called only once", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "encryption-key").Return(s.encryptionKeyBase64, nil).Once()
		concrete := s.service.(*hsmCryptoService)

		firstResult, err := concrete.resolveKey(context.Background(), "encryption-key")
		s.Require().NoError(err)
		secondResult, err := concrete.resolveKey(context.Background(), "encryption-key")
		s.Require().NoError(err)

		s.Assert().Equal(firstResult, secondResult)
	})

	s.Run("when resolver returns an error then it is wrapped with the key name", func() {
		resolverErr := errors.New("vault down")
		s.mockResolver.On("GetSecretValue", mock.Anything, "missing-key").Return("", resolverErr).Once()
		concrete := s.service.(*hsmCryptoService)

		_, err := concrete.resolveKey(context.Background(), "missing-key")
		s.Require().Error(err)
		s.Assert().ErrorIs(err, resolverErr)
		s.Assert().Contains(err.Error(), `resolve key "missing-key"`)
	})

	s.Run("when resolver fails then the key is not cached and a retry succeeds", func() {
		transientErr := errors.New("vault timeout")
		s.mockResolver.On("GetSecretValue", mock.Anything, "retry-key").Return("", transientErr).Once()
		s.mockResolver.On("GetSecretValue", mock.Anything, "retry-key").Return(s.encryptionKeyBase64, nil).Once()
		ctx := context.Background()

		_, err := s.service.EncryptData(ctx, "payload", "retry-key")
		s.Require().Error(err)
		s.Assert().ErrorIs(err, transientErr)

		ciphertext, err := s.service.EncryptData(ctx, "payload", "retry-key")
		s.Require().NoError(err)
		s.Require().NotEmpty(ciphertext)
	})
}

func (s *testSuiteHSMCrypto) TestResolveKey_ConcurrentAccess() {
	s.Run("when many goroutines resolve the same key then resolver is called exactly once", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "shared-key").Return(s.encryptionKeyBase64, nil).Once()
		concrete := s.service.(*hsmCryptoService)

		const goroutineCount = 64
		var resolvedKeys [goroutineCount]string
		var errs [goroutineCount]error
		var wg sync.WaitGroup

		for i := range goroutineCount {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				resolved, err := concrete.resolveKey(context.Background(), "shared-key")
				errs[index] = err
				resolvedKeys[index] = resolved
			}(i)
		}
		wg.Wait()

		for i := range goroutineCount {
			s.Require().NoError(errs[i], "goroutine %d returned an error", i)
			s.Assert().Equal(resolvedKeys[0], resolvedKeys[i], "goroutine %d received a different key", i)
		}
	})
}

func (s *testSuiteHSMCrypto) TestVerifyPIN() {
	s.Run("when PIN was encrypted with the same key then verify returns true", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "pin-key").Return(s.encryptionKeyBase64, nil).Once()
		ctx := context.Background()

		encryptedPIN, err := s.service.EncryptData(ctx, "1234", "pin-key")
		s.Require().NoError(err)

		valid, err := s.service.VerifyPIN(ctx, encryptedPIN, "pin-key")
		s.Require().NoError(err)
		s.Assert().True(valid)
	})

	s.Run("when encrypted payload is malformed then verify returns false with error", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "pin-key").Return(s.encryptionKeyBase64, nil).Once()

		valid, err := s.service.VerifyPIN(context.Background(), "not-a-valid-payload", "pin-key")
		s.Require().Error(err)
		s.Assert().False(valid)
		s.Assert().Contains(err.Error(), "verify PIN error")
	})

	s.Run("when resolver fails then error propagates and verify returns false", func() {
		resolverErr := errors.New("no vault")
		s.mockResolver.On("GetSecretValue", mock.Anything, "pin-key").Return("", resolverErr).Once()

		valid, err := s.service.VerifyPIN(context.Background(), "anything", "pin-key")
		s.Require().Error(err)
		s.Assert().False(valid)
		s.Assert().ErrorIs(err, resolverErr)
	})
}

func (s *testSuiteHSMCrypto) TestEncryptDecrypt_RoundTrip() {
	table := []struct {
		name      string
		plaintext string
	}{
		{"when plaintext is a PIN", "1234"},
		{"when plaintext is a card number", "4111111111111111"},
		{"when plaintext is empty", ""},
		{"when plaintext is unicode", "café naïve 日本"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			s.mockResolver.On("GetSecretValue", mock.Anything, "data-key").Return(s.encryptionKeyBase64, nil).Once()
			ctx := context.Background()

			ciphertext, err := s.service.EncryptData(ctx, tc.plaintext, "data-key")
			s.Require().NoError(err)

			decrypted, err := s.service.DecryptData(ctx, ciphertext, "data-key")
			s.Require().NoError(err)
			s.Assert().Equal(tc.plaintext, decrypted)
		})
	}
}

func (s *testSuiteHSMCrypto) TestEncryptData_ResolverError() {
	s.Run("when resolver fails then error propagates", func() {
		resolverErr := errors.New("resolver down")
		s.mockResolver.On("GetSecretValue", mock.Anything, "data-key").Return("", resolverErr).Once()

		_, err := s.service.EncryptData(context.Background(), "plaintext", "data-key")
		s.Require().Error(err)
		s.Assert().ErrorIs(err, resolverErr)
	})
}

func (s *testSuiteHSMCrypto) TestEncryptData_InvalidKeyFromResolver() {
	s.Run("when resolver returns invalid base64 then error propagates from crypto layer", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "bad-key").Return("not-valid-base64!!!", nil).Once()

		_, err := s.service.EncryptData(context.Background(), "plaintext", "bad-key")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "key")
	})

	s.Run("when resolver returns wrong key size then error propagates from crypto layer", func() {
		shortKeyBase64 := base64.StdEncoding.EncodeToString([]byte("short"))
		s.mockResolver.On("GetSecretValue", mock.Anything, "short-key").Return(shortKeyBase64, nil).Once()

		_, err := s.service.EncryptData(context.Background(), "plaintext", "short-key")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "32")
	})
}

func (s *testSuiteHSMCrypto) TestDecryptData_ResolverError() {
	s.Run("when resolver fails then error propagates", func() {
		resolverErr := errors.New("resolver down")
		s.mockResolver.On("GetSecretValue", mock.Anything, "data-key").Return("", resolverErr).Once()

		_, err := s.service.DecryptData(context.Background(), "ciphertext", "data-key")
		s.Require().Error(err)
		s.Assert().ErrorIs(err, resolverErr)
	})
}

func (s *testSuiteHSMCrypto) TestDecryptData_InvalidCiphertext() {
	s.Run("when ciphertext is garbage then error is returned and no panic", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "data-key").Return(s.encryptionKeyBase64, nil).Once()

		_, err := s.service.DecryptData(context.Background(), "garbage-not-valid-payload", "data-key")
		s.Require().Error(err)
	})

	s.Run("when ciphertext is empty then error is returned and no panic", func() {
		s.mockResolver.On("GetSecretValue", mock.Anything, "data-key").Return(s.encryptionKeyBase64, nil).Once()

		_, err := s.service.DecryptData(context.Background(), "", "data-key")
		s.Require().Error(err)
	})
}

func (s *testSuiteHSMCrypto) TestEncryptDecrypt_IndependentKeys() {
	s.Run("when two keys are cached then each decrypts only its own ciphertext", func() {
		secondKeyBase64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
		s.mockResolver.On("GetSecretValue", mock.Anything, "key-alpha").Return(s.encryptionKeyBase64, nil).Once()
		s.mockResolver.On("GetSecretValue", mock.Anything, "key-beta").Return(secondKeyBase64, nil).Once()
		ctx := context.Background()

		ciphertextAlpha, err := s.service.EncryptData(ctx, "secret-one", "key-alpha")
		s.Require().NoError(err)
		ciphertextBeta, err := s.service.EncryptData(ctx, "secret-two", "key-beta")
		s.Require().NoError(err)

		decryptedAlpha, err := s.service.DecryptData(ctx, ciphertextAlpha, "key-alpha")
		s.Require().NoError(err)
		s.Assert().Equal("secret-one", decryptedAlpha)

		decryptedBeta, err := s.service.DecryptData(ctx, ciphertextBeta, "key-beta")
		s.Require().NoError(err)
		s.Assert().Equal("secret-two", decryptedBeta)
	})
}

func (s *testSuiteHSMCrypto) TestKeyCache_ConcurrentMultipleKeys() {
	s.Run("when goroutines encrypt and decrypt with two keys concurrently then resolver is called once per key", func() {
		secondKeyBase64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
		s.mockResolver.On("GetSecretValue", mock.Anything, "key-alpha").Return(s.encryptionKeyBase64, nil).Once()
		s.mockResolver.On("GetSecretValue", mock.Anything, "key-beta").Return(secondKeyBase64, nil).Once()
		ctx := context.Background()

		type result struct {
			decrypted string
			encErr    error
			decErr    error
		}

		const goroutineCount = 40
		results := make([]result, goroutineCount)
		var wg sync.WaitGroup
		for i := range goroutineCount {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				selectedKey := "key-alpha"
				if index%2 == 0 {
					selectedKey = "key-beta"
				}
				ciphertext, encErr := s.service.EncryptData(ctx, "payload", selectedKey)
				if encErr != nil {
					results[index] = result{encErr: encErr}
					return
				}
				decrypted, decErr := s.service.DecryptData(ctx, ciphertext, selectedKey)
				results[index] = result{decrypted: decrypted, decErr: decErr}
			}(i)
		}
		wg.Wait()

		for i, r := range results {
			s.Require().NoError(r.encErr, "goroutine %d encrypt failed", i)
			s.Require().NoError(r.decErr, "goroutine %d decrypt failed", i)
			s.Assert().Equal("payload", r.decrypted, "goroutine %d round-trip mismatch", i)
		}
	})
}
