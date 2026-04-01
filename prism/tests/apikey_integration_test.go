package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jasonKoogler/abraxis/tests/testutil"
)

// apiKeyCreateRequest matches dto.ApiKeyCreateRequest.
type apiKeyCreateRequest struct {
	Name          string     `json:"name"`
	Scopes        []string   `json:"scopes"`
	ExpiresInDays *int       `json:"expires_in_days,omitempty"`
	TenantID      *uuid.UUID `json:"tenant_id,omitempty"`
	UserID        *uuid.UUID `json:"user_id,omitempty"`
}

// apiKeyCreateResponse matches dto.ApiKeyCreateResponse.
type apiKeyCreateResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	KeyPrefix string   `json:"key_prefix"`
	RawAPIKey string   `json:"raw_api_key"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
	CreatedAt string   `json:"created_at"`
}

// apiKeyResponse matches the JSON from domain.APIKey (via api.Respond).
type apiKeyResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	KeyPrefix string   `json:"key_prefix"`
	Scopes    []string `json:"scopes"`
	IsActive  bool     `json:"is_active"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// apiKeyValidateRequest matches dto.ApiKeyValidateRequest.
type apiKeyValidateRequest struct {
	ApiKey string `json:"api_key"`
}

// updateApiKeyMetadataRequest matches dto.UpdateApiKeyMetadataRequest.
type updateApiKeyMetadataRequest struct {
	Name          *string   `json:"name,omitempty"`
	Scopes        *[]string `json:"scopes,omitempty"`
	IsActive      *bool     `json:"is_active,omitempty"`
	ExpiresInDays *int      `json:"expires_in_days,omitempty"`
}

func migrationsPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../deploy/migrations")
	if err != nil {
		t.Fatalf("failed to resolve migrations path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("migrations directory not found at %s: %v", abs, err)
	}
	return abs
}

// seedTenant inserts a test tenant into the database and returns its UUID.
func seedTenant(t *testing.T, pg *testutil.PostgresContainer) uuid.UUID {
	t.Helper()
	tenantID := uuid.New()
	_, err := pg.Pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, status) VALUES ($1, $2, 'active')`,
		tenantID, "test-tenant",
	)
	require.NoError(t, err, "failed to seed tenant")
	return tenantID
}

func TestApiKeyLifecycle(t *testing.T) {
	pg := testutil.SetupPostgres(t, migrationsPath(t))
	rd := testutil.SetupRedis(t)
	server := StartPrismServer(t, pg, rd)

	tenantID := seedTenant(t, pg)

	var createdKeyID string
	var rawApiKey string

	t.Run("create_api_key", func(t *testing.T) {
		days := 30
		body := apiKeyCreateRequest{
			Name:          "test-key",
			Scopes:        []string{"read:all"},
			ExpiresInDays: &days,
			TenantID:      &tenantID,
		}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		resp, err := http.Post(server.BaseURL+"/apikey", "application/json", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusCreated, resp.StatusCode, "unexpected status: %s", string(respBody))

		var created apiKeyCreateResponse
		err = json.Unmarshal(respBody, &created)
		require.NoError(t, err)

		require.NotEmpty(t, created.ID, "ID should be returned")
		require.NotEmpty(t, created.RawAPIKey, "raw API key should be returned on create")
		require.Equal(t, "test-key", created.Name)
		require.Equal(t, []string{"read:all"}, created.Scopes)
		require.NotEmpty(t, created.KeyPrefix)
		require.Contains(t, created.RawAPIKey, "ak_", "raw key should have ak_ prefix")
		require.NotEmpty(t, created.ExpiresAt)

		createdKeyID = created.ID
		rawApiKey = created.RawAPIKey
	})

	t.Run("get_api_key", func(t *testing.T) {
		require.NotEmpty(t, createdKeyID, "create test must run first")

		resp, err := http.Get(server.BaseURL + "/apikey/" + createdKeyID)
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(respBody))

		var key apiKeyResponse
		err = json.Unmarshal(respBody, &key)
		require.NoError(t, err)

		require.Equal(t, "test-key", key.Name)
		require.Equal(t, []string{"read:all"}, key.Scopes)
		require.True(t, key.IsActive)
	})

	t.Run("list_api_keys", func(t *testing.T) {
		require.NotEmpty(t, createdKeyID, "create test must run first")

		// List by tenant_id using the prefixed format expected by the converter.
		resp, err := http.Get(fmt.Sprintf("%s/apikey?tenant_id=tnt_%s", server.BaseURL, tenantID.String()))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(respBody))

		var keys []apiKeyResponse
		err = json.Unmarshal(respBody, &keys)
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(keys), 1, "should list at least one key")
	})

	t.Run("validate_api_key", func(t *testing.T) {
		require.NotEmpty(t, rawApiKey, "create test must run first")

		body := apiKeyValidateRequest{ApiKey: rawApiKey}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		resp, err := http.Post(server.BaseURL+"/apikey/validate", "application/json", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(respBody))

		var key apiKeyResponse
		err = json.Unmarshal(respBody, &key)
		require.NoError(t, err)

		require.Equal(t, "test-key", key.Name)
		require.True(t, key.IsActive)
	})

	t.Run("update_metadata", func(t *testing.T) {
		require.NotEmpty(t, createdKeyID, "create test must run first")

		updatedName := "updated-key-name"
		body := updateApiKeyMetadataRequest{
			Name: &updatedName,
		}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/apikey/%s/metadata", server.BaseURL, createdKeyID), bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(respBody))

		var key apiKeyResponse
		err = json.Unmarshal(respBody, &key)
		require.NoError(t, err)

		require.Equal(t, "updated-key-name", key.Name)
		require.True(t, key.IsActive)
	})

	t.Run("revoke_api_key", func(t *testing.T) {
		require.NotEmpty(t, createdKeyID, "create test must run first")

		req, err := http.NewRequest(http.MethodDelete, server.BaseURL+"/apikey/"+createdKeyID, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusNoContent, resp.StatusCode, "unexpected status: %s", string(respBody))
	})

	t.Run("validate_after_revoke", func(t *testing.T) {
		require.NotEmpty(t, rawApiKey, "create test must run first")

		body := apiKeyValidateRequest{ApiKey: rawApiKey}
		bodyBytes, err := json.Marshal(body)
		require.NoError(t, err)

		resp, err := http.Post(server.BaseURL+"/apikey/validate", "application/json", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		defer resp.Body.Close()

		// After revocation (deletion), the key should not validate successfully.
		require.NotEqual(t, http.StatusOK, resp.StatusCode, "validate should fail after revocation")
	})
}
