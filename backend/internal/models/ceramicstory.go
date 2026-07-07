package models

// CeramicStory represents the characteristics of Jingdezhen ceramics in a dynasty.
// This struct will be used for transferring data between layers and for API responses.
type CeramicStory struct {
	ID                   int64  `json:"id" db:"id"`
	DynastyName          string `json:"dynasty_name" db:"dynasty_name"`
	Slug                 string `json:"slug" db:"slug"`                       // For URL-friendly access (e.g., "ming-dynasty")
	Period               string `json:"period,omitempty" db:"period"`         // e.g., "Early Ming", "Late Qing"
	StartYear            *int   `json:"start_year,omitempty" db:"start_year"` // Pointer to allow NULL
	EndYear              *int   `json:"end_year,omitempty" db:"end_year"`     // Pointer to allow NULL
	Description          string `json:"description" db:"description"`
	CharacteristicsCraft string `json:"characteristics_craft,omitempty" db:"characteristics_craft"`
	CharacteristicsArt   string `json:"characteristics_art,omitempty" db:"characteristics_art"`
	ImageURL             string `json:"image_url,omitempty" db:"image_url"`
	Takeaways            string `json:"takeaways,omitempty" db:"takeaways"` // Brief key points for timeline view
	DisplayOrder         int    `json:"display_order" db:"display_order"`   // For ordering in the timeline
	// Consider adding CreatedAt and UpdatedAt if you want to track changes
	// CreatedAt           time.Time `json:"created_at" db:"created_at"`
	// UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// CreateCeramicStoryData defines the structure for data needed to create a new ceramic story.
// This would typically be used by an admin interface.
type CreateCeramicStoryData struct {
	DynastyName          string `json:"dynasty_name" validate:"required,max=100"`
	Slug                 string `json:"slug" validate:"required,alphanumdash,max=100"` // Alphanumeric + dashes
	Period               string `json:"period,omitempty" validate:"max=100"`
	StartYear            *int   `json:"start_year,omitempty" validate:"omitempty,ltecsfield=EndYear"`
	EndYear              *int   `json:"end_year,omitempty" validate:"omitempty,gtecsfield=StartYear"`
	Description          string `json:"description" validate:"required"`
	CharacteristicsCraft string `json:"characteristics_craft,omitempty"`
	CharacteristicsArt   string `json:"characteristics_art,omitempty"`
	ImageURL             string `json:"image_url,omitempty" validate:"omitempty,url"`
	Takeaways            string `json:"takeaways,omitempty"`
	DisplayOrder         int    `json:"display_order" validate:"gte=0"`
}

// UpdateCeramicStoryData defines the structure for data needed to update an existing ceramic story.
// This would typically be used by an admin interface.
type UpdateCeramicStoryData struct {
	DynastyName          *string `json:"dynasty_name,omitempty" validate:"omitempty,max=100"`
	Slug                 *string `json:"slug,omitempty" validate:"omitempty,alphanumdash,max=100"`
	Period               *string `json:"period,omitempty" validate:"omitempty,max=100"`
	StartYear            *int    `json:"start_year,omitempty" validate:"omitempty,ltecsfield=EndYear"`
	EndYear              *int    `json:"end_year,omitempty" validate:"omitempty,gtecsfield=StartYear"`
	Description          *string `json:"description,omitempty"`
	CharacteristicsCraft *string `json:"characteristics_craft,omitempty"`
	CharacteristicsArt   *string `json:"characteristics_art,omitempty"`
	ImageURL             *string `json:"image_url,omitempty" validate:"omitempty,url"`
	Takeaways            *string `json:"takeaways,omitempty"`
	DisplayOrder         *int    `json:"display_order,omitempty" validate:"omitempty,gte=0"`
}
