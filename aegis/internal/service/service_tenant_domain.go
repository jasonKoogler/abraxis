package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonKoogler/aegis/internal/common/log"
	"github.com/jasonKoogler/aegis/internal/domain"
	"github.com/jasonKoogler/aegis/internal/ports"
)

// TenantDomainService handles custom domain management for tenants
type TenantDomainService struct {
	tenantRepo   ports.TenantRepository
	auditService *AuditService
	logger       *log.Logger
}

// NewTenantDomainService creates a new tenant domain service
func NewTenantDomainService(
	tenantRepo ports.TenantRepository,
	auditService *AuditService,
	logger *log.Logger,
) *TenantDomainService {
	return &TenantDomainService{
		tenantRepo:   tenantRepo,
		auditService: auditService,
		logger:       logger,
	}
}

// DomainVerificationStatus represents the status of domain verification
type DomainVerificationStatus string

const (
	// DomainPending indicates the domain is pending verification
	DomainPending DomainVerificationStatus = "pending"
	// DomainVerified indicates the domain is verified
	DomainVerified DomainVerificationStatus = "verified"
	// DomainFailed indicates domain verification failed
	DomainFailed DomainVerificationStatus = "failed"
)

// VerificationMethod defines the method used for domain verification
type VerificationMethod string

const (
	// VerificationMethodDNS verifies domain ownership via DNS TXT record
	VerificationMethodDNS VerificationMethod = "dns"
	// VerificationMethodHTTP verifies domain ownership via HTTP file
	VerificationMethodHTTP VerificationMethod = "http"
)

