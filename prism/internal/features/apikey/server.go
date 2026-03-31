package apikey

import (
	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
	"github.com/jasonKoogler/abraxis/prism/internal/ports"
)

type Server struct {
	apiKeyHandler *apiKeyHandler
	config        *config.Config
	logger        *log.Logger
}

func NewServer(apiKeyService ports.ApiKeyService, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		apiKeyHandler: NewApiKeyHandler(apiKeyService),
		config:        config,
		logger:        logger,
	}
}

func (s *Server) RegisterRoutes(r chi.Router) {
	r.Route("/apikey", func(r chi.Router) {
		r.Get("/", api.Make(s.apiKeyHandler.listApiKeys, s.logger))
		r.Post("/", api.Make(s.apiKeyHandler.createApiKey, s.logger))
		r.Post("/validate", api.Make(s.apiKeyHandler.validateApiKey, s.logger))
		r.Get("/{apikeyID}", api.Make(s.apiKeyHandler.getApiKey, s.logger))
		r.Delete("/{apikeyID}", api.Make(s.apiKeyHandler.revokeApiKey, s.logger))
		r.Put("/{apikeyID}/metadata", api.Make(s.apiKeyHandler.updateApiKeyMetadata, s.logger))
	})
}
