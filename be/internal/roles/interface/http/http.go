// Package http serves roles and the permission catalog.
package http

import (
	"github.com/gofiber/fiber/v2"

	"govite/internal/platform/httpx"
	"govite/internal/policy"
	"govite/internal/rbac"
	"govite/internal/roles/application"
	"govite/internal/roles/domain"
)

type Handler struct{ Svc *application.Service }

type createReq struct {
	Name             string  `json:"name" validate:"required,min=1,max=50"`
	Description      *string `json:"description" validate:"omitempty,max=200"`
	RequireTwoFactor bool    `json:"requireTwoFactor"`
}

type updateReq struct {
	Name             *string `json:"name" validate:"omitempty,min=1,max=50"`
	Description      *string `json:"description" validate:"omitempty,max=200"`
	RequireTwoFactor *bool   `json:"requireTwoFactor"`
}

type permsReq struct {
	Permissions []rbac.Grant `json:"permissions" validate:"max=200,dive"`
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/permissions", "permissions.view", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{
			"permissions": policy.Permissions, "resources": policy.Resources,
			"actions": policy.Actions, "scopes": policy.Scopes,
		})
	})

	g.Get("/roles", "roles.manage", func(c *fiber.Ctx) error {
		var q httpx.Pagination
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		rows, total, err := h.Svc.List(c.UserContext(), q.SearchPtr(), int32(q.Limit), q.Offset())
		if err != nil {
			return err
		}
		return httpx.Page(c, rows, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		v, err := h.Svc.Get(c.UserContext(), id)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/roles", "roles.manage", func(c *fiber.Ctx) error {
		var b createReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.Create(c.UserContext(), b.Name, httpx.TrimPtr(b.Description), b.RequireTwoFactor, c.AllParams())
		if err != nil {
			return err
		}
		return httpx.Created(c, v)
	})

	g.Patch("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b updateReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.Update(c.UserContext(), id, domain.Changes{Name: httpx.TrimPtr(b.Name), Description: httpx.TrimPtr(b.Description), RequireTwoFactor: b.RequireTwoFactor}, c.AllParams())
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Delete("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		if err := h.Svc.Delete(c.UserContext(), id, c.AllParams()); err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"id": id})
	})

	g.Put("/roles/:id/permissions", "roles.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b permsReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.SetPermissions(c.UserContext(), id, b.Permissions, c.AllParams())
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})
}
