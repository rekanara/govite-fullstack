-- name: ListAnnouncements :many
SELECT sqlc.embed(a), COALESCE(u.full_name, u.email)::text AS author_name
FROM announcement a JOIN "user" u ON u.id = a.author_id
WHERE (sqlc.narg(status)::text IS NULL OR a.status = sqlc.narg(status))
  AND (sqlc.narg(search)::text IS NULL OR a.title ILIKE '%' || sqlc.narg(search) || '%'
       OR a.content ILIKE '%' || sqlc.narg(search) || '%')
ORDER BY a.is_pinned DESC, a.published_at DESC NULLS FIRST, a.created_at DESC, a.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAnnouncements :one
SELECT count(*) FROM announcement a
WHERE (sqlc.narg(status)::text IS NULL OR a.status = sqlc.narg(status))
  AND (sqlc.narg(search)::text IS NULL OR a.title ILIKE '%' || sqlc.narg(search) || '%'
       OR a.content ILIKE '%' || sqlc.narg(search) || '%');

-- name: GetAnnouncement :one
SELECT sqlc.embed(a), COALESCE(u.full_name, u.email)::text AS author_name
FROM announcement a JOIN "user" u ON u.id = a.author_id WHERE a.id = $1;

-- name: CreateAnnouncement :exec
INSERT INTO announcement (id, title, content, image_url, is_pinned, status, published_at, author_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- An omitted field stays untouched; set_image_url distinguishes "clear" from "omitted".
-- name: UpdateAnnouncement :exec
UPDATE announcement SET
  title = COALESCE(sqlc.narg(title), title),
  content = COALESCE(sqlc.narg(content), content),
  image_url = CASE WHEN sqlc.arg(set_image_url)::bool THEN sqlc.narg(image_url) ELSE image_url END,
  is_pinned = COALESCE(sqlc.narg(is_pinned), is_pinned),
  status = COALESCE(sqlc.narg(status), status),
  published_at = CASE WHEN sqlc.arg(publish_now)::bool THEN now() ELSE published_at END,
  updated_at = now()
WHERE id = sqlc.arg(id);

-- name: DeleteAnnouncement :exec
DELETE FROM announcement WHERE id = $1;

-- Claim so two concurrent publishes push exactly once.
-- name: ClaimAnnouncementNotify :execrows
UPDATE announcement SET notified_at = now() WHERE id = $1 AND notified_at IS NULL AND status = 'published';

-- name: CountAnnouncementsUsingImage :one
SELECT count(*) FROM announcement WHERE image_url = $1;
