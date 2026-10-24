package storage

import (
	"context"
	"time"

	"govite/internal/kernel"
)

var (
	ErrUnavailable   = kernel.ErrStorageUnavailable
	ErrImageRequired = kernel.ErrImageRequired
	ErrImageTooLarge = kernel.ErrImageTooLarge
	ErrInvalidImage  = kernel.ErrInvalidImage
)

// PutImage validates image bytes (magic bytes, never the client mimetype)
// and stores them under feature. Returns the key and its public URL.
func (s *Store) PutImage(ctx context.Context, body []byte, feature, scope string) (key, url string, err error) {
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
	key = Key(feature, scope, kernel.NewID(), ext, time.Now())
	if err := s.Put(ctx, key, body, mime); err != nil {
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
