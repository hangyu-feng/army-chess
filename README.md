# Army Chess / 四国军棋

Army Chess is a self-hosted web application for four-player, two-versus-two
Chinese Army Chess (四国军棋). It provides private invite-code rooms, four
player seats, spectators, hidden-information views, reconnection, persistent
matches, replay, profiles, and Docker Compose deployment.

## Current v1 scope

The repository contains a deployable v1 vertical slice:

- Go HTTP/WebSocket server with authoritative game state and server-side clocks.
- React command-table client for sign-in, rooms, setup, gameplay, spectators,
  profiles, and replay.
- PostgreSQL migrations and persistence for sessions, rooms, matches, events,
  snapshots, profiles, and saved layouts.
- Versioned board and realtime/API contracts in [`contracts/`](contracts/).
- Caddy HTTPS termination and Docker Compose orchestration.

The executable rules baseline is documented in [docs/rules.md](docs/rules.md),
with the canonical board diagram in [docs/board-layout.md](docs/board-layout.md).
The larger product roadmap and explicit non-goals are in [PLAN.md](PLAN.md).

## Production deployment on a VPS

### Requirements

You need:

- A Linux VPS with Docker Engine and the Docker Compose v2 plugin.
- A DNS A/AAAA record pointing your domain to the VPS.
- Firewall access to TCP ports 80 and 443.
- Git, if deploying from the source repository.

The VPS does not need Go, Node.js, or PostgreSQL installed locally. The Docker
build and Compose stack provide those components.

### 1. Install Docker

