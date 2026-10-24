// Package application holds the role use cases.
package application

import (
	"context"

	"govite/internal/kernel"
	rbac "govite/internal/rbac/domain"
	"govite/internal/roles/domain"
)

type Service struct {
	Repo  domain.Repository
	Cache domain.Cache
	Tx    kernel.Tx
	Audit kernel.Auditor
}

func (s *Service) Get(ctx context.Context, id string) (domain.Role, error) {
	return s.Repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, search *string, limit, offset int32) ([]domain.Role, int64, error) {
	return s.Repo.List(ctx, search, limit, offset)
}

func (s *Service) Create(ctx context.Context, name string, description *string, requireTwoFactor bool, input any) (domain.Role, error) {
	id := kernel.NewID()
	if err := s.Repo.Create(ctx, id, name, description, requireTwoFactor); err != nil {
		return domain.Role{}, err
	}
	r, err := s.Repo.Get(ctx, id)
	if err == nil {
		s.Audit.Record(ctx, "role.create", "role", id, input, r)
	}
	return r, err
}

func (s *Service) Update(ctx context.Context, id string, c domain.Changes, input any) (domain.Role, error) {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if err := cur.CanRename(c.Name); err != nil {
		return cur, err
	}
	if err := s.Repo.Update(ctx, id, c); err != nil {
		return cur, err
	}
	r, err := s.Repo.Get(ctx, id)
	if err == nil {
		s.Audit.Record(ctx, "role.update", "role", id, input, r)
	}
	return r, err
}

func (s *Service) Delete(ctx context.Context, id string, input any) error {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := cur.CanDelete(); err != nil {
		return err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	s.Cache.Invalidate(ctx, id)
	s.Audit.Record(ctx, "role.delete", "role", id, input, map[string]string{"id": id})
	return nil
}

// SetPermissions replaces a role's grants, refusing to strip the last
// holder of a guarded permission (so nobody can lock admins out).
func (s *Service) SetPermissions(ctx context.Context, id string, gs []rbac.Grant, input any) (domain.Role, error) {
	if err := domain.ValidGrants(gs); err != nil {
		return domain.Role{}, err
	}
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if err := cur.CanSetPermissions(); err != nil {
		return cur, err
	}
	for _, g := range cur.Dropped(gs) {
		n, err := s.Repo.OtherHolders(ctx, id, g.Resource, g.Action)
		if err != nil {
			return cur, err
		}
		if n == 0 {
			return cur, domain.ErrLastHolder(g.Resource + "." + g.Action)
		}
	}
	if err := s.Tx.Do(ctx, func(ctx context.Context) error { return s.Repo.SetPermissions(ctx, id, gs) }); err != nil {
		return cur, err
	}
	s.Cache.Invalidate(ctx, id)
	r, err := s.Repo.Get(ctx, id)
	if err == nil {
		s.Audit.Record(ctx, "role.permissions.set", "role", id, input, r)
	}
	return r, err
}
