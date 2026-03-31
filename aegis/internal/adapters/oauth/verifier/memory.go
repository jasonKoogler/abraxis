package verifier

import (
	"context"
	"errors"
	"sync"

	"github.com/jasonKoogler/aegis/internal/adapters/oauth"
)

type MemoryVerifierStorage struct {
	mu        sync.Mutex
	verifiers map[string]string
}

var _ oauth.VerifierStorage = &MemoryVerifierStorage{}

func NewMemoryVerifierStorage() *MemoryVerifierStorage {
	return &MemoryVerifierStorage{
		verifiers: make(map[string]string),
	}
}

func (s *MemoryVerifierStorage) Set(ctx context.Context, state, verifier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifiers[state] = verifier
	return nil
}

func (s *MemoryVerifierStorage) Get(ctx context.Context, state string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	verifier, exists := s.verifiers[state]
	if !exists {
		return "", errors.New("verifier not found")
	}
	return verifier, nil
}

func (s *MemoryVerifierStorage) Del(ctx context.Context, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.verifiers, state)
	return nil
}
