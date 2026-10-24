-- name: InsertWebhookEvent :one
INSERT INTO webhook_event (id, provider, event_key, event_type, payload) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (provider, event_key) DO NOTHING RETURNING id;

-- name: GetWebhookEventByKey :one
SELECT * FROM webhook_event WHERE provider = $1 AND event_key = $2;

-- name: GetWebhookEvent :one
SELECT * FROM webhook_event WHERE id = $1;

-- Claims a fresh/retried event. A concurrent delivery loses the compare-and-set.
-- name: ClaimWebhook :execrows
UPDATE webhook_event SET status = 'processing', error = NULL
WHERE id = $1 AND (status IN ('received', 'failed') OR (status = 'processing' AND received_at < now() - interval '5 minutes')); 

-- name: MarkWebhookProcessed :exec
UPDATE webhook_event SET status = 'processed', processed_at = now() WHERE id = $1;

-- name: MarkWebhookFailed :exec
UPDATE webhook_event SET status = 'failed', error = $2 WHERE id = $1;

-- name: ListWebhookEvents :many
SELECT * FROM webhook_event
WHERE (sqlc.narg(provider)::text IS NULL OR provider = sqlc.narg(provider))
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY received_at DESC, id DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountWebhookEvents :one
SELECT count(*) FROM webhook_event
WHERE (sqlc.narg(provider)::text IS NULL OR provider = sqlc.narg(provider))
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status));

-- name: ListAPIKeys :many
SELECT * FROM api_key ORDER BY created_at DESC;

-- name: CreateAPIKey :one
INSERT INTO api_key (id, name, prefix, key_hash) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateAPIKey :one
UPDATE api_key SET name = COALESCE(sqlc.narg(name), name), is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = sqlc.arg(id) RETURNING *;

-- name: DeleteAPIKey :execrows
DELETE FROM api_key WHERE id = $1;

-- name: PurgeUserTokens :execrows
DELETE FROM user_token WHERE used_at IS NOT NULL OR expires_at < $1;

-- name: PurgeRefreshTokens :execrows
DELETE FROM refresh_token WHERE expires_at < $1 OR used_at < $1;

-- name: PurgeWebhookEvents :execrows
DELETE FROM webhook_event WHERE received_at < $1 AND status = 'processed';

-- name: PurgeNotifications :execrows
DELETE FROM notification WHERE created_at < $1 AND read_at IS NOT NULL;
