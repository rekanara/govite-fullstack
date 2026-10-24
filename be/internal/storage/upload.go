package storage

import (
	"context"
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/httpx"
)

var (
	ErrUnavailable   = httpx.E(503, "STORAGE_UNAVAILABLE", "Object storage is not configured")
	ErrImageRequired = httpx.E(400, "IMAGE_REQUIRED", "An image file is required")
	ErrImageTooLarge = httpx.E(400, "IMAGE_TOO_LARGE", "Image must be 5 MB or smaller")
	ErrInvalidImage  = httpx.E(400, "INVALID_IMAGE", "Only JPEG, PNG or WebP images")
)

// ReadFile reads a multipart field, refusing anything over max bytes.
// Returns (nil, nil) when the field is missing.
func ReadFile(c *fiber.Ctx, field string, max int64) ([]byte, string, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		return nil, "", nil
	}
	if fh.Size > max {
		return nil, fh.Filename, fiber.ErrRequestEntityTooLarge
	}
	f, err := fh.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(b)) > max {
		return nil, fh.Filename, fiber.ErrRequestEntityTooLarge
	}
	return b, fh.Filename, err
}

// UploadImage validates an image field (magic bytes, size) and stores it
// under feature. Returns the key and its public URL ("" if private).
func (s *Store) UploadImage(c *fiber.Ctx, field, feature, scope string, max int64) (key, url string, err error) {
	body, _, err := ReadFile(c, field, max)
	if err == fiber.ErrRequestEntityTooLarge {
		return "", "", ErrImageTooLarge
	}
	if err != nil {
		return "", "", err
	}
	if len(body) == 0 {
		return "", "", ErrImageRequired
	}
	mime, ext, ok := DetectImage(body)
	if !ok {
		return "", "", ErrInvalidImage
	}
	if s == nil {
		return "", "", ErrUnavailable
	}
	key = Key(feature, scope, uuid.Must(uuid.NewV7()).String(), ext, time.Now())
	if err := s.Put(c.UserContext(), key, body, mime); err != nil {
		return "", "", err
	}
	return key, s.URL(key), nil
}

// RemoveOwned deletes a file only if it is ours (a key under feature/public).
// Failures are swallowed: an orphan file is better than a failed save.
func (s *Store) RemoveOwned(ctx context.Context, urlOrKey, feature string) {
	key := KeyFromURL(urlOrKey)
	if s == nil || key == "" || len(key) < len(feature)+8 || key[:len(feature)+8] != feature+"/public/" {
		return
	}
	_ = s.Remove(ctx, key)
}
