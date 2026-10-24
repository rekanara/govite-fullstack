// Package http mounts the announcement endpoints.
package http

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"govite/internal/announcements/application"
	"govite/internal/announcements/domain"
	"govite/internal/kernel"
	"govite/internal/platform/httpx"
	"govite/internal/platform/storage"
	"govite/internal/rbac"
)

type Handler struct{ Svc *application.Service }

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

func (b updateReq) changes() (domain.Changes, error) {
	c := domain.Changes{Title: httpx.TrimPtr(b.Title), Content: httpx.TrimPtr(b.Content), IsPinned: b.IsPinned, Status: b.Status, SetImage: len(b.ImageURL) > 0}
	if c.SetImage && string(b.ImageURL) != "null" {
		var u string
		if json.Unmarshal(b.ImageURL, &u) != nil || httpx.Check(&struct {
			U string `json:"imageUrl" validate:"url,max=2048"`
		}{u}) != nil {
			return c, httpx.Validation("imageUrl: url")
		}
		c.ImageURL = &u
	}
	return c, nil
}

func (h Handler) list(c *fiber.Ctx, all bool) error {
	var q struct {
		httpx.Pagination
		Status string `query:"status" validate:"omitempty,oneof=draft published"`
	}
	if err := httpx.Query(c, &q); err != nil {
		return err
	}
	q.Normalize()
	status := kernel.Ptr(q.Status)
	if !all { // readers see published only
		pub := domain.Published
		status = &pub
	}
	rows, total, err := h.Svc.List(c.UserContext(), status, q.SearchPtr(), int32(q.Limit), q.Offset())
	if err != nil {
		return err
	}
	return httpx.Page(c, rows, httpx.NewPageMeta(total, q.Page, q.Limit))
}

func (h Handler) get(drafts bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		v, err := h.Svc.Get(c.UserContext(), id, drafts)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	}
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/announcements", "announcements.view", func(c *fiber.Ctx) error { return h.list(c, false) })
	// Declared before ":id" so "manage" is not parsed as an id.
	g.Get("/announcements/manage", "announcements.manage", func(c *fiber.Ctx) error { return h.list(c, true) })

	g.Post("/announcements/image", "announcements.manage", func(c *fiber.Ctx) error {
		up, err := httpx.FormFile(c, "image", storage.MaxImageBytes)
		if err == httpx.ErrTooLarge {
			return storage.ErrImageTooLarge
		}
		if err != nil {
			return err
		}
		key, url, err := h.Svc.UploadImage(c.UserContext(), up.Body)
		if err != nil {
			return err
		}
		return httpx.Created(c, fiber.Map{"key": key, "url": url})
	})

	g.Get("/announcements/manage/:id", "announcements.manage", h.get(true))
	g.Get("/announcements/:id", "announcements.view", h.get(false))

	g.Post("/announcements", "announcements.manage", func(c *fiber.Ctx) error {
		var b createReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.Create(c.UserContext(), b.Title, b.Content, b.ImageURL, b.IsPinned, b.Status)
		if err != nil {
			return err
		}
		return httpx.Created(c, v)
	})

	g.Patch("/announcements/:id", "announcements.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b updateReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ch, err := b.changes()
		if err != nil {
			return err
		}
		v, err := h.Svc.Update(c.UserContext(), id, ch)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Delete("/announcements/:id", "announcements.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		if err := h.Svc.Delete(c.UserContext(), id); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})
}
