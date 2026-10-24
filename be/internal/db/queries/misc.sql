-- name: InsertAudit :exec
INSERT INTO audit_log (id, actor_user_id, action, entity_type, entity_id, before, after)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAudit :many
SELECT * FROM audit_log
WHERE (sqlc.narg(actor_user_id)::text IS NULL OR actor_user_id = sqlc.narg(actor_user_id))
  AND (sqlc.narg(entity_type)::text IS NULL OR entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(entity_id)::text IS NULL OR entity_id = sqlc.narg(entity_id))
  AND (sqlc.narg(action)::text IS NULL OR action LIKE '%' || sqlc.narg(action) || '%')
  AND (sqlc.narg(user_id)::text IS NULL OR actor_user_id = sqlc.narg(user_id)
       OR (entity_type = 'user' AND entity_id = sqlc.narg(user_id)))
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR created_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR created_at < sqlc.narg(to_at))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAudit :one
SELECT count(*) FROM audit_log
WHERE (sqlc.narg(actor_user_id)::text IS NULL OR actor_user_id = sqlc.narg(actor_user_id))
  AND (sqlc.narg(entity_type)::text IS NULL OR entity_type = sqlc.narg(entity_type))
  AND (sqlc.narg(entity_id)::text IS NULL OR entity_id = sqlc.narg(entity_id))
  AND (sqlc.narg(action)::text IS NULL OR action LIKE '%' || sqlc.narg(action) || '%')
  AND (sqlc.narg(user_id)::text IS NULL OR actor_user_id = sqlc.narg(user_id)
       OR (entity_type = 'user' AND entity_id = sqlc.narg(user_id)))
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR created_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR created_at < sqlc.narg(to_at));

-- name: GetSetting :one
SELECT value FROM app_setting WHERE id = $1;

-- name: PutSetting :exec
INSERT INTO app_setting (id, value) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: ListNotifications :many
SELECT * FROM notification
WHERE user_id = sqlc.arg(user_id) AND (NOT sqlc.arg(unread_only)::bool OR read_at IS NULL)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountNotifications :one
SELECT count(*) AS total, count(*) FILTER (WHERE read_at IS NULL) AS unread
FROM notification WHERE user_id = $1;

-- name: MarkNotificationRead :one
UPDATE notification SET read_at = COALESCE(read_at, now())
WHERE id = $1 AND user_id = $2 RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notification SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;

-- name: ListPublishedAnnouncements :many
SELECT sqlc.embed(a), COALESCE(u.full_name, u.email)::text AS author_name
FROM announcement a JOIN "user" u ON u.id = a.author_id
WHERE a.status = 'published'
ORDER BY a.is_pinned DESC, a.published_at DESC NULLS FIRST, a.created_at DESC, a.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountPublishedAnnouncements :one
SELECT count(*) FROM announcement WHERE status = 'published';

-- name: ListAttachments :many
SELECT * FROM attachment WHERE owner_user_id = $1 ORDER BY created_at DESC;
