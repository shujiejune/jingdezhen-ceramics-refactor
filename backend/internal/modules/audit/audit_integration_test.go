package audit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"
	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const auditKey = "test-consent-key"

// seedActor inserts a user + returns the pool, a wired audit service, + the
// actor's UUID (so tests can assert rows + filter by actor).
func seedActor(t *testing.T) (*pgxpool.Pool, audit.ServiceInterface, string) {
	db := testutil.NewDBPool(t)
	ctx := context.Background()

	// Create a super_admin user (the actor).
	var actorID string
	err := db.QueryRow(ctx, `
		INSERT INTO users (nickname, email, password_hash, is_active, auth_provider)
		VALUES ('Audit Actor', 'audit-actor@jingdezhen.test', 'x', true, 'email')
		RETURNING id::text`).Scan(&actorID)
	require.NoError(t, err)

	// Assign super_admin role.
	var roleID int64
	err = db.QueryRow(ctx, `SELECT id FROM roles WHERE key = 'super_admin'`).Scan(&roleID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, actorID, roleID)
	require.NoError(t, err)

	repo := audit.NewRepository(db)
	svc := audit.NewService(repo, []byte(auditKey), db)
	return db, svc, actorID
}

func TestAudit_Log_InsertsRow(t *testing.T) {
	db, svc, actorID := seedActor(t)
	_ = db
	ctx := context.Background()

	err := svc.Log(ctx, actorID, "1.2.3.4", models.AuditActionOrderRefund,
		models.AuditEntityOrder, "42", map[string]any{"reason": "customer request"})
	require.NoError(t, err)

	var action, entityType, entityID, actorIDOut string
	var ipHash *string
	var detail []byte
	err = db.QueryRow(ctx, `
		SELECT action, entity_type, entity_id, actor_id::text, actor_ip_hash, detail
		FROM audit_log ORDER BY id DESC LIMIT 1`).
		Scan(&action, &entityType, &entityID, &actorIDOut, &ipHash, &detail)
	require.NoError(t, err)
	assert.Equal(t, "order.refund", action)
	assert.Equal(t, "order", entityType)
	assert.Equal(t, "42", entityID)
	assert.Equal(t, actorID, actorIDOut)
	require.NotNil(t, ipHash, "actor IP hashed")
	assert.NotEmpty(t, *ipHash)

	var d map[string]any
	require.NoError(t, json.Unmarshal(detail, &d))
	assert.Equal(t, "customer request", d["reason"])
}

func TestAudit_Log_NoActorID_NULL(t *testing.T) {
	db, svc, _ := seedActor(t)
	ctx := context.Background()

	err := svc.Log(ctx, "", "", models.AuditActionContentApprove,
		models.AuditEntityProduct, "1", nil)
	require.NoError(t, err)

	var actorID *string
	var ipHash *string
	err = db.QueryRow(ctx, `SELECT actor_id, actor_ip_hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&actorID, &ipHash)
	require.NoError(t, err)
	assert.Nil(t, actorID, "empty actor_id → NULL")
	assert.Nil(t, ipHash, "empty IP → NULL ip_hash")
}

func TestAudit_List_FiltersAndPaginates(t *testing.T) {
	db, svc, actorID := seedActor(t)
	_ = db
	ctx := context.Background()

	// Seed 3 audit rows: 2 order.refund + 1 content.approve.
	require.NoError(t, svc.Log(ctx, actorID, "1.1.1.1", models.AuditActionOrderRefund, models.AuditEntityOrder, "1", nil))
	require.NoError(t, svc.Log(ctx, actorID, "1.1.1.1", models.AuditActionOrderRefund, models.AuditEntityOrder, "2", nil))
	require.NoError(t, svc.Log(ctx, actorID, "1.1.1.1", models.AuditActionContentApprove, models.AuditEntityProduct, "1", nil))

	// Filter by action=order.refund → 2 rows.
	rows, total, err := svc.List(ctx, models.AuditLogFilter{Action: models.AuditActionOrderRefund}, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, rows, 2)

	// Filter by entity_type=product → 1 row.
	rows, total, err = svc.List(ctx, models.AuditLogFilter{EntityType: models.AuditEntityProduct}, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, rows, 1)

	// Filter by actor_id → 3 rows.
	aid := actorID
	rows, total, err = svc.List(ctx, models.AuditLogFilter{ActorID: &aid}, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 3)

	// Pagination: page 1 limit 2 → 2 rows, total 3.
	rows, total, err = svc.List(ctx, models.AuditLogFilter{}, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, rows, 2)

	// Page 2 → 1 row.
	rows, _, err = svc.List(ctx, models.AuditLogFilter{}, 2, 2)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestAudit_NoopLogger(t *testing.T) {
	var nl audit.NoopLogger
	err := nl.Log(context.Background(), "", "", "", "", "", nil)
	assert.NoError(t, err)
}

func TestAudit_Handler_CSV(t *testing.T) {
	db, svc, actorID := seedActor(t)
	_ = db
	ctx := context.Background()
	require.NoError(t, svc.Log(ctx, actorID, "1.2.3.4", models.AuditActionOrderRefund, models.AuditEntityOrder, "99", map[string]any{"reason": "test"}))

	h := audit.NewHandler(svc)
	app := fiber.New()
	app.Get("/audit", h.List)

	req := httptest.NewRequest("GET", "/audit?format=csv", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, "text/csv", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "audit-log.csv")
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "id,created_at,actor_id,action,entity_type,entity_id,detail")
	assert.Contains(t, string(body), "order.refund")
	assert.Contains(t, string(body), "99")
}

func TestAudit_Handler_JSONPaginated(t *testing.T) {
	db, svc, actorID := seedActor(t)
	_ = db
	ctx := context.Background()
	require.NoError(t, svc.Log(ctx, actorID, "1.2.3.4", models.AuditActionContentApprove, models.AuditEntityProduct, "1", nil))

	h := audit.NewHandler(svc)
	app := fiber.New()
	app.Get("/audit", h.List)

	req := httptest.NewRequest("GET", "/audit?page=1&limit=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "content.approve")
	assert.Contains(t, string(body), "\"total\":1")
}

func TestAudit_Handler_DateRangeFilter(t *testing.T) {
	db, svc, actorID := seedActor(t)
	_ = db
	ctx := context.Background()
	require.NoError(t, svc.Log(ctx, actorID, "1.2.3.4", models.AuditActionOrderRefund, models.AuditEntityOrder, "1", nil))

	h := audit.NewHandler(svc)
	app := fiber.New()
	app.Get("/audit", h.List)

	// Valid range.
	from := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	to := time.Now().UTC().Format("2006-01-02")
	req := httptest.NewRequest("GET", "/audit?from="+from+"&to="+to, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Invalid range (from > to).
	req = httptest.NewRequest("GET", "/audit?from=2026-12-31&to=2026-01-01", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

// _ keeps imports alive.
var _ = strconv.Itoa
var _ = utils.GetPageLimit
