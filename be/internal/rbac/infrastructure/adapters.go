package infrastructure

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"govite/internal/platform/postgres/store"
	"govite/internal/rbac/domain"
)

type Repository struct{ Queries *store.Queries }

func (r Repository) Grants(ctx context.Context, roleID string) ([]domain.Grant, error) {
	rows, err := r.Queries.RolePermissions(ctx, []string{roleID})
	if err != nil {
		return nil, err
	}
	grants := make([]domain.Grant, len(rows))
	for i, row := range rows {
		grants[i] = domain.Grant{Resource: row.Resource, Action: row.Action, Scope: row.Scope}
	}
	return grants, nil
}

type Cache struct{ Client *redis.Client }

func cacheKey(roleID string) string { return "role-permissions:" + roleID }

func (c Cache) Get(ctx context.Context, roleID string) ([]domain.Grant, bool) {
	raw, err := c.Client.Get(ctx, cacheKey(roleID)).Bytes()
	if err != nil {
		return nil, false
	}
	var grants []domain.Grant
	if json.Unmarshal(raw, &grants) != nil {
		return nil, false
	}
	return grants, true
}

func (c Cache) Set(ctx context.Context, roleID string, grants []domain.Grant) {
	raw, _ := json.Marshal(grants)
	if err := c.Client.Set(ctx, cacheKey(roleID), raw, 60*time.Second).Err(); err != nil {
		slog.WarnContext(ctx, "rbac cache write failed", "err", err)
	}
}

func (c Cache) Delete(ctx context.Context, roleID string) {
	if err := c.Client.Del(ctx, cacheKey(roleID)).Err(); err != nil {
		slog.WarnContext(ctx, "rbac cache invalidate failed", "err", err)
	}
}
