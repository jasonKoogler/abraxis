// Package config provides the configuration for the authn/authz service
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type EnvironmentType string

const (
	Development EnvironmentType = "development"
	Staging     EnvironmentType = "staging"
	Production  EnvironmentType = "production"
)

func (e EnvironmentType) String() string {
	return string(e)
}

func IsValidEnvironmentType(env string) bool {
	switch EnvironmentType(env) {
	case Development, Staging, Production:
		return true
	default:
		return false
	}
}

// Config represents the entire application configuration
type Config struct {
	Environment         EnvironmentType
	LogLevel            LogLevel
	UseRedisRateLimiter bool
	APIKeys             map[string]string
	// FeatureFlags        FeatureFlags
	Timeouts Timeouts
	Postgres PostgresConfig
	// Redpanda            RedpandaConfig
	Redis      RedisConfig
	HTTPServer HTTPServerConfig
	RateLimit  RateLimitConfig
	Auth       AuthConfig
	// Billing             BillingConfig
	// Email               EmailConfig
	Analytics      AnalyticsConfig
	Services       []ServiceConfig
	CircuitBreaker *CircuitBreakerConfig
	Aegis          AegisConfig
}

// AegisConfig holds configuration for the Aegis gRPC client and JWKS-based
// JWT validation.
type AegisConfig struct {
	GRPCAddress         string
	SyncEnabled         bool
	CacheTTL            time.Duration
	MaxBackoff          time.Duration
	JWKSURL             string
	JWKSRefreshInterval time.Duration
}

type AnalyticsConfig struct {
	APIKey          string
	Enabled         bool
	Environment     string
	BatchSize       int
	FlushInterval   time.Duration
	MaxRetries      int
	EndpointURL     string
	DebugMode       bool
	SamplingRate    float64 // Value between 0 and 1
	AllowedDomains  []string
	ExcludedPaths   []string
	CustomDimension map[string]string
}

// type EmailConfig struct {
// 	Provider string // sendgrid/smtp
// 	// SMTP/Provider configuration
// 	SMTPHost     string
// 	SMTPPort     int
// 	SMTPUsername string
// 	SMTPPassword string

// 	// Template configuration
// 	TemplatesDir string

// 	// Service configuration
// 	MaxRetries  int
// 	Environment string // e.g., "development", "production"

// 	// Optional: Rate limiting
// 	RateLimit int // emails per minute

// 	// Optional: External service integration (choose one based on your provider)
// 	SendGridAPIKey string
// 	// PostmarkAPIKey string
// 	// AWSSESConfig   *aws.Config
// }

type AuthConfig struct {
	AuthN AuthNConfig
	AuthZ AuthZConfig
}

type AuthNConfig struct {
	RedisConfig RedisConfig

	SessionManager string // redis/memory

	UseCustomJWT bool
	JWTSecret    string
	JWTIssuer    string

	AccessTokenExpiration  time.Duration
	RefreshTokenExpiration time.Duration
	// SessionKeyExpiration   time.Duration
	TokenRotationInterval time.Duration

	OAuthConfig OAuthConfig
}

type OAuthConfig struct {
	VerifierStorage string
	Providers       []Oauth2ProviderConfig
}

type AuthZConfig struct {
	PolicyFilePath string `env:"AUTHZ_POLICY_FILE_PATH" envDefault:"policies/gauth_policy.rego"`
	WebhookPath    string `env:"AUTHZ_WEBHOOK_PATH" envDefault:"/webhooks/authz/policy"`
	GitHubToken    string `env:"AUTHZ_GITHUB_TOKEN" envDefault:""`
	EnableWebhook  bool   `env:"AUTHZ_ENABLE_WEBHOOK" envDefault:"false"`
}

// Add new Oauth2ProviderConfig type that matches the oauth package
type Oauth2ProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// LogLevel represents the logging level
type LogLevel string

// Available log levels
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

func (l LogLevel) String() string {
	return string(l)
}

// // FeatureFlags represents feature toggle configurations
// type FeatureFlags struct {
// 	EnableCaching      bool
// 	EnableRateLimiting bool
// 	// UseAsyncProcessing    bool
// 	// EnableDetailedMetrics bool
// 	// EnableBetaAPIs        bool
// 	// UseSecondaryDatabase  bool
// }

// Timeouts represents various timeout configurations
type Timeouts struct {
	DatabaseQuery time.Duration
	HTTPRequest   time.Duration
	CacheExpiry   time.Duration
}

// PostgresConfig represents PostgreSQL database configuration
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
	SSLMode  string
	Timezone string
	Timeout  string
}

// // RedpandaConfig represents Redpanda configuration
// type RedpandaConfig struct {
// 	Brokers []string
// 	Group   string
// 	Topic   string
// }

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	Username string
}

type MemcachedConfig struct {
	Host     string
	Port     string
	Password string
	Username string
}

