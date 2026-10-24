// Package http mounts the Midtrans callback, webhook admin and API keys.
package http

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"govite/internal/integrations/application"
	"govite/internal/integrations/domain"
	"govite/internal/kernel"
	"govite/internal/platform/httpx"
	"govite/internal/rbac"
)

type Handler struct{ Svc *application.Service }

var signature = regexp.MustCompile(`(?i)^[a-f0-9]{128}$`)

// midtransPayload validates the untrusted callback body.
func midtransPayload(body []byte) (map[string]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, httpx.Validation("body: invalid JSON")
	}
	p := make(map[string]string, len(raw))
	for k, v := range raw {
		str, ok := v.(string)
		if !ok {
			return nil, httpx.Validation(k + ": expected string")
		}
		p[k] = str
	}
	var problems []string
	check := func(field string, max int, required bool) {
		v := p[field]
		if (required && v == "") || len(v) > max {
			problems = append(problems, field+": invalid")
		}
	}
	check("order_id", 100, true)
	check("status_code", 10, true)
	check("gross_amount", 30, true)
	check("transaction_status", 50, false)
	check("transaction_id", 100, false)
	if !signature.MatchString(p["signature_key"]) {
		problems = append(problems, "signature_key: invalid")
	}
	if len(problems) > 0 {
		return nil, httpx.Validation(strings.Join(problems, "; "))
	}
	return p, nil
}

// PublicRoutes: the provider callback. Throttled separately (providers retry in bursts).
func (h Handler) PublicRoutes(r fiber.Router, limit fiber.Handler) {
	r.Post("/webhooks/midtrans", limit, func(c *fiber.Ctx) error {
		p, err := midtransPayload(c.Body())
		if err != nil {
			return err
		}
		dup, err := h.Svc.ReceiveMidtrans(c.UserContext(), p)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"duplicate": dup})
	})
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/webhooks/events", "webhooks.manage", func(c *fiber.Ctx) error {
		var q struct {
			httpx.Pagination
			Provider string `query:"provider" validate:"max=50"`
			Status   string `query:"status" validate:"omitempty,oneof=received processed failed"`
		}
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		rows, total, err := h.Svc.Events(c.UserContext(), kernel.Ptr(strings.TrimSpace(q.Provider)), kernel.Ptr(q.Status), int32(q.Limit), q.Offset())
		if err != nil {
			return err
		}
		return httpx.Page(c, rows, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Post("/webhooks/events/:id/replay", "webhooks.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		dup, err := h.Svc.Replay(c.UserContext(), id)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"duplicate": dup})
	})

	g.Get("/api-keys", "apiKeys.manage", func(c *fiber.Ctx) error {
		rows, err := h.Svc.Keys(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.OK(c, rows)
	})

	g.Post("/api-keys", "apiKeys.manage", func(c *fiber.Ctx) error {
		var b struct {
			Name string `json:"name" validate:"required,min=1,max=100"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		k, secret, err := h.Svc.CreateKey(c.UserContext(), b.Name)
		if err != nil {
			return err
		}
		return httpx.Created(c, struct {
			domain.APIKey
			Key string `json:"key"`
		}{k, secret})
	})

	g.Patch("/api-keys/:id", "apiKeys.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b struct {
			Name     *string `json:"name" validate:"omitempty,min=1,max=100"`
			IsActive *bool   `json:"isActive"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		k, err := h.Svc.UpdateKey(c.UserContext(), id, httpx.TrimPtr(b.Name), b.IsActive)
		if err != nil {
			return err
		}
		return httpx.OK(c, k)
	})

	g.Delete("/api-keys/:id", "apiKeys.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		if err := h.Svc.DeleteKey(c.UserContext(), id); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})
}
