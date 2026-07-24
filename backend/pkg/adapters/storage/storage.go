// Package storage defines the object-storage adapter (TDD §4.1/§2.1): media
// never flows through the VPS — the admin browser uploads directly to OSS via
// a presigned URL, the CDN serves it. The API only signs the upload + records
// the resulting oss_key on a media_assets row.
//
// Two implementations:
//   - LocalStore (STORAGE_MODE=local, dev default): writes to a local dir,
//     served via a Fiber static mount. No real presign — the dev upload
//     handler calls Put server-side.
//   - OSSStore (STORAGE_MODE=oss, live): Alibaba Cloud OSS (HK). Built from
//     the public OSS API spec; NOT live-tested until merchant OSS creds land
//     post-MVP (same situation as Airwallex/PayPal). PresignUpload returns a
//     signed PUT URL the browser uploads to directly.
//
// Services depend on the Store interface, never a concrete client, so swapping
// dev→live is an env-var flip (TDD §4.1, §10).
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrUnsupportedKind is returned by Key when the mime/kind pair can't derive a
// storage key (e.g. an unknown file extension).
var ErrUnsupportedKind = errors.New("storage: unsupported media kind")

// Kind classifies a media asset. Videos need transcoding (media:transcode job,
// TDD line 230); images do not.
type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
)

// Store is the contract an object-storage provider satisfies. Services depend
// on this interface, never on a concrete client (TDD §4.1).
type Store interface {
	// Mode returns "local" or "oss" — used by the upload handler to decide
	// whether to call Put server-side (local) or return a presigned URL (oss).
	Mode() string

	// PresignUpload returns a signed URL the browser uploads to directly (OSS).
	// For LocalStore it returns an empty URL + a marker header so the upload
	// handler knows to call Put server-side instead (no real presign in dev).
	PresignUpload(ctx context.Context, key, contentType string, size int64) (url string, headers map[string]string, err error)

	// PublicURL resolves a stored oss_key to its publicly-served URL (CDN URL
	// for OSS, a /<base>/<key> path for local). Called at read time so the
	// media_assets row stores only the stable oss_key, never a baked-in URL
	// (a bucket/CDN-domain change is a config edit, not a data migration).
	PublicURL(key string) string

	// Put writes a file server-side. Used by the dev upload handler (LocalStore)
	// + tests. OSS mode uses browser-direct presigned PUTs, so Put is rarely
	// called there (kept for server-side copies/tests).
	Put(ctx context.Context, key string, r io.Reader, contentType string) error

	// Delete removes an object. Used for orphan/GC when a media_assets row is
	// deleted (best-effort — a missing object is not an error).
	Delete(ctx context.Context, key string) error

	// Key derives a stable storage key for a new upload: media/<yyyy>/<mm>/<uuid>.<ext>.
	// The uuid makes collisions vanishingly unlikely; the date prefix aids
	// OSS partitioning + manual browsing.
	Key(kind Kind, mime string) (string, error)
}
