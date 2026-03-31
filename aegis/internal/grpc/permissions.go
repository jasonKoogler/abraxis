package grpc

import (
	"context"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CheckPermission evaluates an authorization request against the OPA policy
// engine via the authz adapter. The request fields are assembled into the
// standard OPA input map and forwarded to Evaluate.
func (s *AegisAuthServer) CheckPermission(ctx context.Context, req *pb.CheckPermissionRequest) (*pb.CheckPermissionResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetAction() == "" {
		return nil, status.Error(codes.InvalidArgument, "action is required")
	}

	input := map[string]interface{}{
		"user": map[string]interface{}{
			"id": req.GetUserId(),
		},
		"action": req.GetAction(),
		"resource": map[string]interface{}{
			"type": req.GetResourceType(),
			"id":   req.GetResourceId(),
		},
		"tenant_id": req.GetTenantId(),
	}

	decision, err := s.authzAdapter.Evaluate(ctx, input)
	if err != nil {
		s.logger.Error("CheckPermission: evaluation failed",
			log.String("user_id", req.GetUserId()),
			log.String("action", req.GetAction()),
			log.Error(err),
		)
		return nil, status.Errorf(codes.Internal, "authorization evaluation failed: %v", err)
	}

	return &pb.CheckPermissionResponse{
		Allowed: decision.Allowed,
		Reason:  decision.Reason,
	}, nil
}
