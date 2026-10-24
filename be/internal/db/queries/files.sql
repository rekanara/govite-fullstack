-- name: SetAvatarKey :exec
UPDATE "user" SET avatar_key = $2 WHERE id = $1;

-- name: CreateAttachment :exec
INSERT INTO attachment (id, owner_user_id, key, filename, content_type, size) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetAttachment :one
SELECT * FROM attachment WHERE id = $1;

-- name: DeleteAttachment :exec
DELETE FROM attachment WHERE id = $1;

-- name: ListAttachments :many
SELECT * FROM attachment WHERE owner_user_id = $1 ORDER BY created_at DESC LIMIT 50;
