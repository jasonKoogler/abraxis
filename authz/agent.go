// Package authz provides a wrapper around the Open Policy Agent (OPA)
// with HTTP middleware and webhook support for policy updates.
package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jasonKoogler/abraxis/authz/cache"
	"github.com/jasonKoogler/abraxis/authz/types"
	"github.com/open-policy-agent/opa/v1/rego"
)

// PolicySource defines how policies are loaded
type PolicySource int

const (
	// PolicySourceLocal loads policies from local modules
	PolicySourceLocal PolicySource = iota
	// PolicySourceExternal loads policies from an external OPA server
	PolicySourceExternal
)

// Config represents the Agent configuration
type Config struct {
	// PolicySource indicates how policies should be loaded and evaluated
	Source PolicySource

	// ExternalOPAURL is the URL of the external OPA server (used when Source is PolicySourceExternal)
	ExternalOPAURL string

	// LocalPolicies contains the Rego policies to use (used when Source is PolicySourceLocal)
	LocalPolicies map[string]string

	// WebhookConfig for policy updates via webhook
	WebhookConfig *WebhookConfig

	// Logger for the agent
	Logger types.Logger

	// Default query path (e.g., "data.authz.allow")
	DefaultQuery string

	// CacheConfig for policy evaluation results
	CacheConfig *cache.CacheConfig

	// RoleProvider is an interface for fetching roles from an external source
	RoleProvider types.RoleProvider

	// ContextTransformers is a list of functions that transform the evaluation context
	ContextTransformers []ContextTransformer
}

// Agent wraps OPA functionality
type Agent struct {
	config  Config
	mutex   sync.RWMutex
	queries map[string]*preparedQuery
	cache   types.Cache
}

type preparedQuery struct {
	query    rego.PreparedEvalQuery
	lastUsed time.Time
}

// ContextTransformer is a function that transforms the evaluation context
type ContextTransformer func(ctx context.Context, input interface{}) (interface{}, error)

// Option is a functional option for configuring the Agent
type Option func(*Config)

// WithLocalPolicies configures the agent to use local policies
func WithLocalPolicies(policies map[string]string) Option {
	return func(c *Config) {
		c.Source = PolicySourceLocal
		c.LocalPolicies = policies
	}
}

// WithExternalOPA configures the agent to use an external OPA server
func WithExternalOPA(url string) Option {
	return func(c *Config) {
		c.Source = PolicySourceExternal
		c.ExternalOPAURL = url
	}
}

// WithDefaultQuery sets the default query path
func WithDefaultQuery(query string) Option {
	return func(c *Config) {
		c.DefaultQuery = query
	}
}

