// Package system serves health probes, metrics, the admin status page and
// the maintenance cleanup.
package system

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"govite/internal/app"
	"govite/internal/httpx"
	"govite/internal/rbac"
)

var started = time.Now()

// Version is set at build time: -ldflags "-X govite/internal/system.Version=1.2.3".
var Version = ""

type Service struct {
	*app.Deps
	PushEnabled    bool
	PaymentEnabled bool
	RetentionDays  func(context.Context) int
}

func probe(ctx context.Context, fn func(context.Context) error) (string, *int64) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	t := time.Now()
	if err := fn(ctx); err != nil {
		return "down", nil
	}
	ms := time.Since(t).Milliseconds()
	return "up", &ms
}

func (s *Service) checks(ctx context.Context) (db, redis fiber.Map) {
	dbStatus, dbMs := probe(ctx, s.Pool.Ping)
	redisStatus, redisMs := "down", (*int64)(nil)
	if s.Redis != nil {
		redisStatus, redisMs = probe(ctx, func(ctx context.Context) error { return s.Redis.Ping(ctx).Err() })
	}
	return fiber.Map{"status": dbStatus, "latencyMs": dbMs}, fiber.Map{"status": redisStatus, "latencyMs": redisMs}
}

func (s *Service) PublicRoutes(r fiber.Router) {
	live := func(c *fiber.Ctx) error { return httpx.OK(c, fiber.Map{"status": "ok"}) }
	r.Get("/health", live)
	r.Get("/health/live", live)
	r.Get("/metrics", s.metrics)
	r.Get("/health/ready", func(c *fiber.Ctx) error {
		db, redis := s.checks(c.UserContext())
		res := fiber.Map{"database": db["status"], "redis": redis["status"]}
		// Redis is optional: only required when configured.
		if db["status"] == "down" || (s.Redis != nil && redis["status"] == "down") {
			raw, _ := json.Marshal(res)
			return httpx.E(503, "NOT_READY", "Dependencies unavailable").WithDetail(string(raw))
		}
		res["status"] = "ok"
		return httpx.OK(c, res)
	})
}

func (s *Service) Routes(g rbac.Guarded) {
	s.maintenanceRoutes(g)
	g.Get("/system/status", "settings.manage", func(c *fiber.Ctx) error {
		db, redis := s.checks(c.UserContext())
		var version any
		if Version != "" {
			version = Version
		}
		return httpx.OK(c, fiber.Map{
			"version": version, "environment": s.Cfg.Env, "uptimeSeconds": int(time.Since(started).Seconds()),
			"checks":       fiber.Map{"database": db, "redis": redis},
			"integrations": fiber.Map{"storage": s.Files.Enabled(), "push": s.PushEnabled, "payment": s.PaymentEnabled, "mailer": s.Cfg.SMTPHost != ""},
		})
	})
}
