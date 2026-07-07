package models

import "time"

// Activity represents a high-level local event or place, like a festival, fair, or museum.
// This is used for the card view on the main "Engage" page.
type Activity struct {
	ID                int64     `json:"id" db:"id"`
	Title             string    `json:"title" db:"title"`
	Type              string    `json:"type" db:"type"` // e.g., 'Festival', 'Fair', 'Museum'
	BriefIntroduction string    `json:"brief_introduction" db:"brief_introduction"`
	PhotographURL     string    `json:"photograph_url" db:"photograph_url"`
	ArticleSlug       string    `json:"article_slug" db:"article_slug"` // The link to the detailed article
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// Article represents the detailed content associated with an activity.
type Article struct {
	ID          int64     `json:"id" db:"id"`
	Slug        string    `json:"slug" db:"slug"`
	Title       string    `json:"title" db:"title"`
	Content     string    `json:"content" db:"content"`               // Markdown or HTML content
	AuthorID    *string   `json:"author_id,omitempty" db:"author_id"` // User ID (UUID string) if written by a platform user
	AuthorName  *string   `json:"author_name,omitempty" db:"-"`       // Populated by JOIN
	PublishedAt time.Time `json:"published_at" db:"published_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
