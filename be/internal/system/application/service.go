// Package application holds health checks and the daily cleanup.
package application

import (
	"context"
	"log/slog"
	"time"

	"govite/internal/system/domain"
)

var Started = time.Now()

type Service struct {
	DB, Redis     domain.Pinger // Redis nil = not configured
	Cleaner       domain.Cleaner
	RetentionDays func(context.Context) int
}

func probe(ctx context.Context, fn domain.Pinger) domain.Check {
	if fn == nil {
		return domain.Check{Status: "down"}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	t := time.Now()
	if err := fn(ctx); err != nil {
		return domain.Check{Status: "down"}
	}
	ms := time.Since(t).Milliseconds()
	return domain.Check{Status: "up", LatencyMs: &ms}
}

// Checks probes the database and Redis. ready is false when the DB is down,
// or Redis is configured and down.
func (s *Service) Checks(ctx context.Context) (db, redis domain.Check, ready bool) {
	db, redis = probe(ctx, s.DB), probe(ctx, s.Redis)
	return db, redis, db.Status == "up" && (s.Redis == nil || redis.Status == "up")
}

func (s *Service) Cleanup(ctx context.Context) (domain.CleanupResult, error) {
	days := 90
	if s.RetentionDays != nil {
		days = s.RetentionDays(ctx)
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	deleted, ran, err := s.Cleaner.Purge(ctx, cutoff)
	res := domain.CleanupResult{Ran: ran, Deleted: deleted}
	if res.Deleted == nil {
		res.Deleted = map[string]int64{}
	}
	if ran {
		res.Cutoff = &cutoff
	}
	return res, err
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
