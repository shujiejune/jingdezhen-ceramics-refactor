package storage

import (
	"context"
	"strings"
	"testing"
)

func TestLocalStore_PutAndPublicURL(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir, "/media")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	if s.Mode() != "local" {
		t.Fatalf("Mode = %q, want local", s.Mode())
	}
	key := "media/2026/01/abc.jpg"
	if err := s.Put(context.Background(), key, strings.NewReader("hello"), "image/jpeg"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	want := "/media/media/2026/01/abc.jpg"
	if got := s.PublicURL(key); got != want {
		t.Fatalf("PublicURL = %q, want %q", got, want)
	}
	// Empty key → base URL.
	if got := s.PublicURL(""); got != "/media" {
		t.Fatalf("PublicURL empty = %q, want /media", got)
	}
	// Full-URL oss_key passes through (seeded/external assets).
	ext := "https://picsum.photos/seed/vase-bw/600/600"
	if got := s.PublicURL(ext); got != ext {
		t.Fatalf("PublicURL(full url) = %q, want passthrough", got)
	}
}

func TestLocalStore_DeleteIdempotent(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalStore(dir, "/media")
	key := "media/x.jpg"
	_ = s.Put(context.Background(), key, strings.NewReader("x"), "image/jpeg")
	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete existing: %v", err)
	}
	// Deleting again (already gone) must not error — GC is idempotent.
	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestLocalStore_PresignUpload(t *testing.T) {
	s, _ := NewLocalStore(t.TempDir(), "/media")
	url, headers, err := s.PresignUpload(context.Background(), "k", "image/jpeg", 10)
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if url != "" {
		t.Fatalf("local presign url = %q, want empty", url)
	}
	if headers["X-Storage-Mode"] != "local" {
		t.Fatalf("marker header missing: %v", headers)
	}
}

func TestDeriveKey(t *testing.T) {
	k1, err := deriveKey(KindImage, "image/jpeg")
	if err != nil {
		t.Fatalf("deriveKey: %v", err)
	}
	if !strings.HasPrefix(k1, "media/") {
		t.Fatalf("key = %q, want media/ prefix", k1)
	}
	if !strings.HasSuffix(k1, ".jpg") {
		t.Fatalf("key = %q, want .jpg suffix", k1)
	}
	k2, _ := deriveKey(KindImage, "image/jpeg")
	if k1 == k2 {
		t.Fatalf("two keys collided: %q (uuid expected unique)", k1)
	}
	if _, err := deriveKey(Kind("bogus"), "image/jpeg"); err != ErrUnsupportedKind {
		t.Fatalf("deriveKey bogus kind: err = %v, want ErrUnsupportedKind", err)
	}
}

func TestOSSStore_PublicURL(t *testing.T) {
	s := NewOSSStore("id", "secret", "jdz-media", "oss-cn-hongkong.aliyuncs.com", "")
	got := s.PublicURL("media/2026/01/abc.jpg")
	want := "https://jdz-media.oss-cn-hongkong.aliyuncs.com/media/2026/01/abc.jpg"
	if got != want {
		t.Fatalf("PublicURL = %q, want %q", got, want)
	}
	// CDN override.
	s2 := NewOSSStore("id", "secret", "b", "e", "https://cdn.jingdezhen.test")
	if got := s2.PublicURL("media/x.jpg"); got != "https://cdn.jingdezhen.test/media/x.jpg" {
		t.Fatalf("CDN PublicURL = %q", got)
	}
}

func TestOSSStore_PresignUploadShape(t *testing.T) {
	s := NewOSSStore("id", "secret", "jdz-media", "oss-cn-hongkong.aliyuncs.com", "")
	url, headers, err := s.PresignUpload(context.Background(), "media/x.jpg", "image/jpeg", 100)
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if !strings.HasPrefix(url, "https://jdz-media.oss-cn-hongkong.aliyuncs.com/media/x.jpg?") {
		t.Fatalf("presign url = %q", url)
	}
	if !strings.Contains(url, "OSSAccessKeyId=id") {
		t.Fatalf("presign url missing access key id: %q", url)
	}
	if !strings.Contains(url, "Signature=") {
		t.Fatalf("presign url missing signature: %q", url)
	}
	if !strings.Contains(url, "Expires=") {
		t.Fatalf("presign url missing expiry: %q", url)
	}
	if headers["Content-Type"] != "image/jpeg" {
		t.Fatalf("headers = %v", headers)
	}
}
