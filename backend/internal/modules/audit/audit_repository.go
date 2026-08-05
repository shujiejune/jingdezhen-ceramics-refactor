package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Executor is the minimal subset of *pgxpool.Pool / pgx.Tx the audit repo needs.
// Defining it locally lets Insert enlist in a caller's transaction (atomicity:
// if the sensitive action rolls back, the audit row rolls back too, TDD §4.3)
// OR use the pool directly when the caller has no tx. Both implement Exec.
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// RepositoryInterface defines audit storage operations.
type RepositoryInterface interface {
	// Insert stores one audit entry, enlisting in exec's transaction if the
	// caller passed one (otherwise exec should be the pool). Best-effort: a
	// failure is returned but callers treat it as non-fatal (the action
	// already succeeded; a missing audit row is preferable to blocking it).
	Insert(ctx context.Context, exec Executor, e models.AuditLog) error
	// List returns audit rows matching filter, newest first, paginated.
	List(ctx context.Context, f models.AuditLogFilter, page, limit int) ([]models.AuditLog, int, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

const auditColumns = "id, actor_id, actor_ip_hash, action, entity_type, entity_id, detail, created_at"

func (r *Repository) scanRow(row pgx.Row) (*models.AuditLog, error) {
	var e models.AuditLog
	var actorID, ipHash, entityID *string
	err := row.Scan(&e.ID, &actorID, &ipHash, &e.Action, &e.EntityType, &entityID, &e.Detail, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.ActorID = actorID
	e.ActorIPHash = ipHash
	e.EntityID = entityID
	return &e, nil
}

func (r *Repository) Insert(ctx context.Context, exec Executor, e models.AuditLog) error {
	detail, err := json.Marshal(e.Detail)
	if err != nil {
		return fmt.Errorf("audit.Insert: marshal detail: %w", err)
	}
	if e.Detail == nil {
		detail = []byte("{}")
	}
	const q = `
		INSERT INTO audit_log (actor_id, actor_ip_hash, action, entity_type, entity_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = exec.Exec(ctx, q, e.ActorID, e.ActorIPHash, e.Action, e.EntityType, e.EntityID, detail)
	if err != nil {
		return fmt.Errorf("audit.Insert: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, f models.AuditLogFilter, page, limit int) ([]models.AuditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Build a parameterised WHERE from the non-zero filter fields. The dynamic
	// arg list is small + bounded (max 6 clauses) — no risk of SQL injection
	// (values are never interpolated, only $N placeholders).
	where := []string{}
	args := []any{}
	n := 1
	add := func(clause string, val any) {
		where = append(where, fmt.Sprintf(clause, n))
		args = append(args, val)
		n++
	}
	if f.ActorID != nil && *f.ActorID != "" {
		add("actor_id = $%d", *f.ActorID)
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}
	if f.EntityType != "" {
		add("entity_type = $%d", f.EntityType)
	}
	if f.EntityID != nil && *f.EntityID != "" {
		add("entity_id = $%d", *f.EntityID)
	}
	if !f.From.IsZero() {
		add("created_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("created_at < $%d", f.To)
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + joinStrings(where, " AND ")
	}

	var total int
	countQ := "SELECT COUNT(*) FROM audit_log" + whereClause
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit.List.count: %w", err)
	}

	offset := (page - 1) * limit
	listQ := "SELECT " + auditColumns + " FROM audit_log" + whereClause +
		" ORDER BY created_at DESC, id DESC LIMIT $" + itoa(n) + " OFFSET $" + itoa(n+1)
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.List.query: %w", err)
	}
	defer rows.Close()

	out := []models.AuditLog{}
	for rows.Next() {
		e, err := r.scanRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("audit.List.scan: %w", err)
		}
		out = append(out, *e)
	}
	return out, total, nil
}

// joinStrings joins ss with sep (avoids pulling strings.Join for a 1-liner).
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}

// itoa returns the decimal string for a small int (avoids strconv for the
// dynamic-placeholder index in List).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// errPoolNotExecutor is unused but keeps the import graph honest if the pool
// is later wrapped; *pgxpool.Pool satisfies Executor via Exec.
var _ Executor = (*pgxpool.Pool)(nil)
var _ = errors.Is
