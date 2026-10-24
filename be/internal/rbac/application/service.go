package application

import (
	"context"

	"govite/internal/rbac/domain"
)

type Service struct {
	Repository domain.GrantRepository
	Cache      domain.GrantCache
	Team       domain.TeamHierarchy
}

func (s *Service) Grants(ctx context.Context, roleID string) ([]domain.Grant, error) {
	if s.Cache != nil {
		if grants, ok := s.Cache.Get(ctx, roleID); ok {
			return grants, nil
		}
	}
	grants, err := s.Repository.Grants(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if s.Cache != nil {
		s.Cache.Set(ctx, roleID, grants)
	}
	return grants, nil
}

func (s *Service) Invalidate(ctx context.Context, roleID string) {
	if s.Cache != nil {
		s.Cache.Delete(ctx, roleID)
	}
}

func (s *Service) Resolve(ctx context.Context, principal domain.Principal, resource, action string) (domain.Scope, error) {
	grants, err := s.Grants(ctx, principal.RoleID)
	if err != nil {
		return domain.Scope{}, err
	}
	for _, grant := range grants {
		if grant.Resource != resource || grant.Action != action {
			continue
		}
		switch grant.Scope {
		case "all":
			return domain.Scope{Kind: "all"}, nil
		case "team":
			if s.Team == nil {
				return domain.Scope{}, domain.ErrTeamScope
			}
			ids, err := s.Team(ctx, principal.ID)
			return domain.Scope{Kind: "team", UserID: principal.ID, SubordinateUserIDs: ids}, err
		default:
			return domain.Scope{Kind: "self", UserID: principal.ID}, nil
		}
	}
	return domain.Scope{}, domain.ErrNotGranted(resource + "." + action)
}
