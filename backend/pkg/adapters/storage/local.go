package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStore writes uploaded files to a local directory and serves them via a
// Fiber static mount (STORAGE_MODE=local, dev default). There is no real
// presign — the dev upload handler calls Put server-side. PublicURL returns a
// path under PublicBaseURL (e.g. /media/...) which the Fiber static mount serves.
type LocalStore struct {
	RootDir       string // on-disk directory (e.g. ./_media)
	PublicBaseURL string // URL prefix (e.g. /media)
}

// NewLocalStore constructs a LocalStore, creating RootDir if missing.
func NewLocalStore(rootDir, publicBaseURL string) (*LocalStore, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("storage.NewLocalStore: mkdir %s: %w", rootDir, err)
	}
	return &LocalStore{RootDir: rootDir, PublicBaseURL: publicBaseURL}, nil
}

func (s *LocalStore) Mode() string { return "local" }

// PresignUpload for LocalStore returns an empty URL + a marker header. The
// upload handler detects Mode()=="local" and calls Put server-side instead of
// returning a presigned URL to the browser.
func (s *LocalStore) PresignUpload(ctx context.Context, key, contentType string, size int64) (string, map[string]string, error) {
	return "", map[string]string{"X-Storage-Mode": "local"}, nil
}

// PublicURL resolves a stored key to its served path under PublicBaseURL.
// If the key is already a full URL (http/https), it's returned as-is — this
// supports seeded/external assets whose oss_key is a remote URL.
func (s *LocalStore) PublicURL(key string) string {
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	base := strings.TrimRight(s.PublicBaseURL, "/")
	if key == "" {
		return base
	}
	return base + "/" + strings.TrimLeft(key, "/")
}

// Put writes the file to RootDir/<key>. Parent dirs are created as needed.
func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	if key == "" {
		return fmt.Errorf("storage.LocalStore.Put: empty key")
	}
	full := filepath.Join(s.RootDir, filepath.Clean("/"+key))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("storage.LocalStore.Put: mkdir: %w", err)
	}
	f, err := os.Create(full)
	if err != nil {
		return fmt.Errorf("storage.LocalStore.Put: create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage.LocalStore.Put: copy: %w", err)
	}
	return nil
}

// Delete removes the file. A missing file is not an error (idempotent GC).
func (s *LocalStore) Delete(ctx context.Context, key string) error {
	full := filepath.Join(s.RootDir, filepath.Clean("/"+key))
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage.LocalStore.Delete: %w", err)
	}
	return nil
}

// preferredExt maps common mime types to a canonical extension, preferred
// over mime.ExtensionsByType's alphabetically-first result (which yields
// .jfif for image/jpeg, an oddity). Falls back to the mime registry.
var preferredExt = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/bmp":       ".bmp",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
}

// Key derives media/<yyyy>/<mm>/<uuid>.<ext> from the kind + mime. The uuid is
// 16 hex bytes of crypto/rand.
func (s *LocalStore) Key(kind Kind, mimeType string) (string, error) {
	return deriveKey(kind, mimeType)
}

// OSSStore also uses the same key derivation.
func deriveKey(kind Kind, mimeType string) (string, error) {
	if kind != KindImage && kind != KindVideo {
		return "", ErrUnsupportedKind
	}
	ext := preferredExt[mimeType]
	if ext == "" {
		exts, _ := mime.ExtensionsByType(mimeType)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("storage.deriveKey: rand: %w", err)
	}
	now := time.Now().UTC()
	return fmt.Sprintf("media/%04d/%02d/%s%s", now.Year(), now.Month(), hex.EncodeToString(b[:]), ext), nil
}
