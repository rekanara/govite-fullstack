// Package infrastructure stores file metadata in Postgres and adapts the
// object store.
package infrastructure

import (
	"context"
	"time"

	"govite/internal/files/domain"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
	"govite/internal/platform/storage"
)

type Repository struct{ DB *postgres.DB }

func toAttachment(a store.Attachment) domain.Attachment {
	return domain.Attachment{ID: a.ID, OwnerUserID: a.OwnerUserID, Key: a.Key, Filename: a.Filename, ContentType: a.ContentType, Size: a.Size, CreatedAt: a.CreatedAt}
}

func (r Repository) AvatarKey(ctx context.Context, userID string) (*string, error) {
	u, err := r.DB.Q(ctx).GetUserByID(ctx, userID)
	return u.AvatarKey, postgres.Err(err)
}

func (r Repository) SetAvatarKey(ctx context.Context, userID, key string) error {
	return r.DB.Q(ctx).SetAvatarKey(ctx, store.SetAvatarKeyParams{ID: userID, AvatarKey: &key})
}

func (r Repository) Attachments(ctx context.Context, ownerID string) ([]domain.Attachment, error) {
	rows, err := r.DB.Q(ctx).ListAttachments(ctx, ownerID)
	out := make([]domain.Attachment, len(rows))
	for i, a := range rows {
		out[i] = toAttachment(a)
	}
	return out, err
}

func (r Repository) Attachment(ctx context.Context, id string) (domain.Attachment, error) {
	a, err := r.DB.Q(ctx).GetAttachment(ctx, id)
	return toAttachment(a), postgres.Err(err)
}

func (r Repository) CreateAttachment(ctx context.Context, a domain.Attachment) error {
	return r.DB.Q(ctx).CreateAttachment(ctx, store.CreateAttachmentParams{ID: a.ID, OwnerUserID: a.OwnerUserID, Key: a.Key, Filename: a.Filename, ContentType: a.ContentType, Size: a.Size})
}

func (r Repository) DeleteAttachment(ctx context.Context, id string) error {
	return r.DB.Q(ctx).DeleteAttachment(ctx, id)
}

// Storage adapts *storage.Store (nil-safe) to domain.Storage.
type Storage struct{ *storage.Store }

func (s Storage) Key(feature, scope, name, ext string) string {
	return storage.Key(feature, scope, name, ext, time.Now())
}
