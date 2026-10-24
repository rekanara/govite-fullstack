// Package app holds the shared dependencies handed to every feature package.
package app

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"govite/internal/config"
	"govite/internal/mail"
	"govite/internal/storage"
	"govite/internal/store"
)

type Deps struct {
	Cfg   config.Config
	Pool  *pgxpool.Pool
	Q     *store.Queries
	Redis *redis.Client // nil when REDIS_URL is unset
	Mail  mail.Sender
	Files *storage.Store // nil when object storage is not configured
}

// Tx runs fn in one transaction; any error rolls back.
func (d *Deps) Tx(ctx context.Context, fn func(q *store.Queries) error) error {
	return pgx.BeginFunc(ctx, d.Pool, func(tx pgx.Tx) error { return fn(d.Q.WithTx(tx)) })
}

// NewID returns a time-ordered UUIDv7.
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// Principal is the authenticated caller, set by auth.Authenticate.
type Principal struct {
	ID        string
	RoleID    string
	SessionID string
}

const principalKey = "principal"

func SetPrincipal(c *fiber.Ctx, p Principal) { c.Locals(principalKey, p) }

// CurrentUser returns the caller; only valid behind auth.Authenticate.
func CurrentUser(c *fiber.Ctx) Principal {
	p, _ := c.Locals(principalKey).(Principal)
	return p
}
