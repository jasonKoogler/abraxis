package domain

type AuthResponse struct {
	TokenPair *TokenPair
	SessionID string
}
