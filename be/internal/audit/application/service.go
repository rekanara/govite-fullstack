// Package application records and queries the audit log.
package application

import (
	"context"
	"log/slog"

	"govite/internal/audit/domain"
	"govite/internal/kernel"
)

// ponytail: exports are built in memory; stream rows when this cap is hit.
const ExportLimit = 10_000

type Service struct{ Repo domain.Repository }

// Record implements kernel.Auditor with the ctx caller as actor. A failed
// write is logged, never returned: an audit hiccup must not fail the action.
func (s *Service) Record(ctx context.Context, action, entityType, entityID string, before, after any) {
	e := domain.Entry{
		ID: kernel.NewID(), ActorUserID: kernel.Ptr(kernel.PrincipalFrom(ctx).ID), Action: action,
		EntityType: entityType, EntityID: entityID, Before: domain.Redact(before), After: domain.Redact(after),
	}
	if err := s.Repo.Insert(ctx, e); err != nil {
		slog.ErrorContext(ctx, "audit write failed", "err", err, "action", action)
	}
}

func (s *Service) List(ctx context.Context, f domain.Filter, limit, offset int32) ([]domain.Entry, int64, error) {
	total, err := s.Repo.Count(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Repo.List(ctx, f, limit, offset)
	return rows, total, err
}

func (s *Service) Export(ctx context.Context, f domain.Filter) ([]domain.Entry, error) {
	return s.Repo.List(ctx, f, ExportLimit, 0)
}
