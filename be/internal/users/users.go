// Package users is admin user management: CRUD, soft delete, bulk, import/export.
package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/auth"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/store"
)

var (
	ErrNotFound      = httpx.E(404, "USER_NOT_FOUND", "User not found")
	ErrEmailTaken    = httpx.E(409, "EMAIL_TAKEN", "Email already in use")
	ErrModifySelf    = httpx.E(403, "CANNOT_MODIFY_SELF", "You cannot change your own role or deactivate yourself")
	ErrInvalidRole   = httpx.E(400, "INVALID_ROLE", "Role does not exist")
	ErrAlreadyActive = httpx.E(409, "USER_ALREADY_ACTIVE", "User has already accepted the invite")
	ErrDeleted       = httpx.E(409, "USER_DELETED", "User is deleted; restore it first")
	ErrNotDeleted    = httpx.E(409, "USER_NOT_DELETED", "User is not deleted")
)

func errImport(detail string) *httpx.Error {
	return httpx.E(400, "INVALID_IMPORT_FILE", "Import file is invalid: "+detail)
}

type Service struct {
	*app.Deps
	Auth  *auth.Service
	Audit *audit.Log
}

type View struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	FullName        *string    `json:"fullName"`
	AvatarURL       *string    `json:"avatarUrl"`
	RoleID          string     `json:"roleId"`
	RoleName        string     `json:"roleName"`
	IsActive        bool       `json:"isActive"`
	Invited         bool       `json:"invited"`
	EmailVerifiedAt *time.Time `json:"emailVerifiedAt"`
	DeletedAt       *time.Time `json:"deletedAt"`
	LastLoginAt     *time.Time `json:"lastLoginAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

func (s *Service) toView(u store.User, roleName string) View {
	var avatar *string
	if u.AvatarKey != nil && s.Files.Enabled() {
		if v := s.Files.URL(*u.AvatarKey); v != "" {
			avatar = &v
		}
	}
	return View{u.ID, u.Email, u.FullName, avatar, u.RoleID, roleName, u.IsActive, u.PasswordHash == nil, u.EmailVerifiedAt, u.DeletedAt, u.LastLoginAt, u.CreatedAt}
}

func (s *Service) Get(ctx context.Context, id string) (View, error) {
	row, err := s.Q.GetUser(ctx, id)
	if err != nil {
		return View{}, ErrNotFound
	}
	return s.toView(row.User, row.RoleName), nil
}

type CreateInput struct {
	Email    string `json:"email" validate:"required,email"`
	FullName string `json:"fullName" validate:"required,min=1,max=100"`
	RoleID   string `json:"roleId" validate:"required,uuid"`
	Password string `json:"password" validate:"omitempty,password"`
}

// Create adds a user. Without a password the user is invited by email.
func (s *Service) Create(ctx context.Context, in CreateInput) (View, error) {
	if ok, err := s.Q.RoleExists(ctx, in.RoleID); err != nil || !ok {
		return View{}, ErrInvalidRole
	}
	var hash *string
	if in.Password != "" {
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			return View{}, err
		}
		hash = &h
	}
	id := app.NewID()
	name := strings.TrimSpace(in.FullName)
	err := s.Q.CreateUser(ctx, store.CreateUserParams{ID: id, Email: strings.ToLower(strings.TrimSpace(in.Email)), FullName: &name, RoleID: in.RoleID, PasswordHash: hash})
	if httpx.IsUnique(err, "email") {
		return View{}, ErrEmailTaken
	}
	if err != nil {
		return View{}, err
	}
	v, err := s.Get(ctx, id)
	if err != nil {
		return v, err
	}
	if hash == nil {
		err = s.Auth.SendInvite(ctx, v.ID, v.Email)
	}
	return v, err
}

type UpdateInput struct {
	FullName *string `json:"fullName" validate:"omitempty,min=1,max=100"`
	RoleID   *string `json:"roleId" validate:"omitempty,uuid"`
	IsActive *bool   `json:"isActive"`
}

func (s *Service) Update(ctx context.Context, actorID, id string, in UpdateInput) (View, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if cur.DeletedAt != nil {
		return cur, ErrDeleted
	}
	roleChange := in.RoleID != nil && *in.RoleID != cur.RoleID
	deactivate := in.IsActive != nil && !*in.IsActive
	if actorID == id && (roleChange || deactivate) {
		return cur, ErrModifySelf
	}
	if in.RoleID != nil {
		if ok, err := s.Q.RoleExists(ctx, *in.RoleID); err != nil || !ok {
			return cur, ErrInvalidRole
		}
	}
	if err := s.Q.UpdateUser(ctx, store.UpdateUserParams{ID: id, FullName: httpx.TrimPtr(in.FullName), RoleID: in.RoleID, IsActive: in.IsActive}); err != nil {
		return cur, err
	}
	// Role change or deactivation ends existing sessions.
	if roleChange || deactivate {
		if err := s.Q.RevokeUserFamilies(ctx, id); err != nil {
			return cur, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) Remove(ctx context.Context, actorID, id string) (View, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if actorID == id {
		return cur, ErrModifySelf
	}
	if cur.DeletedAt != nil {
		return cur, nil
	}
	now := time.Now()
	if err := s.Q.SetUserDeleted(ctx, store.SetUserDeletedParams{ID: id, DeletedAt: &now}); err != nil {
		return cur, err
	}
	if err := s.Q.RevokeUserFamilies(ctx, id); err != nil {
		return cur, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Restore(ctx context.Context, id string) (View, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return cur, err
	}
	if cur.DeletedAt == nil {
		return cur, ErrNotDeleted
	}
	if err := s.Q.SetUserDeleted(ctx, store.SetUserDeletedParams{ID: id}); err != nil {
		return cur, err
	}
	return s.Get(ctx, id)
}

type listQuery struct {
	httpx.Pagination
	RoleID   string `query:"roleId" validate:"omitempty,uuid"`
	Sort     string `query:"sort" validate:"omitempty,oneof=createdAt email fullName lastLoginAt"`
	Order    string `query:"order" validate:"omitempty,oneof=asc desc"`
	Deleted  string `query:"deleted" validate:"omitempty,oneof=true false"`
	IsActive string `query:"isActive" validate:"omitempty,oneof=true false"`
}

func (q *listQuery) normalize() {
	q.Normalize()
	if q.Sort == "" {
		q.Sort = "createdAt"
	}
	if q.Order == "" {
		q.Order = "desc"
	}
}

func (q listQuery) isActive() *bool {
	if q.IsActive == "" {
		return nil
	}
	v := q.IsActive == "true"
	return &v
}

func (q listQuery) params(limit int32, offset int32) store.ListUsersParams {
	return store.ListUsersParams{RoleID: httpx.Ptr(q.RoleID), IsActive: q.isActive(), Deleted: q.Deleted == "true", Search: q.SearchPtr(), Sort: q.Sort, Dir: q.Order, Lim: limit, Off: offset}
}

func validID(c *fiber.Ctx) (string, error) {
	id := c.Params("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", httpx.Validation("id: must be a UUID")
	}
	return id, nil
}

type bulkReq struct {
	IDs    []string `json:"ids" validate:"required,min=1,max=100,dive,uuid"`
	Action string   `json:"action" validate:"required,oneof=activate deactivate delete restore"`
}

type bulkFailure struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Service) Routes(g rbac.Guarded) {
	g.Get("/users", "users.manage", func(c *fiber.Ctx) error {
		var q listQuery
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		q.normalize()
		ctx := c.UserContext()
		p := q.params(int32(q.Limit), q.Offset())
		total, err := s.Q.CountUsers(ctx, store.CountUsersParams{RoleID: p.RoleID, IsActive: p.IsActive, Deleted: p.Deleted, Search: p.Search})
		if err != nil {
			return err
		}
		rows, err := s.Q.ListUsers(ctx, p)
		if err != nil {
			return err
		}
		out := make([]View, len(rows))
		for i, r := range rows {
			out[i] = s.toView(r.User, r.RoleName)
		}
		return httpx.Page(c, out, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/users/export", "users.manage", s.export)
	g.Get("/users/import-template", "users.manage", func(c *fiber.Ctx) error {
		return httpx.File(c, "users-import-template.csv", "text/csv; charset=utf-8", importTemplate())
	})
	g.Post("/users/import/preview", "users.manage", func(c *fiber.Ctx) error {
		rows, err := s.validateImport(c)
		if err != nil {
			return err
		}
		out := make([]previewRow, len(rows))
		for i, r := range rows {
			out[i] = r.previewRow
		}
		return httpx.OK(c, out)
	})
	g.Post("/users/import", "users.manage", s.importUsers)

	g.Post("/users/bulk", "users.manage", func(c *fiber.Ctx) error {
		var b bulkReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ctx, actor := c.UserContext(), app.CurrentUser(c).ID
		succeeded, failed := []string{}, []bulkFailure{}
		seen := map[string]bool{}
		for _, id := range b.IDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			var err error
			switch b.Action {
			case "delete":
				_, err = s.Remove(ctx, actor, id)
			case "restore":
				_, err = s.Restore(ctx, id)
			default:
				active := b.Action == "activate"
				_, err = s.Update(ctx, actor, id, UpdateInput{IsActive: &active})
			}
			var de *httpx.Error
			switch {
			case err == nil:
				succeeded = append(succeeded, id)
			case errors.As(err, &de):
				failed = append(failed, bulkFailure{id, de.Code, de.Message})
			default:
				return err
			}
		}
		res := fiber.Map{"succeeded": succeeded, "failed": failed}
		s.Audit.From(c, "user.bulk", "user", "", c.AllParams(), res)
		return httpx.OK(c, res)
	})

	g.Get("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.Get(c.UserContext(), id)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/users", "users.manage", func(c *fiber.Ctx) error {
		var b CreateInput
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := s.Create(c.UserContext(), b)
		if err != nil {
			return err
		}
		s.Audit.From(c, "user.create", "user", v.ID, c.AllParams(), v)
		return httpx.Created(c, v)
	})

	g.Patch("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		var b UpdateInput
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := s.Update(c.UserContext(), app.CurrentUser(c).ID, id, b)
		if err != nil {
			return err
		}
		s.Audit.From(c, "user.update", "user", id, c.AllParams(), v)
		return httpx.OK(c, v)
	})

	g.Post("/users/:id/invite", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		ctx := c.UserContext()
		v, err := s.Get(ctx, id)
		if err != nil {
			return err
		}
		if v.DeletedAt != nil {
			return ErrDeleted
		}
		if !v.Invited {
			return ErrAlreadyActive
		}
		if err := s.Auth.SendInvite(ctx, v.ID, v.Email); err != nil {
			return err
		}
		s.Audit.From(c, "user.invite", "user", id, c.AllParams(), nil)
		return httpx.NoContent(c)
	})

	g.Post("/users/:id/reset-password", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		var b struct {
			Password string `json:"password" validate:"required,password"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ctx := c.UserContext()
		if _, err := s.Get(ctx, id); err != nil {
			return err
		}
		hash, err := auth.HashPassword(b.Password)
		if err != nil {
			return err
		}
		err = s.Tx(ctx, func(q *store.Queries) error {
			if err := q.SetPassword(ctx, store.SetPasswordParams{ID: id, PasswordHash: &hash}); err != nil {
				return err
			}
			return q.RevokeUserFamilies(ctx, id)
		})
		if err != nil {
			return err
		}
		s.Audit.From(c, "user.reset_password", "user", id, c.AllParams(), nil)
		return httpx.NoContent(c)
	})

	g.Delete("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.Remove(c.UserContext(), app.CurrentUser(c).ID, id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "user.delete", "user", id, c.AllParams(), v)
		return httpx.OK(c, v)
	})

	g.Post("/users/:id/restore", "users.manage", func(c *fiber.Ctx) error {
		id, err := validID(c)
		if err != nil {
			return err
		}
		v, err := s.Restore(c.UserContext(), id)
		if err != nil {
			return err
		}
		s.Audit.From(c, "user.restore", "user", id, c.AllParams(), v)
		return httpx.Created(c, v)
	})
}
