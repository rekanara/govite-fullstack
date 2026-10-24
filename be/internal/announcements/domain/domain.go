// Package domain is announcements: drafts, pinning, images, publish-once push.
package domain

import (
	"context"
	"strings"
	"time"

	"govite/internal/kernel"
)

var ErrNotFound = kernel.E(kernel.NotFound, "ANNOUNCEMENT_NOT_FOUND", "Announcement not found")

const (
	Draft      = "draft"
	Published  = "published"
	previewLen = 140
)

type Announcement struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	ImageURL    *string    `json:"imageUrl"`
	IsPinned    bool       `json:"isPinned"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt"`
	NotifiedAt  *time.Time `json:"notifiedAt"`
	AuthorName  string     `json:"authorName"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Preview flattens whitespace and caps the push body.
func Preview(s string) string {
	flat := strings.Join(strings.Fields(s), " ")
	if r := []rune(flat); len(r) > previewLen {
		return string(r[:previewLen-1]) + "…"
	}
	return flat
}

type New struct {
	ID, Title, Content, Status, AuthorID string
	ImageURL                             *string
	IsPinned                             bool
	PublishedAt                          *time.Time
}

// Changes: SetImage with ImageURL nil clears the image.
type Changes struct {
	Title, Content, Status *string
	SetImage               bool
	ImageURL               *string
	IsPinned               *bool
	PublishNow             bool
}

func (c Changes) Empty() bool {
	return c.Title == nil && c.Content == nil && !c.SetImage && c.IsPinned == nil && c.Status == nil
}

type Repository interface {
	Get(ctx context.Context, id string) (Announcement, error)
	Count(ctx context.Context, status, search *string) (int64, error)
	List(ctx context.Context, status, search *string, limit, offset int32) ([]Announcement, error)
	Create(ctx context.Context, a New) error
	Update(ctx context.Context, id string, c Changes) error
	Delete(ctx context.Context, id string) error
	// ClaimNotify marks it notified; false when already claimed.
	ClaimNotify(ctx context.Context, id string) (bool, error)
	ImageInUse(ctx context.Context, url string) (bool, error)
}

type Images interface {
	PutImage(ctx context.Context, body []byte, feature, scope string) (key, url string, err error)
	RemoveOwned(ctx context.Context, urlOrKey, feature string)
}

// Broadcaster pushes one announcement to every active user.
type Broadcaster interface {
	Announce(ctx context.Context, a Announcement) error
}
