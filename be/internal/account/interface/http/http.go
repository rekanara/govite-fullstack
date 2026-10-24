// Package http mounts the caller's profile endpoints under /account.
package http

import (
	"github.com/gofiber/fiber/v2"

	"govite/internal/account/application"
	"govite/internal/platform/httpx"
)

type Handler struct{ Svc *application.Service }

func (h Handler) Routes(r fiber.Router) {
	g := r.Group("/account")

	g.Get("/notifications/preferences", func(c *fiber.Ctx) error {
		on, err := h.Svc.PushEnabled(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"pushEnabled": on})
	})

	g.Patch("/notifications/preferences", func(c *fiber.Ctx) error {
		var b struct {
			PushEnabled *bool `json:"pushEnabled" validate:"required"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := h.Svc.SetPushEnabled(c.UserContext(), *b.PushEnabled); err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"pushEnabled": *b.PushEnabled})
	})

	g.Patch("/profile", func(c *fiber.Ctx) error {
		var b struct {
			FullName string `json:"fullName" validate:"required,min=1,max=100"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		id, name, err := h.Svc.SetFullName(c.UserContext(), b.FullName)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"id": id, "fullName": name})
	})
}
