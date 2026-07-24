package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OSSStore is the Alibaba Cloud OSS adapter (TDD §2.1/§4.1). Built from the
// public OSS REST API spec (v1 signature, the simpler scheme used for presigned
// PUT-object URLs). NOT live-tested — merchant OSS creds land post-MVP; the
// LocalStore is the fully-tested dev path (same approach as Airwallex/PayPal).
//
// Bucket URL shape: https://<bucket>.<endpoint>/<key>. For a custom CDN domain
// set PublicBaseURL; otherwise the bucket URL is used directly.
type OSSStore struct {
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
	Endpoint        string // e.g. oss-cn-hongkong.aliyuncs.com
	// PublicBaseURL overrides the bucket URL for reads (e.g. a CDN domain).
	// If empty, PublicURL returns https://<bucket>.<endpoint>/<key>.
	PublicBaseURL string
}

func NewOSSStore(id, secret, bucket, endpoint, publicBaseURL string) *OSSStore {
	return &OSSStore{
		AccessKeyID:     id,
		AccessKeySecret: secret,
		Bucket:          bucket,
		Endpoint:        endpoint,
		PublicBaseURL:   publicBaseURL,
	}
}

func (s *OSSStore) Mode() string { return "oss" }

// PresignUpload returns a signed PUT URL (OSS v1 signature) the browser uploads
// to directly. The signature covers method, content-md5 (omitted),
// content-type, expiry, and the canonical resource path /<bucket>/<key>.
// Expiry is 15 minutes — long enough for a browser upload, short enough to
// limit replay.
func (s *OSSStore) PresignUpload(ctx context.Context, key, contentType string, size int64) (string, map[string]string, error) {
	if key == "" {
		return "", nil, fmt.Errorf("storage.OSSStore.PresignUpload: empty key")
	}
	expire := 15 * time.Minute
	expires := fmt.Sprintf("%d", time.Now().Add(expire).Unix())
	resource := "/" + s.Bucket + "/" + key
	// StringToSign = HTTP-Verb\nContent-MD5\nContent-Type\nExpires\nCanonicalizedOSSResource
	stringToSign := "PUT\n\n" + contentType + "\n" + expires + "\n" + resource
	mac := hmac.New(sha1.New, []byte(s.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	u := fmt.Sprintf("https://%s.%s/%s", s.Bucket, s.Endpoint, key)
	q := fmt.Sprintf("?Expires=%s&OSSAccessKeyId=%s&Signature=%s",
		expires, s.AccessKeyID, urlEncode(sig))
	headers := map[string]string{
		"Content-Type": contentType,
	}
	return u + q, headers, nil
}

// PublicURL resolves a stored key to its publicly-served URL. With a CDN domain
// (PublicBaseURL) it returns that; otherwise the bucket URL. If the key is
// already a full URL (http/https), it's returned as-is — this supports
// seeded/external assets whose oss_key is a remote URL.
func (s *OSSStore) PublicURL(key string) string {
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	if s.PublicBaseURL != "" {
		base := strings.TrimRight(s.PublicBaseURL, "/")
		return base + "/" + strings.TrimLeft(key, "/")
	}
	return fmt.Sprintf("https://%s.%s/%s", s.Bucket, s.Endpoint, strings.TrimLeft(key, "/"))
}

// Put writes a file server-side via the OSS PutObject REST API. Rarely used in
// OSS mode (browser presigns directly), but kept for server-side copies/tests.
func (s *OSSStore) Put(ctx context.Context, key string, r io.Reader, contentType string) error {
	if key == "" {
		return fmt.Errorf("storage.OSSStore.Put: empty key")
	}
	resource := "/" + s.Bucket + "/" + key
	// GMT date in RFC 1123 format.
	date := time.Now().UTC().Format(http.TimeFormat)
	// StringToSign = PUT\n\n<content-type>\n<date>\n<canonical-resource>
	stringToSign := "PUT\n\n" + contentType + "\n" + date + "\n" + resource
	mac := hmac.New(sha1.New, []byte(s.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("https://%s.%s/%s", s.Bucket, s.Endpoint, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, r)
	if err != nil {
		return fmt.Errorf("storage.OSSStore.Put: request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", "OSS "+s.AccessKeyID+":"+sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage.OSSStore.Put: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("storage.OSSStore.Put: oss status %d", resp.StatusCode)
	}
	return nil
}

// Delete removes an object via the OSS DeleteObject REST API. A missing object
// returns 204 in OSS (idempotent GC).
func (s *OSSStore) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	resource := "/" + s.Bucket + "/" + key
	date := time.Now().UTC().Format(http.TimeFormat)
	stringToSign := "DELETE\n\n\n" + date + "\n" + resource
	mac := hmac.New(sha1.New, []byte(s.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	url := fmt.Sprintf("https://%s.%s/%s", s.Bucket, s.Endpoint, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("storage.OSSStore.Delete: request: %w", err)
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", "OSS "+s.AccessKeyID+":"+sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage.OSSStore.Delete: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("storage.OSSStore.Delete: oss status %d", resp.StatusCode)
	}
	return nil
}

func (s *OSSStore) Key(kind Kind, mime string) (string, error) {
	return deriveKey(kind, mime)
}

// urlEncode percent-encodes a signature for use in a query string. OSS expects
// RFC 3986 encoding (+ → %2B, / → %2F, = → %3D).
func urlEncode(s string) string {
	s = strings.ReplaceAll(s, "+", "%2B")
	s = strings.ReplaceAll(s, "/", "%2F")
	s = strings.ReplaceAll(s, "=", "%3D")
	return s
}
