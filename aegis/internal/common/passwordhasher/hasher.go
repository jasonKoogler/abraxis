package passwordhasher

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    uint32 = 3         // Number of iterations
	argon2Memory  uint32 = 64 * 1024 // 64 MB memory
	argon2Threads uint8  = 4         // Number of threads
	argon2KeyLen  uint32 = 32        // Length of the derived key
	saltLength    int    = 16        // Length of the random salt
)

// Argon2PasswordHasher implements the PasswordHasher interface using Argon2id.
type Argon2PasswordHasher struct{}

// NewArgon2PasswordHasher creates a new instance of Argon2PasswordHasher.
func NewArgon2PasswordHasher() *Argon2PasswordHasher {
	return &Argon2PasswordHasher{}
}

// Hash generates a hashed password using Argon2id and encodes it in the PHC string format.
func (h *Argon2PasswordHasher) Hash(password string) (string, error) {
	// Generate a cryptographically secure random salt.
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Derive the key using Argon2id.
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode the parameters, salt, and hash into a single string in PHC format.
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))

	return encodedHash, nil
}

// Verify checks if the provided password matches the hashed password.
func (h *Argon2PasswordHasher) Verify(password, encodedHash string) (bool, error) {
	// Extract the parameters, salt, and hash from the encoded password.
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Derive the key using the same parameters.
	derivedHash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, uint32(len(hash)))

	// Compare the hashes in constant time.
	if subtle.ConstantTimeCompare(hash, derivedHash) == 1 {
		return true, nil
	}
	return false, nil
}

// ValidateHash checks if the provided hash string is a valid Argon2 hash in PHC format.
func (h *Argon2PasswordHasher) ValidateHash(encodedHash string) error {
	// Try to decode the hash. If decoding succeeds, the hash is valid.
	_, _, _, err := decodeHash(encodedHash)
	return err
}

// argon2Params holds the parameters used for Argon2 hashing.
type argon2Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
}

func decodeHash(encodedHash string) (params *argon2Params, salt, hash []byte, err error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, fmt.Errorf("invalid hash format")
	}

	if parts[1] != "argon2id" {
		return nil, nil, nil, fmt.Errorf("unsupported algorithm: %s", parts[1])
	}

	var version int
	_, err = fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid version format: %v", err)
	}
	if version != argon2.Version {
		return nil, nil, nil, fmt.Errorf("incompatible Argon2 version: %d", version)
	}

	params = &argon2Params{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Time, &params.Threads)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid parameters format: %v", err)
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid salt encoding: %v", err)
	}

	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid hash encoding: %v", err)
	}

	return params, salt, hash, nil
}
