-- name: GetUserByEmail :one
SELECT * FROM "user" WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = $1;

-- name: TouchLastLogin :exec
UPDATE "user" SET last_login_at = now() WHERE id = $1;

-- name: SetPassword :exec
UPDATE "user" SET password_hash = $2 WHERE id = $1;

-- name: AcceptInvite :exec
UPDATE "user" SET full_name = $2, password_hash = $3, email_verified_at = now() WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE "user" SET email_verified_at = now() WHERE id = $1;

-- name: CreateFamily :exec
INSERT INTO refresh_token_family (id, user_id, user_agent, ip_address) VALUES ($1, $2, $3, $4);

-- name: AddRefreshToken :exec
INSERT INTO refresh_token (id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, $4);

-- name: FindRefreshToken :one
SELECT t.id, t.family_id, t.expires_at, t.used_at, f.user_id, f.revoked_at
FROM refresh_token t JOIN refresh_token_family f ON f.id = t.family_id
WHERE t.token_hash = $1;

-- Compare-and-set: of two concurrent refreshes with one token, only one wins.
-- name: MarkRefreshTokenUsed :execrows
UPDATE refresh_token SET used_at = now(), replaced_by_token_hash = $2
WHERE id = $1 AND used_at IS NULL;

-- name: RevokeFamily :exec
UPDATE refresh_token_family SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeUserFamilies :exec
UPDATE refresh_token_family SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeOtherFamilies :exec
UPDATE refresh_token_family SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL AND id <> $2;

-- name: GetFamily :one
SELECT * FROM refresh_token_family WHERE id = $1;

-- name: ListSessions :many
SELECT f.id, f.user_agent, f.ip_address, f.created_at, t.created_at AS last_used_at
FROM refresh_token_family f
JOIN LATERAL (
  SELECT rt.created_at FROM refresh_token rt
  WHERE rt.family_id = f.id AND rt.used_at IS NULL AND rt.expires_at > now()
  ORDER BY rt.created_at DESC LIMIT 1
) t ON true
WHERE f.user_id = $1 AND f.revoked_at IS NULL
ORDER BY f.created_at DESC;

-- name: InvalidateUserTokens :exec
UPDATE user_token SET used_at = now()
WHERE user_id = $1 AND purpose = $2 AND channel = $3 AND used_at IS NULL;

-- name: CreateUserToken :exec
INSERT INTO user_token (id, user_id, purpose, channel, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: FindUserToken :one
SELECT * FROM user_token
WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now();

-- name: LatestOtp :one
SELECT * FROM user_token
WHERE user_id = $1 AND purpose = $2 AND channel = 'mobile' AND used_at IS NULL
  AND expires_at > now() AND attempts < $3
ORDER BY created_at DESC LIMIT 1;

-- name: BumpUserTokenAttempts :exec
UPDATE user_token SET attempts = attempts + 1 WHERE id = $1;

-- name: UseUserToken :execrows
UPDATE user_token SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- name: GetTwoFactor :one
SELECT * FROM user_two_factor WHERE user_id = $1;

-- name: SetTwoFactorSecret :exec
INSERT INTO user_two_factor (user_id, secret_encrypted) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE
SET secret_encrypted = EXCLUDED.secret_encrypted, enabled_at = NULL, updated_at = now();

-- name: EnableTwoFactor :exec
UPDATE user_two_factor SET enabled_at = COALESCE(enabled_at, now()), updated_at = now()
WHERE user_id = $1;

-- name: DeleteTwoFactor :exec
DELETE FROM user_two_factor WHERE user_id = $1;

-- name: DeleteBackupCodes :exec
DELETE FROM user_backup_code WHERE user_id = $1;

-- name: InsertBackupCode :exec
INSERT INTO user_backup_code (id, user_id, code_hash) VALUES ($1, $2, $3);

-- name: UseBackupCode :execrows
UPDATE user_backup_code SET used_at = now()
WHERE id = (
  SELECT b.id FROM user_backup_code b
  WHERE b.user_id = $1 AND b.code_hash = $2 AND b.used_at IS NULL LIMIT 1
) AND used_at IS NULL;

-- name: CountBackupCodes :one
SELECT count(*) FROM user_backup_code WHERE user_id = $1 AND used_at IS NULL;
