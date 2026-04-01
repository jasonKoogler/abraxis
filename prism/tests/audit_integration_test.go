package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/jasonKoogler/abraxis/prism/internal/domain"
	"github.com/jasonKoogler/abraxis/tests/testutil"
)

// auditLogResponse mirrors domain.AuditLog but uses plain strings for ID fields
// so the test can unmarshal the JSON without needing the id.ID interface.
type auditLogResponse struct {
	ID           string         `json:"id"`
	EventType    string         `json:"event_type"`
	ActorType    string         `json:"actor_type"`
	ActorID      string         `json:"actor_id"`
	TenantID     string         `json:"tenant_id"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	IPAddress    string         `json:"ip_address"`
	UserAgent    string         `json:"user_agent"`
	EventData    map[string]any `json:"event_data,omitempty"`
	CreatedAt    string         `json:"created_at"`
}

type auditLogListResponse struct {
	Data       []auditLogResponse         `json:"data"`
	Pagination domain.PaginationMetadata  `json:"pagination"`
}

// seedAuditLogs inserts known audit log entries directly into the audit_logs
// table and returns the generated UUIDs so the get-by-ID test can reference them.
func seedAuditLogs(t *testing.T, pg *testutil.PostgresContainer, tenantID uuid.UUID) []uuid.UUID {
	t.Helper()
	ctx := context.Background()

	type row struct {
		eventType    string
		actorType    string
		actorID      uuid.UUID
		resourceType string
		resourceID   uuid.UUID
		ipAddress    string
		userAgent    string
		eventData    string
	}

	actorID := uuid.New()
	resourceID := uuid.New()

	rows := []row{
		{"user.login", "user", actorID, "session", resourceID, "10.0.0.1", "TestAgent/1.0", `{"method":"password"}`},
		{"user.login", "user", actorID, "session", resourceID, "10.0.0.2", "TestAgent/1.0", `{"method":"sso"}`},
		{"user.login", "user", actorID, "session", resourceID, "10.0.0.3", "TestAgent/1.0", `{"method":"password"}`},
		{"apikey.created", "user", actorID, "apikey", uuid.New(), "10.0.0.1", "TestAgent/2.0", `{"scope":"read:all"}`},
		{"apikey.created", "user", actorID, "apikey", uuid.New(), "10.0.0.1", "TestAgent/2.0", `{"scope":"write:all"}`},
		{"user.logout", "user", actorID, "session", resourceID, "10.0.0.1", "TestAgent/1.0", `{}`},
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for i, r := range rows {
		var id uuid.UUID
		// Stagger created_at so ordering is deterministic.
		createdAt := time.Now().Add(-time.Duration(len(rows)-i) * time.Minute)
		err := pg.Pool.QueryRow(ctx,
			`INSERT INTO audit_logs
				(event_type, actor_type, actor_id, tenant_id, resource_type, resource_id,
				 ip_address, user_agent, event_data, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7::inet,$8,$9::jsonb,$10)
			 RETURNING id`,
			r.eventType, r.actorType, r.actorID, tenantID, r.resourceType, r.resourceID,
			r.ipAddress, r.userAgent, r.eventData, createdAt,
		).Scan(&id)
		require.NoError(t, err, "failed to seed audit log row %d", i)
		ids = append(ids, id)
	}

	return ids
}

func TestAuditLogQueries(t *testing.T) {
	pg := testutil.SetupPostgres(t, migrationsPath(t))
	rd := testutil.SetupRedis(t)
	server := StartPrismServer(t, pg, rd)

	tenantID := seedTenant(t, pg)
	seededIDs := seedAuditLogs(t, pg, tenantID)
	require.Len(t, seededIDs, 6, "expected 6 seeded audit logs")

	t.Run("list_by_event_type", func(t *testing.T) {
		url := fmt.Sprintf("%s/audit?event_type=user.login", server.BaseURL)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		var result auditLogListResponse
		require.NoError(t, json.Unmarshal(body, &result))

		require.Equal(t, 3, len(result.Data), "should have 3 login events")
		require.Equal(t, 3, result.Pagination.TotalItems)
		for _, entry := range result.Data {
			require.Equal(t, "user.login", entry.EventType)
		}
	})

	t.Run("list_by_tenant", func(t *testing.T) {
		url := fmt.Sprintf("%s/audit?tenant_id=tnt_%s", server.BaseURL, tenantID.String())
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		var result auditLogListResponse
		require.NoError(t, json.Unmarshal(body, &result))

		require.Equal(t, 6, len(result.Data), "all 6 logs belong to the seeded tenant")
		require.Equal(t, 6, result.Pagination.TotalItems)
	})

	t.Run("list_no_filter_returns_400", func(t *testing.T) {
		resp, err := http.Get(server.BaseURL + "/audit")
		require.NoError(t, err)
		defer resp.Body.Close()

		// With no filters provided, the service rejects the request.
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("get_audit_log_by_id", func(t *testing.T) {
		// Use the first seeded ID. The API expects the prefixed form: aud_<uuid>.
		auditPrefixedID := fmt.Sprintf("aud_%s", seededIDs[0].String())
		url := fmt.Sprintf("%s/audit/%s", server.BaseURL, auditPrefixedID)

		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		var entry auditLogResponse
		require.NoError(t, json.Unmarshal(body, &entry))

		require.Equal(t, "user.login", entry.EventType)
		require.Equal(t, "user", entry.ActorType)
		require.Equal(t, auditPrefixedID, entry.ID)
	})

	t.Run("get_audit_log_not_found", func(t *testing.T) {
		fakeID := fmt.Sprintf("aud_%s", uuid.New().String())
		resp, err := http.Get(fmt.Sprintf("%s/audit/%s", server.BaseURL, fakeID))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("aggregate_by_event_type", func(t *testing.T) {
		url := fmt.Sprintf("%s/audit/aggregate/event_type", server.BaseURL)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		var result domain.AuditLogAggregateResponse
		require.NoError(t, json.Unmarshal(body, &result))

		require.Equal(t, "event_type", result.GroupBy)
		require.Equal(t, 6, result.TotalCount)

		// Build a map for easier assertions.
		counts := make(map[string]int)
		for _, agg := range result.Data {
			counts[agg.GroupKey] = agg.Count
		}
		require.Equal(t, 3, counts["user.login"])
		require.Equal(t, 2, counts["apikey.created"])
		require.Equal(t, 1, counts["user.logout"])
	})

	t.Run("aggregate_by_actor_type", func(t *testing.T) {
		url := fmt.Sprintf("%s/audit/aggregate/actor_type", server.BaseURL)
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		var result domain.AuditLogAggregateResponse
		require.NoError(t, json.Unmarshal(body, &result))

		require.Equal(t, "actor_type", result.GroupBy)
		require.Equal(t, 6, result.TotalCount)
		require.Len(t, result.Data, 1, "all events have actor_type=user")
		require.Equal(t, "user", result.Data[0].GroupKey)
		require.Equal(t, 6, result.Data[0].Count)
	})

	t.Run("export_csv", func(t *testing.T) {
		url := fmt.Sprintf("%s/audit/export?tenant_id=tnt_%s", server.BaseURL, tenantID.String())
		resp, err := http.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		require.Equal(t, "text/csv", resp.Header.Get("Content-Type"))
		require.Contains(t, resp.Header.Get("Content-Disposition"), "audit_logs.csv")

		csv := string(body)
		lines := strings.Split(strings.TrimSpace(csv), "\n")
		// 1 header + 6 data rows
		require.Equal(t, 7, len(lines), "expected CSV header + 6 data rows, got:\n%s", csv)

		// Verify the header row.
		require.Contains(t, lines[0], "ID")
		require.Contains(t, lines[0], "Event Type")
		require.Contains(t, lines[0], "Actor Type")
	})

	t.Run("export_csv_no_filter", func(t *testing.T) {
		// Export without any filter should still work (returns all rows).
		resp, err := http.Get(server.BaseURL + "/audit/export")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equalf(t, http.StatusOK, resp.StatusCode, "unexpected status: %s", string(body))

		csv := string(body)
		lines := strings.Split(strings.TrimSpace(csv), "\n")
		require.GreaterOrEqual(t, len(lines), 7, "should have at least header + 6 rows")
	})
}
