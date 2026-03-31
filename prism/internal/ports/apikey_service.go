package ports

import (
	"context"

	"github.com/jasonKoogler/abraxis/prism/internal/domain"
)

type ApiKeyService interface {
	Create(ctx context.Context, params *domain.APIKey_CreateParams) (*domain.APIKey_CreateResponse, error)
	Validate(ctx context.Context, rawKey, ipAddress string) (*domain.APIKey, error)
	Get(ctx context.Context, id string) (*domain.APIKey, error)
	UpdateMetadata(ctx context.Context, params *domain.APIKey_UpdateMetadataParams) (*domain.APIKey, error)
	Revoke(ctx context.Context, id string) error
	List(ctx context.Context, params *domain.APIKey_ListParams) ([]*domain.APIKey, error)
	HasScope(apiKey *domain.APIKey, requiredScope string) bool
}
