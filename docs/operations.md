# VPS operations

## First deployment

1. Install Docker Engine and the Compose v2 plugin on a Linux VPS.
2. Point a DNS A/AAAA record at the VPS.
3. Copy `.env.example` to `deploy/.env` and set `DOMAIN` and a strong,
   URL-safe `POSTGRES_PASSWORD`.
4. Start the stack:

   ```sh
   docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
   ```

5. Verify `db`, `migrate`, `app`, and `caddy` are healthy with `docker compose
   --env-file deploy/.env -f deploy/compose.yaml ps`.

Caddy obtains and renews the certificate when the domain resolves and ports
80 and 443 are reachable. For local testing, set `DOMAIN=localhost`,
`HTTP_PORT=8080`, `HTTPS_PORT=8443`, and `COOKIE_SECURE=false`, then use the
local certificate explicitly:

```sh
curl -k --resolve localhost:8443:127.0.0.1 https://localhost:8443/readyz
```

## Updates and backups

Pull the new source, review migration files, and rerun the same `up -d --build`
command. The migration service is idempotent for the current schema. Back up
before upgrades:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml exec -T db \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > army-chess-$(date +%Y%m%d).sql
```

Restore into a stopped application stack only after taking a second backup.
Named volumes hold PostgreSQL data and Caddy certificate state; do not remove
them during a routine application update.

## Logs and readiness

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f app
curl -fsS https://$DOMAIN/healthz
curl -fsS https://$DOMAIN/readyz
```

`/healthz` means the process is serving. `/readyz` also verifies the PostgreSQL
connection. The app exposes no public database port in the Compose file.
