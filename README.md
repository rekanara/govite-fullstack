# govite-fullstack

Reusable fullstack starter: Go API + Vite/React admin. Each side runs on its own.

```
be/   Go 1.26, Fiber v2, pgx + sqlc, goose migrations
fe/   pnpm workspace: apps/admin (Vite SPA), packages/ui (shadcn), packages/contracts (zod)
```

## Features

- Auth: JWT access token + rotating refresh cookie (reuse revokes the session), forgot/reset password (web link or mobile OTP), invites, email verification, session list/revoke, mobile clients via `X-Client: mobile`.
- RBAC: roles, permission catalog, `self|team|all` scopes, role-required TOTP 2FA with backup codes.
- Users: list/sort/filter, bulk actions, soft delete/restore, CSV/XLSX export, CSV import with preview.
- Audit log with filters and CSV export. App settings and public branding. Health, Prometheus-style metrics and system status.
- Notifications: in-app + optional FCM push, device registration and push opt-outs.
- Announcements: drafts, pinning, image upload and one-time notification on publish.
- Optional MinIO/S3-compatible storage: avatars, branding, announcement images, private signed attachments.
- Verified, idempotent Midtrans webhooks; API key management; daily cleanup with a Postgres advisory lock; optional Sentry.

## Setup

```bash
docker compose up -d postgres redis mailpit minio     # postgres :5434, redis :6380, mailpit :8025, MinIO console :9101
cd be && cp .env.example .env && make migrate seed
cd fe && pnpm install && cp apps/admin/.env.example apps/admin/.env
```

## Development

```bash
cd be && make dev     # or: make run          → http://localhost:3000/api/v1
cd fe && pnpm dev     # admin                 → http://localhost:5173
```

Vite proxies `/api` to `:3000`, so the browser sees one origin (no CORS, refresh cookie just works).
Super Admin requires 2FA: the first login shows a QR code.
Outside production, Swagger UI is at `/api/docs`; raw OpenAPI is at `/api/docs-json` and `/api/docs-yaml`.

## Verification

```bash
cd be && make lint test
docker compose up -d postgres_test && make test-e2e
cd fe && pnpm lint && pnpm typecheck && pnpm test && pnpm build
API_BASE_URL=http://localhost:3000/api/v1 scripts/smoke.sh   # needs a user without 2FA
```

## Adding a frontend (e.g. SEO landing)

```bash
cd fe && pnpm create astro@latest apps/landing
```

Add `"@app/ui": "workspace:*"` (and `@astrojs/react` for React islands), import `@app/ui/globals.css`.
`packages/ui` already scans `apps/*/src/**/*.astro` for Tailwind classes.

## Production checklist

- `APP_ENV=production`, strong `JWT_ACCESS_SECRET`, `REFRESH_TOKEN_PEPPER`, `SUPER_ADMIN_PASSWORD`.
- Serve web and API on one origin behind HTTPS (refresh cookie is `Secure`, path `/api/v1/auth`). If they must be on separate origins, set `CORS_ORIGIN` and `VITE_API_BASE_URL`.
- Behind a reverse proxy set `TRUST_PROXY=true` so rate limits see the client IP.
- Set `REDIS_URL` when running more than one API instance (shared rate limits, permission cache).
- Run `api migrate` before serving traffic; `api seed` after changing the permission catalog.
- Optional production stack: `docker-compose.prod.yml` serves the SPA and proxies `/api` to the API.
