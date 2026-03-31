package ports

import (
	"net/http/httputil"

	"github.com/jasonKoogler/prism/internal/config"
)

type ServiceRegistry interface {
	Register(cfg config.ServiceConfig) error
	Deregister(serviceName string) error
	Get(serviceName string) (*ServiceEntry, bool)
	List() []config.ServiceConfig
	Update(cfg config.ServiceConfig) error
	LoadFromConfig(cfg *config.Config) error
	LoadFromRepository() error
}

// ServiceEntry represents a registered service with its proxy and configuration
type ServiceEntry struct {
	Config config.ServiceConfig
	Proxy  *httputil.ReverseProxy
}
