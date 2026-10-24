package system

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// metrics is a minimal Prometheus text exposition of Go runtime stats.
// ponytail: add prometheus/client_golang when you need histograms or labels.
func (s *Service) metrics(c *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	st := s.Pool.Stat()
	var b strings.Builder
	gauge := func(name, help string, v any) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, help, name, name, v)
	}
	gauge("process_uptime_seconds", "Process uptime.", int(time.Since(started).Seconds()))
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
