.PHONY: test test-go test-web build-image compose-up compose-down

test: test-go test-web

test-go:
	GOCACHE=$${TMPDIR:-/tmp}/army-chess-go-cache GOPATH=$${TMPDIR:-/tmp}/army-chess-gopath go test ./...

test-web:
	cd web && npm run build

build-image:
	docker build -t army-chess:local .

compose-up:
	docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build

compose-down:
	docker compose --env-file deploy/.env -f deploy/compose.yaml down
