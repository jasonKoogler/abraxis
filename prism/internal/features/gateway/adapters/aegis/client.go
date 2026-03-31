package aegis

import (
	"context"
	"sync/atomic"
	"time"

	pb "github.com/jasonKoogler/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/jasonKoogler/prism/internal/common/redis"
	"github.com/jasonKoogler/prism/internal/features/auth/adapters/authz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client manages the gRPC connection to the Aegis auth service and
// keeps local caches (Redis) and policies (OPA) in sync.
type Client struct {
	conn         *grpc.ClientConn
	client       pb.AegisAuthClient
	logger       *log.Logger
	redisClient  *redis.RedisClient
	authzAdapter *authz.Adapter
	address      string
	cacheTTL     time.Duration
	maxBackoff   time.Duration

	ready atomic.Bool
}

// Config holds configuration for the Aegis gRPC client.
type Config struct {
	Address    string
	CacheTTL   time.Duration
	MaxBackoff time.Duration
}

// NewClient creates a new Aegis gRPC client. Call Start to begin
// background sync goroutines.
func NewClient(
	cfg Config,
	logger *log.Logger,
	redisClient *redis.RedisClient,
	authzAdapter *authz.Adapter,
) *Client {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = 60 * time.Second
	}

	return &Client{
		logger:       logger,
		redisClient:  redisClient,
		authzAdapter: authzAdapter,
		address:      cfg.Address,
		cacheTTL:     cfg.CacheTTL,
		maxBackoff:   cfg.MaxBackoff,
	}
}

// Start dials the Aegis gRPC server and launches background sync loops
// for auth data and policies. It blocks until the initial connection is
// established (or ctx is cancelled).
func (c *Client) Start(ctx context.Context) error {
	conn, err := grpc.NewClient(
		c.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	c.conn = conn
	c.client = pb.NewAegisAuthClient(conn)

	c.logger.Info("aegis gRPC client connected", log.String("address", c.address))

	go c.syncAuthDataLoop(ctx)
	go c.syncPoliciesLoop(ctx)

	return nil
}

// Stop closes the underlying gRPC connection.
func (c *Client) Stop() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsReady returns true after the initial SyncComplete event has been
// received from the auth data stream.
func (c *Client) IsReady() bool {
	return c.ready.Load()
}

// CheckPermission calls the Aegis CheckPermission RPC.
func (c *Client) CheckPermission(
	ctx context.Context,
	userID, action, resourceType, resourceID, tenantID string,
) (*pb.CheckPermissionResponse, error) {
	return c.client.CheckPermission(ctx, &pb.CheckPermissionRequest{
		UserId:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceId:   resourceID,
		TenantId:     tenantID,
	})
}

// ValidateAPIKey calls the Aegis ValidateAPIKey RPC.
func (c *Client) ValidateAPIKey(ctx context.Context, apiKey string) (*pb.ValidateAPIKeyResponse, error) {
	return c.client.ValidateAPIKey(ctx, &pb.ValidateAPIKeyRequest{
		ApiKey: apiKey,
	})
}

// backoff returns an exponential backoff duration capped at maxBackoff.
// attempt is zero-indexed: 0 -> 1s, 1 -> 2s, 2 -> 4s, ...
func (c *Client) backoff(attempt int) time.Duration {
	d := time.Second
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= c.maxBackoff {
			return c.maxBackoff
		}
	}
	return d
}
