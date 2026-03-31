package http

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jasonKoogler/abraxis/aegis/internal/domain"
)

func setAuthHeaders(w http.ResponseWriter, tokenPair *domain.TokenPair, sessionID string) {
	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", tokenPair.AccessToken))
	w.Header().Set("X-Refresh-Token", string(tokenPair.RefreshToken))
	w.Header().Set("X-Session-ID", sessionID)
}

// ExtractTenantIDFromRequest attempts to retrieve the tenant ID from the request.
// It first tries a URL parameter named "tenantID"; if not found, it looks for an "X-Tenant-ID" header.
func ExtractTenantIDFromRequest(r *http.Request) (uuid.UUID, bool) {
	tenantIDStr := chi.URLParam(r, "tenantID")
	if tenantIDStr == "" {
		tenantIDStr = r.Header.Get("X-Tenant-ID")
	}
	if tenantIDStr == "" {
		return uuid.UUID{}, false
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.UUID{}, false
	}
	return tenantID, true
}
