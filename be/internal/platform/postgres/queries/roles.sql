-- name: ListRoles :many
SELECT sqlc.embed(r), (SELECT count(*) FROM "user" u WHERE u.role_id = r.id)::int AS user_count
FROM role r
WHERE sqlc.narg(search)::text IS NULL OR r.name ILIKE '%' || sqlc.narg(search) || '%'
ORDER BY r.name
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountRoles :one
SELECT count(*) FROM role r
WHERE sqlc.narg(search)::text IS NULL OR r.name ILIKE '%' || sqlc.narg(search) || '%';

-- name: GetRole :one
SELECT sqlc.embed(r), (SELECT count(*) FROM "user" u WHERE u.role_id = r.id)::int AS user_count
FROM role r WHERE r.id = $1;

-- name: RolePermissions :many
SELECT role_id, resource, action, scope FROM role_permission
WHERE role_id = ANY(@role_ids::text[])
ORDER BY resource, action;

-- name: CreateRole :exec
INSERT INTO role (id, name, description, require_two_factor) VALUES ($1, $2, $3, $4);

-- name: UpdateRole :exec
UPDATE role SET
  name = COALESCE(sqlc.narg(name), name),
  description = COALESCE(sqlc.narg(description), description),
  require_two_factor = COALESCE(sqlc.narg(require_two_factor), require_two_factor)
WHERE id = sqlc.arg(id);

-- name: DeleteRole :exec
DELETE FROM role WHERE id = $1;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permission WHERE role_id = $1;

-- name: InsertRolePermission :exec
INSERT INTO role_permission (id, role_id, resource, action, scope) VALUES ($1, $2, $3, $4, $5);

-- name: CountPermissionHolders :one
SELECT count(*) FROM role_permission WHERE resource = $1 AND action = $2 AND role_id <> $3;

-- name: RoleExists :one
SELECT EXISTS(SELECT 1 FROM role WHERE id = $1);

-- name: RoleNames :many
SELECT id, name FROM role;

-- name: SyncPermission :exec
INSERT INTO permission (id, resource, action) VALUES ($1, $2, $3)
ON CONFLICT (resource, action) DO NOTHING;

-- name: DeleteUnknownPermissions :exec
DELETE FROM permission WHERE NOT (resource || '.' || action = ANY(@keys::text[]));

-- name: UpsertSystemRole :exec
-- 2FA starts off so a fresh install (and scripts/smoke.sh) can sign in; an
-- admin turns it on per role, and re-seeding keeps that choice.
INSERT INTO role (id, name, description, is_system, require_two_factor) VALUES ($1, $2, $3, true, false)
ON CONFLICT (id) DO UPDATE SET is_system = true;

-- name: UpsertRolePermission :exec
INSERT INTO role_permission (id, role_id, resource, action, scope) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (role_id, resource, action) DO UPDATE SET scope = EXCLUDED.scope;

-- name: UpsertSuperAdmin :exec
INSERT INTO "user" (id, email, full_name, role_id, password_hash, email_verified_at)
VALUES ($1, $2, 'Super Admin', $3, $4, now())
ON CONFLICT (email) DO UPDATE SET role_id = EXCLUDED.role_id, is_active = true;
