// Package rbac wires permission resolution layers and re-exports the guarded
// router used by feature routes.
package rbac

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"govite/internal/platform/postgres/store"
	"govite/internal/rbac/application"
	"govite/internal/rbac/domain"
	"govite/internal/rbac/infrastructure"
	rbachttp "govite/internal/rbac/interface/http"
)

type (
	Grant         = domain.Grant
	Scope         = domain.Scope
	TeamHierarchy = domain.TeamHierarchy
	Guarded       = rbachttp.Router
)

type RBAC struct{ *application.Service }

// New builds the service; client nil disables the Redis cache.
func New(q *store.Queries, client *redis.Client) *RBAC {
	svc := &application.Service{Repository: infrastructure.Repository{Queries: q}}
	if client != nil {
		svc.Cache = infrastructure.Cache{Client: client}
	}
	return &RBAC{svc}
}

func (r *RBAC) Require(resource, action string) fiber.Handler {
	return rbachttp.Guard{Service: r.Service}.Require(resource, action)
}

func (r *RBAC) Guard(router fiber.Router) Guarded {
	return Guarded{Router: router, Guard: rbachttp.Guard{Service: r.Service}}
}

func CurrentScope(c *fiber.Ctx) Scope { return rbachttp.CurrentScope(c) }
