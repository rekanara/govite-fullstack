package system

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
)

const cleanupLockID = 742101

type CleanupResult struct {
	Ran     bool             `json:"ran"`
	Cutoff  *time.Time       `json:"cutoff"`
	Deleted map[string]int64 `json:"deleted"`
}

// Cleanup deletes expired/used tokens, old processed webhooks and old read
// notifications. A Postgres advisory lock keeps it to one instance at a time.
func (s *Service) Cleanup(ctx context.Context) (CleanupResult, error) {
	res := CleanupResult{Deleted: map[string]int64{}}
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", cleanupLockID).Scan(&locked); err != nil || !locked {
		return res, err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", cleanupLockID) //nolint:errcheck

	days := 90
	if s.RetentionDays != nil {
		days = s.RetentionDays(ctx)
	}
	cutoff := time.Now().AddDate(0, 0, -days)
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
			res.Deleted[st.name] = n
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	res.Ran, res.Cutoff = true, &cutoff
	return res, nil
}

// RunDaily runs Cleanup once a day at 03:17 local time until ctx ends.
func (s *Service) RunDaily(ctx context.Context) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 17, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			if r, err := s.Cleanup(ctx); err != nil {
				slog.ErrorContext(ctx, "maintenance cleanup failed", "err", err)
			} else {
				slog.InfoContext(ctx, "maintenance cleanup", "ran", r.Ran, "deleted", r.Deleted)
			}
		}
	}
}

func (s *Service) maintenanceRoutes(g rbac.Guarded) {
	g.Post("/system/maintenance/cleanup", "settings.manage", func(c *fiber.Ctx) error {
		r, err := s.Cleanup(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.Created(c, r)
	})
}
