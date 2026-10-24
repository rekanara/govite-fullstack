// Package http mounts the caller's inbox and device registration.
package http

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"govite/internal/notify/application"
	"govite/internal/platform/httpx"
)

type Handler struct{ Svc *application.Service }

func (h Handler) Routes(r fiber.Router) {
	r.Post("/notifications/devices", func(c *fiber.Ctx) error {
		var b struct {
			DeviceID string         `json:"deviceId" validate:"required,min=1,max=191"`
			Token    string         `json:"token" validate:"required,min=1,max=1024"`
			Device   map[string]any `json:"device"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if b.Device == nil {
			b.Device = map[string]any{}
		}
		raw, _ := json.Marshal(b.Device)
		if err := h.Svc.RegisterDevice(c.UserContext(), b.DeviceID, b.Token, raw); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	r.Delete("/notifications/devices/:deviceId", func(c *fiber.Ctx) error {
		if err := h.Svc.DeleteDevice(c.UserContext(), c.Params("deviceId")); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	r.Get("/notifications", func(c *fiber.Ctx) error {
		var q struct {
			httpx.Pagination
			UnreadOnly string `query:"unreadOnly" validate:"omitempty,oneof=true false"`
		}
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		rows, total, unread, err := h.Svc.Inbox(c.UserContext(), q.UnreadOnly == "true", int32(q.Limit), q.Offset())
		if err != nil {
			return err
		}
		meta := httpx.NewPageMeta(total, q.Page, q.Limit)
		meta.UnreadCount = &unread
		return httpx.Page(c, rows, meta)
	})

	r.Post("/notifications/read-all", func(c *fiber.Ctx) error {
		if err := h.Svc.MarkAllRead(c.UserContext()); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	r.Post("/notifications/:id/read", func(c *fiber.Ctx) error {
		n, err := h.Svc.MarkRead(c.UserContext(), c.Params("id"))
		if err != nil {
			return err
		}
		return httpx.Created(c, n)
	})
}
