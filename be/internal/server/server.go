// Package server wires every feature into one Fiber app.
package server

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"govite/internal/account"
	"govite/internal/app"
	"govite/internal/audit"
	"govite/internal/auth"
	"govite/internal/httpx"
	"govite/internal/rbac"
	"govite/internal/roles"
	"govite/internal/settings"
	"govite/internal/system"
	"govite/internal/users"
)

var validRequestID = regexp.MustCompile(`^[\w-]{1,64}$`)

// New builds the HTTP app. Routes live under /api/v1.
func New(d *app.Deps) *fiber.App {
	f := fiber.New(fiber.Config{
		// fasthttp reuses request buffers; Immutable makes every string from
		// c.Params/c.Query/c.Body safe to keep after the handler returns.
		Immutable:             true,
		ErrorHandler:          httpx.ErrorHandler,
		DisableStartupMessage: true,
		BodyLimit:             4 << 20,
		ProxyHeader:           proxyHeader(d.Cfg.TrustProxy),
	})
	f.Use(recover.New(), requestID, accessLog(d.Cfg.LogLevel), helmet.New())
	if len(d.Cfg.CORSOrigins) > 0 {
		f.Use(cors.New(cors.Config{AllowOrigins: strings.Join(d.Cfg.CORSOrigins, ","), AllowCredentials: true, AllowHeaders: "Authorization,Content-Type,X-Client,X-Request-Id"}))
	}

	store := limiterStorage(d.Redis)
	globalLimit := limiter.New(limiter.Config{Max: d.Cfg.ThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany})
	loginLimit := limiter.New(limiter.Config{
		Max: d.Cfg.LoginThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany,
		KeyGenerator: func(c *fiber.Ctx) string { return "login:" + c.IP() },
	})

	api := f.Group("/api/v1", globalLimit)

	authSvc := auth.NewService(d)
	rb := rbac.New(d)
	auditLog := audit.New(d)
	settingsSvc := &settings.Service{Deps: d, Audit: auditLog}
	sys := &system.Service{Deps: d}
	if d.Mail == nil {
		panic("server: Deps.Mail is nil")
	}

	// Public.
	sys.PublicRoutes(api)
	settingsSvc.PublicRoutes(api)
	authSvc.Routes(api, rb, loginLimit)

	// Signed in, no permission: self-service.
	me := api.Group("", authSvc.Authenticate)
	(&account.Service{Deps: d, Auth: authSvc, Audit: auditLog}).Routes(me)
	sys.NotificationRoutes(me)

	// Signed in + permission. Guarded routes must name their permission.
	g := rb.Guard(me)
	(&users.Service{Deps: d, Auth: authSvc, Audit: auditLog}).Routes(g)
	(&roles.Service{Deps: d, RBAC: rb, Audit: auditLog}).Routes(g)
	auditLog.Routes(g)
	settingsSvc.Routes(g)
	sys.Routes(g)

	f.Use(func(*fiber.Ctx) error { return fiber.ErrNotFound })
	return f
}

func tooMany(*fiber.Ctx) error {
	return httpx.E(429, "TOO_MANY_REQUESTS", "Too Many Requests")
}

func proxyHeader(trust bool) string {
	if trust {
		return fiber.HeaderXForwardedFor
	}
	return ""
}

func requestID(c *fiber.Ctx) error {
	id := c.Get(fiber.HeaderXRequestID)
	if !validRequestID.MatchString(id) {
		id = uuid.NewString()
	}
	c.Locals("requestid", id)
	c.Set(fiber.HeaderXRequestID, id)
	return c.Next()
}

func accessLog(level string) fiber.Handler {
	if level == "silent" {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			} else if de, ok := err.(*httpx.Error); ok {
				status = de.Status
			}
		}
		slog.Info("http", "method", c.Method(), "path", c.Path(), "status", status, "ms", time.Since(start).Milliseconds(), "requestId", c.Locals("requestid"))
		return err
	}
}

// limiterStorage keeps rate-limit counters in Redis when configured, so limits
// hold across instances; otherwise fiber's in-memory store is used.
func limiterStorage(r *redis.Client) fiber.Storage {
	if r == nil {
		return nil
	}
	return redisStorage{r}
}

type redisStorage struct{ r *redis.Client }

const limiterPrefix = "ratelimit:"

func (s redisStorage) Get(key string) ([]byte, error) {
	v, err := s.r.Get(context.Background(), limiterPrefix+key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return v, err
}

func (s redisStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.r.Set(context.Background(), limiterPrefix+key, val, exp).Err()
}

func (s redisStorage) Delete(key string) error {
	return s.r.Del(context.Background(), limiterPrefix+key).Err()
}

func (s redisStorage) Reset() error { return nil }
func (s redisStorage) Close() error { return nil }
