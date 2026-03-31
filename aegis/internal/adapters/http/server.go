package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/api"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
	"github.com/jasonKoogler/abraxis/aegis/internal/config"
	_ "github.com/jasonKoogler/abraxis/aegis/internal/domain" // swagger type resolution
	"github.com/jasonKoogler/abraxis/aegis/internal/service"
)

type Server struct {
	authHandler *authHandler
	userHandler *userHandler

	config *config.Config
	logger *log.Logger
}

func NewServer(authManager *service.AuthManager, userService *service.UserService, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		authHandler: NewAuthHandler(authManager, config, logger),
		userHandler: NewUserHandler(userService),
		config:      config,
		logger:      logger,
	}
}

// RegisterRoutes wires all HTTP routes onto the given chi router.
// Public auth routes are mounted without middleware; protected routes
// are wrapped with the supplied authMiddleware.
func (s *Server) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	// Public auth routes — no authentication required.
	r.Group(func(r chi.Router) {
		r.Post("/auth/login", api.Make(s.authHandler.loginUserWithPassword, s.logger))
		r.Post("/auth/register", api.Make(s.authHandler.registerUser, s.logger))
		r.Post("/auth/refresh", api.Make(s.authHandler.refreshToken, s.logger))
		r.Get("/auth/{provider}", api.Make(s.initiateSocialLogin, s.logger))
		r.Get("/auth/{provider}/callback", api.Make(s.handleSocialLoginCallback, s.logger))
	})

	// Protected routes — require valid authentication.
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		r.Post("/auth/logout", api.Make(s.authHandler.logoutUser, s.logger))

		r.Get("/users", api.Make(s.listUsers, s.logger))
		r.Get("/users/{userID}", api.Make(s.getUserByID, s.logger))
		r.Post("/users/{userID}", api.Make(s.updateUserByID, s.logger))
	})
}

// ---------------------------------------------------------------------------
// Wrapper methods — extract URL/query params and delegate to inner handlers.
// Each carries swaggo annotations for API documentation.
// ---------------------------------------------------------------------------

// initiateSocialLogin godoc
// @Summary      Initiate social login
// @Description  Redirects the user to the specified OAuth provider's login page.
// @Tags         Auth
// @Produce      json
// @Param        provider  path      string  true  "OAuth provider name (e.g. google, github)"
// @Success      200       {object}  map[string]string  "authURL and state"
// @Failure      500       {object}  api.APIError
// @Router       /auth/{provider} [get]
func (s *Server) initiateSocialLogin(w http.ResponseWriter, r *http.Request) error {
	provider := chi.URLParam(r, "provider")
	return s.authHandler.initiateSocialLogin(w, r, provider)
}

// handleSocialLoginCallback godoc
// @Summary      Handle social login callback
// @Description  Processes the OAuth callback from the provider and issues tokens.
// @Tags         Auth
// @Produce      json
// @Param        provider  path      string  true   "OAuth provider name"
// @Param        code      query     string  true   "Authorization code from provider"
// @Param        state     query     string  true   "OAuth state parameter"
// @Success      200       "Tokens set in response headers"
// @Failure      500       {object}  api.APIError
// @Router       /auth/{provider}/callback [get]
func (s *Server) handleSocialLoginCallback(w http.ResponseWriter, r *http.Request) error {
	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	return s.authHandler.ProviderCallback(w, r, provider, code, state)
}

// listUsers godoc
// @Summary      List users
// @Description  Returns a paginated list of all users.
// @Tags         Users
// @Produce      json
// @Param        page      query     int  false  "Page number (default 1)"
// @Param        pageSize  query     int  false  "Page size (default 10)"
// @Success      200       {array}   domain.User
// @Failure      401       {object}  api.APIError
// @Failure      500       {object}  api.APIError
// @Security     BearerAuth
// @Router       /users [get]
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	page := intQueryParam(r, "page", 1)
	pageSize := intQueryParam(r, "pageSize", 10)
	return s.userHandler.getUsers(w, r, &page, &pageSize)
}

// getUserByID godoc
// @Summary      Get user by ID
// @Description  Returns a single user by their unique identifier.
// @Tags         Users
// @Produce      json
// @Param        userID  path      string  true  "User ID"
// @Success      200     {object}  domain.User
// @Failure      400     {object}  api.APIError
// @Failure      401     {object}  api.APIError
// @Failure      500     {object}  api.APIError
// @Security     BearerAuth
// @Router       /users/{userID} [get]
func (s *Server) getUserByID(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "userID")
	return s.userHandler.getUserByID(w, r, id)
}

// updateUserByID godoc
// @Summary      Update user by ID
// @Description  Updates user fields for the given user ID.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        userID  path      string             true  "User ID"
// @Param        body    body      UpdateUserRequest   true  "Fields to update"
// @Success      200     {object}  domain.User
// @Failure      400     {object}  api.APIError
// @Failure      401     {object}  api.APIError
// @Failure      500     {object}  api.APIError
// @Security     BearerAuth
// @Router       /users/{userID} [post]
func (s *Server) updateUserByID(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "userID")
	return s.userHandler.updateUserByID(w, r, id)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// intQueryParam parses an integer query parameter from the request, returning
// defaultVal if the parameter is absent or not a valid integer.
func intQueryParam(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
