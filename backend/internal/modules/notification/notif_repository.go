package notification

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface provides database access for notifications using raw SQL.
type RepositoryInterface interface {
	Create(ctx context.Context, notification *models.Notification) error
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error)
	GetTotalCountByUserID(ctx context.Context, userID string) (int, error)
	MarkAsRead(ctx context.Context, notificationID int64, userID string) (bool, error)
	MarkAllAsRead(ctx context.Context, userID string) (int64, error)
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
}

// This interface represents anything that can execute a SQL query,
// which includes both a connection pool and a transaction.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	db       *pgxpool.Pool
	executor DBExecutor
}

// NewRepository creates a new notification repository.
func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{
		db:       db,
		executor: db,
	}
}

// Create saves a new notification to the database and populates the generated ID and CreatedAt fields.
func (r *Repository) Create(ctx context.Context, notification *models.Notification) error {
	query := `
        INSERT INTO notifications (recipient_user_id, actor_user_id, notification_type, entity_type, entity_id, message, is_read)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, created_at`

	// Use QueryRow to execute the query and scan the returned values back into the struct.
	err := r.executor.QueryRow(ctx, query,
		notification.RecipientUserID,
		notification.ActorUserID,
		notification.NotificationType,
		notification.EntityType,
		notification.EntityID,
		notification.Message,
		notification.IsRead,
	).Scan(&notification.ID, &notification.CreatedAt)

	return err
}

// GetByUserID retrieves a paginated list of notifications for a specific user.
func (r *Repository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error) {
	query := `
        SELECT id, recipient_user_id, actor_user_id, notification_type, entity_type, entity_id, message, is_read, created_at
        FROM notifications
        WHERE recipient_user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := r.executor.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	notifications, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Notification])
	if err != nil {
		return nil, fmt.Errorf("failed to collect notification rows: %w", err)
	}

	return notifications, nil
}

// GetTotalCountByUserID gets the total number of notifications for a user (for pagination).
func (r *Repository) GetTotalCountByUserID(ctx context.Context, userID string) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1`
	err := r.executor.QueryRow(ctx, query, userID).Scan(&total)
	return total, err
}

// MarkAsRead marks a single notification as read. Returns true if a row was updated.
func (r *Repository) MarkAsRead(ctx context.Context, notificationID int64, userID string) (bool, error) {
	query := `UPDATE notifications SET is_read = TRUE WHERE id = $1 AND recipient_user_id = $2`
	result, err := r.executor.Exec(ctx, query, notificationID, userID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, err
}

// MarkAllAsRead marks all notifications for a user as read. Returns the number of rows updated.
func (r *Repository) MarkAllAsRead(ctx context.Context, userID string) (int64, error) {
	query := `UPDATE notifications SET is_read = TRUE WHERE recipient_user_id = $1 AND is_read = FALSE`
	result, err := r.executor.Exec(ctx, query, userID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetUnreadCount retrieves the count of unread notifications for a user.
func (r *Repository) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1 AND is_read = FALSE`
	err := r.executor.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}
