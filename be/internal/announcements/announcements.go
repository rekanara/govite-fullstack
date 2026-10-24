// Package announcements: drafts, pinning, images; publishing pushes once.
package announcements

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/httpx"
	"govite/internal/notify"
	"govite/internal/rbac"
	"govite/internal/storage"
	"govite/internal/store"
)

var ErrNotFound = httpx.E(404, "ANNOUNCEMENT_NOT_FOUND", "Announcement not found")

const previewLen = 140

type Service struct {
	*app.Deps
	Notify *notify.Service
	Audit  *audit.Log
}

type View struct {
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

func toView(a store.Announcement, author string) View {
	return View{a.ID, a.Title, a.Content, a.ImageUrl, a.IsPinned, a.Status, a.PublishedAt, a.NotifiedAt, author, a.CreatedAt, a.UpdatedAt}
}

func preview(s string) string {
	flat := strings.Join(strings.Fields(s), " ")
	if r := []rune(flat); len(r) > previewLen {
		return string(r[:previewLen-1]) + "…"
	}
	return flat
}

type createReq struct {
	Title    string  `json:"title" validate:"required,min=1,max=200"`
	Content  string  `json:"content" validate:"required,min=1,max=10000"`
	ImageURL *string `json:"imageUrl" validate:"omitempty,url,max=2048"`
	IsPinned bool    `json:"isPinned"`
	Status   string  `json:"status" validate:"omitempty,oneof=draft published"`
}

// updateReq: an omitted field stays untouched; imageUrl:null clears it.
type updateReq struct {
	Title    *string         `json:"title" validate:"omitempty,min=1,max=200"`
	Content  *string         `json:"content" validate:"omitempty,min=1,max=10000"`
	ImageURL json.RawMessage `json:"imageUrl"`
	IsPinned *bool           `json:"isPinned"`
	Status   *string         `json:"status" validate:"omitempty,oneof=draft published"`
}

type listQuery struct {
	httpx.Pagination
	Status string `query:"status" validate:"omitempty,oneof=draft published"`
}

func (s *Service) get(ctx context.Context, id string, drafts bool) (View, error) {
	row, err := s.Q.GetAnnouncement(ctx, id)
	if err != nil || (!drafts && row.Announcement.Status != "published") {
		return View{}, ErrNotFound
	}
	return toView(row.Announcement, row.AuthorName), nil
}

func (s *Service) list(c *fiber.Ctx, all bool) error {
	var q listQuery
	if err := httpx.Query(c, &q); err != nil {
		return err
	}
	q.Normalize()
	status := httpx.Ptr(q.Status)
	if !all { // readers see published only
		pub := "published"
		status = &pub
	}
	ctx := c.UserContext()
	total, err := s.Q.CountAnnouncements(ctx, store.CountAnnouncementsParams{Status: status, Search: q.SearchPtr()})
	if err != nil {
		return err
	}
	rows, err := s.Q.ListAnnouncements(ctx, store.ListAnnouncementsParams{Status: status, Search: q.SearchPtr(), Lim: int32(q.Limit), Off: q.Offset()})
	if err != nil {
		return err
	}
	out := make([]View, len(rows))
	for i, r := range rows {
		out[i] = toView(r.Announcement, r.AuthorName)
	}
	return httpx.Page(c, out, httpx.NewPageMeta(total, q.Page, q.Limit))
}

// notifyOnce pushes a published announcement exactly once (claimed in SQL).
// Delivery failures never fail the save.
func (s *Service) notifyOnce(ctx context.Context, id string) {
	n, err := s.Q.ClaimAnnouncementNotify(ctx, id)
	if err != nil || n == 0 {
		if err != nil {
			slog.ErrorContext(ctx, "announcement claim failed", "err", err)
		}
		return
	}
	a, err := s.Q.GetAnnouncement(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, "announcement push failed", "err", err)
		return
	}
	img := ""
	if a.Announcement.ImageUrl != nil {
		img = *a.Announcement.ImageUrl
	}
	err = s.Notify.NotifyAllActive(ctx, notify.Input{
		Type: "announcement", Title: a.Announcement.Title, Body: preview(a.Announcement.Content),
		Data: map[string]string{"modules": "announcement", "id": id}, DedupeKey: "announcement:" + id, ImageURL: img,
	})
	if err != nil {
		slog.ErrorContext(ctx, "announcement push failed", "err", err)
	}
}

