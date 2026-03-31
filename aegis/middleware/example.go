package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/aegis/internal/adapters/authz"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/service"
)

// ExampleUsage demonstrates how to use the combined middleware
func ExampleUsage(authManager *service.AuthManager, logger *log.Logger) {
	// Create a router
	router := chi.NewRouter()

	// Create the Authz adapter with webhook path
	webhookPath := "/api/policy-update"
	authzAdapter, err := authz.New(authz.Config{
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		CacheTTL:      time.Minute * 5,
		MaxCacheSize:  1000,
		Logger:        logger,
		LogLevel:      "debug",
		WebhookPath:   webhookPath,
		WebhookSecret: "my-secret-key",
	})
	if err != nil {
		logger.Fatal("Failed to create authz adapter", log.Error(err))
	}

	// Create the combined middleware
	combinedMiddleware := NewCombinedMiddleware(authManager, authzAdapter)

	// Define public paths that don't require authentication
	publicPaths := []string{
		"/auth/login",
		"/auth/register",
		"/auth/google",
		"/auth/google/callback",
	}

	// Example 1: Apply conditional protection to all routes
	// This will skip authentication for paths in the publicPaths list
	router.Use(combinedMiddleware.ConditionalProtect(
		APIInputExtractor,
		publicPaths,
	))

	// Example 2: Protect a specific route with role-based authorization
	router.With(combinedMiddleware.ProtectWithRoles("admin")).
		Get("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Admin Dashboard"))
		})

	// Example 3: Protect a route with policy-based authorization
	router.With(combinedMiddleware.ProtectWithPolicy(APIInputExtractor)).
		Get("/api/resources", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Resources List"))
		})

	// Example 4: Use the UserContextExtractor for simplified policy evaluation
	router.With(combinedMiddleware.Protect(combinedMiddleware.UserContextExtractor())).
		Get("/api/profile", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("User Profile"))
		})

	// Example 5: Use resource-specific input extractor
	resourceAttrs := map[string]interface{}{
		"public": true,
	}
	router.With(combinedMiddleware.Protect(ResourceInputExtractor("document", resourceAttrs))).
		Get("/api/documents", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Documents List"))
		})

	// Example 6: Use ownership-based authorization
	router.With(combinedMiddleware.Protect(OwnershipExtractor("userID"))).
		Get("/api/users/{userID}/documents", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("User Documents"))
		})

	// Example 7: Register the webhook handler for policy updates
	// Convert chi router to standard ServeMux for webhook registration
	if webhookPath != "" {
		webhookMux := http.NewServeMux()
		authzAdapter.RegisterWebhook(webhookMux)

		// Mount the webhook handler on the chi router
		router.Handle(webhookPath, webhookMux)
	}

	// Start the server
	logger.Info("Starting server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		logger.Fatal("Server error", log.Error(err))
	}
}

// ExampleAppIntegration shows how to integrate with the App struct
func ExampleAppIntegration() {
	/*
		// In your app.go file:

		// Add a field for the combined middleware
		type App struct {
			// ... existing fields
			combinedMiddleware *middleware.CombinedMiddleware
		}

		// In your app constructor:
		func NewApp(cfg *config.Config, userRepo domain.UserRepository) *App {
			// ... existing code

			// Create the Authz adapter
			authzAdapter, err := authz.New(authz.Config{
				RedisAddr:     cfg.Auth.AuthZ.RedisConfig.Host + ":" + cfg.Auth.AuthZ.RedisConfig.Port,
				RedisPassword: cfg.Auth.AuthZ.RedisConfig.Password,
				RedisDB:       0,
				CacheTTL:      time.Minute * 5,
				MaxCacheSize:  1000,
				Logger:        logger,
				LogLevel:      cfg.LogLevel.String(),
				Policies:      loadPolicies(cfg.Auth.AuthZ.PolicyFilePath),
				WebhookPath:   cfg.Auth.AuthZ.WebhookPath,
				WebhookSecret: cfg.Auth.AuthZ.GitHubToken,
			})
			if err != nil {
				logger.Fatal("Failed to create authz adapter", log.Error(err))
			}

			// Create the combined middleware
			combinedMiddleware := middleware.NewCombinedMiddleware(authService, authzAdapter)

			return &App{
				// ... existing fields
				combinedMiddleware: combinedMiddleware,
			}
		}

		// Add a method to apply the combined middleware
		func (a *App) ApplyCombinedMiddleware(next http.Handler) http.Handler {
			return a.combinedMiddleware.Protect(middleware.APIInputExtractor)(next)
		}

		// Add a method to apply conditional middleware
		func (a *App) ApplyConditionalMiddleware(next http.Handler, publicPaths []string) http.Handler {
			return a.combinedMiddleware.ConditionalProtect(middleware.APIInputExtractor, publicPaths)(next)
		}
	*/
}

