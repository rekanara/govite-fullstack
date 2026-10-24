// Package roles manages roles and their permission grants.
package roles

import (
	"context"
	"slices"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/httpx"
	"govite/internal/policy"
	"govite/internal/rbac"
	"govite/internal/store"
)

var (
	ErrNotFound  = httpx.E(404, "ROLE_NOT_FOUND", "Role not found")
	ErrNameTaken = httpx.E(409, "ROLE_NAME_TAKEN", "Role name already exists")
	ErrSystem    = httpx.E(403, "SYSTEM_ROLE_IMMUTABLE", "System roles cannot be renamed, deleted or have permissions changed")
	ErrInUse     = httpx.E(409, "ROLE_IN_USE", "Role still has users assigned")
)

func errLastHolder(perm string) *httpx.Error {
	return httpx.E(409, "LAST_PERMISSION_HOLDER", "No role would be left holding "+perm)
}

type Service struct {
	*app.Deps
	RBAC  *rbac.RBAC
	Audit *audit.Log
}

type View struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      *string      `json:"description"`
	IsSystem         bool         `json:"isSystem"`
	RequireTwoFactor bool         `json:"requireTwoFactor"`
	UserCount        int32        `json:"userCount"`
	Permissions      []rbac.Grant `json:"permissions"`
}

func (s *Service) views(ctx context.Context, roles []store.Role, counts []int32) ([]View, error) {
	ids := make([]string, len(roles))
	for i, r := range roles {
		ids[i] = r.ID
	}
	rows, err := s.Q.RolePermissions(ctx, ids)
	if err != nil {
		return nil, err
	}
	by := map[string][]rbac.Grant{}
	for _, p := range rows {
		by[p.RoleID] = append(by[p.RoleID], rbac.Grant{Resource: p.Resource, Action: p.Action, Scope: p.Scope})
	}
	out := make([]View, len(roles))
	for i, r := range roles {
		perms := by[r.ID]
		if perms == nil {
			perms = []rbac.Grant{}
		}
		out[i] = View{r.ID, r.Name, r.Description, r.IsSystem, r.RequireTwoFactor, counts[i], perms}
	}
	return out, nil
}

func (s *Service) get(ctx context.Context, id string) (View, error) {
	row, err := s.Q.GetRole(ctx, id)
	if err != nil {
		return View{}, ErrNotFound
	}
	v, err := s.views(ctx, []store.Role{row.Role}, []int32{row.UserCount})
	if err != nil {
		return View{}, err
	}
	return v[0], nil
}

type createReq struct {
	Name             string  `json:"name" validate:"required,min=1,max=50"`
	Description      *string `json:"description" validate:"omitempty,max=200"`
	RequireTwoFactor bool    `json:"requireTwoFactor"`
}

type updateReq struct {
	Name             *string `json:"name" validate:"omitempty,min=1,max=50"`
	Description      *string `json:"description" validate:"omitempty,max=200"`
	RequireTwoFactor *bool   `json:"requireTwoFactor"`
}

type permsReq struct {
	Permissions []rbac.Grant `json:"permissions" validate:"max=200,dive"`
}

func validID(c *fiber.Ctx) (string, error) {
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", httpx.Validation("id: must be a UUID")
	}
	return id, nil
}

