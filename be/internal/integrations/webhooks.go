// Package integrations: Midtrans webhooks (verified, idempotent) and API keys.
package integrations

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
)

var (
	ErrProviderDisabled = httpx.E(503, "WEBHOOK_PROVIDER_DISABLED", "Midtrans is not configured")
	ErrBadSignature     = httpx.E(401, "INVALID_WEBHOOK_SIGNATURE", "Invalid webhook signature")
	ErrEventNotFound    = httpx.E(404, "WEBHOOK_EVENT_NOT_FOUND", "Webhook event not found")
	ErrUnsupported      = httpx.E(400, "WEBHOOK_PROVIDER_UNSUPPORTED", "Webhook provider is not supported")
	ErrNotFailed        = httpx.E(400, "WEBHOOK_REPLAY_NOT_FAILED", "Only failed webhook events can be replayed")
)

type Service struct {
	*app.Deps
	Audit *audit.Log
	// MidtransServerKey empty = payment gateway disabled.
	MidtransServerKey string
	// HandleMidtrans applies a verified notification to your payment domain.
	// ponytail: nil (no orders yet). Mark an order paid on settlement here.
	HandleMidtrans func(ctx context.Context, payload map[string]string) error
}

func (s *Service) PaymentEnabled() bool { return s.MidtransServerKey != "" }

// VerifyMidtrans checks sha512(order_id+status_code+gross_amount+serverKey).
func (s *Service) VerifyMidtrans(p map[string]string) bool {
	if !s.PaymentEnabled() || p["signature_key"] == "" {
		return false
	}
	sum := sha512.Sum512([]byte(p["order_id"] + p["status_code"] + p["gross_amount"] + s.MidtransServerKey))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(p["signature_key"]))) == 1
}

func midtransKey(p map[string]string) string {
	return strings.Join([]string{p["order_id"], p["transaction_id"], p["transaction_status"], p["status_code"], p["gross_amount"]}, ":")
}

// ReceiveMidtrans verifies, stores once, then processes. Duplicate deliveries
// answer {duplicate:true}; a failed earlier attempt is reclaimed and retried.
func (s *Service) ReceiveMidtrans(ctx context.Context, p map[string]string) (bool, error) {
	if !s.PaymentEnabled() {
		return false, ErrProviderDisabled
	}
	if !s.VerifyMidtrans(p) {
		return false, ErrBadSignature
	}
	key := midtransKey(p)
	raw, _ := json.Marshal(p)
	evType := p["transaction_status"]
	if evType == "" {
		evType = "unknown"
	}
	id, err := s.Q.InsertWebhookEvent(ctx, store.InsertWebhookEventParams{ID: app.NewID(), Provider: "midtrans", EventKey: key, EventType: evType, Payload: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		ev, getErr := s.Q.GetWebhookEventByKey(ctx, store.GetWebhookEventByKeyParams{Provider: "midtrans", EventKey: key})
		if getErr != nil {
			return false, getErr
		}
		id = ev.ID
	} else if err != nil {
		return false, err
	}
	n, err := s.Q.ClaimWebhook(ctx, id)
	if err != nil || n == 0 {
		return true, err
	}
	if s.HandleMidtrans != nil {
		err = s.HandleMidtrans(ctx, p)
	}
	if err == nil {
		err = s.Q.MarkWebhookProcessed(ctx, id)
	}
	if err != nil {
		// Keep it "failed" for inspection/replay; the 5xx makes Midtrans retry.
		slog.ErrorContext(ctx, "midtrans webhook failed", "key", key, "err", err)
		msg := err.Error()
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		_ = s.Q.MarkWebhookFailed(context.WithoutCancel(ctx), store.MarkWebhookFailedParams{ID: id, Error: &msg})
		return false, err
	}
	return false, nil
}

var signature = regexp.MustCompile(`(?i)^[a-f0-9]{128}$`)

// PublicRoutes: the provider callback. Throttled separately (providers retry in bursts).
func (s *Service) PublicRoutes(r fiber.Router, limit fiber.Handler) {
	r.Post("/webhooks/midtrans", limit, func(c *fiber.Ctx) error {
		var raw map[string]any
		if err := json.Unmarshal(c.Body(), &raw); err != nil {
			return httpx.Validation("body: invalid JSON")
		}
		p := make(map[string]string, len(raw))
		for k, v := range raw {
			str, ok := v.(string)
			if !ok {
				return httpx.Validation(k + ": expected string")
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
			return httpx.Validation(strings.Join(problems, "; "))
		}
		dup, err := s.ReceiveMidtrans(c.UserContext(), p)
		if err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"duplicate": dup})
	})
}

type event struct {
	ID          string          `json:"id"`
	Provider    string          `json:"provider"`
	EventKey    string          `json:"eventKey"`
	EventType   string          `json:"eventType"`
	Status      string          `json:"status"`
	Error       *string         `json:"error"`
	Payload     json.RawMessage `json:"payload"`
	ReceivedAt  time.Time       `json:"receivedAt"`
	ProcessedAt *time.Time      `json:"processedAt"`
}

func (s *Service) Routes(g rbac.Guarded) {
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
		ctx := c.UserContext()
		prov, st := httpx.Ptr(strings.TrimSpace(q.Provider)), httpx.Ptr(q.Status)
		total, err := s.Q.CountWebhookEvents(ctx, store.CountWebhookEventsParams{Provider: prov, Status: st})
		if err != nil {
			return err
		}
		rows, err := s.Q.ListWebhookEvents(ctx, store.ListWebhookEventsParams{Provider: prov, Status: st, Lim: int32(q.Limit), Off: q.Offset()})
		if err != nil {
			return err
		}
		out := make([]event, len(rows))
		for i, e := range rows {
			out[i] = event{e.ID, e.Provider, e.EventKey, e.EventType, e.Status, e.Error, e.Payload, e.ReceivedAt, e.ProcessedAt}
		}
		return httpx.Page(c, out, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Post("/webhooks/events/:id/replay", "webhooks.manage", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Validation("id: must be a UUID")
		}
		ctx := c.UserContext()
		ev, err := s.Q.GetWebhookEvent(ctx, id)
		if err != nil {
			return ErrEventNotFound
		}
		if ev.Provider != "midtrans" {
			return ErrUnsupported
		}
		if ev.Status != "failed" {
			return ErrNotFailed
		}
		var p map[string]string
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		dup, err := s.ReceiveMidtrans(ctx, p)
		if err != nil {
			return err
		}
		res := fiber.Map{"duplicate": dup}
		s.Audit.From(c, "webhook.replay", "webhook_event", id, nil, res)
		return httpx.OK(c, res)
	})
}
