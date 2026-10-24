-- Indexes for foreign keys and hot lookups that were scanning whole tables
-- (found with EXPLAIN on 200k rows). Postgres does not index FKs on its own,
-- so cascade deletes and per-user lookups were sequential scans.
-- ponytail: plain CREATE INDEX locks writes on these tables while it runs.
-- On a large live table, run the same statements with CONCURRENTLY by hand.

-- +goose Up
CREATE INDEX "refresh_token_family_id_idx" ON "refresh_token"("family_id");
CREATE INDEX "refresh_token_family_user_id_idx" ON "refresh_token_family"("user_id") WHERE "revoked_at" IS NULL;
CREATE INDEX "user_role_id_idx" ON "user"("role_id");
CREATE INDEX "announcement_author_id_idx" ON "announcement"("author_id");
CREATE INDEX "audit_log_actor_user_id_created_at_idx" ON "audit_log"("actor_user_id", "created_at");
CREATE INDEX "audit_log_entity_created_at_idx" ON "audit_log"("entity_type", "entity_id", "created_at");

-- +goose Down
DROP INDEX "audit_log_entity_created_at_idx";
DROP INDEX "audit_log_actor_user_id_created_at_idx";
DROP INDEX "announcement_author_id_idx";
DROP INDEX "user_role_id_idx";
DROP INDEX "refresh_token_family_user_id_idx";
DROP INDEX "refresh_token_family_id_idx";
