// Package application holds the avatar and attachment use cases.
package application

import (
	"context"

	"govite/internal/files/domain"
	"govite/internal/kernel"
)

type Service struct {
	Repo  domain.Repository
	Store domain.Storage
	Audit kernel.Auditor
}

func owner(ctx context.Context) string { return kernel.PrincipalFrom(ctx).ID }

// SetAvatar stores the new image first, so a failed upload never loses the old one.
func (s *Service) SetAvatar(ctx context.Context, body []byte) (string, error) {
	if !s.Store.Enabled() {
		return "", kernel.ErrStorageUnavailable
	}
	id := owner(ctx)
	prev, err := s.Repo.AvatarKey(ctx, id)
	if err != nil {
		return "", err
	}
	key, url, err := s.Store.PutImage(ctx, body, "avatars", id)
	switch err {
	case nil:
	case kernel.ErrImageRequired, kernel.ErrInvalidImage:
		return "", domain.ErrInvalidAvatar
	default:
		return "", err
	}
	if err := s.Repo.SetAvatarKey(ctx, id, key); err != nil {
		return "", err
	}
	if prev != nil && *prev != key {
		_ = s.Store.Remove(ctx, *prev)
	}
	s.Audit.Record(ctx, "account.avatar.update", "user", id, nil, map[string]string{"avatarUrl": url})
	return url, nil
}

func (s *Service) sign(ctx context.Context, a domain.Attachment) (domain.Attachment, error) {
	var err error
	a.URL, err = s.Store.SignedURL(ctx, a.Key, a.Filename, domain.SignedTTL)
	return a, err
}

func (s *Service) List(ctx context.Context) ([]domain.Attachment, error) {
	if !s.Store.Enabled() {
		return []domain.Attachment{}, nil
	}
	rows, err := s.Repo.Attachments(ctx, owner(ctx))
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i], err = s.sign(ctx, rows[i]); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func (s *Service) Upload(ctx context.Context, body []byte, name, contentType string) (domain.Attachment, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	a := domain.Attachment{ID: kernel.NewID(), OwnerUserID: owner(ctx), Filename: domain.Truncate(name, 255), ContentType: contentType, Size: int32(len(body))}
	a.Key = s.Store.Key("misc", a.OwnerUserID, kernel.NewID(), domain.Ext(contentType))
	if err := s.Store.Put(ctx, a.Key, body, contentType); err != nil {
		return a, err
	}
	if err := s.Repo.CreateAttachment(ctx, a); err != nil {
		_ = s.Store.Remove(ctx, a.Key)
		return a, err
	}
	stored, err := s.Repo.Attachment(ctx, a.ID)
	if err != nil {
		return a, err
	}
	return s.sign(ctx, stored)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	a, err := s.Repo.Attachment(ctx, id)
	if err != nil || a.OwnerUserID != owner(ctx) {
		return domain.ErrAttachmentGone
	}
	if err := s.Repo.DeleteAttachment(ctx, id); err != nil {
		return err
	}
	if s.Store.Enabled() {
		_ = s.Store.Remove(ctx, a.Key)
	}
	return nil
}
