package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/config"
	"github.com/jasonKoogler/prism/internal/features/apikey"
	"github.com/jasonKoogler/prism/internal/features/audit"
	"github.com/jasonKoogler/prism/internal/ports"
)

// RegisterAuditRoutes registers the audit routes with the provided router.
func RegisterAuditRoutes(r chi.Router, auditService ports.AuditService, cfg *config.Config, logger *log.Logger) {
	auditHandler := audit.NewAuditHandler(auditService, cfg, logger)
	auditServer := audit.NewServer(auditHandler, cfg, logger)
	auditServer.RegisterRoutes(r)
	logger.Info("Registered audit routes")
}

// RegisterApiKeyRoutes registers the API key routes with the provided router.
func RegisterApiKeyRoutes(r chi.Router, apiKeyService ports.ApiKeyService, cfg *config.Config, logger *log.Logger) {
	apiKeyServer := apikey.NewServer(apiKeyService, cfg, logger)
	apiKeyServer.RegisterRoutes(r)
	logger.Info("Registered API key routes")
}
