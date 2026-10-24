// Package http serves health probes, metrics, admin status and cleanup.
package http

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"govite/internal/platform/httpx"
	"govite/internal/rbac"
	"govite/internal/system/application"
)

// Version is set at build time: -ldflags "-X govite/internal/system/interface/http.Version=1.2.3".
var Version = ""

type Integrations struct {
	Storage bool `json:"storage"`
	Push    bool `json:"push"`
	Payment bool `json:"payment"`
	Mailer  bool `json:"mailer"`
}

type Handler struct {
	Svc          *application.Service
	Pool         *pgxpool.Pool // metrics only
	Env          string
	Integrations Integrations
}

func (h Handler) PublicRoutes(r fiber.Router) {
	live := func(c *fiber.Ctx) error { return httpx.OK(c, fiber.Map{"status": "ok"}) }
	r.Get("/health", live)
	r.Get("/health/live", live)
	r.Get("/metrics", h.metrics)
	r.Get("/health/ready", func(c *fiber.Ctx) error {
		db, redis, ready := h.Svc.Checks(c.UserContext())
		res := fiber.Map{"database": db.Status, "redis": redis.Status}
		if !ready {
			raw, _ := json.Marshal(res)
			return httpx.E(503, "NOT_READY", "Dependencies unavailable").WithDetail(string(raw))
		}
		res["status"] = "ok"
		return httpx.OK(c, res)
	})
}

func (h Handler) Routes(g rbac.Guarded) {
	g.Post("/system/maintenance/cleanup", "settings.manage", func(c *fiber.Ctx) error {
		r, err := h.Svc.Cleanup(c.UserContext())
		if err != nil {
			return err
		}
		return httpx.Created(c, r)
	})
	g.Get("/system/status", "settings.manage", func(c *fiber.Ctx) error {
		db, redis, _ := h.Svc.Checks(c.UserContext())
		var version any
		if Version != "" {
			version = Version
		}
		return httpx.OK(c, fiber.Map{
			"version": version, "environment": h.Env, "uptimeSeconds": int(time.Since(application.Started).Seconds()),
			"checks": fiber.Map{"database": db, "redis": redis}, "integrations": h.Integrations,
		})
	})
}

// metrics is a minimal Prometheus text exposition of Go runtime stats.
// ponytail: add prometheus/client_golang when you need histograms or labels.
func (h Handler) metrics(c *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	st := h.Pool.Stat()
	var b strings.Builder
	gauge := func(name, help string, v any) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, help, name, name, v)
	}
	gauge("process_uptime_seconds", "Process uptime.", int(time.Since(application.Started).Seconds()))
	gauge("go_goroutines", "Number of goroutines.", runtime.NumGoroutine())
	gauge("go_memstats_alloc_bytes", "Heap bytes allocated and in use.", m.Alloc)
	gauge("go_memstats_sys_bytes", "Bytes obtained from the OS.", m.Sys)
	gauge("go_gc_cycles_total", "Completed GC cycles.", m.NumGC)
	gauge("db_pool_acquired_conns", "Database connections in use.", st.AcquiredConns())
	gauge("db_pool_idle_conns", "Idle database connections.", st.IdleConns())
	gauge("db_pool_total_conns", "Open database connections.", st.TotalConns())
	c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
	return c.SendString(b.String())
}
