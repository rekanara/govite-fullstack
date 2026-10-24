// Package infrastructure stores audit entries in Postgres.
package infrastructure

import (
	"context"
	"encoding/json"

	"govite/internal/audit/domain"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func (r Repository) Insert(ctx context.Context, e domain.Entry) error {
	return r.DB.Q(ctx).InsertAudit(ctx, store.InsertAuditParams{
		ID: e.ID, ActorUserID: e.ActorUserID, Action: e.Action,
		EntityType: e.EntityType, EntityID: e.EntityID, Before: e.Before, After: e.After,
	})
}

func (r Repository) Count(ctx context.Context, f domain.Filter) (int64, error) {
	return r.DB.Q(ctx).CountAudit(ctx, store.CountAuditParams{
		ActorUserID: f.ActorUserID, EntityType: f.EntityType, EntityID: f.EntityID,
		Action: f.Action, UserID: f.UserID, FromAt: f.From, ToAt: f.To,
	})
}

func (r Repository) List(ctx context.Context, f domain.Filter, limit, offset int32) ([]domain.Entry, error) {
	rows, err := r.DB.Q(ctx).ListAudit(ctx, store.ListAuditParams{
		ActorUserID: f.ActorUserID, EntityType: f.EntityType, EntityID: f.EntityID,
		Action: f.Action, UserID: f.UserID, FromAt: f.From, ToAt: f.To, Lim: limit, Off: offset,
	})
	if err != nil {
		return nil, err
	}
	null := json.RawMessage("null")
	out := make([]domain.Entry, len(rows))
	for i, a := range rows {
		if a.Before == nil {
			a.Before = null
		}
		if a.After == nil {
			a.After = null
		}
		out[i] = domain.Entry{ID: a.ID, ActorUserID: a.ActorUserID, Action: a.Action, EntityType: a.EntityType, EntityID: a.EntityID, Before: a.Before, After: a.After, CreatedAt: a.CreatedAt}
	}
	return out, nil
}
