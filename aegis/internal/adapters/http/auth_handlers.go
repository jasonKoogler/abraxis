package http

import (
	"errors"
	"net/http"

	"github.com/jasonKoogler/aegis/internal/common/api"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/config"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/service"
)

type authHandler struct {
	authManager *service.AuthManager
	config      *config.Config
	logger      *log.Logger
}

func NewAuthHandler(authManager *service.AuthManager, config *config.Config, logger *log.Logger) *authHandler {
	return &authHandler{authManager, config, logger}
}

// loginUserWithPassword godoc
// @Summary      Login with password
// @Description  Authenticates a user with email and password credentials.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  PasswordLoginRequest  true  "Login credentials"
// @Success      200   "Tokens set in response headers"
// @Failure      400   {object}  api.APIError
// @Failure      401   {object}  api.APIError
// @Router       /auth/login [post]
func (ah *authHandler) loginUserWithPassword(w http.ResponseWriter, r *http.Request) error {
	var loginRequest PasswordLoginRequest

	if err := api.BindRequest(r, &loginRequest); err != nil {
		return api.InvalidJSONError()
	}

	sessionParams := domain.SessionMetaDataParamsFromHTTP(r)

	authResponse, err := ah.authManager.AuthenticateWithPassword(r.Context(), loginRequest.Email, loginRequest.Password, sessionParams)
	if err != nil {
		return api.Unauthorized()
	}

	setAuthHeaders(w, authResponse.TokenPair, authResponse.SessionID)
	api.Respond(w, http.StatusOK, nil)
	return nil
}

// logoutUser godoc
// @Summary      Logout
// @Description  Logs the authenticated user out and invalidates their session.
// @Tags         Auth
// @Success      302   "Redirect to home"
// @Failure      401   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (ah *authHandler) logoutUser(w http.ResponseWriter, r *http.Request) error {
	// Retrieve user from context
	usrCtxData, ok := domain.UserContextDataFromContext(r.Context())
	if !ok {
		return api.Unauthorized()
	}

	if err := ah.authManager.Logout(r.Context(), usrCtxData.UserID, usrCtxData.SessionID); err != nil {
		return api.InternalError(err)
	}

	// Redirect to home page or send appropriate response
	api.Redirect(w, r, "/")
	return nil
}

// refreshToken godoc
// @Summary      Refresh tokens
// @Description  Issues a new access/refresh token pair using the current refresh token.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  domain.TokenPair
// @Failure      401  {object}  api.APIError
// @Router       /auth/refresh [post]
func (ah *authHandler) refreshToken(w http.ResponseWriter, r *http.Request) error {
	// Extract the refresh token from the header.
	refreshToken, err := domain.ExtractTokenFromHeader(r)
	if err != nil {
		return api.Unauthorized()
	}

	// Attempt to get user information from the request context.
	// var userID, sessionID, authProvider string
	// if usrCtxData, ok := domain.UserContextDataFromContext(r.Context()); ok {
	// 	// userID = usrCtxData.UserID
	// 	// sessionID = usrCtxData.SessionID
	// 	// authProvider = usrCtxData.AuthProvider
	// } else {
	// 	// If context is missing, decode the refresh token to extract claims.
	// 	claims, err := ah.authManager.GetTokenManager().ParseRefreshToken(refreshToken)
	// 	if err != nil {
	// 		return api.Unauthorized()
	// 	}
	// 	// userID = claims.GetUserID()
	// 	// sessionID = claims.GetSessionID()
	// 	// authProvider = claims.GetAuthProvider()
	// }

	// Retrieve session parameters from the HTTP request.
	sessionParams := domain.SessionMetaDataParamsFromHTTP(r)

	// Call the auth manager's Refresh method with the extracted information.
	tokenPair, err := ah.authManager.RefreshTokens(
		r.Context(),
		refreshToken,
		sessionParams,
	)
	if err != nil {
		return api.Unauthorized()
	}

	// Return the new token pair in the response.
	api.Respond(w, http.StatusOK, tokenPair)
	return nil
}

// registerUser godoc
// @Summary      Register a new user
// @Description  Creates a new user account with password credentials and returns tokens.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  UserRegistrationRequest  true  "Registration details"
// @Success      201   "Tokens set in response headers"
// @Failure      400   {object}  api.APIError
// @Failure      409   {object}  api.APIError
// @Failure      500   {object}  api.APIError
// @Router       /auth/register [post]
func (ah *authHandler) registerUser(w http.ResponseWriter, r *http.Request) error {
	params, err := ReqistrationRequestToUserParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	sessionParams := &domain.SessionMetaDataParams{
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		DeviceFingerprint: r.Header.Get("X-Device-Fingerprint"),
		Location:          r.Header.Get("X-Location"),
	}

	tokenPair, sessionID, err := ah.authManager.RegisterUser(r.Context(), params, sessionParams)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			return api.UserAlreadyExists(err)
		} else {
			return api.InternalError(err)
		}
	}

	setAuthHeaders(w, tokenPair, sessionID)
	api.Respond(w, http.StatusCreated, nil)
	return nil
}

func (ah *authHandler) initiateSocialLogin(w http.ResponseWriter, r *http.Request, provider string) error {
	authURL, state, err := ah.authManager.OAuthGetLoginURL(r.Context(), provider)
	if err != nil {
		return api.InternalError(err)
	}

	// todo: not sure if we should respond with both the authURL and state
	api.Respond(w, http.StatusOK, struct {
		AuthURL string `json:"authURL"`
		State   string `json:"state"`
	}{
		AuthURL: authURL,
		State:   state,
	})
	return nil
}

// ProviderCallback handles authentication callbacks from social providers.
func (ah *authHandler) ProviderCallback(w http.ResponseWriter, r *http.Request, provider, code, state string) error {
	// Retrieve provider from the URL parameter if necessary
	// providerParam := r.URL.Query().Get("provider")
	// if providerParam != "" {
	// 	r = r.WithContext(context.WithValue(r.Context(), gothic.ProviderParamKey, providerParam))
	// }

	sessionMetaDataParams := domain.SessionMetaDataParamsFromHTTP(r)
	tokenPair, sessionID, err := ah.authManager.OAuthCallback(r.Context(), provider, code, state, sessionMetaDataParams)
	if err != nil {
		return api.InternalError(err)
	}

	setAuthHeaders(w, tokenPair, sessionID)
	api.Respond(w, http.StatusOK, nil)
	return nil
}
