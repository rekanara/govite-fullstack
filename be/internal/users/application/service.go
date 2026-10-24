// Package application holds the admin user use cases.
package application

import (
	"context"
	"strings"
	"time"

	"govite/internal/kernel"
	"govite/internal/users/domain"
)

type Service struct {
	Repo  domain.Repository
	Creds domain.Credentials
	Tx    kernel.Tx
	Audit kernel.Auditor
}

func actor(ctx context.Context) string { return kernel.PrincipalFrom(ctx).ID }

func (s *Service) Get(ctx context.Context, id string) (domain.User, error) {
	return s.Repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, f domain.Filter, limit, offset int32) ([]domain.User, int64, error) {
	total, err := s.Repo.Count(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Repo.List(ctx, f, limit, offset)
	return rows, total, err
}

type CreateInput struct {
	Email    string `json:"email" validate:"required,email"`
	FullName string `json:"fullName" validate:"required,min=1,max=100"`
	RoleID   string `json:"roleId" validate:"required,uuid"`
	Password string `json:"password" validate:"omitempty,password"`
}

// Create adds a user. Without a password the user is invited by email.
func (s *Service) Create(ctx context.Context, in CreateInput) (domain.User, error) {
	if ok, err := s.Repo.RoleExists(ctx, in.RoleID); err != nil || !ok {
		return domain.User{}, domain.ErrInvalidRole
	}
	var hash *string
	if in.Password != "" {
		h, err := s.Creds.HashPassword(in.Password)
		if err != nil {
			return domain.User{}, err
		}
		hash = &h
	}
	id := kernel.NewID()
	err := s.Repo.Create(ctx, domain.NewUser{ID: id, Email: strings.ToLower(strings.TrimSpace(in.Email)), FullName: strings.TrimSpace(in.FullName), RoleID: in.RoleID, PasswordHash: hash})
	if err != nil {
		return domain.User{}, err
	}
	u, err := s.Repo.Get(ctx, id)
	if err == nil && hash == nil {
		err = s.Creds.SendInvite(ctx, u.ID, u.Email)
	}
	return u, err
}

// Update changes fields; a role change or deactivation signs the user out
// in the same transaction.
func (s *Service) Update(ctx context.Context, id string, c domain.Changes) (domain.User, error) {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	revoke, err := cur.CheckUpdate(actor(ctx), c)
	if err != nil {
		return cur, err
	}
	if c.RoleID != nil {
		if ok, err := s.Repo.RoleExists(ctx, *c.RoleID); err != nil || !ok {
			return cur, domain.ErrInvalidRole
		}
	}
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.Update(ctx, id, c); err != nil {
			return err
		}
		if revoke {
			return s.Repo.RevokeSessions(ctx, id)
		}
		return nil
	})
	if err != nil {
		return cur, err
	}
	return s.Repo.Get(ctx, id)
}

// Remove soft-deletes and signs out atomically; deleting twice is a no-op.
func (s *Service) Remove(ctx context.Context, id string) (domain.User, error) {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if actor(ctx) == id {
		return cur, domain.ErrModifySelf
	}
	if cur.DeletedAt != nil {
		return cur, nil
	}
	now := time.Now()
	err = s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.SetDeleted(ctx, id, &now); err != nil {
			return err
		}
		return s.Repo.RevokeSessions(ctx, id)
	})
	if err != nil {
		return cur, err
	}
	return s.Repo.Get(ctx, id)
}

func (s *Service) Restore(ctx context.Context, id string) (domain.User, error) {
	cur, err := s.Repo.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if cur.DeletedAt == nil {
		return cur, domain.ErrNotDeleted
	}
	if err := s.Repo.SetDeleted(ctx, id, nil); err != nil {
		return cur, err
	}
	return s.Repo.Get(ctx, id)
}

func (s *Service) Reinvite(ctx context.Context, id string) error {
	u, err := s.Repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if u.DeletedAt != nil {
		return domain.ErrDeleted
	}
	if !u.Invited {
		return domain.ErrAlreadyActive
	}
	return s.Creds.SendInvite(ctx, u.ID, u.Email)
}

func (s *Service) ResetPassword(ctx context.Context, id, password string) error {
	if _, err := s.Repo.Get(ctx, id); err != nil {
		return err
	}
	hash, err := s.Creds.HashPassword(password)
	if err != nil {
		return err
	}
	return s.Tx.Do(ctx, func(ctx context.Context) error {
		if err := s.Repo.SetPassword(ctx, id, hash); err != nil {
			return err
		}
		return s.Repo.RevokeSessions(ctx, id)
	})
}

type BulkFailure struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Bulk applies action to each distinct id. Domain errors are collected per
// id; anything else aborts.
func (s *Service) Bulk(ctx context.Context, ids []string, action string) (succeeded []string, failed []BulkFailure, err error) {
	succeeded, failed = []string{}, []BulkFailure{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		var err error
		switch action {
		case "delete":
			_, err = s.Remove(ctx, id)
		case "restore":
			_, err = s.Restore(ctx, id)
		default:
			active := action == "activate"
			_, err = s.Update(ctx, id, domain.Changes{IsActive: &active})
		}
		var de *kernel.Error
		switch {
		case err == nil:
			succeeded = append(succeeded, id)
		case asKernel(err, &de):
			failed = append(failed, BulkFailure{id, de.Code, de.Message})
		default:
			return nil, nil, err
		}
	}
	return succeeded, failed, nil
}