Install Docker Engine and the Compose plugin using the instructions for your
Linux distribution in the [Docker documentation](https://docs.docker.com/engine/install/).
Confirm that both commands work:

```sh
docker --version
docker compose version
```

### 2. Check out the application

Replace the repository URL with your GitHub repository URL:

```sh
git clone https://github.com/YOUR_GITHUB_USERNAME/YOUR_REPOSITORY.git
cd YOUR_REPOSITORY
```

### 3. Configure the deployment

Create the deployment environment file:

```sh
cp .env.example deploy/.env
```

Edit `deploy/.env` and set at least:

```dotenv
DOMAIN=chess.example.com
PUBLIC_BASE_URL=https://chess.example.com
POSTGRES_PASSWORD=replace-with-a-long-random-password
COOKIE_SECURE=true
HTTP_PORT=80
HTTPS_PORT=443
```

Use a long URL-safe password. A hexadecimal password can be generated with:

```sh
openssl rand -hex 32
```

Keep `deploy/.env` private. It is ignored by Git and contains the database
password used by the application.

### 4. Validate and start the stack

Validate Compose without printing the expanded configuration:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml config --quiet
```

Build the application image, apply database migrations, and start the services:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
```

The stack contains:

- `db`: PostgreSQL, with a named persistent data volume.
- `migrate`: one-shot migration service.
- `app`: the Go server and compiled React client on the internal port 8080.
- `caddy`: public HTTP/HTTPS edge and WebSocket reverse proxy.

### 5. Verify the deployment

Inspect service status:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
```

`db` and `app` should be healthy. `migrate` should show `Exited (0)` after
successfully applying migrations. `caddy` should be running.

Check the application endpoints:

```sh
curl -fsS https://chess.example.com/healthz
curl -fsS https://chess.example.com/readyz
```

`/healthz` confirms that the process is serving. `/readyz` also verifies the
PostgreSQL connection. Open `https://chess.example.com/` in a browser to use
the application.

Caddy obtains and renews the TLS certificate automatically when the domain
resolves to the VPS and ports 80 and 443 are reachable from the internet.

## Local Docker verification

For local testing, use non-standard ports and disable secure cookies because
the local Caddy certificate is self-signed:

```sh
cp .env.example deploy/.env
```

Set these values in `deploy/.env`:

```dotenv
DOMAIN=localhost
PUBLIC_BASE_URL=https://localhost:8443
POSTGRES_PASSWORD=local-development-password
COOKIE_SECURE=false
HTTP_PORT=8080
HTTPS_PORT=8443
```

Start the stack:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
```

Verify readiness through Caddy:

```sh
curl -k --resolve localhost:8443:127.0.0.1 \
  https://localhost:8443/readyz
```

Open `https://localhost:8443/` in a browser and accept the local certificate
warning. Stop the temporary stack without deleting its named volumes:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml down
```

Do not use `docker compose down -v` unless you intentionally want to delete
the PostgreSQL and Caddy volumes.

## Updating an installation

Back up the database before applying a schema or application update:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml exec -T db \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > army-chess-$(date +%Y%m%d-%H%M%S).sql
```

Pull the new source and rebuild:

```sh
git pull
docker compose --env-file deploy/.env -f deploy/compose.yaml config --quiet
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
```

The migration service runs before the application and applies new migration
files in order. Routine updates do not require removing volumes.

For a restore, first make a second backup and stop the application and Caddy.
Restore the SQL dump into a disposable or freshly prepared database before
bringing the application back online; do not blindly import a dump over a
live production database.

## Logs and troubleshooting

Follow application and edge logs:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f app
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f caddy
```

Common problems:

- **TLS certificate failure:** confirm DNS points to the VPS and TCP ports 80
  and 443 are open. Caddy needs both ports during certificate management.
- **502 from Caddy:** check `docker compose ... ps` and `docker compose ...
  logs app`; the app must pass `/readyz` before Caddy serves it.
- **Database readiness failure:** inspect `db` and `migrate` logs. Verify the
  password in `deploy/.env` and keep it URL-safe.
- **Local login problems:** use `COOKIE_SECURE=false` only for local HTTPS
  testing. Production must use `COOKIE_SECURE=true`.
- **Lost data after maintenance:** avoid `docker compose down -v`; named
  volumes contain PostgreSQL data and Caddy certificates.

## Optional: publish the image to GHCR

The workflow in
[.github/workflows/publish-image.yml](.github/workflows/publish-image.yml)
publishes a container when a `v*` Git tag is pushed or when the workflow is run
manually:

```sh
git add .github/workflows/publish-image.yml
git commit -m "ci: publish container image to GHCR"
git push
git tag v0.1.0
git push origin v0.1.0
```

The workflow publishes both a versioned tag and the moving `latest` tag:

```text
ghcr.io/YOUR_GITHUB_USERNAME/YOUR_REPOSITORY:v0.1.0
ghcr.io/YOUR_GITHUB_USERNAME/YOUR_REPOSITORY:latest
```

After the first workflow run, set the package visibility to Public in GitHub
if anonymous VPS pulls are desired. The current Compose file builds from
source; publishing an image does not change deployment automatically.

## Development and verification

For local development, install Go 1.26, Node.js 24, npm, and Docker. Then run:

```sh
make test
make build-image
```

The frontend can also be developed directly:

```sh
cd web
npm ci
npm run dev
```

Useful commands from the repository root:

```sh
make test-go
make test-web
make compose-up
make compose-down
```

Tests are kept separate from production code: backend tests live under
`server/tests/`, and frontend tests live under `web/tests/`. `make test` runs
both suites and the frontend production build.

## Security notes

- The database is not published as a host port.
- Production cookies use `HttpOnly`, `Secure`, and `SameSite=Lax` settings.
- Do not commit `deploy/.env`, database dumps, or registry credentials.
- v1 uses passwordless usernames. Anyone who enters the same username can
  access that username's profile and matches; do not treat usernames as an
  account-ownership mechanism.
- The current v1 does not include rate limiting, an admin console, or
  multi-replica room ownership.

## Project documentation

- [PLAN.md](PLAN.md): product specification and roadmap.
- [docs/rules.md](docs/rules.md): executable v1 rules baseline.
- [docs/board-layout.md](docs/board-layout.md): canonical board diagram, coordinates, and edge topology.
- [docs/architecture.md](docs/architecture.md): runtime architecture.
- [docs/operations.md](docs/operations.md): VPS operations reference.
- [contracts/](contracts/): board, OpenAPI, and realtime contracts.
