package ports

import "github.com/jasonKoogler/abraxis/prism/internal/config"

type ServiceRepository interface {
	Load() ([]config.ServiceConfig, error)
	Save(services []config.ServiceConfig) error
}
