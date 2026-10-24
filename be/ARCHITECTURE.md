# Backend architecture

Each feature in `internal/<feature>` has four layers. Dependencies point inward only.

```
interface/http  ->  application  ->  domain  <-  infrastructure
```

| Layer | Holds | May import | Must not import |
|---|---|---|---|
| `domain` | entities, invariants, error values, ports (interfaces) | `kernel`, `policy`, other features' `domain` | Fiber, `platform/*`, pgx, Redis, `store` |
| `application` | use cases, transactions, audit calls | own `domain`, `kernel`, other features' `application`/`domain` | Fiber, `platform/*`, pgx, Redis, `store` |
| `infrastructure` | adapters for the ports: Postgres repos, crypto, TOTP, mail, FCM, Redis | `domain`, `platform/*`, `store` | `interface` |
| `interface/http` | route wiring, bind + validate, DTOs, status mapping | `application`, `domain`, `platform/httpx`, `rbac` | `infrastructure`, `store` |

Composition happens in one place: `internal/server/wire.go`. Nothing else constructs adapters.

## Rules

- **Errors** are `kernel.E(kind, CODE, message)` values declared in `domain`. `httpx.ErrorHandler` maps `Kind` to a status and renders the envelope. Add new codes to `platform/httpx/envelope.go`.
- **Caller** travels in `context`: `httpx.SetPrincipal` (auth middleware) writes it, use cases read `kernel.PrincipalFrom(ctx)`.
- **Transactions**: use cases call `kernel.Tx.Do(ctx, fn)`. Repositories obtain the transaction-bound queries through `postgres.DB.Q(ctx)`. Nested `Do` joins the outer transaction. Multi-step writes always go through `Do`.
- **Audit**: use cases (or the HTTP handler, when the action name is route-level) call `kernel.Auditor.Record`. Secrets are redacted by `audit/domain.Redact`.
- **Missing rows**: repositories return `postgres.Err(err)`, which maps `pgx.ErrNoRows` to `kernel.ErrNotFound`. Use cases translate it to the feature's own not-found error.
- **Ports only at real seams** (DB, object storage, mail, push, clock-like randomness). No interface with one implementation and no external dependency.
- **Cross-feature use** goes through the other feature's `application` (for example `users` uses `auth` to hash a password and send an invite, `announcements` uses `notify` to broadcast).

## Adding a feature

1. `domain`: entity, rules, errors, `Repository` port.
2. `application`: `Service` with the use cases.
3. `infrastructure`: implement the port with `postgres.DB` and `store` queries (`internal/platform/postgres/queries/*.sql`, then `sqlc generate`).
4. `interface/http`: `Handler{Svc}.Routes(g rbac.Guarded)`; every guarded route names its permission.
5. Wire it in `server/wire.go`, add the permission to `policy`, extend the e2e TRUNCATE list, document the routes in `platform/docs/openapi.json`.

## Checks

- `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./internal/.../domain ./internal/.../application | grep -E "fiber|httpx|postgres|pgx|redis|platform/"` must print nothing.
- `internal/kernel/rules_test.go` covers domain invariants without a database.
