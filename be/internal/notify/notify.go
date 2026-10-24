// Package notify writes in-app notifications, pushes them to devices and
// serves the caller's inbox and device registration.
package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/store"
)

const (
	userBatch = 500
	tokenPage = 1000
)

type Input struct {
	Type      string // system | security | admin | announcement
	Title     string
	Body      string
	Data      map[string]string
	DedupeKey string // one notification per user per key
	ImageURL  string
}

type Service struct {
	*app.Deps
	Push Pusher // nil = push disabled
	// PushAllowed is the admin kill switch (app settings).
	PushAllowed func(context.Context) bool
}

// Notify writes the in-app row for each user (deduplicated) and pushes to
// those who got a new row. Failures are logged; they never fail the caller.
func (s *Service) Notify(ctx context.Context, userIDs []string, in Input) {
	seen := map[string]bool{}
	var unique []string
	for _, id := range userIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	for i := 0; i < len(unique); i += userBatch {
		if err := s.deliver(ctx, unique[i:min(i+userBatch, len(unique))], in); err != nil {
			slog.ErrorContext(ctx, "notify failed", "key", in.DedupeKey, "err", err)
		}
	}
}

// NotifyAllActive pages through every active user.
func (s *Service) NotifyAllActive(ctx context.Context, in Input) error {
	after := ""
	for {
		ids, err := s.Q.ActiveUserIDs(ctx, store.ActiveUserIDsParams{After: after, Lim: userBatch})
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		s.Notify(ctx, ids, in)
		after = ids[len(ids)-1]
		if len(ids) < userBatch {
			return nil
		}
	}
}

func (s *Service) deliver(ctx context.Context, userIDs []string, in Input) error {
	data := in.Data
	if data == nil {
		data = map[string]string{}
	}
	raw, _ := json.Marshal(data)
	ids := make([]string, len(userIDs))
	for i := range ids {
		ids[i] = app.NewID()
	}
	inserted, err := s.Q.InsertNotifications(ctx, store.InsertNotificationsParams{Type: in.Type, Title: in.Title, Body: in.Body, Data: raw, DedupeKey: in.DedupeKey, Ids: ids, UserIds: userIDs})
	if err != nil || len(inserted) == 0 {
		return err
	}
	return s.push(ctx, inserted, PushMessage{Title: in.Title, Body: in.Body, ImageURL: in.ImageURL, Data: data})
}

func (s *Service) push(ctx context.Context, userIDs []string, m PushMessage) error {
	if s.Push == nil || (s.PushAllowed != nil && !s.PushAllowed(ctx)) {
		return nil
	}
	after := ""
	for {
		devices, err := s.Q.PushDevices(ctx, store.PushDevicesParams{UserIds: userIDs, After: after, Lim: tokenPage})
		if err != nil || len(devices) == 0 {
			return err
		}
		tokens := make([]string, len(devices))
		for i, d := range devices {
			tokens[i] = d.Token
		}
		if dead := s.Push.Send(ctx, tokens, m); len(dead) > 0 {
			if err := s.Q.DeleteDevicesByToken(ctx, dead); err != nil {
				return err
			}
		}
		after = devices[len(devices)-1].ID
		if len(devices) < tokenPage {
			return nil
		}
	}
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

// Routes: the caller's own inbox and devices (auth only).
func (s *Service) Routes(r fiber.Router) {
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
		ctx, id := c.UserContext(), app.CurrentUser(c).ID
		err := s.Tx(ctx, func(q *store.Queries) error {
			// A token belongs to one device: drop it from any other owner first.
			if err := q.DeleteDeviceTokenElsewhere(ctx, store.DeleteDeviceTokenElsewhereParams{Token: b.Token, UserID: id, DeviceID: b.DeviceID}); err != nil {
				return err
			}
			return q.UpsertDevice(ctx, store.UpsertDeviceParams{ID: app.NewID(), UserID: id, DeviceID: b.DeviceID, Token: b.Token, Device: raw})
		})
		if err != nil {
			return err
		}
		return httpx.NoContent(c)
	})

	r.Delete("/notifications/devices/:deviceId", func(c *fiber.Ctx) error {
		if err := s.Q.DeleteDevice(c.UserContext(), store.DeleteDeviceParams{UserID: app.CurrentUser(c).ID, DeviceID: c.Params("deviceId")}); err != nil {
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
