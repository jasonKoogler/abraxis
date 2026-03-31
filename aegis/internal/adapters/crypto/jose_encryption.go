package crypto

import (
	"github.com/go-jose/go-jose/v4"
)

// EncryptWithJWE encrypts a plaintext string (for example, a JWT) using go-jose and returns a compact serialized JWE.
func EncryptWithJWE(plaintext string, key []byte) (string, error) {
	// Use the DIRECT algorithm to directly encrypt with a symmetric key.
	recipient := jose.Recipient{
		Algorithm: jose.DIRECT,
		Key:       key,
	}
	// For example, we choose A256GCM as our content encryption method.
	encrypter, err := jose.NewEncrypter(jose.A256GCM, recipient, nil)
	if err != nil {
		return "", err
	}

	jweObject, err := encrypter.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}

	// Serialize the JWE object into the compact format.
	return jweObject.CompactSerialize()
}

// DecryptWithJWE decrypts a compact serialized JWE string to retrieve the original plaintext.
func DecryptWithJWE(compactJWE string, key []byte) (string, error) {
	jweObject, err := jose.ParseEncrypted(compactJWE, []jose.KeyAlgorithm{jose.RSA_OAEP}, []jose.ContentEncryption{jose.A256GCM})
	if err != nil {
		return "", err
	}

	// Decrypt returns the original plaintext.
	decrypted, err := jweObject.Decrypt(key)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}
