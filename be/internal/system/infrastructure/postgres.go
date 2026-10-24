// Package infrastructure runs the cleanup under a Postgres advisory lock.
package infrastructure

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"govite/internal/platform/postgres/store"
)

const cleanupLockID = 742101

type Cleaner struct{ Pool *pgxpool.Pool }

// Purge keeps cleanup to one instance at a time via pg_try_advisory_lock.
func (c Cleaner) Purge(ctx context.Context, cutoff time.Time) (map[string]int64, bool, error) {
	conn, err := c.Pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", cleanupLockID).Scan(&locked); err != nil || !locked {
		return nil, false, err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", cleanupLockID) //nolint:errcheck

	deleted := map[string]int64{}
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		q := store.New(tx)
		steps := []struct {
			name string
			fn   func(context.Context, time.Time) (int64, error)
		}{
			{"userTokens", q.PurgeUserTokens},
			{"refreshTokens", q.PurgeRefreshTokens},
			{"webhookEvents", q.PurgeWebhookEvents},
			{"notifications", q.PurgeNotifications},
		}
		for _, st := range steps {
			n, err := st.fn(ctx, cutoff)
			if err != nil {
				return err
			}
			deleted[st.name] = n
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return deleted, true, nil
}
