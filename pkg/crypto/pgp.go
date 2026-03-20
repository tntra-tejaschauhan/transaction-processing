package crypto

import (
	"bytes"
	"crypto"
	"errors"
	"fmt"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// pgpConfig supplies defaults for OpenPGP operations.
var pgpConfig = &packet.Config{DefaultHash: crypto.SHA256}

const pgpMessageType = "PGP MESSAGE"

// PGPEncrypt encrypts plaintext for the given recipient public key (armored).
func PGPEncrypt(plaintext []byte, recipientPublicKeyArmored string) (string, error) {
	if recipientPublicKeyArmored == "" {
		return "", errors.New("pgp encrypt: recipient public key is empty")
	}
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(recipientPublicKeyArmored))
	if err != nil {
		return "", fmt.Errorf("pgp encrypt: read recipient key: %w", err)
	}
	if len(entityList) == 0 {
		return "", errors.New("pgp encrypt: no entity in recipient key")
	}
	var output bytes.Buffer
	writer, err := openpgp.Encrypt(&output, entityList, nil, nil, pgpConfig)
	if err != nil {
		return "", fmt.Errorf("pgp encrypt: %w", err)
	}
	if _, err := writer.Write(plaintext); err != nil {
		_ = writer.Close()
		return "", fmt.Errorf("pgp encrypt: write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("pgp encrypt: close: %w", err)
	}
	return armorBytes(output.Bytes(), pgpMessageType)
}

// PGPDecrypt decrypts an armored PGP message with the given private key (armored).
// passphrase can be nil for unencrypted private keys.
func PGPDecrypt(ciphertextArmored string, privateKeyArmored string, passphrase []byte) ([]byte, error) {
	if ciphertextArmored == "" {
		return nil, errors.New("pgp decrypt: ciphertext is empty")
	}
	if privateKeyArmored == "" {
		return nil, errors.New("pgp decrypt: private key is empty")
	}
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(privateKeyArmored))
	if err != nil {
		return nil, fmt.Errorf("pgp decrypt: read private key: %w", err)
	}
	if len(entityList) == 0 {
		return nil, errors.New("pgp decrypt: no entity in private key")
	}
	entity := entityList[0]
	if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
		if err := entity.PrivateKey.Decrypt(passphrase); err != nil {
			return nil, fmt.Errorf("pgp decrypt: decrypt private key: %w", err)
		}
	}
	for _, subkey := range entity.Subkeys {
		if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
			if err := subkey.PrivateKey.Decrypt(passphrase); err != nil {
				return nil, fmt.Errorf("pgp decrypt: decrypt subkey: %w", err)
			}
		}
	}
	block, err := armor.Decode(bytes.NewBufferString(ciphertextArmored))
	if err != nil {
		return nil, fmt.Errorf("pgp decrypt: decode armor: %w", err)
	}
	msgDetails, err := openpgp.ReadMessage(block.Body, entityList, func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if symmetric {
			return passphrase, nil
		}
		return nil, nil
	}, pgpConfig)
	if err != nil {
		return nil, fmt.Errorf("pgp decrypt: read message: %w", err)
	}
	plaintext, err := io.ReadAll(msgDetails.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("pgp decrypt: read body: %w", err)
	}
	if msgDetails.SignatureError != nil {
		return plaintext, fmt.Errorf("pgp decrypt: signature verification failed: %w", msgDetails.SignatureError)
	}
	return plaintext, nil
}

// PGPSign produces a detached armored signature of plaintext using the signer's private key (armored).
// passphrase can be nil for unencrypted private keys.
func PGPSign(plaintext []byte, signerPrivateKeyArmored string, passphrase []byte) (string, error) {
	if signerPrivateKeyArmored == "" {
		return "", errors.New("pgp sign: private key is empty")
	}
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(signerPrivateKeyArmored))
	if err != nil {
		return "", fmt.Errorf("pgp sign: read private key: %w", err)
	}
	if len(entityList) == 0 {
		return "", errors.New("pgp sign: no entity in private key")
	}
	entity := entityList[0]
	if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
		if err := entity.PrivateKey.Decrypt(passphrase); err != nil {
			return "", fmt.Errorf("pgp sign: decrypt private key: %w", err)
		}
	}
	var output bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&output, entity, bytes.NewReader(plaintext), pgpConfig); err != nil {
		return "", fmt.Errorf("pgp sign: %w", err)
	}
	return output.String(), nil
}

// PGPVerify verifies a detached armored signature over plaintext with the signer's public key (armored).
func PGPVerify(plaintext []byte, signatureArmored string, signerPublicKeyArmored string) error {
	if signatureArmored == "" {
		return errors.New("pgp verify: signature is empty")
	}
	if signerPublicKeyArmored == "" {
		return errors.New("pgp verify: signer public key is empty")
	}
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(signerPublicKeyArmored))
	if err != nil {
		return fmt.Errorf("pgp verify: read signer key: %w", err)
	}
	if len(entityList) == 0 {
		return errors.New("pgp verify: no entity in signer key")
	}
	_, err = openpgp.CheckArmoredDetachedSignature(entityList, bytes.NewReader(plaintext), bytes.NewBufferString(signatureArmored), pgpConfig)
	if err != nil {
		return fmt.Errorf("pgp verify: %w", err)
	}
	return nil
}

func armorBytes(data []byte, blockType string) (string, error) {
	// Armor is base64 (4/3 expansion) plus header/footer (~80 bytes).
	size := 4*((len(data)+2)/3) + 80
	var output bytes.Buffer
	output.Grow(size)
	writer, err := armor.Encode(&output, blockType, nil)
	if err != nil {
		return "", err
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return output.String(), nil
}
