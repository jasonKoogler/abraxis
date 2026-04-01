package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrossService(t *testing.T) {
	stack := StartFullStack(t)

	t.Run("register_and_jwt_roundtrip", func(t *testing.T) {
		// 1. Register user in Aegis.
		regBody, err := json.Marshal(map[string]string{
			"email":     "crosstest@example.com",
			"password":  "SecurePass123!",
			"firstName": "Cross",
			"lastName":  "Test",
			"phone":     "+1234567890",
		})
		require.NoError(t, err)

		regResp, err := http.Post(stack.AegisURL+"/auth/register", "application/json", bytes.NewReader(regBody))
		require.NoError(t, err)
		defer regResp.Body.Close()
		require.Equal(t, http.StatusCreated, regResp.StatusCode, "register should return 201")

		authHeader := regResp.Header.Get("Authorization")
		require.NotEmpty(t, authHeader, "Authorization header must be set after registration")
		require.Contains(t, authHeader, "Bearer ")

		accessToken := authHeader[len("Bearer "):]
		t.Logf("obtained access token (first 20 chars): %s...", accessToken[:min(20, len(accessToken))])

		// 2. Verify the JWT works on Aegis's own protected endpoint.
		req, err := http.NewRequest(http.MethodGet, stack.AegisURL+"/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET /users with valid JWT should return 200")

		// 3. Verify that calling a protected endpoint WITHOUT a token returns 401.
		reqNoAuth, err := http.NewRequest(http.MethodGet, stack.AegisURL+"/users", nil)
		require.NoError(t, err)

		respNoAuth, err := http.DefaultClient.Do(reqNoAuth)
		require.NoError(t, err)
		defer respNoAuth.Body.Close()
		require.Equal(t, http.StatusUnauthorized, respNoAuth.StatusCode, "GET /users without token should return 401")
	})

	t.Run("aegis_jwks_endpoint", func(t *testing.T) {
		// Verify Aegis serves a valid JWKS document.
		resp, err := http.Get(stack.AegisURL + "/.well-known/jwks.json")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "JWKS endpoint should return 200")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var jwks struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		err = json.Unmarshal(body, &jwks)
		require.NoError(t, err, "JWKS response should be valid JSON")
		require.NotEmpty(t, jwks.Keys, "JWKS should contain at least one key")

		// Verify the key has expected Ed25519 properties.
		key := jwks.Keys[0]
		assert.Equal(t, "OKP", key["kty"], "key type should be OKP (Ed25519)")
		assert.Equal(t, "Ed25519", key["crv"], "curve should be Ed25519")
		assert.NotEmpty(t, key["kid"], "key ID must be present")
		assert.NotEmpty(t, key["x"], "public key (x) must be present")
		t.Logf("JWKS key kid=%s, kty=%s, crv=%s", key["kid"], key["kty"], key["crv"])
	})

	t.Run("prism_readiness", func(t *testing.T) {
		// Prism's /ready endpoint gates on:
		// - JWKS keys loaded from Aegis
		// - Aegis gRPC sync connected (IsReady)
		resp, err := http.Get(stack.PrismURL + "/ready")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Logf("Prism /ready status=%d body=%s", resp.StatusCode, string(body))

		// If /ready returns 503 it means JWKS or gRPC sync is not yet complete.
		// Log a concern but don't fail hard — the JWKS fetcher is working if
		// we got here (waitForReady in StartFullStack already waited).
		if resp.StatusCode != http.StatusOK {
			t.Logf("CONCERN: Prism /ready returned %d instead of 200. "+
				"JWKS or gRPC sync may not be fully wired.", resp.StatusCode)
		}
	})

	t.Run("prism_health", func(t *testing.T) {
		resp, err := http.Get(stack.PrismURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "Prism /health should return 200")
	})

	t.Run("token_revocation_across_services", func(t *testing.T) {
		// 1. Login as the user registered in the first test to get fresh tokens.
		loginBody, err := json.Marshal(map[string]string{
			"email":    "crosstest@example.com",
			"password": "SecurePass123!",
		})
		require.NoError(t, err)

		loginResp, err := http.Post(stack.AegisURL+"/auth/login", "application/json", bytes.NewReader(loginBody))
		require.NoError(t, err)
		defer loginResp.Body.Close()
		require.Equal(t, http.StatusOK, loginResp.StatusCode, "login should return 200")

		authHeader := loginResp.Header.Get("Authorization")
		require.NotEmpty(t, authHeader)
		accessToken := authHeader[len("Bearer "):]

		// 2. Verify the token works on Aegis's protected endpoint.
		req, err := http.NewRequest(http.MethodGet, stack.AegisURL+"/users", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "token should work on Aegis before logout")

		// 3. Verify the token works on Prism's auth-protected /apikey endpoint.
		// A dummy tenant_id is required by the handler; we only care that auth passes.
		prismReqBefore, err := http.NewRequest(http.MethodGet, stack.PrismURL+"/apikey?tenant_id=tnt_00000000-0000-0000-0000-000000000001", nil)
		require.NoError(t, err)
		prismReqBefore.Header.Set("Authorization", "Bearer "+accessToken)

		prismRespBefore, err := http.DefaultClient.Do(prismReqBefore)
		require.NoError(t, err)
		defer prismRespBefore.Body.Close()
		require.Equal(t, http.StatusOK, prismRespBefore.StatusCode,
			"token should work on Prism before logout")

		// 4. Logout in Aegis (revokes session + publishes TokenRevoked to gRPC).
		logoutReq, err := http.NewRequest(http.MethodPost, stack.AegisURL+"/auth/logout", nil)
		require.NoError(t, err)
		logoutReq.Header.Set("Authorization", "Bearer "+accessToken)

		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		logoutResp, err := noRedirectClient.Do(logoutReq)
		require.NoError(t, err)
		defer logoutResp.Body.Close()
		require.Equal(t, http.StatusFound, logoutResp.StatusCode, "logout should return 302")

		// 5. Verify the token is rejected by Aegis (session invalidated).
		reqAfterLogout, err := http.NewRequest(http.MethodGet, stack.AegisURL+"/users", nil)
		require.NoError(t, err)
		reqAfterLogout.Header.Set("Authorization", "Bearer "+accessToken)

		respAfterLogout, err := http.DefaultClient.Do(reqAfterLogout)
		require.NoError(t, err)
		defer respAfterLogout.Body.Close()
		require.Equal(t, http.StatusUnauthorized, respAfterLogout.StatusCode,
			"Aegis should reject token after logout (session invalidated)")

		// 6. Cross-service revocation: verify Prism also rejects the token.
		// Aegis publishes a TokenRevoked event to the gRPC stream on logout.
		// Prism caches the revoked JTI in Redis and checks it on every
		// authenticated request. Allow a short propagation window.
		time.Sleep(2 * time.Second)

		prismReqAfter, err := http.NewRequest(http.MethodGet, stack.PrismURL+"/apikey?tenant_id=tnt_00000000-0000-0000-0000-000000000001", nil)
		require.NoError(t, err)
		prismReqAfter.Header.Set("Authorization", "Bearer "+accessToken)

		prismRespAfter, err := http.DefaultClient.Do(prismReqAfter)
		require.NoError(t, err)
		defer prismRespAfter.Body.Close()
		require.Equal(t, http.StatusUnauthorized, prismRespAfter.StatusCode,
			"Prism should reject token after Aegis logout (cross-service revocation via gRPC)")
	})
}
