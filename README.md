# Cirrus

A daily cloud image service. 

[![CI](https://github.com/hmcalister/cirrus/actions/workflows/cicd.yml/badge.svg)](https://github.com/hmcalister/cirrus/actions/workflows/cicd.yml)
[![codecov](https://codecov.io/gh/hmcalister/cirrus/branch/main/graph/badge.svg)](https://codecov.io/gh/hmcalister/cirrus)
[![Go Reference](https://pkg.go.dev/badge/github.com/hmcalister/cirrus.svg)](https://pkg.go.dev/github.com/hmcalister/cirrus)

## Getting started

Cirrus needs a `.env` file in the repository root. At a minimum it must set the values that have no default in `compose.yaml`:

```sh
POSTGRES_USER=cirrus
POSTGRES_PASSWORD=change-me
POSTGRES_DB=cirrus

EMAIL_SMTP_HOST=smtp.example.com
EMAIL_SMTP_PORT=465
EMAIL_SMTP_USERNAME=user
EMAIL_SMTP_PASSWORD=secret
EMAIL_FROM_ADDRESS=noreply@example.com
EMAIL_FROM_NAME=Cirrus

S3_BUCKET=cirrus-clouds
S3_REGION=region
S3_ENDPOINT=https://bucket.com
```

The AWS credentials are optional and fall back to the default AWS chain:

```sh
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

Then:

```sh
make init    # install the web dependencies
make up      # build the image and start the app, mailer, and database
make web-dev # for hot reloading the frontend
```

The app listens on `http://localhost:8080`. Signing in emails a single-use link through the configured SMTP server. For local development, point `EMAIL_SMTP_HOST` at a local SMTP catcher such as Mailpit or MailHog. To develop the front end connect to `http://localhost:5173`, which still routes requests to the server on `:8080`.

The schema in `sqlc/schema.sql` is mounted as the Postgres entrypoint, so it is applied only when the `database-data` volume is first created. After a schema change, remove the volume before starting again, or apply the change to the running database by hand.

## Configuration

All runtime configuration is environment variables. `internal/config` reads them, and each executable loads only the variables it uses, so a missing variable stops that process rather than its sibling. `DEBUG` is optional and turns on debug logging when set to anything.

`compose.yaml` is the single place that defines the defaults:

- `x-common-environment` holds the variables shared by the server and the mailer: the database and SMTP settings.
- `x-server-environment` adds the HTTP timeouts, auth settings, and magic link rate limits.
- `x-mailer-environment` adds the daily send time, the BCC batch size, and the bucket settings.
- `.env` supplies the secrets and provider-specific values that have no default: `POSTGRES_*`, `EMAIL_SMTP_*`, `EMAIL_FROM_*`, `S3_BUCKET`, `S3_REGION`, and optionally the AWS credentials.
- Everything else has a default in compose, written as `${VARIABLE:-default}`.

The database container reads `.env` directly for its own `POSTGRES_*` setup.

## Directory layout

| Path | Purpose |
| --- | --- |
| `cmd/server` | Process entrypoint: configuration, logging, database pool, S3 store, Stratus, HTTP server, graceful shutdown |
| `cmd/mailer` | Daily email job: waits for the configured time, sends the day's cloud, deletes it, and alerts admins when images run low |
| `cmd/generate-openapi` | Writes `openapi.yaml` from the Huma operations. Run by `make openapi` and by the Dockerfile |
| `internal/api` | The HTTP surface: Huma operations, auth and role middleware, request logging, and the embedded-SPA handler |
| `internal/config` | Environment configuration |
| `internal/database` | sqlc output. Generated. Do not edit |
| `internal/filestore` | The cloud image store interface and a no-op implementation |
| `internal/filestore/s3` | The S3-compatible image store |
| `internal/logging` | `slog` setup |
| `internal/postgres` | The `pgxpool` connection pool |
| `sqlc` | sqlc config, schema, and queries |
| `web` | The Vue frontend application |
| `web/dist` | Vite output, embedded by `web/embed.go` |

## Roles and the admin page

Roles live in `user_accounts.roles` as a text array. A standard user has no roles. Operations that need a role are guarded by `makeRequiredRoleMiddleware` in `internal/api/middleware.go`.

To grant the role by hand:

```sh
podman compose exec database-container sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

```sql
UPDATE user_accounts SET roles = array_append(roles, 'admin') WHERE email = 'user@example.com' AND NOT ('admin' = ANY (roles));
```

## How the API contract works

OpenAPI allows the frontend to use backend types. For example, `Account`, defined in `internal/api/handlers.go`:

```go
type Account struct {
    Email  string `json:"email" doc:"Signed-in email address."`
    Active bool   `json:"active" doc:"Whether daily emails are delivered."`
}
```

After running `make openapi`, the same shape is available in TypeScript:

```ts
import type { components } from "@/api/schema";

type Account = components["schemas"]["Account"];
```
