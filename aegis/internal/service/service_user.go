package service

import (
	"context"
	"fmt"

	"github.com/jasonKoogler/aegis/internal/common/util"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

type UserService struct {
	userRepo ports.UserRepository
}

var _ ports.UserService = &UserService{}

func NewUserService(userRepo ports.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

func (s *UserService) Create(ctx context.Context, params *domain.UserCreateParams) (*domain.User, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Check if user exists
	existingUser, err := s.userRepo.GetByEmail(ctx, params.Email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, domain.ErrUserAlreadyExists
	}

	// Create user
	newUser, err := domain.NewUser(params)
	if err != nil {
		return nil, err
	}

	// Save user
	user, err := s.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	uid, err := util.TrimUserPrefix(id)
	if err != nil {
		return nil, err
	}

	return s.userRepo.GetByID(ctx, uid)
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if !util.IsValidEmail(email) {
		return nil, fmt.Errorf("invalid email")
	}

	return s.userRepo.GetByEmail(ctx, email)
}

func (s *UserService) Update(ctx context.Context, id string, params *domain.UpdateUserParams) (*domain.User, error) {
	uid, err := util.TrimUserPrefix(id)
	if err != nil {
		return nil, err
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	existingUser, err := s.userRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	if existingUser == nil {
		return nil, domain.ErrUserNotFound
	}

	err = existingUser.Update(params)
	if err != nil {
		return nil, err
	}

	updatedUser, err := s.userRepo.Update(ctx, uid, existingUser)
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	uid, err := util.TrimUserPrefix(id)
	if err != nil {
		return err
	}
	return s.userRepo.Delete(ctx, uid)
}

func (s *UserService) ListAll(ctx context.Context, page, pageSize int) ([]*domain.User, error) {
	return s.userRepo.ListAll(ctx, page, pageSize)
}

func (s *UserService) SetLastLoginDate(ctx context.Context, id string) error {
	uid, err := util.TrimUserPrefix(id)
	if err != nil {
		return err
	}
	return s.userRepo.SetLastLoginDate(ctx, uid)
}

func (s *UserService) GetUserByProviderUserID(ctx context.Context, provider string, providerUserID string) (*domain.User, error) {
	return s.userRepo.GetUserByProviderUserID(ctx, provider, providerUserID)
}
