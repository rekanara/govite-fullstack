package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"govite/internal/kernel"
	"govite/internal/platform/postgres/store"
)

type txKey struct{}

// DB is the shared adapter: it implements kernel.Tx and hands repositories
// the query set bound to the transaction carried in ctx, if any.
type DB struct {
	Pool *pgxpool.Pool
	q    *store.Queries
}

func New(pool *pgxpool.Pool) *DB { return &DB{Pool: pool, q: store.New(pool)} }

// Do runs fn in one transaction. Nested calls join the outer transaction.
func (d *DB) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, d.Pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Q returns queries bound to the ctx transaction, or to the pool.
func (d *DB) Q(ctx context.Context) *store.Queries {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return d.q.WithTx(tx)
	}
	return d.q
}

// Err turns a missing row into kernel.ErrNotFound; other errors pass through.
func Err(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return kernel.ErrNotFound
	}
	return err
}

// IsUnique reports a unique violation on a constraint containing `on`.
func IsUnique(err error, on string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, on)
}
