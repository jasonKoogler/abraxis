package domain

import "errors"

var (
	ErrInvalidUserStatus = errors.New("invalid user status")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrInvalidPhone      = errors.New("invalid phone")
	ErrNoChangesProvided = errors.New("no changes provided")
	ErrNilUpdateParams   = errors.New("nil update params")
	ErrPasswordTooWeak   = errors.New("password too weak")
)
