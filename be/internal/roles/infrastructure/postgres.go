// Package infrastructure stores roles in Postgres.
package infrastructure

import (
	"context"

	"govite/internal/kernel"
	"govite/internal/platform/postgres"
	"govite/internal/platform/postgres/store"
	rbac "govite/internal/rbac/domain"
	"govite/internal/roles/domain"
)

type Repository struct{ DB *postgres.DB }

func nameErr(err error) error {
	if postgres.IsUnique(err, "name") {
		return domain.ErrNameTaken
	}
	return err
}

func (r Repository) withGrants(ctx context.Context, roles []store.Role, counts []int32) ([]domain.Role, error) {
	ids := make([]string, len(roles))
	for i, x := range roles {
		ids[i] = x.ID
	}
	rows, err := r.DB.Q(ctx).RolePermissions(ctx, ids)
	if err != nil {
		return nil, err
	}
	by := map[string][]rbac.Grant{}
	for _, p := range rows {
		by[p.RoleID] = append(by[p.RoleID], rbac.Grant{Resource: p.Resource, Action: p.Action, Scope: p.Scope})
	}
	out := make([]domain.Role, len(roles))
	for i, x := range roles {
		perms := by[x.ID]
		if perms == nil {
			perms = []rbac.Grant{}
		}
		out[i] = domain.Role{ID: x.ID, Name: x.Name, Description: x.Description, IsSystem: x.IsSystem, RequireTwoFactor: x.RequireTwoFactor, UserCount: counts[i], Permissions: perms}
	}
	return out, nil
}

func (r Repository) Get(ctx context.Context, id string) (domain.Role, error) {
	row, err := r.DB.Q(ctx).GetRole(ctx, id)
	if err != nil {
		if postgres.Err(err) == kernel.ErrNotFound {
			return domain.Role{}, domain.ErrNotFound
		}
		return domain.Role{}, err
	}
	v, err := r.withGrants(ctx, []store.Role{row.Role}, []int32{row.UserCount})
	if err != nil {
		return domain.Role{}, err
	}
	return v[0], nil
}

func (r Repository) List(ctx context.Context, search *string, limit, offset int32) ([]domain.Role, int64, error) {
	q := r.DB.Q(ctx)
	total, err := q.CountRoles(ctx, search)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListRoles(ctx, store.ListRolesParams{Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	roles, counts := make([]store.Role, len(rows)), make([]int32, len(rows))
	for i, x := range rows {
		roles[i], counts[i] = x.Role, x.UserCount
	}
	out, err := r.withGrants(ctx, roles, counts)
	return out, total, err
}

func (r Repository) Create(ctx context.Context, id, name string, description *string, requireTwoFactor bool) error {
	return nameErr(r.DB.Q(ctx).CreateRole(ctx, store.CreateRoleParams{ID: id, Name: name, Description: description, RequireTwoFactor: requireTwoFactor}))
}

func (r Repository) Update(ctx context.Context, id string, c domain.Changes) error {
	return nameErr(r.DB.Q(ctx).UpdateRole(ctx, store.UpdateRoleParams{ID: id, Name: c.Name, Description: c.Description, RequireTwoFactor: c.RequireTwoFactor}))
}

func (r Repository) Delete(ctx context.Context, id string) error {
	return r.DB.Q(ctx).DeleteRole(ctx, id)
}

func (r Repository) OtherHolders(ctx context.Context, roleID, resource, action string) (int64, error) {
	return r.DB.Q(ctx).CountPermissionHolders(ctx, store.CountPermissionHoldersParams{Resource: resource, Action: action, RoleID: roleID})
}

func (r Repository) SetPermissions(ctx context.Context, roleID string, gs []rbac.Grant) error {
	q := r.DB.Q(ctx)
	if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
		return err
	}
	for _, g := range gs {
		if err := q.InsertRolePermission(ctx, store.InsertRolePermissionParams{ID: kernel.NewID(), RoleID: roleID, Resource: g.Resource, Action: g.Action, Scope: g.Scope}); err != nil {
			return err
		}
	}
	return nil
}
