// Package http mounts admin user management under /users.
package http

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/kernel"
	"govite/internal/platform/httpx"
	"govite/internal/platform/xport"
	"govite/internal/rbac"
	"govite/internal/users/application"
	"govite/internal/users/domain"
)

const importBytes = 1_000_000

type Handler struct {
	Svc   *application.Service
	Audit kernel.Auditor
}

type listQuery struct {
	httpx.Pagination
	RoleID   string `query:"roleId" validate:"omitempty,uuid"`
	Sort     string `query:"sort" validate:"omitempty,oneof=createdAt email fullName lastLoginAt"`
	Order    string `query:"order" validate:"omitempty,oneof=asc desc"`
	Deleted  string `query:"deleted" validate:"omitempty,oneof=true false"`
	IsActive string `query:"isActive" validate:"omitempty,oneof=true false"`
}

func (q *listQuery) filter() domain.Filter {
	q.Normalize()
	f := domain.Filter{RoleID: kernel.Ptr(q.RoleID), Deleted: q.Deleted == "true", Search: q.SearchPtr(), Sort: q.Sort, Order: q.Order}
	if f.Sort == "" {
		f.Sort = "createdAt"
	}
	if f.Order == "" {
		f.Order = "desc"
	}
	if q.IsActive != "" {
		v := q.IsActive == "true"
		f.IsActive = &v
	}
	return f
}

type updateReq struct {
	FullName *string `json:"fullName" validate:"omitempty,min=1,max=100"`
	RoleID   *string `json:"roleId" validate:"omitempty,uuid"`
	IsActive *bool   `json:"isActive"`
}

type bulkReq struct {
	IDs    []string `json:"ids" validate:"required,min=1,max=100,dive,uuid"`
	Action string   `json:"action" validate:"required,oneof=activate deactivate delete restore"`
}

func iso(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// readTable parses the uploaded CSV, capped at importBytes.
func readTable(c *fiber.Ctx) ([][]string, error) {
	up, err := httpx.FormFile(c, "file", importBytes)
	if err != nil {
		return nil, err
	}
	if len(up.Body) == 0 {
		return nil, domain.ErrImport("file is empty")
	}
	table, err := xport.ParseCSV(up.Body)
	if err != nil {
		return nil, domain.ErrImport(err.Error())
	}
	return table, nil
}

func (h Handler) audit(c *fiber.Ctx, action, id string, after any) {
	h.Audit.Record(c.UserContext(), action, "user", id, c.AllParams(), after)
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Get("/users", "users.manage", func(c *fiber.Ctx) error {
		var q listQuery
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		rows, total, err := h.Svc.List(c.UserContext(), q.filter(), int32(q.Limit), q.Offset())
		if err != nil {
			return err
		}
		return httpx.Page(c, rows, httpx.NewPageMeta(total, q.Page, q.Limit))
	})

	g.Get("/users/export", "users.manage", func(c *fiber.Ctx) error {
		var q struct {
			listQuery
			Format string `query:"format" validate:"omitempty,oneof=csv xlsx"`
		}
		if err := httpx.Query(c, &q); err != nil {
			return err
		}
		rows, err := h.Svc.Export(c.UserContext(), q.filter())
		if err != nil {
			return err
		}
		table := [][]any{{"email", "fullName", "role", "status", "emailVerifiedAt", "lastLoginAt", "createdAt"}}
		for _, v := range rows {
			table = append(table, []any{v.Email, v.FullName, v.RoleName, v.Status(), iso(v.EmailVerifiedAt), iso(v.LastLoginAt), iso(&v.CreatedAt)})
		}
		stamp := time.Now().UTC().Format(time.DateOnly)
		h.audit(c, "user.export", "", fiber.Map{"file": true})
		if q.Format == "xlsx" {
			return httpx.File(c, "users-"+stamp+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xport.XLSX(table, "Users"))
		}
		return httpx.File(c, "users-"+stamp+".csv", "text/csv; charset=utf-8", xport.CSV(table))
	})

	g.Get("/users/import-template", "users.manage", func(c *fiber.Ctx) error {
		return httpx.File(c, "users-import-template.csv", "text/csv; charset=utf-8", xport.CSV([][]any{{"email", "fullName", "role"}, {"jane@example.com", "Jane Doe", "Admin"}}))
	})

	g.Post("/users/import/preview", "users.manage", func(c *fiber.Ctx) error {
		table, err := readTable(c)
		if err != nil {
			return err
		}
		rows, err := h.Svc.ValidateImport(c.UserContext(), table)
		if err != nil {
			return err
		}
		out := make([]application.PreviewRow, len(rows))
		for i, r := range rows {
			out[i] = r.PreviewRow
		}
		return httpx.OK(c, out)
	})

	g.Post("/users/import", "users.manage", func(c *fiber.Ctx) error {
		table, err := readTable(c)
		if err != nil {
			return err
		}
		created, errs, err := h.Svc.Import(c.UserContext(), table)
		if err != nil {
			return err
		}
		res := fiber.Map{"created": created, "errors": errs}
		h.audit(c, "user.import", "", res)
		return httpx.Created(c, res)
	})

	g.Post("/users/bulk", "users.manage", func(c *fiber.Ctx) error {
		var b bulkReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		ok, failed, err := h.Svc.Bulk(c.UserContext(), b.IDs, b.Action)
		if err != nil {
			return err
		}
		res := fiber.Map{"succeeded": ok, "failed": failed}
		h.audit(c, "user.bulk", "", res)
		return httpx.OK(c, res)
	})

	g.Get("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		v, err := h.Svc.Get(c.UserContext(), id)
		if err != nil {
			return err
		}
		return httpx.OK(c, v)
	})

	g.Post("/users", "users.manage", func(c *fiber.Ctx) error {
		var b application.CreateInput
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.Create(c.UserContext(), b)
		if err != nil {
			return err
		}
		h.audit(c, "user.create", v.ID, v)
		return httpx.Created(c, v)
	})

	g.Patch("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b updateReq
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		v, err := h.Svc.Update(c.UserContext(), id, domain.Changes{FullName: httpx.TrimPtr(b.FullName), RoleID: b.RoleID, IsActive: b.IsActive})
		if err != nil {
			return err
		}
		h.audit(c, "user.update", id, v)
		return httpx.OK(c, v)
	})

	g.Post("/users/:id/invite", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		if err := h.Svc.Reinvite(c.UserContext(), id); err != nil {
			return err
		}
		h.audit(c, "user.invite", id, nil)
		return httpx.NoContent(c)
	})

	g.Post("/users/:id/reset-password", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		var b struct {
			Password string `json:"password" validate:"required,password"`
		}
		if err := httpx.Body(c, &b); err != nil {
			return err
		}
		if err := h.Svc.ResetPassword(c.UserContext(), id, b.Password); err != nil {
			return err
		}
		h.audit(c, "user.reset_password", id, nil)
		return httpx.NoContent(c)
	})

	g.Delete("/users/:id", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		v, err := h.Svc.Remove(c.UserContext(), id)
		if err != nil {
			return err
		}
		h.audit(c, "user.delete", id, v)
		return httpx.OK(c, v)
	})

	g.Post("/users/:id/restore", "users.manage", func(c *fiber.Ctx) error {
		id, err := httpx.ID(c, "id")
		if err != nil {
			return err
		}
		v, err := h.Svc.Restore(c.UserContext(), id)
		if err != nil {
			return err
		}
		h.audit(c, "user.restore", id, v)
		return httpx.Created(c, v)
	})
}