// TenantDomain represents a custom domain for a tenant
type TenantDomain struct {
	ID                uuid.UUID                `json:"id"`
	TenantID          uuid.UUID                `json:"tenant_id"`
	Domain            string                   `json:"domain"`
	VerificationCode  string                   `json:"verification_code,omitempty"`
	VerificationToken string                   `json:"verification_token,omitempty"`
	Status            DomainVerificationStatus `json:"status"`
	VerifiedAt        *time.Time               `json:"verified_at,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// AddDomainParams contains parameters for adding a custom domain
type AddDomainParams struct {
	TenantID           uuid.UUID
	Domain             string
	VerificationMethod VerificationMethod
}

// AddDomain adds a custom domain for a tenant
func (s *TenantDomainService) AddDomain(ctx context.Context, params *AddDomainParams, actorID uuid.UUID, actorType string) (*TenantDomain, error) {
	// Validate inputs
	if params.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// Default to DNS verification if not specified
	if params.VerificationMethod == "" {
		params.VerificationMethod = VerificationMethodDNS
	}

	// Normalize domain (remove protocol, path, etc.)
	normalizedDomain, err := s.normalizeDomain(params.Domain)
	if err != nil {
		return nil, err
	}

	// Check if tenant exists
	tenant, err := s.tenantRepo.GetByID(ctx, params.TenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Check if domain is already in use
	existingTenant, err := s.tenantRepo.GetByDomain(ctx, normalizedDomain)
	if err == nil && existingTenant != nil && existingTenant.ID != params.TenantID {
		return nil, fmt.Errorf("domain '%s' is already in use by another tenant", normalizedDomain)
	}

	// Generate verification code and token
	verificationCode := s.generateVerificationCode()
	verificationToken := s.generateVerificationToken()

	// Create domain record
	domain := &TenantDomain{
		ID:                uuid.New(),
		TenantID:          params.TenantID,
		Domain:            normalizedDomain,
		VerificationCode:  verificationCode,
		VerificationToken: verificationToken,
		Status:            DomainPending,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Update tenant with the domain
	tenant.Domain = normalizedDomain
	_, err = s.tenantRepo.Update(ctx, tenant.ID, tenant)
	if err != nil {
		return nil, fmt.Errorf("failed to update tenant with domain: %w", err)
	}

	// Log the event
	eventData := map[string]interface{}{
		"domain":              normalizedDomain,
		"verification_method": string(params.VerificationMethod),
	}

	s.auditService.LogEvent(
		ctx,
		"tenant.domain.added",
		actorType,
		actorID,
		&params.TenantID,
		"tenant",
		&params.TenantID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Custom domain added for tenant",
		log.String("tenant_id", params.TenantID.String()),
		log.String("domain", normalizedDomain),
		log.String("verification_method", string(params.VerificationMethod)))

	return domain, nil
}

// VerifyDomainParams contains parameters for verifying a domain
type VerifyDomainParams struct {
	TenantID           uuid.UUID
	Domain             string
	VerificationMethod VerificationMethod
}

// VerifyDomain verifies a tenant's custom domain
func (s *TenantDomainService) VerifyDomain(ctx context.Context, params *VerifyDomainParams, actorID uuid.UUID, actorType string) error {
	// Default to DNS verification if not specified
	if params.VerificationMethod == "" {
		params.VerificationMethod = VerificationMethodDNS
	}

	// Normalize domain
	normalizedDomain, err := s.normalizeDomain(params.Domain)
	if err != nil {
		return err
	}

	// Check if tenant exists and has this domain
	tenant, err := s.tenantRepo.GetByID(ctx, params.TenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	if tenant.Domain != normalizedDomain {
		return fmt.Errorf("domain '%s' is not registered for this tenant", normalizedDomain)
	}

	// Get domain info (in a real implementation, this would come from a domain repository)
	domainInfo := &TenantDomain{
		TenantID:         params.TenantID,
		Domain:           normalizedDomain,
		VerificationCode: "verify-12345678", // This would come from the repository
	}

	// Perform verification based on method
	var verified bool
	switch params.VerificationMethod {
	case VerificationMethodDNS:
		verified, err = s.verifyDNS(normalizedDomain, domainInfo.VerificationCode)
	case VerificationMethodHTTP:
		verified, err = s.verifyHTTP(normalizedDomain, domainInfo.VerificationToken)
	default:
		return fmt.Errorf("unsupported verification method: %s", params.VerificationMethod)
	}

	if err != nil {
		return fmt.Errorf("domain verification failed: %w", err)
	}

	if !verified {
		return fmt.Errorf("domain verification failed: verification records not properly configured")
	}

	// In a real implementation, this would update a domain record in the database
	// with verification status and timestamp
	now := time.Now()

	// Update tenant status (in a real implementation)
	// domainInfo.Status = DomainVerified
	// domainInfo.VerifiedAt = &now
	// repository.Update(domainInfo)

	// Log the event
	eventData := map[string]interface{}{
		"domain":              normalizedDomain,
		"status":              string(DomainVerified),
		"verification_method": string(params.VerificationMethod),
		"verified_at":         now.Format(time.RFC3339),
	}

	s.auditService.LogEvent(
		ctx,
		"tenant.domain.verified",
		actorType,
		actorID,
		&params.TenantID,
		"tenant",
		&params.TenantID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Custom domain verified for tenant",
		log.String("tenant_id", params.TenantID.String()),
		log.String("domain", normalizedDomain),
		log.String("verification_method", string(params.VerificationMethod)))

	return nil
}

// RemoveDomain removes a custom domain from a tenant
func (s *TenantDomainService) RemoveDomain(ctx context.Context, tenantID uuid.UUID, actorID uuid.UUID, actorType string) error {
	// Check if tenant exists
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	if tenant.Domain == "" {
		return fmt.Errorf("tenant does not have a custom domain")
	}

	domainToRemove := tenant.Domain

	// Update tenant to remove domain
	tenant.Domain = ""
	_, err = s.tenantRepo.Update(ctx, tenant.ID, tenant)
	if err != nil {
		return fmt.Errorf("failed to remove domain from tenant: %w", err)
	}

	// Log the event
	eventData := map[string]interface{}{
		"domain": domainToRemove,
	}

	s.auditService.LogEvent(
		ctx,
		"tenant.domain.removed",
		actorType,
		actorID,
		&tenantID,
		"tenant",
		&tenantID,
		nil, // IP address not available here
		"",  // User agent not available here
		eventData,
	)

	s.logger.Info("Custom domain removed from tenant",
		log.String("tenant_id", tenantID.String()),
		log.String("domain", domainToRemove))

	return nil
}

// GetDomainInfo gets information about a tenant's custom domain
func (s *TenantDomainService) GetDomainInfo(ctx context.Context, tenantID uuid.UUID) (*TenantDomain, error) {
	// Check if tenant exists
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	if tenant.Domain == "" {
		return nil, fmt.Errorf("tenant does not have a custom domain")
	}

	// For now, we're just returning basic info since we don't have a separate repository for domains
	// In a real implementation, you would fetch this from a domain repository
	return &TenantDomain{
		TenantID: tenantID,
		Domain:   tenant.Domain,
		// Other fields would be populated from the repository
	}, nil
}

// GetTenantByDomain gets a tenant by its custom domain
func (s *TenantDomainService) GetTenantByDomain(ctx context.Context, domain string) (*domain.Tenant, error) {
	// Normalize domain
	normalizedDomain, err := s.normalizeDomain(domain)
	if err != nil {
		return nil, err
	}

	return s.tenantRepo.GetByDomain(ctx, normalizedDomain)
}

// Helper methods

// normalizeDomain normalizes a domain by removing protocol, path, etc.
func (s *TenantDomainService) normalizeDomain(domain string) (string, error) {
	// Remove protocol if present
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")

	// Remove path and query parameters
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	// Remove port if present
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}

	// Convert to lowercase
	domain = strings.ToLower(domain)

	// Validate domain format
	if !s.isValidDomain(domain) {
		return "", fmt.Errorf("invalid domain format: %s", domain)
	}

	return domain, nil
}

// isValidDomain checks if a domain has a valid format
func (s *TenantDomainService) isValidDomain(domain string) bool {
	// More comprehensive domain validation
	if len(domain) > 255 {
		return false
	}

	// Must have at least one dot and no spaces
	if !strings.Contains(domain, ".") || strings.Contains(domain, " ") {
		return false
	}

	// Check for valid characters (alphanumeric, hyphen, dot)
	for _, char := range domain {
		if (char < 'a' || char > 'z') &&
			(char < '0' || char > '9') &&
			char != '.' && char != '-' {
			return false
		}
	}

	// Domain cannot start or end with hyphen or dot
	if strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") ||
		strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	// Check that each label is valid (between dots)
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
	}

	return true
}

// generateVerificationCode generates a random verification code for domain verification
func (s *TenantDomainService) generateVerificationCode() string {
	// Generate a secure random string for DNS TXT record verification
	prefix := "gauth-verify-"
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to UUID if crypto/rand fails
		return fmt.Sprintf("%s%s", prefix, uuid.New().String()[:16])
	}

	// Encode as base64 and remove non-alphanumeric characters
	code := base64.RawURLEncoding.EncodeToString(randomBytes)
	return fmt.Sprintf("%s%s", prefix, code)
}

// generateVerificationToken generates a random token for HTTP verification
func (s *TenantDomainService) generateVerificationToken() string {
	// Generate a secure random string for HTTP file verification
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to UUID if crypto/rand fails
		return uuid.New().String()
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes)
}

// verifyDNS verifies that DNS records are properly configured for the domain
func (s *TenantDomainService) verifyDNS(domain string, verificationCode string) (bool, error) {
	// Look up TXT records for the domain
	txtRecords, err := net.LookupTXT(domain)
	if err != nil {
		// Try with www prefix if the main domain fails
		if !strings.HasPrefix(domain, "www.") {
			txtRecords, err = net.LookupTXT("www." + domain)
			if err != nil {
				return false, fmt.Errorf("DNS TXT lookup failed: %w", err)
			}
		} else {
			return false, fmt.Errorf("DNS TXT lookup failed: %w", err)
		}
	}

	// Check if any TXT record matches our verification code
	for _, txt := range txtRecords {
		if txt == verificationCode {
			return true, nil
		}
	}

	// Also check for CNAME records to verify domain ownership
	cname, err := net.LookupCNAME(domain)
	if err == nil && strings.Contains(cname, "gauth.example.com") {
		return true, nil
	}

	// Check MX records as another verification method
	mxRecords, err := net.LookupMX(domain)
	if err == nil {
		for _, mx := range mxRecords {
			if strings.Contains(mx.Host, "gauth.example.com") {
				return true, nil
			}
		}
	}

	return false, fmt.Errorf("verification code not found in DNS records")
}

// verifyHTTP verifies domain ownership via HTTP file
func (s *TenantDomainService) verifyHTTP(domain string, verificationToken string) (bool, error) {
	// In a real implementation, this would make an HTTP request to the domain
	// to verify that a specific file with the verification token exists

	// For now, we'll just simulate success
	s.logger.Info("HTTP verification not implemented, simulating success",
		log.String("domain", domain))

	return true, nil
}
