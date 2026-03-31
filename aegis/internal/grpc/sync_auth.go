package grpc

import (
	"github.com/google/uuid"
	"google.golang.org/grpc"

	pb "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
)

const (
	syncAuthPageSize = 100
)

// SyncAuthData streams a full snapshot of all user-role mappings followed by
// incremental updates. The snapshot is sent as one UserRolesSnapshot event per
// user-tenant pair, terminated by a SyncComplete marker. After the snapshot
// the stream stays open and forwards events from the auth event bus until the
// client disconnects.
func (s *AegisAuthServer) SyncAuthData(req *pb.SyncRequest, stream grpc.ServerStreamingServer[pb.AuthDataEvent]) error {
	ctx := stream.Context()
	subID := uuid.New().String()

	// Subscribe BEFORE sending snapshot so we don't miss events that occur
	// between the snapshot query and the subscription.
	events := s.authEventBus.Subscribe(subID)
	defer s.authEventBus.Unsubscribe(subID)

	// --- Snapshot phase ---
	page := 1
	for {
		users, err := s.userRepo.ListAll(ctx, page, syncAuthPageSize)
		if err != nil {
			s.logger.Error("SyncAuthData: failed to list users", log.Error(err))
			return err
		}

		for _, user := range users {
			roles := user.GetRoles()
			if len(roles) == 0 {
				continue
			}

			for tenantID, role := range roles {
				event := &pb.AuthDataEvent{
					Version: s.authEventBus.NextVersion(),
					Event: &pb.AuthDataEvent_UserRoles{
						UserRoles: &pb.UserRolesSnapshot{
							UserId:   user.ID.String(),
							TenantId: tenantID.String(),
							Roles:    []string{string(role)},
						},
					},
				}
				if err := stream.Send(event); err != nil {
					return err
				}
			}
		}

		if len(users) < syncAuthPageSize {
			break
		}
		page++
	}

	// Send SyncComplete marker.
	if err := stream.Send(&pb.AuthDataEvent{
		Version: s.authEventBus.NextVersion(),
		Event: &pb.AuthDataEvent_SyncComplete{
			SyncComplete: &pb.SyncComplete{},
		},
	}); err != nil {
		return err
	}

	// --- Incremental phase ---
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// PublishUserRolesChanged publishes a UserRolesSnapshot event to all connected
// SyncAuthData streams. Call this from services when a user's roles change.
func (s *AegisAuthServer) PublishUserRolesChanged(userID, tenantID string, roles []string) {
	s.authEventBus.Publish(&pb.AuthDataEvent{
		Event: &pb.AuthDataEvent_UserRoles{
			UserRoles: &pb.UserRolesSnapshot{
				UserId:   userID,
				TenantId: tenantID,
				Roles:    roles,
			},
		},
	})
}

// PublishTokenRevoked publishes a TokenRevoked event to all connected
// SyncAuthData streams. Call this from services when a token is revoked.
func (s *AegisAuthServer) PublishTokenRevoked(jti string, expiresAt int64) {
	s.authEventBus.Publish(&pb.AuthDataEvent{
		Event: &pb.AuthDataEvent_TokenRevoked{
			TokenRevoked: &pb.TokenRevoked{
				Jti:       jti,
				ExpiresAt: expiresAt,
			},
		},
	})
}
