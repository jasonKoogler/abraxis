package ports

import (
	"context"
	"time"

	"github.com/jasonKoogler/prism/internal/domain"
)

// Repository defines the interface for schema registry data storage
type Repository interface {
	// Schema operations
	CreateSchema(ctx context.Context, schema domain.Schema) (string, error)
	GetSchema(ctx context.Context, serviceName, name, version string) (domain.Schema, error)
	UpdateSchema(ctx context.Context, schema domain.Schema) error
	DeleteSchema(ctx context.Context, serviceName, name, version string) error
	ListSchemas(ctx context.Context, serviceName, name, version string, schemaType domain.SchemaType, page, pageSize int) ([]domain.Schema, int, error)

	// Bundle operations
	CreateBundle(ctx context.Context, bundle domain.SchemaBundle) (string, error)
	GetBundle(ctx context.Context, serviceName, version string) (domain.SchemaBundle, error)
	ListBundles(ctx context.Context, serviceName, version string, page, pageSize int) ([]domain.SchemaBundle, int, error)

	// Service operations
	RegisterService(ctx context.Context, service domain.ServiceRegistration) (string, error)
	DeregisterService(ctx context.Context, serviceID string) error
	UpdateServiceHeartbeat(ctx context.Context, serviceID, status string) error
	ListServices(ctx context.Context, serviceName, version string, page, pageSize int) ([]domain.ServiceRegistration, int, error)
	GetService(ctx context.Context, serviceID string) (domain.ServiceRegistration, error)
	CleanupStaleServices(ctx context.Context, threshold time.Duration) (int, error)

	// Compatibility operations
	SaveCompatibilityCheck(ctx context.Context, check domain.SchemaCompatibility) (string, error)
	GetCompatibilityHistory(ctx context.Context, serviceName, schemaName string, page, pageSize int) ([]domain.SchemaCompatibility, int, error)

	// Event operations
	SaveSchemaEvent(ctx context.Context, event domain.SchemaEvent) (string, error)
	GetSchemaEvents(ctx context.Context, serviceName, schemaName, version string, since time.Time, page, pageSize int) ([]domain.SchemaEvent, int, error)
	SaveServiceEvent(ctx context.Context, event domain.ServiceEvent) (string, error)
	GetServiceEvents(ctx context.Context, serviceName, serviceID string, since time.Time, page, pageSize int) ([]domain.ServiceEvent, int, error)
}
