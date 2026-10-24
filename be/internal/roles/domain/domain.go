// Package domain holds roles, their grants and the rules that protect them.
package domain

import (
	"context"
	"slices"

	"govite/internal/kernel"
	"govite/internal/policy"
	rbac "govite/internal/rbac/domain"
)

var (
	ErrNotFound  = kernel.E(kernel.NotFound, "ROLE_NOT_FOUND", "Role not found")
	ErrNameTaken = kernel.E(kernel.Conflict, "ROLE_NAME_TAKEN", "Role name already exists")
	ErrSystem    = kernel.E(kernel.Forbidden, "SYSTEM_ROLE_IMMUTABLE", "System roles cannot be renamed, deleted or have permissions changed")
	ErrInUse     = kernel.E(kernel.Conflict, "ROLE_IN_USE", "Role still has users assigned")
)

func ErrLastHolder(perm string) error {
	return kernel.E(kernel.Conflict, "LAST_PERMISSION_HOLDER", "No role would be left holding "+perm)
}

type Role struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      *string      `json:"description"`
	IsSystem         bool         `json:"isSystem"`
	RequireTwoFactor bool         `json:"requireTwoFactor"`
	UserCount        int32        `json:"userCount"`
	Permissions      []rbac.Grant `json:"permissions"`
}

type Changes struct {
	Name             *string
	Description      *string
	RequireTwoFactor *bool
}

// CanRename / CanDelete / CanSetPermissions are the system-role invariants.
func (r Role) CanRename(name *string) error {
	if r.IsSystem && name != nil && *name != r.Name {
		return ErrSystem
	}
	return nil
}

func (r Role) CanDelete() error {
	if r.IsSystem {
		return ErrSystem
	}
	if r.UserCount > 0 {
		return ErrInUse
	}
	return nil
}

func (r Role) CanSetPermissions() error {
	if r.IsSystem {
		return ErrSystem
	}
	return nil
}

// ValidGrants rejects resource/action/scope triples outside the policy.
func ValidGrants(gs []rbac.Grant) error {
	for _, g := range gs {
		if !policy.IsKnown(g.Resource, g.Action) || !slices.Contains(policy.Scopes, g.Scope) {
			return kernel.Validation("permissions: unknown resource/action/scope")
		}
	}
	return nil
}

// Dropped lists guarded permissions r holds that next would remove.
func (r Role) Dropped(next []rbac.Grant) []rbac.Grant {
	keep := map[string]bool{}
	for _, g := range next {
		keep[g.Resource+"."+g.Action] = true
	}
	var out []rbac.Grant
	for _, g := range r.Permissions {
		key := g.Resource + "." + g.Action
		if slices.Contains(policy.Guarded, key) && !keep[key] {
			out = append(out, g)
		}
	}
	return out
}

type Repository interface {
	Get(ctx context.Context, id string) (Role, error)
	List(ctx context.Context, search *string, limit, offset int32) ([]Role, int64, error)
	Create(ctx context.Context, id, name string, description *string, requireTwoFactor bool) error
	Update(ctx context.Context, id string, c Changes) error
	Delete(ctx context.Context, id string) error
	// OtherHolders counts roles other than roleID holding resource.action.
	OtherHolders(ctx context.Context, roleID, resource, action string) (int64, error)
	SetPermissions(ctx context.Context, roleID string, gs []rbac.Grant) error
}

// Cache drops cached grants after a change.
type Cache interface {
	Invalidate(ctx context.Context, roleID string)
}
