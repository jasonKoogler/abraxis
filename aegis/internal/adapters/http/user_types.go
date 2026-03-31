package http

import (
	"errors"
	"net/http"

	"github.com/jasonKoogler/abraxis/aegis/internal/common/api"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

func ReqistrationRequestToUserParams(r *http.Request) (*domain.UserCreateParams, error) {
	req := &UserRegistrationRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	if req.Password == "" {
		return nil, errors.New("password is required")
	}

	return domain.NewCreateUserParams(
		req.Email,
		req.FirstName,
		req.LastName,
		req.Phone,
		&req.Password,
		domain.AuthProviderPassword,
		nil,
	)
}

func UpdateUserRequestToParams(r *http.Request) (*domain.UpdateUserParams, error) {
	req := &UpdateUserRequest{}
	if err := api.BindRequest(r, req); err != nil {
		return nil, err
	}

	params := domain.UpdateUserParams{
		Provider:  req.AuthProvider,
		AvatarURL: req.AvatarURL,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Status:    req.Status,
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &params, nil
}