// ExampleRouteHandlers shows how to use the combined middleware in route handlers
func ExampleRouteHandlers() {
	/*
		// In your routes.go file:

		func RegisterRoutes(router chi.Router, app *App) {
			// Public routes (no authentication required)
			router.Group(func(r chi.Router) {
				r.Post("/auth/login", app.authHandler.Login)
				r.Post("/auth/register", app.authHandler.Register)
			})

			// Protected routes (authentication required)
			router.Group(func(r chi.Router) {
				// Apply the combined middleware to all routes in this group
				r.Use(app.combinedMiddleware.Protect(middleware.APIInputExtractor))

				r.Get("/api/users", app.userHandler.ListUsers)
				r.Get("/api/users/{id}", app.userHandler.GetUser)
				r.Put("/api/users/{id}", app.userHandler.UpdateUser)
				r.Delete("/api/users/{id}", app.userHandler.DeleteUser)
			})

			// Admin-only routes
			router.Group(func(r chi.Router) {
				// Apply role-based protection
				r.Use(app.combinedMiddleware.ProtectWithRoles("admin"))

				r.Get("/admin/dashboard", app.adminHandler.Dashboard)
				r.Get("/admin/users", app.adminHandler.ListUsers)
			})

			// Resource-specific routes with ownership checks
			router.Group(func(r chi.Router) {
				// Apply ownership-based protection
				r.Use(app.combinedMiddleware.Protect(middleware.OwnershipExtractor("userID")))

				r.Get("/api/users/{userID}/documents", app.documentHandler.ListUserDocuments)
				r.Post("/api/users/{userID}/documents", app.documentHandler.CreateUserDocument)
			})
		}
	*/
}

// ExampleServerIntegration shows how to integrate with the Server struct
func ExampleServerIntegration() {
	/*
		// In your server.go file:

		// Add a field for the combined middleware
		type Server struct {
			// ... existing fields
			combinedMiddleware *middleware.CombinedMiddleware
		}

		// In your server constructor:
		func NewServer(cfg *config.Config, logger *log.Logger, opts ...ServerOption) (*Server, error) {
			s := &Server{
				cfg:        cfg,
				logger:     logger,
				router:     chi.NewRouter(),
				shutdownCh: make(chan struct{}),
			}

			// Apply options
			for _, opt := range opts {
				if err := opt(s); err != nil {
					return nil, err
				}
			}

			return s, nil
		}

		// Add a server option for the combined middleware
		func WithCombinedMiddleware(authManager *service.AuthManager, authzAdapter *authz.Adapter) ServerOption {
			return func(s *Server) error {
				s.combinedMiddleware = middleware.NewCombinedMiddleware(authManager, authzAdapter)
				return nil
			}
		}

		// Add methods to register protected handlers
		func (s *Server) RegisterProtectedHandler(path string, handler http.Handler) {
			// Apply the combined middleware
			protectedHandler := s.combinedMiddleware.Protect(middleware.APIInputExtractor)(handler)
			s.router.Mount(path, protectedHandler)
		}

		func (s *Server) RegisterRoleProtectedHandler(path string, handler http.Handler, roles ...string) {
			// Apply the role-based middleware
			protectedHandler := s.combinedMiddleware.ProtectWithRoles(roles...)(handler)
			s.router.Mount(path, protectedHandler)
		}

		func (s *Server) RegisterPolicyProtectedHandler(path string, handler http.Handler) {
			// Apply the policy-based middleware
			protectedHandler := s.combinedMiddleware.ProtectWithPolicy(middleware.APIInputExtractor)(handler)
			s.router.Mount(path, protectedHandler)
		}
	*/
}
