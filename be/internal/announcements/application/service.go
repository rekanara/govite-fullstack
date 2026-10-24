// Package application holds the announcement use cases.
package application

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"govite/internal/announcements/domain"
	"govite/internal/kernel"
)

type Service struct {
	Repo   domain.Repository
	Images domain.Images
	Push   domain.Broadcaster
	Audit  kernel.Auditor
}

// Get hides drafts unless drafts is true.
func (s *Service) Get(ctx context.Context, id string, drafts bool) (domain.Announcement, error) {
	a, err := s.Repo.Get(ctx, id)
	if err != nil || (!drafts && a.Status != domain.Published) {
		return domain.Announcement{}, domain.ErrNotFound
	}
	return a, nil
}

func (s *Service) List(ctx context.Context, status, search *string, limit, offset int32) ([]domain.Announcement, int64, error) {
	total, err := s.Repo.Count(ctx, status, search)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Repo.List(ctx, status, search, limit, offset)
	return rows, total, err
}

func (s *Service) UploadImage(ctx context.Context, body []byte) (key, url string, err error) {
	return s.Images.PutImage(ctx, body, "announcements", "")
}

// notifyOnce pushes a published announcement exactly once (claimed in SQL).
// Delivery failures never fail the save.
func (s *Service) notifyOnce(ctx context.Context, id string) {
	ok, err := s.Repo.ClaimNotify(ctx, id)
	if err != nil || !ok {
		if err != nil {
			slog.ErrorContext(ctx, "announcement claim failed", "err", err)
		}
		return
	}
	a, err := s.Repo.Get(ctx, id)
	if err == nil {
		err = s.Push.Announce(ctx, a)
	}
	if err != nil {
		slog.ErrorContext(ctx, "announcement push failed", "err", err)
	}
}

// removeOwnImage deletes a file only if we stored it and nothing else uses it.
func (s *Service) removeOwnImage(ctx context.Context, prev, next *string) {
	if prev == nil || (next != nil && *prev == *next) {
		return
	}
	if used, err := s.Repo.ImageInUse(ctx, *prev); err != nil || used {
		return
	}
	s.Images.RemoveOwned(ctx, *prev, "announcements")
}

func (s *Service) Create(ctx context.Context, title, content string, image *string, pinned bool, status string) (domain.Announcement, error) {
	if status == "" {
		status = domain.Published
	}
	n := domain.New{ID: kernel.NewID(), Title: strings.TrimSpace(title), Content: strings.TrimSpace(content), ImageURL: image, IsPinned: pinned, Status: status, AuthorID: kernel.PrincipalFrom(ctx).ID}
	if status == domain.Published {
		now := time.Now()
		n.PublishedAt = &now
	}
	if err := s.Repo.Create(ctx, n); err != nil {
		return domain.Announcement{}, err
	}
	if n.PublishedAt != nil {
		s.notifyOnce(ctx, n.ID)
	}
	a, err := s.Repo.Get(ctx, n.ID)
	if err == nil {
		s.Audit.Record(ctx, "announcement.create", "announcement", n.ID, nil, a)
	}
	return a, err
}

func (s *Service) Update(ctx context.Context, id string, c domain.Changes) (domain.Announcement, error) {
	if c.Empty() {
		return domain.Announcement{}, kernel.Validation("body: Nothing to update")
	}
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, domain.ErrNotFound
	}
	c.PublishNow = c.Status != nil && *c.Status == domain.Published && cur.Status != domain.Published
	if err := s.Repo.Update(ctx, id, c); err != nil {
		return cur, err
	}
	if c.SetImage {
		s.removeOwnImage(ctx, cur.ImageURL, c.ImageURL)
	}
	if c.PublishNow {
		s.notifyOnce(ctx, id)
	}
	a, err := s.Repo.Get(ctx, id)
	if err == nil {
		s.Audit.Record(ctx, "announcement.update", "announcement", id, nil, a)
	}
	return a, err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return domain.ErrNotFound
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	s.removeOwnImage(ctx, cur.ImageURL, nil)
	s.Audit.Record(ctx, "announcement.delete", "announcement", id, nil, nil)
	return nil
}
