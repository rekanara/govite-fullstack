// Package infrastructure stores announcements in Postgres and broadcasts
// them through notify.
package infrastructure

import (
	"context"

	"govite/internal/announcements/domain"
	notify "govite/internal/notify/application"
	notifydomain "govite/internal/notify/domain"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func toAnnouncement(a store.Announcement, author string) domain.Announcement {
	return domain.Announcement{ID: a.ID, Title: a.Title, Content: a.Content, ImageURL: a.ImageUrl, IsPinned: a.IsPinned, Status: a.Status, PublishedAt: a.PublishedAt, NotifiedAt: a.NotifiedAt, AuthorName: author, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}

func (r Repository) Get(ctx context.Context, id string) (domain.Announcement, error) {
	row, err := r.DB.Q(ctx).GetAnnouncement(ctx, id)
	return toAnnouncement(row.Announcement, row.AuthorName), postgres.Err(err)
}

func (r Repository) Count(ctx context.Context, status, search *string) (int64, error) {
	return r.DB.Q(ctx).CountAnnouncements(ctx, store.CountAnnouncementsParams{Status: status, Search: search})
}

func (r Repository) List(ctx context.Context, status, search *string, limit, offset int32) ([]domain.Announcement, error) {
	rows, err := r.DB.Q(ctx).ListAnnouncements(ctx, store.ListAnnouncementsParams{Status: status, Search: search, Lim: limit, Off: offset})
	out := make([]domain.Announcement, len(rows))
	for i, x := range rows {
		out[i] = toAnnouncement(x.Announcement, x.AuthorName)
	}
	return out, err
}

func (r Repository) Create(ctx context.Context, a domain.New) error {
	return r.DB.Q(ctx).CreateAnnouncement(ctx, store.CreateAnnouncementParams{ID: a.ID, Title: a.Title, Content: a.Content, ImageUrl: a.ImageURL, IsPinned: a.IsPinned, Status: a.Status, PublishedAt: a.PublishedAt, AuthorID: a.AuthorID})
}

func (r Repository) Update(ctx context.Context, id string, c domain.Changes) error {
	return r.DB.Q(ctx).UpdateAnnouncement(ctx, store.UpdateAnnouncementParams{ID: id, Title: c.Title, Content: c.Content, SetImageUrl: c.SetImage, ImageUrl: c.ImageURL, IsPinned: c.IsPinned, Status: c.Status, PublishNow: c.PublishNow})
}

func (r Repository) Delete(ctx context.Context, id string) error {
	return r.DB.Q(ctx).DeleteAnnouncement(ctx, id)
}

func (r Repository) ClaimNotify(ctx context.Context, id string) (bool, error) {
	n, err := r.DB.Q(ctx).ClaimAnnouncementNotify(ctx, id)
	return n > 0, err
}

func (r Repository) ImageInUse(ctx context.Context, url string) (bool, error) {
	n, err := r.DB.Q(ctx).CountAnnouncementsUsingImage(ctx, &url)
	return n > 0, err
}

// Broadcaster sends an announcement to every active user via notify.
type Broadcaster struct{ Notify *notify.Service }

func (b Broadcaster) Announce(ctx context.Context, a domain.Announcement) error {
	img := ""
	if a.ImageURL != nil {
		img = *a.ImageURL
	}
	return b.Notify.NotifyAllActive(ctx, notifydomain.Input{
		Type: "announcement", Title: a.Title, Body: domain.Preview(a.Content),
		Data: map[string]string{"modules": "announcement", "id": a.ID}, DedupeKey: "announcement:" + a.ID, ImageURL: img,
	})
}
