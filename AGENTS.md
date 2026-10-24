# govite-fullstack

Go port of `~/Documents/Dev/private/Rekanara/codebase/monorepo-fullstack` (NestJS). HTTP contract (paths, envelope, cookies, error codes) is kept identical so the admin app works unchanged; check the Nest repo for behaviour questions.

## Layout

- `be/`: one Go module (`govite`). `cmd/api` = server + `migrate` + `seed` subcommands; `cmd/gen` = policy to TS. Feature packages in `internal/<feature>` (`auth`, `account`, `users`, `roles`, `audit`, `settings`, `system`, `announcements`, `notify`, `files`, `integrations`), shared bits in `internal/{app,httpx,rbac,mail,xport,policy,config,db,store}`.
- `fe/`: pnpm workspace, no Turbo. `apps/admin` (package `admin`), `packages/ui` (`@app/ui`), `packages/contracts` (`@app/contracts`, consumed from `src`, no build).
- `docker-compose.yml` at root: infra only (postgres 5434, postgres_test 5436, redis 6380, mailpit 1025/8025, MinIO 9100/9101).

## Commands

- BE (in `be/`): `make lint test`, `make test-e2e` (needs `postgres_test`), `make migrate`, `make seed`, `make gen`, `make dev` (air) / `make run`.
- FE (in `fe/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`; single app `pnpm --filter admin <script>`.
- Smoke: `scripts/smoke.sh` (`API_BASE_URL`, `SUPER_ADMIN_*`; the user must not need 2FA).

## Backend rules

- Fiber v2 with `Immutable: true` (fasthttp buffer reuse). Keep it; don't stash `c.Body()`/`c.Params()` slices otherwise.
- Handlers return errors; `httpx.ErrorHandler` renders the envelope `{data,meta?,error?,status}` with Indonesian `messageInd` (add codes to `httpx/envelope.go`). Domain errors are `httpx.E(status, CODE, message)` package vars.
- Success helpers: `httpx.OK/Created/Page/NoContent/File`. Lists use `httpx.Pagination` + `httpx.NewPageMeta`.
- Validation: `httpx.Body`/`httpx.Query` with `validate` tags (custom tag `password` reads `policy.Security`).
- Routes: public on `api`, self-service on `me` (JWT only), everything else on `rbac.Guarded` whose methods take a required `"resource.action"`, so an undeclared route can't compile; unknown permissions panic at boot.
- Permission catalog and security limits live only in `internal/policy`. After editing: `make gen` (writes `fe/packages/contracts/src/generated.ts`) and `make seed`.
- SQL: add queries in `internal/db/queries/*.sql`, run `sqlc generate`; never edit `internal/store`. Schema changes = new goose file in `internal/db/migrations` (embedded in the binary). Tables snake_case, IDs UUIDv7 (`app.NewID`).
- Multi-step writes go through `d.Tx`. Mutations call `audit.From(c, action, entityType, id, before, after)` explicitly (secrets redacted).
- Role permissions cache in Redis `role-permissions:<roleId>` 60s; call `rbac.Invalidate` on change. Redis optional (nil client = in-memory limiter, no cache).
- Passwords: argon2id PHC, node-argon2 compatible (Nest hashes still verify). 2FA secrets AES-GCM keyed by `REFRESH_TOKEN_PEPPER`.
- Tests: e2e in `internal/server/server_test.go` via `fiber.App.Test` against real Postgres; extend the TRUNCATE list when adding tables.

## Frontend rules

- Copied from the Nest repo's web app; see its AGENTS.md for routing/auth conventions. API base defaults to `/api/v1` (Vite proxy in dev).
- shadcn: `pnpm dlx shadcn@latest add <name> -c apps/admin` from `fe/`; primitives land in `packages/ui`.
- `apps/admin/src/routeTree.gen.ts` is generated; run `pnpm --filter admin exec vite build` after route changes before typecheck.
- New app (e.g. Astro landing): `apps/<name>`, depend on `@app/ui` / `@app/contracts`.
