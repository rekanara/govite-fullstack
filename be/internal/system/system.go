// Package system serves health probes, the admin status page and the
// read-only feeds the admin dashboard needs (notifications, announcements).
package system

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
)

var started = time.Now()

// Version is set at build time: -ldflags "-X govite/internal/system.Version=1.2.3".
var Version = ""

type Service struct{ *app.Deps }

func probe(ctx context.Context, fn func(context.Context) error) (string, *int64) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	t := time.Now()
	if err := fn(ctx); err != nil {
		return "down", nil
	}
	ms := time.Since(t).Milliseconds()
	return "up", &ms
}

func (s *Service) checks(ctx context.Context) (db, redis fiber.Map) {
	dbStatus, dbMs := probe(ctx, s.Pool.Ping)
	redisStatus, redisMs := "down", (*int64)(nil)
	if s.Redis != nil {
		redisStatus, redisMs = probe(ctx, func(ctx context.Context) error { return s.Redis.Ping(ctx).Err() })
	}
	return fiber.Map{"status": dbStatus, "latencyMs": dbMs}, fiber.Map{"status": redisStatus, "latencyMs": redisMs}
}

func (s *Service) PublicRoutes(r fiber.Router) {
	live := func(c *fiber.Ctx) error { return httpx.OK(c, fiber.Map{"status": "ok"}) }
	r.Get("/health", live)
	r.Get("/health/live", live)
	r.Get("/health/ready", func(c *fiber.Ctx) error {
		db, redis := s.checks(c.UserContext())
		res := fiber.Map{"database": db["status"], "redis": redis["status"]}
		// Redis is optional: only required when configured.
		if db["status"] == "down" || (s.Redis != nil && redis["status"] == "down") {
			raw, _ := json.Marshal(res)
			return httpx.E(503, "NOT_READY", "Dependencies unavailable").WithDetail(string(raw))
		}
		res["status"] = "ok"
		return httpx.OK(c, res)
	})
}

func (s *Service) Routes(g rbac.Guarded) {
	g.Get("/system/status", "settings.manage", func(c *fiber.Ctx) error {
		db, redis := s.checks(c.UserContext())
		var version any
		if Version != "" {
			version = Version
		}
		return httpx.OK(c, fiber.Map{
			"version": version, "environment": s.Cfg.Env, "uptimeSeconds": int(time.Since(started).Seconds()),
			"checks":       fiber.Map{"database": db, "redis": redis},
			"integrations": fiber.Map{"storage": false, "push": false, "payment": false, "mailer": s.Cfg.SMTPHost != ""},
		})
	})

	g.Get("/announcements", "announcements.view", func(c *fiber.Ctx) error {
		var q httpx.Pagination
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		ctx := c.UserContext()
		total, err := s.Q.CountPublishedAnnouncements(ctx)
		if err != nil {
			return err
		}
		rows, err := s.Q.ListPublishedAnnouncements(ctx, store.ListPublishedAnnouncementsParams{Lim: int32(q.Limit), Off: q.Offset()})
		if err != nil {
			return err
		}
		out := make([]fiber.Map, len(rows))
		for i, r := range rows {
			a := r.Announcement
			out[i] = fiber.Map{"id": a.ID, "title": a.Title, "content": a.Content, "imageUrl": a.ImageUrl, "isPinned": a.IsPinned, "status": a.Status, "publishedAt": a.PublishedAt, "notifiedAt": a.NotifiedAt, "authorName": r.AuthorName, "createdAt": a.CreatedAt, "updatedAt": a.UpdatedAt}
		}
		return httpx.Page(c, out, httpx.NewPageMeta(total, q.Page, q.Limit))
	})
}

type notification struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data"`
	ReadAt    *time.Time      `json:"readAt"`
	CreatedAt time.Time       `json:"createdAt"`
}

func toNotification(n store.Notification) notification {
	return notification{n.ID, n.Type, n.Title, n.Body, n.Data, n.ReadAt, n.CreatedAt}
}

// NotificationRoutes: the caller's own inbox (auth only).
func (s *Service) NotificationRoutes(r fiber.Router) {
	r.Get("/notifications", func(c *fiber.Ctx) error {
		var q struct {
			httpx.Pagination
			UnreadOnly string `query:"unreadOnly" validate:"omitempty,oneof=true false"`
		}
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		counts, err := s.Q.CountNotifications(ctx, id)
		if err != nil {
			return err
		}
		unreadOnly := q.UnreadOnly == "true"
		rows, err := s.Q.ListNotifications(ctx, store.ListNotificationsParams{UserID: id, UnreadOnly: unreadOnly, Lim: int32(q.Limit), Off: q.Offset()})
		if err != nil {
			return err
		}
		out := make([]notification, len(rows))
		for i, n := range rows {
			out[i] = toNotification(n)
		}
		total := counts.Total
		if unreadOnly {
			total = counts.Unread
		}
		meta := httpx.NewPageMeta(total, q.Page, q.Limit)
		meta.UnreadCount = &counts.Unread
		return httpx.Page(c, out, meta)
	})

	r.Post("/notifications/read-all", func(c *fiber.Ctx) error {
		if err := s.Q.MarkAllNotificationsRead(c.UserContext(), app.CurrentUser(c).ID); err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	r.Post("/notifications/:id/read", func(c *fiber.Ctx) error {
		n, err := s.Q.MarkNotificationRead(c.UserContext(), store.MarkNotificationReadParams{ID: c.Params("id"), UserID: app.CurrentUser(c).ID})
		if err != nil {
			return httpx.E(404, "NOTIFICATION_NOT_FOUND", "Notification not found")
		}
		return httpx.Created(c, toNotification(n))
	})
}
