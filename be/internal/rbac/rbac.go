// Package rbac resolves a caller's permission scope. Guarded routes must name
// their permission (see Guarded), so an undeclared route cannot compile.
package rbac

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/policy"
	"govite/internal/store"
)

var ErrTeamScope = httpx.E(403, "TEAM_SCOPE_UNAVAILABLE", "Team scope requires a hierarchy provider")

func ErrNotGranted(perm string) *httpx.Error {
	return httpx.E(403, "PERMISSION_NOT_GRANTED", "Permission not granted: "+perm)
}

type Grant struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Scope    string `json:"scope"`
}

// Scope is what the caller may touch for the route's permission.
type Scope struct {
	Kind               string // self | team | all
	UserID             string
	SubordinateUserIDs []string
}

// TeamHierarchy is optional; without it `team` grants are denied, never widened.
type TeamHierarchy func(ctx context.Context, userID string) ([]string, error)

type RBAC struct {
	q     *store.Queries
	redis *redis.Client
	Team  TeamHierarchy
}

func New(d *app.Deps) *RBAC { return &RBAC{q: d.Q, redis: d.Redis} }

const cacheTTL = 60 * time.Second

func cacheKey(roleID string) string { return "role-permissions:" + roleID }

// Grants returns a role's permissions, cached in Redis for 60s.
// Redis failures degrade to a DB read.
func (r *RBAC) Grants(ctx context.Context, roleID string) ([]Grant, error) {
	if r.redis != nil {
		if raw, err := r.redis.Get(ctx, cacheKey(roleID)).Bytes(); err == nil {
			var g []Grant
			if json.Unmarshal(raw, &g) == nil {
				return g, nil
			}
		}
	}
	rows, err := r.q.RolePermissions(ctx, []string{roleID})
	if err != nil {
		return nil, err
	}
	g := make([]Grant, len(rows))
	for i, row := range rows {
		g[i] = Grant{row.Resource, row.Action, row.Scope}
	}
	if r.redis != nil {
		raw, _ := json.Marshal(g)
		if err := r.redis.Set(ctx, cacheKey(roleID), raw, cacheTTL).Err(); err != nil {
			slog.WarnContext(ctx, "rbac cache write failed", "err", err)
		}
	}
	return g, nil
}

// Invalidate drops a role's cached grants; call after any permission change.
func (r *RBAC) Invalidate(ctx context.Context, roleID string) {
	if r.redis != nil {
		if err := r.redis.Del(ctx, cacheKey(roleID)).Err(); err != nil {
			slog.WarnContext(ctx, "rbac cache invalidate failed", "err", err)
		}
	}
}

func (r *RBAC) Resolve(ctx context.Context, p app.Principal, resource, action string) (Scope, error) {
	grants, err := r.Grants(ctx, p.RoleID)
	if err != nil {
		return Scope{}, err
	}
	for _, g := range grants {
		if g.Resource != resource || g.Action != action {
			continue
		}
		switch g.Scope {
		case "all":
			return Scope{Kind: "all"}, nil
		case "team":
			if r.Team == nil {
				return Scope{}, ErrTeamScope
			}
			ids, err := r.Team(ctx, p.ID)
			return Scope{Kind: "team", UserID: p.ID, SubordinateUserIDs: ids}, err
		default:
			return Scope{Kind: "self", UserID: p.ID}, nil
		}
	}
	return Scope{}, ErrNotGranted(resource + "." + action)
}

const scopeKey = "scope"

// Require enforces a permission and stores the resolved scope. It panics at
// boot on a permission missing from policy.Permissions.
func (r *RBAC) Require(resource, action string) fiber.Handler {
	if !policy.IsKnown(resource, action) {
		panic("rbac: unknown permission " + resource + "." + action + " (add it to policy.Permissions)")
	}
	return func(c *fiber.Ctx) error {
		p := app.CurrentUser(c)
		if p.ID == "" {
			return httpx.ErrNotAuthenticated
		}
		s, err := r.Resolve(c.UserContext(), p, resource, action)
		if err != nil {
			return err
		}
		c.Locals(scopeKey, s)
		return c.Next()
	}
}

// Guarded wraps a router so every route must name its permission: the
// signature makes an undeclared route a compile error (fails closed).
type Guarded struct {
	r    fiber.Router
	rbac *RBAC
}

func (r *RBAC) Guard(router fiber.Router) Guarded { return Guarded{router, r} }

// Perm is "resource.action", e.g. "users.manage".
func (g Guarded) handle(method, path, perm string, h ...fiber.Handler) {
	res, act, ok := strings.Cut(perm, ".")
	if !ok {
		panic("rbac: permission must be resource.action, got " + perm)
	}
	g.r.Add(method, path, append([]fiber.Handler{g.rbac.Require(res, act)}, h...)...)
}

func (g Guarded) Get(path, perm string, h ...fiber.Handler)    { g.handle("GET", path, perm, h...) }
func (g Guarded) Post(path, perm string, h ...fiber.Handler)   { g.handle("POST", path, perm, h...) }
func (g Guarded) Put(path, perm string, h ...fiber.Handler)    { g.handle("PUT", path, perm, h...) }
func (g Guarded) Patch(path, perm string, h ...fiber.Handler)  { g.handle("PATCH", path, perm, h...) }
func (g Guarded) Delete(path, perm string, h ...fiber.Handler) { g.handle("DELETE", path, perm, h...) }

// CurrentScope returns the scope set by Require.
func CurrentScope(c *fiber.Ctx) Scope {
	s, _ := c.Locals(scopeKey).(Scope)
	return s
}
