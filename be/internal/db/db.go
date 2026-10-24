// Package db embeds the migrations and runs them with goose.
package db

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate runs a goose command ("up", "down", "status", ...) against the pool.
func Migrate(ctx context.Context, pool *pgxpool.Pool, command string, args ...string) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return goose.RunContext(ctx, command, sqlDB, "migrations", args...)
}