// CORSConfig represents CORS configuration
type corsConfig struct {
	AllowedOrigins string
}

// HTTPServerConfig represents HTTP server configuration
type HTTPServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	CORS            corsConfig
}

type RateLimitConfig struct {
	RequestsPerSecond int
	Burst             int
	TTL               time.Duration
}

// ServiceConfig holds configuration for a microservice
type ServiceConfig struct {
	Name            string        `yaml:"name"`
	URL             string        `yaml:"url"`
	HealthCheckPath string        `yaml:"health_check_path"`
	RequiresAuth    bool          `yaml:"requires_auth"`
	Timeout         time.Duration `yaml:"timeout"`
	RetryCount      int           `yaml:"retry_count"`
	AllowedRoles    []string      `yaml:"allowed_roles"`
	AllowedMethods  []string      `yaml:"allowed_methods"`
	Routes          []RouteConfig `yaml:"routes,omitempty"` // Optional custom routes
}

// RouteConfig defines a custom route for a service
type RouteConfig struct {
	Path           string   `yaml:"path"`            // Path pattern (e.g., "/api/users/:id")
	Method         string   `yaml:"method"`          // HTTP method (GET, POST, etc.), or * for any
	Public         bool     `yaml:"public"`          // Whether this route is public (no auth required)
	RequiredScopes []string `yaml:"required_scopes"` // Required permission scopes (if any)
	Priority       int      `yaml:"priority"`        // Higher priority routes are matched first
}

// type StripeConfig struct {
// 	APIKey              string
// 	EndpointSecret      string
// 	StripeWebhookSecret string
// }

// type BillingConfig struct {
// 	StripeConfig StripeConfig
// }

