# Crypto Adapter

## Overview

The Crypto adapter provides secure cryptographic operations for the application, including encryption, decryption, and secure data handling. It implements industry-standard encryption algorithms and protocols to protect sensitive data, such as JSON Web Tokens (JWTs) and other confidential information.

This adapter plays a critical role in the application's security infrastructure by:

1. Securing data at rest and in transit
2. Protecting authentication tokens from tampering
3. Ensuring confidentiality of sensitive information
4. Enabling secure communication between services

## Key Components

### JWE Encryption and Decryption

The adapter provides JSON Web Encryption (JWE) capabilities:

```go
// EncryptWithJWE encrypts a plaintext string using go-jose and returns a compact serialized JWE.
func EncryptWithJWE(plaintext string, key []byte) (string, error)

// DecryptWithJWE decrypts a compact serialized JWE string to retrieve the original plaintext.
func DecryptWithJWE(compactJWE string, key []byte) (string, error)
```

These functions provide:

- Encryption of JWT tokens or other sensitive data
- Standard JWE format compatible with other systems
- AES-256-GCM symmetric encryption for high security
- Compact serialization for efficient transmission

## Implementation Details

### JOSE Library Integration

The adapter integrates with the Go JOSE (Javascript Object Signing and Encryption) library to implement JWE operations:

- Uses `A256GCM` (AES-256 in Galois/Counter Mode) for content encryption
- Implements the `DIRECT` key management algorithm
- Handles compact serialization and deserialization
- Provides proper error handling

### Security Considerations

The adapter incorporates several security best practices:

- Industry-standard encryption algorithms
- No custom cryptographic implementations
- Strong key management practices
- Proper error handling to avoid leaking information

## Related Components

### Password Hasher

While not part of the Crypto adapter directly, the system includes a secure password hashing utility:

```go
// Argon2PasswordHasher implements secure password hashing
type Argon2PasswordHasher struct{}

// Hash generates a hashed password using Argon2id
func (h *Argon2PasswordHasher) Hash(password string) (string, error)

// Verify checks if the provided password matches the hashed password
func (h *Argon2PasswordHasher) Verify(password, encodedHash string) (bool, error)
```

This uses the Argon2id algorithm, which is designed to be:

- Resistant to brute-force attacks
- Memory-hard to defend against GPU/ASIC attacks
- Configurable for different security levels

### API Key Generation

The application also includes secure API key generation functions:

```go
// generateAPIKey generates a new random API key, its prefix for lookup, and its hash for storage
func generateAPIKey() (string, string, string, error)

// hashAPIKey creates a SHA-256 hash of an API key
func hashAPIKey(key string) string

// verifyAPIKey verifies if a raw API key matches a stored hash
func verifyAPIKey(rawKey, storedHash string) bool
```

## Usage Examples

### Encrypting a JWT Token

```go
// Example key - in production, use a securely generated key
encryptionKey := []byte("a-secure-32-byte-key-for-encryption")

// JWT token to encrypt
jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"

// Encrypt the JWT token
encryptedToken, err := crypto.EncryptWithJWE(jwtToken, encryptionKey)
if err != nil {
    log.Fatalf("Failed to encrypt token: %v", err)
}

fmt.Printf("Encrypted token: %s\n", encryptedToken)
```

### Decrypting a JWT Token

```go
// Decrypt the JWT token
decryptedToken, err := crypto.DecryptWithJWE(encryptedToken, encryptionKey)
if err != nil {
    log.Fatalf("Failed to decrypt token: %v", err)
}

fmt.Printf("Decrypted token: %s\n", decryptedToken)
```

### Password Hashing and Verification

```go
import "github.com/jasonKoogler/gauth/internal/common/passwordhasher"

// Hash a user password
hasher := passwordhasher.NewArgon2PasswordHasher()
hashedPassword, err := hasher.Hash("user-secure-password")
if err != nil {
    log.Fatalf("Failed to hash password: %v", err)
}

// Later, verify a password attempt
valid, err := hasher.Verify("password-attempt", hashedPassword)
if err != nil {
    log.Fatalf("Error during verification: %v", err)
}

if valid {
    fmt.Println("Password is correct")
} else {
    fmt.Println("Password is incorrect")
}
```

## Integration with Authentication System

The Crypto adapter is used throughout the authentication system:

```go
// In AuthManager implementation
func (am *AuthManager) AuthenticateWithPassword(ctx context.Context, email, password string, params *domain.SessionMetaDataParams) (*domain.AuthResponse, error) {
    // Get user from database
    user, err := am.GetUserByEmail(ctx, email)
    if err != nil {
        return nil, err
    }

    // Verify password using password hasher
    valid, err := user.ValidatePasswordHash(password)
    if err != nil {
        return nil, err
    }
    if !valid {
        return nil, domain.ErrInvalidCredentials
    }

    // Create session and generate token
    sessionID, err := am.sessionManager.CreateSession(ctx, user.FormatID(), sessionParams)
    if err != nil {
        return nil, err
    }

    // Generate and encrypt tokens
    tokenPair, err := am.tokenManager.GenerateTokenPair(user.FormatID(), sessionID, domain.AuthProviderPassword, roles, nil)
    if err != nil {
        return nil, err
    }

    return &domain.AuthResponse{
        TokenPair: tokenPair,
        SessionID: sessionID,
    }, nil
}
```

## Security Best Practices

When using the Crypto adapter, follow these best practices:

1. **Secure Key Management**

   - Store encryption keys securely
   - Rotate keys periodically
   - Use appropriate key lengths (e.g., 256 bits for AES)

2. **Error Handling**

   - Never expose detailed crypto errors to users
   - Log cryptographic failures for monitoring
   - Use generic error messages in responses

3. **Performance Considerations**

   - Cache frequently used encryption/decryption results when safe
   - Consider the performance impact of cryptographic operations in high-throughput endpoints
   - Use appropriate algorithms for the security/performance tradeoff

4. **Compliance Requirements**
   - Ensure cryptographic algorithms meet relevant standards (e.g., FIPS 140-2)
   - Document cryptographic implementations for security audits
   - Maintain awareness of algorithm deprecations and vulnerabilities
