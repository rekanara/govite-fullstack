// Package application writes notifications, pushes them and serves the inbox.
package application

import (
	"context"
	"encoding/json"
	"log/slog"

	"govite/internal/kernel"
	"govite/internal/notify/domain"
)

const (
	userBatch = 500
	tokenPage = 1000
)

type Service struct {
	Repo domain.Repository
	Tx   kernel.Tx
	Push domain.Pusher // nil = push disabled
	// PushAllowed is the admin kill switch (app settings).
	PushAllowed func(context.Context) bool
}

// Notify writes the in-app row for each user (deduplicated) and pushes to
// those who got a new row. Failures are logged; they never fail the caller.
func (s *Service) Notify(ctx context.Context, userIDs []string, in domain.Input) {
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
func (s *Service) NotifyAllActive(ctx context.Context, in domain.Input) error {
	after := ""
	for {
		ids, err := s.Repo.ActiveUserIDs(ctx, after, userBatch)
		if err != nil || len(ids) == 0 {
			return err
		}
		s.Notify(ctx, ids, in)
		after = ids[len(ids)-1]
		if len(ids) < userBatch {
			return nil
		}
	}
}

func (s *Service) deliver(ctx context.Context, userIDs []string, in domain.Input) error {
	data := in.Data
	if data == nil {
		data = map[string]string{}
	}
	raw, _ := json.Marshal(data)
	ids := make([]string, len(userIDs))
	for i := range ids {
		ids[i] = kernel.NewID()
	}
	inserted, err := s.Repo.Insert(ctx, in, raw, ids, userIDs)
	if err != nil || len(inserted) == 0 {
		return err
	}
	return s.push(ctx, inserted, domain.PushMessage{Title: in.Title, Body: in.Body, ImageURL: in.ImageURL, Data: data})
}

func (s *Service) push(ctx context.Context, userIDs []string, m domain.PushMessage) error {
	if s.Push == nil || (s.PushAllowed != nil && !s.PushAllowed(ctx)) {
		return nil
	}
	after := ""
	for {
		devices, err := s.Repo.Devices(ctx, userIDs, after, tokenPage)
		if err != nil || len(devices) == 0 {
			return err
		}
		tokens := make([]string, len(devices))
		for i, d := range devices {
			tokens[i] = d.Token
		}
		if dead := s.Push.Send(ctx, tokens, m); len(dead) > 0 {
			if err := s.Repo.DeleteDeviceTokens(ctx, dead); err != nil {
				return err
			}
		}
		after = devices[len(devices)-1].ID
		if len(devices) < tokenPage {
			return nil
		}
	}
}

func me(ctx context.Context) string { return kernel.PrincipalFrom(ctx).ID }

func (s *Service) RegisterDevice(ctx context.Context, deviceID, token string, device json.RawMessage) error {
	return s.Tx.Do(ctx, func(ctx context.Context) error {
		return s.Repo.RegisterDevice(ctx, me(ctx), deviceID, token, device)
	})
}

func (s *Service) DeleteDevice(ctx context.Context, deviceID string) error {
	return s.Repo.DeleteDevice(ctx, me(ctx), deviceID)
}

// Inbox returns a page, the total for the filter, and the unread count.
func (s *Service) Inbox(ctx context.Context, unreadOnly bool, limit, offset int32) ([]domain.Notification, int64, int64, error) {
	total, unread, err := s.Repo.Counts(ctx, me(ctx))
	if err != nil {
		return nil, 0, 0, err
	}
	rows, err := s.Repo.List(ctx, me(ctx), unreadOnly, limit, offset)
	if unreadOnly {
		total = unread
	}
	return rows, total, unread, err
}

func (s *Service) MarkAllRead(ctx context.Context) error { return s.Repo.MarkAllRead(ctx, me(ctx)) }

func (s *Service) MarkRead(ctx context.Context, id string) (domain.Notification, error) {
	n, err := s.Repo.MarkRead(ctx, id, me(ctx))
	if err != nil {
		return n, domain.ErrNotFound
	}
	return n, nil
}
