// Package infrastructure stores the caller's profile in Postgres.
package infrastructure

import (
	"context"

	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
)

type Repository struct{ DB *postgres.DB }

func (r Repository) PushEnabled(ctx context.Context, userID string) (bool, error) {
	u, err := r.DB.Q(ctx).GetUserByID(ctx, userID)
	return u.NotificationPushEnabled, postgres.Err(err)
}

func (r Repository) SetPushEnabled(ctx context.Context, userID string, on bool) error {
	return r.DB.Q(ctx).SetPushEnabled(ctx, store.SetPushEnabledParams{ID: userID, NotificationPushEnabled: on})
}

func (r Repository) SetFullName(ctx context.Context, userID, name string) error {
	return r.DB.Q(ctx).UpdateProfile(ctx, store.UpdateProfileParams{ID: userID, FullName: &name})
}