func (s *Service) Routes(g rbac.Guarded) {
	g.Get("/permissions", "permissions.view", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{
			"permissions": policy.Permissions, "resources": policy.Resources,
			"actions": policy.Actions, "scopes": policy.Scopes,
		})
	})

	g.Get("/roles", "roles.manage", func(c *fiber.Ctx) error {
		var q httpx.Pagination
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.Normalize()
		ctx := c.UserContext()
		total, err := s.Q.CountRoles(ctx, q.SearchPtr())
		if err != nil {
			return err
		}
		rows, err := s.Q.ListRoles(ctx, store.ListRolesParams{Search: q.SearchPtr(), Lim: int32(q.Limit), Off: q.Offset()})
		if err != nil {
			return err
		}
		roles, counts := make([]store.Role, len(rows)), make([]int32, len(rows))
		for i, r := range rows {
			roles[i], counts[i] = r.Role, r.UserCount
		}
		v, err := s.views(ctx, roles, counts)
		if err != nil {
			return err
		}
		return httpx.Page(c, v, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.get(c.UserContext(), id)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/roles", "roles.manage", func(c *fiber.Ctx) error {
		var b createReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		id := app.NewID()
		err := s.Q.CreateRole(c.UserContext(), store.CreateRoleParams{ID: id, Name: b.Name, Description: httpx.TrimPtr(b.Description), RequireTwoFactor: b.RequireTwoFactor})
		if httpx.IsUnique(err, "name") {
			return ErrNameTaken
		}
		if err != nil {
			return err
		}
		v, err := s.get(c.UserContext(), id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "role.create", "role", id, c.AllParams(), v)
		return httpx.Created(c, v)
	})

	g.Patch("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		var b updateReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ctx := c.UserContext()
		cur, err := s.get(ctx, id)
		if err != nil {
			return err
		}
		if cur.IsSystem && b.Name != nil && *b.Name != cur.Name {
			return ErrSystem
		}
		err = s.Q.UpdateRole(ctx, store.UpdateRoleParams{ID: id, Name: httpx.TrimPtr(b.Name), Description: httpx.TrimPtr(b.Description), RequireTwoFactor: b.RequireTwoFactor})
		if httpx.IsUnique(err, "name") {
			return ErrNameTaken
		}
		if err != nil {
			return err
		}
		v, err := s.get(ctx, id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "role.update", "role", id, c.AllParams(), v)
		return httpx.OK(c, v)
	})

	g.Delete("/roles/:id", "roles.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		ctx := c.UserContext()
		cur, err := s.get(ctx, id)
		if err != nil {
			return err
		}
		if cur.IsSystem {
			return ErrSystem
		}
		if cur.UserCount > 0 {
			return ErrInUse
		}
		if err := s.Q.DeleteRole(ctx, id); err != nil {
			return err
		}
		s.RBAC.Invalidate(ctx, id)
		s.Audit.From(c, "role.delete", "role", id, c.AllParams(), fiber.Map{"id": id})
		return httpx.OK(c, fiber.Map{"id": id})
	})

	g.Put("/roles/:id/permissions", "roles.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		var b permsReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		for _, p := range b.Permissions {
			if !policy.IsKnown(p.Resource, p.Action) || !slices.Contains(policy.Scopes, p.Scope) {
				return httpx.Validation("permissions: unknown resource/action/scope")
			}
		}
		ctx := c.UserContext()
		cur, err := s.get(ctx, id)
		if err != nil {
			return err
		}
		if cur.IsSystem {
			return ErrSystem
		}
		next := map[string]bool{}
		for _, p := range b.Permissions {
			next[p.Resource+"."+p.Action] = true
		}
		for _, p := range cur.Permissions {
			key := p.Resource + "." + p.Action
			if !slices.Contains(policy.Guarded, key) || next[key] {
				continue
			}
			n, err := s.Q.CountPermissionHolders(ctx, store.CountPermissionHoldersParams{Resource: p.Resource, Action: p.Action, RoleID: id})
			if err != nil {
				return err
			}
			if n == 0 {
				return errLastHolder(key)
			}
		}
		err = s.Tx(ctx, func(q *store.Queries) error {
			if err := q.DeleteRolePermissions(ctx, id); err != nil {
				return err
			}
			for _, p := range b.Permissions {
				if err := q.InsertRolePermission(ctx, store.InsertRolePermissionParams{ID: app.NewID(), RoleID: id, Resource: p.Resource, Action: p.Action, Scope: p.Scope}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		s.RBAC.Invalidate(ctx, id)
		v, err := s.get(ctx, id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "role.permissions.set", "role", id, c.AllParams(), v)
		return httpx.OK(c, v)
	})
}
