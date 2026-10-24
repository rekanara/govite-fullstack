// Package storage keeps files in MinIO / any S3-compatible store. It owns the
// key layout and decides what is public; callers store only the returned key.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Visibility per feature: the one place that decides what is public.
// Adding a feature means choosing its visibility here, deliberately.
var featureVisibility = map[string]string{
	"avatars":       "public",
	"branding":      "public",
	"announcements": "public", // shown to every user and in push notifications
	"misc":          "private",
}

type Config struct {
	Endpoint       string // host:port, e.g. localhost:9000
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
	PublicEndpoint string // browser-facing base when it differs (proxy/CDN host)
	PublicURL      string // base URL of public files; default <endpoint>/<bucket>
}

// Store is nil-safe: a nil *Store means storage is not configured.
type Store struct {
	client     *minio.Client
	signer     *minio.Client // signs against the browser-facing host
	bucket     string
	publicBase string
	ready      sync.Once
}

// New returns nil when credentials are missing; file features then answer 503.
func New(cfg Config) (*Store, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, nil
	}
	make := func(endpoint string, secure bool) (*minio.Client, error) {
		return minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: secure, Region: "us-east-1"})
	}
	client, err := make(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	external := strings.TrimRight(cfg.PublicEndpoint, "/")
	if external == "" {
		external = scheme + "://" + cfg.Endpoint
	}
	s := &Store{client: client, signer: client, bucket: cfg.Bucket}
	if u, err := url.Parse(external); err == nil && u.Host != cfg.Endpoint {
		if s.signer, err = make(u.Host, u.Scheme == "https"); err != nil {
			return nil, err
		}
	}
	s.publicBase = strings.TrimRight(cfg.PublicURL, "/")
	if s.publicBase == "" {
		s.publicBase = external + "/" + cfg.Bucket
	}
	return s, nil
}

func (s *Store) Enabled() bool { return s != nil }

// EnsureBucket creates the bucket and allows anonymous reads only under
// <feature>/public/. Failures are logged, never fatal.
func (s *Store) EnsureBucket(ctx context.Context) error {
	if s == nil {
		return nil
	}
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err == nil && !ok {
		err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	}
	if err == nil {
		policy, _ := json.Marshal(map[string]any{
			"Version": "2012-10-17",
			"Statement": []any{map[string]any{
				"Effect": "Allow", "Principal": map[string]any{"AWS": []string{"*"}},
				"Action": []string{"s3:GetObject"}, "Resource": []string{"arn:aws:s3:::" + s.bucket + "/*/public/*"},
			}},
		})
		err = s.client.SetBucketPolicy(ctx, s.bucket, string(policy))
	}
	if err != nil {
		slog.ErrorContext(ctx, "object storage unavailable", "bucket", s.bucket, "err", err)
	}
	return err
}

var unsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func clean(s string) string { return unsafe.ReplaceAllString(s, "") }

// Key builds `{feature}/{visibility}/{scope?}/{yyyy}/{mm}/{name}.{ext}`.
// Every part is stripped to [a-zA-Z0-9_-], so callers can't escape with `..`.
func Key(feature, scope, name, ext string, at time.Time) string {
	vis, ok := featureVisibility[feature]
	if !ok {
		panic("storage: unknown feature " + feature)
	}
	parts := []string{feature, vis}
	if scope != "" {
		parts = append(parts, clean(scope))
	}
	parts = append(parts, fmt.Sprintf("%04d", at.UTC().Year()), fmt.Sprintf("%02d", int(at.UTC().Month())))
	return strings.Join(parts, "/") + "/" + clean(name) + "." + clean(ext)
}

func IsPublic(key string) bool {
	parts := strings.SplitN(key, "/", 3)
	return len(parts) > 1 && parts[1] == "public"
}

func (s *Store) Put(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

// Remove is idempotent: a missing object is not an error.
func (s *Store) Remove(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// URL is the permanent link for public keys, "" for private ones.
// Absolute URLs (legacy rows) pass through.
func (s *Store) URL(key string) string {
	if key == "" || s == nil {
		return ""
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	if !IsPublic(key) {
		return ""
	}
	return s.publicBase + "/" + key
}

// SignedURL is a time-limited link; the way to hand out a private file.
func (s *Store) SignedURL(ctx context.Context, key, downloadAs string, ttl time.Duration) (string, error) {
	params := url.Values{}
	if downloadAs != "" {
		ascii := regexp.MustCompile(`[^\x20-\x7E]+|"`).ReplaceAllString(downloadAs, "_")
		params.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(downloadAs)))
	}
	u, err := s.signer.PresignedGetObject(ctx, s.bucket, key, ttl, params)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

var keyInURL = regexp.MustCompile(`(?i)([a-z0-9_-]+/(?:public|private)/[^?#]+)`)

// KeyFromURL recovers the key from a stored public URL, e.g. to delete it.
func KeyFromURL(u string) string {
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return u
	}
	if m := keyInURL.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return ""
}

// MaxImageBytes caps logo, banner and announcement images.
const MaxImageBytes = 5 << 20

// DetectImage reads JPEG/PNG/WebP magic bytes; it never trusts the client's
// mimetype. Anything else (including SVG) returns ok=false.
func DetectImage(b []byte) (mime, ext string, ok bool) {
	switch {
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return "image/jpeg", "jpg", true
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", "png", true
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp", "webp", true
	}
	return "", "", false
}
