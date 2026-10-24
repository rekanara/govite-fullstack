// Package audit records mutations and serves the audit log.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
	"govite/internal/xport"
)

var sensitive = map[string]bool{
	"passwordHash": true, "password": true, "tokenHash": true, "token": true,
	"accessToken": true, "refreshToken": true, "backupCodes": true, "currentPassword": true,
	"newPassword": true, "manualKey": true, "otpauthUrl": true, "key": true,
}

// Redact round-trips v through JSON and drops secret keys at any depth.
func Redact(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var generic any
	if json.Unmarshal(raw, &generic) != nil {
		return raw
	}
	out, _ := json.Marshal(strip(generic))
	return out
}

func strip(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if sensitive[k] {
				delete(t, k)
			} else {
				t[k] = strip(val)
			}
		}
	case []any:
		for i := range t {
			t[i] = strip(t[i])
		}
	}
	return v
}

type Log struct{ q *store.Queries }

func New(d *app.Deps) *Log { return &Log{q: d.Q} }

// Record writes one audit row. A failed write is logged, never returned:
// an audit hiccup must not turn a successful action into an error.
func (l *Log) Record(ctx context.Context, actorID, action, entityType, entityID string, before, after any) {
	err := l.q.InsertAudit(ctx, store.InsertAuditParams{
		ID: app.NewID(), ActorUserID: httpx.Ptr(actorID), Action: action,
		EntityType: entityType, EntityID: entityID, Before: Redact(before), After: Redact(after),
	})
	if err != nil {
		slog.ErrorContext(ctx, "audit write failed", "err", err, "action", action)
	}
}

// From records using the request's caller as actor.
func (l *Log) From(c *fiber.Ctx, action, entityType, entityID string, before, after any) {
	l.Record(c.UserContext(), app.CurrentUser(c).ID, action, entityType, entityID, before, after)
}

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

func (q query) params() store.ListAuditParams {
	p := store.ListAuditParams{
		ActorUserID: httpx.Ptr(q.ActorUserID), UserID: httpx.Ptr(q.UserID),
		EntityType: httpx.Ptr(q.EntityType), EntityID: httpx.Ptr(q.EntityID), Action: httpx.Ptr(q.Action),
	}
	if t, err := time.Parse(time.DateOnly, q.From); err == nil {
		p.FromAt = &t
	}
	if t, err := time.Parse(time.DateOnly, q.To); err == nil {
		t = t.AddDate(0, 0, 1) // `to` includes the whole day
		p.ToAt = &t
	}
	return p
}

type entry struct {
	ID          string          `json:"id"`
	ActorUserID *string         `json:"actorUserId"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entityType"`
	EntityID    string          `json:"entityId"`
	Before      json.RawMessage `json:"before"`
	After       json.RawMessage `json:"after"`
	CreatedAt   time.Time       `json:"createdAt"`
}

func toEntry(r store.AuditLog) entry {
	null := json.RawMessage("null")
	if r.Before == nil {
		r.Before = null
	}
	if r.After == nil {
		r.After = null
	}
	return entry{r.ID, r.ActorUserID, r.Action, r.EntityType, r.EntityID, r.Before, r.After, r.CreatedAt}
}

// ponytail: exports are built in memory; stream rows when this cap is hit.
const exportLimit = 10_000

func (l *Log) Routes(g rbac.Guarded) {
	g.Get("/audit", "audit.view", func(c *fiber.Ctx) error {
		var q query
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		p := q.params()
		total, err := l.q.CountAudit(c.UserContext(), store.CountAuditParams{
			ActorUserID: p.ActorUserID, EntityType: p.EntityType, EntityID: p.EntityID,
			Action: p.Action, UserID: p.UserID, FromAt: p.FromAt, ToAt: p.ToAt,
		})
		if err != nil {
			return err
		}
		p.Lim, p.Off = int32(q.Limit), q.Offset()
		rows, err := l.q.ListAudit(c.UserContext(), p)
		if err != nil {
			return err
		}
		out := make([]entry, len(rows))
		for i, r := range rows {
			out[i] = toEntry(r)
		}
		return httpx.Page(c, out, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/audit/export", "audit.view", func(c *fiber.Ctx) error {
		var q query
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		p := q.params()
		p.Lim = exportLimit
		rows, err := l.q.ListAudit(c.UserContext(), p)
		if err != nil {
			return err
		}
		table := [][]any{{"createdAt", "action", "entityType", "entityId", "actorUserId", "before", "after"}}
		for _, r := range rows {
			actor := ""
			if r.ActorUserID != nil {
				actor = *r.ActorUserID
			}
			table = append(table, []any{r.CreatedAt.UTC().Format(time.RFC3339Nano), r.Action, r.EntityType, r.EntityID, actor, orNull(r.Before), orNull(r.After)})
		}
		name := "audit-" + time.Now().UTC().Format(time.DateOnly) + ".csv"
		return httpx.File(c, name, "text/csv; charset=utf-8", xport.CSV(table))
	})
}

func orNull(r json.RawMessage) string {
	if len(r) == 0 {
		return "null"
	}
	return strings.TrimSpace(string(r))
}
