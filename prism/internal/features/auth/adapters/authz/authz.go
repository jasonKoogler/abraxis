package authz

import (
	"context"
	"fmt"

	"net/http"
	"time"

	"github.com/jasonKoogler/authz"
	"github.com/jasonKoogler/authz/cache"
	"github.com/jasonKoogler/authz/roles"
	"github.com/jasonKoogler/authz/types"
	"github.com/jasonKoogler/prism/internal/common/log"
	"github.com/redis/go-redis/v9"
)

// Config holds the configuration for the Authz adapter
type Config struct {
	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Cache configuration
	CacheTTL     time.Duration
	MaxCacheSize int

	// Policy configuration
	Policies map[string]string

	// Webhook configuration
	WebhookPath   string
	WebhookSecret string

	// Logger
	Logger   *log.Logger
	LogLevel string
}

// Adapter is the Authz adapter
type Adapter struct {
	agent        *authz.Agent
	roleProvider types.RoleProvider
	config       Config
}

// New creates a new Authz adapter
func New(config Config) (*Adapter, error) {
	// Set default logger if not provided
	if config.Logger == nil {
		config.Logger = log.NewLogger(config.LogLevel)
	}

	// Set default policies if not provided
	if config.Policies == nil {
		config.Policies = defaultPolicies()
	}

	// Create options for the Authz agent
	opts := []authz.Option{
		authz.WithLocalPolicies(config.Policies),
		authz.WithLogger(config.Logger),
	}

	// Configure caching
	if config.RedisAddr != "" {
		// Set up Redis client
		redisClient := redis.NewClient(&redis.Options{
			Addr:     config.RedisAddr,
			Password: config.RedisPassword,
			DB:       config.RedisDB,
		})

		// Create Redis cache
		redisCache := cache.NewRedisCache(redisClient,
			cache.WithKeyPrefix("authz:policy:cache:"),
		)

		// Add Redis cache to options
		opts = append(opts, authz.WithExternalCache(redisCache, config.CacheTTL))

		// Create Redis role provider
		roleProvider := roles.NewRedisRoleProvider(redisClient,
			roles.WithKeyPrefix("authz:roles:"),
			roles.WithDefaultTTL(time.Hour*24),
			roles.WithDefaultRoles([]string{"user"}),
		)

		// Create role transformer
		roleTransformer := roles.CreateRoleTransformer(roleProvider)

		// Add role provider and transformer to options
		opts = append(opts,
			authz.WithRoleProvider(roleProvider),
			authz.WithContextTransformer(roleTransformer),
		)
	} else {
		// Use memory cache if Redis is not configured
		opts = append(opts, authz.WithMemoryCache(config.CacheTTL, config.MaxCacheSize))
	}

	// Configure webhook if path is provided
	if config.WebhookPath != "" {
		opts = append(opts, authz.WithWebhook(config.WebhookPath, config.WebhookSecret, nil))
	}

	// Create the Authz agent
	agent, err := authz.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create authz agent: %w", err)
	}

	return &Adapter{
		agent:  agent,
		config: config,
	}, nil
}

// RegisterWebhook registers the webhook handler with the provided HTTP mux
func (a *Adapter) RegisterWebhook(mux *http.ServeMux) {
	if a.config.WebhookPath != "" {
		a.agent.RegisterWebhook(mux)
	}
}

// Middleware returns an HTTP middleware that enforces authorization
func (a *Adapter) Middleware(extractInput func(*http.Request) (interface{}, error)) func(http.Handler) http.Handler {
	return a.agent.Middleware(extractInput)
}

// Evaluate evaluates the authorization policy for the given input
func (a *Adapter) Evaluate(ctx context.Context, input interface{}) (types.Decision, error) {
	return a.agent.Evaluate(ctx, input)
}

// EvaluateQuery evaluates a custom query with the given input
func (a *Adapter) EvaluateQuery(ctx context.Context, query string, input interface{}) (types.Decision, error) {
	return a.agent.EvaluateQuery(ctx, query, input)
}

// UpdatePolicies replaces the local OPA policies with the provided map.
// This delegates to the underlying authz.Agent.UpdatePolicies method.
func (a *Adapter) UpdatePolicies(policies map[string]string) error {
	return a.agent.UpdatePolicies(policies)
}

// defaultPolicies returns the default authorization policies
func defaultPolicies() map[string]string {
	return map[string]string{
		"authz.rego": `
package authz

default allow = false

# Allow access based on roles
allow if {
    # Admin can do anything
    input.user.roles[_] == "admin"
}

allow if {
    # Users can read their own data
    input.user.roles[_] == "user"
    input.action == "read"
    input.resource.owner == input.user.id
}

allow if {
    # Users can read public resources
    input.user.roles[_] == "user"
    input.action == "read"
    input.resource.public == true
}
`,
	}
}
