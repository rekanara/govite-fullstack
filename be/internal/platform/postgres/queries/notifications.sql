-- name: DeleteDeviceTokenElsewhere :exec
DELETE FROM fcm_device WHERE token = $1 AND NOT (user_id = $2 AND device_id = $3);

-- name: UpsertDevice :exec
INSERT INTO fcm_device (id, user_id, device_id, token, device) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, device_id) DO UPDATE SET token = EXCLUDED.token, device = EXCLUDED.device, updated_at = now();

-- name: DeleteDevice :exec
DELETE FROM fcm_device WHERE user_id = $1 AND device_id = $2;

-- name: DeleteDevicesByToken :exec
DELETE FROM fcm_device WHERE token = ANY(@tokens::text[]);

-- name: ActiveUserIDs :many
SELECT id FROM "user" WHERE is_active AND deleted_at IS NULL AND id > @after::text ORDER BY id LIMIT @lim;

-- Rows come back only for users that did not already have this dedupe_key.
-- name: InsertNotifications :many
INSERT INTO notification (id, user_id, type, title, body, data, dedupe_key)
SELECT ids.i, users.u, @type::text, @title::text, @body::text, @data::jsonb, @dedupe_key::text
FROM unnest(@ids::text[]) WITH ORDINALITY AS ids(i, n)
JOIN unnest(@user_ids::text[]) WITH ORDINALITY AS users(u, n) USING (n)
ON CONFLICT (user_id, dedupe_key) DO NOTHING
RETURNING user_id;

-- name: PushDevices :many
SELECT d.id, d.token FROM fcm_device d JOIN "user" u ON u.id = d.user_id
WHERE d.user_id = ANY(@user_ids::text[]) AND u.is_active AND u.notification_push_enabled
  AND d.id > @after::text
ORDER BY d.id LIMIT @lim;
