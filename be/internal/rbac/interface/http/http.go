package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"govite/internal/platform/httpx"
	"govite/internal/policy"
	"govite/internal/rbac/application"
	"govite/internal/rbac/domain"
)

const scopeKey = "scope"

type Guard struct{ Service *application.Service }

func (g Guard) Require(resource, action string) fiber.Handler {
	if !policy.IsKnown(resource, action) {
		panic("rbac: unknown permission " + resource + "." + action + " (add it to policy.Permissions)")
	}
	return func(c *fiber.Ctx) error {
		p := httpx.Principal(c)
		if p.ID == "" {
			return httpx.ErrNotAuthenticated
		}
		scope, err := g.Service.Resolve(c.UserContext(), domain.Principal{ID: p.ID, RoleID: p.RoleID}, resource, action)
		if err != nil {
			return err
		}
		c.Locals(scopeKey, scope)
		return c.Next()
	}
}

type Router struct {
	Router fiber.Router
	Guard  Guard
}

func (r Router) handle(method, path, permission string, handlers ...fiber.Handler) {
	resource, action, ok := strings.Cut(permission, ".")
	if !ok {
		panic("rbac: permission must be resource.action, got " + permission)
	}
	r.Router.Add(method, path, append([]fiber.Handler{r.Guard.Require(resource, action)}, handlers...)...)
}

func (r Router) Get(path, permission string, handlers ...fiber.Handler) {
	r.handle("GET", path, permission, handlers...)
}
func (r Router) Post(path, permission string, handlers ...fiber.Handler) {
	r.handle("POST", path, permission, handlers...)
}
func (r Router) Put(path, permission string, handlers ...fiber.Handler) {
	r.handle("PUT", path, permission, handlers...)
}
func (r Router) Patch(path, permission string, handlers ...fiber.Handler) {
	r.handle("PATCH", path, permission, handlers...)
}
func (r Router) Delete(path, permission string, handlers ...fiber.Handler) {
	r.handle("DELETE", path, permission, handlers...)
}

func CurrentScope(c *fiber.Ctx) domain.Scope {
	scope, _ := c.Locals(scopeKey).(domain.Scope)
	return scope
}
