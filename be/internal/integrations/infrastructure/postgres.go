// Package infrastructure stores webhook events and API keys in Postgres.
package infrastructure

import (
	"context"
	"encoding/json"

	"govite/internal/integrations/domain"
	"govite/internal/kernel"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func toEvent(e store.WebhookEvent) domain.Event {
	return domain.Event{ID: e.ID, Provider: e.Provider, EventKey: e.EventKey, EventType: e.EventType, Status: e.Status, Error: e.Error, Payload: e.Payload, ReceivedAt: e.ReceivedAt, ProcessedAt: e.ProcessedAt}
}

func toKey(k store.ApiKey) domain.APIKey {
	return domain.APIKey{ID: k.ID, Name: k.Name, Prefix: k.Prefix, IsActive: k.IsActive, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt}
}

func (r Repository) InsertEvent(ctx context.Context, provider, key, eventType string, payload json.RawMessage) (string, error) {
	id, err := r.DB.Q(ctx).InsertWebhookEvent(ctx, store.InsertWebhookEventParams{ID: kernel.NewID(), Provider: provider, EventKey: key, EventType: eventType, Payload: payload})
	if postgres.Err(err) == kernel.ErrNotFound { // ON CONFLICT DO NOTHING
		return "", nil
	}
	return id, err
}

func (r Repository) EventByKey(ctx context.Context, provider, key string) (domain.Event, error) {
	e, err := r.DB.Q(ctx).GetWebhookEventByKey(ctx, store.GetWebhookEventByKeyParams{Provider: provider, EventKey: key})
	return toEvent(e), postgres.Err(err)
}

func (r Repository) Event(ctx context.Context, id string) (domain.Event, error) {
	e, err := r.DB.Q(ctx).GetWebhookEvent(ctx, id)
	return toEvent(e), postgres.Err(err)
}

func (r Repository) Claim(ctx context.Context, id string) (bool, error) {
	n, err := r.DB.Q(ctx).ClaimWebhook(ctx, id)
	return n > 0, err
}

func (r Repository) MarkProcessed(ctx context.Context, id string) error {
	return r.DB.Q(ctx).MarkWebhookProcessed(ctx, id)
}

func (r Repository) MarkFailed(ctx context.Context, id, msg string) error {
	return r.DB.Q(ctx).MarkWebhookFailed(ctx, store.MarkWebhookFailedParams{ID: id, Error: &msg})
}

func (r Repository) CountEvents(ctx context.Context, provider, status *string) (int64, error) {
	return r.DB.Q(ctx).CountWebhookEvents(ctx, store.CountWebhookEventsParams{Provider: provider, Status: status})
}

func (r Repository) ListEvents(ctx context.Context, provider, status *string, limit, offset int32) ([]domain.Event, error) {
	rows, err := r.DB.Q(ctx).ListWebhookEvents(ctx, store.ListWebhookEventsParams{Provider: provider, Status: status, Lim: limit, Off: offset})
	out := make([]domain.Event, len(rows))
	for i, e := range rows {
		out[i] = toEvent(e)
	}
	return out, err
}

func (r Repository) ListKeys(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := r.DB.Q(ctx).ListAPIKeys(ctx)
	out := make([]domain.APIKey, len(rows))
	for i, k := range rows {
		out[i] = toKey(k)
	}
	return out, err
}

func (r Repository) CreateKey(ctx context.Context, name, prefix, hash string) (domain.APIKey, error) {
	k, err := r.DB.Q(ctx).CreateAPIKey(ctx, store.CreateAPIKeyParams{ID: kernel.NewID(), Name: name, Prefix: prefix, KeyHash: hash})
	return toKey(k), err
}

func (r Repository) UpdateKey(ctx context.Context, id string, name *string, active *bool) (domain.APIKey, error) {
	k, err := r.DB.Q(ctx).UpdateAPIKey(ctx, store.UpdateAPIKeyParams{ID: id, Name: name, IsActive: active})
	return toKey(k), postgres.Err(err)
}

func (r Repository) DeleteKey(ctx context.Context, id string) (bool, error) {
	n, err := r.DB.Q(ctx).DeleteAPIKey(ctx, id)
	return n > 0, err
}
