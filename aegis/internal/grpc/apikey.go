package grpc

import (
	"context"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ValidateAPIKey validates a raw API key by delegating to the APIKeyService.
// On success it returns the owner, tenant, and scopes associated with the key.
// The IP address field is left empty because gRPC calls do not carry the
// original client IP in the same way HTTP requests do; the upstream gateway
// should populate this if needed.
func (s *AegisAuthServer) ValidateAPIKey(ctx context.Context, req *pb.ValidateAPIKeyRequest) (*pb.ValidateAPIKeyResponse, error) {
	if req.GetApiKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	apiKey, err := s.apiKeyService.ValidateAPIKey(ctx, req.GetApiKey(), "")
	if err != nil {
		s.logger.Warn("ValidateAPIKey: validation failed",
			log.String("error", err.Error()),
		)
		return &pb.ValidateAPIKeyResponse{
			Valid: false,
		}, nil
	}

	return &pb.ValidateAPIKeyResponse{
		Valid:    true,
		OwnerId:  apiKey.UserID.String(),
		TenantId: apiKey.TenantID.String(),
		Scopes:   apiKey.Scopes,
	}, nil
}
