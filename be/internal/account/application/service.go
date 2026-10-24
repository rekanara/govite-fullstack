// Package application holds the self-service profile use cases. Security
// (password, 2FA, sessions) lives in auth.
package application

import (
	"context"
	"strings"

	"govite/internal/account/domain"
	"govite/internal/kernel"
)

type Service struct {
	Repo  domain.Repository
	Audit kernel.Auditor
}

func (s *Service) PushEnabled(ctx context.Context) (bool, error) {
	return s.Repo.PushEnabled(ctx, kernel.PrincipalFrom(ctx).ID)
}

func (s *Service) SetPushEnabled(ctx context.Context, on bool) error {
	id := kernel.PrincipalFrom(ctx).ID
	if err := s.Repo.SetPushEnabled(ctx, id, on); err != nil {
		return err
	}
	s.Audit.Record(ctx, "account.notifications.update", "user", id, nil, map[string]bool{"pushEnabled": on})
	return nil
}

func (s *Service) SetFullName(ctx context.Context, name string) (id, trimmed string, err error) {
	id, trimmed = kernel.PrincipalFrom(ctx).ID, strings.TrimSpace(name)
	if err := s.Repo.SetFullName(ctx, id, trimmed); err != nil {
		return id, trimmed, err
	}
	s.Audit.Record(ctx, "account.profile.update", "user", id, nil, map[string]string{"id": id, "fullName": trimmed})
	return id, trimmed, nil
}
