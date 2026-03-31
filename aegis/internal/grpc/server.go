package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/aegis/internal/adapters/authz"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/ports"
	"github.com/jasonKoogler/aegis/internal/service"
)

// AegisAuthServer implements the aegispb.AegisAuthServer interface.
type AegisAuthServer struct {
	pb.UnimplementedAegisAuthServer

	logger         *log.Logger
	userRepo       ports.UserRepository
	apiKeyService  *service.APIKeyService
	authzAdapter   *authz.Adapter
	policyFilePath string

	authEventBus   *AuthEventBus
	policyEventBus *PolicyEventBus
}

// ServerConfig holds the dependencies needed to construct an AegisAuthServer.
type ServerConfig struct {
	Port           string
	Logger         *log.Logger
	UserRepo       ports.UserRepository
	APIKeyService  *service.APIKeyService
	AuthzAdapter   *authz.Adapter
	PolicyFilePath string
}

func NewAegisAuthServer(cfg ServerConfig) *AegisAuthServer {
	return &AegisAuthServer{
		logger:         cfg.Logger,
		userRepo:       cfg.UserRepo,
		apiKeyService:  cfg.APIKeyService,
		authzAdapter:   cfg.AuthzAdapter,
		policyFilePath: cfg.PolicyFilePath,
		authEventBus:   NewAuthEventBus(),
		policyEventBus: NewPolicyEventBus(),
	}
}

// Start binds to the given TCP port and serves gRPC requests.
// This call blocks until the server is stopped or an error occurs.
func (s *AegisAuthServer) Start(port string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAegisAuthServer(grpcServer, s)
	reflection.Register(grpcServer)

	s.logger.Info("gRPC server starting", log.String("port", port))
	return grpcServer.Serve(lis)
}

func (s *AegisAuthServer) GetAuthEventBus() *AuthEventBus {
	return s.authEventBus
}

func (s *AegisAuthServer) GetPolicyEventBus() *PolicyEventBus {
	return s.policyEventBus
}