// removeOwnImage deletes a file only if we stored it and nothing else uses it.
func (s *Service) removeOwnImage(ctx context.Context, prev, next *string) {
	if prev == nil || (next != nil && *prev == *next) {
		return
	}
	if n, err := s.Q.CountAnnouncementsUsingImage(ctx, prev); err != nil || n > 0 {
		return
	}
	s.Files.RemoveOwned(ctx, *prev, "announcements")
}

func validID(c *fiber.Ctx) (string, error) {
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", httpx.Validation("id: must be a UUID")
	}
	return id, nil
}

func (s *Service) Routes(g rbac.Guarded) {
	g.Get("/announcements", "announcements.view", func(c *fiber.Ctx) error { return s.list(c, false) })
	// Declared before ":id" so "manage" is not parsed as an id.
	g.Get("/announcements/manage", "announcements.manage", func(c *fiber.Ctx) error { return s.list(c, true) })

	g.Post("/announcements/image", "announcements.manage", func(c *fiber.Ctx) error {
		key, url, err := s.Files.UploadImage(c, "image", "announcements", "", storage.MaxImageBytes)
		if err != nil {
			return err
		}
		return httpx.Created(c, fiber.Map{"key": key, "url": url})
	})

	g.Get("/announcements/manage/:id", "announcements.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.get(c.UserContext(), id, true)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Get("/announcements/:id", "announcements.view", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.get(c.UserContext(), id, false)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/announcements", "announcements.manage", func(c *fiber.Ctx) error {
		var b createReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if b.Status == "" {
			b.Status = "published"
		}
		ctx, id := c.UserContext(), app.NewID()
		var published *time.Time
		if b.Status == "published" {
			now := time.Now()
			published = &now
		}
		err := s.Q.CreateAnnouncement(ctx, store.CreateAnnouncementParams{ID: id, Title: strings.TrimSpace(b.Title), Content: strings.TrimSpace(b.Content), ImageUrl: b.ImageURL, IsPinned: b.IsPinned, Status: b.Status, PublishedAt: published, AuthorID: app.CurrentUser(c).ID})
		if err != nil {
			return err
		}
		if published != nil {
			s.notifyOnce(ctx, id)
		}
		v, err := s.get(ctx, id, true)
		if err != nil {
			return err
		}
		s.Audit.From(c, "announcement.create", "announcement", id, nil, v)
		return httpx.Created(c, v)
	})

	g.Patch("/announcements/:id", "announcements.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		var b updateReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		setImage, newImage := len(b.ImageURL) > 0, (*string)(nil)
		if setImage && string(b.ImageURL) != "null" {
			var u string
			if json.Unmarshal(b.ImageURL, &u) != nil || httpx.Check(&struct {
				U string `json:"imageUrl" validate:"url,max=2048"`
			}{u}) != nil {
				return httpx.Validation("imageUrl: url")
			}
			newImage = &u
		}
		if b.Title == nil && b.Content == nil && !setImage && b.IsPinned == nil && b.Status == nil {
			return httpx.Validation("body: Nothing to update")
		}
		ctx := c.UserContext()
		cur, err := s.Q.GetAnnouncement(ctx, id)
		if err != nil {
			return ErrNotFound
		}
		nowPublished := b.Status != nil && *b.Status == "published" && cur.Announcement.Status != "published"
		err = s.Q.UpdateAnnouncement(ctx, store.UpdateAnnouncementParams{
			ID: id, Title: httpx.TrimPtr(b.Title), Content: httpx.TrimPtr(b.Content),
			SetImageUrl: setImage, ImageUrl: newImage, IsPinned: b.IsPinned, Status: b.Status, PublishNow: nowPublished,
		})
		if err != nil {
			return err
		}
		if setImage {
			s.removeOwnImage(ctx, cur.Announcement.ImageUrl, newImage)
		}
		if nowPublished {
			s.notifyOnce(ctx, id)
		}
		v, err := s.get(ctx, id, true)
		if err != nil {
			return err
		}
		s.Audit.From(c, "announcement.update", "announcement", id, nil, v)
		return httpx.OK(c, v)
	})

	g.Delete("/announcements/:id", "announcements.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		ctx := c.UserContext()
		cur, err := s.Q.GetAnnouncement(ctx, id)
		if err != nil {
			return ErrNotFound
		}
		if err := s.Q.DeleteAnnouncement(ctx, id); err != nil {
			return err
		}
		s.removeOwnImage(ctx, cur.Announcement.ImageUrl, nil)
		s.Audit.From(c, "announcement.delete", "announcement", id, nil, nil)
		return httpx.NoContent(c)
	})
}
