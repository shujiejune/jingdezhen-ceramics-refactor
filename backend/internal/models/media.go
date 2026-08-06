package models

import "time"

// =============================================================================
// Media assets + ordered galleries (TDD §3.4 line 127/132/143, PRD §3.2.1)
//
// media_assets is a central registry: one row = one file in object storage
// (OSS in prod, a local dir in dev). The DB stores only the stable oss_key;
// the publicly-served URL is resolved at read time by the storage adapter
// (so a bucket/CDN-domain change is a config edit, not a data migration).
//
// Per-entity *_media join tables attach media to content entities in an
// ordered gallery (sort_order 0 = primary display image). product_media is
// the first gallery wired; artist_media / ceramic_story_media follow the same
// pattern.
//
// Upload flow (TDD §2.2): admin browser → presigned OSS PUT (direct to OSS,
// never through the VPS) → POST the returned oss_key + metadata to register
// the media_assets row → attach to a product via product_media. In local-dev
// mode the upload handler calls Store.Put server-side (no real presign).
// =============================================================================

// MediaAsset is one stored file. PublicURL is populated by the service at read
// time (resolved from OSSKey via the storage adapter) — never persisted.
type MediaAsset struct {
	ID         int64     `json:"id" db:"id"`
	Kind       string    `json:"kind" db:"kind"` // image | video
	OSSKey     string    `json:"oss_key" db:"oss_key"`
	MIME       string    `json:"mime" db:"mime"`
	Width      *int      `json:"width,omitempty" db:"width"`
	Height     *int      `json:"height,omitempty" db:"height"`
	Duration   *int      `json:"duration,omitempty" db:"duration"` // seconds (videos)
	HLSKey     *string   `json:"hls_key,omitempty" db:"hls_key"`   // transcoded HLS playlist (videos)
	UploadedBy *string   `json:"uploaded_by,omitempty" db:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	// PublicURL resolved at read time by the storage adapter. Not in DB.
	PublicURL string `json:"public_url,omitempty" db:"-"`
}

// ProductMediaItem is one entry in a product's ordered gallery: the media
// asset + its sort_order + caption on this product.
type ProductMediaItem struct {
	ID        int64     `json:"id" db:"id"` // product_media.id
	ProductID int64     `json:"product_id" db:"product_id"`
	MediaID   int64     `json:"media_id" db:"media_id"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	Caption   *string   `json:"caption,omitempty" db:"caption"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// The media asset, loaded via JOIN. Embedded so the JSON nests it.
	MediaAsset MediaAsset `json:"media" db:"media"`
}

// GalleryItem is the entity-agnostic ordered-gallery entry used by the
// artist / ceramic-story / activity galleries (the per-entity FK is omitted —
// the caller already knows which entity it asked for). product_media keeps its
// own ProductMediaItem (shipped unchanged) so the product API is stable; the
// three newer galleries share this one type to avoid triplicated structs.
//
// Fields mirror ProductMediaItem minus the entity FK: the join-table id, the
// media_id, the sort_order (0 = primary display image), the optional caption,
// and the joined media asset with its public URL resolved at read time.
type GalleryItem struct {
	ID        int64     `json:"id" db:"id"` // *_media.id
	MediaID   int64     `json:"media_id" db:"media_id"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	Caption   *string   `json:"caption,omitempty" db:"caption"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// The media asset, loaded via JOIN. Embedded so the JSON nests it.
	MediaAsset MediaAsset `json:"media" db:"media"`
}

// --- Admin DTOs ---

// RegisterAssetData registers an already-uploaded file as a media_assets row.
// The browser uploads to OSS (or the dev upload handler Put's locally) FIRST,
// then POSTs the returned oss_key + metadata here.
type RegisterAssetData struct {
	Kind     string `json:"kind" validate:"required,oneof=image video"`
	OSSKey   string `json:"oss_key" validate:"required"`
	MIME     string `json:"mime" validate:"required"`
	Width    *int   `json:"width,omitempty" validate:"omitempty,gte=0"`
	Height   *int   `json:"height,omitempty" validate:"omitempty,gte=0"`
	Duration *int   `json:"duration,omitempty" validate:"omitempty,gte=0"` // videos
}

// AttachMediaData attaches a media_assets row to a product in its ordered gallery.
type AttachMediaData struct {
	MediaID   int64  `json:"media_id" validate:"required"`
	SortOrder *int   `json:"sort_order,omitempty" validate:"omitempty,gte=0"` // defaults to append-last
	Caption   string `json:"caption,omitempty"`
}

// ReorderMediaItem is one entry in a batch reorder request.
type ReorderMediaItem struct {
	MediaID   int64 `json:"media_id" validate:"required"`
	SortOrder int   `json:"sort_order" validate:"gte=0"`
}