// WithLogger sets the logger for the agent
func WithLogger(logger types.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithMemoryCache enables in-memory caching with the specified TTL and max entries
func WithMemoryCache(ttl time.Duration, maxEntries int) Option {
	return func(c *Config) {
		c.CacheConfig = &cache.CacheConfig{
			Type:       cache.CacheTypeMemory,
			TTL:        ttl,
			MaxEntries: maxEntries,
		}
	}
}

// WithExternalCache configures the agent to use an external cache implementation
func WithExternalCache(cacheImpl types.Cache, ttl time.Duration) Option {
	return func(c *Config) {
		c.CacheConfig = &cache.CacheConfig{
			Type:          cache.CacheTypeExternal,
			TTL:           ttl,
			ExternalCache: cacheImpl,
		}
	}
}

// WithCacheKeyFields specifies which fields in the input should be used for cache key generation
func WithCacheKeyFields(fields []string) Option {
	return func(c *Config) {
		if c.CacheConfig == nil {
			c.CacheConfig = &cache.CacheConfig{
				Type: cache.CacheTypeMemory,
				TTL:  time.Minute * 5,
			}
		}
		c.CacheConfig.CacheKeyFields = fields
	}
}

// WithNoCache disables caching
func WithNoCache() Option {
	return func(c *Config) {
		c.CacheConfig = &cache.CacheConfig{
			Type: cache.CacheTypeNone,
		}
	}
}

// WithWebhook configures the webhook for policy updates
func WithWebhook(endpoint, secret string, allowedSources []string) Option {
	return func(c *Config) {
		c.WebhookConfig = &WebhookConfig{
			Endpoint:       endpoint,
			Secret:         secret,
			AllowedSources: allowedSources,
		}
	}
}

// WithRoleProvider configures the role provider
func WithRoleProvider(provider types.RoleProvider) Option {
	return func(c *Config) {
		c.RoleProvider = provider
	}
}

// WithContextTransformer adds a context transformer
func WithContextTransformer(transformer ContextTransformer) Option {
	return func(c *Config) {
		c.ContextTransformers = append(c.ContextTransformers, transformer)
	}
}

// New creates a new Agent instance using functional options
func New(options ...Option) (*Agent, error) {
	// Default configuration
	config := Config{
		DefaultQuery: "data.authz.allow",
		Logger:       log.New(io.Discard, "", 0),
		CacheConfig:  &cache.CacheConfig{Type: cache.CacheTypeNone},
	}

	// Apply options
	for _, option := range options {
		option(&config)
	}

	// Validate configuration
	if config.Source == PolicySourceExternal && config.ExternalOPAURL == "" {
		return nil, errors.New("external OPA URL is required when using PolicySourceExternal")
	}

	if config.Source == PolicySourceLocal && len(config.LocalPolicies) == 0 {
		return nil, errors.New("local policies are required when using PolicySourceLocal")
	}

	agent := &Agent{
		config:  config,
		queries: make(map[string]*preparedQuery),
	}

	// Initialize cache based on configuration
	var cacheImpl types.Cache
	if config.CacheConfig != nil {
		switch config.CacheConfig.Type {
		case cache.CacheTypeMemory:
			memCache := cache.NewMemoryCache(config.CacheConfig.MaxEntries)
			memCache.StartCleanup(config.CacheConfig.TTL / 2)
			cacheImpl = memCache
		case cache.CacheTypeExternal:
			if config.CacheConfig.ExternalCache == nil {
				return nil, errors.New("external cache implementation is required when using CacheTypeExternal")
			}
			cacheImpl = config.CacheConfig.ExternalCache
		}
		agent.cache = cacheImpl
	}

	// If using local policies, prepare them for evaluation
	if config.Source == PolicySourceLocal {
		if err := agent.prepareLocalPolicies(); err != nil {
			return nil, fmt.Errorf("failed to prepare local policies: %w", err)
		}
	}

	return agent, nil
}

// prepareLocalPolicies prepares the local Rego policies for evaluation
func (a *Agent) prepareLocalPolicies() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Prepare default query
	ctx := context.Background()
	modules := make(map[string]string)
	for name, content := range a.config.LocalPolicies {
		modules[name] = content
	}

	// Create options for rego.New
	opts := []func(*rego.Rego){
		rego.Query(a.config.DefaultQuery),
		rego.Strict(true),
	}

	// Add modules to options
	for name, content := range modules {
		opts = append(opts, rego.Module(name, content))
	}

	// Create rego instance with all options
	r := rego.New(opts...)

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		return err
	}

	a.queries[a.config.DefaultQuery] = &preparedQuery{
		query:    pq,
		lastUsed: time.Now(),
	}

	return nil
}

// Evaluate evaluates a policy decision
func (a *Agent) Evaluate(ctx context.Context, input interface{}) (types.Decision, error) {
	return a.EvaluateQuery(ctx, a.config.DefaultQuery, input)
}

// EvaluateQuery evaluates a specific policy query
func (a *Agent) EvaluateQuery(ctx context.Context, queryPath string, input interface{}) (types.Decision, error) {
	// Apply context transformers if any
	transformedInput := input
	var err error
	for _, transformer := range a.config.ContextTransformers {
		transformedInput, err = transformer(ctx, transformedInput)
		if err != nil {
			return types.Decision{}, fmt.Errorf("context transformation failed: %w", err)
		}
	}

	// Check cache first if enabled
	if a.cache != nil && a.config.CacheConfig.Type != cache.CacheTypeNone {
		key, err := a.generateCacheKey(queryPath, transformedInput)
		if err == nil {
			if decision, found := a.cache.Get(key); found {
				decision.Cached = true
				return decision, nil
			}
		}
	}

	// Based on the policy source, evaluate the query
	var decision types.Decision
	switch a.config.Source {
	case PolicySourceLocal:
		decision, err = a.evaluateLocalPolicy(ctx, queryPath, transformedInput)
	case PolicySourceExternal:
		decision, err = a.evaluateExternalPolicy(ctx, queryPath, transformedInput)
	default:
		return types.Decision{}, errors.New("unknown policy source")
	}

	if err != nil {
		return types.Decision{}, err
	}

	// Set timestamp on the decision
	decision.Timestamp = time.Now()

	// If caching is enabled, store the result
	if a.cache != nil && a.config.CacheConfig.Type != cache.CacheTypeNone {
		if key, err := a.generateCacheKey(queryPath, transformedInput); err == nil {
			a.cache.Set(key, decision, a.config.CacheConfig.TTL)
		}
	}

	return decision, nil
}

