// Package http serves the audit log.
package http

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/audit/application"
	"govite/internal/audit/domain"
	"govite/internal/kernel"
	"govite/internal/platform/httpx"
	"govite/internal/platform/xport"
	"govite/internal/rbac"
)

type Handler struct{ Svc *application.Service }

type query struct {
	httpx.Pagination
	ActorUserID string `query:"actorUserId" validate:"omitempty,uuid"`
	UserID      string `query:"userId" validate:"omitempty,uuid"`
	EntityType  string `query:"entityType" validate:"max=50"`
	EntityID    string `query:"entityId" validate:"max=100"`
	Action      string `query:"action" validate:"max=100"`
	From        string `query:"from" validate:"omitempty,datetime=2006-01-02"`
	To          string `query:"to" validate:"omitempty,datetime=2006-01-02"`
}

func (q query) filter() domain.Filter {
	f := domain.Filter{
		ActorUserID: kernel.Ptr(q.ActorUserID), UserID: kernel.Ptr(q.UserID),
		EntityType: kernel.Ptr(q.EntityType), EntityID: kernel.Ptr(q.EntityID), Action: kernel.Ptr(q.Action),
	}
	if t, err := time.Parse(time.DateOnly, q.From); err == nil {
		f.From = &t
	}
	if t, err := time.Parse(time.DateOnly, q.To); err == nil {
		t = t.AddDate(0, 0, 1) // `to` includes the whole day
		f.To = &t
	}
	return f
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/audit", "audit.view", func(c *fiber.Ctx) error {
		var q query
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		rows, total, err := h.Svc.List(c.UserContext(), q.filter(), int32(q.Limit), q.Offset())
		if err != nil {
			return err
		}
		return httpx.Page(c, rows, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/audit/export", "audit.view", func(c *fiber.Ctx) error {
		var q query
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		rows, err := h.Svc.Export(c.UserContext(), q.filter())
		if err != nil {
			return err
		}
		table := [][]any{{"createdAt", "action", "entityType", "entityId", "actorUserId", "before", "after"}}
		for _, r := range rows {
			actor := ""
			if r.ActorUserID != nil {
				actor = *r.ActorUserID
			}
			table = append(table, []any{r.CreatedAt.UTC().Format(time.RFC3339Nano), r.Action, r.EntityType, r.EntityID, actor, strings.TrimSpace(string(r.Before)), strings.TrimSpace(string(r.After))})
		}
		name := "audit-" + time.Now().UTC().Format(time.DateOnly) + ".csv"
		return httpx.File(c, name, "text/csv; charset=utf-8", xport.CSV(table))
	})
}
