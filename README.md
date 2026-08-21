# Army Chess

Army Chess is a self-hosted web application for playing four-player, two-versus-two Chinese Army Chess (四国军棋) online with friends.

## Overview

- Create private rooms and invite friends with a room code.
- Play as one of four players, with opposite seats forming a team.
- Join active rooms as a spectator.
- Support hidden-information and visible-piece game modes.
- Reconnect to active games and review completed matches.
- Deploy the application on a personal server with Docker Compose.

## Status

The repository now contains the v1 vertical slice: a Go HTTP/WebSocket
service, the React command-table client, board/rules/realtime contracts,
PostgreSQL migrations, and a production-oriented Docker Compose deployment.

## Run with Docker Compose

On a cloud VPS with Docker Compose v2:

```sh
cp .env.example deploy/.env
# Edit deploy/.env: set DOMAIN and a strong POSTGRES_PASSWORD.
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
```

Point DNS for `DOMAIN` at the VPS. Caddy terminates TLS automatically when the
domain resolves and ports 80/443 are reachable. Check the service with:

```sh
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
curl https://chess.example.com/readyz
```

PostgreSQL data and Caddy certificates live in named Docker volumes. See
[PLAN.md](PLAN.md) for the complete product specification and
[docs/rules.md](docs/rules.md) for the executable v1 rules baseline.
