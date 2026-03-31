package audit

import (
	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
)

type Server struct {
	auditHandler *auditHandler
	config       *config.Config
	logger       *log.Logger
}

func NewServer(auditHandler *auditHandler, config *config.Config, logger *log.Logger) *Server {
	return &Server{
		auditHandler: auditHandler,
		config:       config,
		logger:       logger,
	}
}

func (s *Server) RegisterRoutes(r chi.Router) {
	r.Route("/audit", func(r chi.Router) {
		r.Get("/", api.Make(s.auditHandler.listAuditLogs, s.logger))
		r.Get("/aggregate/{groupBy}", api.Make(s.auditHandler.aggregateAuditLogs, s.logger))
		r.Get("/export", api.Make(s.auditHandler.exportAuditLogs, s.logger))
		r.Get("/{auditID}", api.Make(s.auditHandler.getAuditLog, s.logger))
	})
}
