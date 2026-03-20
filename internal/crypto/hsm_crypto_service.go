package crypto

import (
	"context"
	"fmt"
	"sync"

	sCrypto "github.com/PayWithSpireInc/transaction-processing/pkg/crypto"
	"github.com/PayWithSpireInc/transaction-processing/pkg/secretvault"
)

type hsmCryptoService struct {
	resolver secretvault.KeyResolver
	mu       sync.RWMutex
	keyCache map[string]string
}

func NewHSMCryptoService(resolver secretvault.KeyResolver) HSMCryptoService {
	return &hsmCryptoService{
		resolver: resolver,
		keyCache: make(map[string]string, 8), //In the future we will use a Redis Cache, passed from the injector here.
	}
}

// resolveKey returns the base64-encoded key for keyName, fetching from the secret vault on first
// use and caching the string thereafter. The base64 value is passed directly to pkg/crypto
func (s *hsmCryptoService) resolveKey(ctx context.Context, name string) (string, error) {
	//check the cache first
	s.mu.RLock()
	key, ok := s.keyCache[name]
	s.mu.RUnlock()
	if ok {
		return key, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok = s.keyCache[name]; ok {
		return key, nil
	}

	//get the key from the secret vault
	secretValue, err := s.resolver.GetSecretValue(ctx, name)
	if err != nil {
		return "", fmt.Errorf("hsm: resolve key %q: %w", name, err)
	}

	s.keyCache[name] = secretValue
	return secretValue, nil
}

func (s *hsmCryptoService) VerifyPIN(ctx context.Context, encryptedPIN, keyName string) (bool, error) {
	key, err := s.resolveKey(ctx, keyName)
	if err != nil {
		return false, err
	}
	if _, err = sCrypto.AESDecrypt(encryptedPIN, key); err != nil {
		return false, fmt.Errorf("verify PIN error: %w", err)
	}
	return true, nil
}

func (s *hsmCryptoService) EncryptData(ctx context.Context, plaintext, keyName string) (string, error) {
	key, err := s.resolveKey(ctx, keyName)
	if err != nil {
		return "", err
	}
	return sCrypto.AESEncrypt(plaintext, key, 0)
}

func (s *hsmCryptoService) DecryptData(ctx context.Context, ciphertext, keyName string) (string, error) {
	key, err := s.resolveKey(ctx, keyName)
	if err != nil {
		return "", err
	}
	return sCrypto.AESDecrypt(ciphertext, key)
}
