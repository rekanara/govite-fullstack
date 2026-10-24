// Package domain is the caller's avatar and private attachments.
package domain

import (
	"context"
	"strings"
	"time"

	"govite/internal/kernel"
)

var (
	ErrInvalidAvatar     = kernel.E(kernel.Invalid, "INVALID_AVATAR", "Avatar must be an image up to 2MB")
	ErrInvalidAttachment = kernel.E(kernel.Invalid, "INVALID_ATTACHMENT", "Attachment is missing or too large")
	ErrAttachmentGone    = kernel.E(kernel.NotFound, "ATTACHMENT_NOT_FOUND", "Attachment not found")
)

const (
	MaxAvatar     = 2_000_000
	MaxAttachment = 10_000_000
	SignedTTL     = time.Hour
)

type Attachment struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"-"`
	Key         string    `json:"-"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	Size        int32     `json:"size"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
}

var extensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "application/pdf": "pdf", "text/csv": "csv"}

// Ext maps a content type to a stored file extension; unknown is "bin".
func Ext(contentType string) string {
	if e, ok := extensions[strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))]; ok {
		return e
	}
	return "bin"
}

func Truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

type Repository interface {
	AvatarKey(ctx context.Context, userID string) (*string, error)
	SetAvatarKey(ctx context.Context, userID, key string) error
	Attachments(ctx context.Context, ownerID string) ([]Attachment, error)
	Attachment(ctx context.Context, id string) (Attachment, error)
	CreateAttachment(ctx context.Context, a Attachment) error
	DeleteAttachment(ctx context.Context, id string) error
}

// Storage is the object store. Unavailable when Enabled is false.
type Storage interface {
	Enabled() bool
	PutImage(ctx context.Context, body []byte, feature, scope string) (key, url string, err error)
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Remove(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key, downloadAs string, ttl time.Duration) (string, error)
	Key(feature, scope, name, ext string) string
}
