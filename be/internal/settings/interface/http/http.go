// Package http mounts the app settings endpoints.
package http

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"govite/internal/platform/httpx"
	"govite/internal/platform/storage"
	"govite/internal/rbac"
	"govite/internal/settings/application"
	"govite/internal/settings/domain"
)

type Handler struct{ Svc *application.Service }

// PublicRoutes: branding shown before sign-in.
func (h Handler) PublicRoutes(r fiber.Router) {
	r.Get("/settings/public", func(c *fiber.Ctx) error {
		v, err := h.Svc.Get(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"appName": v.AppName, "tagline": v.Tagline, "logoUrl": v.LogoURL, "bannerUrl": v.BannerURL, "supportEmail": v.SupportEmail, "loginNotice": v.LoginNotice})
	})
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/settings", "settings.manage", func(c *fiber.Ctx) error {
		v, err := h.Svc.Get(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Put("/settings", "settings.manage", func(c *fiber.Ctx) error {
		v := domain.Defaults()
		if err := json.Unmarshal(c.Body(), &v); err != nil {
			return httpx.Validation("body: invalid JSON")
		}
		v.Normalize()
		if err := httpx.Check(&v); err != nil {
			return err
		}
		if err := h.Svc.Put(c.UserContext(), v); err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/settings/image", "settings.manage", func(c *fiber.Ctx) error {
		up, err := httpx.FormFile(c, "image", storage.MaxImageBytes)
		if err == httpx.ErrTooLarge {
			return storage.ErrImageTooLarge
		}
		if err != nil {
			return err
		}
		url, err := h.Svc.UploadImage(c.UserContext(), up.Body)
		if err != nil {
			return err
		}
		return httpx.Created(c, fiber.Map{"url": url})
	})
}
