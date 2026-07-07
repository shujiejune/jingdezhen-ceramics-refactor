package models

// Tag represents a single, reusable tag that can be applied to
// forum posts, portfolio works, or gallery artworks.
type Tag struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// TagWithPostCount is a specific view model for the "Tag Cloud" feature.
// It includes the count of posts associated with each tag.
type TagWithPostCount struct {
	ID        int64  `json:"id" db:"id"`
	Name      string `json:"name" db:"name"`
	PostCount int    `json:"post_count" db:"post_count"`
}
