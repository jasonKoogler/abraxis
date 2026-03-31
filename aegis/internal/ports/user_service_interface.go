package ports

import (
	"context"

	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

type UserService interface {
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, params *domain.UserCreateParams) (*domain.User, error)
	Update(ctx context.Context, id string, params *domain.UpdateUserParams) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context, page, pageSize int) ([]*domain.User, error)
	SetLastLoginDate(ctx context.Context, id string) error
	GetUserByProviderUserID(ctx context.Context, provider string, providerUserID string) (*domain.User, error)
}