// evaluateLocalPolicy evaluates a policy using the local Rego engine
func (a *Agent) evaluateLocalPolicy(ctx context.Context, queryPath string, input interface{}) (types.Decision, error) {
	a.mutex.RLock()
	pq, exists := a.queries[queryPath]
	a.mutex.RUnlock()

	if !exists {
		return types.Decision{}, fmt.Errorf("query %s not prepared", queryPath)
	}

	results, err := pq.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return types.Decision{}, err
	}

	// Update last used time
	a.mutex.Lock()
	pq.lastUsed = time.Now()
	a.mutex.Unlock()

	allowed := false
	var reason string
	if len(results) > 0 && results[0].Expressions != nil && len(results[0].Expressions) > 0 {
		// Check if the result is a boolean
		if val, ok := results[0].Expressions[0].Value.(bool); ok {
			allowed = val
		} else if obj, ok := results[0].Expressions[0].Value.(map[string]interface{}); ok {
			// Handle case where the result is an object with allowed and reason fields
			if allow, ok := obj["allowed"].(bool); ok {
				allowed = allow
			}
			if r, ok := obj["reason"].(string); ok {
				reason = r
			}
		}
	}

	return types.Decision{
		Allowed: allowed,
		Reason:  reason,
	}, nil
}

// evaluateExternalPolicy evaluates a policy using an external OPA server
func (a *Agent) evaluateExternalPolicy(ctx context.Context, queryPath string, input interface{}) (types.Decision, error) {
	// Prepare request to OPA server
	path := queryPath
	if path[0] == 'd' && path[0:4] == "data" {
		// If the path starts with "data.", remove it for the URL
		path = path[5:]
	}

	url := fmt.Sprintf("%s/v1/data/%s", a.config.ExternalOPAURL, path)

	requestBody, err := json.Marshal(map[string]interface{}{
		"input": input,
	})
	if err != nil {
		return types.Decision{}, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return types.Decision{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return types.Decision{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.Decision{}, fmt.Errorf("OPA server returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var result struct {
		Result interface{} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return types.Decision{}, err
	}

	// Handle different response formats
	allowed := false
	var reason string
	if boolResult, ok := result.Result.(bool); ok {
		allowed = boolResult
	} else if objResult, ok := result.Result.(map[string]interface{}); ok {
		if allow, ok := objResult["allowed"].(bool); ok {
			allowed = allow
		}
		if r, ok := objResult["reason"].(string); ok {
			reason = r
		}
	}

	return types.Decision{
		Allowed: allowed,
		Reason:  reason,
	}, nil
}

// UpdatePolicies updates the local policies
func (a *Agent) UpdatePolicies(policies map[string]string) error {
	if a.config.Source != PolicySourceLocal {
		return errors.New("can only update policies when using PolicySourceLocal")
	}

	// Update local policies
	a.config.LocalPolicies = policies

	// Prepare them for evaluation
	if err := a.prepareLocalPolicies(); err != nil {
		return err
	}

	// Clear cache if enabled
	if a.cache != nil && a.config.CacheConfig.Type != cache.CacheTypeNone {
		a.cache.Clear()
	}

	return nil
}

// filterInputFields filters input fields to only include the specified fields
func (a *Agent) filterInputFields(input interface{}, fields []string) interface{} {
	// If input is not a map, return it unchanged
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return input
	}

	// Create a new map with only the specified fields
	filtered := make(map[string]interface{})
	for _, field := range fields {
		// Support for nested fields using dot notation
		parts := splitPath(field)
		value := getNestedValue(inputMap, parts)
		if value != nil {
			setNestedValue(filtered, parts, value)
		}
	}

	return filtered
}

// generateCacheKey generates a cache key for the given query and input
func (a *Agent) generateCacheKey(queryPath string, input interface{}) (string, error) {
	// Filter input fields if specified
	filteredInput := input
	if a.config.CacheConfig != nil && len(a.config.CacheConfig.CacheKeyFields) > 0 {
		filteredInput = a.filterInputFields(input, a.config.CacheConfig.CacheKeyFields)
	}

	// Generate a hash of the filtered input
	key, err := hashInput(queryPath, filteredInput)
	if err != nil {
		return "", err
	}

	return key, nil
}
