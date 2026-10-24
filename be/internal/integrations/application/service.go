// Package application holds the webhook and API key use cases.
package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"govite/internal/integrations/domain"
	"govite/internal/kernel"
)

type Service struct {
	Repo  domain.Repository
	Audit kernel.Auditor
	// Token returns n random bytes, base64url.
	Token func(n int) string
	// MidtransServerKey empty = payment gateway disabled.
	MidtransServerKey string
	// HandleMidtrans applies a verified notification to your payment domain.
	// ponytail: nil (no orders yet). Mark an order paid on settlement here.
	HandleMidtrans func(ctx context.Context, payload map[string]string) error
}

func (s *Service) PaymentEnabled() bool { return s.MidtransServerKey != "" }

// ReceiveMidtrans verifies, stores once, then processes. Duplicate deliveries
// answer duplicate=true; a failed earlier attempt is reclaimed and retried.
func (s *Service) ReceiveMidtrans(ctx context.Context, p map[string]string) (bool, error) {
	if !s.PaymentEnabled() {
		return false, domain.ErrProviderDisabled
	}
	if !domain.VerifyMidtrans(s.MidtransServerKey, p) {
		return false, domain.ErrBadSignature
	}
	key := domain.MidtransKey(p)
	raw, _ := json.Marshal(p)
	evType := p["transaction_status"]
	if evType == "" {
		evType = "unknown"
	}
	id, err := s.Repo.InsertEvent(ctx, "midtrans", key, evType, raw)
	if err != nil {
		return false, err
	}
	if id == "" {
		ev, err := s.Repo.EventByKey(ctx, "midtrans", key)
		if err != nil {
			return false, err
		}
		id = ev.ID
	}
	claimed, err := s.Repo.Claim(ctx, id)
	if err != nil || !claimed {
		return true, err
	}
	if s.HandleMidtrans != nil {
		err = s.HandleMidtrans(ctx, p)
	}
	if err == nil {
		err = s.Repo.MarkProcessed(ctx, id)
	}
	if err != nil {
		// Keep it "failed" for inspection/replay; the 5xx makes Midtrans retry.
		slog.ErrorContext(ctx, "midtrans webhook failed", "key", key, "err", err)
		msg := err.Error()
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		_ = s.Repo.MarkFailed(context.WithoutCancel(ctx), id, msg)
		return false, err
	}
	return false, nil
}

func (s *Service) Events(ctx context.Context, provider, status *string, limit, offset int32) ([]domain.Event, int64, error) {
	total, err := s.Repo.CountEvents(ctx, provider, status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Repo.ListEvents(ctx, provider, status, limit, offset)
	return rows, total, err
}

// Replay reprocesses a failed event.
func (s *Service) Replay(ctx context.Context, id string) (bool, error) {
	ev, err := s.Repo.Event(ctx, id)
	if err != nil {
		return false, domain.ErrEventNotFound
	}
	if ev.Provider != "midtrans" {
		return false, domain.ErrUnsupported
	}
	if ev.Status != "failed" {
		return false, domain.ErrNotFailed
	}
	var p map[string]string
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return false, err
	}
	dup, err := s.ReceiveMidtrans(ctx, p)
	if err == nil {
		s.Audit.Record(ctx, "webhook.replay", "webhook_event", id, nil, map[string]bool{"duplicate": dup})
	}
	return dup, err
}

func (s *Service) Keys(ctx context.Context) ([]domain.APIKey, error) { return s.Repo.ListKeys(ctx) }

// CreateKey returns the plaintext secret exactly once.
func (s *Service) CreateKey(ctx context.Context, name string) (domain.APIKey, string, error) {
	secret := "rk_" + s.Token(32)
	k, err := s.Repo.CreateKey(ctx, strings.TrimSpace(name), secret[:10], domain.HashAPIKey(secret))
	if err != nil {
		return k, "", err
	}
	s.Audit.Record(ctx, "api_key.create", "api_key", k.ID, nil, k)
	return k, secret, nil
}

func (s *Service) UpdateKey(ctx context.Context, id string, name *string, active *bool) (domain.APIKey, error) {
	k, err := s.Repo.UpdateKey(ctx, id, name, active)
	if err != nil {
		return k, domain.ErrKeyNotFound
	}
	s.Audit.Record(ctx, "api_key.update", "api_key", id, nil, k)
	return k, nil
}

func (s *Service) DeleteKey(ctx context.Context, id string) error {
	ok, err := s.Repo.DeleteKey(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrKeyNotFound
	}
	s.Audit.Record(ctx, "api_key.delete", "api_key", id, nil, nil)
	return nil
}
