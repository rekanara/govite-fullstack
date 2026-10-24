// Package server wires every feature into one Fiber app.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"govite/internal/platform/docs"
	"govite/internal/platform/httpx"
	sysapp "govite/internal/system/application"
)

var validRequestID = regexp.MustCompile(`^[\w-]{1,64}$`)

// New builds the HTTP app (routes under /api/v1) and returns the system
// service so main can run the daily cleanup.
func New(d Deps) (*fiber.App, *sysapp.Service) {
	if d.Mail == nil {
		panic("server: Deps.Mail is nil")
	}
	f := fiber.New(fiber.Config{
		// fasthttp reuses request buffers; Immutable makes every string from
		// c.Params/c.Query/c.Body safe to keep after the handler returns.
		Immutable:             true,
		ErrorHandler:          httpx.ErrorHandler,
		DisableStartupMessage: true,
		BodyLimit:             12 << 20,
		// Without these a slow client holds a connection (and goroutine) forever.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		ProxyHeader:  proxyHeader(d.Cfg.TrustProxy),
	})
	f.Use(recoverPanic, requestID, accessLog(d.Cfg.LogLevel), helmet.New())
	if !d.Cfg.Production() {
		docs.Routes(f)
	}
	if len(d.Cfg.CORSOrigins) > 0 {
		f.Use(cors.New(cors.Config{AllowOrigins: strings.Join(d.Cfg.CORSOrigins, ","), AllowCredentials: true, AllowHeaders: "Authorization,Content-Type,X-Client,X-Request-Id"}))
	}

	store := limiterStorage(d.Redis)
	globalLimit := limiter.New(limiter.Config{Max: d.Cfg.ThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany})
	loginLimit := limiter.New(limiter.Config{
		Max: d.Cfg.LoginThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany,
		KeyGenerator: func(c *fiber.Ctx) string { return "login:" + c.IP() },
	})
	// Refresh is called by every open tab, so it gets its own, looser bucket.
	refreshLimit := limiter.New(limiter.Config{
		Max: 6 * d.Cfg.LoginThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany,
		KeyGenerator: func(c *fiber.Ctx) string { return "refresh:" + c.IP() },
	})
	webhookLimit := limiter.New(limiter.Config{
		Max: d.Cfg.WebhookThrottleLimit, Expiration: time.Minute, Storage: store, LimitReached: tooMany,
		KeyGenerator: func(c *fiber.Ctx) string { return "webhook:" + c.IP() },
	})

	api := f.Group("/api/v1", globalLimit)
	h, sys := wire(d)

	// Public.
	h.system.PublicRoutes(api)
	h.settings.PublicRoutes(api)
	h.integrations.PublicRoutes(api, webhookLimit)
	h.auth.Routes(api, loginLimit, refreshLimit)

	// Signed in, no permission: self-service.
	me := api.Group("", h.auth.Authenticate)
	h.auth.AccountRoutes(me)
	h.account.Routes(me)
	h.files.Routes(me)
	h.notify.Routes(me)

	// Signed in + permission. Guarded routes must name their permission.
	g := h.rbac.Guard(me)
	h.users.Routes(g)
	h.roles.Routes(g)
	h.audit.Routes(g)
	h.settings.Routes(g)
	h.announcements.Routes(g)
	h.integrations.Routes(g)
	h.system.Routes(g)

	f.Use(func(*fiber.Ctx) error { return fiber.ErrNotFound })
	return f, sys
}

// recoverPanic turns a panic into a 500 envelope, logs the stack as JSON and
// reports it to Sentry with the stack attached.
func recoverPanic(c *fiber.Ctx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.ErrorContext(c.UserContext(), "panic", "value", fmt.Sprint(r), "path", c.Path(), "requestId", c.Locals("requestid"), "stack", stack)
			sentry.CurrentHub().Recover(r)
			err = httpx.E(500, "INTERNAL_SERVER_ERROR", "Internal Server Error")
		}
	}()
	return c.Next()
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
			status = httpx.StatusOf(err)
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
