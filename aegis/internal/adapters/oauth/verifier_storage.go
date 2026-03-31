package oauth

import (
	"context"
)

type VerifierStorage interface {
	Get(ctx context.Context, state string) (string, error)
	Set(ctx context.Context, state, verifier string) error
	Del(ctx context.Context, state string) error
}
