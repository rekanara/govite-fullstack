// Package http mounts avatar and attachment uploads for the caller.
package http

import (
	"github.com/gofiber/fiber/v2"

	"govite/internal/files/application"
	"govite/internal/files/domain"
	"govite/internal/platform/httpx"
	"govite/internal/platform/storage"
)

type Handler struct{ Svc *application.Service }

func (h Handler) Routes(r fiber.Router) {
	r.Post("/account/avatar", func(c *fiber.Ctx) error {
		if !h.Svc.Store.Enabled() {
			return storage.ErrUnavailable
		}
		up, err := httpx.FormFile(c, "file", domain.MaxAvatar)
		if err == httpx.ErrTooLarge {
			return domain.ErrInvalidAvatar
		}
		if err != nil {
			return err
		}
		url, err := h.Svc.SetAvatar(c.UserContext(), up.Body)
		if err != nil {
			return err
		}
		return httpx.Created(c, fiber.Map{"avatarUrl": url})
	})

	r.Get("/attachments", func(c *fiber.Ctx) error {
		rows, err := h.Svc.List(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, rows)
	})

	r.Post("/attachments", func(c *fiber.Ctx) error {
		if !h.Svc.Store.Enabled() {
			return storage.ErrUnavailable
		}
		up, err := httpx.FormFile(c, "file", domain.MaxAttachment)
		if err == httpx.ErrTooLarge || (err == nil && len(up.Body) == 0) {
			return domain.ErrInvalidAttachment
		}
		if err != nil {
			return err
		}
		a, err := h.Svc.Upload(c.UserContext(), up.Body, up.Name, up.ContentType)
		if err != nil {
			return err
		}
		return httpx.Created(c, a)
	})

	r.Delete("/attachments/:id", func(c *fiber.Ctx) error {
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