// LoadConfig loads the configuration from files and environment variables
func LoadConfig() (*Config, error) {
	// rate limit
	useRedisRateLimiter, err := strconv.ParseBool(os.Getenv("USE_REDIS_RATE_LIMITER"))
	if err != nil {
		return nil, fmt.Errorf("error parsing USE_REDIS_RATE_LIMITER: %w", err)
	}
	requestsPerSecond, err := strconv.ParseInt(os.Getenv("RATE_LIMIT_REQUESTS_PER_SECOND"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing RATE_LIMIT_REQUESTS_PER_SECOND: %w", err)
	}
	burst, err := strconv.ParseInt(os.Getenv("RATE_LIMIT_BURST"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing RATE_LIMIT_BURST: %w", err)
	}
	ttl, err := time.ParseDuration(os.Getenv("RATE_LIMIT_TTL"))
	if err != nil {
		return nil, fmt.Errorf("error parsing RATE_LIMIT_TTL: %w", err)
	}

	// auth - JWT validation config (token issuance handled by Aegis)
	sessionManager := os.Getenv("SESSION_MANAGER")
	if sessionManager == "" {
		sessionManager = "redis" // default
	}

	useCustomJWT, _ := strconv.ParseBool(os.Getenv("USE_CUSTOM_JWT"))

	accessTokenExpiration, _ := time.ParseDuration(os.Getenv("ACCESS_TOKEN_EXPIRATION"))
	if accessTokenExpiration == 0 {
		accessTokenExpiration = 15 * time.Minute
	}

	refreshTokenExpiration, _ := time.ParseDuration(os.Getenv("REFRESH_TOKEN_EXPIRATION"))
	if refreshTokenExpiration == 0 {
		refreshTokenExpiration = 24 * time.Hour
	}

	tokenRotationInterval, _ := time.ParseDuration(os.Getenv("TOKEN_ROTATION_INTERVAL"))
	if tokenRotationInterval == 0 {
		tokenRotationInterval = 7 * 24 * time.Hour
	}

	// http server
	readTimeout, err := time.ParseDuration(os.Getenv("HTTP_SERVER_READ_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTP_SERVER_READ_TIMEOUT: %w", err)
	}
	writeTimeout, err := time.ParseDuration(os.Getenv("HTTP_SERVER_WRITE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTP_SERVER_WRITE_TIMEOUT: %w", err)
	}
	idleTimeout, err := time.ParseDuration(os.Getenv("HTTP_SERVER_IDLE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTP_SERVER_IDLE_TIMEOUT: %w", err)
	}
	shutdownTimeout, err := time.ParseDuration(os.Getenv("HTTP_SERVER_SHUTDOWN_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTP_SERVER_SHUTDOWN_TIMEOUT: %w", err)
	}

	// email
	// smtpHost := os.Getenv("EMAIL_SMTP_HOST")
	// smtpPort, err := strconv.Atoi(os.Getenv("EMAIL_SMTP_PORT"))
	// if err != nil {
	// 	return nil, fmt.Errorf("error parsing EMAIL_SMTP_PORT: %w", err)
	// }
	// smtpUsername := os.Getenv("EMAIL_SMTP_USERNAME")
	// smtpPassword := os.Getenv("EMAIL_SMTP_PASSWORD")
	// templatesDir := os.Getenv("EMAIL_TEMPLATES_DIR")
	// emailMaxRetries, err := strconv.Atoi(os.Getenv("EMAIL_MAX_RETRIES"))
	// if err != nil {
	// 	return nil, fmt.Errorf("error parsing EMAIL_MAX_RETRIES: %w", err)
	// }

	cfg := &Config{
		Environment:         EnvironmentType(os.Getenv("ENV")),
		LogLevel:            LogLevel(os.Getenv("LOG_LEVEL")),
		UseRedisRateLimiter: useRedisRateLimiter,
		APIKeys:             map[string]string{},
		// FeatureFlags:        FeatureFlags{},
		Timeouts: Timeouts{},

		Postgres: PostgresConfig{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			DB:       os.Getenv("POSTGRES_DB"),
			SSLMode:  os.Getenv("POSTGRES_SSL_MODE"),
			Timezone: os.Getenv("POSTGRES_TIMEZONE"),
			Timeout:  os.Getenv("POSTGRES_TIMEOUT"),
		},
		// Redpanda: RedpandaConfig{
		// 	Brokers: strings.Split(os.Getenv("REDPANDA_BROKERS"), ","),
		// 	Group:   os.Getenv("REDPANDA_GROUP"),
		// 	Topic:   os.Getenv("REDPANDA_TOPIC"),
		// },

		HTTPServer: HTTPServerConfig{
			Port:            os.Getenv("HTTP_SERVER_PORT"),
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
			CORS: corsConfig{
				AllowedOrigins: os.Getenv("CORS_ALLOWED_ORIGINS"),
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: int(requestsPerSecond),
			Burst:             int(burst),
			TTL:               ttl,
		},
		// Billing: BillingConfig{
		// 	StripeConfig: StripeConfig{
		// 		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		// 		APIKey:              os.Getenv("STRIPE_API_KEY"),
		// 		EndpointSecret:      os.Getenv("STRIPE_ENDPOINT_SECRET"),
		// 	},
		// },
		Auth: AuthConfig{
			AuthN: AuthNConfig{
				RedisConfig: RedisConfig{
					Host:     os.Getenv("REDIS_HOST"),
					Port:     os.Getenv("REDIS_PORT"),
					Password: os.Getenv("REDIS_PASSWORD"),
					Username: os.Getenv("REDIS_USERNAME"),
				},
				SessionManager:         sessionManager,
				UseCustomJWT:           useCustomJWT,
				JWTSecret:              os.Getenv("JWT_SECRET"),
				JWTIssuer:              os.Getenv("JWT_ISSUER"),
				AccessTokenExpiration:  accessTokenExpiration,
				RefreshTokenExpiration: refreshTokenExpiration,
				TokenRotationInterval:  tokenRotationInterval,
				OAuthConfig:            OAuthConfig{VerifierStorage: os.Getenv("OAUTH_VERIFIER_STORAGE")},
			},
			AuthZ: AuthZConfig{
				PolicyFilePath: os.Getenv("AUTHZ_POLICY_FILE_PATH"),
				WebhookPath:    os.Getenv("AUTHZ_WEBHOOK_PATH"),
				GitHubToken:    os.Getenv("AUTHZ_GITHUB_TOKEN"),
				EnableWebhook:  os.Getenv("AUTHZ_ENABLE_WEBHOOK") == "true",
			},
		},
		// Email: EmailConfig{
		// 	SMTPHost:     smtpHost,
		// 	SMTPPort:     smtpPort,
		// 	SMTPUsername: smtpUsername,
		// 	SMTPPassword: smtpPassword,
		// 	TemplatesDir: templatesDir,
		// 	MaxRetries:   emailMaxRetries,
		// },
		Services: []ServiceConfig{
			{
				Name:            "authn",
				URL:             os.Getenv("AUTHN_URL"),
				HealthCheckPath: os.Getenv("AUTHN_HEALTH_CHECK_PATH"),
			},
		},
		Analytics: AnalyticsConfig{
			APIKey:          os.Getenv("ANALYTICS_API_KEY"),
			Enabled:         os.Getenv("ANALYTICS_ENABLED") == "true",
			Environment:     os.Getenv("ANALYTICS_ENV"),
			BatchSize:       getEnvInt("ANALYTICS_BATCH_SIZE", 100),
			FlushInterval:   getEnvDuration("ANALYTICS_FLUSH_INTERVAL", "30s"),
			MaxRetries:      getEnvInt("ANALYTICS_MAX_RETRIES", 3),
			EndpointURL:     os.Getenv("ANALYTICS_ENDPOINT_URL"),
			DebugMode:       os.Getenv("ANALYTICS_DEBUG_MODE") == "true",
			SamplingRate:    getEnvFloat("ANALYTICS_SAMPLING_RATE", 1.0),
			AllowedDomains:  strings.Split(os.Getenv("ANALYTICS_ALLOWED_DOMAINS"), ","),
			ExcludedPaths:   strings.Split(os.Getenv("ANALYTICS_EXCLUDED_PATHS"), ","),
			CustomDimension: make(map[string]string),
		},
		CircuitBreaker: &CircuitBreakerConfig{
			Enabled:                  os.Getenv("CIRCUIT_BREAKER_ENABLED") == "true",
			Provider:                 os.Getenv("CIRCUIT_BREAKER_PROVIDER"),
			Threshold:                getEnvInt("CIRCUIT_BREAKER_THRESHOLD", 5),
			Timeout:                  getEnvDuration("CIRCUIT_BREAKER_TIMEOUT", "5s"),
			HalfOpenSuccessThreshold: getEnvInt("CIRCUIT_BREAKER_HALF_OPEN_SUCCESS_THRESHOLD", 3),
			MaxConcurrentRequests:    getEnvInt("CIRCUIT_BREAKER_MAX_CONCURRENT_REQUESTS", 10),
			RequestTimeout:           getEnvDuration("CIRCUIT_BREAKER_REQUEST_TIMEOUT", "1s"),
			Redis:                    nil, // Redis configuration will be set later
		},
		Aegis: AegisConfig{
			GRPCAddress:         getEnvString("AEGIS_GRPC_ADDRESS", "localhost:9090"),
			SyncEnabled:         os.Getenv("AEGIS_SYNC_ENABLED") == "true",
			CacheTTL:            getEnvDuration("AEGIS_CACHE_TTL", "60s"),
			MaxBackoff:          getEnvDuration("AEGIS_RECONNECT_MAX_BACKOFF", "30s"),
			JWKSURL:             getEnvString("AEGIS_JWKS_URL", "http://localhost:8080/.well-known/jwks.json"),
			JWKSRefreshInterval: getEnvDuration("AEGIS_JWKS_REFRESH_INTERVAL", "5m"),
		},
	}

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func getEnvString(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		duration, err := time.ParseDuration(defaultValue)
		if err != nil {
			// If the default value is invalid, return 0 duration
			return 0
		}
		return duration
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		// If the environment value is invalid, fall back to default
		fallback, fallbackErr := time.ParseDuration(defaultValue)
		if fallbackErr != nil {
			return 0
		}
		return fallback
	}
	return duration
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func validateConfig(config *Config) error {
	if !isValidLogLevel(config.LogLevel) {
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}
	if config.Postgres.Host == "" {
		return fmt.Errorf("postgres host is required")
	}
	// if len(config.Redpanda.Brokers) == 0 {
	// 	return fmt.Errorf("at least one Redpanda broker is required")
	// }
	// if config.Redpanda.Topic == "" {
	// 	return fmt.Errorf("redpanda topic is required")
	// }
	if config.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}
	if config.Redis.Port == "" {
		return fmt.Errorf("redis port is required")
	}
	if config.Redis.Password == "" {
		return fmt.Errorf("redis password is required")
	}
	if config.Redis.Username == "" {
		return fmt.Errorf("redis username is required")
	}
	if config.HTTPServer.Port == "" {
		return fmt.Errorf("http server port is required")
	}

	return nil
}

func isValidLogLevel(level LogLevel) bool {
	validLevels := []LogLevel{LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal}
	for _, l := range validLevels {
		if level == l {
			return true
		}
	}
	return false
}

func isValidSessionManager(sessionManager string) bool {
	validSessionManagers := []string{"redis", "memory"}
	for _, sm := range validSessionManagers {
		if sessionManager == sm {
			return true
		}
	}
	return false
}

// GetProjectRoot returns the root directory of the project
func GetProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error getting current directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return "", fmt.Errorf("could not find project root (go.mod file)")
		}
		currentDir = parentDir
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled                  bool                       `yaml:"enabled"`
	Provider                 string                     `yaml:"provider"` // memory or redis
	Threshold                int                        `yaml:"threshold"`
	Timeout                  time.Duration              `yaml:"timeout"`
	HalfOpenSuccessThreshold int                        `yaml:"half_open_success_threshold"`
	MaxConcurrentRequests    int                        `yaml:"max_concurrent_requests"`
	RequestTimeout           time.Duration              `yaml:"request_timeout"`
	Redis                    *CircuitBreakerRedisConfig `yaml:"redis,omitempty"`
}

// CircuitBreakerRedisConfig holds Redis-specific circuit breaker config
type CircuitBreakerRedisConfig struct {
	Address  string        `yaml:"address"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	CacheTTL time.Duration `yaml:"cache_ttl"`
}
