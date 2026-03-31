package ports

type OAuthProvider interface {
	GetOAuthProvider() string
	GetRedirectURL() string
	GetClientID() string
	GetClientSecret() string
	GetScopes() []string
	GetAuthURL() string
	GetTokenURL() string

	BeginAuth() (string, error)
	CompleteAuth(code string) (string, error)

	GetUserInfo(code string) (UserInfo, error)
	GetToken(code string) (string, error)
	RefreshToken(refreshToken string) (string, error)
	RevokeToken(token string) error
}

type UserInfo struct {
	ID       string
	Email    string
	Username string
	Avatar   string
}
