package ports

// TokenRevoker publishes token revocation events to downstream consumers
// (e.g. the gRPC event bus that Prism subscribes to).
type TokenRevoker interface {
	// PublishTokenRevoked broadcasts a revocation event for the given JWT ID.
	// expiresAt is the original token's expiry as a Unix timestamp so that
	// consumers can set an appropriate cache TTL.
	PublishTokenRevoked(jti string, expiresAt int64)
}
