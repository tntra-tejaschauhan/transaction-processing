package crypto

import "context"

//it can be broken down into HSM and Crypto services.
type HSMCryptoService interface {
	VerifyPIN(ctx context.Context, encryptedPIN, keyName string) (bool, error)
	EncryptData(ctx context.Context, plaintext, keyName string) (string, error)
	DecryptData(ctx context.Context, ciphertext, keyName string) (string, error)
}
