package domain

type AuthProvider string

const (
	AuthProviderPassword AuthProvider = "password"
	AuthProviderGoogle   AuthProvider = "google"
)
