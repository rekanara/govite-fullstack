// Package infrastructure stores the settings row in Postgres.
package infrastructure

import (
	"context"

	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

const rowID = "app"

type Repository struct{ DB *postgres.DB }

func (r Repository) Get(ctx context.Context) ([]byte, error) {
	raw, err := r.DB.Q(ctx).GetSetting(ctx, rowID)
	return raw, postgres.Err(err)
}

func (r Repository) Put(ctx context.Context, raw []byte) error {
	return r.DB.Q(ctx).PutSetting(ctx, store.PutSettingParams{ID: rowID, Value: raw})
}
