package engage

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBExecutor defines an interface for executing SQL queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type RepositoryInterface interface {
	FindAllActivities(ctx context.Context, page, limit int) ([]models.Activity, int, error)
	FindArticleByIDOrSlug(ctx context.Context, idOrSlug string) (*models.Article, error)
}

type Repository struct {
	db       *pgxpool.Pool
	executor DBExecutor
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db, executor: db}
}

// FindAllActivities retrieves a paginated list of all activities.
func (r *Repository) FindAllActivities(ctx context.Context, page, limit int) ([]models.Activity, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM activities`
	err := r.executor.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllActivities.Count: %w", err)
	}

	if total == 0 {
		return []models.Activity{}, 0, nil
	}

	offset := (page - 1) * limit
	query := `
		SELECT id, title, type, brief_introduction, photograph_url, article_slug, created_at, updated_at
		FROM activities
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.executor.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllActivities.Query: %w", err)
	}
	defer rows.Close()

	activities := []models.Activity{}
	for rows.Next() {
		var act models.Activity
		if err := rows.Scan(
			&act.ID, &act.Title, &act.Type, &act.BriefIntroduction,
			&act.PhotographURL, &act.ArticleSlug, &act.CreatedAt, &act.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllActivities.Scan: %w", err)
		}
		activities = append(activities, act)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllActivities.RowsErr: %w", err)
	}

	return activities, total, nil
}

// FindArticleByIDOrSlug retrieves a single article by its ID or slug.
func (r *Repository) FindArticleByIDOrSlug(ctx context.Context, idOrSlug string) (*models.Article, error) {
	var article models.Article
	query := `
		SELECT ar.id, ar.slug, ar.title, ar.content, ar.author_id, u.nickname as author_name, 
		       ar.published_at, ar.created_at, ar.updated_at
		FROM articles ar
		LEFT JOIN users u ON ar.author_id = u.id
	`
	var err error
	id, convErr := strconv.ParseInt(idOrSlug, 10, 64)
	if convErr == nil {
		query += " WHERE ar.id = $1"
		err = r.executor.QueryRow(ctx, query, id).Scan(
			&article.ID, &article.Slug, &article.Title, &article.Content, &article.AuthorID,
			&article.AuthorName, &article.PublishedAt, &article.CreatedAt, &article.UpdatedAt,
		)
	} else {
		query += " WHERE ar.slug = $1"
		err = r.executor.QueryRow(ctx, query, idOrSlug).Scan(
			&article.ID, &article.Slug, &article.Title, &article.Content, &article.AuthorID,
			&article.AuthorName, &article.PublishedAt, &article.CreatedAt, &article.UpdatedAt,
		)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindArticleByIDOrSlug: %w", err)
	}
	return &article, nil
}
